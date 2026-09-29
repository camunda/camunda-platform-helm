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

package helm

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDetectWaitFlagFromVersionOutput(t *testing.T) {
	tests := []struct {
		name string
		out  string
		want string
	}{
		{name: "helm v3 short", out: "v3.20.1+g8a36c9b\n", want: "--wait"},
		{name: "helm v4 short", out: "v4.1.4+g05fa379\n", want: "--wait=legacy"},
		{name: "leading whitespace", out: "  v4.0.0\n", want: "--wait=legacy"},
		{name: "empty falls back to wait", out: "", want: "--wait"},
		{name: "garbage falls back to wait", out: "not a version\n", want: "--wait"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := waitFlagFromOutput([]byte(tt.out)); got != tt.want {
				t.Errorf("waitFlagFromOutput(%q) = %q, want %q", tt.out, got, tt.want)
			}
		})
	}
}

func TestMissingVendoredDependencies(t *testing.T) {
	const chartYAML = `apiVersion: v2
name: camunda-platform
dependencies:
  - name: common
    repository: file://../common
  - name: keycloak
    repository: oci://registry-1.docker.io/bitnamicharts
  - name: elasticsearch
    repository: oci://registry-1.docker.io/bitnamicharts
`

	tests := []struct {
		name   string
		vendor func(t *testing.T, chartPath string)
		want   []string
	}{
		{
			name:   "nothing vendored reports every dependency",
			vendor: func(t *testing.T, chartPath string) {},
			want:   []string{"common", "keycloak", "elasticsearch"},
		},
		{
			name: "expanded directories and packages both count as vendored",
			vendor: func(t *testing.T, chartPath string) {
				mkdir(t, filepath.Join(chartPath, "charts", "common"))
				writeFile(t, filepath.Join(chartPath, "charts", "keycloak-24.4.3.tgz"), "")
				writeFile(t, filepath.Join(chartPath, "charts", "elasticsearch-21.3.14.tgz"), "")
			},
			want: nil,
		},
		{
			name: "a partially vendored chart reports only what is absent",
			vendor: func(t *testing.T, chartPath string) {
				mkdir(t, filepath.Join(chartPath, "charts", "common"))
				writeFile(t, filepath.Join(chartPath, "charts", "keycloak-24.4.3.tgz"), "")
			},
			want: []string{"elasticsearch"},
		},
		{
			name: "a file named after a dependency is not a vendored subchart",
			vendor: func(t *testing.T, chartPath string) {
				mkdir(t, filepath.Join(chartPath, "charts"))
				writeFile(t, filepath.Join(chartPath, "charts", "common"), "")
				mkdir(t, filepath.Join(chartPath, "charts", "keycloak"))
				mkdir(t, filepath.Join(chartPath, "charts", "elasticsearch"))
			},
			want: []string{"common"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chartPath := t.TempDir()
			writeFile(t, filepath.Join(chartPath, "Chart.yaml"), chartYAML)
			tt.vendor(t, chartPath)

			got, err := MissingVendoredDependencies(chartPath)
			if err != nil {
				t.Fatalf("MissingVendoredDependencies: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MissingVendoredDependencies() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMissingVendoredDependenciesWithoutDependencies(t *testing.T) {
	chartPath := t.TempDir()
	writeFile(t, filepath.Join(chartPath, "Chart.yaml"), "apiVersion: v2\nname: standalone\n")

	got, err := MissingVendoredDependencies(chartPath)
	if err != nil {
		t.Fatalf("MissingVendoredDependencies: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("MissingVendoredDependencies() = %v, want none", got)
	}
}

func TestMissingVendoredDependenciesWithoutChartYAML(t *testing.T) {
	if _, err := MissingVendoredDependencies(t.TempDir()); err == nil {
		t.Fatal("MissingVendoredDependencies() = nil error, want a read failure")
	}
}

func mkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("MkdirAll %s: %v", path, err)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	mkdir(t, filepath.Dir(path))
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile %s: %v", path, err)
	}
}
