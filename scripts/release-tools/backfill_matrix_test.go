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

func TestParseReadmeHelmCLI(t *testing.T) {
	readme := `<!-- THIS FILE IS AUTO-GENERATED, DO NOT EDIT IT MANUALLY! -->
🔙 [Back to version matrix index](../)

# Camunda 8.9 Helm Chart Version Matrix

## ToC

- [Helm chart 14.7.0](#helm-chart-1470)

## Helm chart 14.7.0

Supported versions:

- Helm values: [14.7.0](https://artifacthub.io/packages/helm/camunda/camunda-platform/14.7.0#parameters)
- Helm CLI: [3.20.2](https://github.com/helm/helm/releases/tag/v3.20.2), [4.2.3](https://github.com/helm/helm/releases/tag/v4.2.3)


## Helm chart 14.6.0

- Helm CLI: [3.20.2](https://github.com/helm/helm/releases/tag/v3.20.2)


## Helm chart 14.0.0-alpha1

- Helm CLI: N/A
`
	path := filepath.Join(t.TempDir(), "README.md")
	if err := os.WriteFile(path, []byte(readme), 0o644); err != nil {
		t.Fatal(err)
	}
	got := parseReadmeHelmCLI(path)
	if got["14.7.0"] != "3.20.2,4.2.3" {
		t.Errorf("14.7.0=%q want 3.20.2,4.2.3", got["14.7.0"])
	}
	if got["14.6.0"] != "3.20.2" {
		t.Errorf("14.6.0=%q want 3.20.2", got["14.6.0"])
	}
	if _, ok := got["14.0.0-alpha1"]; ok {
		t.Errorf("N/A section must not produce a value, got %q", got["14.0.0-alpha1"])
	}
	if len(parseReadmeHelmCLI(filepath.Join(t.TempDir(), "missing.md"))) != 0 {
		t.Error("missing README must yield an empty map")
	}
}

func TestParseReadmeImages(t *testing.T) {
	readme := `<!-- AUTO -->
# Camunda 8.6 Helm Chart Version Matrix

## Helm chart 11.0.0

Supported versions:

- Camunda applications: [8.6](https://example.invalid)
- Helm CLI: [3.16.2](https://example.invalid)

Camunda images:

- docker.io/camunda/zeebe:8.6.0
- docker.io/camunda/operate:8.6.0

Non-Camunda images:

- docker.io/bitnami/elasticsearch:8.12.2

Enterprise images ([Camunda Enterprise](https://example.invalid)):

- registry.camunda.cloud/keycloak-ee/keycloak:24.0.5
`
	path := filepath.Join(t.TempDir(), "README.md")
	if err := os.WriteFile(path, []byte(readme), 0o644); err != nil {
		t.Fatal(err)
	}
	got := parseReadmeImages(path)
	si, ok := got["11.0.0"]
	if !ok {
		t.Fatalf("no section parsed: %v", got)
	}
	wantStd := []string{
		"docker.io/bitnami/elasticsearch:8.12.2",
		"docker.io/camunda/operate:8.6.0",
		"docker.io/camunda/zeebe:8.6.0",
	}
	if !equalStringSets(si.standard, wantStd) {
		t.Errorf("standard=%v want %v", si.standard, wantStd)
	}
	if !equalStringSets(si.enterprise, []string{"registry.camunda.cloud/keycloak-ee/keycloak:24.0.5"}) {
		t.Errorf("enterprise=%v", si.enterprise)
	}
	for _, img := range si.standard {
		if strings.Contains(img, "example.invalid") || strings.Contains(img, "[") {
			t.Errorf("non-image line leaked: %q", img)
		}
	}
}

func TestHelmCLIAnnotationRe(t *testing.T) {
	chart := "apiVersion: v2\nannotations:\n  camunda.io/helmCLIVersion: \"3.20.2,4.2.3\"\n  other: x\n"
	m := helmCLIAnnotationRe.FindStringSubmatch(chart)
	if m == nil || m[1] != "3.20.2,4.2.3" {
		t.Fatalf("annotation parse=%v", m)
	}
	bare := "annotations:\n  camunda.io/helmCLIVersion: 3.16.1\n"
	if m := helmCLIAnnotationRe.FindStringSubmatch(bare); m == nil || strings.TrimSpace(m[1]) != "3.16.1" {
		t.Fatalf("bare annotation parse=%v", m)
	}
}
