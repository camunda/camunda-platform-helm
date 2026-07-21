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
	"strings"
	"testing"
)

func TestReleaseSectionWithHeader(t *testing.T) {
	entry := ChartEntry{
		ChartVersion: "12.11.0",
		ChartImages: []string{
			"docker.io/camunda/zeebe:8.7.30",
			"docker.io/bitnamilegacy/elasticsearch:8.17.4",
			"registry.camunda.cloud/camunda/keycloak:26.3.3",
		},
		ChartEnterpriseImages: []string{
			"registry.camunda.cloud/keycloak-ee/keycloak:26.4.0",
		},
	}
	got := ReleaseSection("8.7", entry, []string{"3.20.2"}, true)
	want := `## Helm chart 12.11.0

Supported versions:

- Camunda applications: [8.7](https://github.com/camunda/camunda/releases?q=tag%3A8.7&expanded=true)
- Camunda version matrix: [8.7](https://helm.camunda.io/camunda-platform/version-matrix/camunda-8.7)
- Helm values: [12.11.0](https://artifacthub.io/packages/helm/camunda/camunda-platform/12.11.0#parameters)
- Helm CLI: [3.20.2](https://github.com/helm/helm/releases/tag/v3.20.2)

Camunda images:

- docker.io/camunda/zeebe:8.7.30
- registry.camunda.cloud/camunda/keycloak:26.3.3

Non-Camunda images:

- docker.io/bitnamilegacy/elasticsearch:8.17.4

Enterprise images ([Camunda Enterprise](https://docs.camunda.io/docs/8.7/self-managed/setup/guides/install-bitnami-enterprise-images/)):

- registry.camunda.cloud/keycloak-ee/keycloak:26.4.0
`
	if got != want {
		t.Errorf("ReleaseSection:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestReleaseSectionNoEnterpriseNoHeader(t *testing.T) {
	entry := ChartEntry{
		ChartVersion: "12.11.0",
		ChartImages:  []string{"docker.io/camunda/zeebe:8.7.30"},
	}
	got := ReleaseSection("8.7", entry, []string{"3.20.2", "4.1.4"}, false)
	// No "## Helm chart" header; multiple Helm CLI versions comma-joined; no
	// Non-Camunda block (empty) and no Enterprise block (both omitted when empty).
	want := `Supported versions:

- Camunda applications: [8.7](https://github.com/camunda/camunda/releases?q=tag%3A8.7&expanded=true)
- Camunda version matrix: [8.7](https://helm.camunda.io/camunda-platform/version-matrix/camunda-8.7)
- Helm values: [12.11.0](https://artifacthub.io/packages/helm/camunda/camunda-platform/12.11.0#parameters)
- Helm CLI: [3.20.2](https://github.com/helm/helm/releases/tag/v3.20.2), [4.1.4](https://github.com/helm/helm/releases/tag/v4.1.4)

Camunda images:

- docker.io/camunda/zeebe:8.7.30
`
	if got != want {
		t.Errorf("ReleaseSection no-enterprise:\n--- got ---\n%q\n--- want ---\n%q", got, want)
	}
}

func TestReleaseSectionOmitsEmptyNonCamunda(t *testing.T) {
	// An orchestration chart with only Camunda images must NOT emit a dangling
	// "Non-Camunda images:" header.
	entry := ChartEntry{
		ChartVersion: "15.0.0-alpha2",
		ChartImages:  []string{"docker.io/camunda/camunda:8.10.0-alpha1"},
	}
	got := ReleaseSection("8.10", entry, []string{"4.1.4"}, true)
	if strings.Contains(got, "Non-Camunda images:") {
		t.Errorf("ReleaseSection emitted empty Non-Camunda header:\n%q", got)
	}
	if !strings.HasSuffix(got, "- docker.io/camunda/camunda:8.10.0-alpha1\n") {
		t.Errorf("ReleaseSection should end with the Camunda image list:\n%q", got)
	}
}

func TestReadmeAnchor(t *testing.T) {
	if got := readmeAnchor("12.0.0-alpha5"); got != "1200-alpha5" {
		t.Errorf("readmeAnchor=%q want 1200-alpha5", got)
	}
}

func TestStabilityLabel(t *testing.T) {
	cases := map[string]string{
		"14.4.0":         "Stable",
		"15.0.0-alpha2":  "Alpha",
		"15.0.0-alpha10": "Alpha",
		"14.0.0-rc1":     "RC",
		"14.0.0-beta1":   "Pre-release",
	}
	for v, want := range cases {
		if got := StabilityLabel(v); got != want {
			t.Errorf("StabilityLabel(%q)=%q want %q", v, got, want)
		}
	}
}

func TestSplitHelmCLI(t *testing.T) {
	got := SplitHelmCLI(" 3.20.2 , 4.1.4 ")
	if len(got) != 2 || got[0] != "3.20.2" || got[1] != "4.1.4" {
		t.Errorf("SplitHelmCLI=%v", got)
	}
	if got := SplitHelmCLI(""); len(got) != 0 {
		t.Errorf("SplitHelmCLI(empty)=%v want empty", got)
	}
}

func TestSortEntriesDescending(t *testing.T) {
	entries := []ChartEntry{
		{ChartVersion: "12.9.0"},
		{ChartVersion: "12.0.0-alpha5"},
		{ChartVersion: "12.11.0"},
		{ChartVersion: "12.10.0"},
		{ChartVersion: "12.0.0"},
	}
	got := SortEntriesDescending(entries)
	want := []string{"12.11.0", "12.10.0", "12.9.0", "12.0.0", "12.0.0-alpha5"}
	for i, e := range got {
		if e.ChartVersion != want[i] {
			t.Errorf("SortEntriesDescending[%d]=%q want %q", i, e.ChartVersion, want[i])
		}
	}
}

func TestSortAppVersionsDescending(t *testing.T) {
	got := SortAppVersionsDescending([]string{"8.7", "8.10", "1.3", "8.9"})
	want := []string{"8.10", "8.9", "8.7", "1.3"}
	for i, v := range got {
		if v != want[i] {
			t.Errorf("SortAppVersionsDescending[%d]=%q want %q", i, v, want[i])
		}
	}
}

func TestChartTableRow(t *testing.T) {
	e := ChartEntry{
		ChartVersion: "14.4.0",
		ChartImages:  []string{"docker.io/camunda/camunda:8.9.7", "docker.io/bitnamilegacy/elasticsearch:8.18.0"},
		ReleaseDate:  "2026-06-04",
		HelmCLI:      "3.20.2,4.1.4",
		ReleaseTag:   "camunda-platform-8.9-14.4.0",
	}
	got := chartTableRow("./camunda-8.9/", e)
	want := "| [14.4.0](./camunda-8.9/#helm-chart-1440) | 8.9.7 | 2026-06-04 | Stable | " +
		"[3.20.2](https://github.com/helm/helm/releases/tag/v3.20.2), [4.1.4](https://github.com/helm/helm/releases/tag/v4.1.4) | " +
		"[ArtifactHub](https://artifacthub.io/packages/helm/camunda/camunda-platform/14.4.0#parameters) | " +
		"[Changelog](https://github.com/camunda/camunda-platform-helm/releases/tag/camunda-platform-8.9-14.4.0) |\n"
	if got != want {
		t.Errorf("chartTableRow:\n--- got ---\n%s--- want ---\n%s", got, want)
	}
}

func TestCoreCamundaVersion(t *testing.T) {
	cases := []struct {
		imgs []string
		want string
	}{
		{[]string{"docker.io/camunda/camunda:8.9.12", "docker.io/camunda/zeebe:9.9.9"}, "8.9.12"},
		{[]string{"docker.io/camunda/operate:8.7.30", "docker.io/camunda/zeebe:8.7.30"}, "8.7.30"},
		{[]string{"docker.io/bitnami/elasticsearch:8.12.2"}, ""},
		{nil, ""},
	}
	for _, tc := range cases {
		if got := coreCamundaVersion(ChartEntry{ChartImages: tc.imgs}); got != tc.want {
			t.Errorf("coreCamundaVersion(%v)=%q want %q", tc.imgs, got, tc.want)
		}
	}
}

func TestChartTableRowMissingFacts(t *testing.T) {
	got := chartTableRow("", ChartEntry{ChartVersion: "15.0.0-alpha3"})
	if !strings.Contains(got, "| _pending_ |") {
		t.Errorf("chartTableRow: missing release date should render _pending_: %q", got)
	}
	if !strings.Contains(got, "| Alpha |") {
		t.Errorf("chartTableRow: pre-release should render Alpha: %q", got)
	}
	if !strings.Contains(got, "| N/A |") {
		t.Errorf("chartTableRow: missing helm_cli should render N/A: %q", got)
	}
	if !strings.Contains(got, "| — |") {
		t.Errorf("chartTableRow: missing release_tag should render —: %q", got)
	}
	if !strings.Contains(got, "[15.0.0-alpha3](#helm-chart-1500-alpha3)") {
		t.Errorf("chartTableRow: empty prefix should keep in-page anchor: %q", got)
	}
}

// testConfig builds a ChartVersionsConfig covering all four buckets.
func testConfig() *ChartVersionsConfig {
	return &ChartVersionsConfig{
		CamundaVersions: Buckets{
			Alpha:           []string{"8.10"},
			SupportStandard: []string{"8.9", "8.8"},
			SupportExtended: []string{"8.6"},
			EndOfLife:       []string{"8.2"},
		},
		CamundaSupportLifecycle: map[string]Lifecycle{
			"8.10": {Note: "Deploys Camunda Hub — see the [Hub documentation](https://example.invalid/hub)."},
			"8.9":  {Released: "2026-04-14", StdSupportUntil: "2027-10-13"},
			"8.8":  {Released: "2025-10-14", StdSupportUntil: "2027-04-13"},
			"8.6":  {Released: "2024-10-08"},
			"8.2":  {Released: "2022-10-11", EOLSince: "2024-10-08", LatestChart: "8.2.34"},
		},
	}
}

func TestRenderIndex(t *testing.T) {
	entriesByApp := map[string][]ChartEntry{
		"8.10": {{ChartVersion: "15.0.0-alpha3", ReleaseDate: "2026-07-09", HelmCLI: "4.2.3", ReleaseTag: "camunda-platform-8.10-15.0.0-alpha3"}},
		"8.9": {
			{ChartVersion: "14.7.0", ReleaseDate: "2026-07-17", HelmCLI: "3.20.2,4.2.3", ReleaseTag: "camunda-platform-8.9-14.7.0"},
			{ChartVersion: "14.6.1", ReleaseDate: "2026-07-08", HelmCLI: "3.20.2,4.2.3", ReleaseTag: "camunda-platform-8.9-14.6.1"},
			{ChartVersion: "14.6.0"},
			{ChartVersion: "14.5.0"},
			{ChartVersion: "14.4.1"},
			{ChartVersion: "14.4.0"},
			{ChartVersion: "14.3.0"},
		},
		"8.8": {{ChartVersion: "13.12.2", ReleaseDate: "2026-07-08", HelmCLI: "3.20.2", ReleaseTag: "camunda-platform-8.8-13.12.2"}},
		"8.6": {{ChartVersion: "11.12.3", ReleaseDate: "2026-04-03", HelmCLI: "3.20.1"}},
	}
	got, err := RenderIndex(testConfig(), entriesByApp)
	if err != nil {
		t.Fatalf("RenderIndex: %v", err)
	}

	checks := []string{
		"<!-- THIS FILE IS AUTO-GENERATED, DO NOT EDIT IT MANUALLY! -->",
		"# Camunda 8 Helm Chart Version Matrix",
		"helm search repo camunda/camunda-platform --versions",
		// Active minors: lifecycle heading + table + all-versions link.
		"## Camunda 8.10 — 🚧 Alpha",
		"> Deploys Camunda Hub — see the [Hub documentation](https://example.invalid/hub).",
		"## Camunda 8.9 — ✅ Standard support until 2027-10-13",
		"| Helm Chart | Camunda | Released | Stability | Helm CLI | Helm Values | Release Notes |",
		"| [14.7.0](./camunda-8.9/#helm-chart-1470) |",
		"[All 7 chart versions for Camunda 8.9 →](./camunda-8.9/)",
		"## Camunda 8.8 — ✅ Standard support until 2027-04-13",
		// Extended support: one compact row per minor.
		"## Extended support — 🔒 contact your CSM",
		"| Camunda | Released | Latest chart | Full matrix |",
		"| 8.6 | 2024-10-08 | [11.12.3](./camunda-8.6/#helm-chart-11123) | [camunda-8.6](./camunda-8.6/) |",
		// EOL: compact row from lifecycle data (no JSON needed).
		"## End of life — ⛔ no longer supported",
		"| Camunda | EOL since | Last chart | Full matrix |",
		"| 8.2 | 2024-10-08 | [8.2.34](./camunda-8.2/#helm-chart-8234) | [camunda-8.2](./camunda-8.2/) |",
	}
	for _, c := range checks {
		if !strings.Contains(got, c) {
			t.Errorf("RenderIndex missing %q", c)
		}
	}

	// Index caps each active minor at indexTableLimit rows: 14.3.0 and 14.4.0
	// (rows 6-7) must NOT appear, while all seven stay reachable via the link.
	if strings.Contains(got, "[14.3.0]") || strings.Contains(got, "[14.4.0](") {
		t.Errorf("RenderIndex: rows beyond the %d-row cap leaked into the index", indexTableLimit)
	}
	// Old heading-list layout must be gone.
	if strings.Contains(got, "### [Helm chart") {
		t.Errorf("RenderIndex: legacy heading-list layout leaked into the index")
	}
}

func TestRenderIndexPlaceholderForEmptyActiveMinor(t *testing.T) {
	// A minor can be classified in chart-versions.yaml before its first chart
	// is promoted (documented minor-rollover chores) — the index must render
	// a placeholder, not fail every other minor's release.
	entriesByApp := map[string][]ChartEntry{
		"8.10": {{ChartVersion: "15.0.0-alpha3"}},
		"8.9":  {{ChartVersion: "14.7.0"}},
		// 8.8 missing entirely.
		"8.6": {{ChartVersion: "11.12.3"}},
	}
	got, err := RenderIndex(testConfig(), entriesByApp)
	if err != nil {
		t.Fatalf("RenderIndex: %v", err)
	}
	if !strings.Contains(got, "## Camunda 8.8 — ✅ Standard support until 2027-04-13") {
		t.Errorf("empty active minor lost its section heading")
	}
	if !strings.Contains(got, "_No chart releases for Camunda 8.8 yet._") {
		t.Errorf("empty active minor missing placeholder:\n%s", got)
	}
	if strings.Contains(got, "[All 0 chart versions") {
		t.Errorf("empty active minor must not render an all-versions link")
	}
}

func TestRenderMinorReadme(t *testing.T) {
	entries := []ChartEntry{
		{ChartVersion: "14.6.1", ReleaseDate: "2026-07-08", HelmCLI: "3.20.2,4.2.3", ReleaseTag: "camunda-platform-8.9-14.6.1",
			ChartImages: []string{"docker.io/camunda/camunda:8.9.11"}},
		{ChartVersion: "14.7.0", ReleaseDate: "2026-07-17", HelmCLI: "3.20.2,4.2.3", ReleaseTag: "camunda-platform-8.9-14.7.0",
			ChartImages: []string{"docker.io/camunda/camunda:8.9.12", "docker.io/bitnamilegacy/elasticsearch:8.18.0"}},
	}
	lc := Lifecycle{Released: "2026-04-14", StdSupportUntil: "2027-10-13"}
	got := RenderMinorReadme("8.9", entries, BucketSupportStandard, lc, nil)

	checks := []string{
		"<!-- THIS FILE IS AUTO-GENERATED, DO NOT EDIT IT MANUALLY! -->",
		"🔙 [Back to version matrix index](../)",
		"# Camunda 8.9 Helm Chart Version Matrix",
		"✅ Standard support until 2027-10-13",
		"| Helm Chart | Camunda | Released | Stability | Helm CLI | Helm Values | Release Notes |",
		// Summary rows link in-page and are sorted newest-first.
		"| [14.7.0](#helm-chart-1470) |",
		"| [14.6.1](#helm-chart-1461) |",
		// Per-version sections preserved with their anchors and image lists.
		"## Helm chart 14.7.0",
		"## Helm chart 14.6.1",
		"- docker.io/camunda/camunda:8.9.12",
		"Non-Camunda images:",
		"- docker.io/bitnamilegacy/elasticsearch:8.18.0",
	}
	for _, c := range checks {
		if !strings.Contains(got, c) {
			t.Errorf("RenderMinorReadme missing %q", c)
		}
	}
	// Sections are newest-first.
	if strings.Index(got, "## Helm chart 14.7.0") > strings.Index(got, "## Helm chart 14.6.1") {
		t.Errorf("RenderMinorReadme: sections not newest-first")
	}
	// Summary table precedes the sections.
	if strings.Index(got, "| [14.7.0](#helm-chart-1470)") > strings.Index(got, "## Helm chart 14.7.0") {
		t.Errorf("RenderMinorReadme: summary table must precede sections")
	}
	if !strings.HasSuffix(got, "\n") || strings.HasSuffix(got, "\n\n") {
		t.Errorf("RenderMinorReadme: want single trailing newline")
	}
}

func TestRenderMinorReadmeEOLStatus(t *testing.T) {
	lc := Lifecycle{Released: "2022-10-11", EOLSince: "2024-10-08"}
	got := RenderMinorReadme("8.2", []ChartEntry{{ChartVersion: "8.2.34"}}, BucketEndOfLife, lc, nil)
	if !strings.Contains(got, "⛔ End of life since 2024-10-08") {
		t.Errorf("RenderMinorReadme: missing EOL status line:\n%q", got[:200])
	}
}

func TestRenderMinorReadmePreservesExistingSections(t *testing.T) {
	// Published section bodies are the historical truth: when a section for a
	// version already exists it must be kept byte-for-byte, even when the JSON
	// entry disagrees (the SUPPORT-33569 drift class). Missing sections render
	// from JSON.
	entries := []ChartEntry{
		{ChartVersion: "14.7.0", ReleaseDate: "2026-07-17", ChartImages: []string{"docker.io/camunda/camunda:WRONG-JSON-TAG"}},
		{ChartVersion: "14.6.1", ReleaseDate: "2026-07-08", ChartImages: []string{"docker.io/camunda/camunda:8.9.11"}, HelmCLI: "3.20.2"},
	}
	existing := map[string]string{
		"14.7.0": "## Helm chart 14.7.0\n\nfrozen published body",
	}
	lc := Lifecycle{Released: "2026-04-14", StdSupportUntil: "2027-10-13"}
	got := RenderMinorReadme("8.9", entries, BucketSupportStandard, lc, existing)

	if !strings.Contains(got, "## Helm chart 14.7.0\n\nfrozen published body") {
		t.Errorf("existing section body not preserved verbatim:\n%s", got)
	}
	// The summary table may legitimately derive its Camunda column from the
	// JSON entry; only the preserved SECTION bodies must stay untouched.
	sections := got[strings.Index(got, "## Helm chart"):]
	if strings.Contains(sections, "WRONG-JSON-TAG") {
		t.Errorf("JSON image set leaked into a preserved section")
	}
	if !strings.Contains(got, "## Helm chart 14.6.1") || !strings.Contains(got, "docker.io/camunda/camunda:8.9.11") {
		t.Errorf("missing section not rendered from JSON:\n%s", got)
	}
	// The summary table still renders every version.
	if !strings.Contains(got, "| [14.7.0](#helm-chart-1470)") || !strings.Contains(got, "| [14.6.1](#helm-chart-1461)") {
		t.Errorf("summary table incomplete")
	}
}

func TestRenderMinorReadmeRerendersUnpublishedAndAlpha(t *testing.T) {
	// An UNSTAMPED entry re-renders from JSON even when a section exists (an
	// in-flight promotion may re-derive its images between RCs)…
	entries := []ChartEntry{{ChartVersion: "14.8.0", ChartImages: []string{"docker.io/camunda/camunda:8.9.13"}}}
	existing := map[string]string{"14.8.0": "## Helm chart 14.8.0\n\nSTALE RC1 BODY"}
	lc := Lifecycle{Released: "2026-04-14", StdSupportUntil: "2027-10-13"}
	got := RenderMinorReadme("8.9", entries, BucketSupportStandard, lc, existing)
	if strings.Contains(got, "STALE RC1 BODY") {
		t.Errorf("unstamped entry kept its stale section:\n%s", got)
	}
	if !strings.Contains(got, "docker.io/camunda/camunda:8.9.13") {
		t.Errorf("unstamped entry not re-rendered from JSON")
	}

	// …and alpha-bucket minors ALWAYS render from JSON: their JSON is written
	// by the current promotion pipeline while earlier splice-era sections can
	// carry another release's images (the 8.10 alpha2 case).
	entries = []ChartEntry{{ChartVersion: "15.0.0-alpha2", ReleaseDate: "2026-06-05", ChartImages: []string{"docker.io/camunda/camunda:8.10.0-alpha2"}}}
	existing = map[string]string{"15.0.0-alpha2": "## Helm chart 15.0.0-alpha2\n\n- docker.io/camunda/camunda:8.10.0-alpha1"}
	got = RenderMinorReadme("8.10", entries, BucketAlpha, Lifecycle{}, existing)
	if strings.Contains(got, "alpha1") {
		t.Errorf("alpha section not re-rendered from JSON:\n%s", got)
	}
	if !strings.Contains(got, "docker.io/camunda/camunda:8.10.0-alpha2") {
		t.Errorf("alpha section missing JSON images")
	}
}

func TestReleaseSectionOmitsEmptyTagRefs(t *testing.T) {
	entry := ChartEntry{
		ChartVersion: "15.0.0-alpha1",
		ChartImages: []string{
			"docker.io/camunda/camunda:8.10.0-alpha1",
			"docker.io/camunda/hub:",
			"docker.io/camunda/hub-websockets:",
		},
		ChartEnterpriseImages: []string{"registry.camunda.cloud/vendor-ee/elasticsearch:"},
	}
	got := ReleaseSection("8.10", entry, nil, true)
	if strings.Contains(got, "hub:") || strings.Contains(got, "hub-websockets:") {
		t.Errorf("empty-tag refs leaked into the rendered section:\n%s", got)
	}
	if strings.Contains(got, "Enterprise images") {
		t.Errorf("enterprise block rendered although all its refs have empty tags")
	}
	if !strings.Contains(got, "docker.io/camunda/camunda:8.10.0-alpha1") {
		t.Errorf("valid image dropped")
	}
}

func TestParseReadmeSections(t *testing.T) {
	readme := "preamble\n| table |\n\n## Helm chart 12.10.0\n\nbody A\n\n\n## Helm chart 12.9.0\n\nbody B\n"
	got := ParseReadmeSections(readme)
	if len(got) != 2 {
		t.Fatalf("ParseReadmeSections: got %d sections want 2", len(got))
	}
	if got["12.10.0"] != "## Helm chart 12.10.0\n\nbody A" {
		t.Errorf("ParseReadmeSections 12.10.0=%q", got["12.10.0"])
	}
	if got["12.9.0"] != "## Helm chart 12.9.0\n\nbody B" {
		t.Errorf("ParseReadmeSections 12.9.0=%q", got["12.9.0"])
	}
	if len(ParseReadmeSections("no sections here")) != 0 {
		t.Errorf("ParseReadmeSections: headerless input should yield empty map")
	}
}

func TestEnterpriseDocsURL(t *testing.T) {
	cases := map[string]string{
		"8.7": "https://docs.camunda.io/docs/8.7/self-managed/setup/guides/install-bitnami-enterprise-images/",
		"8.8": "https://docs.camunda.io/docs/8.8/self-managed/deployment/helm/configure/registry-and-images/install-bitnami-enterprise-images/",
		"8.9": "https://docs.camunda.io/docs/8.9/self-managed/deployment/helm/configure/registry-and-images/install-bitnami-enterprise-images/",
	}
	for appVersion, want := range cases {
		if got := enterpriseDocsURL(appVersion); got != want {
			t.Errorf("enterpriseDocsURL(%q) = %q, want %q", appVersion, got, want)
		}
	}
}

func TestSyncHelmCLILineInPreservedSection(t *testing.T) {
	entries := []ChartEntry{{
		ChartVersion: "14.7.0", ReleaseDate: "2026-07-17", HelmCLI: "3.20.2,4.2.2",
	}}
	existing := map[string]string{
		"14.7.0": "## Helm chart 14.7.0\n\nSupported versions:\n\n" +
			"- Helm CLI: [3.20.2](https://github.com/helm/helm/releases/tag/v3.20.2), [4.2.3](https://github.com/helm/helm/releases/tag/v4.2.3)\n\n" +
			"Camunda images:\n\n- docker.io/camunda/camunda:8.9.12\n",
	}
	lc := Lifecycle{Released: "2026-04-14", StdSupportUntil: "2027-10-13"}
	got := RenderMinorReadme("8.9", entries, BucketSupportStandard, lc, existing)
	if strings.Contains(got, "4.2.3") {
		t.Errorf("stale Helm CLI line survived in preserved section:\n%s", got)
	}
	if strings.Count(got, "- Helm CLI: [3.20.2](https://github.com/helm/helm/releases/tag/v3.20.2), [4.2.2](https://github.com/helm/helm/releases/tag/v4.2.2)") != 1 {
		t.Errorf("Helm CLI line not synced to recorded helm_cli:\n%s", got)
	}
	if !strings.Contains(got, "- docker.io/camunda/camunda:8.9.12") {
		t.Errorf("image list of preserved section altered")
	}
}
