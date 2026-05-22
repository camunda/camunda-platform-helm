// Copyright 2024 Camunda Services GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package scenarios

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"scripts/camunda-core/pkg/versionmatrix"
)

// TestUpgradePrefixCoverage enforces the invariant for issue #6172 acceptance
// criterion 4: for every enabled PR scenario whose flow is an upgrade flow,
// the set of $*_INDEX_PREFIX placeholders referenced by the install-step
// layered values is a subset of the set referenced by the upgrade-step values.
//
// Failure mode this guards against: install creates indices named by the
// install-side prefix; upgrade reads from a different prefix; result is empty
// indices after upgrade (PR #6160 was the 8.8 OpenSearch instance of this bug).
//
// See docs/ci-scenario-matrix.md for the surrounding context.
func TestUpgradePrefixCoverage(t *testing.T) {
	repoRoot := mustFindRepoRoot(t)

	chartVersions := []string{"8.7", "8.8", "8.9", "8.10"}
	var cases []upgradeScenarioCase
	for _, v := range chartVersions {
		ciConfigPath := filepath.Join(repoRoot, "charts", "camunda-platform-"+v, "test", "ci-test-config.yaml")
		rows, err := loadEnabledUpgradeRows(ciConfigPath, v)
		if err != nil {
			t.Fatalf("load %s: %v", ciConfigPath, err)
		}
		cases = append(cases, rows...)
	}
	if len(cases) == 0 {
		t.Fatal("no enabled upgrade scenarios discovered across any chart version")
	}

	for _, tc := range cases {
		tc := tc
		t.Run(fmt.Sprintf("v%s/%s/%s", tc.Version, tc.Shortname, tc.Flow), func(t *testing.T) {
			runPrefixCoverageCase(t, repoRoot, tc)
		})
	}
}

func runPrefixCoverageCase(t *testing.T, repoRoot string, tc upgradeScenarioCase) {
	t.Helper()

	cfg, err := BuildDeploymentConfig(tc.Name, BuilderOverrides{
		Identity:    tc.Identity,
		Persistence: tc.Persistence,
		Platform:    "gke",
		Features:    tc.Features,
		Flow:        tc.Flow,
	})
	if err != nil {
		t.Fatalf("build deployment config: %v", err)
	}

	currentScenariosDir := filepath.Join(repoRoot, "charts", "camunda-platform-"+tc.Version,
		"test", "integration", "scenarios", "chart-full-setup")

	prevVersion, err := versionmatrix.PreviousAppVersion(tc.Version)
	if err != nil {
		t.Fatalf("previous app version: %v", err)
	}
	prevScenariosDir := filepath.Join(repoRoot, "charts", "camunda-platform-"+prevVersion,
		"test", "integration", "scenarios", "chart-full-setup")

	// upgrade-patch installs and upgrades within the same chart version.
	// upgrade-minor / modular-upgrade-minor install with the previous version.
	installScenariosDir := prevScenariosDir
	if tc.Flow == "upgrade-patch" {
		installScenariosDir = currentScenariosDir
	}

	if missing := requiredLayerMissing(cfg, installScenariosDir); missing != "" {
		t.Skipf("install-step layer file missing for v%s layer %s — see G6 in docs/ci-scenario-matrix.md",
			tc.Version, missing)
	}
	if missing := requiredLayerMissing(cfg, currentScenariosDir); missing != "" {
		t.Skipf("upgrade-step layer file missing for v%s layer %s — see G6 in docs/ci-scenario-matrix.md",
			tc.Version, missing)
	}

	installCfg := *cfg
	installCfg.Upgrade = false
	installPaths, err := installCfg.ResolvePaths(installScenariosDir)
	if err != nil {
		t.Fatalf("resolve install paths: %v", err)
	}

	upgradeCfg := *cfg
	upgradeCfg.Upgrade = true
	upgradePaths, err := upgradeCfg.ResolvePaths(currentScenariosDir)
	if err != nil {
		t.Fatalf("resolve upgrade paths: %v", err)
	}

	installSet, err := collectPrefixes(installPaths)
	if err != nil {
		t.Fatalf("collect install-step prefixes: %v", err)
	}
	upgradeSet, err := collectPrefixes(upgradePaths)
	if err != nil {
		t.Fatalf("collect upgrade-step prefixes: %v", err)
	}

	installOnly := diff(installSet, upgradeSet)
	upgradeOnly := diff(upgradeSet, installSet)

	if len(installOnly) > 0 {
		t.Errorf("install-step references %v but upgrade-step does not — indices created by install will be invisible to upgrade", installOnly)
	}
	if len(upgradeOnly) > 0 {
		t.Logf("note: upgrade-step references %v but install-step does not — likely dead env-var generation", upgradeOnly)
	}
}

type upgradeScenarioCase struct {
	Version     string
	Name        string
	Shortname   string
	Flow        string
	Identity    string
	Persistence string
	Features    []string
}

// ciTestConfig mirrors only the fields needed for the upgrade-coverage test.
type ciTestConfig struct {
	Integration struct {
		Case struct {
			PR struct {
				Scenario []ciScenarioRow `yaml:"scenario"`
			} `yaml:"pr"`
		} `yaml:"case"`
	} `yaml:"integration"`
}

type ciScenarioRow struct {
	Name        string   `yaml:"name"`
	Shortname   string   `yaml:"shortname"`
	Flow        string   `yaml:"flow"`
	Enabled     bool     `yaml:"enabled"`
	Identity    string   `yaml:"identity"`
	Persistence string   `yaml:"persistence"`
	Features    []string `yaml:"features"`
}

func loadEnabledUpgradeRows(path, version string) ([]upgradeScenarioCase, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg ciTestConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	var out []upgradeScenarioCase
	for _, row := range cfg.Integration.Case.PR.Scenario {
		if !row.Enabled || row.Flow == "" {
			continue
		}
		for _, flow := range strings.Split(row.Flow, ",") {
			flow = strings.TrimSpace(flow)
			if !isUpgradeFlow(flow) {
				continue
			}
			out = append(out, upgradeScenarioCase{
				Version:     version,
				Name:        row.Name,
				Shortname:   row.Shortname,
				Flow:        flow,
				Identity:    row.Identity,
				Persistence: row.Persistence,
				Features:    row.Features,
			})
		}
	}
	return out, nil
}

func isUpgradeFlow(flow string) bool {
	switch flow {
	case "upgrade-patch", "upgrade-minor", "modular-upgrade-minor":
		return true
	}
	return false
}

// requiredLayerMissing returns the name of the first required layer whose YAML
// file does not exist under scenariosDir/values/, or "" if all are present.
// This matches the lookups ResolvePaths performs without surfacing a render error.
func requiredLayerMissing(cfg *DeploymentConfig, scenariosDir string) string {
	valuesDir := filepath.Join(scenariosDir, ValuesDir)
	checks := []struct {
		dir  string
		name string
	}{
		{IdentityDir, cfg.Identity},
		{PersistenceDir, cfg.Persistence},
	}
	for _, c := range checks {
		if c.name == "" {
			continue
		}
		p := filepath.Join(valuesDir, c.dir, c.name+".yaml")
		if _, err := os.Stat(p); err != nil {
			return c.dir + "/" + c.name + ".yaml"
		}
	}
	for _, f := range cfg.Features {
		p := filepath.Join(valuesDir, FeaturesDir, f+".yaml")
		if _, err := os.Stat(p); err != nil {
			return FeaturesDir + "/" + f + ".yaml"
		}
	}
	return ""
}

var prefixPlaceholder = regexp.MustCompile(`\$([A-Z][A-Z0-9_]*)_INDEX_PREFIX`)

// collectPrefixes walks each YAML file via yaml.v3 Node traversal and returns
// the set of *_INDEX_PREFIX placeholder names referenced in scalar values
// (so commented lines are ignored).
func collectPrefixes(paths []string) (map[string]struct{}, error) {
	set := map[string]struct{}{}
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", p, err)
		}
		var root yaml.Node
		if err := yaml.Unmarshal(data, &root); err != nil {
			return nil, fmt.Errorf("parse %s: %w", p, err)
		}
		walkScalars(&root, func(s string) {
			for _, m := range prefixPlaceholder.FindAllStringSubmatch(s, -1) {
				set[m[1]+"_INDEX_PREFIX"] = struct{}{}
			}
		})
	}
	return set, nil
}

func walkScalars(n *yaml.Node, fn func(string)) {
	if n == nil {
		return
	}
	if n.Kind == yaml.ScalarNode {
		fn(n.Value)
		return
	}
	for _, c := range n.Content {
		walkScalars(c, fn)
	}
}

func diff(a, b map[string]struct{}) []string {
	var out []string
	for k := range a {
		if _, ok := b[k]; !ok {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// mustFindRepoRoot walks up from the test file's location to the directory
// whose go.mod declares "module scripts/camunda-core", then climbs three more
// levels to the camunda-platform-helm repo root.
func mustFindRepoRoot(t *testing.T) string {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	dir := filepath.Dir(here)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
			if err == nil && strings.Contains(string(data), "module scripts/camunda-core") {
				// dir == .../scripts/camunda-core; repo root is two levels up.
				return filepath.Clean(filepath.Join(dir, "..", ".."))
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("camunda-core go.mod not found while walking up from test file")
		}
		dir = parent
	}
}
