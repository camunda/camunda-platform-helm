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
// now uses for MGMT_HOST/ORCH_HOST instead of fabricating
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
			Role:            "management",
			NamespaceSuffix: "mgmt",
			Values:          "features/multinamespace-management.yaml",
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
			Values:          "features/multinamespace-orchestration.yaml",
			Identity:        "keycloak-external",
			Persistence:     "elasticsearch-external",
			DependsOn:       "management",
		},
		{
			Role:            "orchestration",
			NamespaceSuffix: "orchb",
			Values:          "features/multinamespace-orchestration.yaml",
			Identity:        "keycloak-external",
			Persistence:     "elasticsearch-external",
			DependsOn:       "management",
		},
	}
}

func TestSynthesizeReleaseEntry_ManagementCarriesOwnLayers(t *testing.T) {
	baseEntry := matrix.Entry{
		Version:   "8.10",
		ChartPath: "charts/camunda-platform-8.10",
		Scenario:  "multinamespace",
		Shortname: "mns",
		Auth:      "keycloak",
	}
	releases := testTopologyReleases()

	mgmtEntry := synthesizeReleaseEntry(baseEntry, releases[0], "gke")

	if mgmtEntry.Identity != "keycloak" {
		t.Errorf("management Identity = %q, want %q", mgmtEntry.Identity, "keycloak")
	}
	if mgmtEntry.Persistence != "" {
		t.Errorf("management Persistence = %q, want empty", mgmtEntry.Persistence)
	}
	if len(mgmtEntry.Dependencies) != 3 {
		t.Fatalf("management Dependencies = %v, want 3 entries (keycloak/postgresql/elasticsearch)", mgmtEntry.Dependencies)
	}
	names := map[string]bool{}
	for _, d := range mgmtEntry.Dependencies {
		names[d.ReleaseName] = true
	}
	for _, want := range []string{"keycloak", "postgresql", "elasticsearch"} {
		if !names[want] {
			t.Errorf("management Dependencies missing %q, got %v", want, mgmtEntry.Dependencies)
		}
	}
	if mgmtEntry.Flow != "install" {
		t.Errorf("Flow = %q, want \"install\"", mgmtEntry.Flow)
	}
	// The release's own overlay file must be wired as a Feature layer (goes
	// through env-var substitution), NOT as an ExtraValues file (bypasses
	// substitution) — this is the correctness fix itself.
	if len(mgmtEntry.Features) != 1 || mgmtEntry.Features[0] != "multinamespace-management" {
		t.Errorf("Features = %v, want [\"multinamespace-management\"]", mgmtEntry.Features)
	}
	if len(mgmtEntry.ExtraValues) != 0 {
		t.Errorf("ExtraValues = %v, want empty (release overlay must go through the substituted Feature path)", mgmtEntry.ExtraValues)
	}
}

func TestSynthesizeReleaseEntry_OrchestrationHasNoDependencies(t *testing.T) {
	baseEntry := matrix.Entry{Version: "8.10", ChartPath: "charts/camunda-platform-8.10", Scenario: "multinamespace", Shortname: "mns", Auth: "keycloak"}
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
	}
}

func TestSynthesizeReleaseEntry_NonFeatureValuesFallsBackToExtraValues(t *testing.T) {
	baseEntry := matrix.Entry{Version: "8.10", ChartPath: "charts/camunda-platform-8.10", Scenario: "multinamespace"}
	rel := matrix.TopologyRelease{Role: "management", NamespaceSuffix: "mgmt", Values: "legacy/some-overlay.yaml"}

	got := synthesizeReleaseEntry(baseEntry, rel, "gke")
	if len(got.Features) != 0 {
		t.Errorf("Features = %v, want empty for a non-features/ values path", got.Features)
	}
	if len(got.ExtraValues) != 1 {
		t.Fatalf("ExtraValues = %v, want a single fallback entry", got.ExtraValues)
	}
}

func TestSynthesizeReleaseOpts_NamespaceOverridePinsRelease(t *testing.T) {
	base := matrix.RunOptions{
		RepoRoot:          "/repo",
		KubeContext:       "kube-ctx",
		IngressBaseDomain: "ci.example.com",
	}

	opts := synthesizeReleaseOpts(base, "gke", "matrix-810-mns-mgmt")

	if opts.NamespaceOverride != "matrix-810-mns-mgmt" {
		t.Errorf("NamespaceOverride = %q, want %q", opts.NamespaceOverride, "matrix-810-mns-mgmt")
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
	namespaces := []string{"matrix-810-mns-mgmt", "matrix-810-mns-orcha", "matrix-810-mns-orchb"}

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
// was never derived — failing the management release's preflight with
// "CAMUNDA_HOSTNAME unset".
func TestSynthesizeReleaseOpts_PropagatesPerPlatformIngressBaseDomains(t *testing.T) {
	base := matrix.RunOptions{
		RepoRoot: "/repo",
		// Only the per-platform map is set — mirrors a user who passed
		// --ingress-base-domain-gke and never set the generic
		// --ingress-base-domain.
		IngressBaseDomains: map[string]string{"gke": "ci.distro.ultrawombat.com"},
	}

	opts := synthesizeReleaseOpts(base, "gke", "matrix-810-mns-mgmt")

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

	opts := synthesizeReleaseOpts(base, "gke", "matrix-810-mns-mgmt")

	if opts.IngressBaseDomain != "ci.distro.ultrawombat.com" {
		t.Errorf("IngressBaseDomain = %q, want %q", opts.IngressBaseDomain, "ci.distro.ultrawombat.com")
	}
}

// TestApplyTopologyReleaseOverrides_ForcesExternalSecrets pins down the fix
// for the live-GKE bug where the management release's bundled Keycloak
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

			applyTopologyReleaseOverrides(flags, map[string]string{"MGMT_NAMESPACE": "ns-mgmt"})

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
		"MGMT_NAMESPACE": "ns-mgmt",
		"KEYCLOAK_REALM": "realm-1",
	}

	applyTopologyReleaseOverrides(flags, crossRefEnv)

	if flags.ExtraEnv["MGMT_NAMESPACE"] != "ns-mgmt" || flags.ExtraEnv["KEYCLOAK_REALM"] != "realm-1" {
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

	applyTopologyReleaseOverrides(flags, map[string]string{"MGMT_NAMESPACE": "ns-mgmt"})

	if flags.ExtraEnv["VENOM_CLIENT_ID"] != "venom" {
		t.Errorf("ExtraEnv[VENOM_CLIENT_ID] = %q, want %q (pre-existing entry should survive)", flags.ExtraEnv["VENOM_CLIENT_ID"], "venom")
	}
	if flags.ExtraEnv["MGMT_NAMESPACE"] != "ns-mgmt" {
		t.Errorf("ExtraEnv[MGMT_NAMESPACE] = %q, want %q (cross-ref entry should be added)", flags.ExtraEnv["MGMT_NAMESPACE"], "ns-mgmt")
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
// (management + both orchestration releases), confirming the override is
// unconditional — not role-dependent.
func TestApplyTopologyReleaseOverrides_AllReleaseRolesGetExternalSecrets(t *testing.T) {
	for _, rel := range testTopologyReleases() {
		flags := &config.RuntimeFlags{
			Secrets: config.SecretsFlags{ExternalSecrets: false},
		}
		applyTopologyReleaseOverrides(flags, map[string]string{"MGMT_NAMESPACE": "ns-mgmt"})
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
func resolveSharedStorageService(topo matrix.Topology, managementRelease matrix.TopologyRelease) string {
	if topo.SharedStorageService != "" {
		return topo.SharedStorageService
	}
	for _, r := range managementRelease.ResolvedDependencies {
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
	managementRelease := matrix.TopologyRelease{
		Role: "management",
		ResolvedDependencies: []matrix.ChartDependency{
			{ReleaseName: "elasticsearch"},
		},
	}
	managementCtx := &deploy.ScenarioContext{Namespace: "matrix-810-mns-mgmt", KeycloakRealm: "mns-abcdef12"}

	sharedStorageService := resolveSharedStorageService(topo, managementRelease)
	if sharedStorageService != "elasticsearch-master" {
		t.Fatalf("resolveSharedStorageService = %q, want %q (SharedStorageService must win over SharedStorage release name)", sharedStorageService, "elasticsearch-master")
	}

	env := deploy.BuildTopologyCrossRefEnv(managementCtx, sharedStorageService, "9200", "http")
	if got, want := env["EXTERNAL_ELASTICSEARCH_HOST"], "elasticsearch-master.matrix-810-mns-mgmt.svc.cluster.local"; got != want {
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
	managementRelease := matrix.TopologyRelease{
		Role: "management",
		ResolvedDependencies: []matrix.ChartDependency{
			{ReleaseName: "elasticsearch"},
		},
	}
	managementCtx := &deploy.ScenarioContext{Namespace: "matrix-810-mns-mgmt", KeycloakRealm: "mns-abcdef12"}

	sharedStorageService := resolveSharedStorageService(topo, managementRelease)
	if sharedStorageService != "elasticsearch" {
		t.Fatalf("resolveSharedStorageService = %q, want %q (fallback to SharedStorage release name)", sharedStorageService, "elasticsearch")
	}

	env := deploy.BuildTopologyCrossRefEnv(managementCtx, sharedStorageService, "9200", "http")
	if got, want := env["EXTERNAL_ELASTICSEARCH_HOST"], "elasticsearch.matrix-810-mns-mgmt.svc.cluster.local"; got != want {
		t.Errorf("EXTERNAL_ELASTICSEARCH_HOST = %q, want %q", got, want)
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

	opts := synthesizeReleaseOpts(base, "gke", "matrix-810-mns-mgmt")

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

	got := synthesizeReleaseOpts(base, "eks", "matrix-810-mns-mgmt")

	// The two fields synthesizeReleaseOpts is explicitly allowed to change.
	if got.Platform != "eks" {
		t.Errorf("Platform = %q, want %q (overridden per-release)", got.Platform, "eks")
	}
	if got.NamespaceOverride != "matrix-810-mns-mgmt" {
		t.Errorf("NamespaceOverride = %q, want %q (overridden per-release)", got.NamespaceOverride, "matrix-810-mns-mgmt")
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

func TestTopologyDeployOrder_ManagementFirst(t *testing.T) {
	order, err := topologyDeployOrder(testTopologyReleases())
	if err != nil {
		t.Fatalf("topologyDeployOrder() unexpected error: %v", err)
	}
	if len(order) != 3 || order[0] != 0 {
		t.Fatalf("order = %v, want management (index 0) first over 3 releases", order)
	}
	// every orchestration (depends-on management) must come after index 0
	for _, idx := range order[1:] {
		if idx == 0 {
			t.Fatalf("management index appeared after position 0: %v", order)
		}
	}
}

func TestTopologyDeployOrder_ChainedDependency(t *testing.T) {
	releases := []matrix.TopologyRelease{
		{Role: "orchestration", NamespaceSuffix: "orchb", DependsOn: "management"},
		{Role: "management", NamespaceSuffix: "mgmt"},
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
		t.Fatalf("order %v does not satisfy management(1) < orchestration(0) < aux(2)", order)
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
