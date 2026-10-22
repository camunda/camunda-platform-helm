// Copyright 2025 Camunda Services GmbH
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

package matrix

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestLifecycleHook_Unmarshal_Fixtures(t *testing.T) {
	src := `
integration:
  case:
    pr:
      scenario:
        - name: rdbms
          enabled: true
          shortname: rdbms
          auth: keycloak
          flow: install
          platforms: [gke]
          pre-install:
            fixtures:
              - postgresql-cluster.yaml
            description: |
              Provisions CloudNativePG Cluster + auth Secret in scenario namespace.
`
	var cfg CITestConfig
	if err := yaml.Unmarshal([]byte(src), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	scns := cfg.Integration.Case.PR.Scenarios
	if len(scns) != 1 {
		t.Fatalf("scenarios: want 1, got %d", len(scns))
	}
	pi := scns[0].PreInstall
	if pi == nil {
		t.Fatal("pre-install: nil")
	}
	if len(pi.Fixtures) != 1 || pi.Fixtures[0] != "postgresql-cluster.yaml" {
		t.Errorf("fixtures: got %v", pi.Fixtures)
	}
	if pi.Script != "" {
		t.Errorf("script: want empty, got %q", pi.Script)
	}
	if pi.Description == "" {
		t.Error("description: empty")
	}
}

func TestLifecycleHook_Unmarshal_Script(t *testing.T) {
	src := `
integration:
  case:
    pr:
      scenario:
        - name: elasticsearch-self-signed
          enabled: true
          shortname: esss
          auth: keycloak
          flow: install
          platforms: [gke]
          pre-install:
            script: pre-install-elasticsearch-self-signed.sh
            description: Generates self-signed CA + node cert via openssl.
`
	var cfg CITestConfig
	if err := yaml.Unmarshal([]byte(src), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	pi := cfg.Integration.Case.PR.Scenarios[0].PreInstall
	if pi == nil {
		t.Fatal("pre-install: nil")
	}
	if pi.Script != "pre-install-elasticsearch-self-signed.sh" {
		t.Errorf("script: got %q", pi.Script)
	}
	if len(pi.Fixtures) != 0 {
		t.Errorf("fixtures: want empty, got %v", pi.Fixtures)
	}
}

func TestFlowHooks_Unmarshal(t *testing.T) {
	src := `
integration:
  flows:
    upgrade-patch:
      pre-upgrade:
        script: pre-upgrade-patch.sh
        description: Deletes orchestration StatefulSets before patch upgrade.
    upgrade-minor:
      pre-upgrade:
        script: pre-upgrade-minor.sh
        description: Deletes PostgreSQL StatefulSets across minor upgrade.
`
	var cfg CITestConfig
	if err := yaml.Unmarshal([]byte(src), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	flows := cfg.Integration.Flows
	if len(flows) != 2 {
		t.Fatalf("flows: want 2, got %d", len(flows))
	}
	patch := flows["upgrade-patch"]
	if patch == nil || patch.PreUpgrade == nil {
		t.Fatal("upgrade-patch.pre-upgrade: nil")
	}
	if patch.PreUpgrade.Script != "pre-upgrade-patch.sh" {
		t.Errorf("upgrade-patch script: got %q", patch.PreUpgrade.Script)
	}
	minor := flows["upgrade-minor"]
	if minor == nil || minor.PreUpgrade == nil {
		t.Fatal("upgrade-minor.pre-upgrade: nil")
	}
	if minor.PreUpgrade.Script != "pre-upgrade-minor.sh" {
		t.Errorf("upgrade-minor script: got %q", minor.PreUpgrade.Script)
	}
}

// TestGenerate_PropagatesPreInstall checks that every scenario declaring a
// pre-install hook in the registry has that hook faithfully propagated into
// the generated Entry. The test is data-driven: it discovers which scenarios
// declare pre-install hooks at load time, so renaming or removing a scenario
// never requires editing this test.
func TestGenerate_PropagatesPreInstall(t *testing.T) {
	repoRoot := findRepoRoot(t)

	chartDir := filepath.Join(repoRoot, "charts", "camunda-platform-8.10")
	cfg, err := LoadRegistry(chartDir)
	if err != nil {
		t.Fatalf("LoadRegistry: %v", err)
	}

	// Collect all enabled PR scenarios that declare a pre-install hook.
	type wantHook struct {
		scenario string
		hook     *LifecycleHook
	}
	var want []wantHook
	for _, s := range cfg.Integration.Case.PR.Scenarios {
		if !s.Enabled {
			continue
		}
		if s.PreInstall != nil {
			want = append(want, wantHook{scenario: s.Name, hook: s.PreInstall})
		}
	}
	require.NotEmpty(t, want, "no enabled PR scenarios with pre-install hooks in 8.10 registry — registry may have regressed")
	// Invariant: hub-external-db always declares a pre-install hook.
	var hasHubExternalDB bool
	for _, w := range want {
		if w.scenario == "hub-external-db" {
			hasHubExternalDB = true
			break
		}
	}
	require.True(t, hasHubExternalDB, "expected hub-external-db to declare a pre-install hook")

	entries, err := Generate(repoRoot, GenerateOptions{Versions: []string{"8.10"}})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	// Index entries by scenario name for fast lookup.
	byScenario := map[string]*Entry{}
	for i := range entries {
		if entries[i].Version == "8.10" {
			byScenario[entries[i].Scenario] = &entries[i]
		}
	}

	for _, w := range want {
		e := byScenario[w.scenario]
		if e == nil {
			t.Errorf("scenario %q: entry not found in Generate output", w.scenario)
			continue
		}
		if e.PreInstall == nil {
			t.Errorf("scenario %q: PreInstall not propagated (nil in entry)", w.scenario)
			continue
		}
		if e.PreInstall.Description == "" {
			t.Errorf("scenario %q: PreInstall.Description is empty", w.scenario)
		}
		// Script and fixtures must match exactly what the registry declared.
		if e.PreInstall.Script != w.hook.Script {
			t.Errorf("scenario %q: PreInstall.Script: got %q, want %q", w.scenario, e.PreInstall.Script, w.hook.Script)
		}
		if !slices.Equal(e.PreInstall.Fixtures, w.hook.Fixtures) {
			t.Errorf("scenario %q: PreInstall.Fixtures: got %v, want %v", w.scenario, e.PreInstall.Fixtures, w.hook.Fixtures)
		}
	}
}

// TestGenerate_PostgresqlCompanionProfiles guards that every scenario carrying an
// internal-postgresql companion dependency has its values-file, release-name, and
// credential env-vars faithfully propagated into the generated Entry. Expected
// values are derived from the registry dependency declarations at load time, so
// renaming a profile or adding a new postgresql-backed scenario never requires
// editing this test.
func TestGenerate_PostgresqlCompanionProfiles(t *testing.T) {
	repoRoot := findRepoRoot(t)

	chartDir := filepath.Join(repoRoot, "charts", "camunda-platform-8.10")
	cfg, err := LoadRegistry(chartDir)
	if err != nil {
		t.Fatalf("LoadRegistry: %v", err)
	}

	// Collect all enabled PR scenarios whose expanded Dependencies include internal-postgresql.
	type wantDep struct {
		scenario string
		dep      ChartDependency
	}
	var want []wantDep
	for _, s := range cfg.Integration.Case.PR.Scenarios {
		if !s.Enabled {
			continue
		}
		for _, d := range s.Dependencies {
			if d.Chart == "charts/internal-postgresql" {
				want = append(want, wantDep{scenario: s.Name, dep: d})
				break
			}
		}
	}
	require.NotEmpty(t, want, "no enabled PR scenarios with internal-postgresql dependency in 8.10 registry — registry may have regressed")
	// Invariant: elasticsearch always carries an internal-postgresql companion.
	var hasElasticsearch bool
	for _, w := range want {
		if w.scenario == "elasticsearch" {
			hasElasticsearch = true
			break
		}
	}
	require.True(t, hasElasticsearch, "expected elasticsearch scenario to declare an internal-postgresql companion")

	entries, err := Generate(repoRoot, GenerateOptions{Versions: []string{"8.10"}})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	byScenario := map[string]*Entry{}
	for i := range entries {
		if entries[i].Version == "8.10" {
			byScenario[entries[i].Scenario] = &entries[i]
		}
	}

	for _, w := range want {
		e := byScenario[w.scenario]
		if e == nil {
			t.Errorf("scenario %q: entry not found in Generate output", w.scenario)
			continue
		}
		var pg *ChartDependency
		for i := range e.Dependencies {
			if e.Dependencies[i].Chart == "charts/internal-postgresql" {
				pg = &e.Dependencies[i]
				break
			}
		}
		if pg == nil {
			t.Errorf("scenario %q: internal-postgresql dependency not propagated; got %v", w.scenario, e.Dependencies)
			continue
		}
		if pg.ValuesFile != w.dep.ValuesFile {
			t.Errorf("scenario %q: values-file: got %q, want %q", w.scenario, pg.ValuesFile, w.dep.ValuesFile)
		}
		if pg.ReleaseName != w.dep.ReleaseName {
			t.Errorf("scenario %q: release-name: got %q, want %q", w.scenario, pg.ReleaseName, w.dep.ReleaseName)
		}
		for _, ev := range w.dep.EnvVars {
			if !slices.Contains(pg.EnvVars, ev) {
				t.Errorf("scenario %q: env-var %q missing from propagated dependency; got %v", w.scenario, ev, pg.EnvVars)
			}
		}
	}
}

// TestGenerate_DependencyValuesFilesExist guards every companion-chart dependency
// across all 8.10 scenarios: each `values-file` (resolved relative to the repo
// root, matching the matrix runner) must exist on disk. This catches a mistyped
// or stale path in any dependency-profile without needing a live deploy.
func TestGenerate_DependencyValuesFilesExist(t *testing.T) {
	repoRoot := findRepoRoot(t)

	entries, err := Generate(repoRoot, GenerateOptions{Versions: []string{"8.10"}})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	seen := map[string]bool{}
	for _, e := range entries {
		for _, dep := range e.Dependencies {
			if dep.ValuesFile == "" || seen[dep.ValuesFile] {
				continue
			}
			seen[dep.ValuesFile] = true
			path := filepath.Join(repoRoot, dep.ValuesFile)
			if info, err := os.Stat(path); err != nil || info.IsDir() {
				t.Errorf("dependency values-file %q (chart %q): missing or not a file at %s",
					dep.ValuesFile, dep.Chart, path)
			}
		}
	}
}

func TestLifecycleHook_Validate(t *testing.T) {
	tests := []struct {
		name    string
		hook    *LifecycleHook
		wantErr string
	}{
		{
			name: "ok fixtures",
			hook: &LifecycleHook{Fixtures: []string{"a.yaml"}, Description: "x"},
		},
		{
			name: "ok script",
			hook: &LifecycleHook{Script: "pre-install.sh", Description: "x"},
		},
		{
			name:    "empty description",
			hook:    &LifecycleHook{Script: "pre-install.sh", Description: "  "},
			wantErr: "description",
		},
		{
			name:    "both modes",
			hook:    &LifecycleHook{Script: "x.sh", Fixtures: []string{"y.yaml"}, Description: "x"},
			wantErr: "exactly one",
		},
		{
			name:    "neither mode",
			hook:    &LifecycleHook{Description: "x"},
			wantErr: "exactly one",
		},
		{
			name:    "script with relative parent",
			hook:    &LifecycleHook{Script: "../foo.sh", Description: "x"},
			wantErr: "plain filename",
		},
		{
			name:    "script with slash",
			hook:    &LifecycleHook{Script: "sub/foo.sh", Description: "x"},
			wantErr: "plain filename",
		},
		{
			name:    "script literal dot-dot",
			hook:    &LifecycleHook{Script: "..", Description: "x"},
			wantErr: "plain filename",
		},
		{
			name:    "fixture with relative parent",
			hook:    &LifecycleHook{Fixtures: []string{"a.yaml", "../b.yaml"}, Description: "x"},
			wantErr: "plain filename",
		},
		{
			name:    "fixture with backslash",
			hook:    &LifecycleHook{Fixtures: []string{`a\b.yaml`}, Description: "x"},
			wantErr: "plain filename",
		},
		{
			name: "nil receiver is no-op",
			hook: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.hook.Validate("ctx")
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestLifecycleHook_NoNewFields_BackwardsCompatible(t *testing.T) {
	// Existing scenario entries with no pre-install: field must parse fine
	// and produce nil PreInstall.
	src := `
integration:
  case:
    pr:
      scenario:
        - name: legacy
          enabled: true
          shortname: lg
          auth: keycloak
          flow: install
          platforms: [gke]
`
	var cfg CITestConfig
	if err := yaml.Unmarshal([]byte(src), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if cfg.Integration.Case.PR.Scenarios[0].PreInstall != nil {
		t.Errorf("pre-install: want nil for legacy entry")
	}
	if cfg.Integration.Flows != nil {
		t.Errorf("flows: want nil when omitted")
	}
}
