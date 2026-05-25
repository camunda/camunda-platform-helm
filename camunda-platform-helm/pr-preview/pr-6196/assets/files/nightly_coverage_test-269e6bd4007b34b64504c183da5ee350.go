package matrix

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

	"scripts/camunda-core/pkg/scenarios"
)

type nightlyWorkflowCall struct {
	workflow  string
	job       string
	version   string
	platform  string
	scenario  string
	shortname string
	flow      string
}

// nightlyWorkflowCalls mirrors the cron-triggered workflows in
// camunda/c8-cross-component-e2e-tests/.github/workflows/playwright_sm_nightly_*.yml.
// Keep this table in sync when cross-component nightly workflow inputs change.
var nightlyWorkflowCalls = []nightlyWorkflowCall{
	{workflow: "playwright_sm_nightly_document_store_8_10.yml", job: "reusable-8-8-plus", version: "8.10", platform: "eks", scenario: "qa-document-store", shortname: "qadoc", flow: "install"},
	{workflow: "playwright_sm_nightly_document_store_8_8.yml", job: "reusable-8-8-plus", version: "8.8", platform: "eks", scenario: "qa-document-store", shortname: "qadoc", flow: "install"},
	{workflow: "playwright_sm_nightly_document_store_8_9.yml", job: "reusable-8-8-plus", version: "8.9", platform: "eks", scenario: "qa-document-store", shortname: "qadoc", flow: "install"},
	{workflow: "playwright_sm_nightly_license_8_8.yml", job: "reusable-v1", version: "8.8", platform: "gke", scenario: "qa-license-tasklist-v1", shortname: "licupg", flow: "install"},
	{workflow: "playwright_sm_nightly_license_8_8.yml", job: "reusable-v2", version: "8.8", platform: "gke", scenario: "qa-license-upg", shortname: "licupg", flow: "install"},
	{workflow: "playwright_sm_nightly_license_8_9.yml", job: "reusable-v1", version: "8.9", platform: "gke", scenario: "qa-license-tasklist-v1", shortname: "licupg", flow: "install"},
	{workflow: "playwright_sm_nightly_license_8_9.yml", job: "reusable-v2", version: "8.9", platform: "gke", scenario: "qa-license-upg", shortname: "licupg", flow: "install"},
	{workflow: "playwright_sm_nightly_mt_8_10.yml", job: "reusable", version: "8.10", platform: "gke", scenario: "qa-elasticsearch-mt", shortname: "qaelmt", flow: "install"},
	{workflow: "playwright_sm_nightly_mt_8_7.yml", job: "reusable", version: "8.7", platform: "gke", scenario: "qa-elasticsearch-mt", shortname: "qaelmt", flow: "install"},
	{workflow: "playwright_sm_nightly_mt_8_8.yml", job: "reusable-v1", version: "8.8", platform: "gke", scenario: "qa-elasticsearch-mt-tasklist-v1", shortname: "qaelmtupg", flow: "install"},
	{workflow: "playwright_sm_nightly_mt_8_8.yml", job: "reusable-v2", version: "8.8", platform: "gke", scenario: "qa-elasticsearch-mt-upg", shortname: "qaelmtupg", flow: "install"},
	{workflow: "playwright_sm_nightly_mt_8_9.yml", job: "reusable-v1", version: "8.9", platform: "gke", scenario: "qa-elasticsearch-mt-tasklist-v1", shortname: "qaelmtupg", flow: "install"},
	{workflow: "playwright_sm_nightly_mt_8_9.yml", job: "reusable-v2", version: "8.9", platform: "gke", scenario: "qa-elasticsearch-mt-upg", shortname: "qaelmtupg", flow: "install"},
	{workflow: "playwright_sm_nightly_rba_8_10.yml", job: "reusable", version: "8.10", platform: "gke", scenario: "qa-elasticsearch-rba", shortname: "qaelrba", flow: "install"},
	{workflow: "playwright_sm_nightly_rba_8_7.yml", job: "reusable", version: "8.7", platform: "gke", scenario: "qa-elasticsearch-rba", shortname: "qaelrba", flow: "install"},
	{workflow: "playwright_sm_nightly_rba_8_8.yml", job: "reusable-v1", version: "8.8", platform: "gke", scenario: "qa-elasticsearch-rba-tasklist-v1", shortname: "qaelrbaupg", flow: "install"},
	{workflow: "playwright_sm_nightly_rba_8_8.yml", job: "reusable-v2", version: "8.8", platform: "gke", scenario: "qa-elasticsearch-rba-upg", shortname: "qaelrbaupg", flow: "install"},
	{workflow: "playwright_sm_nightly_rba_8_9.yml", job: "reusable-v1", version: "8.9", platform: "gke", scenario: "qa-elasticsearch-rba-tasklist-v1", shortname: "qaelrbaupg", flow: "install"},
	{workflow: "playwright_sm_nightly_rba_8_9.yml", job: "reusable-v2", version: "8.9", platform: "gke", scenario: "qa-elasticsearch-rba-upg", shortname: "qaelrbaupg", flow: "install"},
	{workflow: "playwright_sm_nightly_tests_chrome_8_10.yml", job: "reusable-v2", version: "8.10", platform: "gke", scenario: "qa-elasticsearch-upg", shortname: "qaelupg", flow: "install"},
	{workflow: "playwright_sm_nightly_tests_chrome_8_7.yml", job: "reusable", version: "8.7", platform: "gke", scenario: "qa-elasticsearch", shortname: "qael", flow: "install"},
	{workflow: "playwright_sm_nightly_tests_chrome_8_8.yml", job: "reusable-v1", version: "8.8", platform: "gke", scenario: "qa-elasticsearch-tasklist-v1", shortname: "qaelupg", flow: "install"},
	{workflow: "playwright_sm_nightly_tests_chrome_8_8.yml", job: "reusable-v2", version: "8.8", platform: "gke", scenario: "qa-elasticsearch-upg", shortname: "qaelupg", flow: "install"},
	{workflow: "playwright_sm_nightly_tests_chrome_8_9.yml", job: "reusable-v1", version: "8.9", platform: "gke", scenario: "qa-elasticsearch-tasklist-v1", shortname: "qaelupg", flow: "install"},
	{workflow: "playwright_sm_nightly_tests_chrome_8_9.yml", job: "reusable-v2", version: "8.9", platform: "gke", scenario: "qa-elasticsearch-upg", shortname: "qaelupg", flow: "install"},
	{workflow: "playwright_sm_nightly_tests_edge_8_7.yml", job: "reusable", version: "8.7", platform: "gke", scenario: "qa-elasticsearch", shortname: "qael", flow: "install"},
	{workflow: "playwright_sm_nightly_tests_firefox_8_7.yml", job: "reusable", version: "8.7", platform: "eks", scenario: "qa-elasticsearch", shortname: "qael", flow: "install"},
	{workflow: "playwright_sm_nightly_tests_opensearch_8_10.yml", job: "reusable", version: "8.10", platform: "gke", scenario: "qa-opensearch", shortname: "qaos", flow: "install"},
	{workflow: "playwright_sm_nightly_tests_opensearch_8_7.yml", job: "reusable", version: "8.7", platform: "gke", scenario: "qa-opensearch", shortname: "qaos", flow: "install"},
	{workflow: "playwright_sm_nightly_tests_opensearch_8_8.yml", job: "reusable-v1", version: "8.8", platform: "gke", scenario: "qa-opensearch-tasklist-v1", shortname: "qaosupg", flow: "install"},
	{workflow: "playwright_sm_nightly_tests_opensearch_8_8.yml", job: "reusable-v2", version: "8.8", platform: "gke", scenario: "qa-opensearch-upg", shortname: "qaosupg", flow: "install"},
	{workflow: "playwright_sm_nightly_tests_opensearch_8_9.yml", job: "reusable-v1", version: "8.9", platform: "gke", scenario: "qa-opensearch-tasklist-v1", shortname: "qaosupg", flow: "install"},
	{workflow: "playwright_sm_nightly_tests_opensearch_8_9.yml", job: "reusable-v2", version: "8.9", platform: "gke", scenario: "qa-opensearch-upg", shortname: "qaosupg", flow: "install"},
	{workflow: "playwright_sm_nightly_upgrade_minor_8_10.yml", job: "reusable-8-9-install", version: "8.9", platform: "gke", scenario: "qa-elasticsearch", shortname: "qaelupg", flow: "install"},
	{workflow: "playwright_sm_nightly_upgrade_minor_8_10.yml", job: "reusable-8-10-upgrade", version: "8.10", platform: "gke", scenario: "qa-elasticsearch-upg", shortname: "qaelupg", flow: "modular-upgrade-minor"},
	{workflow: "playwright_sm_nightly_upgrade_minor_8_8.yml", job: "reusable-8-7-install", version: "8.7", platform: "gke", scenario: "qa-elasticsearch", shortname: "qaelupg", flow: "install"},
	{workflow: "playwright_sm_nightly_upgrade_minor_8_8.yml", job: "reusable-8-8-plus-upgrade", version: "8.8", platform: "gke", scenario: "qa-elasticsearch-upg", shortname: "qaelupg", flow: "modular-upgrade-minor"},
	{workflow: "playwright_sm_nightly_upgrade_minor_8_9.yml", job: "reusable-8-8-install", version: "8.8", platform: "gke", scenario: "qa-elasticsearch", shortname: "qaelupg", flow: "install"},
	{workflow: "playwright_sm_nightly_upgrade_minor_8_9.yml", job: "reusable-8-9-upgrade", version: "8.9", platform: "gke", scenario: "qa-elasticsearch-upg", shortname: "qaelupg", flow: "modular-upgrade-minor"},
	{workflow: "playwright_sm_nightly_upgrade_minor_document_store_8_10.yml", job: "reusable-8-9-install", version: "8.9", platform: "eks", scenario: "qa-document-store", shortname: "qadocupg", flow: "install"},
	{workflow: "playwright_sm_nightly_upgrade_minor_document_store_8_10.yml", job: "reusable-8-10-upgrade", version: "8.10", platform: "eks", scenario: "qa-document-store-upg", shortname: "qadocupg", flow: "modular-upgrade-minor"},
	{workflow: "playwright_sm_nightly_upgrade_minor_document_store_8_9.yml", job: "reusable-8-8-install", version: "8.8", platform: "eks", scenario: "qa-document-store", shortname: "qadocupg", flow: "install"},
	{workflow: "playwright_sm_nightly_upgrade_minor_document_store_8_9.yml", job: "reusable-8-9-upgrade", version: "8.9", platform: "eks", scenario: "qa-document-store-upg", shortname: "qadocupg", flow: "modular-upgrade-minor"},
	{workflow: "playwright_sm_nightly_upgrade_minor_mt_8_10.yml", job: "reusable-8-9-install", version: "8.9", platform: "gke", scenario: "qa-elasticsearch-mt", shortname: "qaelmtupg", flow: "install"},
	{workflow: "playwright_sm_nightly_upgrade_minor_mt_8_10.yml", job: "reusable-8-10-upgrade", version: "8.10", platform: "gke", scenario: "qa-elasticsearch-mt-upg", shortname: "qaelmtupg", flow: "modular-upgrade-minor"},
	{workflow: "playwright_sm_nightly_upgrade_minor_mt_8_9.yml", job: "reusable-8-8-install", version: "8.8", platform: "gke", scenario: "qa-elasticsearch-mt", shortname: "qaelmtupg", flow: "install"},
	{workflow: "playwright_sm_nightly_upgrade_minor_mt_8_9.yml", job: "reusable-8-9-upgrade", version: "8.9", platform: "gke", scenario: "qa-elasticsearch-mt-upg", shortname: "qaelmtupg", flow: "modular-upgrade-minor"},
	{workflow: "playwright_sm_nightly_upgrade_minor_opensearch_8_10.yml", job: "reusable-8-9-install", version: "8.9", platform: "gke", scenario: "qa-opensearch", shortname: "qaosupg", flow: "install"},
	{workflow: "playwright_sm_nightly_upgrade_minor_opensearch_8_10.yml", job: "reusable-8-10-upgrade", version: "8.10", platform: "gke", scenario: "qa-opensearch-upg", shortname: "qaosupg", flow: "modular-upgrade-minor"},
	{workflow: "playwright_sm_nightly_upgrade_minor_opensearch_8_9.yml", job: "reusable-8-8-install", version: "8.8", platform: "gke", scenario: "qa-opensearch", shortname: "qaosupg", flow: "install"},
	{workflow: "playwright_sm_nightly_upgrade_minor_opensearch_8_9.yml", job: "reusable-8-9-upgrade", version: "8.9", platform: "gke", scenario: "qa-opensearch-upg", shortname: "qaosupg", flow: "modular-upgrade-minor"},
}

func TestNightlyWorkflowCallsHaveCITestConfigRows(t *testing.T) {
	entries := loadAllEntriesForNightlyTests(t)

	for _, call := range nightlyWorkflowCalls {
		call := call
		t.Run(fmt.Sprintf("%s/%s", call.workflow, call.job), func(t *testing.T) {
			if _, ok := findNightlyEntry(entries, call); !ok {
				t.Fatalf("missing ci-test-config row for version=%s platform=%s scenario=%s shortname=%s flow=%s",
					call.version, call.platform, call.scenario, call.shortname, call.flow)
			}
		})
	}
}

func TestIssue6172UpgradeGapsRunInPRCI(t *testing.T) {
	entries := loadAllEntriesForNightlyTests(t)
	required := []nightlyWorkflowCall{
		{version: "8.9", platform: "gke", scenario: "qa-opensearch-upg", shortname: "qaosupg", flow: "modular-upgrade-minor"},
		{version: "8.9", platform: "eks", scenario: "qa-document-store-upg", shortname: "qadocupg", flow: "modular-upgrade-minor"},
		{version: "8.10", platform: "gke", scenario: "qa-opensearch-upg", shortname: "qaosupg", flow: "modular-upgrade-minor"},
		{version: "8.10", platform: "eks", scenario: "qa-document-store-upg", shortname: "qadocupg", flow: "modular-upgrade-minor"},
		{version: "8.10", platform: "gke", scenario: "qa-elasticsearch-mt-upg", shortname: "qaelmtupg", flow: "modular-upgrade-minor"},
	}

	for _, call := range required {
		call := call
		t.Run(fmt.Sprintf("v%s/%s/%s/%s", call.version, call.platform, call.scenario, call.shortname), func(t *testing.T) {
			entry, ok := findNightlyEntry(entries, call)
			if !ok {
				t.Fatalf("missing required issue #6172 PR CI row: %+v", call)
			}
			if !entry.Enabled {
				t.Fatalf("required issue #6172 row is present but disabled: %+v", call)
			}
		})
	}
}

func TestNightlyUpgradePrefixAlignment(t *testing.T) {
	repoRoot := mustFindRepoRoot(t)
	entries := loadAllEntriesForNightlyTests(t)
	pairs := []struct {
		name    string
		install nightlyWorkflowCall
		upgrade nightlyWorkflowCall
	}{
		{
			name:    "8.8 elasticsearch",
			install: nightlyWorkflowCall{version: "8.7", platform: "gke", scenario: "qa-elasticsearch", shortname: "qaelupg", flow: "install"},
			upgrade: nightlyWorkflowCall{version: "8.8", platform: "gke", scenario: "qa-elasticsearch-upg", shortname: "qaelupg", flow: "modular-upgrade-minor"},
		},
		{
			name:    "8.9 elasticsearch",
			install: nightlyWorkflowCall{version: "8.8", platform: "gke", scenario: "qa-elasticsearch", shortname: "qaelupg", flow: "install"},
			upgrade: nightlyWorkflowCall{version: "8.9", platform: "gke", scenario: "qa-elasticsearch-upg", shortname: "qaelupg", flow: "modular-upgrade-minor"},
		},
		{
			name:    "8.10 elasticsearch",
			install: nightlyWorkflowCall{version: "8.9", platform: "gke", scenario: "qa-elasticsearch", shortname: "qaelupg", flow: "install"},
			upgrade: nightlyWorkflowCall{version: "8.10", platform: "gke", scenario: "qa-elasticsearch-upg", shortname: "qaelupg", flow: "modular-upgrade-minor"},
		},
		{
			name:    "8.9 opensearch",
			install: nightlyWorkflowCall{version: "8.8", platform: "gke", scenario: "qa-opensearch", shortname: "qaosupg", flow: "install"},
			upgrade: nightlyWorkflowCall{version: "8.9", platform: "gke", scenario: "qa-opensearch-upg", shortname: "qaosupg", flow: "modular-upgrade-minor"},
		},
		{
			name:    "8.10 opensearch",
			install: nightlyWorkflowCall{version: "8.9", platform: "gke", scenario: "qa-opensearch", shortname: "qaosupg", flow: "install"},
			upgrade: nightlyWorkflowCall{version: "8.10", platform: "gke", scenario: "qa-opensearch-upg", shortname: "qaosupg", flow: "modular-upgrade-minor"},
		},
		{
			name:    "8.9 documentstore",
			install: nightlyWorkflowCall{version: "8.8", platform: "eks", scenario: "qa-document-store", shortname: "qadocupg", flow: "install"},
			upgrade: nightlyWorkflowCall{version: "8.9", platform: "eks", scenario: "qa-document-store-upg", shortname: "qadocupg", flow: "modular-upgrade-minor"},
		},
		{
			name:    "8.10 documentstore",
			install: nightlyWorkflowCall{version: "8.9", platform: "eks", scenario: "qa-document-store", shortname: "qadocupg", flow: "install"},
			upgrade: nightlyWorkflowCall{version: "8.10", platform: "eks", scenario: "qa-document-store-upg", shortname: "qadocupg", flow: "modular-upgrade-minor"},
		},
		{
			name:    "8.9 multitenancy",
			install: nightlyWorkflowCall{version: "8.8", platform: "gke", scenario: "qa-elasticsearch-mt", shortname: "qaelmtupg", flow: "install"},
			upgrade: nightlyWorkflowCall{version: "8.9", platform: "gke", scenario: "qa-elasticsearch-mt-upg", shortname: "qaelmtupg", flow: "modular-upgrade-minor"},
		},
		{
			name:    "8.10 multitenancy",
			install: nightlyWorkflowCall{version: "8.9", platform: "gke", scenario: "qa-elasticsearch-mt", shortname: "qaelmtupg", flow: "install"},
			upgrade: nightlyWorkflowCall{version: "8.10", platform: "gke", scenario: "qa-elasticsearch-mt-upg", shortname: "qaelmtupg", flow: "modular-upgrade-minor"},
		},
	}

	for _, pair := range pairs {
		pair := pair
		t.Run(pair.name, func(t *testing.T) {
			installEntry, ok := findNightlyEntry(entries, pair.install)
			if !ok {
				t.Fatalf("missing install entry: %+v", pair.install)
			}
			upgradeEntry, ok := findNightlyEntry(entries, pair.upgrade)
			if !ok {
				t.Fatalf("missing upgrade entry: %+v", pair.upgrade)
			}

			installPrefixes := collectEntryPrefixes(t, repoRoot, installEntry, false)
			upgradePrefixes := collectEntryPrefixes(t, repoRoot, upgradeEntry, true)
			if len(installPrefixes) == 0 && len(upgradePrefixes) == 0 {
				return
			}
			if effectivePrefixKey(installEntry) != effectivePrefixKey(upgradeEntry) {
				t.Fatalf("prefix keys differ: install=%q upgrade=%q", effectivePrefixKey(installEntry), effectivePrefixKey(upgradeEntry))
			}
			if installOnly := diffPrefixSets(installPrefixes, upgradePrefixes); len(installOnly) > 0 {
				t.Fatalf("install references prefixes %v that upgrade does not", installOnly)
			}
		})
	}
}

func loadAllEntriesForNightlyTests(t *testing.T) []Entry {
	t.Helper()
	entries, err := Generate(mustFindRepoRoot(t), GenerateOptions{
		Versions:        []string{"8.7", "8.8", "8.9", "8.10"},
		IncludeDisabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	return entries
}

func findNightlyEntry(entries []Entry, call nightlyWorkflowCall) (Entry, bool) {
	var matches []Entry
	for _, entry := range entries {
		if entry.Version == call.version &&
			entry.Scenario == call.scenario &&
			entry.Shortname == call.shortname &&
			entry.Flow == call.flow &&
			entryMatchesPlatform(entry, call.platform) {
			matches = append(matches, entry)
		}
	}
	if len(matches) == 1 {
		return matches[0], true
	}
	return Entry{}, false
}

func entryMatchesPlatform(entry Entry, platform string) bool {
	entryPlatform := strings.ToLower(entry.Platform)
	if entryPlatform == "" {
		entryPlatform = "gke"
	}
	return entryPlatform == strings.ToLower(platform)
}

func effectivePrefixKey(entry Entry) string {
	if entry.PrefixKey != "" {
		return entry.PrefixKey
	}
	return entry.Scenario
}

var prefixPlaceholder = regexp.MustCompile(`\$([A-Z][A-Z0-9_]*)_INDEX_PREFIX`)

func collectEntryPrefixes(t *testing.T, repoRoot string, entry Entry, upgradeStep bool) map[string]struct{} {
	t.Helper()
	scenarioDir := filepath.Join(repoRoot, "charts", "camunda-platform-"+entry.Version, "test/integration/scenarios/chart-full-setup")
	deployConfig, err := scenarios.BuildDeploymentConfig(entry.Scenario, scenarios.BuilderOverrides{
		Identity:    entry.Identity,
		Persistence: entry.Persistence,
		Platform:    entry.Platform,
		Features:    entry.Features,
		InfraType:   entry.InfraType,
		Flow:        entry.Flow,
		QA:          entry.QA,
		ImageTags:   entry.ImageTags,
		Upgrade:     upgradeStep,
		ChartDir:    scenarioDir,
	})
	if err != nil {
		t.Fatalf("build deployment config for %s/%s/%s: %v", entry.Version, entry.Scenario, entry.Flow, err)
	}
	paths, err := deployConfig.ResolvePaths(scenarioDir)
	if err != nil {
		t.Fatalf("resolve paths for %s/%s/%s: %v", entry.Version, entry.Scenario, entry.Flow, err)
	}
	return collectPrefixesFromFiles(t, paths)
}

func collectPrefixesFromFiles(t *testing.T, paths []string) map[string]struct{} {
	t.Helper()
	set := map[string]struct{}{}
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		var root yaml.Node
		if err := yaml.Unmarshal(data, &root); err != nil {
			t.Fatalf("parse %s: %v", p, err)
		}
		walkScalars(&root, func(s string) {
			for _, match := range prefixPlaceholder.FindAllStringSubmatch(s, -1) {
				set[match[1]+"_INDEX_PREFIX"] = struct{}{}
			}
		})
	}
	return set
}

func walkScalars(node *yaml.Node, fn func(string)) {
	if node == nil {
		return
	}
	if node.Kind == yaml.ScalarNode {
		fn(node.Value)
		return
	}
	for _, child := range node.Content {
		walkScalars(child, fn)
	}
}

func diffPrefixSets(a, b map[string]struct{}) []string {
	var diff []string
	for k := range a {
		if _, ok := b[k]; !ok {
			diff = append(diff, k)
		}
	}
	sort.Strings(diff)
	return diff
}

func mustFindRepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	dir := filepath.Dir(file)
	for {
		if _, err := os.Stat(filepath.Join(dir, "charts", "chart-versions.yaml")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("repo root not found while walking up from %s", file)
		}
		dir = parent
	}
}
