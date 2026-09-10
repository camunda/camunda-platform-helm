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
	"sort"
	"strings"
	"testing"
)

// The cases below replicate test/scripts/generate_chart_matrix.bats against
// the real scenario registry, so the Go port keeps the exact behavior of
// scripts/generate-chart-matrix.sh + generate-chart-matrix.jq.

var planActiveVersions = mustLoadPlanActiveVersions()

func mustLoadPlanActiveVersions() []string {
	chartVersions, err := LoadChartVersions("../../..")
	if err != nil {
		panic(err)
	}
	versions := chartVersions.ActiveVersions()
	if len(versions) == 0 {
		panic("chart-versions.yaml contains no active versions")
	}
	return versions
}

func planVersionsIn(entries []PlanEntry) []string {
	seen := map[string]bool{}
	var out []string
	for _, e := range entries {
		if !seen[e.Version] {
			seen[e.Version] = true
			out = append(out, e.Version)
		}
	}
	return out
}

func planFlowsIn(entries []PlanEntry) map[string]bool {
	flows := map[string]bool{}
	for _, e := range entries {
		flows[e.Flow] = true
	}
	return flows
}

func TestPlanManualAllBuildsNonEmpty(t *testing.T) {
	result, err := Plan(findRepoRoot(t), PlanOptions{ActiveVersions: planActiveVersions, ManualTrigger: "all"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Include) == 0 {
		t.Fatal("expected non-empty matrix")
	}
	if len(result.Versions) != len(planActiveVersions) {
		t.Errorf("versions = %v, want all of %v", result.Versions, planActiveVersions)
	}
	wantVersions := append([]string(nil), planActiveVersions...)
	sort.Strings(wantVersions)
	if got, want := strings.Join(result.Versions, ","), strings.Join(wantVersions, ","); got != want {
		t.Errorf("versions = %s, want legacy sorted order %s", got, want)
	}
}

func TestPlanManualSingleVersion(t *testing.T) {
	result, err := Plan(findRepoRoot(t), PlanOptions{ActiveVersions: planActiveVersions, ManualTrigger: "8.9"})
	if err != nil {
		t.Fatal(err)
	}
	if got := planVersionsIn(result.Include); len(got) != 1 || got[0] != "8.9" {
		t.Errorf("versions = %v, want [8.9]", got)
	}
}

func TestPlanManualScenarioExact(t *testing.T) {
	result, err := Plan(findRepoRoot(t), PlanOptions{
		ActiveVersions: planActiveVersions,
		ManualTrigger:  "8.10",
		ManualScenario: "rdbms",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Include) == 0 {
		t.Fatal("expected rdbms entries")
	}
	for _, entry := range result.Include {
		if entry.Scenario != "rdbms" {
			t.Errorf("scenario = %q, want rdbms", entry.Scenario)
		}
	}
}

func TestPlanTopologyWorkflowMetadata(t *testing.T) {
	result, err := Plan(findRepoRoot(t), PlanOptions{
		ActiveVersions: planActiveVersions,
		ManualTrigger:  "8.10",
		ManualScenario: "multinamespace-2orch",
		ManualFlow:     "install",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Include) != 1 {
		t.Fatalf("entries = %d, want 1", len(result.Include))
	}
	entry := result.Include[0]
	if entry.IsTopology != "true" {
		t.Errorf("isTopology = %q, want true", entry.IsTopology)
	}
	if entry.TopologyNamespaceSuffixes != `["hub","orcha","orchb","orchc","orchd"]` {
		t.Errorf("topologyNamespaceSuffixes = %q", entry.TopologyNamespaceSuffixes)
	}
	if entry.TopologyHubSuffix != "hub" {
		t.Errorf("topologyHubSuffix = %q, want hub", entry.TopologyHubSuffix)
	}
	wantSmoke := `[{"orchestration_suffix":"orcha","modeler_cluster_id":"orcha","modeler_cluster_name":"Orchestration A","shard_index":"1","chart_version":"8.10","chart_dir":"camunda-platform-8.10","suite":"orchestration","test_chart_dir":"camunda-platform-8.10","playwright_project":"topology-orchestration"},{"orchestration_suffix":"orchb","modeler_cluster_id":"orchb","modeler_cluster_name":"Orchestration B","shard_index":"2","chart_version":"8.9","chart_dir":"camunda-platform-8.9","suite":"orchestration","test_chart_dir":"camunda-platform-8.9","playwright_project":"topology-orchestration"},{"orchestration_suffix":"orchc","modeler_cluster_id":"orchc","modeler_cluster_name":"Orchestration C","shard_index":"3","chart_version":"8.8","chart_dir":"camunda-platform-8.8","suite":"orchestration","test_chart_dir":"camunda-platform-8.8","playwright_project":"topology-orchestration"},{"orchestration_suffix":"orchd","modeler_cluster_id":"orchd","modeler_cluster_name":"Orchestration D","shard_index":"4","chart_version":"8.7","chart_dir":"camunda-platform-8.7","suite":"orchestration","test_chart_dir":"camunda-platform-8.7","playwright_project":"topology-orchestration"}]`
	if entry.TopologySmokeMatrix != wantSmoke {
		t.Errorf("topologySmokeMatrix = %q, want %q", entry.TopologySmokeMatrix, wantSmoke)
	}
}

func TestPlanTopologyMetadataUsesReleaseChartVersion(t *testing.T) {
	_, _, smoke := planTopologyMetadata("8.10", &Topology{Releases: []TopologyRelease{
		{Role: "orchestration", NamespaceSuffix: "current", ModelerClusterID: "current", ModelerClusterName: "Current"},
		{ChartVersion: "8.9", Role: "orchestration", NamespaceSuffix: "previous", ModelerClusterID: "previous", ModelerClusterName: "Previous"},
	}})
	want := `[{"orchestration_suffix":"current","modeler_cluster_id":"current","modeler_cluster_name":"Current","shard_index":"1","chart_version":"8.10","chart_dir":"camunda-platform-8.10","suite":"orchestration","test_chart_dir":"camunda-platform-8.10","playwright_project":"topology-orchestration"},{"orchestration_suffix":"previous","modeler_cluster_id":"previous","modeler_cluster_name":"Previous","shard_index":"2","chart_version":"8.9","chart_dir":"camunda-platform-8.9","suite":"orchestration","test_chart_dir":"camunda-platform-8.9","playwright_project":"topology-orchestration"}]`
	if smoke != want {
		t.Fatalf("smoke = %s, want %s", smoke, want)
	}
}

func TestPlanOrdinaryScenarioHasEmptyTopologyWorkflowMetadata(t *testing.T) {
	result, err := Plan(findRepoRoot(t), PlanOptions{
		ActiveVersions: planActiveVersions,
		ManualTrigger:  "8.10",
		ManualScenario: "elasticsearch",
		ManualFlow:     "install",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Include) != 1 {
		t.Fatalf("entries = %d, want 1", len(result.Include))
	}
	entry := result.Include[0]
	if entry.IsTopology != "false" || entry.TopologyNamespaceSuffixes != "[]" || entry.TopologyHubSuffix != "" || entry.TopologySmokeMatrix != "[]" {
		t.Errorf("unexpected topology metadata: %+v", entry)
	}
}

func TestPlanManualScenarioIncludesDisabled(t *testing.T) {
	result, err := Plan(findRepoRoot(t), PlanOptions{
		ActiveVersions: planActiveVersions,
		ManualTrigger:  "8.10",
		ManualScenario: "elasticsearch-basic",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Include) == 0 {
		t.Fatal("expected disabled elasticsearch-basic entries")
	}
	for _, entry := range result.Include {
		if entry.Scenario != "elasticsearch-basic" {
			t.Errorf("scenario = %q, want elasticsearch-basic", entry.Scenario)
		}
	}
}

func TestPlanManualTriggerUnknownVersionFails(t *testing.T) {
	_, err := Plan(findRepoRoot(t), PlanOptions{ActiveVersions: planActiveVersions, ManualTrigger: "9.99"})
	if err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("err = %v, want chart-directory error", err)
	}
}

func TestPlanChangedFilesSingleChart(t *testing.T) {
	result, err := Plan(findRepoRoot(t), PlanOptions{
		ActiveVersions: planActiveVersions,
		ManualTrigger:  "none",
		ChangedFiles:   "charts/camunda-platform-8.9/templates/any.yaml",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := planVersionsIn(result.Include); len(got) != 1 || got[0] != "8.9" {
		t.Errorf("versions = %v, want [8.9]", got)
	}
}

func TestPlanScriptsChangeTriggersAll(t *testing.T) {
	// Regression for #6108: a top-level scripts/ change is invoked by chart
	// tests (e.g. render-e2e-env.sh) and must rebuild every version.
	result, err := Plan(findRepoRoot(t), PlanOptions{
		ActiveVersions: planActiveVersions,
		ManualTrigger:  "none",
		ChangedFiles:   "scripts/render-e2e-env.sh",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := planVersionsIn(result.Include); len(got) != len(planActiveVersions) {
		t.Errorf("versions = %v, want all of %v", got, planActiveVersions)
	}
}

func TestPlanNonDeployScriptsNoTrigger(t *testing.T) {
	result, err := Plan(findRepoRoot(t), PlanOptions{
		ActiveVersions: planActiveVersions,
		ManualTrigger:  "none",
		ChangedFiles:   "scripts/notify-pr-activity/main.go scripts/release-tools/pkg/harbor/client.go",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := planVersionsIn(result.Include); len(got) != 0 {
		t.Errorf("versions = %v, want none", got)
	}
}

func TestPlanNonChartWorkflowNoTrigger(t *testing.T) {
	result, err := Plan(findRepoRoot(t), PlanOptions{
		ActiveVersions: planActiveVersions,
		ManualTrigger:  "none",
		ChangedFiles:   ".github/workflows/repo-pr-labeler.yaml",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := planVersionsIn(result.Include); len(got) != 0 {
		t.Errorf("versions = %v, want none", got)
	}
}

func TestPlanChartCIWorkflowTriggersAll(t *testing.T) {
	result, err := Plan(findRepoRoot(t), PlanOptions{
		ActiveVersions: planActiveVersions,
		ManualTrigger:  "none",
		ChangedFiles:   ".github/workflows/test-integration-runner.yaml",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := planVersionsIn(result.Include); len(got) != len(planActiveVersions) {
		t.Errorf("versions = %v, want all of %v", got, planActiveVersions)
	}
}

func TestPlanActionChangeTriggersAll(t *testing.T) {
	result, err := Plan(findRepoRoot(t), PlanOptions{
		ActiveVersions: planActiveVersions,
		ManualTrigger:  "none",
		ChangedFiles:   ".github/actions/playwright-e2e-tests/action.yaml",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := planVersionsIn(result.Include); len(got) != len(planActiveVersions) {
		t.Errorf("versions = %v, want all of %v", got, planActiveVersions)
	}
}

func TestPlanChartDocOnlyEmptyMatrix(t *testing.T) {
	result, err := Plan(findRepoRoot(t), PlanOptions{
		ActiveVersions: planActiveVersions,
		ManualTrigger:  "none",
		ChangedFiles:   "charts/camunda-platform-8.10/README.md charts/camunda-platform-8.10/RELEASE-NOTES.md",
	})
	if err != nil {
		t.Fatal(err)
	}
	matrixJSON, err := result.MatrixJSON()
	if err != nil {
		t.Fatal(err)
	}
	if matrixJSON != `{"include":[]}` {
		t.Errorf("matrix = %s, want empty include", matrixJSON)
	}
}

func TestPlanChartDocWithTemplateChangeBuildsVersion(t *testing.T) {
	result, err := Plan(findRepoRoot(t), PlanOptions{
		ActiveVersions: planActiveVersions,
		ManualTrigger:  "none",
		ChangedFiles:   "charts/camunda-platform-8.10/README.md charts/camunda-platform-8.10/templates/common/_helpers.tpl",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := planVersionsIn(result.Include); len(got) != 1 || got[0] != "8.10" {
		t.Errorf("versions = %v, want [8.10]", got)
	}
}

func TestPlanChartNotesTxtIsNotDoc(t *testing.T) {
	result, err := Plan(findRepoRoot(t), PlanOptions{
		ActiveVersions: planActiveVersions,
		ManualTrigger:  "none",
		ChangedFiles:   "charts/camunda-platform-8.10/templates/NOTES.txt",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := planVersionsIn(result.Include); len(got) != 1 || got[0] != "8.10" {
		t.Errorf("versions = %v, want [8.10]", got)
	}
}

func TestPlanToolVersionsTriggersAll(t *testing.T) {
	result, err := Plan(findRepoRoot(t), PlanOptions{
		ActiveVersions: planActiveVersions,
		ManualTrigger:  "none",
		ChangedFiles:   ".tool-versions",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := planVersionsIn(result.Include); len(got) != len(planActiveVersions) {
		t.Errorf("versions = %v, want all of %v", got, planActiveVersions)
	}
}

func TestPlanSharedE2EConfigTriggersAll(t *testing.T) {
	result, err := Plan(findRepoRoot(t), PlanOptions{
		ActiveVersions: planActiveVersions,
		ManualTrigger:  "none",
		ChangedFiles:   "test/e2e/playwright.base.config.ts",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := planVersionsIn(result.Include); len(got) != len(planActiveVersions) {
		t.Errorf("versions = %v, want all of %v", got, planActiveVersions)
	}
}

func TestPlanChartScopedE2EPathOnlyTriggersChart(t *testing.T) {
	result, err := Plan(findRepoRoot(t), PlanOptions{
		ActiveVersions: planActiveVersions,
		ManualTrigger:  "none",
		ChangedFiles:   "charts/camunda-platform-8.10/test/e2e/foo.spec.ts",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := planVersionsIn(result.Include); len(got) != 1 || got[0] != "8.10" {
		t.Errorf("versions = %v, want [8.10]", got)
	}
}

func TestPlanUnrelatedPathEmptyMatrix(t *testing.T) {
	result, err := Plan(findRepoRoot(t), PlanOptions{
		ActiveVersions: planActiveVersions,
		ManualTrigger:  "none",
		ChangedFiles:   "README.md",
	})
	if err != nil {
		t.Fatal(err)
	}
	matrixJSON, err := result.MatrixJSON()
	if err != nil {
		t.Fatal(err)
	}
	if matrixJSON != `{"include":[]}` {
		t.Errorf("matrix = %s, want empty include", matrixJSON)
	}
}

func TestPlanScriptsSubstringNoTrigger(t *testing.T) {
	// Anchored regex must not match e.g. "myscripts" or "descripts/foo".
	result, err := Plan(findRepoRoot(t), PlanOptions{
		ActiveVersions: planActiveVersions,
		ManualTrigger:  "none",
		ChangedFiles:   "myscripts/foo.sh",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Include) != 0 {
		t.Errorf("expected empty matrix, got %d entries", len(result.Include))
	}
}

func TestPlanChartScriptsPathOnlyTriggersChart(t *testing.T) {
	result, err := Plan(findRepoRoot(t), PlanOptions{
		ActiveVersions: planActiveVersions,
		ManualTrigger:  "none",
		ChangedFiles:   "charts/camunda-platform-8.10/test/scripts",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := planVersionsIn(result.Include); len(got) != 1 || got[0] != "8.10" {
		t.Errorf("versions = %v, want [8.10]", got)
	}
}

func TestPlanConfigChangeExcludingReleasePlease(t *testing.T) {
	// Only release-please config changed: the config trigger must not fire.
	result, err := Plan(findRepoRoot(t), PlanOptions{
		ActiveVersions: planActiveVersions,
		ManualTrigger:  "none",
		ChangedFiles:   ".github/config/release-please/config.json",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Include) != 0 {
		t.Errorf("release-please-only change must not trigger builds, got %d entries", len(result.Include))
	}

	// The action emits space-separated paths. A second, non-excluded config
	// path must fire the trigger even when an excluded path appears first.
	result, err = Plan(findRepoRoot(t), PlanOptions{
		ActiveVersions: planActiveVersions,
		ManualTrigger:  "none",
		ChangedFiles:   ".github/config/release-please/config.json .github/config/infra.yaml",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := planVersionsIn(result.Include); len(got) != len(planActiveVersions) {
		t.Errorf("versions = %v, want all of %v", got, planActiveVersions)
	}
}

func TestPlanManualFlowSingle(t *testing.T) {
	result, err := Plan(findRepoRoot(t), PlanOptions{
		ActiveVersions: planActiveVersions, ManualTrigger: "8.9", ManualFlow: "upgrade-minor",
	})
	if err != nil {
		t.Fatal(err)
	}
	flows := planFlowsIn(result.Include)
	if len(flows) != 1 || !flows["upgrade-minor"] {
		t.Errorf("flows = %v, want only upgrade-minor", flows)
	}
}

func TestPlanManualFlowMultiple(t *testing.T) {
	result, err := Plan(findRepoRoot(t), PlanOptions{
		ActiveVersions: planActiveVersions, ManualTrigger: "8.9", ManualFlow: "install,upgrade-patch",
	})
	if err != nil {
		t.Fatal(err)
	}
	flows := planFlowsIn(result.Include)
	if !flows["install"] || !flows["upgrade-patch"] || len(flows) != 2 {
		t.Errorf("flows = %v, want install+upgrade-patch", flows)
	}
}

func TestPlanInvalidManualFlow(t *testing.T) {
	_, err := Plan(findRepoRoot(t), PlanOptions{
		ActiveVersions: planActiveVersions, ManualTrigger: "8.9", ManualFlow: "bogus-flow",
	})
	if err == nil || !strings.Contains(err.Error(), "invalid flow") {
		t.Fatalf("err = %v, want invalid-flow error", err)
	}
}

func TestPlanPermittedFlowsDenyUpgradeMinorForOldVersions(t *testing.T) {
	result, err := Plan(findRepoRoot(t), PlanOptions{
		ActiveVersions: planActiveVersions, ManualTrigger: "8.7",
		ManualFlow: "install,upgrade-patch,upgrade-minor",
	})
	if err != nil {
		t.Fatal(err)
	}
	if flows := planFlowsIn(result.Include); flows["upgrade-minor"] {
		t.Errorf("flows = %v, upgrade-minor must be denied for <=8.7", flows)
	}
}

func TestPlanPermittedFlowsDenyUpgradePatchFor810(t *testing.T) {
	result, err := Plan(findRepoRoot(t), PlanOptions{
		ActiveVersions: planActiveVersions, ManualTrigger: "8.10",
		ManualFlow: "install,upgrade-patch,upgrade-minor",
	})
	if err != nil {
		t.Fatal(err)
	}
	if flows := planFlowsIn(result.Include); flows["upgrade-patch"] {
		t.Errorf("flows = %v, upgrade-patch must be denied for ==8.10", flows)
	}
}

func TestPlanKeycloakOriginalSkipsUpgradePatch(t *testing.T) {
	result, err := Plan(findRepoRoot(t), PlanOptions{ActiveVersions: planActiveVersions, ManualTrigger: "8.7"})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range result.Include {
		if e.Scenario == "keycloak-original" {
			found = true
			if e.Flow != "install" {
				t.Errorf("keycloak-original entry with flow %q, want install only", e.Flow)
			}
		}
	}
	if !found {
		t.Skip("keycloak-original not present in the 8.7 registry")
	}
}

func TestPlanKeycloakMtSkipsUpgradePatchEvenWithManualFlow(t *testing.T) {
	result, err := Plan(findRepoRoot(t), PlanOptions{
		ActiveVersions: planActiveVersions, ManualTrigger: "8.7", ManualFlow: "install,upgrade-patch",
	})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range result.Include {
		if e.Scenario == "keycloak-mt" {
			found = true
			if e.Flow != "install" {
				t.Errorf("keycloak-mt entry with flow %q, want install only", e.Flow)
			}
		}
	}
	if !found {
		t.Skip("keycloak-mt not present in the 8.7 registry")
	}
}

func TestPlanOidcUpgradeMinorExcluded(t *testing.T) {
	result, err := Plan(findRepoRoot(t), PlanOptions{ActiveVersions: planActiveVersions, ManualTrigger: "8.10"})
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range result.Include {
		if e.Scenario == "oidc" && e.Flow == "upgrade-minor" {
			t.Error("oidc + upgrade-minor must be excluded")
		}
	}
}

func TestPlanEntryFields(t *testing.T) {
	result, err := Plan(findRepoRoot(t), PlanOptions{ActiveVersions: planActiveVersions, ManualTrigger: "8.10"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Include) == 0 {
		t.Fatal("expected entries")
	}
	e := result.Include[0]
	if e.CamundaVersionPrevious != "8.9" {
		t.Errorf("camundaVersionPrevious = %q, want 8.9", e.CamundaVersionPrevious)
	}
	if e.Case != "pr" {
		t.Errorf("case = %q, want pr", e.Case)
	}
	if e.InfraTypeGke == "" || e.InfraTypeEks == "" {
		t.Errorf("infra types must default, got gke=%q eks=%q", e.InfraTypeGke, e.InfraTypeEks)
	}
	if e.Platforms == "" {
		t.Error("platforms must default to gke")
	}
	for _, v := range []string{e.QA, e.Upgrade, e.SkipE2E} {
		if v != "true" && v != "false" {
			t.Errorf("boolean fields must be stringified, got %q", v)
		}
	}
	versionsJSON, err := result.VersionsJSON()
	if err != nil {
		t.Fatal(err)
	}
	if versionsJSON != `["8.10"]` {
		t.Errorf("camunda-versions = %s, want [\"8.10\"]", versionsJSON)
	}
}

func TestPlanTierFilter(t *testing.T) {
	all, err := Plan(findRepoRoot(t), PlanOptions{ActiveVersions: planActiveVersions, ManualTrigger: "8.10"})
	if err != nil {
		t.Fatal(err)
	}
	tier1, err := Plan(findRepoRoot(t), PlanOptions{ActiveVersions: planActiveVersions, ManualTrigger: "8.10", Tier: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(tier1.Include) == 0 || len(tier1.Include) > len(all.Include) {
		t.Errorf("tier-1 entries = %d, all = %d; want 0 < tier1 <= all", len(tier1.Include), len(all.Include))
	}
}

func TestPlanTierOneIncludesMixedTopology(t *testing.T) {
	result, err := Plan(findRepoRoot(t), PlanOptions{
		ActiveVersions: planActiveVersions,
		ManualTrigger:  "8.10",
		Tier:           1,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range result.Include {
		if entry.Scenario == "multinamespace-2orch" {
			return
		}
	}
	t.Fatal("tier-1 matrix does not include multinamespace-2orch")
}
