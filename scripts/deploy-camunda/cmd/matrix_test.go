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

	"scripts/camunda-core/pkg/logging"
	"scripts/deploy-camunda/config"
	"scripts/deploy-camunda/matrix"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestRootScenarioRegistryResolution(t *testing.T) {
	previousFlags, previousConfig, previousLogger := flags, configFile, logging.Logger
	t.Cleanup(func() { flags, configFile, logging.Logger = previousFlags, previousConfig, previousLogger })
	flags = config.RuntimeFlags{}
	chart, err := filepath.Abs("../../../charts/camunda-platform-8.10")
	require.NoError(t, err)
	directory := t.TempDir()
	configPath := filepath.Join(directory, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte("features: [documentstore]\nqa: true\n"), 0644))
	command := NewRootCommand()
	command.SetArgs([]string{
		"--config", configPath, "--env-file", filepath.Join(directory, ".env"),
		"--chart-path", chart, "--namespace", "test", "--release", "integration",
		"--scenario", "optimize-tls", "--platform", "gke", "--features=", "--qa=false", "--log-level", "error",
	})
	called := false
	command.RunE = func(*cobra.Command, []string) error {
		called = true
		require.True(t, flags.SelectionResolved)
		require.Empty(t, flags.Selection.Features)
		require.False(t, flags.Selection.QA)
		require.Len(t, flags.CompanionCharts, 3)
		require.Len(t, flags.PreInstallHooks, 1)
		return nil
	}
	require.NoError(t, command.Execute())
	require.True(t, called)
}

func TestRootDeploymentSetupIsNotInherited(t *testing.T) {
	previousFlags, previousConfig, previousLogger := flags, configFile, logging.Logger
	t.Cleanup(func() { flags, configFile, logging.Logger = previousFlags, previousConfig, previousLogger })
	configPath := filepath.Join(t.TempDir(), "invalid.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte("invalid: ["), 0644))

	for _, args := range [][]string{nil, {"utility"}, {"group", "leaf"}} {
		t.Run(strings.Join(append([]string{"root"}, args...), "/"), func(t *testing.T) {
			flags = config.RuntimeFlags{}
			command := NewRootCommand()
			called := false
			run := func(*cobra.Command, []string) error { called = true; return nil }
			command.RunE = run
			command.AddCommand(&cobra.Command{Use: "utility", RunE: run})
			group := &cobra.Command{Use: "group"}
			group.AddCommand(&cobra.Command{Use: "leaf", RunE: run})
			command.AddCommand(group)
			command.SetArgs(append([]string{"--config", configPath}, args...))
			err := command.Execute()
			if len(args) == 0 {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, len(args) > 0, called)
		})
	}
}

func TestMatrixRunOptionsAndConfigPrecedence(t *testing.T) {
	previousConfig, previousLogger := configFile, logging.Logger
	t.Cleanup(func() { configFile, logging.Logger = previousConfig, previousLogger })
	directory := t.TempDir()
	configFile = filepath.Join(directory, "config.yaml")
	require.NoError(t, os.WriteFile(configFile, []byte("matrix:\n  helmTimeout: 99\n  testE2E: true\n  scenarioFilter: configured\n  keycloakHost: configured\n"), 0644))

	var run matrixRunFlags
	command := run.command()
	require.NoError(t, command.ParseFlags([]string{
		"--repo-root", directory, "--env-file", os.DevNull, "--platform=eks", "--log-level=error",
		"--timeout=17", "--test-e2e=false", "--scenario-filter=", "--keycloak-host=",
		"--delete-namespace", "--wait-ingress-ready", "--ingress-ready-timeout=23",
		"--extra-values=first,comma.yaml", "--extra-values=last.yaml",
		"--image-override=orchestration=example/camunda:test", "--allow-digest-shadow",
		"--extra-helm-set=a=b,c=d", "--extra-helm-arg=--atomic",
		"--kube-context-gke=gke-context", "--env-file-8.9=version.env", "--use-vault-backed-secrets-eks",
	}))
	require.Equal(t, "stringArray", command.Flags().Lookup("extra-values").Value.Type())
	require.NoError(t, run.configure(command))
	require.Equal(t, 17, run.HelmTimeout)
	require.False(t, run.TestE2E)
	require.Empty(t, run.filter.ScenarioFilter)
	require.Empty(t, run.KeycloakHost)
	require.Equal(t, "eks", run.Platform)
	require.Equal(t, "matrix", run.NamespacePrefix)
	require.Equal(t, 1, run.MaxParallel)
	require.True(t, run.DeleteNamespaceFirst)
	require.True(t, run.WaitIngressReady)
	require.Equal(t, 23, run.IngressReadyTimeoutMinutes)
	require.Equal(t, []string{"first,comma.yaml", "last.yaml"}, run.ExtraValues)
	require.Equal(t, []string{"orchestration=example/camunda:test"}, run.ImageOverrides)
	require.True(t, run.AllowDigestShadow)
	require.Equal(t, []string{"a=b", "c=d"}, run.ExtraHelmSets)
	require.Equal(t, []string{"--atomic"}, run.ExtraHelmArgs)
	require.Equal(t, "gke-context", run.Infra.KubeContexts["gke"])
	require.Equal(t, "version.env", run.Infra.EnvFiles["8.9"])
	require.True(t, run.Infra.VaultBackedSecrets["eks"])
	base := run.RunOptions
	perRelease := synthesizeReleaseOpts(base, "gke", "release-ns")
	perRelease.Platform, perRelease.NamespaceOverride = base.Platform, base.NamespaceOverride
	require.Equal(t, base, perRelease)
}

func TestMatrixCLIInfraKeepsEmptyFlags(t *testing.T) {
	command := newMatrixRunCommand()
	require.NoError(t, command.ParseFlags([]string{"--platform=", "--kube-context=", "--kube-context-gke", "gke-ctx"}))
	rc := &config.RootConfig{Matrix: config.MatrixConfig{InfraConfig: config.InfraConfig{Platform: "eks", KubeContext: "matrix-ctx", RepoRoot: "/repo"}}}
	cli := matrixCLIInfra(command.Flags())
	require.Equal(t, config.InfraConfig{RepoRoot: "/repo"}, rc.ResolveInfra(true, "eks", "", cli))
	require.Equal(t, "gke-ctx", rc.ResolveInfra(true, "gke", "", cli).KubeContext)
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

func TestTopologyPlaywrightProject(t *testing.T) {
	tests := []struct {
		name string
		leg  matrix.TopologyE2ELeg
		want string
	}{
		{
			name: "uses the topology project by default",
			leg:  matrix.TopologyE2ELeg{PlaywrightProject: "topology-orchestration"},
			want: "topology-orchestration",
		},
		{
			name: "uses the dependent additional project when configured",
			leg: matrix.TopologyE2ELeg{
				PlaywrightProject:           "topology-orchestration",
				AdditionalPlaywrightProject: "physical-tenants",
			},
			want: "physical-tenants",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := topologyPlaywrightProject(tt.leg); got != tt.want {
				t.Fatalf("topologyPlaywrightProject() = %q, want %q", got, tt.want)
			}
		})
	}
}
