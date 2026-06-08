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
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Allowlists for orphan exemption live in lifecycle_allowlist.go (consumed
// by both RegistryValidator's load-time orphan walk and TestLifecycleFixtures's
// cross-version dead-entry check below).

// TestLifecycleFixtures asserts the integrity of the declarative lifecycle
// fixture system across every chart version:
//
//   - LifecycleHook.Validate passes (description non-empty, exactly one mode);
//   - every LifecycleHook.Script value resolves to an existing file;
//   - every LifecycleHook.Fixtures[i] resolves to an existing file in
//     common/resources/;
//   - every script in pre-setup-scripts/ (modulo the allowlist) is referenced
//     by ≥1 hook — no orphan scripts;
//   - integration.flows.* keys reference flows that some scenario actually uses;
//   - allowlist entries point to files that exist in at least one chart version.
func TestLifecycleFixtures(t *testing.T) {
	repoRoot := findRepoRoot(t)
	chartsDir := filepath.Join(repoRoot, "charts")
	entries, err := os.ReadDir(chartsDir)
	if err != nil {
		t.Fatalf("read charts dir: %v", err)
	}

	versions := make([]string, 0)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		const prefix = "camunda-platform-"
		if !strings.HasPrefix(e.Name(), prefix) {
			continue
		}
		versions = append(versions, strings.TrimPrefix(e.Name(), prefix))
	}
	sort.Strings(versions)

	// Allowlist usage tracking — entries unused across every version are dead
	// references and must be removed.
	usedScriptAllowlist := make(map[string]bool)
	usedFixtureAllowlist := make(map[string]bool)

	for _, v := range versions {
		v := v
		t.Run(v, func(t *testing.T) {
			validateLifecycleFixturesForVersion(t, repoRoot, v, usedScriptAllowlist, usedFixtureAllowlist)
		})
	}

	for name := range preSetupScriptAllowlist {
		if !usedScriptAllowlist[name] {
			t.Errorf("preSetupScriptAllowlist entry %q matches no file in any version — remove it", name)
		}
	}
	for name := range commonResourcesAllowlist {
		if !usedFixtureAllowlist[name] {
			t.Errorf("commonResourcesAllowlist entry %q matches no file in any version — remove it", name)
		}
	}
}

func validateLifecycleFixturesForVersion(t *testing.T, repoRoot, version string,
	usedScriptAllowlist, usedFixtureAllowlist map[string]bool) {
	chartDir := filepath.Join(repoRoot, "charts", "camunda-platform-"+version)
	// Some chart dirs (end-of-life, alpha channel) ship without a test/ tree
	// at all — those are not testable here. Skip the version entirely.
	testDir := filepath.Join(chartDir, "test")
	if _, err := os.Stat(testDir); errors.Is(err, fs.ErrNotExist) {
		t.Skipf("no test/ tree for %s", version)
		return
	}
	// Dispatch matches matrix.Generate: prefer the live registry when present
	// so registry-driven versions (post-ADR-0093 migration) are validated
	// against the registry, not the frozen ci-test-config.yaml snapshot.
	// Without this branch, new hooks added to the registry would false-positive
	// the orphan walk because the frozen snapshot does not see them.
	var cfg *CITestConfig
	if HasRegistry(chartDir) {
		var err error
		cfg, err = LoadRegistry(chartDir)
		if err != nil {
			t.Fatalf("LoadRegistry: %v", err)
		}
	} else {
		cfgPath := filepath.Join(testDir, "ci-test-config.yaml")
		if _, err := os.Stat(cfgPath); err != nil {
			t.Errorf("ci-test-config.yaml missing for %s at %s: %v", version, cfgPath, err)
			return
		}
		var err error
		cfg, err = LoadCITestConfig(chartDir)
		if err != nil {
			t.Fatalf("LoadCITestConfig: %v", err)
		}
	}

	scriptsDir := filepath.Join(chartDir, "test", "integration", "scenarios", "pre-setup-scripts")
	resourcesDir := filepath.Join(chartDir, "test", "integration", "scenarios", "common", "resources")

	referencedScripts := make(map[string]bool)
	referencedFixtures := make(map[string]bool)

	collect := func(label string, hook *LifecycleHook) {
		if hook == nil {
			return
		}
		if err := hook.Validate(version + ": " + label); err != nil {
			t.Error(err)
			return
		}
		if hook.Script != "" {
			referencedScripts[hook.Script] = true
			scriptPath := filepath.Join(scriptsDir, hook.Script)
			if info, err := os.Stat(scriptPath); err != nil || info.IsDir() {
				t.Errorf("%s: %s: script %q: missing or not a file at %s", version, label, hook.Script, scriptPath)
			}
		}
		for _, fx := range hook.Fixtures {
			referencedFixtures[fx] = true
			fxPath := filepath.Join(resourcesDir, fx)
			if info, err := os.Stat(fxPath); err != nil || info.IsDir() {
				t.Errorf("%s: %s: fixture %q: missing or not a file at %s", version, label, fx, fxPath)
			}
		}
	}

	// Source of truth for valid flow names: permitted-flows.yaml defaults
	// plus any per-scenario `flow:` value in this version's config.
	knownFlows := map[string]bool{}
	if pf, err := LoadPermittedFlows(repoRoot); err != nil {
		t.Fatalf("LoadPermittedFlows: %v", err)
	} else {
		for _, f := range pf.Defaults.Flows {
			knownFlows[f] = true
		}
	}
	addScenarioFlows := func(scn CIScenario) {
		for _, f := range strings.Split(scn.Flow, ",") {
			if f = strings.TrimSpace(f); f != "" {
				knownFlows[f] = true
			}
		}
	}
	for _, scn := range cfg.Integration.Case.PR.Scenarios {
		collect("scenario "+scn.Name+" (PR pre-install)", scn.PreInstall)
		collect("scenario "+scn.Name+" (PR post-deploy)", scn.PostDeploy)
		addScenarioFlows(scn)
	}
	for _, scn := range cfg.Integration.Case.Nightly.Scenarios {
		collect("scenario "+scn.Name+" (nightly pre-install)", scn.PreInstall)
		collect("scenario "+scn.Name+" (nightly post-deploy)", scn.PostDeploy)
		addScenarioFlows(scn)
	}
	// Validate dependency-profile pre-install fixtures directly, so a typo in a
	// profile not yet referenced by any scenario is still caught.
	for profName, prof := range cfg.Integration.DependencyProfiles {
		collect("dependency-profile "+profName+" (pre-install)", prof.PreInstall)
	}
	for flowName, hooks := range cfg.Integration.Flows {
		if !knownFlows[flowName] {
			t.Errorf("%s: integration.flows.%s: not a known flow (not in permitted-flows.yaml defaults and no scenario uses it) — typo or dead key", version, flowName)
		}
		if hooks == nil {
			continue
		}
		collect("flow "+flowName+" (pre-upgrade)", hooks.PreUpgrade)
	}

	// Orphan check: every script in pre-setup-scripts/ (minus allowlist)
	// must be referenced by at least one declarative hook. Allowlist matches
	// here count as "used" for the cross-version dead-entry check.
	scriptsEntries, err := os.ReadDir(scriptsDir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("%s: read pre-setup-scripts/: %v", version, err)
	}
	for _, sEntry := range scriptsEntries {
		if sEntry.IsDir() {
			continue
		}
		name := sEntry.Name()
		if !strings.HasSuffix(name, ".sh") {
			continue
		}
		if preSetupScriptAllowlist[name] {
			usedScriptAllowlist[name] = true
			continue
		}
		if !referencedScripts[name] {
			t.Errorf("%s: orphan script %q has no LifecycleHook reference in ci-test-config.yaml", version, name)
		}
	}

	// Orphan check: every YAML in common/resources/ must be referenced by at
	// least one declarative hook (fixtures: list).
	resEntries, err := os.ReadDir(resourcesDir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("%s: read common/resources/: %v", version, err)
	}
	for _, rEntry := range resEntries {
		if rEntry.IsDir() {
			continue
		}
		name := rEntry.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}
		if commonResourcesAllowlist[name] {
			usedFixtureAllowlist[name] = true
			continue
		}
		if !referencedFixtures[name] {
			t.Errorf("%s: orphan fixture %q has no LifecycleHook reference in ci-test-config.yaml", version, name)
		}
	}
}
