// Copyright 2026 Camunda Services GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"scripts/camunda-core/pkg/executil"
	"scripts/camunda-core/pkg/releasenotes"
	"scripts/camunda-core/pkg/releaseplease"
	"scripts/camunda-core/pkg/versionmatrix"
)

// runBackfillMatrix populates the release-time facts (release_date, helm_cli,
// release_tag) for every historical version-matrix.json entry that misses
// them, and reconciles historical image sets against the published README
// sections. One-time migration; the release pipeline maintains the fields for
// new entries (update-matrix at promotion, stamp-release at public release).
//
// Sources, in order of authority:
//
//   - helm_cli: the chart's camunda.io/helmCLIVersion annotation at the
//     entry's release tag (stamped at release time — correct even where the
//     README's "Helm CLI:" line was corrupted by pre-Go-port regeneration,
//     see #5080), then the README line, then the .tool-versions pin at the
//     release tag clamped per minor. An annotation value overrides a
//     disagreeing existing value; the weaker sources only fill empty fields.
//   - release_date / release_tag: the published GitHub release, tried under
//     the current tag scheme camunda-platform-<minor>-<version> and the
//     legacy scheme camunda-platform-<version>.
//   - chart_images / chart_enterprise_images (non-alpha minors only): the
//     entry's published README section. Pre-Go-port chores swept historical
//     entries with image sets rendered from the then-current working tree —
//     unfiltered and at the wrong tags (the bash generator applied its
//     registry.camunda.cloud filter only on the README display path, never at
//     JSON write). README sections were spliced once at each release and are
//     the per-release truth. Alpha minors are skipped: their sections predate
//     the promotion-time pipeline and their JSON entries are fresher.
//
// Entries that cannot be resolved are reported and fail the run (loud) —
// unless --allow-missing-release is set, which leaves release_date/release_tag
// empty for entries whose GitHub release genuinely does not exist.
func runBackfillMatrix(args []string) error {
	fs := flag.NewFlagSet("backfill-matrix", flag.ContinueOnError)
	var dryRun, allowMissingRelease bool
	fs.BoolVar(&dryRun, "dry-run", false, "report planned changes without writing files")
	fs.BoolVar(&allowMissingRelease, "allow-missing-release", false, "tolerate entries with no GitHub release (leave release_date empty)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := versionmatrix.LoadChartVersionsConfig(versionmatrix.ChartVersionsPath("."))
	if err != nil {
		return err
	}

	dirEntries, err := os.ReadDir("version-matrix")
	if err != nil {
		return fmt.Errorf("read version-matrix: %w", err)
	}

	var failures []string
	for _, de := range dirEntries {
		if !de.IsDir() || !strings.HasPrefix(de.Name(), "camunda-") {
			continue
		}
		app := strings.TrimPrefix(de.Name(), "camunda-")
		matrixFile := filepath.Join("version-matrix", de.Name(), "version-matrix.json")
		data, err := os.ReadFile(matrixFile)
		if err != nil {
			if os.IsNotExist(err) {
				continue // frozen minor without JSON
			}
			return fmt.Errorf("read %s: %w", matrixFile, err)
		}
		var entries []versionmatrix.ChartEntry
		if err := json.Unmarshal(data, &entries); err != nil {
			return fmt.Errorf("parse %s: %w", matrixFile, err)
		}

		readmePath := filepath.Join("version-matrix", de.Name(), "README.md")
		readmeCLI := parseReadmeHelmCLI(readmePath)
		readmeImages := parseReadmeImages(readmePath)
		reconcileImages := cfg.BucketOf(app) != versionmatrix.BucketAlpha

		changed := false
		for i := range entries {
			e := &entries[i]
			if e.ReleaseDate == "" || e.ReleaseTag == "" {
				tag, date := resolveGitHubRelease(app, e.ChartVersion)
				if tag == "" {
					msg := fmt.Sprintf("%s %s: no GitHub release found under either tag scheme", app, e.ChartVersion)
					if allowMissingRelease {
						fmt.Fprintf(os.Stderr, "warn: %s\n", msg)
					} else {
						failures = append(failures, msg)
					}
				} else {
					e.ReleaseTag = tag
					e.ReleaseDate = date
					changed = true
				}
			}
			if e.ReleaseTag != "" {
				if ann := helmCLIAnnotationAtRef(app, e.ReleaseTag, e.ChartVersion); ann != "" && ann != e.HelmCLI {
					if e.HelmCLI != "" {
						fmt.Fprintf(os.Stderr, "reconcile %s %s: helm_cli %q → %q (release-tag annotation wins)\n",
							app, e.ChartVersion, e.HelmCLI, ann)
					}
					e.HelmCLI = ann
					changed = true
				}
			}
			if e.HelmCLI == "" {
				if cli, ok := readmeCLI[e.ChartVersion]; ok {
					e.HelmCLI = cli
					changed = true
				} else if pin := helmPinAtRef(e.ReleaseTag); e.ReleaseTag != "" && pin != "" {
					e.HelmCLI = releasenotes.HelmCLIVersion(app, pin)
					changed = true
				} else {
					failures = append(failures, fmt.Sprintf("%s %s: no Helm CLI source (annotation, README section, or release-tag .tool-versions)", app, e.ChartVersion))
				}
			}
			if imgs, ok := readmeImages[e.ChartVersion]; ok && reconcileImages {
				if !equalStringSets(e.ChartImages, imgs.standard) {
					fmt.Fprintf(os.Stderr, "reconcile %s %s: chart_images %d → %d (README section wins)\n",
						app, e.ChartVersion, len(e.ChartImages), len(imgs.standard))
					e.ChartImages = imgs.standard
					changed = true
				}
				if !equalStringSets(e.ChartEnterpriseImages, imgs.enterprise) {
					fmt.Fprintf(os.Stderr, "reconcile %s %s: chart_enterprise_images %d → %d (README section wins)\n",
						app, e.ChartVersion, len(e.ChartEnterpriseImages), len(imgs.enterprise))
					e.ChartEnterpriseImages = imgs.enterprise
					changed = true
				}
			}
		}

		if !changed {
			continue
		}
		out, err := versionmatrix.EncodeEntries(entries)
		if err != nil {
			return err
		}
		if dryRun {
			fmt.Fprintf(os.Stderr, "[dry-run] would update %s\n", matrixFile)
			continue
		}
		if err := os.WriteFile(matrixFile, out, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", matrixFile, err)
		}
		fmt.Fprintf(os.Stderr, "updated %s (%d entries)\n", matrixFile, len(entries))
	}

	if len(failures) > 0 {
		sort.Strings(failures)
		return fmt.Errorf("backfill incomplete for %d entries:\n  - %s", len(failures), strings.Join(failures, "\n  - "))
	}
	return nil
}

// readmeHelmCLILinkedRe matches linked versions ("[3.18.6](…)") on a
// "- Helm CLI:" line; readmeHelmCLIBareRe is the fallback for unlinked lines.
var (
	readmeHelmCLILinkedRe = regexp.MustCompile(`\[v?(\d+\.\d+\.\d+)\]\(`)
	readmeHelmCLIBareRe   = regexp.MustCompile(`v?(\d+\.\d+\.\d+)`)
)

// helmCLIVersionsFromLine extracts the version list from a "- Helm CLI:" line,
// preferring link texts and falling back to bare version strings.
func helmCLIVersionsFromLine(line string) []string {
	var versions []string
	for _, m := range readmeHelmCLILinkedRe.FindAllStringSubmatch(line, -1) {
		versions = append(versions, m[1])
	}
	if len(versions) > 0 {
		return versions
	}
	for _, m := range readmeHelmCLIBareRe.FindAllStringSubmatch(line, -1) {
		versions = append(versions, m[1])
	}
	return versions
}

// parseReadmeHelmCLI extracts chartVersion → comma-joined Helm CLI versions
// from an existing per-minor README's "## Helm chart <v>" sections. Returns
// an empty map when the README does not exist.
func parseReadmeHelmCLI(path string) map[string]string {
	data, err := os.ReadFile(path)
	if err != nil {
		return map[string]string{}
	}
	result := map[string]string{}
	for version, section := range versionmatrix.ParseReadmeSections(string(data)) {
		for _, line := range strings.Split(section, "\n") {
			if !strings.HasPrefix(strings.TrimSpace(line), "- Helm CLI:") {
				continue
			}
			if versions := helmCLIVersionsFromLine(line); len(versions) > 0 {
				result[version] = strings.Join(versions, ",")
			}
			break
		}
	}
	return result
}

// sectionImages holds a README section's image lists: standard is the union
// of the "Camunda images:" and "Non-Camunda images:" blocks, enterprise the
// "Enterprise images" block. Both sorted.
type sectionImages struct {
	standard   []string
	enterprise []string
}

// readmeImageRe matches one "- <image-ref>" list line.
var readmeImageRe = regexp.MustCompile(`^- (\S+)$`)

// parseReadmeImages extracts each "## Helm chart <v>" section's image lists
// from an existing per-minor README. Returns an empty map when the README
// does not exist.
func parseReadmeImages(path string) map[string]sectionImages {
	data, err := os.ReadFile(path)
	if err != nil {
		return map[string]sectionImages{}
	}
	result := map[string]sectionImages{}
	for version, section := range versionmatrix.ParseReadmeSections(string(data)) {
		var si sectionImages
		block := ""
		for _, line := range strings.Split(section, "\n") {
			trimmed := strings.TrimSpace(line)
			switch {
			case strings.HasPrefix(trimmed, "Camunda images:"):
				block = "standard"
			case strings.HasPrefix(trimmed, "Non-Camunda images:"):
				block = "standard"
			case strings.HasPrefix(trimmed, "Enterprise images"):
				block = "enterprise"
			case trimmed == "":
				// blank lines separate list bodies from their headers; keep block
			default:
				m := readmeImageRe.FindStringSubmatch(trimmed)
				if m == nil {
					block = ""
					continue
				}
				switch block {
				case "standard":
					si.standard = append(si.standard, m[1])
				case "enterprise":
					si.enterprise = append(si.enterprise, m[1])
				}
			}
		}
		sort.Strings(si.standard)
		sort.Strings(si.enterprise)
		result[version] = si
	}
	return result
}

// equalStringSets compares two slices as sets (order-insensitive).
func equalStringSets(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	as := append([]string(nil), a...)
	bs := append([]string(nil), b...)
	sort.Strings(as)
	sort.Strings(bs)
	for i := range as {
		if as[i] != bs[i] {
			return false
		}
	}
	return true
}

// resolveGitHubRelease finds the published GitHub release for a chart version,
// trying the current camunda-platform-<minor>-<version> tag scheme first and
// the legacy camunda-platform-<version> scheme second. Returns the resolved
// tag and its publish date (YYYY-MM-DD), or empty strings when neither exists.
func resolveGitHubRelease(app, chartVersion string) (tag, date string) {
	for _, candidate := range []string{
		releaseplease.ReleaseTag(app, chartVersion),
		"camunda-platform-" + chartVersion,
	} {
		out, err := executil.RunCommandCapture(context.Background(), "gh",
			[]string{"release", "view", candidate, "--json", "publishedAt", "--jq", ".publishedAt"}, nil, "")
		if err != nil {
			continue
		}
		published := strings.TrimSpace(string(out))
		if len(published) >= 10 {
			return candidate, published[:10]
		}
	}
	return "", ""
}
