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
	"strings"
	"testing"

	"scripts/deploy-camunda/config"
	"scripts/deploy-camunda/deploy"
)

// TestGlobalHostFromHelmSets_FindsKey pins down that globalHostFromHelmSets reads
// the "global.host" value out of the "key=value" ExtraHelmSets pairs CI
// passes via --extra-helm-set global.host=<host> — the value runTopologyEntry
// now uses for HUB_HOST/ORCH_HOST instead of fabricating
// "<namespace>.<base-domain>" (see #6651).
func TestGlobalHostFromHelmSets_FindsKey(t *testing.T) {
	pairs := []string{
		"orchestration.upgrade.allowPreReleaseImages=true",
		"global.host=abc123-mns.ci.distro.ultrawombat.com",
	}
	if got := globalHostFromHelmSets(pairs); got != "abc123-mns.ci.distro.ultrawombat.com" {
		t.Fatalf("globalHostFromHelmSets() = %q, want %q", got, "abc123-mns.ci.distro.ultrawombat.com")
	}
}

func TestGlobalHostFromHelmSets_MissingKeyReturnsEmpty(t *testing.T) {
	pairs := []string{"orchestration.upgrade.allowPreReleaseImages=true"}
	if got := globalHostFromHelmSets(pairs); got != "" {
		t.Fatalf("globalHostFromHelmSets() = %q, want empty string", got)
	}
}

func TestGlobalHostFromHelmSets_EmptyInputReturnsEmpty(t *testing.T) {
	if got := globalHostFromHelmSets(nil); got != "" {
		t.Fatalf("globalHostFromHelmSets() = %q, want empty string", got)
	}
}

// TestGlobalHostFromHelmSets_LastDuplicateWins mirrors how repeated
// --extra-helm-set flags for the same key are applied by Helm (last wins).
func TestGlobalHostFromHelmSets_LastDuplicateWins(t *testing.T) {
	pairs := []string{"global.host=first.example.com", "global.host=second.example.com"}
	if got := globalHostFromHelmSets(pairs); got != "second.example.com" {
		t.Fatalf("globalHostFromHelmSets() = %q, want %q", got, "second.example.com")
	}
}

// TestGlobalHostFromHelmSets_SkipsMalformedEntries pins down that entries
// without "=" (or with "=" as the first character) are ignored rather than
// panicking or matching spuriously, mirroring parseHelmSetPairs' handling.
func TestGlobalHostFromHelmSets_SkipsMalformedEntries(t *testing.T) {
	pairs := []string{"malformed-entry-no-equals", "=leading-equals-is-skipped", "global.host=host.example.com"}
	if got := globalHostFromHelmSets(pairs); got != "host.example.com" {
		t.Fatalf("globalHostFromHelmSets() = %q, want %q", got, "host.example.com")
	}
}

func TestAddTopologyIngressHosts_DerivesPerReleaseHosts(t *testing.T) {
	env := deploy.BuildTopologyCrossRefEnv(
		&deploy.ScenarioContext{Namespace: "matrix-810-mns-hub"},
		"elasticsearch",
		"9200",
		"http",
	)
	opts := RunOptions{Infra: config.InfraOverride{
		InfraConfig: config.InfraConfig{IngressBaseDomain: "fallback.example.com"},
		InfraMaps:   config.InfraMaps{IngressBaseDomains: map[string]string{"gke": "ci.distro.ultrawombat.com"}},
	}}

	if err := testTopologyRun(
		env,
		opts,
		[]TopologyRelease{{Role: "hub", NamespaceSuffix: "hub"}, {Role: "orchestration", NamespaceSuffix: "orcha"}},
		[]*deploy.ScenarioContext{{Namespace: "matrix-810-mns-hub"}, {Namespace: "matrix-810-mns-orcha"}},
	).addTopologyIngressHosts(&deploy.ScenarioContext{Namespace: "matrix-810-mns-hub"}); err != nil {
		t.Fatal(err)
	}

	if got, want := env["HUB_HOST"], "matrix-810-mns-hub.ci.distro.ultrawombat.com"; got != want {
		t.Errorf("HUB_HOST = %q, want %q", got, want)
	}
	if got, want := env["ORCHA_HOST"], "matrix-810-mns-orcha.ci.distro.ultrawombat.com"; got != want {
		t.Errorf("ORCHA_HOST = %q, want %q", got, want)
	}
	if got, want := env["ORCH_HOST"], "matrix-810-mns-orcha.ci.distro.ultrawombat.com"; got != want {
		t.Errorf("ORCH_HOST = %q, want %q", got, want)
	}
}

func TestAddTopologyIngressHosts_DerivesEveryOrchestrationHost(t *testing.T) {
	env := map[string]string{}
	releases := testTopologyReleases()
	contexts := []*deploy.ScenarioContext{
		{Namespace: "matrix-810-mns-hub"},
		{Namespace: "matrix-810-mns-orcha"},
		{Namespace: "matrix-810-mns-orchb"},
	}

	if err := testTopologyRun(env, RunOptions{Infra: config.InfraOverride{InfraConfig: config.InfraConfig{IngressBaseDomain: "ci.example.com"}}}, releases, contexts).addTopologyIngressHosts(contexts[0]); err != nil {
		t.Fatal(err)
	}

	if got := env["ORCHA_HOST"]; got != "matrix-810-mns-orcha.ci.example.com" {
		t.Errorf("ORCHA_HOST = %q", got)
	}
	if got := env["ORCHB_HOST"]; got != "matrix-810-mns-orchb.ci.example.com" {
		t.Errorf("ORCHB_HOST = %q", got)
	}
	if _, exists := env["ORCH_HOST"]; exists {
		t.Error("ORCH_HOST must not alias one release in a multi-orchestration topology")
	}
}

func TestAddTopologyIngressHosts_RejectsSharedHostForMultipleOrchestrations(t *testing.T) {
	releases := testTopologyReleases()
	contexts := []*deploy.ScenarioContext{
		{Namespace: "matrix-810-mns-hub"},
		{Namespace: "matrix-810-mns-orcha"},
		{Namespace: "matrix-810-mns-orchb"},
	}
	opts := RunOptions{ExtraHelmSets: []string{"global.host=abc123-mns.ci.example.com"}}

	err := testTopologyRun(map[string]string{}, opts, releases, contexts).addTopologyIngressHosts(contexts[0])
	if err == nil || !strings.Contains(err.Error(), "global.host") {
		t.Fatalf("expected a global.host error for a multi-orchestration topology, got %v", err)
	}

	opts.Infra.IngressBaseDomain = "ci.example.com"
	env := map[string]string{}
	if err := testTopologyRun(env, opts, releases, contexts).addTopologyIngressHosts(contexts[0]); err != nil {
		t.Fatalf("a base domain must derive per-release hosts, got %v", err)
	}
	if env["HUB_HOST"] == "" || env["ORCHA_HOST"] == "" || env["ORCHB_HOST"] == "" {
		t.Errorf("hosts not derived: %v", env)
	}
}

func TestBuildTopologyReleaseEnv_SelectsLocalOrchestrationReferences(t *testing.T) {
	shared := map[string]string{
		"ORCHA_NAMESPACE":  "ns-orcha",
		"ORCHA_HOST":       "orcha.example.com",
		"ORCHA_ZEEBE_GRPC": "grpc://orcha:26500",
		"ORCHA_ZEEBE_REST": "http://orcha:8080",
		"ORCHB_NAMESPACE":  "ns-orchb",
		"ORCHB_HOST":       "orchb.example.com",
		"ORCHB_ZEEBE_GRPC": "grpc://orchb:26500",
		"ORCHB_ZEEBE_REST": "http://orchb:8080",
	}
	release := TopologyRelease{
		Role:            "orchestration",
		NamespaceSuffix: "orchb",
		Env:             map[string]string{"ORCH_ORCHESTRATION_CLIENT_ID": "orchestration-orchb"},
	}

	env := buildTopologyReleaseEnv(shared, release)
	if got := env["ORCH_NAMESPACE"]; got != "ns-orchb" {
		t.Errorf("ORCH_NAMESPACE = %q", got)
	}
	if got := env["ORCH_HOST"]; got != "orchb.example.com" {
		t.Errorf("ORCH_HOST = %q", got)
	}
	if got := env["ORCH_ZEEBE_GRPC"]; got != "grpc://orchb:26500" {
		t.Errorf("ORCH_ZEEBE_GRPC = %q", got)
	}
	if got := env["ORCH_ORCHESTRATION_CLIENT_ID"]; got != "orchestration-orchb" {
		t.Errorf("ORCH_ORCHESTRATION_CLIENT_ID = %q", got)
	}
}

func TestBuildTopologyReleaseEnv_SetsCamundaHostnameToServedHost(t *testing.T) {
	shared := map[string]string{
		"HUB_HOST":   "ns-hub.example.com",
		"ORCHA_HOST": "ns-orcha.example.com",
	}
	cases := []struct {
		name    string
		release TopologyRelease
		want    string
	}{
		{"hub", TopologyRelease{Role: "hub", NamespaceSuffix: "hub"}, "ns-hub.example.com"},
		{"orchestration", TopologyRelease{Role: "orchestration", NamespaceSuffix: "orcha"}, "ns-orcha.example.com"},
		{"optimize", TopologyRelease{Role: "optimize", NamespaceSuffix: "opta", Serves: "orcha"}, "ns-hub.example.com"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := buildTopologyReleaseEnv(shared, tc.release)["CAMUNDA_HOSTNAME"]; got != tc.want {
				t.Errorf("CAMUNDA_HOSTNAME = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestBuildTopologyReleaseEnv_OmitsCamundaHostnameWithoutHost(t *testing.T) {
	env := buildTopologyReleaseEnv(map[string]string{}, TopologyRelease{Role: "hub", NamespaceSuffix: "hub"})
	if _, ok := env["CAMUNDA_HOSTNAME"]; ok {
		t.Errorf("CAMUNDA_HOSTNAME set to %q with no derived host; want it left to the process env", env["CAMUNDA_HOSTNAME"])
	}
}

func TestBuildTopologyReleaseEnv_PublishesServedReferencesForOptimize(t *testing.T) {
	shared := map[string]string{
		"ORCHA_NAMESPACE":                  "ns-orcha",
		"ORCHA_HOST":                       "orcha.example.com",
		"ORCHA_ORCHESTRATION_INDEX_PREFIX": "job-orcha",
		"ORCHB_NAMESPACE":                  "ns-orchb",
		"ORCHB_HOST":                       "orchb.example.com",
		"ORCHB_ORCHESTRATION_INDEX_PREFIX": "job-orchb",
	}
	release := TopologyRelease{
		Role:                "optimize",
		NamespaceSuffix:     "optb",
		Serves:              "orchb",
		Tenant:              "tenantb",
		OptimizeContextPath: "/optimize-orchb",
	}

	env := buildTopologyReleaseEnv(shared, release)
	if got := env["SERVED_ORCHESTRATION_INDEX_PREFIX"]; got != "job-orchb" {
		t.Errorf("SERVED_ORCHESTRATION_INDEX_PREFIX = %q, want the prefix of the release named by serves", got)
	}
	if got := env["SERVED_NAMESPACE"]; got != "ns-orchb" {
		t.Errorf("SERVED_NAMESPACE = %q", got)
	}
	if got := env["RELEASE_OPTIMIZE_CONTEXT_PATH"]; got != "/optimize-orchb" {
		t.Errorf("RELEASE_OPTIMIZE_CONTEXT_PATH = %q", got)
	}
	if got := env["RELEASE_TENANT_ID"]; got != "tenantb" {
		t.Errorf("RELEASE_TENANT_ID = %q", got)
	}
}

// Repointing serves must repoint the records this Optimize reads. The values
// layer names SERVED_ORCHESTRATION_INDEX_PREFIX rather than a per-release token,
// so the declaration is the only place the mapping is written.
func TestBuildTopologyReleaseEnv_ServedPrefixFollowsServes(t *testing.T) {
	shared := map[string]string{
		"ORCHA_ORCHESTRATION_INDEX_PREFIX": "job-orcha",
		"ORCHB_ORCHESTRATION_INDEX_PREFIX": "job-orchb",
	}
	base := TopologyRelease{Role: "optimize", NamespaceSuffix: "opta", Tenant: "default", OptimizeContextPath: "/optimize-a"}

	servesA := base
	servesA.Serves = "orcha"
	servesB := base
	servesB.Serves = "orchb"

	if got := buildTopologyReleaseEnv(shared, servesA)["SERVED_ORCHESTRATION_INDEX_PREFIX"]; got != "job-orcha" {
		t.Errorf("serves=orcha gave prefix %q", got)
	}
	if got := buildTopologyReleaseEnv(shared, servesB)["SERVED_ORCHESTRATION_INDEX_PREFIX"]; got != "job-orchb" {
		t.Errorf("serves=orchb gave prefix %q", got)
	}
}

func TestBuildTopologyReleaseEnv_OmitsOptimizeKeysForOtherRoles(t *testing.T) {
	env := buildTopologyReleaseEnv(map[string]string{}, TopologyRelease{
		Role:            "orchestration",
		NamespaceSuffix: "orcha",
	})
	for _, key := range []string{"RELEASE_TENANT_ID", "RELEASE_OPTIMIZE_CONTEXT_PATH", "SERVED_ORCHESTRATION_INDEX_PREFIX", "SERVED_NAMESPACE"} {
		if _, exists := env[key]; exists {
			t.Errorf("%s must not be published for a non-optimize release", key)
		}
	}
}

// The keys derived from the declaration are authoritative: a release env entry
// naming one is applied first and overwritten, so a topology author cannot
// deploy a context path or reader prefix that disagrees with
// optimize-context-path/serves and with the smoke matrix.
// Topology.Validate rejects the entry outright; this covers the ordering
// that makes the rejection safe to rely on.
func TestBuildTopologyReleaseEnv_DerivedKeysOutrankReleaseEnv(t *testing.T) {
	shared := map[string]string{
		"ORCHA_NAMESPACE":                  "ns-orcha",
		"ORCHA_HOST":                       "orcha.example.com",
		"ORCHA_ORCHESTRATION_INDEX_PREFIX": "job-orcha",
	}
	release := TopologyRelease{
		Role:                "optimize",
		NamespaceSuffix:     "opta",
		Serves:              "orcha",
		Tenant:              "default",
		OptimizeContextPath: "/optimize-orcha",
		Env: map[string]string{
			"RELEASE_TENANT_ID":                 "stale-tenant",
			"RELEASE_OPTIMIZE_CONTEXT_PATH":     "/optimize-stale",
			"SERVED_ORCHESTRATION_INDEX_PREFIX": "job-somewhere-else",
			"SERVED_NAMESPACE":                  "ns-somewhere-else",
			"SERVED_HOST":                       "somewhere.example.com",
		},
	}

	env := buildTopologyReleaseEnv(shared, release)
	for key, want := range map[string]string{
		"RELEASE_TENANT_ID":                 "default",
		"RELEASE_OPTIMIZE_CONTEXT_PATH":     "/optimize-orcha",
		"SERVED_ORCHESTRATION_INDEX_PREFIX": "job-orcha",
		"SERVED_NAMESPACE":                  "ns-orcha",
		"SERVED_HOST":                       "orcha.example.com",
	} {
		if got := env[key]; got != want {
			t.Errorf("%s = %q, want the derived %q", key, got, want)
		}
	}
}

func TestBuildTopologyReleaseEnv_DerivedOrchestrationKeysOutrankReleaseEnv(t *testing.T) {
	shared := map[string]string{
		"ORCHA_NAMESPACE":  "ns-orcha",
		"ORCHA_HOST":       "orcha.example.com",
		"ORCHA_ZEEBE_GRPC": "grpc://orcha:26500",
		"ORCHA_ZEEBE_REST": "http://orcha:8080",
	}
	release := TopologyRelease{
		Role:            "orchestration",
		NamespaceSuffix: "orcha",
		Env: map[string]string{
			"ORCH_NAMESPACE":  "ns-stale",
			"ORCH_HOST":       "stale.example.com",
			"ORCH_ZEEBE_GRPC": "grpc://stale:26500",
			"ORCH_ZEEBE_REST": "http://stale:8080",
		},
	}

	env := buildTopologyReleaseEnv(shared, release)
	for key, want := range map[string]string{
		"ORCH_NAMESPACE":  "ns-orcha",
		"ORCH_HOST":       "orcha.example.com",
		"ORCH_ZEEBE_GRPC": "grpc://orcha:26500",
		"ORCH_ZEEBE_REST": "http://orcha:8080",
	} {
		if got := env[key]; got != want {
			t.Errorf("%s = %q, want the derived %q", key, got, want)
		}
	}
}

// In a multi-orchestration topology every orchestration release needs its own
// CAMUNDA_HOSTNAME, not only its own global.host. Pre-8.10 scenario values set
// global.ingress.host (and the external-dns hostname) from $CAMUNDA_HOSTNAME, so
// without it the 8.9/8.8/8.7 legs of mns2 all shared the CI-wide host and the
// browser could not resolve their Operate URL.
func TestApplyTopologyReleaseHostname_PerOrchestrationRelease(t *testing.T) {
	releases := testTopologyReleases()
	contexts := []*deploy.ScenarioContext{
		{Namespace: "matrix-810-mns-hub"},
		{Namespace: "matrix-810-mns-orcha"},
		{Namespace: "matrix-810-mns-orchb"},
	}
	crossRefEnv := map[string]string{}
	if err := testTopologyRun(crossRefEnv, RunOptions{Infra: config.InfraOverride{InfraConfig: config.InfraConfig{IngressBaseDomain: "ci.example.com"}}}, releases[:3], contexts).addTopologyIngressHosts(contexts[0]); err != nil {
		t.Fatal(err)
	}

	for i, want := range []string{"matrix-810-mns-hub.ci.example.com", "matrix-810-mns-orcha.ci.example.com", "matrix-810-mns-orchb.ci.example.com"} {
		release := releases[i]
		flags := &config.RuntimeFlags{ExtraEnv: map[string]string{"CAMUNDA_HOSTNAME": "ci-wide.example.com"}}
		host := topologyReleaseHost(crossRefEnv, release, 2)
		applyTopologyReleaseOverrides(flags, buildTopologyReleaseEnv(crossRefEnv, release))
		applyTopologyReleaseHostname(flags, host)
		if host != want {
			t.Errorf("%s: topologyReleaseHost = %q, want %q", release.NamespaceSuffix, host, want)
		}
		if got := flags.ExtraEnv["CAMUNDA_HOSTNAME"]; got != want {
			t.Errorf("%s: CAMUNDA_HOSTNAME = %q, want %q (must match the release's global.host)", release.NamespaceSuffix, got, want)
		}
	}
}

// A single-orchestration topology leaves the orchestration release on its
// default host, so the helper must not touch CAMUNDA_HOSTNAME there.
func TestApplyTopologyReleaseHostname_NoAssignedHostIsNoop(t *testing.T) {
	flags := &config.RuntimeFlags{}
	applyTopologyReleaseHostname(flags, topologyReleaseHost(map[string]string{"ORCHA_HOST": "orcha.example.com"}, TopologyRelease{Role: "orchestration", NamespaceSuffix: "orcha"}, 1))
	if _, ok := flags.ExtraEnv["CAMUNDA_HOSTNAME"]; ok {
		t.Errorf("CAMUNDA_HOSTNAME = %q, want unset for a single-orchestration release", flags.ExtraEnv["CAMUNDA_HOSTNAME"])
	}
}

func TestAddTopologyIngressHosts_UsesExplicitSharedHost(t *testing.T) {
	env := map[string]string{}
	opts := RunOptions{ExtraHelmSets: []string{"global.host=abc123-mns.ci.distro.ultrawombat.com"}}

	if err := testTopologyRun(
		env,
		opts,
		[]TopologyRelease{{Role: "hub", NamespaceSuffix: "hub"}, {Role: "orchestration", NamespaceSuffix: "orcha"}},
		[]*deploy.ScenarioContext{{Namespace: "matrix-810-mns-hub"}, {Namespace: "matrix-810-mns-orcha"}},
	).addTopologyIngressHosts(&deploy.ScenarioContext{Namespace: "matrix-810-mns-hub"}); err != nil {
		t.Fatal(err)
	}

	for _, key := range []string{"HUB_HOST", "ORCHA_HOST", "ORCH_HOST"} {
		if got, want := env[key], "abc123-mns.ci.distro.ultrawombat.com"; got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
}

func TestAddTopologyIngressHosts_OmitsOrchestrationHostWithoutRelease(t *testing.T) {
	env := map[string]string{}
	opts := RunOptions{ExtraHelmSets: []string{"global.host=abc123-mns.ci.distro.ultrawombat.com"}}

	if err := testTopologyRun(
		env,
		opts,
		[]TopologyRelease{{Role: "hub", NamespaceSuffix: "hub"}},
		[]*deploy.ScenarioContext{{Namespace: "matrix-810-mns-hub"}},
	).addTopologyIngressHosts(&deploy.ScenarioContext{Namespace: "matrix-810-mns-hub"}); err != nil {
		t.Fatal(err)
	}

	if got, want := env["HUB_HOST"], "abc123-mns.ci.distro.ultrawombat.com"; got != want {
		t.Errorf("HUB_HOST = %q, want %q", got, want)
	}
	if _, ok := env["ORCH_HOST"]; ok {
		t.Errorf("ORCH_HOST = %q, want key omitted without an orchestration release", env["ORCH_HOST"])
	}
}

// TestRunTopologyEntry_SharedStorageServiceOverridesReleaseName pins down
// that when a topology sets SharedStorageService (e.g. "elasticsearch-master"
// for the Elastic Helm chart's headless service, which differs from its
// release name), the resolved EXTERNAL_ELASTICSEARCH_HOST cross-ref env uses
// that service name rather than the SharedStorage release name.
func TestRunTopologyEntry_SharedStorageServiceOverridesReleaseName(t *testing.T) {
	topo := Topology{
		SharedStorage:        "elasticsearch",
		SharedStorageService: "elasticsearch-master",
	}
	hubRelease := TopologyRelease{
		Role: "hub",
		ResolvedDependencies: []ChartDependency{
			{ReleaseName: "elasticsearch"},
		},
	}
	hubCtx := &deploy.ScenarioContext{Namespace: "matrix-810-mns-hub", KeycloakRealm: "mns-abcdef12"}

	sharedStorageService := resolveSharedStorageServiceName(&topo, hubRelease.ResolvedDependencies)
	if sharedStorageService != "elasticsearch-master" {
		t.Fatalf("resolveSharedStorageServiceName = %q, want %q (SharedStorageService must win over SharedStorage release name)", sharedStorageService, "elasticsearch-master")
	}

	env := deploy.BuildTopologyCrossRefEnv(hubCtx, sharedStorageService, "9200", "http")
	if got, want := env["EXTERNAL_ELASTICSEARCH_HOST"], "elasticsearch-master.matrix-810-mns-hub.svc.cluster.local"; got != want {
		t.Errorf("EXTERNAL_ELASTICSEARCH_HOST = %q, want %q", got, want)
	}
}

// TestRunTopologyEntry_SharedStorageServiceFallsBackToSharedStorage pins
// down that when SharedStorageService is empty, the resolution falls back
// to the SharedStorage release name (matched via ResolvedDependencies), and
// the cross-ref env is built from that name.
func TestRunTopologyEntry_SharedStorageServiceFallsBackToSharedStorage(t *testing.T) {
	topo := Topology{
		SharedStorage: "elasticsearch",
	}
	hubRelease := TopologyRelease{
		Role: "hub",
		ResolvedDependencies: []ChartDependency{
			{ReleaseName: "elasticsearch"},
		},
	}
	hubCtx := &deploy.ScenarioContext{Namespace: "matrix-810-mns-hub", KeycloakRealm: "mns-abcdef12"}

	sharedStorageService := resolveSharedStorageServiceName(&topo, hubRelease.ResolvedDependencies)
	if sharedStorageService != "elasticsearch" {
		t.Fatalf("resolveSharedStorageServiceName = %q, want %q (fallback to SharedStorage release name)", sharedStorageService, "elasticsearch")
	}

	env := deploy.BuildTopologyCrossRefEnv(hubCtx, sharedStorageService, "9200", "http")
	if got, want := env["EXTERNAL_ELASTICSEARCH_HOST"], "elasticsearch.matrix-810-mns-hub.svc.cluster.local"; got != want {
		t.Errorf("EXTERNAL_ELASTICSEARCH_HOST = %q, want %q", got, want)
	}
}

// TestBuildOrchestrationZeebeEnv pins down that ORCH_ZEEBE_GRPC/ORCH_ZEEBE_REST
// are derived from the orchestration release's context (release name +
// namespace) into the cross-namespace Zeebe gateway Service FQDN
// ("<release>-zeebe-gateway.<namespace>.svc.cluster.local") on the gRPC
// (26500) and REST (8080) ports — the fix for Web Modeler's empty "Deploy &
// run" cluster dropdown in the multinamespace topology (orchestration.enabled
// is false in the Hub release, so the default cluster helper has
// nothing to register; webModeler.restapi.clusters must be set explicitly).
func TestBuildOrchestrationZeebeEnv(t *testing.T) {
	tests := []struct {
		name      string
		release   string
		namespace string
		wantGRPC  string
		wantREST  string
	}{
		{
			name:      "integration release in foo-orcha namespace",
			release:   "integration",
			namespace: "foo-orcha",
			wantGRPC:  "grpc://integration-zeebe-gateway.foo-orcha.svc.cluster.local:26500",
			wantREST:  "http://integration-zeebe-gateway.foo-orcha.svc.cluster.local:8080",
		},
		{
			name:      "matrix-run namespace",
			release:   "integration",
			namespace: "matrix-810-mns-orcha",
			wantGRPC:  "grpc://integration-zeebe-gateway.matrix-810-mns-orcha.svc.cluster.local:26500",
			wantREST:  "http://integration-zeebe-gateway.matrix-810-mns-orcha.svc.cluster.local:8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orchestrationCtx := &deploy.ScenarioContext{Release: tt.release, Namespace: tt.namespace}

			env := buildOrchestrationZeebeEnv(orchestrationCtx)
			if got := env["ORCH_ZEEBE_GRPC"]; got != tt.wantGRPC {
				t.Errorf("ORCH_ZEEBE_GRPC = %q, want %q", got, tt.wantGRPC)
			}
			if got := env["ORCH_ZEEBE_REST"]; got != tt.wantREST {
				t.Errorf("ORCH_ZEEBE_REST = %q, want %q", got, tt.wantREST)
			}
		})
	}
}

func TestResolveSharedStorageServiceName(t *testing.T) {
	deps := []ChartDependency{
		{ReleaseName: "elasticsearch"},
	}

	t.Run("explicit service name wins", func(t *testing.T) {
		topo := &Topology{
			SharedStorage:        "elasticsearch",
			SharedStorageService: "elasticsearch-master",
		}
		if got := resolveSharedStorageServiceName(topo, deps); got != "elasticsearch-master" {
			t.Errorf("resolveSharedStorageServiceName() = %q, want %q", got, "elasticsearch-master")
		}
	})

	t.Run("falls back to matching dependency release name", func(t *testing.T) {
		topo := &Topology{
			SharedStorage: "elasticsearch",
		}
		if got := resolveSharedStorageServiceName(topo, deps); got != "elasticsearch" {
			t.Errorf("resolveSharedStorageServiceName() = %q, want %q", got, "elasticsearch")
		}
	})

	t.Run("no match returns empty string", func(t *testing.T) {
		topo := &Topology{
			SharedStorage: "opensearch",
		}
		if got := resolveSharedStorageServiceName(topo, deps); got != "" {
			t.Errorf("resolveSharedStorageServiceName() = %q, want empty string", got)
		}
	})
}

func TestTopologyReleaseHostKeyPinsOptimizeToHub(t *testing.T) {
	cases := []struct {
		name               string
		role               string
		namespaceSuffix    string
		orchestrationCount int
		want               string
	}{
		{"optimize with one orchestration", "optimize", "opta", 1, "HUB_HOST"},
		{"optimize with several orchestrations", "optimize", "opta", 2, "HUB_HOST"},
		{"hub with one orchestration keeps default", "hub", "hub", 1, ""},
		{"orchestration with one orchestration keeps default", "orchestration", "orcha", 1, ""},
		{"orchestration with several orchestrations", "orchestration", "orcha", 2, "ORCHA_HOST"},
		{"hub with several orchestrations", "hub", "hub", 2, "HUB_HOST"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := topologyReleaseHostKey(tc.role, tc.namespaceSuffix, tc.orchestrationCount); got != tc.want {
				t.Fatalf("topologyReleaseHostKey(%q, %q, %d) = %q, want %q", tc.role, tc.namespaceSuffix, tc.orchestrationCount, got, tc.want)
			}
		})
	}
}
