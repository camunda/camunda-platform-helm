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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"scripts/camunda-core/pkg/helm"
	"scripts/deploy-camunda/config"
	"scripts/deploy-camunda/deploy"
	"slices"
	"sync"

	"gopkg.in/yaml.v3"
)

func RunTopologies(ctx context.Context, entries []Entry, opts RunOptions) error {
	if len(entries) == 0 {
		return nil
	}
	if opts.DryRun || opts.Coverage {
		label := "dry-run"
		if !opts.DryRun && opts.Coverage {
			label = "coverage"
		}
		fmt.Fprintf(os.Stdout, "\n=== Topology entries (%s — listing only, no deploy) ===\n", label)
		for _, entry := range entries {
			fmt.Fprintf(os.Stdout, "%s/%s (%s): %d releases\n", entry.Version, entry.Scenario, entry.Shortname, len(entry.Topology.Releases))
		}
		return nil
	}
	opts.EnsureDockerRegistry = true
	for _, entry := range entries {
		if err := runTopologyEntry(ctx, entry, opts); err != nil {
			return fmt.Errorf("topology entry %s/%s failed: %w", entry.Version, entry.Scenario, err)
		}
	}
	return nil
}

var prepareScenarioFn = deploy.PrepareScenario

var executePreparedFn = deploy.ExecutePrepared

var dependencyUpdateFn = helm.DependencyUpdate

// preparedTopologyRelease pairs a topology release with the flags and prepared scenario built for
// it, so the deploy loop and the topology-level post-deploy hook can both address it.
type preparedTopologyRelease struct {
	release   TopologyRelease
	flags     *config.RuntimeFlags
	namespace string
	prepared  *deploy.PreparedScenario
}

// topologyChartPaths returns the distinct local chart directories the topology's
// releases render from, in deploy order. A release pinning its own chart-version
// contributes a second directory, which is why this is not simply the entry's
// chart path. Releases rendered from an external chart reference (OCI or .tgz)
// have nothing to vendor locally and are skipped.
func topologyChartPaths(releases []preparedTopologyRelease) []string {
	seen := make(map[string]struct{}, len(releases))
	paths := make([]string, 0, len(releases))
	for _, release := range releases {
		if release.flags == nil || release.flags.Chart.Chart != "" || release.flags.Chart.ChartPath == "" {
			continue
		}
		chartPath := release.flags.Chart.ChartPath
		if _, ok := seen[chartPath]; ok {
			continue
		}
		seen[chartPath] = struct{}{}
		paths = append(paths, chartPath)
	}
	return paths
}

type topologyRun struct {
	entry                Entry
	opts                 RunOptions
	platform             string
	baseNamespace        string
	contexts             []*deploy.ScenarioContext
	crossRefEnv          map[string]string
	orchestrationIndices []int
	prepared             []preparedTopologyRelease
	preparedLevels       [][]preparedTopologyRelease
	removers             []func()
}

// runTopologyEntry drives a topology-bearing Entry's multi-namespace
// fan-out: one Entry becomes N Helm releases (one per topology
// release) instead of Run's usual one-namespace-per-entry deploy.
//
// opts is the SAME fully-built RunOptions the normal (single-namespace)
// `matrix run` path constructs — every
// per-release RunOptions is a copy of it with only Platform/NamespaceOverride
// overridden (see synthesizeReleaseOpts). Passing the whole struct through,
// rather than hand-picking a subset into a bespoke options type, is what
// prevents fields from silently dropping for topology deploys (this class of
// bug has already bitten IngressBaseDomains, HelmTimeout, and
// DeleteNamespaceFirst).
//
// Each release gets its OWN identity/persistence/features/dependencies layer
// selection (per TopologyRelease) rather than the scenario's uniform
// layers - e.g. the Hub release uses the bundled-Keycloak identity
// layer and deploys keycloak/postgresql/elasticsearch, while orchestration
// releases use the external-Keycloak layer and deploy no companions of their
// own. For each release, an Entry is synthesized carrying that
// release's layer selection + its own namespace, and
// BuildEntryFlags — the SAME flags/companion-chart/ingress-hostname/
// vault-mapping assembly a normal matrix entry gets — builds its
// RuntimeFlags, so nothing is hand-rolled or silently omitted.
//
// Every release's namespace and Keycloak realm are computed UP FRONT (via
// deploy.GenerateTopologyContexts — pure, no cluster access) since all
// namespaces are known before any release is deployed. The resulting
// cross-ref env (HUB_NAMESPACE, KEYCLOAK_REALM, EXTERNAL_ELASTICSEARCH_*) is
// injected into EVERY release's ExtraEnv, including the Hub release
// itself — its inherited placeholders (it has none today, but future layers
// might) resolve harmlessly since the values are self-consistent.
//
// Deploy order is Hub-first: its Keycloak/Identity/shared-storage
// companions must be reachable before orchestration releases render (their
// external-Keycloak layer points straight at the Hub namespace) or
// start.
func runTopologyEntry(ctx context.Context, entry Entry, opts RunOptions) error {
	run, err := newTopologyRun(entry, opts)
	if err != nil {
		return err
	}
	levels, err := topologyDeployLevels(entry.Topology.Releases)
	if err != nil {
		return fmt.Errorf("topology entry %s/%s: %w", entry.Version, entry.Scenario, err)
	}
	defer run.cleanup()
	if err := run.prepareReleases(ctx, levels); err != nil {
		return err
	}
	if err := run.validateRenderedContracts(ctx); err != nil {
		return err
	}

	if err := updateTopologyChartDependencies(ctx, run.prepared); err != nil {
		return fmt.Errorf("topology entry %s/%s: %w", entry.Version, entry.Scenario, err)
	}
	if err := deployTopologyLevels(ctx, entry.Scenario, run.preparedLevels); err != nil {
		return err
	}

	if err := runTopologyPostDeployHook(ctx, entry, opts, run.prepared); err != nil {
		return err
	}

	return run.runTopologyE2ELegs(ctx)
}

func validateTopologyEntry(entry Entry, opts RunOptions) error {
	if entry.Auth != "keycloak" || entry.Flow != "install" {
		return fmt.Errorf("topology entry %s/%s: multi-namespace topology currently supports only auth=keycloak and flow=install (got auth=%q, flow=%q)", entry.Version, entry.Scenario, entry.Auth, entry.Flow)
	}

	if opts.ChartRef != "" {
		return fmt.Errorf("topology entry %s/%s: --chart-ref is not supported on the multi-namespace topology path yet (applyChartRefOverride runs only in executeEntry); tracked in #6656", entry.Version, entry.Scenario)
	}
	if opts.Cleanup {
		return fmt.Errorf("topology entry %s/%s: --cleanup is not supported on the multi-namespace topology path yet (no per-release namespace teardown); tracked in #6656", entry.Version, entry.Scenario)
	}
	return nil
}

func newTopologyRun(entry Entry, opts RunOptions) (*topologyRun, error) {
	platform := entry.Platform
	if platform == "" {
		platform = opts.Platform
	}
	if platform == "" {
		platform = "gke"
	}

	if err := validateTopologyEntry(entry, opts); err != nil {
		return nil, err
	}

	releases := make([]deploy.TopologyRelease, 0, len(entry.Topology.Releases))
	for _, r := range entry.Topology.Releases {
		releases = append(releases, deploy.TopologyRelease{
			Role:            r.Role,
			NamespaceSuffix: r.NamespaceSuffix,
			DependsOn:       r.DependsOn,
		})
	}

	// The "as if single-namespace" base namespace this entry would otherwise
	// get — every release's namespace is "<this>-<namespace-suffix>".
	baseOpts := opts
	baseOpts.Platform = platform
	baseNamespace := resolveNamespace(baseOpts, entry)

	contexts, err := deploy.GenerateTopologyContexts(entry.Scenario, releases, &config.RuntimeFlags{
		Deployment: config.DeploymentFlags{Namespace: baseNamespace},
	})
	if err != nil {
		return nil, fmt.Errorf("topology entry %s/%s: %w", entry.Version, entry.Scenario, err)
	}

	run := &topologyRun{entry: entry, opts: opts, platform: platform, baseNamespace: baseNamespace, contexts: contexts}
	if err := run.buildCrossRefEnv(); err != nil {
		return nil, err
	}
	return run, nil
}

func (t *topologyRun) prepareReleases(ctx context.Context, levels [][]int) error {
	levelOf := make([]int, len(t.entry.Topology.Releases))
	order := make([]int, 0, len(t.entry.Topology.Releases))
	for level, indices := range levels {
		for _, i := range indices {
			levelOf[i] = level
			order = append(order, i)
		}
	}

	t.prepared = make([]preparedTopologyRelease, 0, len(order))
	t.preparedLevels = make([][]preparedTopologyRelease, len(levels))
	var releaseErrs []error
	for _, i := range order {
		release, err := t.prepareRelease(ctx, i)
		if err != nil {
			releaseErrs = append(releaseErrs, err)
			continue
		}
		t.prepared = append(t.prepared, release)
		t.preparedLevels[levelOf[i]] = append(t.preparedLevels[levelOf[i]], release)
	}
	return errors.Join(releaseErrs...)
}

func (t *topologyRun) prepareRelease(ctx context.Context, i int) (preparedTopologyRelease, error) {
	entry, rel, releaseCtx := t.entry, t.entry.Topology.Releases[i], t.contexts[i]
	release, releaseEntry, err := t.releaseFlags(rel, releaseCtx.Namespace)
	if err != nil {
		return release, fmt.Errorf("topology release %s/%s (namespace-suffix %q): build flags: %w", entry.Scenario, rel.Role, rel.NamespaceSuffix, err)
	}

	removeManifest, err := applyTopologyCredentialsManifest(release.flags, t.opts.RepoRoot, entry.Topology.CredentialsManifest, t.baseNamespace)
	if err != nil {
		return release, fmt.Errorf("topology release %s/%s (namespace-suffix %q): %w", entry.Scenario, rel.Role, rel.NamespaceSuffix, err)
	}
	t.removers = append(t.removers, removeManifest)
	if err := registerDeclarativePostInfraHook(release.flags, releaseEntry.PostInfra, t.opts.RepoRoot, releaseEntry.Version, releaseEntry.Scenario); err != nil {
		return release, fmt.Errorf("topology release %s/%s (namespace-suffix %q): register post-infra hook: %w", entry.Scenario, rel.Role, rel.NamespaceSuffix, err)
	}
	if err := registerDeclarativePostDeployHook(release.flags, releaseEntry.PostDeploy, t.opts.RepoRoot, releaseEntry.Version, releaseEntry.Scenario); err != nil {
		return release, fmt.Errorf("topology release %s/%s (namespace-suffix %q): register post-deploy hook: %w", entry.Scenario, rel.Role, rel.NamespaceSuffix, err)
	}
	releaseCtx.IngressHost = release.flags.ResolveIngressHostname()
	prepared, err := prepareScenarioFn(ctx, releaseCtx, release.flags)
	if err != nil {
		return release, fmt.Errorf("topology release %s/%s (namespace-suffix %q) prepare failed: %w", entry.Scenario, rel.Role, rel.NamespaceSuffix, err)
	}
	release.prepared = prepared
	return release, nil
}

func (t *topologyRun) validateRenderedContracts(ctx context.Context) error {
	entry := t.entry
	for _, chartPath := range topologyChartPaths(t.prepared) {
		if err := helm.EnsureDependencies(ctx, chartPath); err != nil {
			return fmt.Errorf("topology entry %s/%s: %w", entry.Version, entry.Scenario, err)
		}
	}

	rendered := make([]RenderedTopologyRelease, 0, len(t.prepared))
	for _, release := range t.prepared {
		manifest, err := deploy.RenderPreparedTopologyContract(ctx, release.prepared, release.flags)
		if err != nil {
			return fmt.Errorf("topology release %s/%s contract render failed: %w", entry.Scenario, release.release.Role, err)
		}
		var contract TopologyContract
		// Charts older than 8.10 publish no contract template, so a release pinned to one
		// renders nothing here. Record the zero contract: ValidateRendered reads these only
		// to check optimize releases, and reports a mismatch if one actually needed this data.
		if len(bytes.TrimSpace(manifest)) > 0 {
			var document struct {
				Data map[string]string `yaml:"data"`
			}
			if err := yaml.Unmarshal(manifest, &document); err != nil {
				return fmt.Errorf("topology release %s/%s contract manifest decode failed: %w", entry.Scenario, release.release.Role, err)
			}
			if err := json.Unmarshal([]byte(document.Data["contract.json"]), &contract); err != nil {
				return fmt.Errorf("topology release %s/%s contract decode failed: %w", entry.Scenario, release.release.Role, err)
			}
		}
		rendered = append(rendered, RenderedTopologyRelease{Release: release.release, Contract: contract})
	}
	return entry.Topology.ValidateRendered(fmt.Sprintf("topology entry %s/%s", entry.Version, entry.Scenario), rendered)
}

func (t *topologyRun) cleanup() {
	for _, remove := range slices.Backward(t.removers) {
		remove()
	}
	for _, release := range t.prepared {
		release.prepared.Cleanup()
	}
}

func updateTopologyChartDependencies(ctx context.Context, releases []preparedTopologyRelease) error {
	updated := make(map[string]bool, len(releases))
	for _, release := range releases {
		if release.flags == nil || release.flags.Chart.SkipDependencyUpdate || release.flags.Chart.Chart != "" || release.flags.Chart.ChartPath == "" {
			continue
		}
		chartPath := release.flags.Chart.ChartPath
		if !updated[chartPath] {
			if err := dependencyUpdateFn(ctx, chartPath); err != nil {
				return err
			}
			updated[chartPath] = true
		}
		release.flags.Chart.SkipDependencyUpdate = true
	}
	return nil
}

func deployTopologyLevels(ctx context.Context, scenario string, levels [][]preparedTopologyRelease) error {
	for _, level := range levels {
		levelErrs := make([]error, len(level))
		var wg sync.WaitGroup
		for i, release := range level {
			wg.Go(func() {
				deployErr := executePreparedFn(ctx, release.prepared, release.flags)

				status := "OK"
				if deployErr != nil {
					status = fmt.Sprintf("FAILED: %v", deployErr)
					levelErrs[i] = fmt.Errorf("topology release %s/%s (namespace-suffix %q) deploy failed: %w", scenario, release.release.Role, release.release.NamespaceSuffix, deployErr)
				}
				fmt.Fprintf(os.Stdout, "topology release %s/%s (namespace %s): %s\n", scenario, release.release.Role, release.namespace, status)
			})
		}
		wg.Wait()
		if err := errors.Join(levelErrs...); err != nil {
			return err
		}
	}
	return nil
}

// runTopologyPostDeployHook runs the scenario's post-deploy hook once, after every release in the
// topology has deployed. synthesizeReleaseEntry deliberately clears PostDeploy on each synthesized
// release, so without this the hook is registered against nothing and never runs: the lifecycle
// boundary of a topology hook is the whole topology, not any one release in it.
//
// It runs before, and independently of, the e2e legs. A topology that sets skip-e2e still has to
// prove its post-deploy assertions - for physicaltenants the hook is the only per-tenant Optimize
// coverage there is, because the browser suite cannot express it yet.
func runTopologyPostDeployHook(ctx context.Context, entry Entry, opts RunOptions, releases []preparedTopologyRelease) error {
	if entry.PostDeploy == nil {
		return nil
	}
	flags := topologyHookFlags(releases)
	if flags == nil {
		return fmt.Errorf("topology entry %s/%s: post-deploy hook declared but the topology prepared no release to run it against", entry.Version, entry.Scenario)
	}
	if err := RunDeclarativePostDeployHook(ctx, flags, entry.PostDeploy, opts.RepoRoot, entry.Version, entry.Scenario); err != nil {
		return fmt.Errorf("topology entry %s/%s: post-deploy hook: %w", entry.Version, entry.Scenario, err)
	}
	return nil
}

// topologyHookFlags picks the release a topology-level hook runs against. Hook scripts read
// TEST_NAMESPACE as the namespace to inspect workloads in, and they inspect the orchestration
// release (Zeebe partitions, exporters), so the orchestration release's flags are the ones to use;
// its ExtraEnv already carries the whole cross-reference env, so the Hub and per-Optimize values
// the scripts also read resolve from it. With more than one orchestration release the first in
// deploy order wins, matching ORCH_NAMESPACE, which is only set for a single-orchestration topology.
func topologyHookFlags(releases []preparedTopologyRelease) *config.RuntimeFlags {
	for _, release := range releases {
		if release.release.Role == "orchestration" {
			return release.flags
		}
	}
	if len(releases) > 0 {
		return releases[0].flags
	}
	return nil
}

func topologyDeployLevels(releases []TopologyRelease) ([][]int, error) {
	deps, err := topologyReleaseDependencies(releases)
	if err != nil {
		return nil, err
	}
	deployed := make([]bool, len(releases))
	var levels [][]int
	for remaining := len(releases); remaining > 0; {
		var level []int
		for i := range releases {
			if deployed[i] {
				continue
			}
			ready := true
			for _, dep := range deps[i] {
				ready = ready && deployed[dep]
			}
			if ready {
				level = append(level, i)
			}
		}
		if len(level) == 0 {
			return nil, fmt.Errorf("topology depends-on graph has a cycle or unresolvable dependency")
		}
		for _, i := range level {
			deployed[i] = true
		}
		remaining -= len(level)
		levels = append(levels, level)
	}
	return levels, nil
}

func topologyReleaseDependencies(releases []TopologyRelease) ([][]int, error) {
	roleIdx := make(map[string]int, len(releases))
	suffixIdx := make(map[string]int, len(releases))
	for i, r := range releases {
		if _, seen := roleIdx[r.Role]; r.Role != "" && !seen {
			roleIdx[r.Role] = i
		}
		suffixIdx[r.NamespaceSuffix] = i
	}
	deps := make([][]int, len(releases))
	for i, r := range releases {
		if r.DependsOn != "" {
			dep, ok := roleIdx[r.DependsOn]
			if !ok {
				return nil, fmt.Errorf("release[%d] (role %q) depends-on %q which is not a declared role", i, r.Role, r.DependsOn)
			}
			deps[i] = append(deps[i], dep)
		}
		if r.Serves != "" {
			dep, ok := suffixIdx[r.Serves]
			if !ok {
				return nil, fmt.Errorf("release[%d] (role %q) serves %q which is not a declared namespace-suffix", i, r.Role, r.Serves)
			}
			deps[i] = append(deps[i], dep)
		}
	}
	return deps, nil
}
