package matrix

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Topology describes a multi-namespace deployment shape for a scenario: one
// "management" release (Identity, Console, Web Modeler, bundled Keycloak)
// plus one or more "orchestration" releases (Zeebe/Operate/Tasklist,
// Connectors, Optimize) that share a single logical cluster via a central
// Identity and a shared secondary storage backend.
//
// Scenarios without a Topology behave byte-for-byte as today — this field is
// additive and opt-in (see registryScenario.Topology / CIScenario.Topology).
type Topology struct {
	// Name is a human-readable label for the topology shape, e.g. "mgmt-2orch".
	Name string `yaml:"name" json:"name"`

	// Releases lists every namespace/release this scenario fans out to.
	Releases []TopologyRelease `yaml:"releases" json:"releases"`

	// SharedStorage names the companion dependency (e.g. "elasticsearch")
	// deployed once (into the management namespace) and referenced by every
	// orchestration release via FQDN, rather than deployed per-release.
	SharedStorage string `yaml:"shared-storage,omitempty" json:"sharedStorage,omitempty"`

	// SharedStorageService is the Kubernetes Service name of the shared storage backend (defaults to SharedStorage/release name; elastic chart uses <clusterName>-master).
	SharedStorageService string `yaml:"shared-storage-service,omitempty" json:"sharedStorageService,omitempty"`
}

// TopologyRelease is one namespace/release within a Topology. Each release
// can select its own identity/persistence/features/dependencies layers —
// e.g. the management release uses the bundled-Keycloak identity layer and
// deploys keycloak/postgresql/elasticsearch, while orchestration releases
// use an external-Keycloak layer pointed back at management and deploy no
// companions of their own (they consume the management release's shared
// Elasticsearch and Identity/Keycloak cross-namespace by FQDN).
type TopologyRelease struct {
	// Role is either "management" or "orchestration". Exactly one
	// "management" role must be declared per Topology.
	Role string `yaml:"role" json:"role"`

	// NamespaceSuffix is appended to the base namespace to form this
	// release's namespace (<base>-<namespace-suffix>). Must be unique
	// within the Topology.
	NamespaceSuffix string `yaml:"namespace-suffix" json:"namespaceSuffix"`

	// Values names the values file (relative to the scenario's
	// chart-full-setup values dir) applied for this release.
	Values string `yaml:"values" json:"values"`

	// DependsOn, when set, names the Role of a release that must be deployed
	// (and, for "management", ready) before this one.
	DependsOn string `yaml:"depends-on,omitempty" json:"dependsOn,omitempty"`

	// Identity, when set, overrides the scenario-level Identity layer for
	// this release only (e.g. "keycloak" for management vs
	// "keycloak-external" for orchestration releases that point back at the
	// management namespace's Keycloak/Identity instead of deploying their
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
	// release that consumes the management release's shared Elasticsearch
	// cross-namespace instead of deploying its own copy).
	Dependencies []string `yaml:"dependencies,omitempty" json:"dependencies,omitempty"`

	// ResolvedDependencies holds the fully-resolved companion chart specs
	// for Dependencies, populated by LoadRegistry (mirroring how
	// registryScenario.DependencyIDs resolves into CIScenario.Dependencies).
	// Not part of the YAML/JSON wire format — it's a load-time cache
	// consumed by the topology deploy driver.
	ResolvedDependencies []ChartDependency `yaml:"-" json:"-"`
}

// Validate enforces Topology's load-time invariants:
//   - at least one release is declared;
//   - every release's Values file resolves on disk under
//     <chartFullSetupDir>/values/<Values>;
//   - every release's Identity/Persistence layer (when set) resolves on disk
//     under <chartFullSetupDir>/values/identity/ or .../persistence/;
//   - every release's Dependencies IDs (when set) resolve to a file under
//     <depsDir>/<id>.yaml;
//   - every release's DependsOn (when set) references a declared Role;
//   - exactly one release has Role == "management";
//   - NamespaceSuffix values are unique and non-empty.
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
	managementCount := 0

	for i, r := range t.Releases {
		label := fmt.Sprintf("%s: topology %q: release[%d] (role %q, namespace-suffix %q)", ctx, t.Name, i, r.Role, r.NamespaceSuffix)

		switch r.Role {
		case "management":
			managementCount++
		case "orchestration":
			// valid
		default:
			problems = append(problems, fmt.Sprintf("%s: role must be \"management\" or \"orchestration\", got %q", label, r.Role))
		}
		roles[r.Role] = true

		if strings.TrimSpace(r.NamespaceSuffix) == "" {
			problems = append(problems, fmt.Sprintf("%s: namespace-suffix is required", label))
		} else if suffixes[r.NamespaceSuffix] {
			problems = append(problems, fmt.Sprintf("%s: duplicate namespace-suffix %q", label, r.NamespaceSuffix))
		} else {
			suffixes[r.NamespaceSuffix] = true
		}
		if len(r.NamespaceSuffix) > 12 {
			problems = append(problems, fmt.Sprintf("%s: namespace-suffix %q is too long (max 12 chars, to keep <namespace>-<suffix> well within the 63-char Kubernetes limit)", label, r.NamespaceSuffix))
		}

		if strings.TrimSpace(r.Values) == "" {
			problems = append(problems, fmt.Sprintf("%s: values is required", label))
		} else {
			valuesPath := filepath.Join(chartFullSetupDir, "values", r.Values)
			if info, err := os.Stat(valuesPath); err != nil || info.IsDir() {
				problems = append(problems, fmt.Sprintf("%s: values %q: missing values file at %s", label, r.Values, valuesPath))
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

	if managementCount != 1 {
		problems = append(problems, fmt.Sprintf("%s: topology %q: exactly one release with role \"management\" is required, found %d", ctx, t.Name, managementCount))
	}

	for i, r := range t.Releases {
		if r.DependsOn == "" {
			continue
		}
		if !roles[r.DependsOn] {
			problems = append(problems, fmt.Sprintf("%s: topology %q: release[%d] depends-on %q does not reference a declared role", ctx, t.Name, i, r.DependsOn))
		}
	}

	if len(problems) == 0 {
		return nil
	}
	return fmt.Errorf("%s", strings.Join(problems, "\n  - "))
}
