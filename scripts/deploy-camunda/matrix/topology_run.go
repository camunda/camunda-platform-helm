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
	"maps"
	"os"
	"path/filepath"
	"scripts/camunda-core/pkg/helm"
	"scripts/deploy-camunda/config"
	"scripts/deploy-camunda/deploy"
	"slices"
	"strings"
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

// runTopologyEntry drives a topology-bearing matrix.Entry's multi-namespace
// fan-out: one matrix.Entry becomes N Helm releases (one per topology
// release) instead of matrix.Run's usual one-namespace-per-entry deploy.
//
// opts is the SAME fully-built matrix.RunOptions the normal (single-namespace)
// `matrix run` path constructs (see the matrix.Run(...) call below) — every
// per-release RunOptions is a copy of it with only Platform/NamespaceOverride
// overridden (see synthesizeReleaseOpts). Passing the whole struct through,
// rather than hand-picking a subset into a bespoke options type, is what
// prevents fields from silently dropping for topology deploys (this class of
// bug has already bitten IngressBaseDomains, HelmTimeout, and
// DeleteNamespaceFirst).
//
// Each release gets its OWN identity/persistence/features/dependencies layer
// selection (per matrix.TopologyRelease) rather than the scenario's uniform
// layers - e.g. the Hub release uses the bundled-Keycloak identity
// layer and deploys keycloak/postgresql/elasticsearch, while orchestration
// releases use the external-Keycloak layer and deploy no companions of their
// own. For each release, a matrix.Entry is synthesized carrying that
// release's layer selection + its own namespace, and
// matrix.BuildEntryFlags — the SAME flags/companion-chart/ingress-hostname/
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

// buildOrchestrationZeebeEnv derives ORCH_ZEEBE_GRPC/ORCH_ZEEBE_REST from the
// orchestration release's context: its Zeebe gateway Service is reachable
// cross-namespace at "<release>-zeebe-gateway.<namespace>.svc.cluster.local"
// (orchestration.serviceName in templates/orchestration/_helpers.tpl), on the
// gRPC (26500) and REST (8080) ports from
// orchestration.service.{grpcPort,httpPort}.
func buildOrchestrationZeebeEnv(orchestrationCtx *deploy.ScenarioContext) map[string]string {
	zeebeGatewayHost := fmt.Sprintf("%s-zeebe-gateway.%s.svc.cluster.local", orchestrationCtx.Release, orchestrationCtx.Namespace)
	return map[string]string{
		"ORCH_ZEEBE_GRPC": fmt.Sprintf("grpc://%s:26500", zeebeGatewayHost),
		"ORCH_ZEEBE_REST": fmt.Sprintf("http://%s:8080", zeebeGatewayHost),
	}
}

// applyTopologyCredentialsManifest points a topology release's deploy at the
// topology's credentials-manifest (repository-relative), rendered for the base
// namespace, so the deployer applies it instead of the CI-wide
// integration-test credentials. With no manifest declared the flags are left
// unchanged. The returned function removes the rendered file.
func applyTopologyCredentialsManifest(flags *config.RuntimeFlags, repoRoot, manifest, base string) (func(), error) {
	if manifest == "" {
		return func() {}, nil
	}
	path, remove, err := renderTopologyCredentialsManifest(filepath.Join(repoRoot, manifest), base)
	if err != nil {
		return func() {}, err
	}
	flags.Secrets.CredentialsManifest = path
	return remove, nil
}

// renderTopologyCredentialsManifest writes the topology's credentials-manifest,
// with the base namespace substituted, to a temporary file the deployer can
// apply, and returns a function that removes it.
func renderTopologyCredentialsManifest(path, base string) (string, func(), error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", func() {}, fmt.Errorf("read credentials manifest: %w", err)
	}
	rendered, err := RenderCredentialsManifest(content, base)
	if err != nil {
		return "", func() {}, err
	}
	f, err := os.CreateTemp("", "credentials-manifest-*.yaml")
	if err != nil {
		return "", func() {}, err
	}
	remove := func() { _ = os.Remove(f.Name()) }
	if _, err := f.Write(rendered); err != nil {
		f.Close()
		remove()
		return "", func() {}, err
	}
	if err := f.Close(); err != nil {
		remove()
		return "", func() {}, err
	}
	return f.Name(), remove, nil
}

// buildTopologyReleaseEnv layers a release's substitution namespace: shared
// cross-release variables first, then the release's own env, and finally the
// keys this driver derives from the topology declaration. The derived keys go
// last on purpose - they are the declaration's only copy, and a release env
// entry that shadowed one would deploy a context path or reader prefix that
// disagrees with optimize-context-path/serves and with the smoke matrix.
// matrix.Topology.Validate rejects such an entry outright; this ordering keeps
// the generated value authoritative even so.
func buildTopologyReleaseEnv(shared map[string]string, release TopologyRelease) map[string]string {
	env := make(map[string]string, len(shared)+len(release.Env)+4)
	maps.Copy(env, shared)
	if host := topologyReleaseServedHost(shared, release); host != "" {
		env["CAMUNDA_HOSTNAME"] = host
	}
	maps.Copy(env, release.Env)
	if release.Role == "orchestration" {
		token := TopologyEnvToken(release.NamespaceSuffix)
		env["ORCH_NAMESPACE"] = env[token+"_NAMESPACE"]
		env["ORCH_HOST"] = env[token+"_HOST"]
		env["ORCH_ZEEBE_GRPC"] = env[token+"_ZEEBE_GRPC"]
		env["ORCH_ZEEBE_REST"] = env[token+"_ZEEBE_REST"]
	}
	if release.Role == "optimize" {
		tenant := release.Tenant
		if tenant == "" {
			tenant = "default"
		}
		env["RELEASE_TENANT_ID"] = tenant
		// RELEASE_-prefixed: buildScenarioEnv seeds this namespace from the process
		// environment, where OPTIMIZE_CONTEXT_PATH is a name the Playwright suite reads
		// (pages/SM-8.10/NavigationPage.ts). Keep the two namespaces from sharing a key.
		env["RELEASE_OPTIMIZE_CONTEXT_PATH"] = release.OptimizeContextPath
		if release.Serves != "" {
			token := TopologyEnvToken(release.Serves)
			env["SERVED_NAMESPACE"] = env[token+"_NAMESPACE"]
			env["SERVED_HOST"] = env[token+"_HOST"]
			env["SERVED_ORCHESTRATION_INDEX_PREFIX"] = env[token+"_ORCHESTRATION_INDEX_PREFIX"]
		}
	}
	return env
}

// topologyReleaseServedHost returns the public host a release is served on:
// its own <TOKEN>_HOST for orchestration, HUB_HOST for hub and optimize.
func topologyReleaseServedHost(shared map[string]string, release TopologyRelease) string {
	if release.Role == "orchestration" {
		return shared[TopologyEnvToken(release.NamespaceSuffix)+"_HOST"]
	}
	return shared["HUB_HOST"]
}

// resolveSharedStorageServiceName resolves the Kubernetes Service name of the
// shared storage backend for a topology: it prefers the explicit
// topo.SharedStorageService, falling back to the ReleaseName of the resolved
// dependency matching topo.SharedStorage, or "" if neither is found.
func resolveSharedStorageServiceName(topo *Topology, resolvedDeps []ChartDependency) string {
	if topo.SharedStorageService != "" {
		return topo.SharedStorageService
	}
	for _, r := range resolvedDeps {
		if r.ReleaseName == topo.SharedStorage {
			return r.ReleaseName
		}
	}
	return ""
}

// globalHostFromHelmSets scans "key=value" helm-set pairs (the same shape
// matrix.RunOptions.ExtraHelmSets and parseHelmSetPairs use) for global.host and
// returns its value, or "" if it isn't present. The last matching entry
// wins, mirroring how repeated --extra-helm-set flags are applied.
func globalHostFromHelmSets(pairs []string) string {
	value := ""
	for _, p := range pairs {
		idx := strings.Index(p, "=")
		if idx <= 0 {
			continue
		}
		if p[:idx] == "global.host" {
			value = p[idx+1:]
		}
	}
	return value
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

func (t *topologyRun) buildCrossRefEnv() error {
	entry, contexts := t.entry, t.contexts
	hubIdx := -1
	var orchestrationIndices []int
	var optimizeIndices []int
	for i, r := range entry.Topology.Releases {
		if r.Role == "hub" {
			hubIdx = i
		}
		if r.Role == "orchestration" {
			orchestrationIndices = append(orchestrationIndices, i)
		}
		if r.Role == "optimize" {
			optimizeIndices = append(optimizeIndices, i)
		}
	}
	if hubIdx == -1 {
		return fmt.Errorf("topology entry %s/%s: topology has no \"hub\" release", entry.Version, entry.Scenario)
	}

	sharedStorageService := resolveSharedStorageServiceName(entry.Topology, entry.Topology.Releases[hubIdx].ResolvedDependencies)

	crossRefEnv := deploy.BuildTopologyCrossRefEnv(contexts[hubIdx], sharedStorageService, "9200", "http")
	t.crossRefEnv = crossRefEnv
	t.orchestrationIndices = orchestrationIndices

	// ORCH_ZEEBE_GRPC/ORCH_ZEEBE_REST let the Hub release (Web
	// Modeler) register the orchestration release's Zeebe gateway as a
	// cluster, reaching it cross-namespace by FQDN — the same
	// "<release>-zeebe-gateway.<namespace>.svc.cluster.local" Service the
	// orchestration release itself exposes (orchestration.serviceName in
	// templates/orchestration/_helpers.tpl), on the gRPC (26500) and REST
	// (8080) ports from orchestration.service.{grpcPort,httpPort}.
	if err := t.addTopologyIngressHosts(contexts[hubIdx]); err != nil {
		return fmt.Errorf("topology entry %s/%s: %w", entry.Version, entry.Scenario, err)
	}
	for _, i := range orchestrationIndices {
		token := TopologyEnvToken(entry.Topology.Releases[i].NamespaceSuffix)
		crossRefEnv[token+"_NAMESPACE"] = contexts[i].Namespace
		crossRefEnv[token+"_ORCHESTRATION_INDEX_PREFIX"] = contexts[i].OrchestrationIndexPrefix
		for key, value := range buildOrchestrationZeebeEnv(contexts[i]) {
			crossRefEnv[token+strings.TrimPrefix(key, "ORCH")] = value
		}
	}
	for _, i := range optimizeIndices {
		release := entry.Topology.Releases[i]
		token := TopologyEnvToken(release.NamespaceSuffix)
		crossRefEnv[token+"_NAMESPACE"] = contexts[i].Namespace
		crossRefEnv[token+"_OPTIMIZE_CONTEXT_PATH"] = release.OptimizeContextPath
	}
	if len(orchestrationIndices) == 1 {
		i := orchestrationIndices[0]
		crossRefEnv["ORCH_NAMESPACE"] = contexts[i].Namespace
		crossRefEnv["ORCH_ORCHESTRATION_INDEX_PREFIX"] = contexts[i].OrchestrationIndexPrefix
		maps.Copy(crossRefEnv, buildOrchestrationZeebeEnv(contexts[i]))
	}
	return nil
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

func (t *topologyRun) releaseFlags(rel TopologyRelease, namespace string) (preparedTopologyRelease, Entry, error) {
	releaseEntry := synthesizeReleaseEntry(t.opts.RepoRoot, t.entry, rel, t.platform)
	releaseOpts := synthesizeReleaseOpts(t.opts, t.platform, namespace)
	releaseHost := topologyReleaseHost(t.crossRefEnv, rel, len(t.orchestrationIndices))
	if releaseHost != "" {
		releaseOpts.ExtraHelmSets = append(releaseOpts.ExtraHelmSets, "global.host="+releaseHost)
	}

	flags, releaseNamespace, _, _, err := BuildEntryFlags(releaseEntry, releaseOpts)
	if err != nil {
		return preparedTopologyRelease{}, releaseEntry, err
	}
	applyTopologyReleaseOverrides(flags, buildTopologyReleaseEnv(t.crossRefEnv, rel))
	applyTopologyReleaseHostname(flags, releaseHost)
	return preparedTopologyRelease{release: rel, flags: flags, namespace: releaseNamespace}, releaseEntry, nil
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

// topologyReleaseHost returns the ingress host the topology assigns to a
// release, or "" when the release keeps BuildEntryFlags' default host.
func topologyReleaseHost(crossRefEnv map[string]string, release TopologyRelease, orchestrationCount int) string {
	key := topologyReleaseHostKey(release.Role, release.NamespaceSuffix, orchestrationCount)
	if key == "" {
		return ""
	}
	return crossRefEnv[key]
}

// applyTopologyReleaseHostname makes a topology-assigned host the release's
// CAMUNDA_HOSTNAME as well as its global.host. Scenario values files take the
// ingress host from $CAMUNDA_HOSTNAME, and charts before 8.10 read it from
// global.ingress.host, which the global.host override does not reach (8.9
// prefers global.ingress.host; 8.8 and 8.7 read nothing else). Without this,
// every pre-8.10 orchestration release in a multi-orchestration topology kept
// the one CI-wide hostname, so they all published the same external-dns name
// instead of the per-release host the Hub inventory and e2e legs use.
// flags.ExtraEnv outranks the scenario-derived CAMUNDA_HOSTNAME when values
// files are rendered, so setting it here is enough.
func applyTopologyReleaseHostname(flags *config.RuntimeFlags, host string) {
	if host == "" {
		return
	}
	if flags.ExtraEnv == nil {
		flags.ExtraEnv = map[string]string{}
	}
	flags.ExtraEnv["CAMUNDA_HOSTNAME"] = host
}

// topologyReleaseHostKey names the crossRefEnv key whose value must be pushed
// into a release's global.host, or "" to leave global.host to BuildEntryFlags'
// namespace-derived default.
//
// An "optimize" release is always pinned to the Hub host: its values layer
// registers an OIDC redirect URL under HUB_HOST, so a namespace-derived host
// would break the callback. Other roles keep the pre-existing behavior of
// overriding only when several orchestration releases must be split across
// distinct hosts.
func topologyReleaseHostKey(role, namespaceSuffix string, orchestrationCount int) string {
	if role == "optimize" {
		return "HUB_HOST"
	}
	if orchestrationCount <= 1 {
		return ""
	}
	if role == "orchestration" {
		return TopologyEnvToken(namespaceSuffix) + "_HOST"
	}
	return "HUB_HOST"
}

func (t *topologyRun) addTopologyIngressHosts(hubCtx *deploy.ScenarioContext) error {
	crossRefEnv, opts, releases, contexts := t.crossRefEnv, t.opts, t.entry.Topology.Releases, t.contexts
	orchestrationCount := 0
	for _, release := range releases {
		if release.Role == "orchestration" {
			orchestrationCount++
		}
	}
	sharedHost := globalHostFromHelmSets(opts.ExtraHelmSets)
	if sharedHost != "" && orchestrationCount <= 1 {
		crossRefEnv["HUB_HOST"] = sharedHost
		for _, release := range releases {
			if release.Role == "orchestration" {
				crossRefEnv[TopologyEnvToken(release.NamespaceSuffix)+"_HOST"] = sharedHost
				crossRefEnv["ORCH_HOST"] = sharedHost
			}
		}
		return nil
	}

	baseDomain := EntryInfra(opts, t.platform, "").IngressBaseDomain
	if baseDomain == "" {
		if sharedHost != "" {
			return fmt.Errorf("global.host=%s is shared, but this topology has %d orchestration releases that each need their own host; set an ingress base domain instead", sharedHost, orchestrationCount)
		}
		return nil
	}
	crossRefEnv["HUB_HOST"] = (&config.IngressFlags{
		IngressSubdomain:  hubCtx.Namespace,
		IngressBaseDomain: baseDomain,
	}).ResolveIngressHostname()
	for i, release := range releases {
		if release.Role != "orchestration" {
			continue
		}
		host := (&config.IngressFlags{
			IngressSubdomain:  contexts[i].Namespace,
			IngressBaseDomain: baseDomain,
		}).ResolveIngressHostname()
		crossRefEnv[TopologyEnvToken(release.NamespaceSuffix)+"_HOST"] = host
		if orchestrationCount == 1 {
			crossRefEnv["ORCH_HOST"] = host
		}
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

// synthesizeReleaseEntry builds the matrix.Entry for one topology release,
// carrying THAT release's own identity/persistence/features/dependencies
// layer selection instead of the scenario-level (uniform) ones — the core of
// the per-release layer fix. Extracted as a pure function for testability.
func synthesizeReleaseEntry(repoRoot string, entry Entry, rel TopologyRelease, platform string) Entry {
	// Feature layers go through the same env-var substitution pipeline as the
	// identity and persistence layers (scenarios.BuildDeploymentConfig →
	// values.Process), so a release's ${...} placeholders resolve before Helm
	// sees them.
	features := append([]string(nil), rel.Features...)

	// A release may pin its own chart-version, so a topology can mix chart
	// versions (e.g. an 8.10 Hub serving an 8.9 orchestration release). Empty
	// inherits the parent matrix entry's version, which is the uniform case.
	version := rel.ChartVersion
	if version == "" {
		version = entry.Version
	}

	releaseEntry := Entry{
		Version:      version,
		ChartPath:    filepath.Join(repoRoot, "charts", "camunda-platform-"+version),
		Scenario:     entry.Scenario,
		Shortname:    entry.Shortname,
		Auth:         entry.Auth,
		Flow:         "install",
		Platform:     platform,
		InfraType:    entry.InfraType,
		Tier:         entry.Tier,
		Identity:     rel.Identity,
		Persistence:  rel.Persistence,
		Features:     features,
		Dependencies: rel.ResolvedDependencies,
	}
	// The scenario's post-infra hook provisions the shared infrastructure the whole
	// topology then deploys onto, so it belongs to the Hub release: that is the one
	// release every other one depends on, and running it per release would repeat
	// the provisioning once per namespace. PostDeploy is deliberately NOT carried
	// here - runTopologyPostDeployHook runs it once after the whole topology is up.
	if rel.Role == "hub" {
		releaseEntry.PostInfra = entry.PostInfra
	}
	// e2e is a topology-level concern, not a per-release one: a release's deploy returns while later
	// releases are still undeployed, so testing here would test a partial topology (and would repeat
	// for every orchestration release). runTopologyEntry runs the legs once the whole topology is up.
	releaseEntry.SkipE2E = true
	return releaseEntry
}

// synthesizeReleaseOpts builds the matrix.RunOptions for one topology
// release: identical to the base topology entry's options, except
// NamespaceOverride pins this release to its own precomputed namespace.
// Extracted as a pure function for testability.
// synthesizeReleaseOpts returns a copy of base (the same fully-built
// matrix.RunOptions passed into runTopologyEntry) with ONLY Platform and
// NamespaceOverride overridden for one topology release. Every other field
// — DeleteNamespaceFirst, EnvFile/EnvFiles, VaultBackedSecrets/
// UseVaultBackedSecrets, ExtraValues, ExtraHelmArgs/ExtraHelmSets,
// SkipDependencyUpdate, KeycloakHost/KeycloakProtocol, UseLatest, UseQA,
// LogDir, HelmTimeout, IngressBaseDomains/IngressBaseDomain, etc. — passes
// through unchanged. RunOptions is passed and returned by value; its map
// fields (KubeContexts, IngressBaseDomains, EnvFiles, VaultBackedSecrets)
// are shared with base, which is safe because callers only ever READ them
// (via BuildEntryFlags) — never mutate a map on the returned copy.
func synthesizeReleaseOpts(base RunOptions, platform string, namespaceOverride string) RunOptions {
	cp := base
	cp.Platform = platform
	cp.NamespaceOverride = namespaceOverride
	return cp
}

// applyTopologyReleaseOverrides applies the topology-specific adjustments
// every release's flags need on top of matrix.BuildEntryFlags' normal
// assembly, mutating flags in place:
//
//   - ExtraEnv: the cross-ref env (HUB_NAMESPACE, KEYCLOAK_REALM,
//     EXTERNAL_ELASTICSEARCH_*, HUB_HOST, ORCH_HOST, ORCH_ZEEBE_GRPC,
//     ORCH_ZEEBE_REST) computed up front, MERGED into EVERY
//     release (Hub included) before render/preflight, on top of
//     any pre-existing ExtraEnv (cross-ref wins on key conflict). The
//     topology path currently bypasses executeEntry (where per-entry
//     client-IDs like VENOM_CLIENT_ID/CONNECTORS_CLIENT_ID and
//     Entra/Auth0 vars are injected) and is guarded to auth=keycloak/
//     flow=install; replacing the map wholesale would silently drop any
//     such per-entry ExtraEnv if that guard is ever relaxed or if
//     BuildEntryFlags starts populating ExtraEnv. Merging is
//     future-proof and functionally identical today.
//
//   - Secrets.ExternalSecrets: forced to true for every release.
//     BuildEntryFlags sets ExternalSecrets = (NamespaceOverride == ""),
//     which is a normal-path optimization: per-scenario CI workflows that
//     pre-create the namespace via --namespace-override already ran
//     .github/actions/cluster-setup-secrets, so re-applying external
//     secrets would be redundant (and can fail on EKS RBAC, see
//     BuildEntryFlags' comment). The topology path ALWAYS sets
//     NamespaceOverride (one per release namespace) but has NO equivalent
//     cluster-setup-secrets action run for it — so the topology driver
//     itself must own provisioning integration-test-credentials (and other
//     external secrets) into every release namespace from the same Vault
//     path, or dependent companions like the Hub release's bundled
//     Keycloak CreateContainerConfigError on the missing secret.
//     NOTE for Stage 3 (CI wiring): when a topology scenario is wired into
//     CI, the workflow must NOT also invoke cluster-setup-secrets for these
//     namespaces — this driver already provisions them, and doing both
//     would double-provision (and reintroduces the EKS RBAC risk
//     BuildEntryFlags' comment warns about).
func applyTopologyReleaseOverrides(flags *config.RuntimeFlags, crossRefEnv map[string]string) {
	if flags.ExtraEnv == nil {
		flags.ExtraEnv = make(map[string]string, len(crossRefEnv))
	}
	maps.Copy(flags.ExtraEnv, crossRefEnv)

	flags.Secrets.ExternalSecrets = true

	// The topology driver provisions the registry-camunda-cloud pull secret into each
	// release namespace (creds resolve from HARBOR_USERNAME/TEST_DOCKER_USERNAME_CAMUNDA_CLOUD
	// env via the deployer fallback), so the deploy is self-sufficient locally and in CI
	// without relying on node image cache.
	flags.Docker.EnsureDockerRegistry = true
}
