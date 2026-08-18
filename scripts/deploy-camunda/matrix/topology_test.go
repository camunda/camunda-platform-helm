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
	writeValuesFile(t, dir, "hub.yaml")
	writeValuesFile(t, dir, "orchestration.yaml")
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
				Values:          "hub.yaml",
				Identity:        "keycloak",
				Dependencies:    []string{"keycloak", "postgresql", "elasticsearch"},
			},
			{
				Role:               "orchestration",
				NamespaceSuffix:    "orcha",
				ModelerClusterID:   "orcha",
				ModelerClusterName: "Orchestration A",
				Values:             "orchestration.yaml",
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
				Values:             "orchestration.yaml",
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
	writeValuesFile(t, dir, "hub.yaml")
	writeValuesFile(t, dir, "orchestration.yaml")
	writeValuesFile(t, dir, "optimize.yaml")
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
				Values:          "hub.yaml",
				Identity:        "keycloak",
				Dependencies:    []string{"elasticsearch"},
			},
			{
				Role:               "orchestration",
				NamespaceSuffix:    "orcha",
				ModelerClusterID:   "orcha",
				ModelerClusterName: "Orchestration A",
				Values:             "orchestration.yaml",
				Identity:           "keycloak-external",
				Persistence:        "elasticsearch-external",
				DependsOn:          "hub",
			},
			{
				Role:            "optimize",
				NamespaceSuffix: "opta",
				Values:          "optimize.yaml",
				Identity:        "keycloak-external",
				Persistence:     "elasticsearch-external",
				DependsOn:       "hub",
			},
			{
				Role:            "optimize",
				NamespaceSuffix: "optb",
				Values:          "optimize.yaml",
				Identity:        "keycloak-external",
				Persistence:     "elasticsearch-external",
				DependsOn:       "hub",
			},
		},
	}
	if err := top.Validate("ctx", dir, depsDir); err != nil {
		t.Fatalf("expected valid topology with optimize releases, got: %v", err)
	}
}

func TestTopologyValidate_OptimizeRoleNeedsNoModelerCluster(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "hub.yaml")
	writeValuesFile(t, dir, "orchestration.yaml")
	writeValuesFile(t, dir, "optimize.yaml")
	top := &Topology{
		Name: "optimize-no-modeler-cluster",
		Releases: []TopologyRelease{
			{Role: "hub", NamespaceSuffix: "hub", Values: "hub.yaml"},
			{Role: "orchestration", NamespaceSuffix: "orcha", Values: "orchestration.yaml", ModelerClusterID: "orcha", ModelerClusterName: "Orchestration A", DependsOn: "hub"},
			{Role: "optimize", NamespaceSuffix: "opta", Values: "optimize.yaml", DependsOn: "hub"},
		},
	}

	if err := top.Validate("ctx", dir, t.TempDir()); err != nil {
		t.Fatalf("optimize role must not require modeler-cluster-id/name, got: %v", err)
	}
}

func TestTopologyValidate_OptimizeRoleRequiresDependsOn(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "hub.yaml")
	writeValuesFile(t, dir, "orchestration.yaml")
	writeValuesFile(t, dir, "optimize.yaml")
	top := &Topology{
		Name: "optimize-without-depends-on",
		Releases: []TopologyRelease{
			{Role: "hub", NamespaceSuffix: "hub", Values: "hub.yaml"},
			{Role: "orchestration", NamespaceSuffix: "orcha", Values: "orchestration.yaml", ModelerClusterID: "orcha", ModelerClusterName: "Orchestration A", DependsOn: "hub"},
			{Role: "optimize", NamespaceSuffix: "opta", Values: "optimize.yaml"},
		},
	}

	err := top.Validate("ctx", dir, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "depends-on is required") {
		t.Fatalf("expected depends-on requirement for optimize role, got %v", err)
	}
}

func TestTopologyValidate_RequiresUniqueModelerClusters(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "hub.yaml")
	writeValuesFile(t, dir, "orchestration.yaml")
	top := &Topology{
		Name: "duplicate-modeler-cluster",
		Releases: []TopologyRelease{
			{Role: "hub", NamespaceSuffix: "hub", Values: "hub.yaml"},
			{Role: "orchestration", NamespaceSuffix: "orcha", Values: "orchestration.yaml", ModelerClusterID: "shared", ModelerClusterName: "Shared"},
			{Role: "orchestration", NamespaceSuffix: "orchb", Values: "orchestration.yaml", ModelerClusterID: "shared", ModelerClusterName: "Shared"},
		},
	}

	err := top.Validate("ctx", dir, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "duplicate modeler-cluster-id") || !strings.Contains(err.Error(), "duplicate modeler-cluster-name") {
		t.Fatalf("expected duplicate Modeler cluster validation errors, got %v", err)
	}
}

func TestTopologyValidate_RequiresOrchestrationRelease(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "hub.yaml")
	top := &Topology{
		Name: "hub-only",
		Releases: []TopologyRelease{
			{Role: "hub", NamespaceSuffix: "hub", Values: "hub.yaml"},
		},
	}

	err := top.Validate("ctx", dir, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "at least one release with role \"orchestration\"") {
		t.Fatalf("expected missing orchestration release error, got %v", err)
	}
}

func TestTopologyValidate_RequiresDNS1123NamespaceSuffix(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "hub.yaml")
	writeValuesFile(t, dir, "orchestration.yaml")
	top := &Topology{
		Name: "invalid-suffix",
		Releases: []TopologyRelease{
			{Role: "hub", NamespaceSuffix: "Hub", Values: "hub.yaml"},
			{Role: "orchestration", NamespaceSuffix: "orch_a", Values: "orchestration.yaml", ModelerClusterID: "orcha", ModelerClusterName: "Orchestration A"},
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
			{Role: "hub", NamespaceSuffix: "hub", Values: "hub.yaml"},
			{Role: "orchestration", NamespaceSuffix: "orcha", Values: "orchestration.yaml"},
		},
	}
	if err := top.Validate("ctx", dir, t.TempDir()); err == nil {
		t.Fatal("expected error for missing values files")
	}
}

func TestTopologyValidate_MissingIdentityLayer(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "hub.yaml")
	top := &Topology{
		Name: "bad-identity",
		Releases: []TopologyRelease{
			{Role: "hub", NamespaceSuffix: "hub", Values: "hub.yaml", Identity: "does-not-exist"},
		},
	}
	if err := top.Validate("ctx", dir, t.TempDir()); err == nil {
		t.Fatal("expected error for missing identity layer file")
	}
}

func TestTopologyValidate_MissingPersistenceLayer(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "orchestration.yaml")
	top := &Topology{
		Name: "bad-persistence",
		Releases: []TopologyRelease{
			{Role: "hub", NamespaceSuffix: "hub", Values: "orchestration.yaml", Persistence: "does-not-exist"},
		},
	}
	if err := top.Validate("ctx", dir, t.TempDir()); err == nil {
		t.Fatal("expected error for missing persistence layer file")
	}
}

func TestTopologyValidate_MissingDependency(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "hub.yaml")
	top := &Topology{
		Name: "bad-dep",
		Releases: []TopologyRelease{
			{Role: "hub", NamespaceSuffix: "hub", Values: "hub.yaml", Dependencies: []string{"does-not-exist"}},
		},
	}
	if err := top.Validate("ctx", dir, filepath.Join(t.TempDir(), "dependencies")); err == nil {
		t.Fatal("expected error for missing dependency file")
	}
}

func TestTopologyValidate_NoHubRole(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "orchestration.yaml")
	top := &Topology{
		Name: "no-hub",
		Releases: []TopologyRelease{
			{Role: "orchestration", NamespaceSuffix: "orcha", Values: "orchestration.yaml"},
			{Role: "orchestration", NamespaceSuffix: "orchb", Values: "orchestration.yaml"},
		},
	}
	if err := top.Validate("ctx", dir, t.TempDir()); err == nil {
		t.Fatal("expected error for missing Hub role")
	}
}

func TestTopologyValidate_TwoHubRoles(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "hub.yaml")
	top := &Topology{
		Name: "two-hub",
		Releases: []TopologyRelease{
			{Role: "hub", NamespaceSuffix: "huba", Values: "hub.yaml"},
			{Role: "hub", NamespaceSuffix: "hubb", Values: "hub.yaml"},
		},
	}
	if err := top.Validate("ctx", dir, t.TempDir()); err == nil {
		t.Fatal("expected error for two Hub roles")
	}
}

func TestTopologyValidate_DuplicateNamespaceSuffix(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "hub.yaml")
	writeValuesFile(t, dir, "orchestration.yaml")
	top := &Topology{
		Name: "dup-suffix",
		Releases: []TopologyRelease{
			{Role: "hub", NamespaceSuffix: "a", Values: "hub.yaml"},
			{Role: "orchestration", NamespaceSuffix: "a", Values: "orchestration.yaml"},
		},
	}
	if err := top.Validate("ctx", dir, t.TempDir()); err == nil {
		t.Fatal("expected error for duplicate namespace-suffix")
	}
}

func TestTopologyValidate_DependsOnUnknownRole(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "hub.yaml")
	writeValuesFile(t, dir, "orchestration.yaml")
	top := &Topology{
		Name: "bad-depends-on",
		Releases: []TopologyRelease{
			{Role: "hub", NamespaceSuffix: "hub", Values: "hub.yaml"},
			{Role: "orchestration", NamespaceSuffix: "orcha", Values: "orchestration.yaml", DependsOn: "storage"},
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
			{Role: "weird", NamespaceSuffix: "w", Values: "weird.yaml"},
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
