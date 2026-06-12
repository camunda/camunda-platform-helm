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

// TestUpdateMatrixRequiresInputMode pins that exactly one of --chart-yaml /
// --chart is required.
func TestUpdateMatrixRequiresInputMode(t *testing.T) {
	dir := t.TempDir()
	matrixFile := filepath.Join(dir, "version-matrix.json")

	cases := map[string][]string{
		"neither": {"--chart-version", "15.0.0", "--matrix-file", matrixFile},
		"both": {
			"--chart-yaml", filepath.Join(dir, "Chart.yaml"),
			"--chart", dir,
			"--chart-version", "15.0.0",
			"--matrix-file", matrixFile,
		},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			err := runUpdateMatrix(args)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), "--chart-yaml") || !strings.Contains(err.Error(), "--chart") {
				t.Errorf("error should reference both flags, got: %v", err)
			}
		})
	}
}
