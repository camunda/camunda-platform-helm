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

package versionmatrix

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// indexTableLimit caps how many chart versions each active minor shows on the
// index page; the per-minor page carries the full history.
const indexTableLimit = 5

// indexHeader is the fixed lead of version-matrix/README.md: title plus one
// orientation line — the tables follow immediately (details live in
// indexNotes at the bottom of the page).
const indexHeader = "" +
	"<!-- THIS FILE IS AUTO-GENERATED, DO NOT EDIT IT MANUALLY! -->\n" +
	"# Camunda 8 Helm Chart Version Matrix\n\n" +
	"Use this page to find which Helm chart version deploys which Camunda 8 release, when it was released, and which Helm CLI it supports. See the [notes](#notes) below for how to read the tables.\n"

// indexNotes is the reference section rendered after the tables.
const indexNotes = "" +
	"\n## Notes\n\n" +
	"- The `Camunda` column is the chart's core application version — find your exact Camunda patch (for example, 8.8.5) there. Pre-release charts carry an `-alpha`/`-rc` suffix in the chart version: previews, not for production use and without a support SLA.\n" +
	"- The Camunda `application version` (`appVersion` in the chart) is different from the Helm `chart version` (`version` in the chart). List both from the live Helm repository (without `--devel`, Helm hides the pre-release charts listed on this page):\n\n" +
	"  ```\n" +
	"  helm repo add camunda https://helm.camunda.io\n" +
	"  helm repo update\n" +
	"  helm search repo camunda/camunda-platform --versions --devel\n" +
	"  ```\n\n" +
	"- Standard support for a Camunda minor lasts 18 months from its release; fixes ship in the newest chart of each supported minor, so stay current within your minor. Extended support is available under contract — contact your Customer Success Manager (CSM).\n" +
	"- The `Helm CLI` column lists the Helm CLI version(s) each chart was released and tested with (recorded at release in the chart annotation `camunda.io/helmCLIVersion`). Camunda 8.9 (chart 14.x) is the last minor that supports Helm v3; Camunda 8.10 (chart 15.x) and later require Helm v4. Older CLI versions may lack template functions the chart uses (for example, `toYamlPretty` requires 3.17+).\n" +
	"- When upgrading across minor versions, go one minor at a time (do not skip minors) using the newest chart of each hop, and review the [upgrade instructions](https://docs.camunda.io/docs/self-managed/upgrade/) first. For a rollback option, take a [backup](https://docs.camunda.io/docs/self-managed/operational-guides/backup-restore/backup-and-restore/) before each hop.\n" +
	"- This page is generated automatically when a chart release is promoted — do not edit it manually.\n"

// chartTableHeader is the column set shared by the index and per-minor tables.
const chartTableHeader = "" +
	"| Helm Chart | Camunda | Released | Helm CLI | Helm Values | Release Notes |\n" +
	"|---|---|---|---|---|---|\n"

// coreCamundaVersion derives the chart's core Camunda application version
// from its image set (the camunda/camunda tag on 8.8+, the zeebe tag on older
// minors) — the direct answer to "Camunda 8.x.y → which chart?". Returns ""
// when no core image is present.
func coreCamundaVersion(e ChartEntry) string {
	for _, prefix := range []string{
		"docker.io/camunda/camunda:",
		"docker.io/camunda/zeebe:",
	} {
		for _, ref := range e.ChartImages {
			if tag, ok := strings.CutPrefix(ref, prefix); ok && tag != "" {
				return tag
			}
		}
	}
	return ""
}

// SortEntriesDescending returns a copy of entries sorted newest-first.
func SortEntriesDescending(entries []ChartEntry) []ChartEntry {
	sorted := make([]ChartEntry, len(entries))
	copy(sorted, entries)
	slices.SortFunc(sorted, func(a, b ChartEntry) int {
		return -CompareChartVersionsFull(a.ChartVersion, b.ChartVersion)
	})
	return sorted
}

// readmeHeader is the fixed preamble of a per-app version-matrix README, up to
// and including the back-link. The title and summary table follow.
const readmeHeader = "<!-- THIS FILE IS AUTO-GENERATED, DO NOT EDIT IT MANUALLY! -->\n" +
	"[Back to version matrix index](../)\n\n"

// SplitHelmCLI splits the comma-separated helm_cli field into its versions,
// trimming whitespace.
func SplitHelmCLI(csv string) []string {
	var result []string
	for _, v := range strings.Split(csv, ",") {
		if v = strings.TrimSpace(v); v != "" {
			result = append(result, v)
		}
	}
	return result
}

// helmCLILinks renders the linked Helm CLI cell/list value ("N/A" when empty).
func helmCLILinks(versions []string) string {
	if len(versions) == 0 {
		return "N/A"
	}
	links := make([]string, len(versions))
	for i, v := range versions {
		links[i] = fmt.Sprintf("[%s](https://github.com/helm/helm/releases/tag/v%s)", v, v)
	}
	return strings.Join(links, ", ")
}

// artifactHubURL returns the chart version's ArtifactHub parameters page.
func artifactHubURL(chartVersion string) string {
	return "https://artifacthub.io/packages/helm/camunda/camunda-platform/" + chartVersion + "#parameters"
}

// releaseURL returns the GitHub release page for a release tag.
func releaseURL(releaseTag string) string {
	return "https://github.com/camunda/camunda-platform-helm/releases/tag/" + releaseTag
}

// chartTableRow renders one chart version's row. linkPrefix prefixes the
// in-page anchor (empty on the per-minor page, "./camunda-<app>/" on the
// index).
func chartTableRow(linkPrefix string, e ChartEntry) string {
	released := e.ReleaseDate
	if released == "" {
		released = "_pending_"
	}
	camunda := coreCamundaVersion(e)
	if camunda == "" {
		camunda = "—"
	}
	notes := "—"
	if e.ReleaseTag != "" {
		notes = fmt.Sprintf("[Changelog](%s)", releaseURL(e.ReleaseTag))
	}
	return fmt.Sprintf("| [%s](%s#helm-chart-%s) | %s | %s | %s | [ArtifactHub](%s) | %s |\n",
		e.ChartVersion, linkPrefix, readmeAnchor(e.ChartVersion),
		camunda,
		released,
		helmCLILinks(SplitHelmCLI(e.HelmCLI)),
		artifactHubURL(e.ChartVersion),
		notes)
}

// chartTable renders the shared six-column table for entries (already sorted
// newest-first), capped at limit rows (0 = no cap).
func chartTable(linkPrefix string, entries []ChartEntry, limit int) string {
	var b strings.Builder
	b.WriteString(chartTableHeader)
	for i, e := range entries {
		if limit > 0 && i >= limit {
			break
		}
		b.WriteString(chartTableRow(linkPrefix, e))
	}
	return b.String()
}

// minorHeading renders the index section heading for an active minor.
func minorHeading(app, bucket string, lc Lifecycle) string {
	switch bucket {
	case BucketAlpha:
		return fmt.Sprintf("## Camunda %s — Alpha", app)
	default:
		return fmt.Sprintf("## Camunda %s — Standard support until %s", app, lc.StdSupportUntil)
	}
}

// minorStatusLine renders the lifecycle status line under a per-minor page's
// title.
func minorStatusLine(bucket string, lc Lifecycle) string {
	switch bucket {
	case BucketAlpha:
		return "Alpha"
	case BucketSupportStandard:
		return fmt.Sprintf("Standard support until %s", lc.StdSupportUntil)
	case BucketSupportExtended:
		return "Extended support — contact your CSM"
	case BucketEndOfLife:
		return fmt.Sprintf("End of life since %s", lc.EOLSince)
	}
	return ""
}

// latestChartCell resolves the "Latest chart" cell for compact rows: the
// newest entry when the minor has version-matrix data, else the frozen
// latestChart recorded in the lifecycle config, else "—". Linked to the
// per-minor anchor when the version is known.
func latestChartCell(app string, entries []ChartEntry, lc Lifecycle) string {
	version := lc.LatestChart
	if len(entries) > 0 {
		if v, err := LatestVersion(entries); err == nil {
			version = v
		}
	}
	if version == "" {
		return "—"
	}
	return fmt.Sprintf("[%s](./camunda-%s/#helm-chart-%s)", version, app, readmeAnchor(version))
}

// RenderIndex renders the top-level version-matrix/README.md: one section per
// active minor (alpha + supportStandard, newest first) with its
// indexTableLimit newest chart versions and a link to the full per-minor
// matrix, then one compact table each for extended-support and end-of-life
// minors. entriesByApp values must be sorted newest-first.
func RenderIndex(cfg *ChartVersionsConfig, entriesByApp map[string][]ChartEntry) (string, error) {
	var b strings.Builder
	b.WriteString(indexHeader)

	active := append(append([]string{}, cfg.CamundaVersions.Alpha...), cfg.CamundaVersions.SupportStandard...)
	for _, app := range SortAppVersionsDescending(active) {
		lc := cfg.CamundaSupportLifecycle[app]
		entries := entriesByApp[app]
		fmt.Fprintf(&b, "\n%s\n\n", minorHeading(app, cfg.BucketOf(app), lc))
		if lc.Note != "" {
			fmt.Fprintf(&b, "> %s\n\n", lc.Note)
		}
		// A minor can be classified before its first chart is promoted (the
		// documented minor-rollover chores) — render a placeholder rather
		// than failing every other minor's release.
		if len(entries) == 0 {
			fmt.Fprintf(&b, "_No chart releases for Camunda %s yet._\n", app)
			continue
		}
		b.WriteString(chartTable("./camunda-"+app+"/", entries, indexTableLimit))
		fmt.Fprintf(&b, "\n[All %d chart versions for Camunda %s →](./camunda-%s/)\n", len(entries), app, app)
	}

	if minors := cfg.CamundaVersions.SupportExtended; len(minors) > 0 {
		b.WriteString("\n## Extended support — contact your CSM\n\n")
		b.WriteString("| Camunda | Released | Latest chart | Full matrix |\n|---|---|---|---|\n")
		for _, app := range SortAppVersionsDescending(minors) {
			lc := cfg.CamundaSupportLifecycle[app]
			fmt.Fprintf(&b, "| %s | %s | %s | [camunda-%s](./camunda-%s/) |\n",
				app, lc.Released, latestChartCell(app, entriesByApp[app], lc), app, app)
		}
	}

	if minors := cfg.CamundaVersions.EndOfLife; len(minors) > 0 {
		b.WriteString("\n## End of life — no longer supported\n\n")
		b.WriteString("| Camunda | EOL since | Last chart | Full matrix |\n|---|---|---|---|\n")
		for _, app := range SortAppVersionsDescending(minors) {
			lc := cfg.CamundaSupportLifecycle[app]
			fmt.Fprintf(&b, "| %s | %s | %s | [camunda-%s](./camunda-%s/) |\n",
				app, lc.EOLSince, latestChartCell(app, entriesByApp[app], lc), app, app)
		}
	}

	b.WriteString(indexNotes)
	return b.String(), nil
}

// RenderMinorReadme renders a full per-minor version-matrix README from its
// entries: back-link, title, lifecycle status line, summary table over every
// chart version, then the per-version sections — newest first, anchors
// preserved as "## Helm chart <v>" headings.
//
// existingSections maps chart version → its current "## Helm chart <v>…"
// section body. Sections of PUBLISHED entries (release_date stamped) in
// non-alpha minors are preserved byte-for-byte — the published section is the
// historical truth of what each release shipped, and version-matrix.json
// image sets have drifted from it in the past (the SUPPORT-33569 class).
// Everything else renders from the JSON entry: versions without a section,
// unstamped entries (an in-flight promotion may re-derive its images), and
// all alpha-bucket minors (their JSON is written by the current promotion
// pipeline and is fresher than any earlier splice-era section).
func RenderMinorReadme(app string, entries []ChartEntry, bucket string, lc Lifecycle, existingSections map[string]string) string {
	sorted := SortEntriesDescending(entries)

	var b strings.Builder
	b.WriteString(readmeHeader)
	fmt.Fprintf(&b, "# Camunda %s Helm Chart Version Matrix\n\n", app)
	if status := minorStatusLine(bucket, lc); status != "" {
		fmt.Fprintf(&b, "%s\n\n", status)
	}
	if lc.Note != "" {
		fmt.Fprintf(&b, "> %s\n\n", lc.Note)
	}
	b.WriteString(chartTable("", sorted, 0))
	b.WriteString("\n" + enterpriseImagesNote + "\n")
	b.WriteString("\n---\n")
	for _, e := range sorted {
		section, ok := existingSections[e.ChartVersion]
		if !ok || bucket == BucketAlpha || e.ReleaseDate == "" {
			section = ReleaseSection(app, e, SplitHelmCLI(e.HelmCLI), true)
		} else {
			section = syncHelmCLILine(section, e)
		}
		b.WriteString("\n")
		b.WriteString(strings.TrimRight(section, "\n"))
		b.WriteString("\n")
	}
	return b.String()
}

// helmCLILineRe matches a section's "- Helm CLI: …" bullet.
var helmCLILineRe = regexp.MustCompile(`(?m)^- Helm CLI: .*$`)

// syncHelmCLILine rewrites a preserved section's "- Helm CLI:" bullet to the
// entry's recorded helm_cli. The bullet is generator-owned metadata with an
// authoritative source (the chart annotation at the release tag) — unlike the
// image lists, which stay byte-for-byte as published.
func syncHelmCLILine(section string, e ChartEntry) string {
	if e.HelmCLI == "" {
		return section
	}
	return helmCLILineRe.ReplaceAllString(section,
		"- Helm CLI: "+helmCLILinks(SplitHelmCLI(e.HelmCLI)))
}

// enterpriseImagesNote explains the relationship between the Non-Camunda and
// Enterprise image lists on the per-minor pages (they are alternatives, not
// additive).
const enterpriseImagesNote = "_Enterprise images replace the matching Non-Camunda (Bitnami OSS) images when using Camunda Enterprise registry access — mirror the set that matches your configuration._\n"

// ParseReadmeSections splits a per-app README body into version → section
// text. Each section starts with its "## Helm chart <v>" heading; trailing
// blank lines are trimmed. Everything before the first section (header,
// summary table) is discarded — RenderMinorReadme regenerates it. An
// empty/headerless input yields an empty map.
func ParseReadmeSections(readme string) map[string]string {
	const marker = "## Helm chart "
	sections := make(map[string]string)
	idx := strings.Index(readme, marker)
	if idx < 0 {
		return sections
	}
	for _, chunk := range strings.Split(readme[idx:], "\n"+marker) {
		chunk = strings.TrimPrefix(chunk, marker)
		nl := strings.IndexByte(chunk, '\n')
		version, rest := chunk, ""
		if nl >= 0 {
			version, rest = chunk[:nl], chunk[nl:]
		}
		if version = strings.TrimSpace(version); version == "" {
			continue
		}
		sections[version] = strings.TrimRight(marker+version+rest, "\n \t")
	}
	return sections
}

// SortAppVersionsDescending returns a copy of appVersions sorted newest-first.
// Handles both "8.10" and "1.3" style versions.
func SortAppVersionsDescending(versions []string) []string {
	sorted := make([]string, len(versions))
	copy(sorted, versions)
	slices.SortFunc(sorted, func(a, b string) int {
		return -CompareChartVersions(a, b)
	})
	return sorted
}

// enterpriseDocsURL returns the version-pinned Bitnami enterprise-images guide
// URL. The docs path for this guide differs between 8.7 and later minors.
func enterpriseDocsURL(appVersion string) string {
	path := "self-managed/deployment/helm/configure/registry-and-images/install-bitnami-enterprise-images"
	if appVersion == "8.7" {
		path = "self-managed/setup/guides/install-bitnami-enterprise-images"
	}
	return fmt.Sprintf("https://docs.camunda.io/docs/%s/%s/", appVersion, path)
}

// ReleaseSection renders one chart version's block of a version-matrix README.
// appVersion is the Camunda minor (e.g.
// "8.7"), entry holds the image sets, helmCLIVersions the supported Helm CLI
// versions for that chart. withHeader adds the "## Helm chart <v>" heading (the
// per-version README uses it; the RELEASE-NOTES footer does not).
func ReleaseSection(appVersion string, entry ChartEntry, helmCLIVersions []string, withHeader bool) string {
	camunda, nonCamunda := splitCamundaImages(validImageRefs(entry.ChartImages))

	var b strings.Builder
	if withHeader {
		fmt.Fprintf(&b, "## Helm chart %s\n\n", entry.ChartVersion)
	}
	b.WriteString("Supported versions:\n\n")
	fmt.Fprintf(&b, "- Camunda applications: [%s](https://github.com/camunda/camunda/releases?q=tag%%3A%s&expanded=true)\n", appVersion, appVersion)
	fmt.Fprintf(&b, "- Camunda version matrix: [%s](https://helm.camunda.io/camunda-platform/version-matrix/camunda-%s)\n", appVersion, appVersion)
	fmt.Fprintf(&b, "- Helm values: [%s](%s)\n", entry.ChartVersion, artifactHubURL(entry.ChartVersion))
	fmt.Fprintf(&b, "- Helm CLI: %s\n", helmCLILinks(helmCLIVersions))
	fmt.Fprintf(&b, "\nCamunda images:\n\n%s\n", imageList(camunda))
	// Non-Camunda and Enterprise blocks are omitted when their image list is
	// empty (e.g. orchestration charts that bundle no subcharts).
	if len(nonCamunda) > 0 {
		fmt.Fprintf(&b, "\nNon-Camunda images:\n\n%s\n", imageList(nonCamunda))
	}
	if enterprise := validImageRefs(entry.ChartEnterpriseImages); len(enterprise) > 0 {
		fmt.Fprintf(&b, "\nEnterprise images ([Camunda Enterprise](%s)):\n\n%s\n", enterpriseDocsURL(appVersion), imageList(enterprise))
	}
	return b.String()
}

// validImageRefs drops image references with an empty tag (a trailing colon,
// e.g. "docker.io/camunda/hub:") — publishing them invites accidental
// :latest pulls; the JSON keeps the raw artifact-derived data.
func validImageRefs(refs []string) []string {
	valid := make([]string, 0, len(refs))
	for _, ref := range refs {
		if strings.HasSuffix(ref, ":") {
			continue
		}
		valid = append(valid, ref)
	}
	return valid
}

// splitCamundaImages partitions chart_images into Camunda (ref contains
// "camunda") and Non-Camunda, matching the `grep camunda` / `grep -v camunda`
// split the README uses.
func splitCamundaImages(images []string) (camunda, nonCamunda []string) {
	for _, ref := range images {
		if strings.Contains(ref, "camunda") {
			camunda = append(camunda, ref)
		} else {
			nonCamunda = append(nonCamunda, ref)
		}
	}
	return camunda, nonCamunda
}

func imageList(images []string) string {
	lines := make([]string, len(images))
	for i, ref := range images {
		lines[i] = "- " + ref
	}
	return strings.Join(lines, "\n")
}

// readmeAnchor turns a chart version into its markdown heading anchor (drop dots),
// matching the template's `strings.ReplaceAll "." ""`.
func readmeAnchor(chartVersion string) string {
	return strings.ReplaceAll(chartVersion, ".", "")
}
