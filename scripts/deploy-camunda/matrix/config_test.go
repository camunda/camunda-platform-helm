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
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

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
	if len(want) == 0 {
		t.Skip("no PR scenarios with pre-install hooks in 8.10 registry")
	}

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
		// Fixtures and Script must match what the registry declared.
		if w.hook.Script != "" && e.PreInstall.Script != w.hook.Script {
			t.Errorf("scenario %q: PreInstall.Script: got %q, want %q", w.scenario, e.PreInstall.Script, w.hook.Script)
		}
		if len(w.hook.Fixtures) > 0 && len(e.PreInstall.Fixtures) != len(w.hook.Fixtures) {
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
	if len(want) == 0 {
		t.Skip("no PR scenarios with internal-postgresql dependency in 8.10 registry")
	}

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

// profileTestConfig is a representative config exercising dependency profiles:
// two companion profiles plus a fixtures-only cnpg profile.
const profileTestConfig = `
integration:
  dependency-profiles:
    keycloak:
      dependencies:
        - chart: charts/internal-keycloak-26
          release-name: keycloak
          values-file: test/integration/companion-values/keycloak.yaml
    elasticsearch:
      dependencies:
        - chart: elastic/elasticsearch
          version: "8.5.1"
          release-name: elasticsearch
          repo-name: elastic
          repo-url: https://helm.elastic.co
          values-file: test/integration/companion-values/elasticsearch.yaml
    cnpg:
      pre-install:
        fixtures: [postgresql-cluster.yaml]
        description: |
          Provisions a CloudNativePG Cluster + auth Secret.
  case:
    pr:
      scenario:
        - name: %s
`

func loadAndResolve(t *testing.T, src string) *CITestConfig {
	t.Helper()
	var cfg CITestConfig
	if err := yaml.Unmarshal([]byte(src), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if err := ResolveProfiles(&cfg); err != nil {
		t.Fatalf("ResolveProfiles: %v", err)
	}
	return &cfg
}

func TestResolveProfiles_ExpandsDependenciesInOrder(t *testing.T) {
	scenario := `elasticsearch
          enabled: true
          shortname: eske
          profiles: [keycloak, elasticsearch, cnpg]`
	cfg := loadAndResolve(t, fmt.Sprintf(profileTestConfig, scenario))

	deps := cfg.Integration.Case.PR.Scenarios[0].Dependencies
	gotCharts := make([]string, len(deps))
	for i, d := range deps {
		gotCharts[i] = d.Chart
	}
	want := []string{"charts/internal-keycloak-26", "elastic/elasticsearch"}
	if len(gotCharts) != len(want) {
		t.Fatalf("dependencies: got %v, want %v", gotCharts, want)
	}
	for i := range want {
		if gotCharts[i] != want[i] {
			t.Errorf("dependency[%d] chart: got %q, want %q", i, gotCharts[i], want[i])
		}
	}
	// The remote ES dependency keeps its repo metadata after expansion.
	if deps[1].RepoURL != "https://helm.elastic.co" || deps[1].Version != "8.5.1" {
		t.Errorf("es dependency lost metadata: %+v", deps[1])
	}
	// cnpg profile contributed the pre-install fixture.
	pi := cfg.Integration.Case.PR.Scenarios[0].PreInstall
	if pi == nil || len(pi.Fixtures) != 1 || pi.Fixtures[0] != "postgresql-cluster.yaml" {
		t.Fatalf("pre-install fixtures: got %+v", pi)
	}
}

func TestResolveProfiles_UnknownProfileErrors(t *testing.T) {
	scenario := `bad
          enabled: true
          shortname: bad
          profiles: [keycloak, nope]`
	var cfg CITestConfig
	if err := yaml.Unmarshal([]byte(fmt.Sprintf(profileTestConfig, scenario)), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	err := ResolveProfiles(&cfg)
	if err == nil {
		t.Fatal("expected error for unknown profile, got nil")
	}
	if !strings.Contains(err.Error(), "nope") {
		t.Errorf("error should name the unknown profile, got: %v", err)
	}
}

func TestResolveProfiles_ScriptPreInstallConflicts(t *testing.T) {
	// A scenario with a script pre-install cannot also pull cnpg fixtures.
	scenario := `conflict
          enabled: true
          shortname: cflt
          profiles: [cnpg]
          pre-install:
            script: pre-install-custom.sh
            description: custom`
	var cfg CITestConfig
	if err := yaml.Unmarshal([]byte(fmt.Sprintf(profileTestConfig, scenario)), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	err := ResolveProfiles(&cfg)
	if err == nil {
		t.Fatal("expected conflict error, got nil")
	}
	if !strings.Contains(err.Error(), "script pre-install") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestResolveProfiles_InlineDependenciesAppendedAfterProfiles(t *testing.T) {
	scenario := `mixed
          enabled: true
          shortname: mix
          profiles: [keycloak]
          dependencies:
            - chart: charts/extra
              release-name: extra`
	cfg := loadAndResolve(t, fmt.Sprintf(profileTestConfig, scenario))
	deps := cfg.Integration.Case.PR.Scenarios[0].Dependencies
	if len(deps) != 2 || deps[0].Chart != "charts/internal-keycloak-26" || deps[1].Chart != "charts/extra" {
		t.Fatalf("expected profile dep then inline dep, got %+v", deps)
	}
}

func TestResolveProfiles_Idempotent(t *testing.T) {
	scenario := `idem
          enabled: true
          shortname: idem
          profiles: [keycloak, elasticsearch, cnpg]`
	src := fmt.Sprintf(profileTestConfig, scenario)
	var cfg CITestConfig
	if err := yaml.Unmarshal([]byte(src), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if err := ResolveProfiles(&cfg); err != nil {
		t.Fatalf("ResolveProfiles (1): %v", err)
	}
	first := len(cfg.Integration.Case.PR.Scenarios[0].Dependencies)
	// Second call must not re-expand the already-resolved dependencies.
	if err := ResolveProfiles(&cfg); err != nil {
		t.Fatalf("ResolveProfiles (2): %v", err)
	}
	second := len(cfg.Integration.Case.PR.Scenarios[0].Dependencies)
	if first != 2 || second != first {
		t.Fatalf("not idempotent: first=%d second=%d (want 2, 2)", first, second)
	}
}

func TestResolveProfiles_MergesTwoFixtureDescriptions(t *testing.T) {
	src := `
integration:
  dependency-profiles:
    fx-a:
      pre-install:
        fixtures: [a.yaml]
        description: AAA
    fx-b:
      pre-install:
        fixtures: [b.yaml]
        description: BBB
  case:
    pr:
      scenario:
        - name: two
          enabled: true
          shortname: two
          profiles: [fx-a, fx-b]
`
	cfg := loadAndResolve(t, src)
	pi := cfg.Integration.Case.PR.Scenarios[0].PreInstall
	if pi == nil {
		t.Fatal("pre-install: nil")
	}
	if len(pi.Fixtures) != 2 || pi.Fixtures[0] != "a.yaml" || pi.Fixtures[1] != "b.yaml" {
		t.Errorf("fixtures: got %v, want [a.yaml b.yaml]", pi.Fixtures)
	}
	if !strings.Contains(pi.Description, "AAA") || !strings.Contains(pi.Description, "BBB") {
		t.Errorf("description dropped a profile's text: %q", pi.Description)
	}
}

func TestResolveProfiles_InlinePreInstallMergesBeforeProfileFixtures(t *testing.T) {
	// Documented ordering: a scenario's own inline pre-install fixtures come
	// before a fixture-contributing profile's. Locks the contract noted in
	// mergeProfilePreInstall.
	src := `
integration:
  dependency-profiles:
    fx-p:
      pre-install:
        fixtures: [profile.yaml]
        description: from profile
  case:
    pr:
      scenario:
        - name: combo
          enabled: true
          shortname: combo
          profiles: [fx-p]
          pre-install:
            fixtures: [inline.yaml]
            description: from scenario
`
	cfg := loadAndResolve(t, src)
	pi := cfg.Integration.Case.PR.Scenarios[0].PreInstall
	if pi == nil {
		t.Fatal("pre-install: nil")
	}
	if len(pi.Fixtures) != 2 || pi.Fixtures[0] != "inline.yaml" || pi.Fixtures[1] != "profile.yaml" {
		t.Errorf("fixtures: got %v, want [inline.yaml profile.yaml] (inline first)", pi.Fixtures)
	}
	if !strings.Contains(pi.Description, "from scenario") || !strings.Contains(pi.Description, "from profile") {
		t.Errorf("description dropped text: %q", pi.Description)
	}
}

func TestResolveProfiles_DeduplicatesMergedFixtures(t *testing.T) {
	// Inline and profile both provide postgresql-cluster.yaml — the merged hook
	// must list it once, not apply the same manifest twice.
	src := `
integration:
  dependency-profiles:
    fx-p:
      pre-install:
        fixtures: [postgresql-cluster.yaml]
        description: from profile
  case:
    pr:
      scenario:
        - name: dup
          enabled: true
          shortname: dup
          profiles: [fx-p]
          pre-install:
            fixtures: [postgresql-cluster.yaml]
            description: from scenario
`
	cfg := loadAndResolve(t, src)
	pi := cfg.Integration.Case.PR.Scenarios[0].PreInstall
	if pi == nil || len(pi.Fixtures) != 1 || pi.Fixtures[0] != "postgresql-cluster.yaml" {
		t.Fatalf("fixtures: got %+v, want exactly [postgresql-cluster.yaml]", pi)
	}
}

func TestResolveProfiles_NoProfilesUnchanged(t *testing.T) {
	// Backward compatibility: a scenario with only inline dependencies and no
	// profiles resolves unchanged.
	src := `
integration:
  case:
    pr:
      scenario:
        - name: inline
          enabled: true
          shortname: inl
          dependencies:
            - chart: charts/internal-keycloak-26
              release-name: keycloak
`
	cfg := loadAndResolve(t, src)
	deps := cfg.Integration.Case.PR.Scenarios[0].Dependencies
	if len(deps) != 1 || deps[0].Chart != "charts/internal-keycloak-26" {
		t.Fatalf("inline deps changed: %+v", deps)
	}
}
