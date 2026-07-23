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

package cmd

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestCIIntegrationMatrixWritesFilteredOutput(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "matrix.yaml")
	writeFile(t, configPath, `matrix:
  distro:
    - platform: gke
      type: kubernetes
    - platform: eks
      type: kubernetes
  scenario:
    - flow: install
      name: Install
    - flow: upgrade-patch
      name: Upgrade
`)
	outputPath := filepath.Join(t.TempDir(), "github_output")
	t.Setenv("GITHUB_OUTPUT", outputPath)

	command := newCIIntegrationMatrixCommand()
	command.SetArgs([]string{
		"--config", configPath,
		"--platforms", "GKE",
		"--flows", "Install",
	})
	if err := command.Execute(); err != nil {
		t.Fatalf("execute integration-matrix: %v", err)
	}

	line := strings.TrimSpace(readFile(t, outputPath))
	matrixJSON, ok := strings.CutPrefix(line, "matrix=")
	if !ok {
		t.Fatalf("GITHUB_OUTPUT missing matrix assignment: %q", line)
	}
	var matrix struct {
		Distro   []map[string]any `json:"distro"`
		Scenario []map[string]any `json:"scenario"`
	}
	if err := json.Unmarshal([]byte(matrixJSON), &matrix); err != nil {
		t.Fatalf("decode matrix output: %v", err)
	}
	if len(matrix.Distro) != 1 || matrix.Distro[0]["platform"] != "gke" {
		t.Errorf("distro = %v, want only gke", matrix.Distro)
	}
	if len(matrix.Scenario) != 1 || matrix.Scenario[0]["flow"] != "install" {
		t.Errorf("scenario = %v, want only install", matrix.Scenario)
	}
}
