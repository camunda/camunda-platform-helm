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
	"os"
	"path/filepath"
	"scripts/deploy-camunda/config"
)

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

// synthesizeReleaseEntry builds the Entry for one topology release,
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

// synthesizeReleaseOpts returns a copy of base (the same fully-built
// RunOptions passed into runTopologyEntry) with ONLY Platform and
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
// every release's flags need on top of BuildEntryFlags' normal
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
