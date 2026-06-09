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

Enterprise images ([Camunda Enterprise](https://docs.camunda.io/docs/self-managed/setup/guides/install-bitnami-enterprise-images/)):

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

func TestSpliceReadmeFresh(t *testing.T) {
	// Empty existing (a new app's first release) → a complete file with one section.
	entry := ChartEntry{
		ChartVersion: "12.0.0",
		ChartImages:  []string{"docker.io/camunda/zeebe:8.7.0"},
	}
	got := SpliceReadme("", "8.7", entry, []string{"3.20.2"})

	if !strings.HasPrefix(got, "<!-- THIS FILE IS AUTO-GENERATED, DO NOT EDIT IT MANUALLY! -->\n🔙 [Back to version matrix index](../)\n\n# Camunda 8.7 Helm Chart Version Matrix\n\n## ToC\n\n- [Helm chart 12.0.0](#helm-chart-1200)\n\n## Helm chart 12.0.0\n") {
		t.Errorf("SpliceReadme fresh: unexpected header/ToC/section start:\n%q", got)
	}
	if !strings.HasSuffix(got, "\n") || strings.HasSuffix(got, "\n\n") {
		t.Errorf("SpliceReadme fresh: want single trailing newline, got %q", got[len(got)-3:])
	}
}

func TestSpliceReadmeInsertPreservesFrozenRows(t *testing.T) {
	// Two existing rows with arbitrary (possibly stale) bodies. Splicing the
	// newest version must leave both existing rows BYTE-for-BYTE intact.
	existing := "<!-- THIS FILE IS AUTO-GENERATED, DO NOT EDIT IT MANUALLY! -->\n" +
		"🔙 [Back to version matrix index](../)\n\n" +
		"# Camunda 8.7 Helm Chart Version Matrix\n\n" +
		"## ToC\n\n" +
		"- [Helm chart 12.10.0](#helm-chart-12100)\n" +
		"- [Helm chart 12.9.0](#helm-chart-1290)\n\n" +
		"## Helm chart 12.10.0\n\nfrozen ten body\n\n\n" +
		"## Helm chart 12.9.0\n\nfrozen nine body\n"

	entry := ChartEntry{
		ChartVersion: "12.11.0",
		ChartImages:  []string{"docker.io/camunda/zeebe:8.7.30"},
	}
	got := SpliceReadme(existing, "8.7", entry, []string{"3.20.2"})

	// ToC regenerated newest-first, including the new version.
	if !containsStr(got, "## ToC\n\n- [Helm chart 12.11.0](#helm-chart-12110)\n- [Helm chart 12.10.0](#helm-chart-12100)\n- [Helm chart 12.9.0](#helm-chart-1290)\n") {
		t.Errorf("SpliceReadme: ToC not regenerated newest-first:\n%q", got)
	}
	// Frozen rows preserved verbatim.
	if !containsStr(got, "## Helm chart 12.10.0\n\nfrozen ten body") {
		t.Errorf("SpliceReadme: 12.10.0 frozen body altered")
	}
	if !containsStr(got, "## Helm chart 12.9.0\n\nfrozen nine body") {
		t.Errorf("SpliceReadme: 12.9.0 frozen body altered")
	}
	// New section rendered and placed first.
	if !containsStr(got, "1290)\n\n## Helm chart 12.11.0\n") {
		t.Errorf("SpliceReadme: new section not first / wrong ToC-to-section spacing")
	}
	// Two blank lines between sections.
	if !containsStr(got, "\n\n\n## Helm chart 12.10.0") || !containsStr(got, "\n\n\n## Helm chart 12.9.0") {
		t.Errorf("SpliceReadme: expected two blank lines between sections")
	}
	// Single trailing newline.
	if !strings.HasSuffix(got, "frozen nine body\n") || strings.HasSuffix(got, "frozen nine body\n\n") {
		t.Errorf("SpliceReadme: want single trailing newline at EOF")
	}
}

func TestSpliceReadmeReplacesSameVersion(t *testing.T) {
	existing := "<!-- THIS FILE IS AUTO-GENERATED, DO NOT EDIT IT MANUALLY! -->\n" +
		"🔙 [Back to version matrix index](../)\n\n" +
		"# Camunda 8.7 Helm Chart Version Matrix\n\n" +
		"## ToC\n\n" +
		"- [Helm chart 12.10.0](#helm-chart-12100)\n" +
		"- [Helm chart 12.9.0](#helm-chart-1290)\n\n" +
		"## Helm chart 12.10.0\n\nOLD ten body\n\n\n" +
		"## Helm chart 12.9.0\n\nfrozen nine body\n"

	entry := ChartEntry{
		ChartVersion: "12.10.0",
		ChartImages:  []string{"docker.io/camunda/zeebe:8.7.99"},
	}
	got := SpliceReadme(existing, "8.7", entry, []string{"3.20.2"})

	if containsStr(got, "OLD ten body") {
		t.Errorf("SpliceReadme: old 12.10.0 body not replaced")
	}
	if !containsStr(got, "docker.io/camunda/zeebe:8.7.99") {
		t.Errorf("SpliceReadme: new 12.10.0 body not rendered")
	}
	// No duplicate ToC entry for the replaced version.
	if strings.Count(got, "- [Helm chart 12.10.0](#helm-chart-12100)") != 1 {
		t.Errorf("SpliceReadme: duplicate/missing ToC entry for replaced version")
	}
	// Sibling stays frozen.
	if !containsStr(got, "## Helm chart 12.9.0\n\nfrozen nine body") {
		t.Errorf("SpliceReadme: 12.9.0 frozen body altered on same-version replace")
	}
}

func TestParseReadmeSections(t *testing.T) {
	readme := "preamble\n## ToC\n\n- x\n\n## Helm chart 12.10.0\n\nbody A\n\n\n## Helm chart 12.9.0\n\nbody B\n"
	got := parseReadmeSections(readme)
	if len(got) != 2 {
		t.Fatalf("parseReadmeSections: got %d sections want 2", len(got))
	}
	if got["12.10.0"] != "## Helm chart 12.10.0\n\nbody A" {
		t.Errorf("parseReadmeSections 12.10.0=%q", got["12.10.0"])
	}
	if got["12.9.0"] != "## Helm chart 12.9.0\n\nbody B" {
		t.Errorf("parseReadmeSections 12.9.0=%q", got["12.9.0"])
	}
	if len(parseReadmeSections("no sections here")) != 0 {
		t.Errorf("parseReadmeSections: headerless input should yield empty map")
	}
}

func TestRenderIndex(t *testing.T) {
	appVersions := []string{"8.10", "8.9"}
	entriesByApp := map[string][]ChartEntry{
		"8.10": {{ChartVersion: "15.0.0-alpha2"}, {ChartVersion: "15.0.0-alpha1"}},
		"8.9":  {{ChartVersion: "14.4.0"}, {ChartVersion: "14.3.0"}},
	}
	got := RenderIndex(appVersions, entriesByApp)

	checks := []string{
		"<!-- THIS FILE IS AUTO-GENERATED, DO NOT EDIT IT MANUALLY! -->",
		"# Camunda 8 Helm Chart Version Matrix",
		"## Overview",
		"## [Camunda 8.10](./camunda-8.10)",
		"### [Helm chart 15.0.0-alpha2](./camunda-8.10/#helm-chart-1500-alpha2)",
		"### [Helm chart 15.0.0-alpha1](./camunda-8.10/#helm-chart-1500-alpha1)",
		"## [Camunda 8.9](./camunda-8.9)",
		"### [Helm chart 14.4.0](./camunda-8.9/#helm-chart-1440)",
	}
	for _, c := range checks {
		if !containsStr(got, c) {
			t.Errorf("RenderIndex missing %q", c)
		}
	}
	// Two blank lines before first app (from header).
	if !containsStr(got, "\n\n\n## [Camunda 8.10]") {
		t.Errorf("RenderIndex: expected two blank lines before first app section")
	}
	// One blank line between apps.
	if !containsStr(got, "alpha1)\n\n## [Camunda 8.9]") {
		t.Errorf("RenderIndex: expected one blank line between app sections")
	}
	// One blank line after app heading.
	if !containsStr(got, "[Camunda 8.9](./camunda-8.9)\n\n### [Helm chart 14.4.0]") {
		t.Errorf("RenderIndex: expected one blank line after app heading")
	}
	// No blank line between consecutive chart entries.
	if !containsStr(got, "alpha2)\n### [Helm chart 15.0.0-alpha1]") {
		t.Errorf("RenderIndex: expected no blank line between chart entries")
	}
}

func containsStr(s, sub string) bool {
	return strings.Contains(s, sub)
}
