package chartmeta

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return p
}

func TestStripDotX(t *testing.T) {
	cases := map[string]string{
		"8.10.x":        "8.10",
		"8.8.x":         "8.8",
		"8.7.x":         "8.7",
		"8.10.0-alpha2": "8.10.0-alpha2", // no ".x" → unchanged
		"":              "",
	}
	for in, want := range cases {
		if got := stripDotX(in); got != want {
			t.Errorf("stripDotX(%q)=%q want %q", in, got, want)
		}
	}
}

func TestReadPackageMetadataFull(t *testing.T) {
	dir := t.TempDir()
	chartYAML := writeFile(t, dir, "Chart.yaml", `
version: 8.10.1
appVersion: 8.10.x
annotations:
  artifacthub.io/prerelease: "true"
  camunda.io/component-image-versions: |
    camunda: 8.10.0
    console: 8.9.25
  camunda.io/imageOverrides: |
    orchestration: custom
`)
	chartVersions := writeFile(t, dir, "chart-versions.yaml", `
camundaVersions:
  supportStandard:
    - "8.10"
    - "8.9"
`)
	meta, err := ReadPackageMetadata(chartYAML, chartVersions)
	if err != nil {
		t.Fatalf("ReadPackageMetadata: %v", err)
	}
	if meta.Version != "8.10.1" {
		t.Errorf("Version=%q want 8.10.1", meta.Version)
	}
	if meta.AppVersion != "8.10" {
		t.Errorf("AppVersion=%q want 8.10", meta.AppVersion)
	}
	if meta.Prerelease != "true" {
		t.Errorf("Prerelease=%q want true", meta.Prerelease)
	}
	if meta.ReleaseTag != "camunda-platform-8.10-8.10.1" {
		t.Errorf("ReleaseTag=%q", meta.ReleaseTag)
	}
	if meta.CosignBundle != "camunda-platform-8.10.1-cosign-bundle.json" {
		t.Errorf("CosignBundle=%q", meta.CosignBundle)
	}
	if meta.CosignVerify != "camunda-platform-8.10.1-cosign-verify.sh" {
		t.Errorf("CosignVerify=%q", meta.CosignVerify)
	}
	if meta.ImageVersions != "camunda: 8.10.0\nconsole: 8.9.25\n" {
		t.Errorf("ImageVersions=%q", meta.ImageVersions)
	}
	if !meta.HasImageOverrides {
		t.Error("HasImageOverrides should be true")
	}
	if meta.IsLatestStable == nil || !*meta.IsLatestStable {
		t.Errorf("IsLatestStable should be true (8.10 == supportStandard[0])")
	}
}

func TestReadPackageMetadataDefaultsAndNotLatest(t *testing.T) {
	dir := t.TempDir()
	chartYAML := writeFile(t, dir, "Chart.yaml", `
version: 8.9.5
appVersion: 8.9.x
annotations:
  camunda.io/component-image-versions: ""
`)
	chartVersions := writeFile(t, dir, "chart-versions.yaml", `
camundaVersions:
  supportStandard:
    - "8.10"
    - "8.9"
`)
	meta, err := ReadPackageMetadata(chartYAML, chartVersions)
	if err != nil {
		t.Fatalf("ReadPackageMetadata: %v", err)
	}
	if meta.Prerelease != "false" {
		t.Errorf("Prerelease default should be false, got %q", meta.Prerelease)
	}
	if meta.HasImageOverrides {
		t.Error("HasImageOverrides should be false when annotation absent")
	}
	if meta.ImageVersions != "" {
		t.Errorf("ImageVersions should be empty, got %q", meta.ImageVersions)
	}
	if meta.IsLatestStable == nil || *meta.IsLatestStable {
		t.Error("IsLatestStable should be false (8.9 != 8.10)")
	}
}

func TestChartImages(t *testing.T) {
	dir := t.TempDir()
	chartYAML := writeFile(t, dir, "Chart.yaml", `apiVersion: v2
name: camunda-platform
version: 15.0.0-test
annotations:
  camunda.io/component-image-versions: |
    camunda: 8.10.0-alpha1
  camunda.io/chart-images: |
    docker.io/camunda/camunda:8.10.0-alpha1
    docker.io/camunda/identity:8.10.0-alpha1

    docker.io/camunda/optimize:8.10.0-alpha1
`)
	got, err := ChartImages(chartYAML)
	if err != nil {
		t.Fatalf("ChartImages: %v", err)
	}
	want := []string{
		"docker.io/camunda/camunda:8.10.0-alpha1",
		"docker.io/camunda/identity:8.10.0-alpha1",
		"docker.io/camunda/optimize:8.10.0-alpha1", // blank line skipped
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("ChartImages:\n got: %v\nwant: %v", got, want)
	}
}

func TestChartImagesMissing(t *testing.T) {
	dir := t.TempDir()
	chartYAML := writeFile(t, dir, "Chart.yaml", "name: x\nversion: 1.0.0\n")
	got, err := ChartImages(chartYAML)
	if err != nil {
		t.Fatalf("ChartImages: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected no images for missing annotation, got %v", got)
	}
}

func TestReadPackageMetadataNoChartVersions(t *testing.T) {
	dir := t.TempDir()
	chartYAML := writeFile(t, dir, "Chart.yaml", "version: 1.0.0\nappVersion: 8.8.x\n")
	meta, err := ReadPackageMetadata(chartYAML, "")
	if err != nil {
		t.Fatalf("ReadPackageMetadata: %v", err)
	}
	if meta.IsLatestStable != nil {
		t.Error("IsLatestStable should be nil when chart-versions not provided")
	}
	if meta.AppVersion != "8.8" {
		t.Errorf("AppVersion=%q want 8.8", meta.AppVersion)
	}
}
