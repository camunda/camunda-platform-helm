// Copyright 2026 Camunda Services GmbH
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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// PlanOptions carries the inputs of the generate-chart-matrix composite
// action: the changed-files trigger context plus the manual overrides.
type PlanOptions struct {
	// ActiveVersions are the active chart versions (chart-versions.yaml
	// supportStandard), e.g. ["8.7", "8.8", "8.9", "8.10"].
	ActiveVersions []string
	// ChangedFiles is the raw changed-files list (whitespace-separated, as
	// emitted by tj-actions/changed-files with dir_names:true).
	ChangedFiles string
	// ManualTrigger is "none", "all", or a single chart version.
	ManualTrigger string
	// ManualScenario keeps only the exactly-matching scenario ("none"/"all"/""
	// keep everything).
	ManualScenario string
	// ManualFlow overrides the flows as a comma-separated list ("none"/""
	// keeps the registry flows).
	ManualFlow string
	// Tier filters scenarios by tier (0 = all).
	Tier int
}

// PlanEntry is one GitHub Actions matrix include entry. Every field is a
// string because GHA matrix values are compared as strings (the bash
// implementation ran `walk(tostring)` over the JSON for the same reason).
type PlanEntry struct {
	Version                   string `json:"version"`
	CamundaVersionPrevious    string `json:"camundaVersionPrevious"`
	Case                      string `json:"case"`
	Platforms                 string `json:"platforms"`
	Scenario                  string `json:"scenario"`
	Shortname                 string `json:"shortname"`
	Auth                      string `json:"auth"`
	Flow                      string `json:"flow"`
	Exclude                   string `json:"exclude"`
	InfraTypeGke              string `json:"infraTypeGke"`
	InfraTypeEks              string `json:"infraTypeEks"`
	Identity                  string `json:"identity"`
	Persistence               string `json:"persistence"`
	Features                  string `json:"features"`
	QA                        string `json:"qa"`
	Upgrade                   string `json:"upgrade"`
	SkipE2E                   string `json:"skipE2E"`
	IsTopology                string `json:"isTopology"`
	TopologyNamespaceSuffixes string `json:"topologyNamespaceSuffixes"`
	TopologyHubSuffix         string `json:"topologyHubSuffix"`
	TopologySmokeMatrix       string `json:"topologySmokeMatrix"`
	HelmVersion               string `json:"helmVersion"`
}

type topologySmokeEntry struct {
	OrchestrationSuffix string `json:"orchestration_suffix"`
	ModelerClusterID    string `json:"modeler_cluster_id"`
	ModelerClusterName  string `json:"modeler_cluster_name"`
	ShardIndex          string `json:"shard_index"`
	// Optimize is empty unless role "optimize" releases declare
	// `serves: <orchestration_suffix>`. When populated, this leg's Optimize
	// instances run in their own namespaces on the Hub host rather than in the
	// orchestration namespace, so an e2e run must not derive an Optimize
	// endpoint from OrchestrationSuffix. One entry per Physical Tenant served by
	// this orchestration release, in declaration order.
	Optimize []topologySmokeOptimize `json:"optimize,omitempty"`
}

type topologySmokeOptimize struct {
	Suffix      string `json:"suffix"`
	ContextPath string `json:"context_path"`
}

// PlanResult is the computed build matrix.
type PlanResult struct {
	Include []PlanEntry
	// Versions are the unique chart versions present in Include, sorted to
	// match the legacy jq `unique` output.
	Versions []string
}

// MatrixJSON returns the {"include": [...]} document consumed as the GHA
// job matrix.
func (r PlanResult) MatrixJSON() (string, error) {
	include := r.Include
	if include == nil {
		include = []PlanEntry{}
	}
	b, err := json.Marshal(map[string][]PlanEntry{"include": include})
	if err != nil {
		return "", fmt.Errorf("marshal matrix: %w", err)
	}
	return string(b), nil
}

// VersionsJSON returns the unique chart versions as a JSON array.
func (r PlanResult) VersionsJSON() (string, error) {
	versions := r.Versions
	if versions == nil {
		versions = []string{}
	}
	b, err := json.Marshal(versions)
	if err != nil {
		return "", fmt.Errorf("marshal versions: %w", err)
	}
	return string(b), nil
}

// buildAllTrigger is a changed-files rule that selects every active chart
// version. Exclude carves paths out of Pattern: the rule only fires when at
// least one changed path matches Pattern without also matching Exclude.
type buildAllTrigger struct {
	Pattern     *regexp.Regexp
	Exclude     *regexp.Regexp
	Description string
}

var chartCIWorkflows = []string{
	"build-ci-runner-image",
	"chart-validate-template",
	"cleanup-namespace",
	"integration-tests-gate",
	"test-chart-version",
	"test-chart-version-template",
	"test-gitops-phase",
	"test-integration-cleanup-template",
	"test-integration-diagnostics-template",
	"test-integration-runner",
	"test-integration-template",
	"test-local-template",
	"test-unit-template",
}

var deployRelevantScriptDirs = []string{
	"camunda-core",
	"ci-result-cache",
	"deploy-camunda",
	"gitops-phase-test",
	"integration-tests-gate",
	"playwright-pin",
	"prepare-helm-values",
	"validate-values-schema",
	"vault-secret-mapper",
}

var deployRelevantScriptFiles = []string{
	"base_playwright_script.sh",
	"check-no-plaintext-datastore.sh",
	"check-values-enterprise.sh",
	"check-values-latest.sh",
	"deploy-camunda.sh",
	"dns-fallback.cjs",
	"download-chart-docker-images.sh",
	"harbor-retry.sh",
	"prepare-helm-values.sh",
	"render-e2e-env.sh",
	"run-e2e-tests.sh",
}

func alternation(names []string) string {
	quoted := make([]string, 0, len(names))
	for _, name := range names {
		quoted = append(quoted, regexp.QuoteMeta(name))
	}
	return strings.Join(quoted, "|")
}

// buildAllTriggers mirrors the BUILD_ALL_TRIGGERS list of the bash
// implementation. Patterns are matched per changed path, like grep.
var buildAllTriggers = []buildAllTrigger{
	{
		Pattern:     regexp.MustCompile(`^\.github/workflows/(` + alternation(chartCIWorkflows) + `)\.yaml$`),
		Description: ".github/workflows (chart-CI workflows)",
	},
	{
		Pattern:     regexp.MustCompile(`^\.github/actions/`),
		Description: ".github/actions",
	},
	{
		Pattern:     regexp.MustCompile(`\.github/config`),
		Exclude:     regexp.MustCompile(`\.github/config/release-please`),
		Description: ".github/config (excluding release-please)",
	},
	{
		Pattern:     regexp.MustCompile(`^scripts/(` + alternation(deployRelevantScriptDirs) + `)/`),
		Description: "scripts/ (deployer and its runtime dependencies)",
	},
	{
		Pattern:     regexp.MustCompile(`^scripts/(` + alternation(deployRelevantScriptFiles) + `)$`),
		Description: "scripts/ (deploy and e2e helpers)",
	},
	{
		Pattern:     regexp.MustCompile(`^\.tool-versions$`),
		Description: ".tool-versions (toolchain pins)",
	},
	{
		Pattern:     regexp.MustCompile(`^test/e2e/`),
		Description: "test/e2e/ (shared Playwright config)",
	},
}

var chartDocPath = regexp.MustCompile(`^charts/camunda-platform-8[^/]*/[^/]+\.(md|MD|txt)$`)

var manualFlowPattern = regexp.MustCompile(`^(install|upgrade-patch|upgrade-minor)(,(install|upgrade-patch|upgrade-minor))*$`)

// Plan computes the chart build matrix for a change set, replacing
// scripts/generate-chart-matrix.sh + generate-chart-matrix.jq.
func Plan(repoRoot string, opts PlanOptions) (PlanResult, error) {
	versions, err := selectPlanVersions(repoRoot, opts)
	if err != nil {
		return PlanResult{}, err
	}
	if len(versions) == 0 {
		return PlanResult{}, nil
	}

	pf, err := LoadPermittedFlows(repoRoot)
	if err != nil {
		return PlanResult{}, err
	}

	manualScenario := opts.ManualScenario
	includeDisabled := manualScenario != "" && manualScenario != "none" && manualScenario != "all"

	var result PlanResult
	for _, version := range versions {
		entries, err := Generate(repoRoot, GenerateOptions{
			Versions:        []string{version},
			IncludeDisabled: includeDisabled,
		})
		if err != nil {
			return PlanResult{}, err
		}
		entries = Filter(entries, FilterOptions{Tier: opts.Tier})

		if includeDisabled {
			var kept []Entry
			for _, e := range entries {
				if e.Scenario == manualScenario {
					kept = append(kept, e)
				}
			}
			entries = kept
		}

		entries, err = applyManualFlow(entries, opts.ManualFlow)
		if err != nil {
			return PlanResult{}, err
		}

		var kept []Entry
		for _, e := range entries {
			if planSkipped(e) {
				continue
			}
			// Re-check permitted-flows: a manual-flow override can inject
			// flows the Generate pre-filter never saw.
			if len(FilterFlows(pf, version, []string{e.Flow})) == 0 {
				continue
			}
			kept = append(kept, e)
		}

		result.Include = append(result.Include, groupPlanEntries(version, kept)...)
	}

	seen := map[string]bool{}
	for _, e := range result.Include {
		if !seen[e.Version] {
			seen[e.Version] = true
			result.Versions = append(result.Versions, e.Version)
		}
	}
	sort.Strings(result.Versions)
	return result, nil
}

// selectPlanVersions decides which chart versions to build: the manual
// trigger wins, otherwise the changed-files rules apply.
func selectPlanVersions(repoRoot string, opts PlanOptions) ([]string, error) {
	switch {
	case opts.ManualTrigger == "all":
		return opts.ActiveVersions, nil
	case opts.ManualTrigger != "" && opts.ManualTrigger != "none":
		chartDir := filepath.Join(repoRoot, "charts", "camunda-platform-"+opts.ManualTrigger)
		if fi, err := os.Stat(chartDir); err != nil || !fi.IsDir() {
			return nil, fmt.Errorf("chart directory %s does not exist", chartDir)
		}
		return []string{opts.ManualTrigger}, nil
	}

	var paths []string
	for _, path := range strings.Fields(opts.ChangedFiles) {
		if !chartDocPath.MatchString(path) {
			paths = append(paths, path)
		}
	}
	for _, trigger := range buildAllTriggers {
		if triggerFires(trigger, paths) {
			return opts.ActiveVersions, nil
		}
	}

	var versions []string
	for _, version := range opts.ActiveVersions {
		needle := "charts/camunda-platform-" + version
		for _, path := range paths {
			if strings.Contains(path, needle) {
				versions = append(versions, version)
				break
			}
		}
	}
	return versions, nil
}

func triggerFires(trigger buildAllTrigger, paths []string) bool {
	matched := false
	matchedOutsideExclude := false
	for _, path := range paths {
		if !trigger.Pattern.MatchString(path) {
			continue
		}
		matched = true
		if trigger.Exclude == nil || !trigger.Exclude.MatchString(path) {
			matchedOutsideExclude = true
		}
	}
	return matched && matchedOutsideExclude
}

// applyManualFlow expands each entry into one entry per override flow.
func applyManualFlow(entries []Entry, manualFlow string) ([]Entry, error) {
	if manualFlow == "" || manualFlow == "none" {
		return entries, nil
	}
	if !manualFlowPattern.MatchString(manualFlow) {
		return nil, fmt.Errorf("invalid flow %q; valid flows: install, upgrade-patch, upgrade-minor (modular-upgrade-minor only via integration-test-template.yaml)", manualFlow)
	}
	flows := strings.Split(manualFlow, ",")
	var out []Entry
	for _, e := range entries {
		for _, flow := range flows {
			expanded := e
			expanded.Flow = flow
			out = append(out, expanded)
		}
	}
	return out, nil
}

// planSkipped preserves the special-case scenario+flow exclusions:
// keycloak-original / keycloak-mt + upgrade-patch (released chart templates
// don't support custom realm bootstrapping) and oidc + upgrade-minor
// (requires Entra client setup not yet configured).
func planSkipped(e Entry) bool {
	if e.Flow == "upgrade-patch" && (e.Scenario == "keycloak-original" || e.Scenario == "keycloak-mt") {
		return true
	}
	return e.Flow == "upgrade-minor" && e.Scenario == "oidc"
}

// groupPlanEntries folds the per-(scenario, flow, platform) entries into one
// matrix entry per (scenario, shortname, flow), collecting platforms into a
// CSV and per-platform infra types into the legacy fields. First-occurrence
// order is preserved.
func groupPlanEntries(version string, entries []Entry) []PlanEntry {
	type groupKey struct{ scenario, shortname, flow string }
	groups := map[groupKey][]Entry{}
	var order []groupKey
	for _, e := range entries {
		key := groupKey{e.Scenario, e.Shortname, e.Flow}
		if _, ok := groups[key]; !ok {
			order = append(order, key)
		}
		groups[key] = append(groups[key], e)
	}

	out := make([]PlanEntry, 0, len(order))
	for _, key := range order {
		group := groups[key]
		first := group[0]

		var platforms []string
		platformSeen := map[string]bool{}
		infraGke, infraEks := "", ""
		for _, e := range group {
			if e.Platform != "" && !platformSeen[e.Platform] {
				platformSeen[e.Platform] = true
				platforms = append(platforms, e.Platform)
			}
			if e.Platform == "gke" && infraGke == "" {
				infraGke = e.InfraType
			}
			if e.Platform == "eks" && infraEks == "" {
				infraEks = e.InfraType
			}
		}
		platformsCSV := strings.Join(platforms, ",")
		if platformsCSV == "" {
			platformsCSV = "gke"
		}
		if infraGke == "" {
			infraGke = "preemptible"
		}
		if infraEks == "" {
			infraEks = "preemptible"
		}

		topologyNamespaceSuffixes, topologyHubSuffix, topologySmokeMatrix := planTopologyMetadata(first.Topology)

		out = append(out, PlanEntry{
			Version:                   version,
			CamundaVersionPrevious:    previousMinor(version),
			Case:                      "pr",
			Platforms:                 platformsCSV,
			Scenario:                  first.Scenario,
			Shortname:                 first.Shortname,
			Auth:                      first.Auth,
			Flow:                      first.Flow,
			Exclude:                   strings.Join(first.Exclude, "|"),
			InfraTypeGke:              infraGke,
			InfraTypeEks:              infraEks,
			Identity:                  first.Identity,
			Persistence:               first.Persistence,
			Features:                  strings.Join(first.Features, ","),
			QA:                        strconv.FormatBool(first.QA),
			Upgrade:                   strconv.FormatBool(first.Upgrade),
			SkipE2E:                   strconv.FormatBool(first.SkipE2E),
			IsTopology:                strconv.FormatBool(first.Topology != nil),
			TopologyNamespaceSuffixes: topologyNamespaceSuffixes,
			TopologyHubSuffix:         topologyHubSuffix,
			TopologySmokeMatrix:       topologySmokeMatrix,
			HelmVersion:               first.HelmVersion,
		})
	}
	return out
}

func planTopologyMetadata(topology *Topology) (string, string, string) {
	suffixes := []string{}
	smoke := []topologySmokeEntry{}
	hubSuffix := ""
	if topology != nil {
		optimizeByServed := map[string][]topologySmokeOptimize{}
		for _, release := range topology.Releases {
			if release.Role == "optimize" && release.Serves != "" {
				optimizeByServed[release.Serves] = append(optimizeByServed[release.Serves], topologySmokeOptimize{
					Suffix:      release.NamespaceSuffix,
					ContextPath: release.OptimizeContextPath,
				})
			}
		}
		for _, release := range topology.Releases {
			suffixes = append(suffixes, release.NamespaceSuffix)
			if release.Role == "hub" {
				hubSuffix = release.NamespaceSuffix
			}
			if release.Role == "orchestration" {
				entry := topologySmokeEntry{
					OrchestrationSuffix: release.NamespaceSuffix,
					ModelerClusterID:    release.ModelerClusterID,
					ModelerClusterName:  release.ModelerClusterName,
					ShardIndex:          strconv.Itoa(len(smoke) + 1),
				}
				entry.Optimize = optimizeByServed[release.NamespaceSuffix]
				smoke = append(smoke, entry)
			}
		}
	}
	suffixesJSON, _ := json.Marshal(suffixes)
	smokeJSON, _ := json.Marshal(smoke)
	return string(suffixesJSON), hubSuffix, string(smokeJSON)
}

// previousMinor computes the previous chart minor version ("8.10" → "8.9").
func previousMinor(version string) string {
	parts := strings.SplitN(version, ".", 2)
	if len(parts) != 2 {
		return version
	}
	major, err1 := strconv.Atoi(parts[0])
	minor, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return version
	}
	return fmt.Sprintf("%d.%d", major, minor-1)
}
