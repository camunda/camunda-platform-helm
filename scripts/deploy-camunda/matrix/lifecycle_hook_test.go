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

// Orphan exemptions live in each chart's .orphan-allowlist.yaml; loaded by
// LoadOrphanAllowlist. Per-version dead-entry checks happen inside
// RegistryValidator (and the orphan walk below).

// TestLifecycleFixtures asserts the integrity of the declarative lifecycle
// fixture system across every chart version:
//
//   - LifecycleHook.Validate passes (description non-empty, exactly one mode);
//   - every LifecycleHook.Script value resolves to an existing file;
//   - every LifecycleHook.Fixtures[i] resolves to an existing file in
//     common/resources/;
//   - every script in pre-setup-scripts/ (modulo the chart's
//     .orphan-allowlist.yaml) is referenced by ≥1 hook — no orphan scripts;
//   - integration.flows.* keys reference flows that some scenario actually uses.
func TestLifecycleFixtures(t *testing.T) {
	repoRoot := findRepoRoot(t)
	// Source of truth for "which versions are under active CI": alpha +
	// supportStandard from charts/chart-versions.yaml. supportExtended series
	// remain on disk for archival reasons but are not in the active CI matrix,
	// so we don't assert their pre-setup-scripts/ hygiene here.
	cv, err := LoadChartVersions(repoRoot)
	if err != nil {
		t.Fatalf("LoadChartVersions: %v", err)
	}
	versions := append([]string(nil), cv.ActiveVersions()...)
	sort.Strings(versions)

	for _, v := range versions {
		t.Run(v, func(t *testing.T) {
			validateLifecycleFixturesForVersion(t, repoRoot, v)
		})
	}
}

func validateLifecycleFixturesForVersion(t *testing.T, repoRoot, version string) {
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

	scriptAllowlist, resourceAllowlist, err := LoadOrphanAllowlist(chartDir)
	if err != nil {
		t.Errorf("%s: load .orphan-allowlist.yaml: %v", version, err)
	}

	// Orphan check: every script in pre-setup-scripts/ (minus exemptions)
	// must be referenced by at least one declarative hook.
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
		if scriptAllowlist[name] {
			continue
		}
		if !referencedScripts[name] {
			t.Errorf("%s: orphan script %q has no LifecycleHook reference in ci-test-config.yaml (add to %s if intentional)", version, name, OrphanAllowlistFile)
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
		if resourceAllowlist[name] {
			continue
		}
		if !referencedFixtures[name] {
			t.Errorf("%s: orphan fixture %q has no LifecycleHook reference in ci-test-config.yaml (add to %s if intentional)", version, name, OrphanAllowlistFile)
		}
	}

	// Dead-entry check: every allowlist entry must match a file that exists
	// in this chart version (the per-chart YAML cannot drift the way the old
	// cross-version Go map could).
	for name := range scriptAllowlist {
		if _, err := os.Stat(filepath.Join(scriptsDir, name)); err != nil {
			t.Errorf("%s: %s pre-setup-scripts entry %q matches no file in pre-setup-scripts/ — remove it", version, OrphanAllowlistFile, name)
		}
	}
	for name := range resourceAllowlist {
		if _, err := os.Stat(filepath.Join(resourcesDir, name)); err != nil {
			t.Errorf("%s: %s common-resources entry %q matches no file in common/resources/ — remove it", version, OrphanAllowlistFile, name)
		}
	}
}
