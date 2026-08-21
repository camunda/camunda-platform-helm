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
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Topology describes a multi-namespace deployment shape for a scenario: one
// "hub" release (Identity, Console, Web Modeler, bundled Keycloak)
// plus one or more "orchestration" releases (Zeebe/Operate/Tasklist,
// Connectors, Optimize) that share a single logical cluster via a central
// Identity and a shared secondary storage backend, and optionally one or more
// "optimize" releases that each run only Optimize against an orchestration
// release's exported records.
//
// Scenarios without a Topology behave byte-for-byte as today — this field is
// additive and opt-in (see registryScenario.Topology / CIScenario.Topology).
type Topology struct {
	// Name is a human-readable label for the topology shape, e.g. "hub-2orch".
	Name string `yaml:"name" json:"name"`

	// Releases lists every namespace/release this scenario fans out to.
	Releases []TopologyRelease `yaml:"releases" json:"releases"`

	// SharedStorage names the companion dependency (e.g. "elasticsearch")
	// deployed once (into the Hub namespace) and referenced by every
	// orchestration release via FQDN, rather than deployed per-release.
	SharedStorage string `yaml:"shared-storage,omitempty" json:"sharedStorage,omitempty"`

	// SharedStorageService is the Kubernetes Service name of the shared storage backend (defaults to SharedStorage/release name; elastic chart uses <clusterName>-master).
	SharedStorageService string `yaml:"shared-storage-service,omitempty" json:"sharedStorageService,omitempty"`
}

// TopologyRelease is one namespace/release within a Topology. Each release
// can select its own identity/persistence/features/dependencies layers —
// e.g. the Hub release uses the bundled-Keycloak identity layer and
// deploys keycloak/postgresql/elasticsearch, while orchestration releases
// use an external-Keycloak layer pointed back at Hub and deploy no
// companions of their own (they consume the Hub release's shared
// Elasticsearch and Identity/Keycloak cross-namespace by FQDN).
type TopologyRelease struct {
	// Role is "hub", "orchestration", or "optimize". Exactly one
	// "hub" role must be declared per Topology.
	Role string `yaml:"role" json:"role"`

	// NamespaceSuffix is appended to the base namespace to form this
	// release's namespace (<base>-<namespace-suffix>). Must be unique
	// within the Topology.
	NamespaceSuffix string `yaml:"namespace-suffix" json:"namespaceSuffix"`

	// Values is rejected by Validate. A release's own overlay is a Feature
	// layer: list it in Features instead.
	Values string `yaml:"values,omitempty" json:"-"`

	// DependsOn, when set, names the Role of a release that must be deployed
	// (and, for "hub", ready) before this one.
	DependsOn string `yaml:"depends-on,omitempty" json:"dependsOn,omitempty"`

	// Identity, when set, overrides the scenario-level Identity layer for
	// this release only (e.g. "keycloak" for Hub vs
	// "keycloak-external" for orchestration releases that point back at the
	// Hub namespace's Keycloak/Identity instead of deploying their
	// own).
	Identity string `yaml:"identity,omitempty" json:"identity,omitempty"`

	// Persistence, when set, overrides the scenario-level Persistence layer
	// for this release only.
	Persistence string `yaml:"persistence,omitempty" json:"persistence,omitempty"`

	// Features, when set, overrides the scenario-level Features layers for
	// this release only.
	Features []string `yaml:"features,omitempty" json:"features,omitempty"`

	// Dependencies lists companion dependency IDs (basenames under
	// registry/dependencies/, resolved the same way a scenario's top-level
	// dependencies are) to deploy alongside THIS release only. Empty means
	// this release deploys no companions of its own (e.g. an orchestration
	// release that consumes the Hub release's shared Elasticsearch
	// cross-namespace instead of deploying its own copy).
	Dependencies []string `yaml:"dependencies,omitempty" json:"dependencies,omitempty"`

	// Env contains release-local values-layer substitutions. The topology
	// driver merges these after shared cross-release variables, so each
	// orchestration release can use distinct auth identifiers.
	Env map[string]string `yaml:"env,omitempty" json:"env,omitempty"`

	ModelerClusterID   string `yaml:"modeler-cluster-id,omitempty" json:"modelerClusterId,omitempty"`
	ModelerClusterName string `yaml:"modeler-cluster-name,omitempty" json:"modelerClusterName,omitempty"`

	// Serves names the NamespaceSuffix of the orchestration release whose
	// exported records this release reads. Required for Role == "optimize",
	// rejected for any other role. It is what lets the topology smoke matrix
	// map an orchestration leg to the Optimize instance that serves it, since
	// an optimize release runs in its own namespace on the Hub host.
	Serves string `yaml:"serves,omitempty" json:"serves,omitempty"`

	// OptimizeContextPath is the ingress path this release's Optimize is served
	// on, matching optimize.contextPath in its values layer. Required for
	// Role == "optimize", rejected for any other role.
	OptimizeContextPath string `yaml:"optimize-context-path,omitempty" json:"optimizeContextPath,omitempty"`

	// ResolvedDependencies holds the fully-resolved companion chart specs
	// for Dependencies, populated by LoadRegistry (mirroring how
	// registryScenario.DependencyIDs resolves into CIScenario.Dependencies).
	// Not part of the YAML/JSON wire format — it's a load-time cache
	// consumed by the topology deploy driver.
	ResolvedDependencies []ChartDependency `yaml:"-" json:"-"`
}

// reservedTopologyEnvKeys are the substitution variables the topology driver
// derives from a release's declaration (see buildTopologyReleaseEnv): the
// orchestration leg a release is, and the context path and served orchestration
// an optimize release reads. A release env entry may not name one. The driver
// applies the derived value last so a stray entry cannot win, and this check
// makes the attempt an error instead of a silently dropped key, because such an
// entry means its author expected it to take effect.
var reservedTopologyEnvKeys = []string{
	"ORCH_NAMESPACE",
	"ORCH_HOST",
	"ORCH_ZEEBE_GRPC",
	"ORCH_ZEEBE_REST",
	"RELEASE_OPTIMIZE_CONTEXT_PATH",
	"SERVED_NAMESPACE",
	"SERVED_HOST",
	"SERVED_ORCHESTRATION_INDEX_PREFIX",
}

// Validate enforces Topology's load-time invariants:
//   - at least one release is declared;
//   - no release sets Values, which Features replaces;
//   - every release declares at least one Features layer, and every one of them
//     resolves on disk under <chartFullSetupDir>/values/features/<id>.yaml;
//   - every release's Identity/Persistence layer (when set) resolves on disk
//     under <chartFullSetupDir>/values/identity/ or .../persistence/;
//   - every release's Dependencies IDs (when set) resolve to a file under
//     <depsDir>/<id>.yaml;
//   - every release's DependsOn (when set) references a declared Role, and is
//     exactly "hub" for every Role == "optimize" release;
//   - exactly one release has Role == "hub";
//   - NamespaceSuffix values are unique and non-empty;
//   - no release Env entry names a variable the topology driver derives
//     (reservedTopologyEnvKeys).
//
// ctx is prepended to error messages, e.g. `scenario "multinamespace": topology: ...`.
func (t *Topology) Validate(ctx string, chartFullSetupDir string, depsDir string) error {
	if t == nil {
		return nil
	}
	var problems []string

	if len(t.Releases) == 0 {
		problems = append(problems, fmt.Sprintf("%s: topology %q: at least one release is required", ctx, t.Name))
	}

	roles := map[string]bool{}
	suffixes := map[string]bool{}
	hubCount := 0
	orchestrationCount := 0
	modelerClusterIDs := map[string]bool{}
	modelerClusterNames := map[string]bool{}

	for i, r := range t.Releases {
		label := fmt.Sprintf("%s: topology %q: release[%d] (role %q, namespace-suffix %q)", ctx, t.Name, i, r.Role, r.NamespaceSuffix)

		switch r.Role {
		case "hub":
			hubCount++
		case "orchestration":
			orchestrationCount++
			if strings.TrimSpace(r.ModelerClusterID) == "" {
				problems = append(problems, fmt.Sprintf("%s: modeler-cluster-id is required", label))
			} else if modelerClusterIDs[r.ModelerClusterID] {
				problems = append(problems, fmt.Sprintf("%s: duplicate modeler-cluster-id %q", label, r.ModelerClusterID))
			} else {
				modelerClusterIDs[r.ModelerClusterID] = true
			}
			if strings.TrimSpace(r.ModelerClusterName) == "" {
				problems = append(problems, fmt.Sprintf("%s: modeler-cluster-name is required", label))
			} else if modelerClusterNames[r.ModelerClusterName] {
				problems = append(problems, fmt.Sprintf("%s: duplicate modeler-cluster-name %q", label, r.ModelerClusterName))
			} else {
				modelerClusterNames[r.ModelerClusterName] = true
			}
		case "optimize":
			if strings.TrimSpace(r.DependsOn) != "hub" {
				problems = append(problems, fmt.Sprintf("%s: depends-on must be \"hub\" so the release deploys after the Management Identity that provisions its client, got %q", label, r.DependsOn))
			}
			if strings.TrimSpace(r.Serves) == "" {
				problems = append(problems, fmt.Sprintf("%s: serves is required and must name the namespace-suffix of the orchestration release this Optimize reads", label))
			}
			if strings.TrimSpace(r.OptimizeContextPath) == "" {
				problems = append(problems, fmt.Sprintf("%s: optimize-context-path is required and must match optimize.contextPath in the release's values layer", label))
			} else if !strings.HasPrefix(r.OptimizeContextPath, "/") {
				problems = append(problems, fmt.Sprintf("%s: optimize-context-path %q must start with \"/\"", label, r.OptimizeContextPath))
			}
		default:
			problems = append(problems, fmt.Sprintf("%s: role must be \"hub\", \"orchestration\", or \"optimize\", got %q", label, r.Role))
		}
		roles[r.Role] = true

		if strings.TrimSpace(r.NamespaceSuffix) == "" {
			problems = append(problems, fmt.Sprintf("%s: namespace-suffix is required", label))
		} else if !isDNS1123Label(r.NamespaceSuffix) {
			problems = append(problems, fmt.Sprintf("%s: namespace-suffix %q must be a lowercase DNS-1123 label", label, r.NamespaceSuffix))
		} else if suffixes[r.NamespaceSuffix] {
			problems = append(problems, fmt.Sprintf("%s: duplicate namespace-suffix %q", label, r.NamespaceSuffix))
		} else {
			suffixes[r.NamespaceSuffix] = true
		}
		if len(r.NamespaceSuffix) > 12 {
			problems = append(problems, fmt.Sprintf("%s: namespace-suffix %q is too long (max 12 chars, to keep <namespace>-<suffix> well within the 63-char Kubernetes limit)", label, r.NamespaceSuffix))
		}

		if strings.TrimSpace(r.Values) != "" {
			problems = append(problems, fmt.Sprintf("%s: values %q is no longer supported: drop the \"features/\" prefix and the \".yaml\" suffix and list it in features instead, so the layer goes through the same env-var substitution as every other feature layer", label, r.Values))
		}
		if len(r.Features) == 0 {
			problems = append(problems, fmt.Sprintf("%s: features is required and must name at least this release's own overlay layer", label))
		}
		for _, featureID := range r.Features {
			if !isPlainFilename(featureID) {
				problems = append(problems, fmt.Sprintf("%s: feature reference %q must be a plain filename (no path separators)", label, featureID))
				continue
			}
			featurePath := filepath.Join(chartFullSetupDir, "values", "features", featureID+".yaml")
			if info, err := os.Stat(featurePath); err != nil || info.IsDir() {
				problems = append(problems, fmt.Sprintf("%s: feature %q: missing values file at %s", label, featureID, featurePath))
			}
		}

		if r.Identity != "" {
			identityPath := filepath.Join(chartFullSetupDir, "values", "identity", r.Identity+".yaml")
			if info, err := os.Stat(identityPath); err != nil || info.IsDir() {
				problems = append(problems, fmt.Sprintf("%s: identity %q: missing values file at %s", label, r.Identity, identityPath))
			}
		}

		if r.Persistence != "" {
			persistencePath := filepath.Join(chartFullSetupDir, "values", "persistence", r.Persistence+".yaml")
			if info, err := os.Stat(persistencePath); err != nil || info.IsDir() {
				problems = append(problems, fmt.Sprintf("%s: persistence %q: missing values file at %s", label, r.Persistence, persistencePath))
			}
		}

		for _, depID := range r.Dependencies {
			if !isPlainFilename(depID) {
				problems = append(problems, fmt.Sprintf("%s: dependency reference %q must be a plain filename (no path separators)", label, depID))
				continue
			}
			depPath := filepath.Join(depsDir, depID+".yaml")
			if info, err := os.Stat(depPath); err != nil || info.IsDir() {
				problems = append(problems, fmt.Sprintf("%s: dependency %q: missing at %s", label, depID, depPath))
			}
		}
	}

	if hubCount != 1 {
		problems = append(problems, fmt.Sprintf("%s: topology %q: exactly one release with role \"hub\" is required, found %d", ctx, t.Name, hubCount))
	}
	if orchestrationCount == 0 {
		problems = append(problems, fmt.Sprintf("%s: topology %q: at least one release with role \"orchestration\" is required", ctx, t.Name))
	}

	orchestrationSuffixes := map[string]bool{}
	for _, r := range t.Releases {
		if r.Role == "orchestration" {
			orchestrationSuffixes[r.NamespaceSuffix] = true
		}
	}

	optimizeContextPaths := map[string]string{}
	for _, r := range t.Releases {
		if r.Role != "optimize" || r.OptimizeContextPath == "" {
			continue
		}
		if owner, seen := optimizeContextPaths[r.OptimizeContextPath]; seen {
			problems = append(problems, fmt.Sprintf("%s: topology %q: optimize releases %q and %q share optimize-context-path %q; they are served on one host so their ingress paths would collide", ctx, t.Name, owner, r.NamespaceSuffix, r.OptimizeContextPath))
			continue
		}
		optimizeContextPaths[r.OptimizeContextPath] = r.NamespaceSuffix
	}

	for i, r := range t.Releases {
		if r.Role != "optimize" {
			if r.Serves != "" {
				problems = append(problems, fmt.Sprintf("%s: topology %q: release[%d] (role %q) must not set serves; it applies only to role \"optimize\"", ctx, t.Name, i, r.Role))
			}
			if r.OptimizeContextPath != "" {
				problems = append(problems, fmt.Sprintf("%s: topology %q: release[%d] (role %q) must not set optimize-context-path; it applies only to role \"optimize\"", ctx, t.Name, i, r.Role))
			}
		} else if r.Serves != "" && !orchestrationSuffixes[r.Serves] {
			problems = append(problems, fmt.Sprintf("%s: topology %q: release[%d] serves %q does not reference a declared orchestration release's namespace-suffix", ctx, t.Name, i, r.Serves))
		}

		for _, key := range reservedTopologyEnvKeys {
			if _, taken := r.Env[key]; taken {
				problems = append(problems, fmt.Sprintf("%s: topology %q: release[%d] env sets %s, which the topology driver derives from this release's declaration; the derived value wins, so the entry would never take effect - remove it and change the declaration instead", ctx, t.Name, i, key))
			}
		}

		if r.DependsOn == "" {
			continue
		}
		if !roles[r.DependsOn] {
			problems = append(problems, fmt.Sprintf("%s: topology %q: release[%d] depends-on %q does not reference a declared role", ctx, t.Name, i, r.DependsOn))
		}
	}

	for _, r := range t.Releases {
		if r.Role != "optimize" {
			continue
		}
		label := fmt.Sprintf("%s: topology %q: release (role %q, namespace-suffix %q)", ctx, t.Name, r.Role, r.NamespaceSuffix)
		problems = append(problems, validateOptimizeLayerValues(label, r, chartFullSetupDir)...)
	}

	if len(problems) == 0 {
		return nil
	}
	return fmt.Errorf("%s", strings.Join(problems, "\n  - "))
}

// optimizeLayerValues is the subset of a values layer that must agree with an
// optimize release's topology declaration.
type optimizeLayerValues struct {
	Optimize struct {
		ContextPath *string `yaml:"contextPath"`
		Database    struct {
			Elasticsearch struct {
				Prefix *string `yaml:"prefix"`
			} `yaml:"elasticsearch"`
			Opensearch struct {
				Prefix *string `yaml:"prefix"`
			} `yaml:"opensearch"`
		} `yaml:"database"`
	} `yaml:"optimize"`
}

// placeholderForms returns the two substitution spellings os.Expand accepts for
// name, which is what a values layer must contain verbatim for the topology
// declaration to remain the value's only copy.
func placeholderForms(name string) []string {
	return []string{"${" + name + "}", "$" + name}
}

// isExactPlaceholder reports whether value is nothing but a substitution of
// name. A value that merely contains the name, or any other dollar sign, is not
// accepted: "${STALE_PATH}" follows no declaration at all.
func isExactPlaceholder(value, name string) bool {
	for _, form := range placeholderForms(name) {
		if value == form {
			return true
		}
	}
	return false
}

// placeholderLeads reports whether value is a substitution of name, optionally
// carried further by a suffix. A Physical Tenant's Optimize reads the tenant's
// own slice of the served orchestration's records, so its index prefix is the
// published one plus a tenant suffix. The placeholder still has to lead, which
// is what makes repointing serves repoint the records: "job-orcha-ta" and
// "wrong-${SERVED_ORCHESTRATION_INDEX_PREFIX}" both fail.
//
// The bare "$NAME" form leads only when the suffix starts with a character that
// ends a shell name, because os.Expand consumes [A-Za-z0-9_] greedily:
// "$SERVED_ORCHESTRATION_INDEX_PREFIX-ta" substitutes the published variable,
// while "$SERVED_ORCHESTRATION_INDEX_PREFIXta" substitutes a variable nothing
// publishes and expands to the empty string.
func placeholderLeads(value, name string) bool {
	if isExactPlaceholder(value, name) {
		return true
	}
	if strings.HasPrefix(value, "${"+name+"}") {
		return true
	}
	if rest, ok := strings.CutPrefix(value, "$"+name); ok && rest != "" {
		return !isShellNameByte(rest[0])
	}
	return false
}

// isShellNameByte reports whether b can continue an unbraced shell variable
// name, which is the rule os.Expand applies when it decides where "$NAME" ends.
func isShellNameByte(b byte) bool {
	return b == '_' ||
		(b >= '0' && b <= '9') ||
		(b >= 'a' && b <= 'z') ||
		(b >= 'A' && b <= 'Z')
}

// validateOptimizeLayerValues rejects a values layer that writes its own copy of
// what the topology declaration already states, and a release whose layers never
// state it at all. The topology driver publishes optimize-context-path as
// RELEASE_OPTIMIZE_CONTEXT_PATH and the serves release's index prefix as
// SERVED_ORCHESTRATION_INDEX_PREFIX, so a layer that hardcodes either keeps
// deploying the old value after the declaration changes, while the smoke matrix
// reports the new one. A layer that sets neither is just as wrong in the other
// direction: the release silently inherits whatever a base layer left there,
// which is some other release's context path or index prefix.
func validateOptimizeLayerValues(label string, r TopologyRelease, chartFullSetupDir string) []string {
	var problems []string
	readAny := false
	contextPathSet := false
	prefixSet := false
	for _, featureID := range r.Features {
		if !isPlainFilename(featureID) {
			continue
		}
		path := filepath.Join(chartFullSetupDir, "values", "features", featureID+".yaml")
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var layer optimizeLayerValues
		if yaml.Unmarshal(content, &layer) != nil {
			continue
		}
		readAny = true

		if cp := layer.Optimize.ContextPath; cp != nil && *cp != "" {
			contextPathSet = true
			if !isExactPlaceholder(*cp, "RELEASE_OPTIMIZE_CONTEXT_PATH") && *cp != r.OptimizeContextPath {
				problems = append(problems, fmt.Sprintf("%s: feature %q sets optimize.contextPath %q but the release declares optimize-context-path %q; set it to exactly %q so the declaration is the only copy", label, featureID, *cp, r.OptimizeContextPath, placeholderForms("RELEASE_OPTIMIZE_CONTEXT_PATH")[0]))
			}
		}

		for _, backend := range []string{"elasticsearch", "opensearch"} {
			prefix := layer.Optimize.Database.Elasticsearch.Prefix
			if backend == "opensearch" {
				prefix = layer.Optimize.Database.Opensearch.Prefix
			}
			if prefix == nil || *prefix == "" {
				continue
			}
			prefixSet = true
			if !placeholderLeads(*prefix, "SERVED_ORCHESTRATION_INDEX_PREFIX") {
				problems = append(problems, fmt.Sprintf("%s: feature %q sets optimize.database.%s.prefix to %q, which does not follow serves %q; it must be %q, or lead with it and carry a per-tenant suffix (the unbraced $NAME form leads only when the suffix starts with a character that ends a shell name, such as a hyphen), so repointing serves repoints the records this Optimize reads", label, featureID, backend, *prefix, r.Serves, placeholderForms("SERVED_ORCHESTRATION_INDEX_PREFIX")[0]))
			}
		}
	}

	// Presence is asserted only over layers this validator could read and parse:
	// a release whose feature layers are absent from disk (or unparseable) is
	// reported by the layer-resolution checks in Validate, not silently blamed
	// for a missing key here.
	if !readAny {
		return problems
	}
	if r.OptimizeContextPath != "" && !contextPathSet {
		problems = append(problems, fmt.Sprintf("%s: no feature layer sets optimize.contextPath, so this release is served on whatever path its base layers left behind rather than on the declared optimize-context-path %q; set optimize.contextPath to %q in one of its feature layers", label, r.OptimizeContextPath, placeholderForms("RELEASE_OPTIMIZE_CONTEXT_PATH")[0]))
	}
	if r.Serves != "" && !prefixSet {
		problems = append(problems, fmt.Sprintf("%s: no feature layer sets optimize.database.elasticsearch.prefix or optimize.database.opensearch.prefix, so this release reads whatever index prefix its base layers left behind rather than the records serves %q writes; set the prefix of the enabled backend to %q in one of its feature layers", label, r.Serves, placeholderForms("SERVED_ORCHESTRATION_INDEX_PREFIX")[0]))
	}
	return problems
}

func isDNS1123Label(value string) bool {
	if value == "" || len(value) > 63 {
		return false
	}
	for i, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || (r == '-' && i > 0 && i < len(value)-1) {
			continue
		}
		return false
	}
	return true
}
