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

func TestGenerate_PropagatesPreInstall(t *testing.T) {
	repoRoot := findRepoRoot(t)

	entries, err := Generate(repoRoot, GenerateOptions{Versions: []string{"8.10"}})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	var rdbms *Entry
	for i := range entries {
		if entries[i].Scenario == "rdbms" && entries[i].Version == "8.10" {
			rdbms = &entries[i]
			break
		}
	}
	if rdbms == nil {
		t.Fatal("rdbms 8.10 entry not found")
	}
	if rdbms.PreInstall == nil {
		t.Fatal("rdbms 8.10: PreInstall: nil")
	}
	if len(rdbms.PreInstall.Fixtures) != 1 || rdbms.PreInstall.Fixtures[0] != "postgresql-cluster.yaml" {
		t.Errorf("rdbms 8.10 fixtures: got %v", rdbms.PreInstall.Fixtures)
	}
	if rdbms.PreInstall.Description == "" {
		t.Error("rdbms 8.10: description: empty")
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
