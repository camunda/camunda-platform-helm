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
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"scripts/deploy-camunda/config"
)

func TestSynthesizeReleaseEntry_HubCarriesOwnLayers(t *testing.T) {
	baseEntry := Entry{
		Version:   "8.10",
		ChartPath: "charts/camunda-platform-8.10",
		Scenario:  "multinamespace",
		Shortname: "mns",
		Auth:      "keycloak",
	}
	releases := testTopologyReleases()

	hubEntry := synthesizeReleaseEntry("/repo", baseEntry, releases[0], "gke")

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
	baseEntry := Entry{
		Version:   "8.10",
		ChartPath: "charts/camunda-platform-8.10",
		Scenario:  "multinamespace",
		Shortname: "mns",
		Auth:      "keycloak",
	}

	for _, rel := range testTopologyReleases() {
		entry := synthesizeReleaseEntry("/repo", baseEntry, rel, "gke")
		if !entry.SkipE2E {
			t.Errorf("role %q: SkipE2E = false, want true (e2e is a topology-level phase)", rel.Role)
		}
	}
}

func TestSynthesizeReleaseEntry_OrchestrationHasNoDependenciesOrPostDeployHook(t *testing.T) {
	postDeployHook := &LifecycleHook{Script: "post-deploy-hub-ping.sh"}
	postInfraHook := &LifecycleHook{Script: "post-infra-legacy-elasticsearch.sh"}
	baseEntry := Entry{Version: "8.10", ChartPath: "charts/camunda-platform-8.10", Scenario: "multinamespace", Shortname: "mns", Auth: "keycloak", PostDeploy: postDeployHook, PostInfra: postInfraHook}
	releases := testTopologyReleases()

	for _, rel := range releases[1:] {
		orchEntry := synthesizeReleaseEntry("/repo", baseEntry, rel, "gke")
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
		if orchEntry.PostInfra != nil {
			t.Errorf("orchestration release %q PostInfra = %v, want nil", rel.NamespaceSuffix, orchEntry.PostInfra)
		}
	}
	hubEntry := synthesizeReleaseEntry("/repo", baseEntry, releases[0], "gke")
	if hubEntry.PostDeploy != nil {
		t.Errorf("Hub PostDeploy = %v, want nil so the hook runs once at topology level", hubEntry.PostDeploy)
	}
	// The post-infra hook provisions the shared infrastructure the rest of the
	// topology deploys onto, so it rides on the Hub release rather than being
	// dropped (which left topology scenarios with no post-infra step at all).
	if hubEntry.PostInfra != postInfraHook {
		t.Errorf("Hub PostInfra = %v, want scenario hook", hubEntry.PostInfra)
	}
}

func TestSynthesizeReleaseEntry_UsesReleaseChartVersion(t *testing.T) {
	baseEntry := Entry{Version: "8.10", ChartPath: "/elsewhere/camunda-platform-8.10"}
	rel := TopologyRelease{ChartVersion: "8.9"}

	got := synthesizeReleaseEntry("/repo", baseEntry, rel, "gke")
	if got.Version != "8.9" {
		t.Errorf("Version = %q, want 8.9", got.Version)
	}
	if got.ChartPath != filepath.Join("/repo", "charts", "camunda-platform-8.9") {
		t.Errorf("ChartPath = %q", got.ChartPath)
	}

	inherited := synthesizeReleaseEntry("/repo", baseEntry, TopologyRelease{}, "gke")
	if inherited.Version != "8.10" || inherited.ChartPath != filepath.Join("/repo", "charts", "camunda-platform-8.10") {
		t.Errorf("inherited chart = %s at %s", inherited.Version, inherited.ChartPath)
	}
}

func TestSynthesizeReleaseOpts_NamespaceOverridePinsRelease(t *testing.T) {
	base := RunOptions{
		RepoRoot: "/repo",
		Infra:    config.InfraOverride{InfraConfig: config.InfraConfig{KubeContext: "kube-ctx", IngressBaseDomain: "ci.example.com"}},
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
	base := RunOptions{RepoRoot: "/repo"}
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
// the ingress base domain for "gke" fell through to the (also empty)
// generic field, ResolveIngressHostname() returned "", and CAMUNDA_HOSTNAME
// was never derived - failing the Hub release's preflight with
// "CAMUNDA_HOSTNAME unset".
func TestSynthesizeReleaseOpts_PropagatesPerPlatformIngressBaseDomains(t *testing.T) {
	base := RunOptions{
		RepoRoot: "/repo",
		// Only the per-platform map is set — mirrors a user who passed
		// --ingress-base-domain-gke and never set the generic
		// --ingress-base-domain.
		Infra: config.InfraOverride{InfraMaps: config.InfraMaps{IngressBaseDomains: map[string]string{"gke": "ci.distro.ultrawombat.com"}}},
	}

	opts := synthesizeReleaseOpts(base, "gke", "matrix-810-mns-hub")

	if got := opts.Infra.IngressBaseDomains["gke"]; got != "ci.distro.ultrawombat.com" {
		t.Fatalf("IngressBaseDomains[\"gke\"] = %q, want %q — per-platform map was dropped", got, "ci.distro.ultrawombat.com")
	}

	// More directly: build the actual per-release flags the way
	// runTopologyEntry does, and confirm the ingress hostname (and hence
	// CAMUNDA_HOSTNAME) is non-empty.
	releaseEntry := Entry{
		Version:   "8.10",
		ChartPath: "charts/camunda-platform-8.10",
		Scenario:  "multinamespace",
		Shortname: "mns",
		Auth:      "keycloak",
		Flow:      "install",
		Platform:  "gke",
	}
	flags, _, _, _, err := BuildEntryFlags(releaseEntry, opts)
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
	base := RunOptions{
		RepoRoot: "/repo",
		Infra:    config.InfraOverride{InfraConfig: config.InfraConfig{IngressBaseDomain: "ci.distro.ultrawombat.com"}},
	}

	opts := synthesizeReleaseOpts(base, "gke", "matrix-810-mns-hub")

	if opts.Infra.IngressBaseDomain != "ci.distro.ultrawombat.com" {
		t.Errorf("IngressBaseDomain = %q, want %q", opts.Infra.IngressBaseDomain, "ci.distro.ultrawombat.com")
	}
}

// TestApplyTopologyReleaseOverrides_ForcesExternalSecrets pins down the fix
// for the live-GKE bug where the Hub release's bundled Keycloak
// CreateContainerConfigError'd on a missing "integration-test-credentials"
// secret. BuildEntryFlags sets Secrets.ExternalSecrets = (NamespaceOverride
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

// TestSynthesizeReleaseOpts_PropagatesHelmTimeout pins down that --timeout
// (helm deployment timeout in minutes) is forwarded to every topology
// release's RunOptions — same class of bug as the ingress-base-domain
// plumbing fix: synthesizeReleaseOpts must carry every field RunOptions
// needs, or the topology path silently falls back to defaults regardless of
// what the user passed. See TestSynthesizeReleaseOpts_ForwardsEntireBaseRunOptions
// below for the general-case regression guard against this whole bug class.
func TestSynthesizeReleaseOpts_PropagatesHelmTimeout(t *testing.T) {
	base := RunOptions{
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
// synthesizeReleaseOpts now copies the ENTIRE base RunOptions and
// overrides only Platform and NamespaceOverride — so this test constructs a
// base with many fields set to distinctive non-zero values and asserts every
// one of them survives into the per-release RunOptions unchanged, with only
// Platform/NamespaceOverride differing.
func TestSynthesizeReleaseOpts_ForwardsEntireBaseRunOptions(t *testing.T) {
	docker := config.DockerFlags{
		DockerUsername:       "docker-user",
		DockerPassword:       "docker-pass",
		EnsureDockerRegistry: true,
		DockerHubUsername:    "dockerhub-user",
		DockerHubPassword:    "dockerhub-pass",
		EnsureDockerHub:      true,
	}
	base := RunOptions{
		DryRun:               true,
		Coverage:             true,
		StopOnFailure:        true,
		Cleanup:              true,
		DeleteNamespaceFirst: true,
		Config:               &config.RootConfig{Current: "dev"},
		Infra: config.InfraOverride{
			InfraConfig: config.InfraConfig{KubeContext: "fallback-ctx", EnvFile: ".env", IngressBaseDomain: "ci.distro.ultrawombat.com"},
			InfraMaps: config.InfraMaps{
				KubeContexts:       map[string]string{"gke": "gke-ctx"},
				EnvFiles:           map[string]string{"8.10": ".env.810"},
				IngressBaseDomains: map[string]string{"gke": "ci.distro.ultrawombat.com"},
				VaultBackedSecrets: map[string]bool{"eks": true},
			},
		},
		NamespacePrefix:      "matrix",
		Platform:             "gke",
		MaxParallel:          3,
		TestE2E:              true,
		TestAll:              true,
		RepoRoot:             "/repo",
		LogLevel:             "debug",
		SkipDependencyUpdate: true,
		KeycloakHost:         "keycloak.example.com",
		KeycloakProtocol:     "https",
		UpgradeFromVersion:   "8.9",
		HelmTimeout:          25,
		DockerFlags:          docker,
		UseLatest:            true,
		UseQA:                true,
		ForceImageOverrides:  true,
		ExtraHelmArgs:        []string{"--set-file=foo=bar"},
		ExtraHelmSets:        []string{"a=b"},
		ExtraValues:          []string{"/tmp/extra.yaml"},
		NamespaceOverride:    "should-be-overridden",
		ChartRef:             "oci://example/camunda-platform",
		ChartRefVersion:      "13-rc-latest",
		LogDir:               "/tmp/matrix-logs",
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

// The topology driver hands each release the rendered credentials-manifest
// through flags.Secrets.CredentialsManifest; without it the deployer falls back
// to the CI-wide integration-test credentials.
func TestApplyTopologyCredentialsManifest(t *testing.T) {
	repoRoot := t.TempDir()
	rel := filepath.Join("charts", "creds.yaml")
	if err := os.MkdirAll(filepath.Join(repoRoot, "charts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, rel), []byte("key: ${TOPOLOGY_BASE}-credentials\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Run("declared manifest is rendered and wired into the deploy flags", func(t *testing.T) {
		flags := &config.RuntimeFlags{}
		remove, err := applyTopologyCredentialsManifest(flags, repoRoot, rel, "dogfood")
		if err != nil {
			t.Fatal(err)
		}
		path := flags.Secrets.CredentialsManifest
		if path == "" {
			t.Fatal("flags.Secrets.CredentialsManifest is empty; the deploy would apply the CI-wide credentials")
		}
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != "key: dogfood-credentials\n" {
			t.Errorf("rendered manifest = %q, want the base namespace substituted", got)
		}
		remove()
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("rendered manifest %s not removed: %v", path, err)
		}
	})

	t.Run("no manifest leaves the flags unchanged", func(t *testing.T) {
		flags := &config.RuntimeFlags{}
		flags.Secrets.CredentialsManifest = "preset.yaml"
		remove, err := applyTopologyCredentialsManifest(flags, repoRoot, "", "dogfood")
		if err != nil {
			t.Fatal(err)
		}
		remove()
		if flags.Secrets.CredentialsManifest != "preset.yaml" {
			t.Errorf("CredentialsManifest = %q, want unchanged", flags.Secrets.CredentialsManifest)
		}
	})

	t.Run("missing manifest is an error", func(t *testing.T) {
		flags := &config.RuntimeFlags{}
		if _, err := applyTopologyCredentialsManifest(flags, repoRoot, "charts/absent.yaml", "dogfood"); err == nil {
			t.Fatal("expected an error for a missing manifest")
		}
		if flags.Secrets.CredentialsManifest != "" {
			t.Errorf("CredentialsManifest = %q, want empty after a failure", flags.Secrets.CredentialsManifest)
		}
	})
}
