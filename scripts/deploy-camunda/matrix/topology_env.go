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
	"fmt"
	"maps"
	"scripts/deploy-camunda/config"
	"scripts/deploy-camunda/deploy"
	"strings"
)

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

// buildTopologyReleaseEnv layers a release's substitution namespace: shared
// cross-release variables first, then the release's own env, and finally the
// keys this driver derives from the topology declaration. The derived keys go
// last on purpose - they are the declaration's only copy, and a release env
// entry that shadowed one would deploy a context path or reader prefix that
// disagrees with optimize-context-path/serves and with the smoke matrix.
// Topology.Validate rejects such an entry outright; this ordering keeps
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
// RunOptions.ExtraHelmSets and parseHelmSetPairs use) for global.host and
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
