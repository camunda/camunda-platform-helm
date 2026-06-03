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

// TestLoadRegistryRejectsPathTraversalHookID exercises the isPlainFilename
// guard in LoadRegistry. A manifest scenario referencing a hook ID with a
// path separator (`../evil`) must be rejected before the file read, so a
// hostile or malformed registry cannot escape <chartDir>/test/ci/registry/
// via filepath.Join.
func TestLoadRegistryRejectsPathTraversalHookID(t *testing.T) {
	dir := t.TempDir()
	chartDir := filepath.Join(dir, "charts", "camunda-platform-99.99")
	regDir := filepath.Join(chartDir, "test", "ci", "registry")
	for _, sub := range []string{"scenarios", "hooks", "dependencies"} {
		if err := os.MkdirAll(filepath.Join(regDir, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	manifest := "integration:\n  vars:\n    tasksBaseDir: x\n    valuesBaseDir: x\n    chartsBaseDir: x\n  scenarios:\n    - id: bad\n      enabled: true\n"
	if err := os.WriteFile(filepath.Join(regDir, "manifest.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	scenario := "name: bad\nshortname: bad\nflows: [install]\npre-install: ../evil\n"
	if err := os.WriteFile(filepath.Join(regDir, "scenarios", "bad.yaml"), []byte(scenario), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadRegistry(chartDir)
	if err == nil || !strings.Contains(err.Error(), "plain filename") {
		t.Fatalf("want plain-filename rejection error, got: %v", err)
	}
}

// TestRegistryValidatorRejectsDeniedFlow exercises the permitted-flows denial
// path. A scenario whose flow is denied by the version's permitted-flows
// rules must be flagged by the validator even when all other invariants hold.
func TestRegistryValidatorRejectsDeniedFlow(t *testing.T) {
	dir := t.TempDir()
	chartDir := filepath.Join(dir, "charts", "camunda-platform-99.99")
	regDir := filepath.Join(chartDir, "test", "ci", "registry")
	for _, sub := range []string{"scenarios", "hooks", "dependencies"} {
		if err := os.MkdirAll(filepath.Join(regDir, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// Empty basename-resolution trees so the validator's filesystem checks pass.
	for _, sub := range []string{
		filepath.Join(chartDir, "test", "integration", "scenarios", "common", "resources"),
		filepath.Join(chartDir, "test", "integration", "scenarios", "pre-setup-scripts"),
		filepath.Join(chartDir, "test", "integration", "scenarios", "chart-full-setup", "values", "features"),
		filepath.Join(dir, ".github", "config"),
	} {
		if err := os.MkdirAll(sub, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	permittedFlows := "defaults:\n  flows: []\nrules:\n  - match: ==99.99\n    deny: [install]\n"
	if err := os.WriteFile(filepath.Join(dir, ".github", "config", "permitted-flows.yaml"), []byte(permittedFlows), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := "integration:\n  vars:\n    tasksBaseDir: x\n    valuesBaseDir: x\n    chartsBaseDir: x\n  scenarios:\n    - id: a\n      enabled: true\n"
	if err := os.WriteFile(filepath.Join(regDir, "manifest.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	scenario := "name: a\nshortname: a\nflows: [install]\nplatforms: [gke]\n"
	if err := os.WriteFile(filepath.Join(regDir, "scenarios", "a.yaml"), []byte(scenario), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadRegistry(chartDir)
	if err == nil || !strings.Contains(err.Error(), "denied by permitted-flows") {
		t.Fatalf("want denied-flow error, got: %v", err)
	}
}
