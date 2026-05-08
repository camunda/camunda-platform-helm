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
	"sort"
	"strings"
	"testing"
)

// preSetupScriptAllowlist names files inside pre-setup-scripts/ that are
// permitted to exist without being referenced by any LifecycleHook in
// ci-test-config.yaml. These files exist for purposes other than runner-driven
// hook execution and must be hand-audited when added.
//
//	pre-install-upgrade.sh         — sed-target marker for values-file
//	                                 uncommenting (alpha8 backwards-compat),
//	                                 not invoked by the matrix runner.
//	create-elasticsearch-tls-secrets.sh — helper sourced by
//	                                 pre-install-elasticsearch-self-signed*.sh,
//	                                 never invoked by the runner directly.
var preSetupScriptAllowlist = map[string]bool{
	"pre-install-upgrade.sh":              true,
	"create-elasticsearch-tls-secrets.sh": true,
}

// TestLifecycleFixtures asserts the integrity of the declarative lifecycle
// fixture system across every chart version:
//
//   - every LifecycleHook.Script value resolves to an existing file;
//   - every LifecycleHook.Fixtures[i] resolves to an existing file in
//     common/resources/;
//   - every LifecycleHook.Description is non-empty;
//   - exactly one of Script / Fixtures is set per hook;
//   - every script in pre-setup-scripts/ (modulo the allowlist) is referenced
//     by ≥1 hook — no orphan scripts.
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

	for _, v := range versions {
		v := v
		t.Run(v, func(t *testing.T) {
			validateLifecycleFixturesForVersion(t, repoRoot, v)
		})
	}
}

func validateLifecycleFixturesForVersion(t *testing.T, repoRoot, version string) {
	chartDir := filepath.Join(repoRoot, "charts", "camunda-platform-"+version)
	cfgPath := filepath.Join(chartDir, "test", "ci-test-config.yaml")
	if _, err := os.Stat(cfgPath); err != nil {
		t.Skipf("no ci-test-config.yaml for %s", version)
		return
	}
	cfg, err := LoadCITestConfig(chartDir)
	if err != nil {
		t.Fatalf("LoadCITestConfig: %v", err)
	}

	scriptsDir := filepath.Join(chartDir, "test", "integration", "scenarios", "pre-setup-scripts")
	resourcesDir := filepath.Join(chartDir, "test", "integration", "scenarios", "common", "resources")

	// Collect referenced scripts + fixtures.
	referencedScripts := make(map[string]bool)
	referencedFixtures := make(map[string]bool)

	collect := func(label string, hook *LifecycleHook) {
		if hook == nil {
			return
		}
		if hook.Description == "" {
			t.Errorf("%s: %s: description: empty (required)", version, label)
		}
		hasFixtures := len(hook.Fixtures) > 0
		hasScript := hook.Script != ""
		if hasFixtures == hasScript {
			t.Errorf("%s: %s: must set exactly one of fixtures or script (fixtures=%v script=%q)",
				version, label, hook.Fixtures, hook.Script)
			return
		}
		if hasScript {
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

	for _, scn := range cfg.Integration.Case.PR.Scenarios {
		collect("scenario "+scn.Name+" (PR)", scn.PreInstall)
	}
	for _, scn := range cfg.Integration.Case.Nightly.Scenarios {
		collect("scenario "+scn.Name+" (Nightly)", scn.PreInstall)
	}
	for flowName, hooks := range cfg.Integration.Flows {
		if hooks == nil {
			continue
		}
		collect("flow "+flowName+" (pre-upgrade)", hooks.PreUpgrade)
	}

	// Orphan check: every script in pre-setup-scripts/ (minus allowlist)
	// must be referenced by at least one declarative hook.
	if scriptsEntries, err := os.ReadDir(scriptsDir); err == nil {
		for _, sEntry := range scriptsEntries {
			if sEntry.IsDir() {
				continue
			}
			name := sEntry.Name()
			if !strings.HasSuffix(name, ".sh") {
				continue
			}
			if preSetupScriptAllowlist[name] {
				continue
			}
			if !referencedScripts[name] {
				t.Errorf("%s: orphan script %q has no LifecycleHook reference in ci-test-config.yaml", version, name)
			}
		}
	}
}
