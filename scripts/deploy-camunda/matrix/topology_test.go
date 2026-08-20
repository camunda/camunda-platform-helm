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
	"strings"
	"testing"
)

func writeValuesFile(t *testing.T, dir, name string) {
	t.Helper()
	path := filepath.Join(dir, "values", name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func writeDepFile(t *testing.T, depsDir, id string) {
	t.Helper()
	if err := os.MkdirAll(depsDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(depsDir, id+".yaml"), []byte("release-name: "+id+"\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func TestTopologyValidate_NilIsNoop(t *testing.T) {
	var top *Topology
	if err := top.Validate("ctx", t.TempDir(), t.TempDir()); err != nil {
		t.Fatalf("nil Topology should be a no-op, got: %v", err)
	}
}

func TestTopologyValidate_Valid(t *testing.T) {
	dir := t.TempDir()
	depsDir := filepath.Join(t.TempDir(), "dependencies")
	writeValuesFile(t, dir, "features/hub.yaml")
	writeValuesFile(t, dir, "features/orchestration.yaml")
	writeValuesFile(t, dir, "identity/keycloak.yaml")
	writeValuesFile(t, dir, "identity/keycloak-external.yaml")
	writeValuesFile(t, dir, "persistence/elasticsearch-external.yaml")
	writeDepFile(t, depsDir, "keycloak")
	writeDepFile(t, depsDir, "postgresql")
	writeDepFile(t, depsDir, "elasticsearch")

	top := &Topology{
		Name:          "hub-2orch",
		SharedStorage: "elasticsearch",
		Releases: []TopologyRelease{
			{
				Role:            "hub",
				NamespaceSuffix: "hub",
				Features:        []string{"hub"},
				Identity:        "keycloak",
				Dependencies:    []string{"keycloak", "postgresql", "elasticsearch"},
			},
			{
				Role:               "orchestration",
				NamespaceSuffix:    "orcha",
				ModelerClusterID:   "orcha",
				ModelerClusterName: "Orchestration A",
				Features:           []string{"orchestration"},
				Identity:           "keycloak-external",
				Persistence:        "elasticsearch-external",
				DependsOn:          "hub",
				Env:                map[string]string{"ORCH_ORCHESTRATION_CLIENT_ID": "orchestration-orcha"},
			},
			{
				Role:               "orchestration",
				NamespaceSuffix:    "orchb",
				ModelerClusterID:   "orchb",
				ModelerClusterName: "Orchestration B",
				Features:           []string{"orchestration"},
				Identity:           "keycloak-external",
				Persistence:        "elasticsearch-external",
				DependsOn:          "hub",
			},
		},
	}
	if err := top.Validate("ctx", dir, depsDir); err != nil {
		t.Fatalf("expected valid topology, got: %v", err)
	}
	if got := top.Releases[1].Env["ORCH_ORCHESTRATION_CLIENT_ID"]; got != "orchestration-orcha" {
		t.Fatalf("release env = %q", got)
	}
}

func TestTopologyValidate_ValidWithOptimizeReleases(t *testing.T) {
	dir := t.TempDir()
	depsDir := filepath.Join(t.TempDir(), "dependencies")
	writeValuesFile(t, dir, "features/hub.yaml")
	writeValuesFile(t, dir, "features/orchestration.yaml")
	writeValuesFile(t, dir, "features/optimize.yaml")
	writeValuesFile(t, dir, "identity/keycloak.yaml")
	writeValuesFile(t, dir, "identity/keycloak-external.yaml")
	writeValuesFile(t, dir, "persistence/elasticsearch-external.yaml")
	writeDepFile(t, depsDir, "elasticsearch")

	top := &Topology{
		Name:          "hub-orch-2optimize",
		SharedStorage: "elasticsearch",
		Releases: []TopologyRelease{
			{
				Role:            "hub",
				NamespaceSuffix: "hub",
				Features:        []string{"hub"},
				Identity:        "keycloak",
				Dependencies:    []string{"elasticsearch"},
			},
			{
				Role:               "orchestration",
				NamespaceSuffix:    "orcha",
				ModelerClusterID:   "orcha",
				ModelerClusterName: "Orchestration A",
				Features:           []string{"orchestration"},
				Identity:           "keycloak-external",
				Persistence:        "elasticsearch-external",
				DependsOn:          "hub",
			},
			{
				Role:                "optimize",
				NamespaceSuffix:     "opta",
				Serves:              "orcha",
				OptimizeContextPath: "/optimize-orcha",
				Features:            []string{"optimize"},
				Identity:            "keycloak-external",
				Persistence:         "elasticsearch-external",
				DependsOn:           "hub",
			},
		},
	}
	if err := top.Validate("ctx", dir, depsDir); err != nil {
		t.Fatalf("expected valid topology with optimize releases, got: %v", err)
	}
}

func TestTopologyValidate_OptimizeRoleNeedsNoModelerCluster(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "features/hub.yaml")
	writeValuesFile(t, dir, "features/orchestration.yaml")
	writeValuesFile(t, dir, "features/optimize.yaml")
	top := &Topology{
		Name: "optimize-no-modeler-cluster",
		Releases: []TopologyRelease{
			{Role: "hub", NamespaceSuffix: "hub", Features: []string{"hub"}},
			{Role: "orchestration", NamespaceSuffix: "orcha", Features: []string{"orchestration"}, ModelerClusterID: "orcha", ModelerClusterName: "Orchestration A", DependsOn: "hub"},
			{Role: "optimize", NamespaceSuffix: "opta", Serves: "orcha", OptimizeContextPath: "/optimize-orcha", Features: []string{"optimize"}, DependsOn: "hub"},
		},
	}

	if err := top.Validate("ctx", dir, t.TempDir()); err != nil {
		t.Fatalf("optimize role must not require modeler-cluster-id/name, got: %v", err)
	}
}

func TestTopologyValidate_OptimizeRoleRequiresDependsOn(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "features/hub.yaml")
	writeValuesFile(t, dir, "features/orchestration.yaml")
	writeValuesFile(t, dir, "features/optimize.yaml")
	top := &Topology{
		Name: "optimize-without-depends-on",
		Releases: []TopologyRelease{
			{Role: "hub", NamespaceSuffix: "hub", Features: []string{"hub"}},
			{Role: "orchestration", NamespaceSuffix: "orcha", Features: []string{"orchestration"}, ModelerClusterID: "orcha", ModelerClusterName: "Orchestration A", DependsOn: "hub"},
			{Role: "optimize", NamespaceSuffix: "opta", Serves: "orcha", OptimizeContextPath: "/optimize-orcha", Features: []string{"optimize"}},
		},
	}

	err := top.Validate("ctx", dir, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), `depends-on must be "hub"`) {
		t.Fatalf("expected depends-on=hub requirement for optimize role, got %v", err)
	}
}

func TestTopologyValidate_OptimizeRoleRejectsNonHubDependsOn(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "features/hub.yaml")
	writeValuesFile(t, dir, "features/orchestration.yaml")
	writeValuesFile(t, dir, "features/optimize.yaml")
	top := &Topology{
		Name: "optimize-depends-on-orchestration",
		Releases: []TopologyRelease{
			{Role: "hub", NamespaceSuffix: "hub", Features: []string{"hub"}},
			{Role: "orchestration", NamespaceSuffix: "orcha", Features: []string{"orchestration"}, ModelerClusterID: "orcha", ModelerClusterName: "Orchestration A", DependsOn: "hub"},
			{Role: "optimize", NamespaceSuffix: "opta", Serves: "orcha", OptimizeContextPath: "/optimize-orcha", Features: []string{"optimize"}, DependsOn: "orchestration"},
		},
	}

	err := top.Validate("ctx", dir, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), `depends-on must be "hub"`) {
		t.Fatalf("expected optimize role to reject depends-on=orchestration, got %v", err)
	}
}

func TestTopologyValidate_OptimizeRoleRequiresServesAndContextPath(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "features/hub.yaml")
	writeValuesFile(t, dir, "features/orchestration.yaml")
	writeValuesFile(t, dir, "features/optimize.yaml")
	top := &Topology{
		Name: "optimize-missing-mapping",
		Releases: []TopologyRelease{
			{Role: "hub", NamespaceSuffix: "hub", Features: []string{"hub"}},
			{Role: "orchestration", NamespaceSuffix: "orcha", Features: []string{"orchestration"}, ModelerClusterID: "orcha", ModelerClusterName: "Orchestration A", DependsOn: "hub"},
			{Role: "optimize", NamespaceSuffix: "opta", Features: []string{"optimize"}, DependsOn: "hub"},
		},
	}

	err := top.Validate("ctx", dir, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "serves is required") || !strings.Contains(err.Error(), "optimize-context-path is required") {
		t.Fatalf("expected serves and optimize-context-path requirements, got %v", err)
	}
}

func TestTopologyValidate_OptimizeServesMustNameOrchestrationRelease(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "features/hub.yaml")
	writeValuesFile(t, dir, "features/orchestration.yaml")
	writeValuesFile(t, dir, "features/optimize.yaml")
	top := &Topology{
		Name: "optimize-unknown-serves",
		Releases: []TopologyRelease{
			{Role: "hub", NamespaceSuffix: "hub", Features: []string{"hub"}},
			{Role: "orchestration", NamespaceSuffix: "orcha", Features: []string{"orchestration"}, ModelerClusterID: "orcha", ModelerClusterName: "Orchestration A", DependsOn: "hub"},
			{Role: "optimize", NamespaceSuffix: "opta", Serves: "nope", OptimizeContextPath: "/optimize-orcha", Features: []string{"optimize"}, DependsOn: "hub"},
		},
	}

	err := top.Validate("ctx", dir, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "does not reference a declared orchestration release") {
		t.Fatalf("expected serves cross-check, got %v", err)
	}
}

func TestTopologyValidate_RejectsOptimizeOnlyFieldsOnOtherRoles(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "features/hub.yaml")
	writeValuesFile(t, dir, "features/orchestration.yaml")
	top := &Topology{
		Name: "serves-on-orchestration",
		Releases: []TopologyRelease{
			{Role: "hub", NamespaceSuffix: "hub", Features: []string{"hub"}},
			{Role: "orchestration", NamespaceSuffix: "orcha", Features: []string{"orchestration"}, ModelerClusterID: "orcha", ModelerClusterName: "Orchestration A", DependsOn: "hub", Serves: "orcha", OptimizeContextPath: "/optimize"},
		},
	}

	err := top.Validate("ctx", dir, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "must not set serves") || !strings.Contains(err.Error(), "must not set optimize-context-path") {
		t.Fatalf("expected serves/optimize-context-path to be rejected on non-optimize roles, got %v", err)
	}
}

func TestTopologyValidate_RequiresUniqueModelerClusters(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "features/hub.yaml")
	writeValuesFile(t, dir, "features/orchestration.yaml")
	top := &Topology{
		Name: "duplicate-modeler-cluster",
		Releases: []TopologyRelease{
			{Role: "hub", NamespaceSuffix: "hub", Features: []string{"hub"}},
			{Role: "orchestration", NamespaceSuffix: "orcha", Features: []string{"orchestration"}, ModelerClusterID: "shared", ModelerClusterName: "Shared"},
			{Role: "orchestration", NamespaceSuffix: "orchb", Features: []string{"orchestration"}, ModelerClusterID: "shared", ModelerClusterName: "Shared"},
		},
	}

	err := top.Validate("ctx", dir, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "duplicate modeler-cluster-id") || !strings.Contains(err.Error(), "duplicate modeler-cluster-name") {
		t.Fatalf("expected duplicate Modeler cluster validation errors, got %v", err)
	}
}

func TestTopologyValidate_RequiresOrchestrationRelease(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "features/hub.yaml")
	top := &Topology{
		Name: "hub-only",
		Releases: []TopologyRelease{
			{Role: "hub", NamespaceSuffix: "hub", Features: []string{"hub"}},
		},
	}

	err := top.Validate("ctx", dir, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "at least one release with role \"orchestration\"") {
		t.Fatalf("expected missing orchestration release error, got %v", err)
	}
}

func TestTopologyValidate_RequiresDNS1123NamespaceSuffix(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "features/hub.yaml")
	writeValuesFile(t, dir, "features/orchestration.yaml")
	top := &Topology{
		Name: "invalid-suffix",
		Releases: []TopologyRelease{
			{Role: "hub", NamespaceSuffix: "Hub", Features: []string{"hub"}},
			{Role: "orchestration", NamespaceSuffix: "orch_a", Features: []string{"orchestration"}, ModelerClusterID: "orcha", ModelerClusterName: "Orchestration A"},
		},
	}

	err := top.Validate("ctx", dir, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "must be a lowercase DNS-1123 label") {
		t.Fatalf("expected DNS-1123 namespace suffix errors, got %v", err)
	}
}

func TestTopologyValidate_MissingValuesFile(t *testing.T) {
	dir := t.TempDir()
	top := &Topology{
		Name: "hub-1orch",
		Releases: []TopologyRelease{
			{Role: "hub", NamespaceSuffix: "hub", Features: []string{"hub"}},
			{Role: "orchestration", NamespaceSuffix: "orcha", Features: []string{"orchestration"}},
		},
	}
	if err := top.Validate("ctx", dir, t.TempDir()); err == nil {
		t.Fatal("expected error for missing values files")
	}
}

func TestTopologyValidate_MissingIdentityLayer(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "features/hub.yaml")
	top := &Topology{
		Name: "bad-identity",
		Releases: []TopologyRelease{
			{Role: "hub", NamespaceSuffix: "hub", Features: []string{"hub"}, Identity: "does-not-exist"},
		},
	}
	if err := top.Validate("ctx", dir, t.TempDir()); err == nil {
		t.Fatal("expected error for missing identity layer file")
	}
}

func TestTopologyValidate_MissingPersistenceLayer(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "features/orchestration.yaml")
	top := &Topology{
		Name: "bad-persistence",
		Releases: []TopologyRelease{
			{Role: "hub", NamespaceSuffix: "hub", Features: []string{"orchestration"}, Persistence: "does-not-exist"},
		},
	}
	if err := top.Validate("ctx", dir, t.TempDir()); err == nil {
		t.Fatal("expected error for missing persistence layer file")
	}
}

func TestTopologyValidate_MissingDependency(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "features/hub.yaml")
	top := &Topology{
		Name: "bad-dep",
		Releases: []TopologyRelease{
			{Role: "hub", NamespaceSuffix: "hub", Features: []string{"hub"}, Dependencies: []string{"does-not-exist"}},
		},
	}
	if err := top.Validate("ctx", dir, filepath.Join(t.TempDir(), "dependencies")); err == nil {
		t.Fatal("expected error for missing dependency file")
	}
}

func TestTopologyValidate_NoHubRole(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "features/orchestration.yaml")
	top := &Topology{
		Name: "no-hub",
		Releases: []TopologyRelease{
			{Role: "orchestration", NamespaceSuffix: "orcha", Features: []string{"orchestration"}},
			{Role: "orchestration", NamespaceSuffix: "orchb", Features: []string{"orchestration"}},
		},
	}
	if err := top.Validate("ctx", dir, t.TempDir()); err == nil {
		t.Fatal("expected error for missing Hub role")
	}
}

func TestTopologyValidate_TwoHubRoles(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "features/hub.yaml")
	top := &Topology{
		Name: "two-hub",
		Releases: []TopologyRelease{
			{Role: "hub", NamespaceSuffix: "huba", Features: []string{"hub"}},
			{Role: "hub", NamespaceSuffix: "hubb", Features: []string{"hub"}},
		},
	}
	if err := top.Validate("ctx", dir, t.TempDir()); err == nil {
		t.Fatal("expected error for two Hub roles")
	}
}

func TestTopologyValidate_DuplicateNamespaceSuffix(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "features/hub.yaml")
	writeValuesFile(t, dir, "features/orchestration.yaml")
	top := &Topology{
		Name: "dup-suffix",
		Releases: []TopologyRelease{
			{Role: "hub", NamespaceSuffix: "a", Features: []string{"hub"}},
			{Role: "orchestration", NamespaceSuffix: "a", Features: []string{"orchestration"}},
		},
	}
	if err := top.Validate("ctx", dir, t.TempDir()); err == nil {
		t.Fatal("expected error for duplicate namespace-suffix")
	}
}

func TestTopologyValidate_DependsOnUnknownRole(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "features/hub.yaml")
	writeValuesFile(t, dir, "features/orchestration.yaml")
	top := &Topology{
		Name: "bad-depends-on",
		Releases: []TopologyRelease{
			{Role: "hub", NamespaceSuffix: "hub", Features: []string{"hub"}},
			{Role: "orchestration", NamespaceSuffix: "orcha", Features: []string{"orchestration"}, DependsOn: "storage"},
		},
	}
	if err := top.Validate("ctx", dir, t.TempDir()); err == nil {
		t.Fatal("expected error for depends-on referencing an undeclared role")
	}
}

func TestTopologyValidate_InvalidRole(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "weird.yaml")
	top := &Topology{
		Name: "bad-role",
		Releases: []TopologyRelease{
			{Role: "weird", NamespaceSuffix: "w", Features: []string{"weird"}},
		},
	}
	if err := top.Validate("ctx", dir, t.TempDir()); err == nil {
		t.Fatal("expected error for invalid role")
	}
}

func TestTopologyValidate_EmptyReleases(t *testing.T) {
	top := &Topology{Name: "empty"}
	if err := top.Validate("ctx", t.TempDir(), t.TempDir()); err == nil {
		t.Fatal("expected error for empty releases")
	}
}

func TestTopologyValidate_AllowsSeveralOptimizeReleasesPerOrchestration(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "features/hub.yaml")
	writeValuesFile(t, dir, "features/orchestration.yaml")
	writeValuesFile(t, dir, "features/optimize.yaml")
	top := &Topology{
		Name: "two-tenants-one-cluster",
		Releases: []TopologyRelease{
			{Role: "hub", NamespaceSuffix: "hub", Features: []string{"hub"}},
			{Role: "orchestration", NamespaceSuffix: "orcha", Features: []string{"orchestration"}, ModelerClusterID: "orcha", ModelerClusterName: "Orchestration A", DependsOn: "hub"},
			{Role: "optimize", NamespaceSuffix: "opta1", Serves: "orcha", OptimizeContextPath: "/optimize-tenanta", Features: []string{"optimize"}, DependsOn: "hub"},
			{Role: "optimize", NamespaceSuffix: "opta2", Serves: "orcha", OptimizeContextPath: "/optimize-tenantb", Features: []string{"optimize"}, DependsOn: "hub"},
		},
	}

	if err := top.Validate("ctx", dir, t.TempDir()); err != nil {
		t.Fatalf("several optimize releases may serve one orchestration release, got: %v", err)
	}
}

func TestTopologyValidate_RejectsDuplicateOptimizeContextPath(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "features/hub.yaml")
	writeValuesFile(t, dir, "features/orchestration.yaml")
	writeValuesFile(t, dir, "features/optimize.yaml")
	top := &Topology{
		Name: "colliding-optimize-paths",
		Releases: []TopologyRelease{
			{Role: "hub", NamespaceSuffix: "hub", Features: []string{"hub"}},
			{Role: "orchestration", NamespaceSuffix: "orcha", Features: []string{"orchestration"}, ModelerClusterID: "orcha", ModelerClusterName: "Orchestration A", DependsOn: "hub"},
			{Role: "optimize", NamespaceSuffix: "opta1", Serves: "orcha", OptimizeContextPath: "/optimize", Features: []string{"optimize"}, DependsOn: "hub"},
			{Role: "optimize", NamespaceSuffix: "opta2", Serves: "orcha", OptimizeContextPath: "/optimize", Features: []string{"optimize"}, DependsOn: "hub"},
		},
	}

	err := top.Validate("ctx", dir, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "share optimize-context-path") {
		t.Fatalf("expected colliding optimize-context-path to be rejected, got %v", err)
	}
}

func TestTopologyValidate_RejectsLegacyValuesKey(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "features/hub.yaml")
	writeValuesFile(t, dir, "features/orchestration.yaml")
	top := &Topology{
		Name: "legacy-values-key",
		Releases: []TopologyRelease{
			{Role: "hub", NamespaceSuffix: "hub", Features: []string{"hub"}},
			{Role: "orchestration", NamespaceSuffix: "orcha", Features: []string{"orchestration"}, ModelerClusterID: "orcha", ModelerClusterName: "Orchestration A", DependsOn: "hub", Values: "features/orchestration.yaml"},
		},
	}

	err := top.Validate("ctx", dir, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "is no longer supported") {
		t.Fatalf("expected the legacy values key to be rejected, got %v", err)
	}
}

func TestTopologyValidate_RequiresFeatures(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "features/hub.yaml")
	top := &Topology{
		Name: "no-features",
		Releases: []TopologyRelease{
			{Role: "hub", NamespaceSuffix: "hub", Features: []string{"hub"}},
			{Role: "orchestration", NamespaceSuffix: "orcha", ModelerClusterID: "orcha", ModelerClusterName: "Orchestration A", DependsOn: "hub"},
		},
	}

	err := top.Validate("ctx", dir, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "features is required") {
		t.Fatalf("expected features to be required, got %v", err)
	}
}

func TestTopologyValidate_RejectsMissingFeatureFile(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "features/hub.yaml")
	top := &Topology{
		Name: "missing-feature-file",
		Releases: []TopologyRelease{
			{Role: "hub", NamespaceSuffix: "hub", Features: []string{"hub"}},
			{Role: "orchestration", NamespaceSuffix: "orcha", Features: []string{"typo-orchestration"}, ModelerClusterID: "orcha", ModelerClusterName: "Orchestration A", DependsOn: "hub"},
		},
	}

	err := top.Validate("ctx", dir, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), `feature "typo-orchestration": missing values file`) {
		t.Fatalf("expected a missing feature layer to be reported, got %v", err)
	}
}
