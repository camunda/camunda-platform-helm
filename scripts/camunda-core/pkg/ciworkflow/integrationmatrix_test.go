// Copyright 2026 Camunda Services GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ciworkflow

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func filterFixture(t *testing.T, platforms, flows string) map[string][]map[string]any {
	t.Helper()
	out, err := FilterIntegrationMatrix(filepath.Join("testdata", "test-integration-matrix.yaml"), platforms, flows)
	if err != nil {
		t.Fatalf("FilterIntegrationMatrix: %v", err)
	}
	var m map[string][]map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("output not valid JSON: %v\n%s", err, out)
	}
	return m
}

func TestFilterIntegrationMatrix(t *testing.T) {
	m := filterFixture(t, "gke,eks", "install,upgrade-patch")
	if len(m["distro"]) != 2 {
		t.Errorf("distro = %v, want gke+eks", m["distro"])
	}
	if len(m["scenario"]) != 2 {
		t.Errorf("scenario = %v, want install+upgrade-patch", m["scenario"])
	}
	for _, s := range m["scenario"] {
		flow := s["flow"].(string)
		if flow != "install" && flow != "upgrade-patch" {
			t.Errorf("unexpected flow %q", flow)
		}
	}
}

func TestFilterIntegrationMatrixCaseInsensitive(t *testing.T) {
	m := filterFixture(t, "GKE", "Install")
	if len(m["distro"]) != 1 || m["distro"][0]["platform"] != "gke" {
		t.Errorf("distro = %v, want single gke entry", m["distro"])
	}
	if len(m["scenario"]) != 1 || m["scenario"][0]["flow"] != "install" {
		t.Errorf("scenario = %v, want single install entry", m["scenario"])
	}
}

func TestFilterIntegrationMatrixKeepsSecretShape(t *testing.T) {
	m := filterFixture(t, "rosa", "modular-upgrade-minor")
	if len(m["distro"]) != 1 {
		t.Fatalf("distro = %v, want single rosa entry", m["distro"])
	}
	secret, ok := m["distro"][0]["secret"].(map[string]any)
	if !ok || secret["server-url"] != "DISTRO_CI_OPENSHIFT_CLUSTER_URL" {
		t.Errorf("secret = %v, want openshift secret mapping", m["distro"][0]["secret"])
	}
}

func TestFilterIntegrationMatrixEmptyInputs(t *testing.T) {
	if _, err := FilterIntegrationMatrix(filepath.Join("testdata", "test-integration-matrix.yaml"), "", "install"); err == nil || !strings.Contains(err.Error(), "must not be empty") {
		t.Errorf("empty platforms: err = %v", err)
	}
	if _, err := FilterIntegrationMatrix(filepath.Join("testdata", "test-integration-matrix.yaml"), "gke", ""); err == nil || !strings.Contains(err.Error(), "must not be empty") {
		t.Errorf("empty flows: err = %v", err)
	}
}

func TestCompactJSON(t *testing.T) {
	got, err := CompactJSON(`{ "distro": [ {"platform": "gke"} ] }`)
	if err != nil {
		t.Fatal(err)
	}
	if got != `{"distro":[{"platform":"gke"}]}` {
		t.Errorf("got %s", got)
	}
	if _, err := CompactJSON("{nope"); err == nil {
		t.Error("expected error for invalid JSON")
	}
}
