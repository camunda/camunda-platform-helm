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
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestArtifactHubImagesYAML(t *testing.T) {
	images := []string{
		"docker.io/camunda/camunda:8.9.13",
		"docker.io/camunda/connectors-bundle@sha256:abc123",
	}
	wantNames := []string{"camunda", "connectors-bundle"}

	got, err := artifactHubImagesYAML(images)
	if err != nil {
		t.Fatalf("artifactHubImagesYAML: %v", err)
	}

	var entries []artifactHubImage
	if err := yaml.Unmarshal(got, &entries); err != nil {
		t.Fatalf("unmarshal Artifact Hub images: %v", err)
	}
	if len(entries) != len(images) {
		t.Fatalf("got %d entries, want %d", len(entries), len(images))
	}
	for i, entry := range entries {
		if entry.Image != images[i] {
			t.Errorf("entry %d image = %q, want %q", i, entry.Image, images[i])
		}
		if entry.Name != wantNames[i] {
			t.Errorf("entry %d name = %q, want %q", i, entry.Name, wantNames[i])
		}
	}
}

func TestImageName(t *testing.T) {
	for _, tc := range []struct{ ref, want string }{
		{"docker.io/camunda/keycloak:26.3.3", "keycloak"},
		{"docker.io/camunda/connectors-bundle@sha256:abc123", "connectors-bundle"},
		{"registry.camunda.cloud/camunda/camunda:8.9.13", "camunda"},
		{"busybox:1.36", "busybox"},
	} {
		if got := imageName(tc.ref); got != tc.want {
			t.Errorf("imageName(%q) = %q, want %q", tc.ref, got, tc.want)
		}
	}
}

func TestValidateImageRefsRejectsPlaceholders(t *testing.T) {
	err := validateImageRefs([]string{"docker.io/camunda/console:$E2E_TESTS_CONSOLE_IMAGE_TAG"})
	if err == nil || !strings.Contains(err.Error(), "unresolved") {
		t.Fatalf("err = %v, want an unresolved-placeholder error", err)
	}

	if err := validateImageRefs([]string{"docker.io/camunda/camunda:8.9.13"}); err != nil {
		t.Errorf("valid ref rejected: %v", err)
	}
}

func TestRunChartImagesWritesArtifactHubOutput(t *testing.T) {
	chartDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(chartDir, "values.yaml"), []byte(`
orchestration:
  image:
    repository: camunda/camunda
    tag: 8.9.13
`), 0o644); err != nil {
		t.Fatal(err)
	}

	artifactHubOut := filepath.Join(t.TempDir(), "artifacthub-images.yaml")
	var stdout bytes.Buffer
	if err := runChartImages([]string{"--chart-dir", chartDir, "--artifacthub-out", artifactHubOut}, &stdout); err != nil {
		t.Fatalf("runChartImages: %v", err)
	}

	if got, want := stdout.String(), "docker.io/camunda/camunda:8.9.13\n"; got != want {
		t.Errorf("canonical output = %q, want %q", got, want)
	}

	artifactHub, err := os.ReadFile(artifactHubOut)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(artifactHub), "- name: camunda\n  image: docker.io/camunda/camunda:8.9.13\n"; got != want {
		t.Errorf("Artifact Hub output = %q, want %q", got, want)
	}
}
