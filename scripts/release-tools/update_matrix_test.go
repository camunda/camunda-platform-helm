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
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestUpdateMatrixHardFailsOnMissingAnnotation pins the promote-rc invariant:
// when --chart-yaml has no camunda.io/chart-images annotation, runUpdateMatrix
// must hard-fail. Dev artifacts built before the build-time annotation step
// landed are intentionally non-promotable; a silent fallback would defeat that.
func TestUpdateMatrixHardFailsOnMissingAnnotation(t *testing.T) {
	dir := t.TempDir()
	chartYAML := filepath.Join(dir, "Chart.yaml")
	if err := os.WriteFile(chartYAML, []byte("name: camunda-platform\nversion: 15.0.0\n"), 0o644); err != nil {
		t.Fatalf("write Chart.yaml: %v", err)
	}
	matrixFile := filepath.Join(dir, "version-matrix.json")

	err := runUpdateMatrix([]string{
		"--chart-yaml", chartYAML,
		"--chart-version", "15.0.0",
		"--matrix-file", matrixFile,
	})
	if err == nil {
		t.Fatal("expected error for empty/missing chart-images annotation, got nil")
	}
	if !strings.Contains(err.Error(), "camunda.io/chart-images") {
		t.Errorf("error should mention the missing annotation, got: %v", err)
	}
	if _, statErr := os.Stat(matrixFile); statErr == nil {
		t.Error("matrix file must not be written when annotation is missing")
	}
}

// TestUpdateMatrixRequiresInputMode pins that at least one input is required.
func TestUpdateMatrixRequiresInputMode(t *testing.T) {
	dir := t.TempDir()
	matrixFile := filepath.Join(dir, "version-matrix.json")

	err := runUpdateMatrix([]string{"--chart-version", "15.0.0", "--matrix-file", matrixFile})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "--chart-yaml") || !strings.Contains(err.Error(), "--chart-dir") {
		t.Errorf("error should reference both flags, got: %v", err)
	}
}

// TestUpdateMatrixArtifactImagesWithEnterpriseChart pins artifact precedence
// when the package image set differs from source-controlled values.
func TestUpdateMatrixArtifactImagesWithEnterpriseChart(t *testing.T) {
	dir := t.TempDir()
	chartYAML := filepath.Join(dir, "Chart.yaml")
	chart := "name: camunda-platform\nversion: 14.7.0\nannotations:\n" +
		"  camunda.io/chart-images: |\n    docker.io/camunda/camunda:8.9.13\n"
	if err := os.WriteFile(chartYAML, []byte(chart), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "values.yaml"), []byte(`
global:
  image:
    registry: docker.io
orchestration:
  image:
    repository: camunda/camunda
    tag: 8.9.12
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "values-enterprise.yaml"), []byte(`
orchestration:
  image:
    registry: registry.camunda.cloud
    repository: camunda/camunda
    tag: 8.9.13
`), 0o644); err != nil {
		t.Fatal(err)
	}
	matrixFile := filepath.Join(dir, "version-matrix.json")

	if err := runUpdateMatrix([]string{
		"--chart-yaml", chartYAML,
		"--chart-dir", dir,
		"--chart-version", "14.7.0",
		"--matrix-file", matrixFile,
	}); err != nil {
		t.Fatalf("runUpdateMatrix: %v", err)
	}

	data, err := os.ReadFile(matrixFile)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"docker.io/camunda/camunda:8.9.13",
		"registry.camunda.cloud/camunda/camunda:8.9.13",
	} {
		if !strings.Contains(string(data), want) {
			t.Errorf("matrix missing %q:\n%s", want, data)
		}
	}
	if strings.Contains(string(data), "docker.io/camunda/camunda:8.9.12") {
		t.Errorf("matrix used source standard image instead of artifact annotation:\n%s", data)
	}
}

// TestUpdateMatrixAppRecordsReleaseFacts pins that --app records helm_cli
// (clamped per minor from the .tool-versions pin) and release_tag, while
// leaving release_date to stamp-release.
func TestUpdateMatrixAppRecordsReleaseFacts(t *testing.T) {
	dir := t.TempDir()
	chartYAML := filepath.Join(dir, "Chart.yaml")
	chart := "name: camunda-platform\nversion: 13.4.0\nannotations:\n" +
		"  camunda.io/chart-images: |\n    docker.io/camunda/camunda:8.8.0\n"
	if err := os.WriteFile(chartYAML, []byte(chart), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".tool-versions"), []byte("helm 4.2.3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	matrixFile := filepath.Join(dir, "version-matrix.json")

	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })

	if err := runUpdateMatrix([]string{
		"--chart-yaml", chartYAML,
		"--chart-version", "13.4.0",
		"--matrix-file", matrixFile,
		"--app", "8.8",
	}); err != nil {
		t.Fatalf("runUpdateMatrix: %v", err)
	}

	data, err := os.ReadFile(matrixFile)
	if err != nil {
		t.Fatal(err)
	}
	// 8.8 is v3-only: a v4 pin clamps to 3.20.2.
	for _, want := range []string{
		`"helm_cli": "3.20.2"`,
		`"release_tag": "camunda-platform-8.8-13.4.0"`,
	} {
		if !strings.Contains(string(data), want) {
			t.Errorf("matrix missing %q:\n%s", want, data)
		}
	}
	if strings.Contains(string(data), "release_date") {
		t.Errorf("update-matrix must not write release_date:\n%s", data)
	}
}

// TestUpdateMatrixKeepsStampedFacts pins the write-once contract: once a
// release is stamped (release_date set), a re-run with --app must not rewrite
// helm_cli/release_tag from the current working tree.
func TestUpdateMatrixKeepsStampedFacts(t *testing.T) {
	dir := t.TempDir()
	chartYAML := filepath.Join(dir, "Chart.yaml")
	chart := "name: camunda-platform\nversion: 13.4.0\nannotations:\n" +
		"  camunda.io/chart-images: |\n    docker.io/camunda/camunda:8.8.1\n"
	if err := os.WriteFile(chartYAML, []byte(chart), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".tool-versions"), []byte("helm 9.9.9\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	matrixFile := filepath.Join(dir, "version-matrix.json")
	stamped := `[
  {
    "chart_version": "13.4.0",
    "chart_images": ["docker.io/camunda/camunda:8.8.0"],
    "chart_enterprise_images": ["registry.camunda.cloud/vendor-ee/elasticsearch:8.19.0"],
    "release_date": "2026-07-08",
    "helm_cli": "3.20.2",
    "release_tag": "camunda-platform-8.8-13.4.0"
  }
]
`
	if err := os.WriteFile(matrixFile, []byte(stamped), 0o644); err != nil {
		t.Fatal(err)
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })

	if err := runUpdateMatrix([]string{
		"--chart-yaml", chartYAML,
		"--chart-version", "13.4.0",
		"--matrix-file", matrixFile,
		"--app", "8.8",
	}); err != nil {
		t.Fatalf("runUpdateMatrix: %v", err)
	}
	data, err := os.ReadFile(matrixFile)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`"release_date": "2026-07-08"`,
		`"helm_cli": "3.20.2"`,
		`"docker.io/camunda/camunda:8.8.1"`,
		`"registry.camunda.cloud/vendor-ee/elasticsearch:8.19.0"`,
	} {
		if !strings.Contains(string(data), want) {
			t.Errorf("stamped entry lost %q:\n%s", want, data)
		}
	}
	if strings.Contains(string(data), "9.9.9") {
		t.Errorf("stamped helm_cli was rewritten from the current pin:\n%s", data)
	}
}
