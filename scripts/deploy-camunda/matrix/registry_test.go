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
// two CIScenarios colliding on (Shortname, Flow, Platform) is rejected.
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

// TestRegistryValidatorRejectsOrphanScript: a .sh file in pre-setup-scripts/
// that no LifecycleHook references must be flagged. Guards against the
// coverage gap bkenez raised on #6318: post-#6302 (frozen ci-test-config.yaml
// deletion), this validator is the only place orphan scripts are caught at
// load time. Allowlist entries (preSetupScriptAllowlist) remain exempt.
func TestRegistryValidatorRejectsOrphanScript(t *testing.T) {
	abs := absChartDir(t)
	orphanPath := filepath.Join(abs, "test", "integration", "scenarios", "pre-setup-scripts", "orphan-test.sh")
	if err := os.WriteFile(orphanPath, []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatalf("write orphan: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(orphanPath) })

	_, err := LoadRegistry(abs)
	if err == nil {
		t.Fatal("expected LoadRegistry to reject orphan script via tail-validator")
	}
	if !strings.Contains(err.Error(), "orphan script") || !strings.Contains(err.Error(), "orphan-test.sh") {
		t.Fatalf("want orphan script error mentioning orphan-test.sh, got: %v", err)
	}
}

// TestRegistryValidatorRejectsOrphanFixture: a .yaml/.yml file in
// common/resources/ that no LifecycleHook references must be flagged. Mirrors
// the orphan-script gate. Allowlist entries (commonResourcesAllowlist) remain
// exempt.
func TestRegistryValidatorRejectsOrphanFixture(t *testing.T) {
	abs := absChartDir(t)
	orphanPath := filepath.Join(abs, "test", "integration", "scenarios", "common", "resources", "orphan-test.yaml")
	if err := os.WriteFile(orphanPath, []byte("kind: Test\n"), 0o644); err != nil {
		t.Fatalf("write orphan: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(orphanPath) })

	_, err := LoadRegistry(abs)
	if err == nil {
		t.Fatal("expected LoadRegistry to reject orphan fixture via tail-validator")
	}
	if !strings.Contains(err.Error(), "orphan fixture") || !strings.Contains(err.Error(), "orphan-test.yaml") {
		t.Fatalf("want orphan fixture error mentioning orphan-test.yaml, got: %v", err)
	}
}

// TestRegistryValidatorExemptsAllowlistedOrphans: a file listed in
// preSetupScriptAllowlist / commonResourcesAllowlist is permitted to exist
// without a hook reference. Asserts both allowlists are consulted by the
// orphan walks (regression guard if the allowlist consumer is removed).
func TestRegistryValidatorExemptsAllowlistedOrphans(t *testing.T) {
	abs := absChartDir(t)
	// pre-install-upgrade.sh is in preSetupScriptAllowlist (sed-target marker).
	allowedScript := filepath.Join(abs, "test", "integration", "scenarios", "pre-setup-scripts", "pre-install-upgrade.sh")
	if err := os.WriteFile(allowedScript, []byte("#!/bin/sh\n# sed marker\n"), 0o644); err != nil {
		t.Fatalf("write allowed script: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(allowedScript) })

	// gateway-proxy-settings.yaml is in commonResourcesAllowlist.
	allowedFixture := filepath.Join(abs, "test", "integration", "scenarios", "common", "resources", "gateway-proxy-settings.yaml")
	if err := os.WriteFile(allowedFixture, []byte("kind: ProxySettingsPolicy\n"), 0o644); err != nil {
		t.Fatalf("write allowed fixture: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(allowedFixture) })

	if _, err := LoadRegistry(abs); err != nil {
		t.Fatalf("LoadRegistry should accept allowlisted files: %v", err)
	}
}

// TestLoadRegistryRejectsPathTraversalHookID exercises the isPlainFilename
// guard in LoadRegistry. A manifest scenario referencing a hook ID with a
// path separator (`../evil`) must be rejected before the file read, so a
// hostile or malformed registry cannot escape <chartDir>/test/ci/registry/
// via filepath.Join.
func TestLoadRegistryRejectsPathTraversalHookID(t *testing.T) {
	_, chartDir, regDir := syntheticChart(t)
	writeManifest(t, regDir, "    - id: bad\n      shortname: bad\n      enabled: true\n")
	writeFile(t, filepath.Join(regDir, "scenarios", "bad.yaml"),
		"name: bad\nflows: [install]\npre-install: ../evil\n")

	_, err := LoadRegistry(chartDir)
	if err == nil || !strings.Contains(err.Error(), "plain filename") {
		t.Fatalf("want plain-filename rejection error, got: %v", err)
	}
}

// TestRegistryValidatorRejectsDeniedFlow exercises the permitted-flows denial
// path. A scenario whose flow is denied by the version's permitted-flows
// rules must be flagged by the validator even when all other invariants hold.
func TestRegistryValidatorRejectsDeniedFlow(t *testing.T) {
	dir, chartDir, regDir := syntheticChart(t)
	writePermittedFlows(t, dir, "rules:\n  - match: ==99.99\n    deny: [install]\n")
	writeManifest(t, regDir, "    - id: a\n      shortname: a\n      enabled: true\n")
	writeFile(t, filepath.Join(regDir, "scenarios", "a.yaml"),
		"name: a\nflows: [install]\nplatforms: [gke]\n")

	_, err := LoadRegistry(chartDir)
	if err == nil || !strings.Contains(err.Error(), "denied by permitted-flows") {
		t.Fatalf("want denied-flow error, got: %v", err)
	}
}

// TestLoadRegistryRejectsPathTraversalDepID mirrors the hook traversal guard
// for dependency IDs. Same `isPlainFilename` helper, separate call site in
// the loader; covered independently so a future refactor that drops the
// guard from one branch is caught.
func TestLoadRegistryRejectsPathTraversalDepID(t *testing.T) {
	_, chartDir, regDir := syntheticChart(t)
	writeManifest(t, regDir, "    - id: bad\n      shortname: bad\n      enabled: true\n")
	writeFile(t, filepath.Join(regDir, "scenarios", "bad.yaml"),
		"name: bad\nflows: [install]\ndependencies:\n  - ../evil\n")

	_, err := LoadRegistry(chartDir)
	if err == nil || !strings.Contains(err.Error(), "plain filename") {
		t.Fatalf("want plain-filename rejection on dep ID, got: %v", err)
	}
}

// TestLoadRegistryRejectsPathTraversalManifestID covers the third call site
// of isPlainFilename: a manifest scenario entry whose ID escapes the
// scenarios/ directory.
func TestLoadRegistryRejectsPathTraversalManifestID(t *testing.T) {
	_, chartDir, regDir := syntheticChart(t)
	writeManifest(t, regDir, "    - id: ../evil\n      enabled: true\n")

	_, err := LoadRegistry(chartDir)
	if err == nil || !strings.Contains(err.Error(), "plain filename") {
		t.Fatalf("want plain-filename rejection on manifest ID, got: %v", err)
	}
}

// TestLoadRegistryCarriesExtraValues pins the #6312 loader plumbing: a
// scenario's `extra-values` list must flow from scenarios/<id>.yaml into the
// assembled CIScenario so it can be threaded into the deploy values chain.
func TestLoadRegistryCarriesExtraValues(t *testing.T) {
	_, chartDir, regDir := syntheticChart(t)
	writeManifest(t, regDir, "    - id: alpha\n      shortname: alph\n      enabled: true\n")
	writeFile(t, filepath.Join(regDir, "scenarios", "alpha.yaml"),
		"name: alpha\nauth: keycloak\nflows: [install]\nidentity: keycloak\npersistence: elasticsearch\nplatforms: [gke]\nextra-values:\n  - values/extra/image.yaml\n  - values/extra/tuning.yaml\n")
	// The validator (run inside LoadRegistry) resolves relative extra-values
	// against chart-full-setup, so the referenced files must exist.
	extraDir := filepath.Join(chartDir, "test", "integration", "scenarios", "chart-full-setup", "values", "extra")
	if err := os.MkdirAll(extraDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(extraDir, "image.yaml"), "{}\n")
	writeFile(t, filepath.Join(extraDir, "tuning.yaml"), "{}\n")

	cfg, err := LoadRegistry(chartDir)
	if err != nil {
		t.Fatalf("LoadRegistry: %v", err)
	}
	scns := cfg.Integration.Case.PR.Scenarios
	if len(scns) != 1 {
		t.Fatalf("want 1 scenario, got %d", len(scns))
	}
	got := scns[0].ExtraValues
	want := []string{"values/extra/image.yaml", "values/extra/tuning.yaml"}
	if len(got) != len(want) {
		t.Fatalf("ExtraValues = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("ExtraValues[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestGenerate_PropagatesExtraValues pins hop 2 of the #6312 chain: the
// CIScenario→Entry copy in Generate. A regression deleting that copy would
// leave the loader test green while silently dropping the field before deploy.
func TestGenerate_PropagatesExtraValues(t *testing.T) {
	dir, chartDir, regDir := syntheticChart(t)
	writeFile(t, filepath.Join(dir, "charts", "chart-versions.yaml"),
		"camundaVersions:\n  supportStandard:\n    - \"99.99\"\n")
	writeManifest(t, regDir, "    - id: alpha\n      shortname: alph\n      enabled: true\n")
	writeFile(t, filepath.Join(regDir, "scenarios", "alpha.yaml"),
		"name: alpha\nauth: keycloak\nflows: [install]\nidentity: keycloak\npersistence: elasticsearch\nplatforms: [gke]\nextra-values:\n  - values/extra/image.yaml\n")
	extraDir := filepath.Join(chartDir, "test", "integration", "scenarios", "chart-full-setup", "values", "extra")
	if err := os.MkdirAll(extraDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(extraDir, "image.yaml"), "{}\n")

	entries, err := Generate(dir, GenerateOptions{Versions: []string{"99.99"}})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("want 1 entry, got %d: %+v", len(entries), entries)
	}
	if !reflect.DeepEqual(entries[0].ExtraValues, []string{"values/extra/image.yaml"}) {
		t.Errorf("Entry.ExtraValues = %v, want [values/extra/image.yaml]", entries[0].ExtraValues)
	}
}

// TestLoadRegistryRejectsMalformedManifest surfaces YAML parse failures at
// load time with the manifest path in the error so authors can locate the
// broken file.
func TestLoadRegistryRejectsMalformedManifest(t *testing.T) {
	_, chartDir, regDir := syntheticChart(t)
	writeFile(t, filepath.Join(regDir, "manifest.yaml"), "integration: [: not yaml\n")

	_, err := LoadRegistry(chartDir)
	if err == nil || !strings.Contains(err.Error(), "parse manifest") {
		t.Fatalf("want parse-manifest error, got: %v", err)
	}
}

// TestLoadRegistryRejectsMissingScenarioFile covers the dangling-reference
// case: manifest names a scenario ID that has no corresponding
// scenarios/<id>.yaml file.
func TestLoadRegistryRejectsMissingScenarioFile(t *testing.T) {
	_, chartDir, regDir := syntheticChart(t)
	writeManifest(t, regDir, "    - id: missing\n      enabled: true\n")

	_, err := LoadRegistry(chartDir)
	if err == nil || !strings.Contains(err.Error(), "read scenario") {
		t.Fatalf("want read-scenario error, got: %v", err)
	}
}

// TestRegistryValidatorRejectsMissingScript exercises checkHook's script
// branch — symmetric to the existing fixture coverage but a separate code
// path in registry_validator.go.
func TestRegistryValidatorRejectsMissingScript(t *testing.T) {
	abs := absChartDir(t)
	cfg, err := LoadRegistry(abs)
	if err != nil {
		t.Fatalf("LoadRegistry: %v", err)
	}
	cfg.Integration.Case.PR.Scenarios[0].PreInstall = &LifecycleHook{
		Script:      "never-exists.sh",
		Description: "synthetic missing script",
	}
	err = (&RegistryValidator{ChartDir: abs}).Validate(cfg)
	if err == nil || !strings.Contains(err.Error(), "never-exists.sh") {
		t.Fatalf("want missing-script error, got: %v", err)
	}
}

// TestRegistryValidatorRejectsMissingFeatureValues exercises checkFeature.
// Feature names resolve to <feature>.yaml under chart-full-setup/values/features/;
// a dangling name must surface as a validation error so PR review catches
// the typo before deployment.
func TestRegistryValidatorRejectsMissingFeatureValues(t *testing.T) {
	abs := absChartDir(t)
	cfg, err := LoadRegistry(abs)
	if err != nil {
		t.Fatalf("LoadRegistry: %v", err)
	}
	cfg.Integration.Case.PR.Scenarios[0].Features = []string{"nonexistent-feature"}
	err = (&RegistryValidator{ChartDir: abs}).Validate(cfg)
	if err == nil || !strings.Contains(err.Error(), "nonexistent-feature") {
		t.Fatalf("want missing-feature error, got: %v", err)
	}
}

// TestRegistryValidatorRejectsMissingExtraValues exercises checkExtraValues:
// a scenario's relative extra-values path must resolve under chart-full-setup;
// a dangling reference is caught at validation, not at deploy time. Absolute
// paths are runtime-supplied and intentionally skipped.
func TestRegistryValidatorRejectsMissingExtraValues(t *testing.T) {
	abs := absChartDir(t)
	cfg, err := LoadRegistry(abs)
	if err != nil {
		t.Fatalf("LoadRegistry: %v", err)
	}
	cfg.Integration.Case.PR.Scenarios[0].ExtraValues = []string{"values/extra/nope.yaml"}
	err = (&RegistryValidator{ChartDir: abs}).Validate(cfg)
	if err == nil || !strings.Contains(err.Error(), "nope.yaml") {
		t.Fatalf("want missing-extra-values error, got: %v", err)
	}

	// An absolute path is not validated (runtime-supplied).
	cfg.Integration.Case.PR.Scenarios[0].ExtraValues = []string{"/tmp/runtime-supplied.yaml"}
	if err := (&RegistryValidator{ChartDir: abs}).Validate(cfg); err != nil {
		t.Fatalf("absolute extra-values must skip validation, got: %v", err)
	}
}

// TestRegistryValidatorRejectsMissingDepValuesFile exercises checkDep.
// values-file paths are repo-root-relative (matching runner.go:1742); a
// dangling reference must be caught at validation, not at deploy time.
func TestRegistryValidatorRejectsMissingDepValuesFile(t *testing.T) {
	abs := absChartDir(t)
	cfg, err := LoadRegistry(abs)
	if err != nil {
		t.Fatalf("LoadRegistry: %v", err)
	}
	cfg.Integration.Case.PR.Scenarios[0].Dependencies = []ChartDependency{{
		ReleaseName: "synthetic-dep",
		ValuesFile:  "does/not/exist.yaml",
	}}
	err = (&RegistryValidator{ChartDir: abs}).Validate(cfg)
	if err == nil || !strings.Contains(err.Error(), "does/not/exist.yaml") {
		t.Fatalf("want missing-dep-values error, got: %v", err)
	}
}

// TestRegistryValidatorRejectsHookValidateFailure covers the upstream
// LifecycleHook.Validate rejection path. The validator delegates to
// LifecycleHook.Validate for cross-field invariants (description present,
// exactly one of fixtures/script). A hook with both set must be flagged
// even though both basenames individually resolve.
func TestRegistryValidatorRejectsHookValidateFailure(t *testing.T) {
	abs := absChartDir(t)
	cfg, err := LoadRegistry(abs)
	if err != nil {
		t.Fatalf("LoadRegistry: %v", err)
	}
	cfg.Integration.Case.PR.Scenarios[0].PreInstall = &LifecycleHook{
		Fixtures:    []string{"postgresql-cluster.yaml"},
		Script:      "pre-upgrade.sh",
		Description: "both set — must be rejected",
	}
	err = (&RegistryValidator{ChartDir: abs}).Validate(cfg)
	if err == nil || !strings.Contains(err.Error(), "exactly one of fixtures or script") {
		t.Fatalf("want hook-validate error, got: %v", err)
	}
}

// TestLoadRegistryCachesHooksAndDepsByID asserts the per-ID caches in
// LoadRegistry return identical content across scenarios that reference
// the same ID. alpha and beta in testdata/registry-good both reference
// `cnpg-default` (hook) and `keycloak-26` + `elasticsearch-8.5.1` (deps);
// a cache miscompute would silently swap content between scenarios.
func TestLoadRegistryCachesHooksAndDepsByID(t *testing.T) {
	cfg, err := LoadRegistry(absChartDir(t))
	if err != nil {
		t.Fatalf("LoadRegistry: %v", err)
	}
	scns := cfg.Integration.Case.PR.Scenarios
	if len(scns) < 2 || scns[0].Name != "alpha" || scns[1].Name != "beta" {
		t.Fatalf("expected alpha + beta as first two scenarios, got %+v", scns)
	}
	// Same hook ID → cached pointer is reused.
	if scns[0].PreInstall != scns[1].PreInstall {
		t.Errorf("hook cache: alpha.PreInstall (%p) != beta.PreInstall (%p)", scns[0].PreInstall, scns[1].PreInstall)
	}
	// Same dep IDs → cached values identical (deps are stored by value, not pointer).
	if len(scns[0].Dependencies) < 2 || len(scns[1].Dependencies) < 2 {
		t.Fatalf("expected ≥2 deps on alpha and beta, got %d/%d", len(scns[0].Dependencies), len(scns[1].Dependencies))
	}
	if !reflect.DeepEqual(scns[0].Dependencies[0], scns[1].Dependencies[0]) {
		t.Errorf("dep cache: alpha[0] != beta[0]\nalpha: %+v\nbeta:  %+v", scns[0].Dependencies[0], scns[1].Dependencies[0])
	}
	if !reflect.DeepEqual(scns[0].Dependencies[1], scns[1].Dependencies[1]) {
		t.Errorf("dep cache: alpha[1] != beta[1]\nalpha: %+v\nbeta:  %+v", scns[0].Dependencies[1], scns[1].Dependencies[1])
	}
}

// syntheticChart sets up a throwaway charts/camunda-platform-99.99 layout
// under t.TempDir() with the registry + basename-resolution directories
// the validator stats. Returns (repoRoot, chartDir, registryDir).
func syntheticChart(t *testing.T) (string, string, string) {
	t.Helper()
	dir := t.TempDir()
	chartDir := filepath.Join(dir, "charts", "camunda-platform-99.99")
	regDir := filepath.Join(chartDir, "test", "ci", "registry")
	dirs := []string{
		filepath.Join(regDir, "scenarios"),
		filepath.Join(regDir, "hooks"),
		filepath.Join(regDir, "dependencies"),
		filepath.Join(chartDir, "test", "integration", "scenarios", "common", "resources"),
		filepath.Join(chartDir, "test", "integration", "scenarios", "pre-setup-scripts"),
		filepath.Join(chartDir, "test", "integration", "scenarios", "chart-full-setup", "values", "features"),
		filepath.Join(dir, ".github", "config"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// Permissive default so tests that don't care about permitted-flows pass.
	writePermittedFlows(t, dir, "rules: []\n")
	return dir, chartDir, regDir
}

// writeManifest writes a minimal manifest.yaml whose scenarios block is the
// caller-supplied YAML fragment (must be properly indented under `scenarios:`).
func writeManifest(t *testing.T, regDir, scenariosFragment string) {
	t.Helper()
	manifest := "integration:\n  vars:\n    tasksBaseDir: x\n    valuesBaseDir: x\n    chartsBaseDir: x\n  scenarios:\n" + scenariosFragment
	writeFile(t, filepath.Join(regDir, "manifest.yaml"), manifest)
}

func writePermittedFlows(t *testing.T, repoRoot, body string) {
	t.Helper()
	writeFile(t, filepath.Join(repoRoot, ".github", "config", "permitted-flows.yaml"), "defaults:\n  flows: []\n"+body)
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
