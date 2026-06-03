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
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const registryGoodChartDir = "testdata/registry-good/charts/camunda-platform-99.99"

// absChartDir resolves the testdata chart directory once per test.
func absChartDir(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs(registryGoodChartDir)
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	return abs
}

// TestHasRegistry: presence of manifest.yaml controls the dispatch decision.
func TestHasRegistry(t *testing.T) {
	if !HasRegistry(absChartDir(t)) {
		t.Fatal("should detect testdata registry")
	}
	missing, err := filepath.Abs("testdata/registry-good/charts/does-not-exist")
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	if HasRegistry(missing) {
		t.Fatal("must not detect missing registry")
	}
}

// TestLoadRegistryAssembly: loader emits the legacy CITestConfig shape with
// plural flows fanned out, hooks/deps resolved by ID, manifest order preserved.
func TestLoadRegistryAssembly(t *testing.T) {
	abs := absChartDir(t)
	cfg, err := LoadRegistry(abs)
	if err != nil {
		t.Fatalf("LoadRegistry: %v", err)
	}

	// vars round-trip
	if got, want := cfg.Integration.Vars.TasksBaseDir, "../../../test/integration/scenarios"; got != want {
		t.Errorf("vars.tasksBaseDir = %q, want %q", got, want)
	}
	if got, want := cfg.Integration.Vars.ValuesBaseDir, "integration/scenarios"; got != want {
		t.Errorf("vars.valuesBaseDir = %q, want %q", got, want)
	}
	if got, want := cfg.Integration.Vars.ChartsBaseDir, "../../../.."; got != want {
		t.Errorf("vars.chartsBaseDir = %q, want %q", got, want)
	}

	// flow hooks round-trip
	fh, ok := cfg.Integration.Flows["upgrade-minor"]
	if !ok || fh == nil || fh.PreUpgrade == nil {
		t.Fatalf("flows.upgrade-minor.pre-upgrade missing: %+v", cfg.Integration.Flows)
	}
	if got, want := fh.PreUpgrade.Script, "pre-upgrade.sh"; got != want {
		t.Errorf("pre-upgrade.script = %q, want %q", got, want)
	}

	// scenarios: 1 + 1 + 2 = 4 post-fan-out entries in manifest order
	scns := cfg.Integration.Case.PR.Scenarios
	if len(scns) != 4 {
		t.Fatalf("scenarios len = %d, want 4 (alpha + beta + gamma×2)", len(scns))
	}

	// alpha
	a := scns[0]
	if a.Name != "alpha" || a.Flow != "install" || a.Tier != 1 || !a.Enabled {
		t.Errorf("alpha = %+v", a)
	}
	if a.PreInstall == nil || !reflect.DeepEqual(a.PreInstall.Fixtures, []string{"postgresql-cluster.yaml"}) {
		t.Errorf("alpha.PreInstall = %+v", a.PreInstall)
	}
	if len(a.Dependencies) != 2 || a.Dependencies[0].ReleaseName != "keycloak" || a.Dependencies[1].ReleaseName != "elasticsearch" {
		t.Errorf("alpha deps = %+v", a.Dependencies)
	}

	// beta carries post-deploy and a feature
	b := scns[1]
	if b.Name != "beta" || b.PostDeploy == nil || b.PostDeploy.Script != "post-deploy-beta.sh" {
		t.Errorf("beta = %+v post=%+v", b, b.PostDeploy)
	}
	if !reflect.DeepEqual(b.Features, []string{"synthetic-feature"}) {
		t.Errorf("beta.Features = %v", b.Features)
	}

	// gamma fans out across two flows; enabled propagates from manifest (false)
	if scns[2].Name != "gamma" || scns[2].Flow != "install" || scns[2].Enabled {
		t.Errorf("gamma[0] = %+v", scns[2])
	}
	if scns[3].Name != "gamma" || scns[3].Flow != "upgrade-minor" || scns[3].Enabled {
		t.Errorf("gamma[1] = %+v", scns[3])
	}
}

// TestRegistryValidatorAcceptsGood: validator is silent on a well-formed registry.
func TestRegistryValidatorAcceptsGood(t *testing.T) {
	abs := absChartDir(t)
	cfg, err := LoadRegistry(abs)
	if err != nil {
		t.Fatalf("LoadRegistry: %v", err)
	}
	if err := (&RegistryValidator{ChartDir: abs}).Validate(cfg); err != nil {
		t.Fatalf("validator should accept good registry: %v", err)
	}
}

// TestRegistryValidatorRejectsDuplicatePlatformFlow: a fabricated config with
// two CIScenarios colliding on (Name, Shortname, Flow, Platform) is rejected.
func TestRegistryValidatorRejectsDuplicatePlatformFlow(t *testing.T) {
	abs := absChartDir(t)
	cfg, err := LoadRegistry(abs)
	if err != nil {
		t.Fatalf("LoadRegistry: %v", err)
	}
	cfg.Integration.Case.PR.Scenarios = append(cfg.Integration.Case.PR.Scenarios, cfg.Integration.Case.PR.Scenarios[0])
	err = (&RegistryValidator{ChartDir: abs}).Validate(cfg)
	if err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("want duplicate-tuple error, got: %v", err)
	}
}

// TestRegistryValidatorRejectsMissingFixture: hook references a fixture file
// that doesn't exist on disk.
func TestRegistryValidatorRejectsMissingFixture(t *testing.T) {
	abs := absChartDir(t)
	cfg, err := LoadRegistry(abs)
	if err != nil {
		t.Fatalf("LoadRegistry: %v", err)
	}
	cfg.Integration.Case.PR.Scenarios[0].PreInstall = &LifecycleHook{
		Fixtures:    []string{"never-exists.yaml"},
		Description: "synthetic missing fixture",
	}
	err = (&RegistryValidator{ChartDir: abs}).Validate(cfg)
	if err == nil || !strings.Contains(err.Error(), "never-exists.yaml") {
		t.Fatalf("want missing-fixture error, got: %v", err)
	}
}
