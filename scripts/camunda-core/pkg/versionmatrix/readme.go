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
	"slices"
	"strings"
)

// indexHeader is the fixed overview prose for version-matrix/README.md.
const indexHeader = "" +
	"<!-- THIS FILE IS AUTO-GENERATED, DO NOT EDIT IT MANUALLY! -->\n" +
	"# Camunda 8 Helm Chart Version Matrix\n\n" +
	"## Overview\n\n" +
	"Camunda 8 Self-Managed is deployed via Helm charts.\n\n" +
	"For the best experience, please remember:\n\n" +
	"- The Camunda `application version` is different from the Helm `chart version`. The Camunda application version is presented by `appVersion` in the chart. The Camunda Helm chart version is presented by `version` in the chart.\n\n" +
	"- You can view application versions and chart versions via Helm CLI.\n\n" +
	"  ```helm search repo camunda/camunda-platform --versions```\n\n" +
	"- Always use the supported `Helm CLI` versions used with the Helm chart. They're mentioned in the matrix for all charts or under chart annotation `camunda.io/helmCLIVersion` for newer charts.\n\n" +
	"- Camunda 8.9 (chart 14.x) is the last minor that supports Helm v3. Camunda 8.10 (chart 15.x) and later require Helm v4.\n\n" +
	"- During the upgrade from the non-patch versions, ensure to review [version update instructions](https://docs.camunda.io/docs/self-managed/deployment/helm/upgrade/).\n\n\n"

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
// and including the back-link. The title and ToC follow.
const readmeHeader = "<!-- THIS FILE IS AUTO-GENERATED, DO NOT EDIT IT MANUALLY! -->\n" +
	"🔙 [Back to version matrix index](../)\n\n"

// SpliceReadme inserts or replaces entry's section in an existing per-app
// version-matrix README, leaving every other version's section byte-for-byte
// intact. existing may be empty (yields a single-section README). The ToC is
// regenerated from the resulting section set; sections are emitted newest-first.
func SpliceReadme(existing, app string, entry ChartEntry, helmCLIVersions []string) string {
	sections := parseReadmeSections(existing)
	sections[entry.ChartVersion] = strings.TrimRight(
		ReleaseSection(app, entry, helmCLIVersions, true), "\n")

	versions := make([]string, 0, len(sections))
	for v := range sections {
		versions = append(versions, v)
	}
	slices.SortFunc(versions, func(a, b string) int {
		return -CompareChartVersionsFull(a, b)
	})

	return assembleReadme(app, versions, sections)
}

// parseReadmeSections splits a per-app README body into version → section text.
// Each section starts with its "## Helm chart <v>" heading; trailing blank
// lines are trimmed. The fixed header and ToC are discarded — assembleReadme
// regenerates them. An empty/headerless input yields an empty map.
func parseReadmeSections(readme string) map[string]string {
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

// assembleReadme renders the full per-app README from prebuilt section bodies.
// versions must be sorted newest-first; sectionByVersion[v] is the full
// "## Helm chart <v>..." block. Spacing: one blank line between the ToC and the
// first section, two blank lines between sections, single trailing newline.
func assembleReadme(app string, versions []string, sectionByVersion map[string]string) string {
	var b strings.Builder
	b.WriteString(readmeHeader)
	fmt.Fprintf(&b, "# Camunda %s Helm Chart Version Matrix\n\n", app)
	b.WriteString("## ToC\n\n")
	for _, v := range versions {
		fmt.Fprintf(&b, "- [Helm chart %s](#helm-chart-%s)\n", v, readmeAnchor(v))
	}
	for i, v := range versions {
		if i == 0 {
			b.WriteString("\n")
		} else {
			b.WriteString("\n\n")
		}
		b.WriteString(strings.TrimRight(sectionByVersion[v], "\n"))
		b.WriteString("\n")
	}
	return b.String()
}

// RenderIndex renders the top-level version-matrix/README.md. appVersions must
// be sorted newest-first; entriesByApp[app] must also be sorted newest-first.
func RenderIndex(appVersions []string, entriesByApp map[string][]ChartEntry) string {
	var b strings.Builder
	b.WriteString(indexHeader)
	for i, app := range appVersions {
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "## [Camunda %s](./camunda-%s)\n\n", app, app)
		for _, e := range entriesByApp[app] {
			fmt.Fprintf(&b, "### [Helm chart %s](./camunda-%s/#helm-chart-%s)\n",
				e.ChartVersion, app, readmeAnchor(e.ChartVersion))
		}
	}
	return b.String()
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

const enterpriseDocsURL = "https://docs.camunda.io/docs/self-managed/setup/guides/install-bitnami-enterprise-images/"

// ReleaseSection renders one chart version's block of a version-matrix README.
// appVersion is the Camunda minor (e.g.
// "8.7"), entry holds the image sets, helmCLIVersions the supported Helm CLI
// versions for that chart. withHeader adds the "## Helm chart <v>" heading (the
// per-version README uses it; the RELEASE-NOTES footer does not).
func ReleaseSection(appVersion string, entry ChartEntry, helmCLIVersions []string, withHeader bool) string {
	camunda, nonCamunda := splitCamundaImages(entry.ChartImages)

	cli := "N/A"
	if len(helmCLIVersions) > 0 {
		links := make([]string, len(helmCLIVersions))
		for i, v := range helmCLIVersions {
			links[i] = fmt.Sprintf("[%s](https://github.com/helm/helm/releases/tag/v%s)", v, v)
		}
		cli = strings.Join(links, ", ")
	}

	var b strings.Builder
	if withHeader {
		fmt.Fprintf(&b, "## Helm chart %s\n\n", entry.ChartVersion)
	}
	b.WriteString("Supported versions:\n\n")
	fmt.Fprintf(&b, "- Camunda applications: [%s](https://github.com/camunda/camunda/releases?q=tag%%3A%s&expanded=true)\n", appVersion, appVersion)
	fmt.Fprintf(&b, "- Camunda version matrix: [%s](https://helm.camunda.io/camunda-platform/version-matrix/camunda-%s)\n", appVersion, appVersion)
	fmt.Fprintf(&b, "- Helm values: [%s](https://artifacthub.io/packages/helm/camunda/camunda-platform/%s#parameters)\n", entry.ChartVersion, entry.ChartVersion)
	fmt.Fprintf(&b, "- Helm CLI: %s\n", cli)
	fmt.Fprintf(&b, "\nCamunda images:\n\n%s\n", imageList(camunda))
	// Non-Camunda and Enterprise blocks are omitted when their image list is
	// empty (e.g. orchestration charts that bundle no subcharts).
	if len(nonCamunda) > 0 {
		fmt.Fprintf(&b, "\nNon-Camunda images:\n\n%s\n", imageList(nonCamunda))
	}
	if len(entry.ChartEnterpriseImages) > 0 {
		fmt.Fprintf(&b, "\nEnterprise images ([Camunda Enterprise](%s)):\n\n%s\n", enterpriseDocsURL, imageList(entry.ChartEnterpriseImages))
	}
	return b.String()
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
