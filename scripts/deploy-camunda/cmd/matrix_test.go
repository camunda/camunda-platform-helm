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

package cmd

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"scripts/deploy-camunda/matrix"
)

// Pins inc-5975: --extra-values must exist on `matrix run` so that
// flags.Deployment.ExtraValues — the only input to the digest-overlay strip —
// gets populated. StringArray (not StringSlice) so paths aren't comma-split.
func TestMatrixRunExtraValuesFlag(t *testing.T) {
	flag := newMatrixRunCommand().Flags().Lookup("extra-values")
	if flag == nil {
		t.Fatal("--extra-values flag missing")
	}
	if got := flag.Value.Type(); got != "stringArray" {
		t.Fatalf("flag type = %q, want stringArray", got)
	}
	for _, v := range []string{"/tmp/a.yaml", "/tmp/b.yaml"} {
		if err := flag.Value.Set(v); err != nil {
			t.Fatalf("set %s: %v", v, err)
		}
	}
	if got := flag.Value.String(); !strings.Contains(got, "/tmp/a.yaml") || !strings.Contains(got, "/tmp/b.yaml") {
		t.Errorf("aggregated value %q missing entries", got)
	}
}

func TestMatrixRunDisabledScenarioHint(t *testing.T) {
	repoRoot, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	previousConfigFile := configFile
	configFile = filepath.Join(t.TempDir(), "absent.yaml")
	t.Cleanup(func() { configFile = previousConfigFile })

	for _, testCase := range []struct {
		name    string
		version string
		args    []string
		wantErr string
	}{
		{name: "explicit version without opt-in", version: "8.6", wantErr: "matching scenarios are disabled; re-run with --include-disabled"},
		{name: "matching filtered entry", version: "8.6", args: []string{"--shortname-filter", "es", "--shortname-exact", "--flow-filter", "install", "--platform", "gke"}, wantErr: "matching scenarios are disabled; re-run with --include-disabled"},
		{name: "unknown shortname", version: "8.6", args: []string{"--shortname-filter", "missing"}, wantErr: "no matrix entries matched the filters"},
		{name: "unknown scenario", version: "8.6", args: []string{"--scenario-filter", "missing"}, wantErr: "no matrix entries matched the filters"},
		{name: "denied flow", version: "8.6", args: []string{"--flow-filter", "upgrade-minor"}, wantErr: "no matrix entries matched the filters"},
		{name: "unsupported platform", version: "8.6", args: []string{"--shortname-filter", "es", "--platform", "rosa"}, wantErr: "no matrix entries matched the filters"},
		{name: "opt-in with unmatched filter", version: "8.6", args: []string{"--include-disabled", "--shortname-filter", "missing"}, wantErr: "no matrix entries matched the filters"},
		{name: "opt-in succeeds", version: "8.6", args: []string{"--include-disabled", "--shortname-filter", "es", "--shortname-exact", "--flow-filter", "install", "--platform", "gke"}},
		{name: "enabled selection succeeds", version: "8.9", args: []string{"--shortname-filter", "eske", "--shortname-exact", "--flow-filter", "install", "--platform", "gke"}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			command := newMatrixRunCommand()
			command.SilenceErrors = true
			command.SilenceUsage = true
			command.SetOut(io.Discard)
			command.SetErr(io.Discard)
			command.SetArgs(append([]string{
				"--repo-root", repoRoot,
				"--env-file", os.DevNull,
				"--versions", testCase.version,
				"--coverage",
			}, testCase.args...))
			err := command.Execute()
			if testCase.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), testCase.wantErr) {
				t.Fatalf("error = %v, want %q", err, testCase.wantErr)
			}
			if !strings.Contains(testCase.wantErr, "--include-disabled") && strings.Contains(err.Error(), "--include-disabled") {
				t.Errorf("unmatched filters produced misleading opt-in hint: %v", err)
			}
		})
	}
}

func TestValidateNamespaceOverride(t *testing.T) {
	tests := []struct {
		name              string
		namespaceOverride string
		namespacePrepared bool
		readOnly          bool
		githubActions     string
		wantErr           string
	}{
		{name: "computed namespace"},
		{name: "local override rejected", namespaceOverride: "existing", wantErr: "replace --namespace-override NAME with --namespace-prefix NAME"},
		{name: "local prepared override", namespaceOverride: "existing", namespacePrepared: true},
		{name: "dry run override", namespaceOverride: "existing", readOnly: true},
		{name: "GitHub Actions override", namespaceOverride: "existing", githubActions: "true"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateNamespaceOverride(tt.namespaceOverride, tt.namespacePrepared, tt.readOnly, tt.githubActions)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want substring %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateChartRefFlags(t *testing.T) {
	tests := []struct {
		name            string
		chartRef        string
		chartRefVersion string
		wantErr         string // substring; empty = expect success
	}{
		{
			name:    "both empty is allowed",
			wantErr: "",
		},
		{
			name:            "version without ref is rejected",
			chartRefVersion: "13-rc-latest",
			wantErr:         "--chart-version requires --chart-ref",
		},
		{
			name:            "OCI ref with version is allowed",
			chartRef:        "oci://registry.camunda.cloud/team-distribution/camunda-platform",
			chartRefVersion: "13-rc-latest",
		},
		{
			name:     "OCI ref without version is rejected",
			chartRef: "oci://registry.camunda.cloud/team-distribution/camunda-platform",
			wantErr:  "--chart-version is required when --chart-ref is an OCI reference",
		},
		{
			name:     "tgz ref without version is allowed",
			chartRef: "/tmp/camunda-platform-13.4.0-rc.tgz",
		},
		{
			name:            "tgz ref with version is allowed",
			chartRef:        "/tmp/camunda-platform-13.4.0-rc.tgz",
			chartRefVersion: "13.4.0-rc",
		},
		{
			name:     "directory ref is rejected",
			chartRef: "/tmp/camunda-platform-8.9",
			wantErr:  "must be an OCI reference (oci://...) or a packaged chart (.tgz)",
		},
		{
			name:     "bare chart name is rejected",
			chartRef: "camunda/camunda-platform",
			wantErr:  "must be an OCI reference (oci://...) or a packaged chart (.tgz)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateChartRefFlags(tt.chartRef, tt.chartRefVersion)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestValidateChartRefVersionSpan(t *testing.T) {
	entriesFor := func(versions ...string) []matrix.Entry {
		out := make([]matrix.Entry, 0, len(versions))
		for _, v := range versions {
			out = append(out, matrix.Entry{Version: v})
		}
		return out
	}

	tests := []struct {
		name     string
		chartRef string
		entries  []matrix.Entry
		wantErr  string // substring; empty = expect success
	}{
		{
			name:    "no chart-ref allows any version span",
			entries: entriesFor("8.8", "8.9", "8.10"),
		},
		{
			name:     "single version single entry is allowed",
			chartRef: "oci://registry.camunda.cloud/team-distribution/camunda-platform",
			entries:  entriesFor("8.10"),
		},
		{
			name:     "single version multiple entries is allowed",
			chartRef: "oci://registry.camunda.cloud/team-distribution/camunda-platform",
			entries:  entriesFor("8.10", "8.10", "8.10"),
		},
		{
			name:     "multiple versions are rejected",
			chartRef: "oci://registry.camunda.cloud/team-distribution/camunda-platform",
			entries:  entriesFor("8.8", "8.9"),
			wantErr:  "spans 2 versions (8.8, 8.9)",
		},
		{
			name:     "empty entries with chart-ref does not panic",
			chartRef: "oci://registry.camunda.cloud/team-distribution/camunda-platform",
			entries:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateChartRefVersionSpan(tt.chartRef, tt.entries)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}
