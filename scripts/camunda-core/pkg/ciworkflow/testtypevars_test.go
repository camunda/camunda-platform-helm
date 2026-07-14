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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"scripts/camunda-core/pkg/ghactions"
)

const sampleConfig = `unit:
  enabled: true
  matrix:
    - name: Management
      packages: identity common
    - name: Orchestration
      packages: orchestration connectors optimize
    - name: Design
      packages: web-modeler
`

// writeConfig creates repoRoot/charts/<chartDir>/test/ci-test-config.yaml and,
// when digest is true, an empty values-digest.yaml next to the chart.
func writeConfig(t *testing.T, chartDir, body string, digest bool) string {
	t.Helper()
	root := t.TempDir()
	chartTestDir := filepath.Join(root, "charts", chartDir, "test")
	if err := os.MkdirAll(chartTestDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(chartTestDir, "ci-test-config.yaml"), []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if digest {
		if err := os.WriteFile(filepath.Join(root, "charts", chartDir, "values-digest.yaml"), []byte("{}\n"), 0o644); err != nil {
			t.Fatalf("write digest: %v", err)
		}
	}
	return root
}

func TestComputeChartPath(t *testing.T) {
	tests := []struct {
		name        string
		in          TestTypeVarsInput
		wantPath    string
		wantAbsPath string
	}{
		{
			name:        "install flow uses current chart",
			in:          TestTypeVarsInput{ChartDir: "camunda-platform-8.10", Flow: "install", GitHubWorkspace: "/ws"},
			wantPath:    "charts/camunda-platform-8.10",
			wantAbsPath: "/ws/charts/camunda-platform-8.10",
		},
		{
			name:        "upgrade-minor install step uses previous chart",
			in:          TestTypeVarsInput{ChartDir: "camunda-platform-8.10", Flow: "upgrade-minor", CamundaVersionPrevious: "8.9", UpgradeStep: false, GitHubWorkspace: "/ws"},
			wantPath:    "charts/camunda-platform-8.9",
			wantAbsPath: "/ws/charts/camunda-platform-8.9",
		},
		{
			name:        "upgrade-minor upgrade step uses current chart",
			in:          TestTypeVarsInput{ChartDir: "camunda-platform-8.10", Flow: "upgrade-minor", CamundaVersionPrevious: "8.9", UpgradeStep: true, GitHubWorkspace: "/ws"},
			wantPath:    "charts/camunda-platform-8.10",
			wantAbsPath: "/ws/charts/camunda-platform-8.10",
		},
		{
			name:        "upgrade-minor without previous falls back to current chart",
			in:          TestTypeVarsInput{ChartDir: "camunda-platform-8.10", Flow: "upgrade-minor", CamundaVersionPrevious: "", GitHubWorkspace: "/ws"},
			wantPath:    "charts/camunda-platform-8.10",
			wantAbsPath: "/ws/charts/camunda-platform-8.10",
		},
		{
			name:        "upgrade-patch uses current chart",
			in:          TestTypeVarsInput{ChartDir: "camunda-platform-8.10", Flow: "upgrade-patch", CamundaVersionPrevious: "8.9", GitHubWorkspace: "/ws"},
			wantPath:    "charts/camunda-platform-8.10",
			wantAbsPath: "/ws/charts/camunda-platform-8.10",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.in.RepoRoot = writeConfig(t, tc.in.ChartDir, sampleConfig, false)
			got, err := Compute(tc.in)
			if err != nil {
				t.Fatalf("Compute: %v", err)
			}
			if got.ChartPath != tc.wantPath {
				t.Errorf("ChartPath = %q, want %q", got.ChartPath, tc.wantPath)
			}
			if got.AbsoluteTestChartDir != tc.wantAbsPath {
				t.Errorf("AbsoluteTestChartDir = %q, want %q", got.AbsoluteTestChartDir, tc.wantAbsPath)
			}
		})
	}
}

func TestComputeEnterpriseArgs(t *testing.T) {
	root := writeConfig(t, "camunda-platform-8.10", sampleConfig, false)

	off, err := Compute(TestTypeVarsInput{ChartDir: "camunda-platform-8.10", Flow: "install", RepoRoot: root})
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if off.TestHelmExtraArgs != "" {
		t.Errorf("TestHelmExtraArgs = %q, want empty when disabled", off.TestHelmExtraArgs)
	}

	on, err := Compute(TestTypeVarsInput{ChartDir: "camunda-platform-8.10", Flow: "install", ValuesEnterprise: true, RepoRoot: root})
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	want := "--values ../../../../charts/camunda-platform-8.10/values-enterprise.yaml"
	if on.TestHelmExtraArgs != want {
		t.Errorf("TestHelmExtraArgs = %q, want %q", on.TestHelmExtraArgs, want)
	}
}

func TestComputeDigestGate(t *testing.T) {
	// File present + enabled -> set.
	rootWith := writeConfig(t, "camunda-platform-8.10", sampleConfig, true)
	withFile, err := Compute(TestTypeVarsInput{ChartDir: "camunda-platform-8.10", Flow: "install", ValuesDigest: true, RepoRoot: rootWith})
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	want := "../../../../charts/camunda-platform-8.10/values-digest.yaml"
	if withFile.TestHelmDigestValues != want {
		t.Errorf("TestHelmDigestValues = %q, want %q", withFile.TestHelmDigestValues, want)
	}

	// Enabled but file absent -> empty.
	rootWithout := writeConfig(t, "camunda-platform-8.10", sampleConfig, false)
	noFile, err := Compute(TestTypeVarsInput{ChartDir: "camunda-platform-8.10", Flow: "install", ValuesDigest: true, RepoRoot: rootWithout})
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if noFile.TestHelmDigestValues != "" {
		t.Errorf("TestHelmDigestValues = %q, want empty when file missing", noFile.TestHelmDigestValues)
	}

	// File present but disabled -> empty.
	disabled, err := Compute(TestTypeVarsInput{ChartDir: "camunda-platform-8.10", Flow: "install", ValuesDigest: false, RepoRoot: rootWith})
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if disabled.TestHelmDigestValues != "" {
		t.Errorf("TestHelmDigestValues = %q, want empty when disabled", disabled.TestHelmDigestValues)
	}
}

func TestComputeUnitBlock(t *testing.T) {
	root := writeConfig(t, "camunda-platform-8.10", sampleConfig, false)
	got, err := Compute(TestTypeVarsInput{ChartDir: "camunda-platform-8.10", Flow: "install", RepoRoot: root})
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if !got.UnitEnabled {
		t.Errorf("UnitEnabled = false, want true")
	}
	if len(got.UnitMatrix) != 3 {
		t.Fatalf("UnitMatrix len = %d, want 3", len(got.UnitMatrix))
	}
	gotJSON, err := got.UnitMatrixJSON()
	if err != nil {
		t.Fatalf("UnitMatrixJSON: %v", err)
	}
	// Must match the compact form previously produced by
	// `yq --indent=0 --output-format json`.
	wantJSON := `[{"name":"Management","packages":"identity common"},` +
		`{"name":"Orchestration","packages":"orchestration connectors optimize"},` +
		`{"name":"Design","packages":"web-modeler"}]`
	if gotJSON != wantJSON {
		t.Errorf("UnitMatrixJSON =\n  %s\nwant\n  %s", gotJSON, wantJSON)
	}
}

func TestComputeMissingConfig(t *testing.T) {
	root := t.TempDir()
	_, err := Compute(TestTypeVarsInput{ChartDir: "camunda-platform-8.10", Flow: "install", RepoRoot: root})
	if err == nil {
		t.Fatalf("Compute: want error for missing config, got nil")
	}
}

func TestEmit(t *testing.T) {
	root := writeConfig(t, "camunda-platform-8.10", sampleConfig, true)
	v, err := Compute(TestTypeVarsInput{
		ChartDir:              "camunda-platform-8.10",
		Flow:                  "install",
		ValuesEnterprise:      true,
		ValuesDigest:          true,
		GitHubWorkspace:       "/ws",
		KeycloakClientsSecret: "s3cr3t",
		RepoRoot:              root,
	})
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}

	envFile := filepath.Join(t.TempDir(), "env")
	outFile := filepath.Join(t.TempDir(), "out")
	if err := v.Emit(&ghactions.Writer{Path: envFile}, &ghactions.Writer{Path: outFile}); err != nil {
		t.Fatalf("Emit: %v", err)
	}

	env := readFile(t, envFile)
	out := readFile(t, outFile)

	wantEnv := []string{
		"CHART_PATH=charts/camunda-platform-8.10",
		"ABSOLUTE_TEST_CHART_DIR=/ws/charts/camunda-platform-8.10",
		"TEST_HELM_EXTRA_ARGS=--values ../../../../charts/camunda-platform-8.10/values-enterprise.yaml",
		"TEST_HELM_DIGEST_VALUES=../../../../charts/camunda-platform-8.10/values-digest.yaml",
		"DISTRO_QA_E2E_TESTS_KEYCLOAK_CLIENTS_SECRET=s3cr3t",
	}
	for _, line := range wantEnv {
		if !strings.Contains(env, line+"\n") {
			t.Errorf("env missing line %q\n--- env ---\n%s", line, env)
		}
	}
	if !strings.Contains(out, "unit-enabled=true\n") {
		t.Errorf("out missing unit-enabled\n--- out ---\n%s", out)
	}
	if !strings.Contains(out, `unit-matrix=[{"name":"Management"`) {
		t.Errorf("out missing unit-matrix\n--- out ---\n%s", out)
	}
}

func TestEmitOmitsEmptyConditionalVars(t *testing.T) {
	root := writeConfig(t, "camunda-platform-8.10", sampleConfig, false)
	v, err := Compute(TestTypeVarsInput{ChartDir: "camunda-platform-8.10", Flow: "install", RepoRoot: root})
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	envFile := filepath.Join(t.TempDir(), "env")
	if err := v.Emit(&ghactions.Writer{Path: envFile}, &ghactions.Writer{Path: filepath.Join(t.TempDir(), "out")}); err != nil {
		t.Fatalf("Emit: %v", err)
	}
	env := readFile(t, envFile)
	if strings.Contains(env, "TEST_HELM_EXTRA_ARGS=") {
		t.Errorf("env should not contain TEST_HELM_EXTRA_ARGS when disabled\n%s", env)
	}
	if strings.Contains(env, "TEST_HELM_DIGEST_VALUES=") {
		t.Errorf("env should not contain TEST_HELM_DIGEST_VALUES when disabled\n%s", env)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}
