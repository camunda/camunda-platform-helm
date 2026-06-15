package matrix

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"scripts/camunda-core/pkg/scenarios"
	"scripts/camunda-core/pkg/versionmatrix"
)

// These tests deliver the CI-coverage guarantees of issue #6172 against the
// composable registry (charts/<v>/test/ci/registry/). They were originally
// written for the pre-refactor monolithic ci-test-config.yaml; here they are
// re-expressed to source scenarios from the generated matrix (Generate) so they
// stay correct after the registry migration that removed LoadCITestConfig.

var indexPrefixPlaceholder = regexp.MustCompile(`\$([A-Z][A-Z0-9_]*)_INDEX_PREFIX`)

// gapRequirement identifies a scenario row that issue #6172 requires to run in
// PR CI. Matching is by generated matrix Entry fields (Scenario is the
// scenario's logical name), not the registry id.
type gapRequirement struct {
	version   string
	platform  string
	scenario  string
	shortname string
	flow      string
}

// TestIssue6172UpgradeGapsRunInPRCI asserts that the modular-upgrade-minor
// coverage gaps identified in issue #6172 (OpenSearch / DocStore / multitenancy
// upgrades on 8.9 and 8.10) are enabled in the composable registry and therefore
// generated into the CI matrix. The matrix is generated with the default
// (enabled-only) options, so a matching row existing here means the scenario is
// enabled — closing the gap that earlier left these upgrade paths untested on
// chart PRs.
func TestIssue6172UpgradeGapsRunInPRCI(t *testing.T) {
	entries := generateEnabledMatrix(t)
	required := []gapRequirement{
		{version: "8.9", platform: "gke", scenario: "qa-opensearch-upg", shortname: "qaosupg", flow: "modular-upgrade-minor"},
		{version: "8.9", platform: "eks", scenario: "qa-document-store-upg", shortname: "qadocupg", flow: "modular-upgrade-minor"},
		{version: "8.10", platform: "gke", scenario: "qa-opensearch-upg", shortname: "qaosupg", flow: "modular-upgrade-minor"},
		{version: "8.10", platform: "eks", scenario: "qa-document-store-upg", shortname: "qadocupg", flow: "modular-upgrade-minor"},
		{version: "8.10", platform: "gke", scenario: "qa-elasticsearch-mt-upg", shortname: "qaelmtupg", flow: "modular-upgrade-minor"},
	}

	for _, req := range required {
		req := req
		t.Run(fmt.Sprintf("v%s/%s/%s", req.version, req.platform, req.scenario), func(t *testing.T) {
			if !gapEntryPresent(entries, req) {
				t.Fatalf("issue #6172 requires this upgrade scenario enabled in the registry, "+
					"but no matching enabled matrix row was generated: %+v", req)
			}
		})
	}
}

// TestUpgradePrefixCoverage enforces issue #6172 acceptance criterion 4: for
// every enabled scenario whose flow is an upgrade flow, the set of
// $*_INDEX_PREFIX placeholders referenced by the install-step layered values is
// a subset of those referenced by the upgrade-step values.
//
// Failure mode this guards against: install creates indices named by the
// install-side prefix; upgrade reads from a different prefix; result is empty
// indices after upgrade (PR #6160 was the 8.8 OpenSearch instance of this bug).
func TestUpgradePrefixCoverage(t *testing.T) {
	repoRoot := findRepoRoot(t)

	var upgrades []Entry
	for _, e := range generateEnabledMatrix(t) {
		if isUpgradeFlow(e.Flow) {
			upgrades = append(upgrades, e)
		}
	}
	if len(upgrades) == 0 {
		t.Fatal("no enabled upgrade-flow scenarios found in the generated matrix")
	}

	for _, e := range upgrades {
		e := e
		t.Run(fmt.Sprintf("v%s/%s/%s/%s", e.Version, e.Shortname, e.Flow, platformOrDefault(e)), func(t *testing.T) {
			checkUpgradePrefixSubset(t, repoRoot, e)
		})
	}
}

func checkUpgradePrefixSubset(t *testing.T, repoRoot string, e Entry) {
	t.Helper()

	currentDir := scenarioFullSetupDir(repoRoot, e.Version)

	// upgrade-patch installs and upgrades within the same chart version.
	// upgrade-minor / modular-upgrade-minor install with the previous version.
	installDir := currentDir
	if e.Flow != "upgrade-patch" {
		prev, err := versionmatrix.PreviousAppVersion(e.Version)
		if err != nil {
			t.Fatalf("previous app version for %s: %v", e.Version, err)
		}
		installDir = scenarioFullSetupDir(repoRoot, prev)
	}

	cfg, err := scenarios.BuildDeploymentConfig(e.Scenario, scenarios.BuilderOverrides{
		Identity:    e.Identity,
		Persistence: e.Persistence,
		Platform:    platformOrDefault(e),
		Features:    e.Features,
		Flow:        e.Flow,
	})
	if err != nil {
		t.Fatalf("build deployment config for %s/%s: %v", e.Version, e.Scenario, err)
	}

	// A layer present on the current version may not exist on the previous
	// version (or vice versa). Skip rather than fail in that case — this test
	// asserts prefix alignment, not cross-version layer parity.
	if err := cfg.ValidateForChart(installDir); err != nil {
		t.Skipf("install-step layer missing under %s: %v", installDir, err)
	}
	if err := cfg.ValidateForChart(currentDir); err != nil {
		t.Skipf("upgrade-step layer missing under %s: %v", currentDir, err)
	}

	installCfg := *cfg
	installCfg.Upgrade = false
	installPaths, err := installCfg.ResolvePaths(installDir)
	if err != nil {
		t.Fatalf("resolve install-step paths: %v", err)
	}

	upgradeCfg := *cfg
	upgradeCfg.Upgrade = true
	upgradePaths, err := upgradeCfg.ResolvePaths(currentDir)
	if err != nil {
		t.Fatalf("resolve upgrade-step paths: %v", err)
	}

	installSet, err := collectIndexPrefixes(installPaths)
	if err != nil {
		t.Fatalf("collect install-step prefixes: %v", err)
	}
	upgradeSet, err := collectIndexPrefixes(upgradePaths)
	if err != nil {
		t.Fatalf("collect upgrade-step prefixes: %v", err)
	}

	if installOnly := prefixDiff(installSet, upgradeSet); len(installOnly) > 0 {
		t.Errorf("install-step references %v but upgrade-step does not — indices created by install will be invisible to the upgraded version", installOnly)
	}
	if upgradeOnly := prefixDiff(upgradeSet, installSet); len(upgradeOnly) > 0 {
		t.Logf("note: upgrade-step references %v not used at install-step (likely dead env-var generation)", upgradeOnly)
	}
}

func generateEnabledMatrix(t *testing.T) []Entry {
	t.Helper()
	entries, err := Generate(findRepoRoot(t), GenerateOptions{
		Versions: []string{"8.7", "8.8", "8.9", "8.10"},
	})
	if err != nil {
		t.Fatalf("generate matrix: %v", err)
	}
	return entries
}

func gapEntryPresent(entries []Entry, req gapRequirement) bool {
	for _, e := range entries {
		if e.Version == req.version &&
			e.Scenario == req.scenario &&
			e.Shortname == req.shortname &&
			e.Flow == req.flow &&
			platformOrDefault(e) == req.platform {
			return true
		}
	}
	return false
}

func isUpgradeFlow(flow string) bool {
	switch flow {
	case "upgrade-patch", "upgrade-minor", "modular-upgrade-minor":
		return true
	}
	return false
}

func platformOrDefault(e Entry) string {
	p := strings.ToLower(e.Platform)
	if p == "" {
		return "gke"
	}
	return p
}

func scenarioFullSetupDir(repoRoot, version string) string {
	return filepath.Join(repoRoot, "charts", "camunda-platform-"+version,
		"test", "integration", "scenarios", "chart-full-setup")
}

// collectIndexPrefixes walks each YAML file via yaml.v3 node traversal and
// returns the set of *_INDEX_PREFIX placeholder names referenced in scalar
// values (so commented-out lines are ignored).
func collectIndexPrefixes(paths []string) (map[string]struct{}, error) {
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
		walkScalarNodes(&root, func(s string) {
			for _, m := range indexPrefixPlaceholder.FindAllStringSubmatch(s, -1) {
				set[m[1]+"_INDEX_PREFIX"] = struct{}{}
			}
		})
	}
	return set, nil
}

func walkScalarNodes(n *yaml.Node, fn func(string)) {
	if n == nil {
		return
	}
	if n.Kind == yaml.ScalarNode {
		fn(n.Value)
		return
	}
	for _, c := range n.Content {
		walkScalarNodes(c, fn)
	}
}

func prefixDiff(a, b map[string]struct{}) []string {
	var out []string
	for k := range a {
		if _, ok := b[k]; !ok {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}
