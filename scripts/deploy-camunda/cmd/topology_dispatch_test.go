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

package cmd

import (
	"context"
	"reflect"
	"testing"

	"scripts/deploy-camunda/config"
	"scripts/deploy-camunda/deploy"
	"scripts/deploy-camunda/matrix"
)

// TestExtractHelmSetValue_FindsKey pins down that extractHelmSetValue reads
// the "global.host" value out of the "key=value" ExtraHelmSets pairs CI
// passes via --extra-helm-set global.host=<host> — the value runTopologyEntry
// now uses for HUB_HOST/ORCH_HOST instead of fabricating
// "<namespace>.<base-domain>" (see #6651).
func TestExtractHelmSetValue_FindsKey(t *testing.T) {
	pairs := []string{
		"orchestration.upgrade.allowPreReleaseImages=true",
		"global.host=abc123-mns.ci.distro.ultrawombat.com",
	}
	if got := extractHelmSetValue(pairs, "global.host"); got != "abc123-mns.ci.distro.ultrawombat.com" {
		t.Fatalf("extractHelmSetValue() = %q, want %q", got, "abc123-mns.ci.distro.ultrawombat.com")
	}
}

func TestExtractHelmSetValue_MissingKeyReturnsEmpty(t *testing.T) {
	pairs := []string{"orchestration.upgrade.allowPreReleaseImages=true"}
	if got := extractHelmSetValue(pairs, "global.host"); got != "" {
		t.Fatalf("extractHelmSetValue() = %q, want empty string", got)
	}
}

func TestExtractHelmSetValue_EmptyInputReturnsEmpty(t *testing.T) {
	if got := extractHelmSetValue(nil, "global.host"); got != "" {
		t.Fatalf("extractHelmSetValue() = %q, want empty string", got)
	}
}

// TestExtractHelmSetValue_LastDuplicateWins mirrors how repeated
// --extra-helm-set flags for the same key are applied by Helm (last wins).
func TestExtractHelmSetValue_LastDuplicateWins(t *testing.T) {
	pairs := []string{"global.host=first.example.com", "global.host=second.example.com"}
	if got := extractHelmSetValue(pairs, "global.host"); got != "second.example.com" {
		t.Fatalf("extractHelmSetValue() = %q, want %q", got, "second.example.com")
	}
}

// TestExtractHelmSetValue_SkipsMalformedEntries pins down that entries
// without "=" (or with "=" as the first character) are ignored rather than
// panicking or matching spuriously, mirroring parseHelmSetPairs' handling.
func TestExtractHelmSetValue_SkipsMalformedEntries(t *testing.T) {
	pairs := []string{"malformed-entry-no-equals", "=leading-equals-is-skipped", "global.host=host.example.com"}
	if got := extractHelmSetValue(pairs, "global.host"); got != "host.example.com" {
		t.Fatalf("extractHelmSetValue() = %q, want %q", got, "host.example.com")
	}
}

func testTopologyReleases() []matrix.TopologyRelease {
	return []matrix.TopologyRelease{
		{
			Role:            "hub",
			NamespaceSuffix: "hub",
			Features:        []string{"multinamespace-hub"},
			Identity:        "keycloak",
			Dependencies:    []string{"keycloak", "postgresql", "elasticsearch"},
			ResolvedDependencies: []matrix.ChartDependency{
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

func TestSynthesizeReleaseEntry_HubCarriesOwnLayers(t *testing.T) {
	baseEntry := matrix.Entry{
		Version:   "8.10",
		ChartPath: "charts/camunda-platform-8.10",
		Scenario:  "multinamespace",
		Shortname: "mns",
		Auth:      "keycloak",
	}
	releases := testTopologyReleases()

	hubEntry := synthesizeReleaseEntry(baseEntry, releases[0], "gke")

	if hubEntry.Identity != "keycloak" {
		t.Errorf("Hub Identity = %q, want %q", hubEntry.Identity, "keycloak")
	}
	if hubEntry.Persistence != "" {
		t.Errorf("Hub Persistence = %q, want empty", hubEntry.Persistence)
	}
	if len(hubEntry.Dependencies) != 3 {
		t.Fatalf("Hub Dependencies = %v, want 3 entries (keycloak/postgresql/elasticsearch)", hubEntry.Dependencies)
	}
	names := map[string]bool{}
	for _, d := range hubEntry.Dependencies {
		names[d.ReleaseName] = true
	}
	for _, want := range []string{"keycloak", "postgresql", "elasticsearch"} {
		if !names[want] {
			t.Errorf("Hub Dependencies missing %q, got %v", want, hubEntry.Dependencies)
		}
	}
	if hubEntry.Flow != "install" {
		t.Errorf("Flow = %q, want \"install\"", hubEntry.Flow)
	}
	// The release's own overlay file must be wired as a Feature layer (goes
	// through env-var substitution), NOT as an ExtraValues file (bypasses
	// substitution) — this is the correctness fix itself.
	if len(hubEntry.Features) != 1 || hubEntry.Features[0] != "multinamespace-hub" {
		t.Errorf("Features = %v, want [\"multinamespace-hub\"]", hubEntry.Features)
	}
	if len(hubEntry.ExtraValues) != 0 {
		t.Errorf("ExtraValues = %v, want empty (release overlay must go through the substituted Feature path)", hubEntry.ExtraValues)
	}
}

// e2e must never run inside the deploy loop: a release's deploy returns while later releases are
// still undeployed, so a leg run here would test a partial topology, and would repeat for every
// orchestration release. runTopologyEntry runs the legs after the whole topology is up.
func TestSynthesizeReleaseEntry_NoRoleRunsE2EDuringDeploy(t *testing.T) {
	baseEntry := matrix.Entry{
		Version:   "8.10",
		ChartPath: "charts/camunda-platform-8.10",
		Scenario:  "multinamespace",
		Shortname: "mns",
		Auth:      "keycloak",
	}

	for _, rel := range testTopologyReleases() {
		entry := synthesizeReleaseEntry(baseEntry, rel, "gke")
		if !entry.SkipE2E {
			t.Errorf("role %q: SkipE2E = false, want true (e2e is a topology-level phase)", rel.Role)
		}
	}
}

func TestSynthesizeReleaseEntry_OrchestrationHasNoDependenciesOrPostDeployHook(t *testing.T) {
	hook := &matrix.LifecycleHook{Script: "post-deploy-hub-ping.sh"}
	baseEntry := matrix.Entry{Version: "8.10", ChartPath: "charts/camunda-platform-8.10", Scenario: "multinamespace", Shortname: "mns", Auth: "keycloak", PostDeploy: hook}
	releases := testTopologyReleases()

	for _, rel := range releases[1:] {
		orchEntry := synthesizeReleaseEntry(baseEntry, rel, "gke")
		if orchEntry.Identity != "keycloak-external" {
			t.Errorf("orchestration Identity = %q, want %q", orchEntry.Identity, "keycloak-external")
		}
		if orchEntry.Persistence != "elasticsearch-external" {
			t.Errorf("orchestration Persistence = %q, want %q", orchEntry.Persistence, "elasticsearch-external")
		}
		if len(orchEntry.Dependencies) != 0 {
			t.Errorf("orchestration release %q Dependencies = %v, want empty (must not deploy its own companion ES)", rel.NamespaceSuffix, orchEntry.Dependencies)
		}
		if len(orchEntry.Features) != 1 || orchEntry.Features[0] != "multinamespace-orchestration" {
			t.Errorf("orchestration release %q Features = %v, want [\"multinamespace-orchestration\"] (substituted Feature path, not ExtraValues)", rel.NamespaceSuffix, orchEntry.Features)
		}
		if len(orchEntry.ExtraValues) != 0 {
			t.Errorf("orchestration release %q ExtraValues = %v, want empty", rel.NamespaceSuffix, orchEntry.ExtraValues)
		}
		if orchEntry.PostDeploy != nil {
			t.Errorf("orchestration release %q PostDeploy = %v, want nil so the hook runs once at topology level", rel.NamespaceSuffix, orchEntry.PostDeploy)
		}
	}
	hubEntry := synthesizeReleaseEntry(baseEntry, releases[0], "gke")
	if hubEntry.PostDeploy != nil {
		t.Errorf("Hub PostDeploy = %v, want nil so the hook runs once at topology level", hubEntry.PostDeploy)
	}
}

func TestSynthesizeReleaseOpts_NamespaceOverridePinsRelease(t *testing.T) {
	base := matrix.RunOptions{
		RepoRoot:          "/repo",
		KubeContext:       "kube-ctx",
		IngressBaseDomain: "ci.example.com",
	}

	opts := synthesizeReleaseOpts(base, "gke", "matrix-810-mns-hub")

	if opts.NamespaceOverride != "matrix-810-mns-hub" {
		t.Errorf("NamespaceOverride = %q, want %q", opts.NamespaceOverride, "matrix-810-mns-hub")
	}
	if opts.RepoRoot != "/repo" {
		t.Errorf("RepoRoot not propagated: got %q", opts.RepoRoot)
	}
	if opts.Platform != "gke" {
		t.Errorf("Platform = %q, want gke", opts.Platform)
	}
}

// TestSynthesizeReleaseEntry_DistinctNamespacesPerRelease pins down that
// pairing synthesizeReleaseOpts with each release's own precomputed
// namespace (as runTopologyEntry does) produces three distinct
// NamespaceOverride values — i.e. cross-ref env (built from these same
// namespaces) is computed against real, distinct release namespaces before
// any release is deployed.
func TestSynthesizeReleaseEntry_DistinctNamespacesPerRelease(t *testing.T) {
	base := matrix.RunOptions{RepoRoot: "/repo"}
	namespaces := []string{"matrix-810-mns-hub", "matrix-810-mns-orcha", "matrix-810-mns-orchb"}

	seen := map[string]bool{}
	for _, ns := range namespaces {
		opts := synthesizeReleaseOpts(base, "gke", ns)
		if seen[opts.NamespaceOverride] {
			t.Fatalf("duplicate NamespaceOverride %q", opts.NamespaceOverride)
		}
		seen[opts.NamespaceOverride] = true
	}
	if len(seen) != 3 {
		t.Fatalf("expected 3 distinct namespaces, got %d", len(seen))
	}
}

// TestSynthesizeReleaseOpts_PropagatesPerPlatformIngressBaseDomains pins down
// the fix for the live-GKE bug where --ingress-base-domain-gke (populating
// the per-platform IngressBaseDomains map) was silently dropped for topology
// entries: synthesizeReleaseOpts used to hand-copy a subset of fields into a
// bespoke options struct, and IngressBaseDomains was missing from it — so
// resolveIngressBaseDomain(opts, "gke") fell through to the (also empty)
// generic field, ResolveIngressHostname() returned "", and CAMUNDA_HOSTNAME
// was never derived - failing the Hub release's preflight with
// "CAMUNDA_HOSTNAME unset".
func TestSynthesizeReleaseOpts_PropagatesPerPlatformIngressBaseDomains(t *testing.T) {
	base := matrix.RunOptions{
		RepoRoot: "/repo",
		// Only the per-platform map is set — mirrors a user who passed
		// --ingress-base-domain-gke and never set the generic
		// --ingress-base-domain.
		IngressBaseDomains: map[string]string{"gke": "ci.distro.ultrawombat.com"},
	}

	opts := synthesizeReleaseOpts(base, "gke", "matrix-810-mns-hub")

	if got := opts.IngressBaseDomains["gke"]; got != "ci.distro.ultrawombat.com" {
		t.Fatalf("IngressBaseDomains[\"gke\"] = %q, want %q — per-platform map was dropped", got, "ci.distro.ultrawombat.com")
	}

	// More directly: build the actual per-release flags the way
	// runTopologyEntry does, and confirm the ingress hostname (and hence
	// CAMUNDA_HOSTNAME) is non-empty.
	releaseEntry := matrix.Entry{
		Version:   "8.10",
		ChartPath: "charts/camunda-platform-8.10",
		Scenario:  "multinamespace",
		Shortname: "mns",
		Auth:      "keycloak",
		Flow:      "install",
		Platform:  "gke",
	}
	flags, _, _, _, cleanup, err := matrix.BuildEntryFlags(releaseEntry, opts)
	defer cleanup()
	if err != nil {
		t.Fatalf("BuildEntryFlags returned error: %v", err)
	}
	if got := flags.ResolveIngressHostname(); got == "" {
		t.Fatal("ResolveIngressHostname() is empty — CAMUNDA_HOSTNAME would be unset at preflight, reproducing the live-GKE bug")
	}
}

// TestSynthesizeReleaseOpts_GenericIngressBaseDomainStillWorks pins down
// that the generic --ingress-base-domain path (no per-platform map) is
// unaffected by this fix.
func TestSynthesizeReleaseOpts_GenericIngressBaseDomainStillWorks(t *testing.T) {
	base := matrix.RunOptions{
		RepoRoot:          "/repo",
		IngressBaseDomain: "ci.distro.ultrawombat.com",
	}

	opts := synthesizeReleaseOpts(base, "gke", "matrix-810-mns-hub")

	if opts.IngressBaseDomain != "ci.distro.ultrawombat.com" {
		t.Errorf("IngressBaseDomain = %q, want %q", opts.IngressBaseDomain, "ci.distro.ultrawombat.com")
	}
}

func TestAddTopologyIngressHosts_DerivesPerReleaseHosts(t *testing.T) {
	env := deploy.BuildTopologyCrossRefEnv(
		&deploy.ScenarioContext{Namespace: "matrix-810-mns-hub"},
		"elasticsearch",
		"9200",
		"http",
	)
	opts := matrix.RunOptions{
		IngressBaseDomains: map[string]string{"gke": "ci.distro.ultrawombat.com"},
		IngressBaseDomain:  "fallback.example.com",
	}

	addTopologyIngressHosts(
		env,
		opts,
		"gke",
		&deploy.ScenarioContext{Namespace: "matrix-810-mns-hub"},
		[]matrix.TopologyRelease{{Role: "hub", NamespaceSuffix: "hub"}, {Role: "orchestration", NamespaceSuffix: "orcha"}},
		[]*deploy.ScenarioContext{{Namespace: "matrix-810-mns-hub"}, {Namespace: "matrix-810-mns-orcha"}},
	)

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

	addTopologyIngressHosts(env, matrix.RunOptions{IngressBaseDomain: "ci.example.com"}, "gke", contexts[0], releases, contexts)

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
	release := matrix.TopologyRelease{
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

func TestBuildTopologyReleaseEnv_PublishesServedReferencesForOptimize(t *testing.T) {
	shared := map[string]string{
		"ORCHA_NAMESPACE":                  "ns-orcha",
		"ORCHA_HOST":                       "orcha.example.com",
		"ORCHA_ORCHESTRATION_INDEX_PREFIX": "job-orcha",
		"ORCHB_NAMESPACE":                  "ns-orchb",
		"ORCHB_HOST":                       "orchb.example.com",
		"ORCHB_ORCHESTRATION_INDEX_PREFIX": "job-orchb",
	}
	release := matrix.TopologyRelease{
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
	base := matrix.TopologyRelease{Role: "optimize", NamespaceSuffix: "opta", Tenant: "default", OptimizeContextPath: "/optimize-a"}

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
	env := buildTopologyReleaseEnv(map[string]string{}, matrix.TopologyRelease{
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
// matrix.Topology.Validate rejects the entry outright; this covers the ordering
// that makes the rejection safe to rely on.
func TestBuildTopologyReleaseEnv_DerivedKeysOutrankReleaseEnv(t *testing.T) {
	shared := map[string]string{
		"ORCHA_NAMESPACE":                  "ns-orcha",
		"ORCHA_HOST":                       "orcha.example.com",
		"ORCHA_ORCHESTRATION_INDEX_PREFIX": "job-orcha",
	}
	release := matrix.TopologyRelease{
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
	release := matrix.TopologyRelease{
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

func TestAddTopologyIngressHosts_UsesExplicitSharedHost(t *testing.T) {
	env := map[string]string{}
	opts := matrix.RunOptions{ExtraHelmSets: []string{"global.host=abc123-mns.ci.distro.ultrawombat.com"}}

	addTopologyIngressHosts(
		env,
		opts,
		"gke",
		&deploy.ScenarioContext{Namespace: "matrix-810-mns-hub"},
		[]matrix.TopologyRelease{{Role: "hub", NamespaceSuffix: "hub"}, {Role: "orchestration", NamespaceSuffix: "orcha"}},
		[]*deploy.ScenarioContext{{Namespace: "matrix-810-mns-hub"}, {Namespace: "matrix-810-mns-orcha"}},
	)

	for _, key := range []string{"HUB_HOST", "ORCHA_HOST", "ORCH_HOST"} {
		if got, want := env[key], "abc123-mns.ci.distro.ultrawombat.com"; got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
}

func TestAddTopologyIngressHosts_OmitsOrchestrationHostWithoutRelease(t *testing.T) {
	env := map[string]string{}
	opts := matrix.RunOptions{ExtraHelmSets: []string{"global.host=abc123-mns.ci.distro.ultrawombat.com"}}

	addTopologyIngressHosts(
		env,
		opts,
		"gke",
		&deploy.ScenarioContext{Namespace: "matrix-810-mns-hub"},
		[]matrix.TopologyRelease{{Role: "hub", NamespaceSuffix: "hub"}},
		[]*deploy.ScenarioContext{{Namespace: "matrix-810-mns-hub"}},
	)

	if got, want := env["HUB_HOST"], "abc123-mns.ci.distro.ultrawombat.com"; got != want {
		t.Errorf("HUB_HOST = %q, want %q", got, want)
	}
	if _, ok := env["ORCH_HOST"]; ok {
		t.Errorf("ORCH_HOST = %q, want key omitted without an orchestration release", env["ORCH_HOST"])
	}
}

// TestApplyTopologyReleaseOverrides_ForcesExternalSecrets pins down the fix
// for the live-GKE bug where the Hub release's bundled Keycloak
// CreateContainerConfigError'd on a missing "integration-test-credentials"
// secret. matrix.BuildEntryFlags sets Secrets.ExternalSecrets = (NamespaceOverride
// == ""), which is false for every topology release (they always set
// NamespaceOverride to their own namespace) — so external secrets were never
// applied. The topology driver has no cluster-setup-secrets action run for
// it, so it must force ExternalSecrets on for every release itself.
func TestApplyTopologyReleaseOverrides_ForcesExternalSecrets(t *testing.T) {
	for _, tc := range []struct {
		name    string
		initial bool
	}{
		{"was false (BuildEntryFlags default under NamespaceOverride)", false},
		{"was already true", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			flags := &config.RuntimeFlags{
				Secrets: config.SecretsFlags{ExternalSecrets: tc.initial},
			}

			applyTopologyReleaseOverrides(flags, map[string]string{"HUB_NAMESPACE": "ns-hub"})

			if !flags.Secrets.ExternalSecrets {
				t.Fatal("Secrets.ExternalSecrets = false, want true for every topology release")
			}
			if !flags.Docker.EnsureDockerRegistry {
				t.Fatal("Docker.EnsureDockerRegistry = false, want true for every topology release")
			}
		})
	}
}

// TestApplyTopologyReleaseOverrides_InjectsCrossRefEnv pins down that the
// cross-ref env is still applied (regression guard alongside the
// ExternalSecrets fix above).
func TestApplyTopologyReleaseOverrides_InjectsCrossRefEnv(t *testing.T) {
	flags := &config.RuntimeFlags{}
	crossRefEnv := map[string]string{
		"HUB_NAMESPACE":  "ns-hub",
		"KEYCLOAK_REALM": "realm-1",
	}

	applyTopologyReleaseOverrides(flags, crossRefEnv)

	if flags.ExtraEnv["HUB_NAMESPACE"] != "ns-hub" || flags.ExtraEnv["KEYCLOAK_REALM"] != "realm-1" {
		t.Fatalf("ExtraEnv = %v, want cross-ref env injected", flags.ExtraEnv)
	}
}

// TestApplyTopologyReleaseOverrides_MergesWithExistingExtraEnv pins down
// that pre-seeded ExtraEnv entries (e.g. per-entry client-IDs injected
// upstream) survive the cross-ref env merge instead of being dropped by a
// wholesale map replacement.
func TestApplyTopologyReleaseOverrides_MergesWithExistingExtraEnv(t *testing.T) {
	flags := &config.RuntimeFlags{
		ExtraEnv: map[string]string{"VENOM_CLIENT_ID": "venom"},
	}

	applyTopologyReleaseOverrides(flags, map[string]string{"HUB_NAMESPACE": "ns-hub"})

	if flags.ExtraEnv["VENOM_CLIENT_ID"] != "venom" {
		t.Errorf("ExtraEnv[VENOM_CLIENT_ID] = %q, want %q (pre-existing entry should survive)", flags.ExtraEnv["VENOM_CLIENT_ID"], "venom")
	}
	if flags.ExtraEnv["HUB_NAMESPACE"] != "ns-hub" {
		t.Errorf("ExtraEnv[HUB_NAMESPACE] = %q, want %q (cross-ref entry should be added)", flags.ExtraEnv["HUB_NAMESPACE"], "ns-hub")
	}
	if !flags.Secrets.ExternalSecrets {
		t.Error("Secrets.ExternalSecrets = false, want true")
	}
	if !flags.Docker.EnsureDockerRegistry {
		t.Error("Docker.EnsureDockerRegistry = false, want true")
	}
}

// TestApplyTopologyReleaseOverrides_AllReleaseRolesGetExternalSecrets
// exercises the fix across every role synthesizeReleaseEntry produces
// (Hub + both orchestration releases), confirming the override is
// unconditional — not role-dependent.
func TestApplyTopologyReleaseOverrides_AllReleaseRolesGetExternalSecrets(t *testing.T) {
	for _, rel := range testTopologyReleases() {
		flags := &config.RuntimeFlags{
			Secrets: config.SecretsFlags{ExternalSecrets: false},
		}
		applyTopologyReleaseOverrides(flags, map[string]string{"HUB_NAMESPACE": "ns-hub"})
		if !flags.Secrets.ExternalSecrets {
			t.Errorf("release role %q (namespace-suffix %q): ExternalSecrets = false, want true", rel.Role, rel.NamespaceSuffix)
		}
		if !flags.Docker.EnsureDockerRegistry {
			t.Errorf("release role %q (namespace-suffix %q): Docker.EnsureDockerRegistry = false, want true", rel.Role, rel.NamespaceSuffix)
		}
	}
}

// resolveSharedStorageService mirrors the shared-storage-service resolution
// snippet in runTopologyEntry (cmd/matrix.go) so it can be pinned down in
// isolation without exercising the full topology dispatch/deploy path.
func resolveSharedStorageService(topo matrix.Topology, hubRelease matrix.TopologyRelease) string {
	if topo.SharedStorageService != "" {
		return topo.SharedStorageService
	}
	for _, r := range hubRelease.ResolvedDependencies {
		if r.ReleaseName == topo.SharedStorage {
			return r.ReleaseName
		}
	}
	return ""
}

// TestRunTopologyEntry_SharedStorageServiceOverridesReleaseName pins down
// that when a topology sets SharedStorageService (e.g. "elasticsearch-master"
// for the Elastic Helm chart's headless service, which differs from its
// release name), the resolved EXTERNAL_ELASTICSEARCH_HOST cross-ref env uses
// that service name rather than the SharedStorage release name.
func TestRunTopologyEntry_SharedStorageServiceOverridesReleaseName(t *testing.T) {
	topo := matrix.Topology{
		SharedStorage:        "elasticsearch",
		SharedStorageService: "elasticsearch-master",
	}
	hubRelease := matrix.TopologyRelease{
		Role: "hub",
		ResolvedDependencies: []matrix.ChartDependency{
			{ReleaseName: "elasticsearch"},
		},
	}
	hubCtx := &deploy.ScenarioContext{Namespace: "matrix-810-mns-hub", KeycloakRealm: "mns-abcdef12"}

	sharedStorageService := resolveSharedStorageService(topo, hubRelease)
	if sharedStorageService != "elasticsearch-master" {
		t.Fatalf("resolveSharedStorageService = %q, want %q (SharedStorageService must win over SharedStorage release name)", sharedStorageService, "elasticsearch-master")
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
	topo := matrix.Topology{
		SharedStorage: "elasticsearch",
	}
	hubRelease := matrix.TopologyRelease{
		Role: "hub",
		ResolvedDependencies: []matrix.ChartDependency{
			{ReleaseName: "elasticsearch"},
		},
	}
	hubCtx := &deploy.ScenarioContext{Namespace: "matrix-810-mns-hub", KeycloakRealm: "mns-abcdef12"}

	sharedStorageService := resolveSharedStorageService(topo, hubRelease)
	if sharedStorageService != "elasticsearch" {
		t.Fatalf("resolveSharedStorageService = %q, want %q (fallback to SharedStorage release name)", sharedStorageService, "elasticsearch")
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

// TestSynthesizeReleaseOpts_PropagatesHelmTimeout pins down that --timeout
// (helm deployment timeout in minutes) is forwarded to every topology
// release's matrix.RunOptions — same class of bug as the ingress-base-domain
// plumbing fix: synthesizeReleaseOpts must carry every field matrix.RunOptions
// needs, or the topology path silently falls back to defaults regardless of
// what the user passed. See TestSynthesizeReleaseOpts_ForwardsEntireBaseRunOptions
// below for the general-case regression guard against this whole bug class.
func TestSynthesizeReleaseOpts_PropagatesHelmTimeout(t *testing.T) {
	base := matrix.RunOptions{
		RepoRoot:    "/repo",
		HelmTimeout: 25,
	}

	opts := synthesizeReleaseOpts(base, "gke", "matrix-810-mns-hub")

	if opts.HelmTimeout != 25 {
		t.Errorf("HelmTimeout = %d, want 25", opts.HelmTimeout)
	}
}

// TestSynthesizeReleaseOpts_ForwardsEntireBaseRunOptions is the general-case
// regression guard for the whole "synthesizeReleaseOpts drops a field" bug
// class — we've been bitten by this 4 times now (IngressBaseDomains,
// HelmTimeout, and DeleteNamespaceFirst most recently: --delete-namespace was
// silently ignored for topology deploys, running over stale namespace state).
//
// synthesizeReleaseOpts now copies the ENTIRE base matrix.RunOptions and
// overrides only Platform and NamespaceOverride — so this test constructs a
// base with many fields set to distinctive non-zero values and asserts every
// one of them survives into the per-release RunOptions unchanged, with only
// Platform/NamespaceOverride differing.
func TestSynthesizeReleaseOpts_ForwardsEntireBaseRunOptions(t *testing.T) {
	base := matrix.RunOptions{
		DryRun:                true,
		Coverage:              true,
		StopOnFailure:         true,
		Cleanup:               true,
		DeleteNamespaceFirst:  true,
		KubeContexts:          map[string]string{"gke": "gke-ctx"},
		KubeContext:           "fallback-ctx",
		NamespacePrefix:       "matrix",
		Platform:              "gke",
		MaxParallel:           3,
		TestE2E:               true,
		TestAll:               true,
		RepoRoot:              "/repo",
		EnvFiles:              map[string]string{"8.10": ".env.810"},
		EnvFile:               ".env",
		IngressBaseDomains:    map[string]string{"gke": "ci.distro.ultrawombat.com"},
		IngressBaseDomain:     "ci.distro.ultrawombat.com",
		LogLevel:              "debug",
		SkipDependencyUpdate:  true,
		VaultBackedSecrets:    map[string]bool{"eks": true},
		UseVaultBackedSecrets: true,
		KeycloakHost:          "keycloak.example.com",
		KeycloakProtocol:      "https",
		UpgradeFromVersion:    "8.9",
		HelmTimeout:           25,
		DockerUsername:        "docker-user",
		DockerPassword:        "docker-pass",
		EnsureDockerRegistry:  true,
		DockerHubUsername:     "dockerhub-user",
		DockerHubPassword:     "dockerhub-pass",
		EnsureDockerHub:       true,
		UseLatest:             true,
		UseQA:                 true,
		ForceImageOverrides:   true,
		ExtraHelmArgs:         []string{"--set-file=foo=bar"},
		ExtraHelmSets:         []string{"a=b"},
		ExtraValues:           []string{"/tmp/extra.yaml"},
		NamespaceOverride:     "should-be-overridden",
		ChartRef:              "oci://example/camunda-platform",
		ChartRefVersion:       "13-rc-latest",
		LogDir:                "/tmp/matrix-logs",
	}

	got := synthesizeReleaseOpts(base, "eks", "matrix-810-mns-hub")

	// The two fields synthesizeReleaseOpts is explicitly allowed to change.
	if got.Platform != "eks" {
		t.Errorf("Platform = %q, want %q (overridden per-release)", got.Platform, "eks")
	}
	if got.NamespaceOverride != "matrix-810-mns-hub" {
		t.Errorf("NamespaceOverride = %q, want %q (overridden per-release)", got.NamespaceOverride, "matrix-810-mns-hub")
	}

	// Every other field must be forwarded byte-for-byte from base. Compare by
	// resetting the two allowed-to-differ fields on a copy of got back to
	// base's values, then requiring deep equality with base.
	normalized := got
	normalized.Platform = base.Platform
	normalized.NamespaceOverride = base.NamespaceOverride

	if !reflect.DeepEqual(normalized, base) {
		t.Fatalf("synthesizeReleaseOpts dropped or altered a field other than Platform/NamespaceOverride.\nbase:       %+v\ngot (norm): %+v", base, normalized)
	}
}

func TestResolveSharedStorageServiceName(t *testing.T) {
	deps := []matrix.ChartDependency{
		{ReleaseName: "elasticsearch"},
	}

	t.Run("explicit service name wins", func(t *testing.T) {
		topo := &matrix.Topology{
			SharedStorage:        "elasticsearch",
			SharedStorageService: "elasticsearch-master",
		}
		if got := resolveSharedStorageServiceName(topo, deps); got != "elasticsearch-master" {
			t.Errorf("resolveSharedStorageServiceName() = %q, want %q", got, "elasticsearch-master")
		}
	})

	t.Run("falls back to matching dependency release name", func(t *testing.T) {
		topo := &matrix.Topology{
			SharedStorage: "elasticsearch",
		}
		if got := resolveSharedStorageServiceName(topo, deps); got != "elasticsearch" {
			t.Errorf("resolveSharedStorageServiceName() = %q, want %q", got, "elasticsearch")
		}
	})

	t.Run("no match returns empty string", func(t *testing.T) {
		topo := &matrix.Topology{
			SharedStorage: "opensearch",
		}
		if got := resolveSharedStorageServiceName(topo, deps); got != "" {
			t.Errorf("resolveSharedStorageServiceName() = %q, want empty string", got)
		}
	})
}

func TestTopologyDeployOrder_HubFirst(t *testing.T) {
	order, err := topologyDeployOrder(testTopologyReleases())
	if err != nil {
		t.Fatalf("topologyDeployOrder() unexpected error: %v", err)
	}
	if len(order) != 3 || order[0] != 0 {
		t.Fatalf("order = %v, want Hub (index 0) first over 3 releases", order)
	}
	// every orchestration (depends-on Hub) must come after index 0
	for _, idx := range order[1:] {
		if idx == 0 {
			t.Fatalf("Hub index appeared after position 0: %v", order)
		}
	}
}

func TestTopologyDeployOrder_ChainedDependency(t *testing.T) {
	releases := []matrix.TopologyRelease{
		{Role: "orchestration", NamespaceSuffix: "orchb", DependsOn: "hub"},
		{Role: "hub", NamespaceSuffix: "hub"},
		{Role: "aux", NamespaceSuffix: "aux", DependsOn: "orchestration"},
	}
	order, err := topologyDeployOrder(releases)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	pos := map[int]int{}
	for p, idx := range order {
		pos[idx] = p
	}
	if !(pos[1] < pos[0] && pos[0] < pos[2]) {
		t.Fatalf("order %v does not satisfy hub(1) < orchestration(0) < aux(2)", order)
	}
}

func TestTopologyDeployOrder_CycleErrors(t *testing.T) {
	releases := []matrix.TopologyRelease{
		{Role: "a", DependsOn: "b"},
		{Role: "b", DependsOn: "a"},
	}
	if _, err := topologyDeployOrder(releases); err == nil {
		t.Fatal("expected cycle error, got nil")
	}
}

func TestRunTopologyEntry_RejectsUnsupportedAuthFlow(t *testing.T) {
	base := matrix.Entry{
		Version:  "8.10",
		Scenario: "multinamespace",
		Auth:     "oidc", // not keycloak
		Flow:     "install",
		Topology: &matrix.Topology{Name: "t", Releases: testTopologyReleases()},
	}
	if err := runTopologyEntry(context.Background(), base, matrix.RunOptions{}); err == nil {
		t.Fatal("expected error for auth=oidc, got nil")
	}

	base.Auth = "keycloak"
	base.Flow = "upgrade-minor" // not install
	if err := runTopologyEntry(context.Background(), base, matrix.RunOptions{}); err == nil {
		t.Fatal("expected error for flow=upgrade-minor, got nil")
	}
}

func TestRunTopologyEntry_RejectsChartRef(t *testing.T) {
	entry := matrix.Entry{
		Version:  "8.10",
		Scenario: "multinamespace",
		Auth:     "keycloak",
		Flow:     "install",
		Topology: &matrix.Topology{Name: "t", Releases: testTopologyReleases()},
	}
	opts := matrix.RunOptions{ChartRef: "oci://example/camunda-platform"}

	err := runTopologyEntry(context.Background(), entry, opts)
	if err == nil {
		t.Fatal("expected error when --chart-ref is set, got nil")
	}
}

func TestRunTopologyEntry_RejectsCleanup(t *testing.T) {
	entry := matrix.Entry{
		Version:  "8.10",
		Scenario: "multinamespace",
		Auth:     "keycloak",
		Flow:     "install",
		Topology: &matrix.Topology{Name: "t", Releases: testTopologyReleases()},
	}
	opts := matrix.RunOptions{Cleanup: true}

	err := runTopologyEntry(context.Background(), entry, opts)
	if err == nil {
		t.Fatal("expected error when --cleanup is set, got nil")
	}
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
