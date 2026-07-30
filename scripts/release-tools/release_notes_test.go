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
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseNotesFooterUsesSeparateImageChart(t *testing.T) {
	root := t.TempDir()
	targetDir := filepath.Join(root, "target")
	imagesDir := filepath.Join(root, "package")
	for _, dir := range []string{targetDir, imagesDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	targetChart := "name: camunda-platform\nversion: 14.7.0\nappVersion: 8.9.x\n"
	if err := os.WriteFile(filepath.Join(targetDir, "Chart.yaml"), []byte(targetChart), 0o644); err != nil {
		t.Fatal(err)
	}
	packageChart := `
name: camunda-platform
version: 14.7.0
appVersion: 8.9.x
annotations:
  camunda.io/chart-images: |
    docker.io/camunda/camunda:8.9.13
    registry.camunda.cloud/camunda/keycloak:26.3.3
dependencies:
  - name: keycloak
    alias: identityKeycloak
    repository: file://../keycloak-24
`
	if err := os.WriteFile(filepath.Join(imagesDir, "Chart.yaml"), []byte(packageChart), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(targetDir, "values.yaml"), []byte(`
orchestration:
  image:
    repository: camunda/camunda
    tag: 8.9.12
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(imagesDir, "values.yaml"), []byte(`
orchestration:
  image:
    repository: camunda/camunda
    tag: 8.9.13
identityKeycloak:
  image:
    repository: camunda/keycloak
    tag: 26.3.3
`), 0o644); err != nil {
		t.Fatal(err)
	}
	keycloakDir := filepath.Join(imagesDir, "charts", "keycloak")
	if err := os.MkdirAll(keycloakDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(keycloakDir, "values.yaml"), []byte(`
image:
  registry: docker.io
  repository: bitnami/keycloak
  tag: 26.3.3
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(imagesDir, "values-enterprise.yaml"), []byte(`
orchestration:
  image:
    registry: registry.camunda.cloud
    repository: camunda/camunda
    tag: 8.9.13
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(targetDir, "RELEASE-NOTES.md"), []byte("release body\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".tool-versions"), []byte("helm 4.2.2\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
	t.Setenv("GITHUB_REPOSITORY", "camunda/camunda-platform-helm")

	if err := releaseNotesFooter(context.Background(), targetDir, imagesDir, true); err != nil {
		t.Fatalf("releaseNotesFooter: %v", err)
	}
	notes, err := os.ReadFile(filepath.Join(targetDir, "RELEASE-NOTES.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"docker.io/camunda/camunda:8.9.13",
		"registry.camunda.cloud/camunda/keycloak:26.3.3",
		"registry.camunda.cloud/camunda/camunda:8.9.13",
	} {
		if !strings.Contains(string(notes), want) {
			t.Errorf("release notes missing %q:\n%s", want, notes)
		}
	}
	if strings.Contains(string(notes), "docker.io/camunda/camunda:8.9.12") {
		t.Errorf("release notes used target chart images instead of package images:\n%s", notes)
	}
	if strings.Contains(string(notes), "docker.io/camunda/keycloak:26.3.3") {
		t.Errorf("release notes recomputed standard images instead of using the package annotation:\n%s", notes)
	}
}
