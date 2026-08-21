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
//     (reservedTopologyEnvKeys);
//   - every Role == "optimize" release's values layers follow its declaration
//     rather than restating it, and state the index prefix of the database
//     backend those layers actually enable;
//   - every Role == "optimize" release presents the same OIDC client id,
//     audience, redirect URL and secret the Hub release's inventory registers for
//     the cluster it serves, and is advertised there on its declared context path.
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

	hubInventory := hubOptimizeInventory(t, chartFullSetupDir)
	for _, r := range t.Releases {
		if r.Role != "optimize" {
			continue
		}
		label := fmt.Sprintf("%s: topology %q: release (role %q, namespace-suffix %q)", ctx, t.Name, r.Role, r.NamespaceSuffix)
		view := buildOptimizeReleaseView(r, chartFullSetupDir)
		problems = append(problems, validateOptimizeLayerValues(label, r, chartFullSetupDir, view.switches.choice())...)
		problems = append(problems, validateOptimizeIdentityAgainstHub(label, t, r, view.oidc, hubInventory)...)
	}

	if len(problems) == 0 {
		return nil
	}
	return fmt.Errorf("%s", strings.Join(problems, "\n  - "))
}

// optimizeLayerValues is the subset of a values layer that must agree with an
// optimize release's topology declaration, plus the two things that decide
// whether the agreement has any effect: which database backend is enabled, and
// which OIDC identity the release presents.
type optimizeLayerValues struct {
	Global struct {
		Elasticsearch struct {
			Enabled *bool `yaml:"enabled"`
		} `yaml:"elasticsearch"`
		Opensearch struct {
			Enabled *bool `yaml:"enabled"`
		} `yaml:"opensearch"`
	} `yaml:"global"`
	Optimize struct {
		ContextPath *string `yaml:"contextPath"`
		Database    struct {
			Elasticsearch optimizeBackendLayer `yaml:"elasticsearch"`
			Opensearch    optimizeBackendLayer `yaml:"opensearch"`
		} `yaml:"database"`
		Security struct {
			Authentication struct {
				OIDC optimizeOIDCLayer `yaml:"oidc"`
			} `yaml:"authentication"`
		} `yaml:"security"`
	} `yaml:"optimize"`
}

// optimizeBackendLayer is one database backend's slice of a values layer.
type optimizeBackendLayer struct {
	Enabled *bool   `yaml:"enabled"`
	Prefix  *string `yaml:"prefix"`
}

// optimizeOIDCLayer is the OIDC identity an Optimize release presents. Every
// field of it is also registered for the same cluster in the Hub release's
// inventory, which is why validateOptimizeIdentityAgainstHub cross-checks the
// two rather than trusting each side's copy.
type optimizeOIDCLayer struct {
	ClientID    *string           `yaml:"clientId"`
	Audience    *string           `yaml:"audience"`
	RedirectURL *string           `yaml:"redirectUrl"`
	Secret      optimizeSecretRef `yaml:"secret"`
}

// optimizeSecretRef is a client-secret reference, spelled the same way in an
// optimize release's values layer and in the Hub inventory.
type optimizeSecretRef struct {
	ExistingSecret    *string `yaml:"existingSecret"`
	ExistingSecretKey *string `yaml:"existingSecretKey"`
	InlineSecret      *string `yaml:"inlineSecret"`
}

// secretForm is a client-secret reference resolved to what the chart emits for
// it, which is what the two sides of a cross-check have to agree on: comparing
// the raw fields lets an inline literal on one side sit next to a Secret
// reference on the other, and never looks at two inline literals at all.
type secretForm struct {
	// kind is "" when nothing is emitted, "ref" for a Secret reference, or
	// "inline" for a literal.
	kind   string
	name   string
	key    string
	inline string
}

// normalizedSecretForm resolves a reference the way
// camundaPlatform.normalizeSecretConfiguration does: a complete existing-Secret
// reference wins, and inlineSecret applies only when there is no complete one.
// This is the precedence behind global.topology.clusters[].components.optimize,
// whose secret the Identity Deployment passes straight to that helper.
//
// An incomplete reference with no inline literal is still reported as "ref": the
// helper emits nothing for it, but it is the reference this side states, and
// comparing it names the half that drifted instead of only the half that is
// missing.
func normalizedSecretForm(ref optimizeSecretRef) secretForm {
	name := stringValue(ref.ExistingSecret)
	key := stringValue(ref.ExistingSecretKey)
	inline := stringValue(ref.InlineSecret)
	switch {
	case name != "" && key != "":
		return secretForm{kind: "ref", name: name, key: key}
	case inline != "":
		return secretForm{kind: "inline", inline: inline}
	case name != "" || key != "":
		return secretForm{kind: "ref", name: name, key: key}
	}
	return secretForm{}
}

// inlineFirstSecretForm resolves a reference the way the two templates that read
// inlineSecret before existingSecret do: identity.clients[] in the Identity
// Deployment, and optimize.security.authentication.oidc.secret through
// optimize.effectiveAuthSecret, which drops the inherited existingSecret entirely
// once the release states an inline value.
func inlineFirstSecretForm(ref optimizeSecretRef) secretForm {
	if inline := stringValue(ref.InlineSecret); inline != "" {
		return secretForm{kind: "inline", inline: inline}
	}
	name := stringValue(ref.ExistingSecret)
	key := stringValue(ref.ExistingSecretKey)
	if name == "" && key == "" {
		return secretForm{}
	}
	return secretForm{kind: "ref", name: name, key: key}
}

// secretFormProblems compares the client secret an optimize release sends against
// the one the Hub provisions for the same client. hubLocation is the values path
// the Hub side lives under, so the message names something the reader can look up.
//
// Inline literals are compared but never quoted into a problem: these messages
// reach CI logs.
func secretFormProblems(label, hubLocation string, hub, release secretForm, hubSubs, releaseSubs map[string]string) []string {
	releaseKey := "optimize.security.authentication.oidc.secret"
	if hub.kind == "" {
		return nil
	}
	if release.kind == "" {
		return []string{fmt.Sprintf("%s: the Hub provisions this client's secret from %s but no layer of this release sets %s, so Optimize sends the chart default instead of the secret Identity registered", label, hubLocation, releaseKey)}
	}
	if hub.kind != release.kind {
		return []string{fmt.Sprintf("%s: this release sends its client secret as %s but the Hub provisions the same client's secret as %s; the value Optimize sends and the value Identity registered have to be the same one, which they cannot be shown to be while the two sides state it in different forms", label, describeSecretForm(release.kind, releaseKey), describeSecretForm(hub.kind, hubLocation))}
	}
	if hub.kind == "inline" {
		if hub.inline == release.inline {
			return nil
		}
		return []string{fmt.Sprintf("%s: %s.inlineSecret and %s.inlineSecret are different literals, so Optimize authenticates with one client secret while Identity registered another; no value is shown here because both are secrets", label, releaseKey, hubLocation)}
	}
	var problems []string
	compare := func(what, field, hubValue, releaseValue string) {
		if hubValue == "" || releaseValue == "" || expandTopologyPlaceholders(hubValue, hubSubs) == expandTopologyPlaceholders(releaseValue, releaseSubs) {
			return
		}
		problems = append(problems, fmt.Sprintf("%s: %s.%s is %q but the Hub provisions this client's %s as %q (%s.%s); the value Optimize presents and the value Identity provisioned have to be the same one", label, releaseKey, field, releaseValue, what, hubValue, hubLocation, field))
	}
	compare("client secret", "existingSecret", hub.name, release.name)
	compare("client secret key", "existingSecretKey", hub.key, release.key)
	if hub.name != "" && release.name == "" {
		problems = append(problems, fmt.Sprintf("%s: the Hub provisions this client's secret from Secret %q (%s.existingSecret) but no layer of this release sets %s.existingSecret", label, hub.name, hubLocation, releaseKey))
	}
	if hub.key != "" && release.key == "" {
		problems = append(problems, fmt.Sprintf("%s: the Hub provisions this client's secret from key %q (%s.existingSecretKey) but no layer of this release sets %s.existingSecretKey", label, hub.key, hubLocation, releaseKey))
	}
	return problems
}

func describeSecretForm(kind, location string) string {
	if kind == "inline" {
		return fmt.Sprintf("an inline literal (%s.inlineSecret)", location)
	}
	return fmt.Sprintf("a Secret reference (%s.existingSecret)", location)
}

// hubLayerValues is the Hub release's cluster inventory: the record Identity
// provisions each cluster's components from. identity.clients is the second half
// of that record - global.topology.clusters[].components.optimize is singular, so
// a cluster with more than one Optimize registers the rest as plain Identity
// clients, and a release the cluster record does not name is provisioned there or
// nowhere.
type hubLayerValues struct {
	Global struct {
		Topology struct {
			Clusters yaml.Node `yaml:"clusters"`
		} `yaml:"topology"`
	} `yaml:"global"`
	Identity struct {
		Clients yaml.Node `yaml:"clients"`
	} `yaml:"identity"`
}

// decodeHubList reports whether a values layer stated the list at node, and
// decodes its entries into out. Presence is read from the parsed node rather than
// from the length of the result because Helm replaces a list wholesale: a later
// layer stating `[]` (or null, which merges as a nil list and ranges as empty)
// deploys no registrations at all, while a length check reads it as a layer that
// said nothing and leaves the earlier layer's entries in place.
func decodeHubList[T any](node yaml.Node, out *[]T) bool {
	if node.Kind == 0 {
		return false
	}
	*out = nil
	if node.Tag == "!!null" {
		return true
	}
	return node.Decode(out) == nil
}

// hubIdentityClient is one entry of the Hub's identity.clients list, narrowed to
// what an optimize release duplicates. RootURL carries the redirect target
// (redirectUris is appended to it), and Permissions name the resource servers
// tokens minted for this client may be audienced to.
type hubIdentityClient struct {
	ID          string            `yaml:"id"`
	RootURL     *string           `yaml:"rootUrl"`
	Secret      optimizeSecretRef `yaml:"secret"`
	Permissions []struct {
		ResourceServerID string `yaml:"resourceServerId"`
	} `yaml:"permissions"`
}

// hubInventory is everything the Hub release registers that an optimize release
// also states: the per-cluster component records, and the standalone clients.
type hubInventory struct {
	clusters map[string]hubClusterValues
	clients  map[string]hubIdentityClient

	// read is true once a Hub release's layers have been parsed, whatever they
	// turned out to register. It is what separates "this topology has no readable
	// Hub inventory to check against" from "the Hub registers nothing", which an
	// empty inventory alone cannot say and which are opposite answers: the first
	// has nothing to report, the second means no Optimize client is provisioned.
	read bool
}

// hubClusterValues is one cluster's entry in that inventory, narrowed to what an
// optimize release duplicates.
type hubClusterValues struct {
	ID           string `yaml:"id"`
	ContextPaths struct {
		Optimize *string `yaml:"optimize"`
	} `yaml:"contextPaths"`
	Components struct {
		Optimize struct {
			Enabled     *bool             `yaml:"enabled"`
			ClientID    *string           `yaml:"clientId"`
			Audience    *string           `yaml:"audience"`
			RedirectURL *string           `yaml:"redirectUrl"`
			Secret      optimizeSecretRef `yaml:"secret"`
		} `yaml:"optimize"`
	} `yaml:"components"`
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

// optimizeBackendChoice names the database backend an optimize release's layers
// leave enabled, or the empty choice when none of the layers this validator can
// read declares one.
type optimizeBackendChoice string

const (
	optimizeBackendUndeclared    optimizeBackendChoice = ""
	optimizeBackendElasticsearch optimizeBackendChoice = "elasticsearch"
	optimizeBackendOpensearch    optimizeBackendChoice = "opensearch"
)

// optimizeBackendSwitches is the merged view of the four flags the chart's
// optimize.indexPrefix consults, merged meaning the last layer to state one wins:
// Helm's own scalar precedence.
type optimizeBackendSwitches struct {
	globalElasticsearch    *bool
	componentElasticsearch *bool
	globalOpensearch       *bool
	componentOpensearch    *bool
}

func (s optimizeBackendSwitches) merge(layer optimizeLayerValues) optimizeBackendSwitches {
	if v := layer.Global.Elasticsearch.Enabled; v != nil {
		s.globalElasticsearch = v
	}
	if v := layer.Optimize.Database.Elasticsearch.Enabled; v != nil {
		s.componentElasticsearch = v
	}
	if v := layer.Global.Opensearch.Enabled; v != nil {
		s.globalOpensearch = v
	}
	if v := layer.Optimize.Database.Opensearch.Enabled; v != nil {
		s.componentOpensearch = v
	}
	return s
}

// choice mirrors optimize.indexPrefix in templates/optimize/_helpers.tpl:
// Elasticsearch is tested first and OpenSearch reached only when it is off, so a
// release that enables Elasticsearch reads optimize.database.elasticsearch.prefix
// and never looks at the OpenSearch one. Returning the empty choice for a backend
// no readable layer declares is deliberate rather than a guess at the default:
// enablement usually arrives in base.yaml or a persistence layer, and a release
// whose layers are absent from disk must not be told which key to set.
func (s optimizeBackendSwitches) choice() optimizeBackendChoice {
	if isEnabled(s.globalElasticsearch) || isEnabled(s.componentElasticsearch) {
		return optimizeBackendElasticsearch
	}
	if isEnabled(s.globalOpensearch) || isEnabled(s.componentOpensearch) {
		return optimizeBackendOpensearch
	}
	return optimizeBackendUndeclared
}

func isEnabled(flag *bool) bool {
	return flag != nil && *flag
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// releaseLayerPaths returns the values layers a topology release's deploy
// composes, in the order DeploymentConfig.ResolvePaths applies them
// (scripts/camunda-core/pkg/scenarios/scenarios.go): base first, then the
// release's identity, persistence and feature layers, last one winning. The
// platform and infra layers are left out on purpose - they are matrix-level
// selections the topology declaration does not name, and they carry ingress and
// scheduling shape rather than component configuration.
func releaseLayerPaths(r TopologyRelease, chartFullSetupDir string) []string {
	valuesDir := filepath.Join(chartFullSetupDir, "values")
	paths := []string{filepath.Join(valuesDir, "base.yaml")}
	if isPlainFilename(r.Identity) {
		paths = append(paths, filepath.Join(valuesDir, "identity", r.Identity+".yaml"))
	}
	if isPlainFilename(r.Persistence) {
		paths = append(paths, filepath.Join(valuesDir, "persistence", r.Persistence+".yaml"))
	}
	for _, featureID := range r.Features {
		if isPlainFilename(featureID) {
			paths = append(paths, filepath.Join(valuesDir, "features", featureID+".yaml"))
		}
	}
	return paths
}

// readOptimizeLayer parses one values layer. A layer that is absent or
// unparseable reports not-ok instead of an error: layer resolution is checked in
// Validate itself, and having a cross-check blame it too would report one problem
// twice in different words.
func readOptimizeLayer(path string) (optimizeLayerValues, bool) {
	content, err := os.ReadFile(path)
	if err != nil {
		return optimizeLayerValues{}, false
	}
	var layer optimizeLayerValues
	if yaml.Unmarshal(content, &layer) != nil {
		return optimizeLayerValues{}, false
	}
	return layer, true
}

// optimizeReleaseView is an optimize release's values as a deploy composes them,
// across every layer the topology declaration names. Both cross-checks below need
// a merged view rather than a per-layer one: the backend a release ends up on and
// the client it ends up sending are properties of the whole stack, and no single
// layer is required to state either.
type optimizeReleaseView struct {
	switches optimizeBackendSwitches
	oidc     optimizeOIDCLayer
}

func buildOptimizeReleaseView(r TopologyRelease, chartFullSetupDir string) optimizeReleaseView {
	var view optimizeReleaseView
	for _, path := range releaseLayerPaths(r, chartFullSetupDir) {
		layer, ok := readOptimizeLayer(path)
		if !ok {
			continue
		}
		view.switches = view.switches.merge(layer)
		view.oidc = mergeOptimizeOIDC(view.oidc, layer.Optimize.Security.Authentication.OIDC)
	}
	return view
}

func mergeOptimizeOIDC(into, from optimizeOIDCLayer) optimizeOIDCLayer {
	if from.ClientID != nil {
		into.ClientID = from.ClientID
	}
	if from.Audience != nil {
		into.Audience = from.Audience
	}
	if from.RedirectURL != nil {
		into.RedirectURL = from.RedirectURL
	}
	if from.Secret.ExistingSecret != nil {
		into.Secret.ExistingSecret = from.Secret.ExistingSecret
	}
	if from.Secret.ExistingSecretKey != nil {
		into.Secret.ExistingSecretKey = from.Secret.ExistingSecretKey
	}
	if from.Secret.InlineSecret != nil {
		into.Secret.InlineSecret = from.Secret.InlineSecret
	}
	return into
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
//
// backend is the database backend the release's whole layer stack enables, and it
// decides which prefix key satisfies the requirement. Only the feature layers are
// searched for the values themselves: those are the release's own overlay, the one
// place a per-release context path or reader prefix belongs.
func validateOptimizeLayerValues(label string, r TopologyRelease, chartFullSetupDir string, backend optimizeBackendChoice) []string {
	var problems []string
	readAny := false
	contextPathSet := false
	prefixSet := map[optimizeBackendChoice]bool{}
	for _, featureID := range r.Features {
		if !isPlainFilename(featureID) {
			continue
		}
		path := filepath.Join(chartFullSetupDir, "values", "features", featureID+".yaml")
		layer, ok := readOptimizeLayer(path)
		if !ok {
			continue
		}
		readAny = true

		if cp := layer.Optimize.ContextPath; cp != nil && *cp != "" {
			contextPathSet = true
			// A literal that happens to match the declaration today is still a
			// second copy, and the next declaration change updates only one of
			// them. Nothing about being in sync right now makes it safe, so the
			// placeholder is required even then - the same standard the index
			// prefix below is held to.
			if !isExactPlaceholder(*cp, "RELEASE_OPTIMIZE_CONTEXT_PATH") {
				problems = append(problems, fmt.Sprintf("%s: feature %q sets optimize.contextPath %q but the release declares optimize-context-path %q; set it to exactly %q so the declaration is the only copy", label, featureID, *cp, r.OptimizeContextPath, placeholderForms("RELEASE_OPTIMIZE_CONTEXT_PATH")[0]))
			}
		}

		for _, candidate := range []optimizeBackendChoice{optimizeBackendElasticsearch, optimizeBackendOpensearch} {
			prefix := layer.Optimize.Database.Elasticsearch.Prefix
			if candidate == optimizeBackendOpensearch {
				prefix = layer.Optimize.Database.Opensearch.Prefix
			}
			if prefix == nil || *prefix == "" {
				continue
			}
			prefixSet[candidate] = true
			if !placeholderLeads(*prefix, "SERVED_ORCHESTRATION_INDEX_PREFIX") {
				problems = append(problems, fmt.Sprintf("%s: feature %q sets optimize.database.%s.prefix to %q, which does not follow serves %q; it must be %q, or lead with it and carry a per-tenant suffix (the unbraced $NAME form leads only when the suffix starts with a character that ends a shell name, such as a hyphen), so repointing serves repoints the records this Optimize reads", label, featureID, candidate, *prefix, r.Serves, placeholderForms("SERVED_ORCHESTRATION_INDEX_PREFIX")[0]))
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
	if r.Serves != "" {
		problems = append(problems, servedPrefixProblems(label, r, backend, prefixSet)...)
	}
	return problems
}

// servedPrefixProblems reports an optimize release whose feature layers never
// state the index prefix Optimize will actually read. Which key that is depends
// on the enabled backend, and satisfying the requirement with the other backend's
// key is the same silent failure as setting neither: optimize.indexPrefix reads
// only the enabled backend's prefix and falls back to "zeebe-record", so the
// release reads the raw records of every orchestration sharing the cluster
// instead of the ones serves writes.
func servedPrefixProblems(label string, r TopologyRelease, backend optimizeBackendChoice, prefixSet map[optimizeBackendChoice]bool) []string {
	want := placeholderForms("SERVED_ORCHESTRATION_INDEX_PREFIX")[0]
	if backend == optimizeBackendUndeclared {
		// No readable layer enables a backend, so which prefix Optimize reads is
		// not knowable here and either key answers the requirement.
		if prefixSet[optimizeBackendElasticsearch] || prefixSet[optimizeBackendOpensearch] {
			return nil
		}
		return []string{fmt.Sprintf("%s: no feature layer sets optimize.database.elasticsearch.prefix or optimize.database.opensearch.prefix, so this release reads whatever index prefix its base layers left behind rather than the records serves %q writes; set the prefix of the enabled backend to %q in one of its feature layers", label, r.Serves, want)}
	}
	if prefixSet[backend] {
		return nil
	}
	other := optimizeBackendOpensearch
	if backend == optimizeBackendOpensearch {
		other = optimizeBackendElasticsearch
	}
	detail := fmt.Sprintf("so this release reads whatever index prefix its base layers left behind rather than the records serves %q writes", r.Serves)
	if prefixSet[other] {
		detail = fmt.Sprintf("only optimize.database.%s.prefix is set, and optimize.indexPrefix never reads it while %s is the enabled backend, so this release falls back to \"zeebe-record\" instead of the records serves %q writes", other, backend, r.Serves)
	}
	return []string{fmt.Sprintf("%s: no feature layer sets optimize.database.%s.prefix, the backend this release's layers enable; %s; set it to %q in one of its feature layers", label, backend, detail, want)}
}

// hubOptimizeInventory returns the Hub release's cluster inventory keyed by
// cluster id, and its standalone Identity clients keyed by client id. Helm
// replaces lists rather than merging them, so for each of the two the last layer
// to state it is the whole list.
func hubOptimizeInventory(t *Topology, chartFullSetupDir string) hubInventory {
	var hub *TopologyRelease
	for i := range t.Releases {
		if t.Releases[i].Role == "hub" {
			hub = &t.Releases[i]
			break
		}
	}
	if hub == nil {
		return hubInventory{}
	}
	var clusters []hubClusterValues
	var clients []hubIdentityClient
	read := false
	for _, path := range releaseLayerPaths(*hub, chartFullSetupDir) {
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var layer hubLayerValues
		if yaml.Unmarshal(content, &layer) != nil {
			continue
		}
		read = true
		var stated []hubClusterValues
		if decodeHubList(layer.Global.Topology.Clusters, &stated) {
			clusters = stated
		}
		var statedClients []hubIdentityClient
		if decodeHubList(layer.Identity.Clients, &statedClients) {
			clients = statedClients
		}
	}
	inventory := hubInventory{
		clusters: make(map[string]hubClusterValues, len(clusters)),
		clients:  make(map[string]hubIdentityClient, len(clients)),
		read:     read,
	}
	for _, c := range clusters {
		if c.ID != "" {
			inventory.clusters[c.ID] = c
		}
	}
	for _, c := range clients {
		if c.ID != "" {
			inventory.clients[c.ID] = c
		}
	}
	return inventory
}

// topologyContextPathSubs returns the OPTIMIZE_CONTEXT_PATH substitutions the
// topology driver publishes: the per-release <SUFFIX>_ form every release can
// read (buildTopologyCrossRefEnv), plus, when own is set, the RELEASE_ form that
// exists only inside that optimize release's own layers (buildTopologyReleaseEnv).
// Resolving them is what lets the two sides of an identity cross-check compare
// equal while spelling the same declared path differently, which the Hub and the
// release necessarily do - the Hub names another release's path, the release its
// own.
func topologyContextPathSubs(t *Topology, own *TopologyRelease) map[string]string {
	subs := map[string]string{}
	for _, r := range t.Releases {
		if r.Role != "optimize" || r.OptimizeContextPath == "" {
			continue
		}
		subs[TopologyEnvToken(r.NamespaceSuffix)+"_OPTIMIZE_CONTEXT_PATH"] = r.OptimizeContextPath
	}
	if own != nil && own.OptimizeContextPath != "" {
		subs["RELEASE_OPTIMIZE_CONTEXT_PATH"] = own.OptimizeContextPath
	}
	return subs
}

// expandTopologyPlaceholders resolves the substitutions this validator knows and
// canonicalizes every other name to its braced form, so two values naming the
// same unresolved variable compare equal whichever spelling each used. A name
// that is published to one side only stays unresolved there, which is the correct
// answer: at deploy time it expands to the empty string.
func expandTopologyPlaceholders(value string, subs map[string]string) string {
	return os.Expand(value, func(name string) string {
		if resolved, ok := subs[name]; ok {
			return resolved
		}
		return "${" + name + "}"
	})
}

// validateOptimizeIdentityAgainstHub cross-checks the OIDC identity an optimize
// release presents against the one the Hub provisions for the cluster it serves.
// The two are written in different layers - the release's feature layer sends the
// client id, audience, redirect URL and secret; the Hub's inventory has Identity
// provision them - so nothing but this check stops one side from changing alone.
// A mismatch is invisible to the deploy: every workload rolls out and reports
// ready, and only a browser login fails, which is exactly the assertion the
// physicaltenants scenario currently skips (skip-e2e).
//
// A cluster record holds one Optimize, so when several optimize releases serve one
// orchestration only one of them is the release that record names; the others are
// provisioned as standalone identity.clients entries and are checked against those.
// Deciding which is which by client id rather than by declaration order is what
// keeps the check from blaming whichever release did not win the registration.
func validateOptimizeIdentityAgainstHub(label string, t *Topology, r TopologyRelease, oidc optimizeOIDCLayer, inventory hubInventory) []string {
	if r.Serves == "" || !inventory.read {
		return nil
	}

	clusterID := ""
	servingSameOrchestration := 0
	for _, other := range t.Releases {
		if other.Role == "optimize" && other.Serves == r.Serves {
			servingSameOrchestration++
		}
		if other.Role == "orchestration" && other.NamespaceSuffix == r.Serves {
			clusterID = other.ModelerClusterID
			if clusterID == "" {
				clusterID = other.NamespaceSuffix
			}
		}
	}
	if clusterID == "" {
		return nil
	}
	hubSubs := topologyContextPathSubs(t, nil)
	releaseSubs := topologyContextPathSubs(t, &r)

	// No cluster record with an enabled Optimize leaves a standalone
	// identity.clients entry as the only thing that can provision this release,
	// which is the same position an Optimize the record does not name is in. That
	// is also the state a later Hub layer produces by replacing the inventory with
	// an empty list, so it is checked rather than passed over.
	cluster, hasRecord := inventory.clusters[clusterID]
	if !hasRecord || !isEnabled(cluster.Components.Optimize.Enabled) {
		return validateOptimizeIdentityAgainstHubClients(label, r, oidc, inventory, clusterID, "", false, hubSubs, releaseSubs)
	}

	registered := stringValue(cluster.Components.Optimize.ClientID)
	presented := stringValue(oidc.ClientID)

	// The cluster record names this release when their client ids agree. With one
	// optimize release serving the orchestration there is no other candidate, so
	// the record names it even if it states no client id - and then the client id
	// comparison below is the check that says so.
	isRecordRelease := servingSameOrchestration == 1 ||
		(registered != "" && presented != "" &&
			expandTopologyPlaceholders(registered, hubSubs) == expandTopologyPlaceholders(presented, releaseSubs))
	if !isRecordRelease {
		return validateOptimizeIdentityAgainstHubClients(label, r, oidc, inventory, clusterID, registered, true, hubSubs, releaseSubs)
	}

	var problems []string
	compare := func(what, hubKey, releaseKey string, hubValue, releaseValue *string) {
		hub := stringValue(hubValue)
		if hub == "" {
			return
		}
		release := stringValue(releaseValue)
		if release == "" {
			problems = append(problems, fmt.Sprintf("%s: the Hub registers this cluster's Optimize %s as %q (global.topology.clusters[id=%q].components.optimize.%s) but no layer of this release sets %s, so it presents the chart default instead of the identity Identity provisioned for it", label, what, hub, clusterID, hubKey, releaseKey))
			return
		}
		if expandTopologyPlaceholders(hub, hubSubs) == expandTopologyPlaceholders(release, releaseSubs) {
			return
		}
		problems = append(problems, fmt.Sprintf("%s: %s is %q but the Hub registers this cluster's Optimize %s as %q (global.topology.clusters[id=%q].components.optimize.%s); the value Optimize presents and the value Identity provisioned have to be the same one", label, releaseKey, release, what, hub, clusterID, hubKey))
	}
	optimizeOIDCKey := "optimize.security.authentication.oidc."
	compare("client id", "clientId", optimizeOIDCKey+"clientId", cluster.Components.Optimize.ClientID, oidc.ClientID)
	compare("audience", "audience", optimizeOIDCKey+"audience", cluster.Components.Optimize.Audience, oidc.Audience)
	compare("redirect URL", "redirectUrl", optimizeOIDCKey+"redirectUrl", cluster.Components.Optimize.RedirectURL, oidc.RedirectURL)
	problems = append(problems, secretFormProblems(
		label,
		fmt.Sprintf("global.topology.clusters[id=%q].components.optimize.secret", clusterID),
		normalizedSecretForm(cluster.Components.Optimize.Secret),
		inlineFirstSecretForm(oidc.Secret),
		hubSubs, releaseSubs,
	)...)

	// The inventory context path is the third copy of optimize-context-path: it is
	// what the Hub's Console and Web Modeler link to, and a stale one sends users
	// to a path no ingress serves while the redirect URL above still matches. It is
	// checked only against the release the record names, since that is the Optimize
	// the record describes.
	if advertised := stringValue(cluster.ContextPaths.Optimize); advertised != "" && r.OptimizeContextPath != "" {
		if expandTopologyPlaceholders(advertised, hubSubs) != r.OptimizeContextPath {
			problems = append(problems, fmt.Sprintf("%s: the Hub advertises this cluster's Optimize at %q (global.topology.clusters[id=%q].contextPaths.optimize) but the release declares optimize-context-path %q; set it to %q so the declaration is the only copy", label, advertised, clusterID, r.OptimizeContextPath, placeholderForms(TopologyEnvToken(r.NamespaceSuffix) + "_OPTIMIZE_CONTEXT_PATH")[0]))
		}
	}
	return problems
}

// validateOptimizeIdentityAgainstHubClients cross-checks an optimize release the
// cluster record does not name against the standalone Identity client that has to
// provision it instead. Absent such an entry the release presents a client id
// Identity never creates, which the deploy cannot see: Optimize starts, reports
// ready, and only the first login fails.
func validateOptimizeIdentityAgainstHubClients(label string, r TopologyRelease, oidc optimizeOIDCLayer, inventory hubInventory, clusterID, registered string, hasRecord bool, hubSubs, releaseSubs map[string]string) []string {
	optimizeOIDCKey := "optimize.security.authentication.oidc."
	presented := stringValue(oidc.ClientID)
	// The record either names another release's client or names none at all; both
	// leave this release to a client of its own, and saying which one it is keeps
	// the message pointing at something the reader can go and look up.
	recordSays := fmt.Sprintf("this cluster's record registers the client id %q", registered)
	if registered == "" {
		recordSays = "this cluster's record registers no Optimize client id"
	}
	if presented == "" {
		// Silent unless a record exists to take the registration away from this
		// release: with no record on either side, neither the Hub nor the release
		// states a client, and this cross-check compares what the two sides state
		// rather than requiring them to state anything.
		if !hasRecord {
			return nil
		}
		return []string{fmt.Sprintf("%s: %s (global.topology.clusters[id=%q].components.optimize), so this release has to present a client of its own, but no layer of it sets %sclientId; it would send the chart default, which Identity provisions for nothing", label, recordSays, clusterID, optimizeOIDCKey)}
	}
	var client hubIdentityClient
	found := false
	for id, candidate := range inventory.clients {
		if expandTopologyPlaceholders(id, hubSubs) == expandTopologyPlaceholders(presented, releaseSubs) {
			client, found = candidate, true
			break
		}
	}
	if !found {
		return []string{fmt.Sprintf("%s: %sclientId is %q and %s, so nothing provisions this release's client; add it to the Hub release's identity.clients so Identity creates it", label, optimizeOIDCKey, presented, recordSays)}
	}

	var problems []string
	clientKey := fmt.Sprintf("identity.clients[id=%q]", client.ID)
	compare := func(what, hubKey, releaseKey, hubValue, releaseValue string) {
		if hubValue == "" || releaseValue == "" {
			return
		}
		if expandTopologyPlaceholders(hubValue, hubSubs) == expandTopologyPlaceholders(releaseValue, releaseSubs) {
			return
		}
		problems = append(problems, fmt.Sprintf("%s: %s is %q but the Hub provisions this client's %s as %q (%s.%s); the value Optimize presents and the value Identity provisioned have to be the same one", label, releaseKey, releaseValue, what, hubValue, clientKey, hubKey))
	}
	// rootUrl is the redirect target Identity registers: redirectUris are resolved
	// against it, so an Optimize whose redirectUrl points elsewhere is refused at
	// the callback.
	compare("root URL", "rootUrl", optimizeOIDCKey+"redirectUrl", stringValue(client.RootURL), stringValue(oidc.RedirectURL))
	problems = append(problems, secretFormProblems(
		label,
		clientKey+".secret",
		inlineFirstSecretForm(client.Secret),
		inlineFirstSecretForm(oidc.Secret),
		hubSubs, releaseSubs,
	)...)

	// The audience is the resource server the token is minted for, and Identity
	// mints one only for a resource server this client is permitted on.
	if audience := stringValue(oidc.Audience); audience != "" && len(client.Permissions) > 0 {
		permitted := false
		var servers []string
		for _, permission := range client.Permissions {
			if permission.ResourceServerID == "" {
				continue
			}
			servers = append(servers, permission.ResourceServerID)
			if expandTopologyPlaceholders(permission.ResourceServerID, hubSubs) == expandTopologyPlaceholders(audience, releaseSubs) {
				permitted = true
			}
		}
		if !permitted && len(servers) > 0 {
			problems = append(problems, fmt.Sprintf("%s: %saudience is %q but the Hub permits this client only on %s (%s.permissions[].resourceServerId), so Identity mints no token Optimize will accept", label, optimizeOIDCKey, audience, strings.Join(servers, ", "), clientKey))
		}
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

// TopologyEnvToken renders a namespace-suffix as the prefix the topology driver
// gives that release's cross-reference substitution variables, e.g. "opta" ->
// "OPTA" in OPTA_OPTIMIZE_CONTEXT_PATH. The driver (buildTopologyCrossRefEnv in
// cmd/matrix.go) and this package's cross-checks have to agree on the spelling,
// or a validator would look up a variable no release publishes and pass
// vacuously, so the rule lives here and the driver calls it.
func TopologyEnvToken(value string) string {
	var token strings.Builder
	for _, r := range strings.ToUpper(value) {
		if r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			token.WriteRune(r)
		} else {
			token.WriteByte('_')
		}
	}
	return strings.Trim(token.String(), "_")
}
