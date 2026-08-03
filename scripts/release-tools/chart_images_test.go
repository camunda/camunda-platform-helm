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
	"testing"

	"gopkg.in/yaml.v3"
)

func TestArtifactHubImagesYAML(t *testing.T) {
	images := []string{
		"docker.io/camunda/camunda:8.9.13",
		"docker.io/camunda/connectors-bundle@sha256:abc123",
	}

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
	stdout, err := os.CreateTemp(t.TempDir(), "chart-images-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	originalStdout := os.Stdout
	os.Stdout = stdout
	runErr := runChartImages([]string{"--chart-dir", chartDir, "--artifacthub-out", artifactHubOut})
	os.Stdout = originalStdout
	if err := stdout.Close(); err != nil {
		t.Fatal(err)
	}
	if runErr != nil {
		t.Fatalf("runChartImages: %v", runErr)
	}

	canonical, err := os.ReadFile(stdout.Name())
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(canonical), "docker.io/camunda/camunda:8.9.13\n"; got != want {
		t.Errorf("canonical output = %q, want %q", got, want)
	}

	artifactHub, err := os.ReadFile(artifactHubOut)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(artifactHub), "- image: docker.io/camunda/camunda:8.9.13\n"; got != want {
		t.Errorf("Artifact Hub output = %q, want %q", got, want)
	}
}
