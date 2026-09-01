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

package matrix

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadChartDependencies(t *testing.T) {
	dir := t.TempDir()
	body := "apiVersion: v2\nname: camunda-platform\ndependencies:\n" +
		"  - name: elasticsearch\n    repository: \"file://../elasticsearch-21\"\n    version: 21.6.3\n    condition: \"elasticsearch.enabled\"\n" +
		"  - name: common\n    version: 2.x.x\n"
	if err := os.WriteFile(filepath.Join(dir, "Chart.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	deps, err := readChartDependencies(dir)
	if err != nil {
		t.Fatalf("readChartDependencies: %v", err)
	}
	if len(deps) != 2 {
		t.Fatalf("want 2 dependencies, got %d: %+v", len(deps), deps)
	}
	if deps[0].Name != "elasticsearch" || deps[0].Condition != "elasticsearch.enabled" {
		t.Errorf("dependency 0: got %+v", deps[0])
	}
	if deps[1].Name != "common" || deps[1].Condition != "" {
		t.Errorf("dependency 1: got %+v", deps[1])
	}

	if !hasChartDependency(deps, "elasticsearch") {
		t.Error("hasChartDependency(elasticsearch): want true")
	}
	if hasChartDependency(deps, "opensearch") {
		t.Error("hasChartDependency(opensearch): want false")
	}

	if _, err := readChartDependencies(filepath.Join(dir, "missing")); err == nil {
		t.Error("want error for a chart dir without Chart.yaml")
	}
}

func TestAnyConditionKeyTrue(t *testing.T) {
	doc := map[string]any{
		"elasticsearch": map[string]any{"enabled": true},
		"opensearch":    map[string]any{"enabled": false},
		"global":        map[string]any{"identity": map[string]any{"auth": map[string]any{"enabled": true}}},
		"scalar":        "not-a-map",
		"stringy":       map[string]any{"enabled": "true"},
	}

	for _, tc := range []struct {
		condition string
		want      bool
	}{
		{"elasticsearch.enabled", true},
		{"opensearch.enabled", false},
		{"global.identity.auth.enabled", true},
		{"opensearch.enabled,elasticsearch.enabled", true},
		{"opensearch.enabled, missing.enabled", false},
		{"missing.enabled", false},
		{"scalar.enabled", false},
		{"stringy.enabled", false},
		{"elasticsearch", false},
		{"", false},
	} {
		if got := anyConditionKeyTrue(doc, tc.condition); got != tc.want {
			t.Errorf("anyConditionKeyTrue(%q) = %v, want %v", tc.condition, got, tc.want)
		}
	}
}

func TestReadYAMLMap(t *testing.T) {
	dir := t.TempDir()
	empty := filepath.Join(dir, "empty.yaml")
	if err := os.WriteFile(empty, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	doc, err := readYAMLMap(empty)
	if err != nil {
		t.Fatalf("readYAMLMap(empty): %v", err)
	}
	if doc == nil {
		t.Fatal("want an empty map, got nil")
	}
	if anyConditionKeyTrue(doc, "elasticsearch.enabled") {
		t.Error("empty values file must not enable anything")
	}

	if _, err := readYAMLMap(filepath.Join(dir, "missing.yaml")); err == nil {
		t.Error("want error for a missing values file")
	}
}
