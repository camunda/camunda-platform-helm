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
)

// Topology describes a multi-namespace deployment shape for a scenario: one
// "hub" release (Identity, Console, Web Modeler, bundled Keycloak)
// plus one or more "orchestration" releases (Zeebe/Operate/Tasklist,
// Connectors, Optimize) that share a single logical cluster via a central
// Identity and a shared secondary storage backend.
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
	// ChartVersion selects the local chart and values layers for this release.
	// Empty inherits the parent matrix entry's version.
	ChartVersion string `yaml:"chart-version,omitempty" json:"chartVersion,omitempty"`

	// Role is either "hub" or "orchestration". Exactly one
	// "hub" role must be declared per Topology.
	Role string `yaml:"role" json:"role"`

	// NamespaceSuffix is appended to the base namespace to form this
	// release's namespace (<base>-<namespace-suffix>). Must be unique
	// within the Topology.
	NamespaceSuffix string `yaml:"namespace-suffix" json:"namespaceSuffix"`

	// Values names the values file (relative to the scenario's
	// chart-full-setup values dir) applied for this release.
	Values string `yaml:"values" json:"values"`

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

	// ResolvedDependencies holds the fully-resolved companion chart specs
	// for Dependencies, populated by LoadRegistry (mirroring how
	// registryScenario.DependencyIDs resolves into CIScenario.Dependencies).
	// Not part of the YAML/JSON wire format — it's a load-time cache
	// consumed by the topology deploy driver.
	ResolvedDependencies []ChartDependency `yaml:"-" json:"-"`
}

// Validate enforces Topology's load-time invariants:
//   - at least one release is declared;
//   - every release's chart-version resolves to a local chart directory;
//   - every release's Values file resolves on disk under
//     its selected chart's chart-full-setup values directory;
//   - every release's Identity/Persistence/Features layers (when set) resolve
//     under the selected chart's chart-full-setup values directory;
//   - every release's Dependencies IDs (when set) resolve to a file under
//     <depsDir>/<id>.yaml;
//   - every release's DependsOn (when set) references a declared Role;
//   - exactly one release has Role == "hub";
//   - NamespaceSuffix values are unique and non-empty.
//
// ctx is prepended to error messages, e.g. `scenario "multinamespace": topology: ...`.
func (t *Topology) Validate(ctx string, chartDir string, depsDir string) error {
	if t == nil {
		return nil
	}
	repoRoot, parentVersion, err := deriveRepoRootAndVersion(chartDir)
	if err != nil {
		return err
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
		chartVersion := r.ChartVersion
		if chartVersion == "" {
			chartVersion = parentVersion
		}
		releaseChartDir := chartDir
		if isPlainFilename(chartVersion) {
			releaseChartDir = filepath.Join(repoRoot, "charts", "camunda-platform-"+chartVersion)
		}
		chartFullSetupDir := filepath.Join(releaseChartDir, "test", "integration", "scenarios", "chart-full-setup")
		if !isPlainFilename(chartVersion) {
			problems = append(problems, fmt.Sprintf("%s: chart-version %q must not contain path separators", label, chartVersion))
		} else if info, err := os.Stat(releaseChartDir); err != nil || !info.IsDir() {
			problems = append(problems, fmt.Sprintf("%s: chart-version %q: missing local chart directory at %s", label, chartVersion, releaseChartDir))
		}

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
		default:
			problems = append(problems, fmt.Sprintf("%s: role must be \"hub\" or \"orchestration\", got %q", label, r.Role))
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

		for _, feature := range r.Features {
			featurePath := filepath.Join(chartFullSetupDir, "values", "features", feature+".yaml")
			if info, err := os.Stat(featurePath); err != nil || info.IsDir() {
				problems = append(problems, fmt.Sprintf("%s: feature %q: missing values file at %s", label, feature, featurePath))
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
