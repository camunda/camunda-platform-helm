// Copyright 2025 Camunda Services GmbH and/or licensed to Camunda
// Services GmbH under one or more contributor license agreements.
// Licensed under the Apache License, Version 2.0.

package matrix

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// nightlyScenario defines a single nightly workflow step that must resolve to at least
// one entry in the Helm chart's ci-test-config.yaml via deploy-camunda matrix run.
//
// Each entry mirrors the exact filters the nightly workflow passes:
//   --scenario-filter <Scenario> --shortname-filter <Shortname> --shortname-exact
//   --flow-filter <Flow> --platform <Platform> --versions <Version> --include-disabled
type nightlyScenario struct {
	// Description is the human-readable name shown in test output.
	Description string
	// Version is the chart version the nightly resolves against (e.g., "8.8").
	Version string
	// Scenario is the --scenario-filter value (substring match).
	Scenario string
	// Shortname is the --shortname-filter value (exact match when ShortnameExact is true).
	Shortname string
	// Flow is the --flow-filter value (exact match).
	Flow string
	// Platform is the --platform value (exact match).
	Platform string
}

// knownNightlyScenarios lists every nightly workflow step that calls
// test-integration-template.yaml → test-integration-runner.yaml →
// deploy-camunda matrix run. Each entry must have a matching ci-test-config entry.
//
// Source: c8-cross-component-e2e-tests/.github/workflows/playwright_sm_nightly_*.yml
//
// When adding a new nightly workflow in the E2E repo, add the corresponding
// entry here so this test catches any missing ci-test-config entries before CI fails.
var knownNightlyScenarios = []nightlyScenario{
	// ===== DocumentStore install nightlies =====
	{
		Description: "DS 8.8 install",
		Version:     "8.8",
		Scenario:    "qa-document-store",
		Shortname:   "qadoc",
		Flow:        "install",
		Platform:    "gke",
	},
	{
		Description: "DS 8.9 install",
		Version:     "8.9",
		Scenario:    "qa-document-store",
		Shortname:   "qadoc",
		Flow:        "install",
		Platform:    "gke",
	},
	{
		Description: "DS 8.10 install",
		Version:     "8.10",
		Scenario:    "qa-document-store",
		Shortname:   "qadoc",
		Flow:        "install",
		Platform:    "gke",
	},

	// ===== Firefox 8.7 nightly =====
	{
		Description: "Firefox 8.7",
		Version:     "8.7",
		Scenario:    "qa-elasticsearch",
		Shortname:   "qael",
		Flow:        "install",
		Platform:    "gke",
	},

	// ===== Upgrade DS 8.9 nightly (8.8 install → 8.9 upgrade) =====
	{
		Description: "Upgrade DS 8.9 — install step (8.8)",
		Version:     "8.8",
		Scenario:    "qa-document-store",
		Shortname:   "qadocupg",
		Flow:        "install",
		Platform:    "gke",
	},
	{
		Description: "Upgrade DS 8.9 — upgrade step (8.9)",
		Version:     "8.9",
		Scenario:    "qa-document-store-upg",
		Shortname:   "qadocupg",
		Flow:        "modular-upgrade-minor",
		Platform:    "gke",
	},

	// ===== Upgrade DS 8.10 nightly (8.9 install → 8.10 upgrade) =====
	{
		Description: "Upgrade DS 8.10 — install step (8.9)",
		Version:     "8.9",
		Scenario:    "qa-document-store",
		Shortname:   "qadocupg",
		Flow:        "install",
		Platform:    "gke",
	},
	{
		Description: "Upgrade DS 8.10 — upgrade step (8.10)",
		Version:     "8.10",
		Scenario:    "qa-document-store-upg",
		Shortname:   "qadocupg",
		Flow:        "modular-upgrade-minor",
		Platform:    "gke",
	},

	// ===== ES upgrade nightlies =====
	{
		Description: "Upgrade ES 8.8 — install step (8.7)",
		Version:     "8.7",
		Scenario:    "qa-elasticsearch",
		Shortname:   "qaelupg",
		Flow:        "install",
		Platform:    "gke",
	},
	{
		Description: "Upgrade ES 8.9 — install step (8.8)",
		Version:     "8.8",
		Scenario:    "qa-elasticsearch",
		Shortname:   "qaelupg",
		Flow:        "install",
		Platform:    "gke",
	},
	{
		Description: "Upgrade ES 8.10 — install step (8.9)",
		Version:     "8.9",
		Scenario:    "qa-elasticsearch",
		Shortname:   "qaelupg",
		Flow:        "install",
		Platform:    "gke",
	},

	// ===== MT upgrade nightlies =====
	{
		Description: "Upgrade MT 8.9 — install step (8.8)",
		Version:     "8.8",
		Scenario:    "qa-elasticsearch-mt",
		Shortname:   "qaelmtupg",
		Flow:        "install",
		Platform:    "gke",
	},
	{
		Description: "Upgrade MT 8.10 — install step (8.9)",
		Version:     "8.9",
		Scenario:    "qa-elasticsearch-mt",
		Shortname:   "qaelmtupg",
		Flow:        "install",
		Platform:    "gke",
	},
}

// TestNightlyScenarioCoverage verifies that every known nightly workflow step
// resolves to at least one ci-test-config.yaml entry via the same Generate + Filter
// pipeline that deploy-camunda matrix run uses.
//
// This test catches:
//   - Missing backward-compat entries (new nightly workflow added without a matching entry)
//   - Platform mismatches (nightly requests eks but entry only has [gke])
//   - Flow mismatches (nightly requests modular-upgrade-minor but entry only has install)
//   - Shortname typos
func TestNightlyScenarioCoverage(t *testing.T) {
	repoRoot := findRepoRootForNightly(t)

	for _, ns := range knownNightlyScenarios {
		t.Run(ns.Description, func(t *testing.T) {
			entries, err := Generate(repoRoot, GenerateOptions{
				Versions:        []string{ns.Version},
				IncludeDisabled: true,
			})
			if err != nil {
				t.Fatalf("Generate(version=%s): %v", ns.Version, err)
			}

			filtered := Filter(entries, FilterOptions{
				ScenarioFilter:  ns.Scenario,
				ShortnameFilter: ns.Shortname,
				ShortnameExact:  true,
				FlowFilter:      ns.Flow,
				Platform:        ns.Platform,
			})

			if len(filtered) == 0 {
				t.Errorf("no ci-test-config.yaml entry matched nightly scenario filters\n"+
					"  version:   %s\n"+
					"  scenario:  %s\n"+
					"  shortname: %s\n"+
					"  flow:      %s\n"+
					"  platform:  %s\n"+
					"\n"+
					"This means the nightly workflow will fail with:\n"+
					"  Error: no matrix entries matched the filters\n"+
					"\n"+
					"Fix: add a matching entry to charts/camunda-platform-%s/test/ci-test-config.yaml\n"+
					"in the backward-compat section with the shortname, flow, and platform above.",
					ns.Version, ns.Scenario, ns.Shortname, ns.Flow, ns.Platform, ns.Version,
				)
			}
		})
	}
}

// TestNightlyScenarioNoDuplicateDescriptions ensures that every entry in
// knownNightlyScenarios has a unique Description field (prevents copy-paste errors).
func TestNightlyScenarioNoDuplicateDescriptions(t *testing.T) {
	seen := make(map[string]bool)
	for _, ns := range knownNightlyScenarios {
		if seen[ns.Description] {
			t.Errorf("duplicate Description in knownNightlyScenarios: %q", ns.Description)
		}
		seen[ns.Description] = true
	}
}

// findRepoRootForNightly mirrors findRepoRoot but is defined here to avoid
// depending on test-internal helpers from the main test file.
func findRepoRootForNightly(t *testing.T) string {
	t.Helper()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}

	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "charts", "chart-versions.yaml")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	t.Skip("cannot find repo root (charts/chart-versions.yaml); skipping integration test")
	return ""
}

// TestNightlyScenarioTableCompleteness is a meta-test that prints a summary of
// coverage. It lists which versions have nightly scenarios defined and helps
// identify gaps when new nightly workflows are added.
func TestNightlyScenarioTableCompleteness(t *testing.T) {
	versionCounts := make(map[string]int)
	for _, ns := range knownNightlyScenarios {
		versionCounts[ns.Version]++
	}

	repoRoot := findRepoRootForNightly(t)
	cv, err := LoadChartVersions(repoRoot)
	if err != nil {
		t.Fatalf("LoadChartVersions: %v", err)
	}

	for _, v := range cv.ActiveVersions() {
		count := versionCounts[v]
		if count == 0 {
			t.Logf("INFO: version %s has no nightly scenarios defined in knownNightlyScenarios — "+
				"add entries if nightly workflows exist for this version", v)
		} else {
			t.Logf("INFO: version %s has %d nightly scenario entries", v, count)
		}
	}

	t.Logf("\nTo add coverage for a new nightly workflow:\n"+
		"1. Read the nightly YAML to extract scenario, shortname, flows, and platform\n"+
		"2. Add entries to knownNightlyScenarios in nightly_coverage_test.go\n"+
		"3. Run: go test -run TestNightlyScenarioCoverage -v\n"+
		"4. Fix any missing ci-test-config.yaml entries flagged by failures")
}

func init() {
	// Verify all entries have required fields populated.
	for i, ns := range knownNightlyScenarios {
		if ns.Description == "" || ns.Version == "" || ns.Scenario == "" ||
			ns.Shortname == "" || ns.Flow == "" || ns.Platform == "" {
			panic(fmt.Sprintf("knownNightlyScenarios[%d] has empty required fields: %+v", i, ns))
		}
	}
}
