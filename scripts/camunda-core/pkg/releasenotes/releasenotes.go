// Package releasenotes holds the pure release-notes transforms:
//
//   - HelmCLIVersion: the camunda.io/helmCLIVersion annotation value, clamped
//     across the Helm v3→v4 migration by Camunda minor.
//   - CliffGroups / ArtifactHubChanges: the artifacthub.io/changes annotation
//     block parsed from the git-cliff RELEASE-NOTES.md sections via the
//     keep-a-changelog map.
//   - ParseChartIdentity / AppMinor / AppStripLastSegment: Chart.yaml field reads.
package releasenotes

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// ChartIdentity holds the Chart.yaml fields the release-notes flow reads.
type ChartIdentity struct {
	Name       string `yaml:"name"`
	Version    string `yaml:"version"`
	AppVersion string `yaml:"appVersion"`
}

// ParseChartIdentity reads name/version/appVersion from Chart.yaml bytes. Reads
// only — Chart.yaml writes go through yq to preserve formatting.
func ParseChartIdentity(chartYAML []byte) (ChartIdentity, error) {
	var c ChartIdentity
	if err := yaml.Unmarshal(chartYAML, &c); err != nil {
		return ChartIdentity{}, fmt.Errorf("parse Chart.yaml: %w", err)
	}
	return c, nil
}

// AppMinor returns the first two dot-segments of an appVersion (8.10.x → 8.10).
func AppMinor(appVersion string) string {
	parts := strings.Split(appVersion, ".")
	if len(parts) <= 2 {
		return appVersion
	}
	return strings.Join(parts[:2], ".")
}

// stripLastDotSegRe matches a trailing ".<single-char>".
var stripLastDotSegRe = regexp.MustCompile(`\..$`)

// AppStripLastSegment removes a trailing ".<char>" from an appVersion (8.10.x → 8.10).
func AppStripLastSegment(appVersion string) string {
	return stripLastDotSegRe.ReplaceAllString(appVersion, "")
}

// HelmCLIVersion returns the camunda.io/helmCLIVersion annotation value for a
// chart whose Camunda minor is appVersion ("8.8", "8.10", ...), given the
// .tool-versions helm pin. Clamps by minor:
//
//   - 8.0–8.8: v3-only. If the pin is v4, clamp to 3.20.2; else the pin.
//   - 8.9:     transitional. If the pin is v4, list "3.20.2,<pin>"; else the pin.
//   - 8.10+:   the pin as-is.
func HelmCLIVersion(appVersion, toolVersionsPin string) string {
	isV4 := strings.HasPrefix(toolVersionsPin, "4")
	if major, minor, ok := splitMajorMinor(appVersion); ok && major == 8 {
		switch {
		case minor >= 0 && minor <= 8:
			if isV4 {
				return "3.20.2"
			}
			return toolVersionsPin
		case minor == 9:
			if isV4 {
				return "3.20.2," + toolVersionsPin
			}
			return toolVersionsPin
		}
	}
	return toolVersionsPin
}

func splitMajorMinor(v string) (major, minor int, ok bool) {
	parts := strings.Split(v, ".")
	if len(parts) < 2 {
		return 0, 0, false
	}
	maj, err1 := strconv.Atoi(parts[0])
	min, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	return maj, min, true
}

var (
	cliffGroupRe = regexp.MustCompile(`group\s*=\s*"([^"]*)"`)
	// Everything that is not a letter or whitespace (drops the `<!-- N -->`
	// sort-comment + digits).
	nonGroupNameRe = regexp.MustCompile(`[^\p{L}\s]+`)
)

// CliffGroups extracts the commit-parser group names from a cliff.toml, in order,
// cleaned of the `<!-- N -->` sort-comment and digits. The order feeds the
// section iteration in ArtifactHubChanges.
func CliffGroups(cliffTOML string) []string {
	var groups []string
	for _, m := range cliffGroupRe.FindAllStringSubmatch(cliffTOML, -1) {
		cleaned := strings.Join(strings.Fields(nonGroupNameRe.ReplaceAllString(m[1], " ")), " ")
		if cleaned != "" {
			groups = append(groups, cleaned)
		}
	}
	return groups
}

// kacMap maps git-cliff section groups to keep-a-changelog change kinds. Only
// these groups produce artifacthub.io/changes entries; others (Documentation,
// Revert) are dropped.
var kacMap = map[string]string{
	"Features": "added",
	"Refactor": "changed",
	"Fixes":    "fixed",
}

// trailingParen strips a trailing " (...)" PR-link suffix (greedy from the first " (").
var trailingParen = regexp.MustCompile(` \(.+\)$`)

// ArtifactHubChanges builds the artifacthub.io/changes annotation block from a
// git-cliff RELEASE-NOTES.md. orderedGroups is the cliff commit-parser group
// order (e.g. Features, Refactor, Fixes, Documentation, Revert); only the
// kac-mapped ones contribute, in that order. The returned string is the YAML
// block the shell merges into Chart.yaml:
//
//	annotations:
//	  artifacthub.io/changes: |
//	    - kind: added
//	      description: "..."
//
// hasItems reports whether any change entries were produced; callers decide what
// an empty set means.
func ArtifactHubChanges(releaseNotesMD string, orderedGroups []string) (block string, hasItems bool) {
	var b strings.Builder
	b.WriteString("annotations:\n  artifacthub.io/changes: |\n")
	for _, group := range orderedGroups {
		kind, ok := kacMap[group]
		if !ok {
			continue
		}
		for _, msg := range sectionBullets(releaseNotesMD, group) {
			fmt.Fprintf(&b, "    - kind: %s\n      description: \"%s\"\n", kind, cleanDescription(msg))
			hasItems = true
		}
	}
	return b.String(), hasItems
}

// sectionBullets returns the bullet lines of the RELEASE-NOTES.md section whose
// heading matches `^#+\s<group>`, up to the next heading, with the leading
// "- " stripped from each bullet.
func sectionBullets(md, group string) []string {
	headingRe := regexp.MustCompile(`^#+\s` + regexp.QuoteMeta(group))
	var out []string
	inSection := false
	for _, line := range strings.Split(md, "\n") {
		if !inSection {
			if headingRe.MatchString(line) {
				inSection = true
			}
			continue
		}
		if strings.HasPrefix(line, "#") { // next heading ends the section
			break
		}
		if strings.HasPrefix(line, "-") {
			out = append(out, strings.TrimPrefix(line, "- "))
		}
	}
	return out
}

// cleanDescription drops double quotes, then strips a trailing " (...)" suffix.
func cleanDescription(msg string) string {
	msg = strings.ReplaceAll(msg, `"`, "")
	return trailingParen.ReplaceAllString(msg, "")
}
