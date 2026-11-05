// Copyright 2026 Camunda Services GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package matrix

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"scripts/deploy-camunda/config"
	"scripts/deploy-camunda/deploy"
)

func testTopologyReleases() []TopologyRelease {
	return []TopologyRelease{
		{
			Role:            "hub",
			NamespaceSuffix: "hub",
			Features:        []string{"multinamespace-hub"},
			Identity:        "keycloak",
			Dependencies:    []string{"keycloak", "postgresql", "elasticsearch"},
			ResolvedDependencies: []ChartDependency{
				{ReleaseName: "keycloak"},
				{ReleaseName: "postgresql"},
				{ReleaseName: "elasticsearch"},
			},
		},
		{
			Role:            "orchestration",
			NamespaceSuffix: "orcha",
			Features:        []string{"multinamespace-orchestration"},
			Identity:        "keycloak-external",
			Persistence:     "elasticsearch-external",
			DependsOn:       "hub",
		},
		{
			Role:            "orchestration",
			NamespaceSuffix: "orchb",
			Features:        []string{"multinamespace-orchestration"},
			Identity:        "keycloak-external",
			Persistence:     "elasticsearch-external",
			DependsOn:       "hub",
		},
	}
}

func testTopologyRun(crossRefEnv map[string]string, opts RunOptions, releases []TopologyRelease, contexts []*deploy.ScenarioContext) *topologyRun {
	return &topologyRun{
		entry:       Entry{Topology: &Topology{Releases: releases}},
		opts:        opts,
		platform:    "gke",
		contexts:    contexts,
		crossRefEnv: crossRefEnv,
	}
}

func TestTopologyDeployLevels_HubFirstThenOrchestrationsTogether(t *testing.T) {
	levels, err := topologyDeployLevels(testTopologyReleases())
	if err != nil {
		t.Fatalf("topologyDeployLevels() unexpected error: %v", err)
	}
	if want := [][]int{{0}, {1, 2}}; !reflect.DeepEqual(levels, want) {
		t.Fatalf("levels = %v, want %v", levels, want)
	}
}

func TestTopologyDeployLevels_OptimizeWaitsForServedOrchestration(t *testing.T) {
	releases := []TopologyRelease{
		{Role: "hub", NamespaceSuffix: "hub"},
		{Role: "orchestration", NamespaceSuffix: "orcha", DependsOn: "hub"},
		{Role: "orchestration", NamespaceSuffix: "orchb", DependsOn: "hub"},
		{Role: "optimize", NamespaceSuffix: "opta", Serves: "orcha", DependsOn: "hub"},
		{Role: "optimize", NamespaceSuffix: "optb", Serves: "orchb", DependsOn: "hub"},
	}
	levels, err := topologyDeployLevels(releases)
	if err != nil {
		t.Fatalf("topologyDeployLevels() unexpected error: %v", err)
	}
	if want := [][]int{{0}, {1, 2}, {3, 4}}; !reflect.DeepEqual(levels, want) {
		t.Fatalf("levels = %v, want %v", levels, want)
	}
}

func TestTopologyDeployLevels_ChainedDependency(t *testing.T) {
	releases := []TopologyRelease{
		{Role: "orchestration", NamespaceSuffix: "orchb", DependsOn: "hub"},
		{Role: "hub", NamespaceSuffix: "hub"},
		{Role: "aux", NamespaceSuffix: "aux", DependsOn: "orchestration"},
	}
	levels, err := topologyDeployLevels(releases)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := [][]int{{1}, {0}, {2}}; !reflect.DeepEqual(levels, want) {
		t.Fatalf("levels = %v, want hub(1) < orchestration(0) < aux(2) as %v", levels, want)
	}
}

func TestTopologyDeployLevels_Errors(t *testing.T) {
	cases := map[string]struct {
		releases []TopologyRelease
		want     string
	}{
		"cycle": {
			releases: []TopologyRelease{
				{Role: "a", NamespaceSuffix: "a", DependsOn: "b"},
				{Role: "b", NamespaceSuffix: "b", DependsOn: "a"},
			},
			want: "cycle",
		},
		"unknown depends-on role": {
			releases: []TopologyRelease{
				{Role: "orchestration", NamespaceSuffix: "orcha", DependsOn: "hub"},
			},
			want: `depends-on "hub"`,
		},
		"unknown served suffix": {
			releases: []TopologyRelease{
				{Role: "hub", NamespaceSuffix: "hub"},
				{Role: "optimize", NamespaceSuffix: "opta", Serves: "orcha", DependsOn: "hub"},
			},
			want: `serves "orcha"`,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := topologyDeployLevels(tc.releases)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("topologyDeployLevels() error = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}

func testPreparedTopologyRelease(role, suffix string) preparedTopologyRelease {
	return preparedTopologyRelease{
		release:   TopologyRelease{Role: role, NamespaceSuffix: suffix},
		flags:     &config.RuntimeFlags{Deployment: config.DeploymentFlags{Namespace: suffix}},
		namespace: suffix,
	}
}

func TestDeployTopologyLevels_RunsReleasesOfALevelConcurrently(t *testing.T) {
	hub := testPreparedTopologyRelease("hub", "hub")
	orchestrations := []preparedTopologyRelease{
		testPreparedTopologyRelease("orchestration", "orcha"),
		testPreparedTopologyRelease("orchestration", "orchb"),
		testPreparedTopologyRelease("orchestration", "orchc"),
	}

	var mu sync.Mutex
	var started []string
	arrived := make(chan struct{}, len(orchestrations))
	unblock := make(chan struct{})
	var unblockOnce sync.Once
	release := func() { unblockOnce.Do(func() { close(unblock) }) }
	t.Cleanup(release)

	original := executePreparedFn
	t.Cleanup(func() { executePreparedFn = original })
	executePreparedFn = func(_ context.Context, _ *deploy.PreparedScenario, flags *config.RuntimeFlags) error {
		mu.Lock()
		started = append(started, flags.Deployment.Namespace)
		mu.Unlock()
		if flags.Deployment.Namespace == "hub" {
			return nil
		}
		arrived <- struct{}{}
		<-unblock
		return nil
	}

	done := make(chan error, 1)
	go func() {
		done <- deployTopologyLevels(context.Background(), "multinamespace-2orch", [][]preparedTopologyRelease{{hub}, orchestrations})
	}()
	for range orchestrations {
		select {
		case <-arrived:
		case <-time.After(5 * time.Second):
			t.Fatal("the releases of one level did not all start before any of them finished")
		}
	}
	release()
	if err := <-done; err != nil {
		t.Fatalf("deployTopologyLevels() unexpected error: %v", err)
	}
	if len(started) != 4 || started[0] != "hub" {
		t.Fatalf("started = %v, want hub first and then all three orchestration releases", started)
	}
}

func TestDeployTopologyLevels_FailureFinishesItsLevelAndStopsTheNext(t *testing.T) {
	var mu sync.Mutex
	var deployed []string
	original := executePreparedFn
	t.Cleanup(func() { executePreparedFn = original })
	executePreparedFn = func(_ context.Context, _ *deploy.PreparedScenario, flags *config.RuntimeFlags) error {
		mu.Lock()
		deployed = append(deployed, flags.Deployment.Namespace)
		mu.Unlock()
		if flags.Deployment.Namespace == "orchb" {
			return errors.New("helm install timed out")
		}
		return nil
	}

	levels := [][]preparedTopologyRelease{
		{testPreparedTopologyRelease("orchestration", "orcha"), testPreparedTopologyRelease("orchestration", "orchb")},
		{testPreparedTopologyRelease("optimize", "opta")},
	}
	err := deployTopologyLevels(context.Background(), "multinamespace-optimize", levels)
	if err == nil || !strings.Contains(err.Error(), `"orchb"`) || !strings.Contains(err.Error(), "helm install timed out") {
		t.Fatalf("deployTopologyLevels() error = %v, want the orchb deploy failure", err)
	}
	slices.Sort(deployed)
	if want := []string{"orcha", "orchb"}; !slices.Equal(deployed, want) {
		t.Fatalf("deployed = %v, want %v and no release of the next level", deployed, want)
	}
}

func TestUpdateTopologyChartDependencies_UpdatesEachChartPathOnceAndSkipsItAtDeploy(t *testing.T) {
	release := func(chartPath string, skip bool) preparedTopologyRelease {
		return preparedTopologyRelease{flags: &config.RuntimeFlags{Chart: config.ChartFlags{ChartPath: chartPath, SkipDependencyUpdate: skip}}}
	}
	releases := []preparedTopologyRelease{
		release("charts/camunda-platform-8.10", false),
		release("charts/camunda-platform-8.10", false),
		release("charts/camunda-platform-8.9", false),
		release("charts/camunda-platform-8.8", true),
	}

	var updated []string
	original := dependencyUpdateFn
	t.Cleanup(func() { dependencyUpdateFn = original })
	dependencyUpdateFn = func(_ context.Context, chartPath string) error {
		updated = append(updated, chartPath)
		return nil
	}

	if err := updateTopologyChartDependencies(context.Background(), releases); err != nil {
		t.Fatalf("updateTopologyChartDependencies() unexpected error: %v", err)
	}
	if want := []string{"charts/camunda-platform-8.10", "charts/camunda-platform-8.9"}; !slices.Equal(updated, want) {
		t.Fatalf("updated = %v, want %v", updated, want)
	}
	for i, r := range releases {
		if !r.flags.Chart.SkipDependencyUpdate {
			t.Fatalf("release[%d] still runs its own dependency update during the concurrent deploy", i)
		}
	}
}

func TestRunTopologyEntry_RejectsUnsupportedAuthFlow(t *testing.T) {
	base := Entry{
		Version:  "8.10",
		Scenario: "multinamespace",
		Auth:     "oidc", // not keycloak
		Flow:     "install",
		Topology: &Topology{Name: "t", Releases: testTopologyReleases()},
	}
	if err := runTopologyEntry(context.Background(), base, RunOptions{}); err == nil {
		t.Fatal("expected error for auth=oidc, got nil")
	}

	base.Auth = "keycloak"
	base.Flow = "upgrade-minor" // not install
	if err := runTopologyEntry(context.Background(), base, RunOptions{}); err == nil {
		t.Fatal("expected error for flow=upgrade-minor, got nil")
	}
}

func TestRunTopologyEntry_RejectsChartRef(t *testing.T) {
	entry := Entry{
		Version:  "8.10",
		Scenario: "multinamespace",
		Auth:     "keycloak",
		Flow:     "install",
		Topology: &Topology{Name: "t", Releases: testTopologyReleases()},
	}
	opts := RunOptions{ChartRef: "oci://example/camunda-platform"}

	err := runTopologyEntry(context.Background(), entry, opts)
	if err == nil {
		t.Fatal("expected error when --chart-ref is set, got nil")
	}
}

func TestRunTopologyEntry_RejectsCleanup(t *testing.T) {
	entry := Entry{
		Version:  "8.10",
		Scenario: "multinamespace",
		Auth:     "keycloak",
		Flow:     "install",
		Topology: &Topology{Name: "t", Releases: testTopologyReleases()},
	}
	opts := RunOptions{Cleanup: true}

	err := runTopologyEntry(context.Background(), entry, opts)
	if err == nil {
		t.Fatal("expected error when --cleanup is set, got nil")
	}
}

// writeTopologyHookScript writes a post-deploy script into the version-scoped pre-setup-scripts
// directory RunDeclarativePostDeployHook resolves against, and returns the repo root holding it.
func writeTopologyHookScript(t *testing.T, body string) string {
	t.Helper()
	repoRoot := t.TempDir()
	scriptDir := filepath.Join(repoRoot, "charts", "camunda-platform-8.10", "test", "integration", "scenarios", "pre-setup-scripts")
	if err := os.MkdirAll(scriptDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(scriptDir, "post-deploy-topology.sh"), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return repoRoot
}

func topologyHookReleases() []preparedTopologyRelease {
	return []preparedTopologyRelease{
		{
			release: TopologyRelease{Role: "hub", NamespaceSuffix: "hub"},
			flags: &config.RuntimeFlags{
				Deployment: config.DeploymentFlags{Namespace: "matrix-810-pt-hub"},
				ExtraEnv:   map[string]string{"HUB_NAMESPACE": "matrix-810-pt-hub"},
			},
		},
		{
			release: TopologyRelease{Role: "orchestration", NamespaceSuffix: "orcha"},
			flags: &config.RuntimeFlags{
				Deployment: config.DeploymentFlags{Namespace: "matrix-810-pt-orcha"},
				ExtraEnv: map[string]string{
					"HUB_NAMESPACE":               "matrix-810-pt-hub",
					"ORCH_NAMESPACE":              "matrix-810-pt-orcha",
					"OPTTA_OPTIMIZE_CONTEXT_PATH": "/optimize-ta",
				},
			},
		},
	}
}

// The topology post-deploy hook must actually run once the whole topology is deployed.
// synthesizeReleaseEntry clears PostDeploy on every synthesized release so it does not fire per
// release against a half-built topology, which left it running nowhere at all: registration was
// the only dispatch, and it was registered against a nil hook.
func TestRunTopologyPostDeployHook_RunsAgainstOrchestrationRelease(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "hook-ran")
	repoRoot := writeTopologyHookScript(t, "#!/bin/bash\nset -eu\n"+
		`test "$TEST_NAMESPACE" = "matrix-810-pt-orcha"`+"\n"+
		`test "$HUB_NAMESPACE" = "matrix-810-pt-hub"`+"\n"+
		`test "$OPTTA_OPTIMIZE_CONTEXT_PATH" = "/optimize-ta"`+"\n"+
		"touch "+marker+"\n")

	entry := Entry{
		Version:    "8.10",
		Scenario:   "physicaltenants",
		PostDeploy: &LifecycleHook{Script: "post-deploy-topology.sh", Description: "Topology-wide post-deploy assertions."},
	}
	if err := runTopologyPostDeployHook(context.Background(), entry, RunOptions{RepoRoot: repoRoot}, topologyHookReleases()); err != nil {
		t.Fatalf("runTopologyPostDeployHook() error = %v", err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("post-deploy hook did not run: %v", err)
	}
}

// A failing hook has to fail the topology entry. physicaltenants sets skip-e2e, so if the hook's
// exit status were swallowed the scenario would report success having asserted nothing.
func TestRunTopologyPostDeployHook_FailurePropagates(t *testing.T) {
	repoRoot := writeTopologyHookScript(t, "#!/bin/bash\nexit 3\n")

	entry := Entry{
		Version:    "8.10",
		Scenario:   "physicaltenants",
		PostDeploy: &LifecycleHook{Script: "post-deploy-topology.sh", Description: "Topology-wide post-deploy assertions."},
	}
	err := runTopologyPostDeployHook(context.Background(), entry, RunOptions{RepoRoot: repoRoot}, topologyHookReleases())
	if err == nil {
		t.Fatal("runTopologyPostDeployHook() = nil, want the hook's failure to fail the topology entry")
	}
	if !strings.Contains(err.Error(), "post-deploy hook") {
		t.Errorf("runTopologyPostDeployHook() error = %q, want it to name the post-deploy hook", err)
	}
}

// A scenario without a post-deploy hook must stay a no-op.
func TestRunTopologyPostDeployHook_NoHookIsNoop(t *testing.T) {
	if err := runTopologyPostDeployHook(context.Background(), Entry{Version: "8.10", Scenario: "multinamespace"}, RunOptions{RepoRoot: t.TempDir()}, topologyHookReleases()); err != nil {
		t.Fatalf("runTopologyPostDeployHook() with no hook error = %v", err)
	}
}

// The hook inspects orchestration workloads, so it must target the orchestration release even
// though the Hub deploys first and heads the prepared slice.
func TestTopologyHookFlags_PrefersOrchestration(t *testing.T) {
	flags := topologyHookFlags(topologyHookReleases())
	if flags == nil || flags.EffectiveNamespace() != "matrix-810-pt-orcha" {
		t.Fatalf("topologyHookFlags() namespace = %v, want the orchestration release's namespace", flags)
	}
	if topologyHookFlags(nil) != nil {
		t.Error("topologyHookFlags(nil) should report that there is no release to run against")
	}
}

// A topology renders every release's contract before any of them deploys, so the
// chart directories the render needs must already have their subchart dependencies
// vendored. CI vendors only the matrix entry's own chart version, which left a
// release pinning chart-version to render against an empty charts/ directory and
// fail with "missing in charts/ directory: keycloak, postgresql, ...".
// topologyChartPaths is what turns that into one EnsureDependencies call per chart.
func TestTopologyChartPaths(t *testing.T) {
	chart := func(chartPath, chartRef string) preparedTopologyRelease {
		return preparedTopologyRelease{
			flags: &config.RuntimeFlags{
				Chart: config.ChartFlags{ChartPath: chartPath, Chart: chartRef},
			},
		}
	}

	tests := []struct {
		name     string
		releases []preparedTopologyRelease
		want     []string
	}{
		{
			name: "a pinned chart-version contributes its own chart directory",
			releases: []preparedTopologyRelease{
				chart("/repo/charts/camunda-platform-8.10", ""),
				chart("/repo/charts/camunda-platform-8.10", ""),
				chart("/repo/charts/camunda-platform-8.9", ""),
			},
			want: []string{"/repo/charts/camunda-platform-8.10", "/repo/charts/camunda-platform-8.9"},
		},
		{
			name: "a uniform topology yields the single shared chart directory",
			releases: []preparedTopologyRelease{
				chart("/repo/charts/camunda-platform-8.10", ""),
				chart("/repo/charts/camunda-platform-8.10", ""),
			},
			want: []string{"/repo/charts/camunda-platform-8.10"},
		},
		{
			name: "an external chart reference has nothing to vendor locally",
			releases: []preparedTopologyRelease{
				chart("/repo/charts/camunda-platform-8.10", "oci://registry/camunda/camunda-platform"),
			},
			want: []string{},
		},
		{
			name:     "releases without flags or a chart path are skipped",
			releases: []preparedTopologyRelease{{}, chart("", "")},
			want:     []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := topologyChartPaths(tt.releases)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("topologyChartPaths() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTopologyReleaseContextsPopulateIngressHost(t *testing.T) {
	releases := testTopologyReleases()
	baseEntry := Entry{
		Version:   "8.10",
		ChartPath: "charts/camunda-platform-8.10",
		Scenario:  "multinamespace",
		Shortname: "mns",
		Auth:      "keycloak",
		Flow:      "install",
		Topology:  &Topology{Name: "multinamespace", Releases: releases},
	}

	cases := []struct {
		name string
		opts RunOptions
	}{
		{
			name: "per-release namespace-derived hosts",
			opts: RunOptions{
				RepoRoot:        "/repo",
				NamespacePrefix: "matrix",
				Infra: config.InfraOverride{
					InfraConfig: config.InfraConfig{IngressBaseDomain: "ci.distro.ultrawombat.com"},
					InfraMaps:   config.InfraMaps{IngressBaseDomains: map[string]string{"gke": "ci.distro.ultrawombat.com"}},
				},
			},
		},
		{
			name: "explicit global.host alongside a base domain, as CI passes it",
			opts: RunOptions{
				RepoRoot:        "/repo",
				NamespacePrefix: "matrix",
				Infra:           config.InfraOverride{InfraConfig: config.InfraConfig{IngressBaseDomain: "ci.distro.ultrawombat.com"}},
				ExtraHelmSets:   []string{"global.host=abc123-mns.ci.distro.ultrawombat.com"},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			errStubPrepare := errors.New("prepare stubbed out: this test never deploys")

			type preparedRelease struct {
				namespace   string
				ingressHost string
				wantHost    string
			}
			var recorded []preparedRelease

			original := prepareScenarioFn
			t.Cleanup(func() { prepareScenarioFn = original })
			prepareScenarioFn = func(_ context.Context, scenarioCtx *deploy.ScenarioContext, flags *config.RuntimeFlags) (*deploy.PreparedScenario, error) {
				recorded = append(recorded, preparedRelease{
					namespace:   scenarioCtx.Namespace,
					ingressHost: scenarioCtx.IngressHost,
					wantHost:    flags.ResolveIngressHostname(),
				})
				return nil, errStubPrepare
			}

			// Every release fails to prepare, so the returned error is the stub's own
			// and carries no signal. The recorded pairs are the subject.
			_ = runTopologyEntry(context.Background(), baseEntry, tc.opts)

			if len(recorded) == 0 {
				t.Fatal("prepareScenarioFn was never called — runTopologyEntry aborted before preparing any release, so the invariant was never exercised")
			}
			for _, rec := range recorded {
				if rec.wantHost == "" {
					t.Fatalf("release %q: flags.ResolveIngressHostname() is empty — the fixture does not exercise a real host", rec.namespace)
				}
				if rec.ingressHost != rec.wantHost {
					t.Errorf("release %q: ScenarioContext.IngressHost = %q, want %q — buildScenarioEnv only emits CAMUNDA_HOSTNAME when this is non-empty", rec.namespace, rec.ingressHost, rec.wantHost)
				}
			}
		})
	}
}
