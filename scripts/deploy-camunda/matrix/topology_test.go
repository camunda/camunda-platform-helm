package matrix

import (
	"os"
	"path/filepath"
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
	writeValuesFile(t, dir, "management.yaml")
	writeValuesFile(t, dir, "orchestration.yaml")
	writeValuesFile(t, dir, "identity/keycloak.yaml")
	writeValuesFile(t, dir, "identity/keycloak-external.yaml")
	writeValuesFile(t, dir, "persistence/elasticsearch-external.yaml")
	writeDepFile(t, depsDir, "keycloak")
	writeDepFile(t, depsDir, "postgresql")
	writeDepFile(t, depsDir, "elasticsearch")

	top := &Topology{
		Name:          "mgmt-2orch",
		SharedStorage: "elasticsearch",
		Releases: []TopologyRelease{
			{
				Role:            "management",
				NamespaceSuffix: "mgmt",
				Values:          "management.yaml",
				Identity:        "keycloak",
				Dependencies:    []string{"keycloak", "postgresql", "elasticsearch"},
			},
			{
				Role:            "orchestration",
				NamespaceSuffix: "orcha",
				Values:          "orchestration.yaml",
				Identity:        "keycloak-external",
				Persistence:     "elasticsearch-external",
				DependsOn:       "management",
			},
			{
				Role:            "orchestration",
				NamespaceSuffix: "orchb",
				Values:          "orchestration.yaml",
				Identity:        "keycloak-external",
				Persistence:     "elasticsearch-external",
				DependsOn:       "management",
			},
		},
	}
	if err := top.Validate("ctx", dir, depsDir); err != nil {
		t.Fatalf("expected valid topology, got: %v", err)
	}
}

func TestTopologyValidate_MissingValuesFile(t *testing.T) {
	dir := t.TempDir()
	top := &Topology{
		Name: "mgmt-1orch",
		Releases: []TopologyRelease{
			{Role: "management", NamespaceSuffix: "mgmt", Values: "management.yaml"},
			{Role: "orchestration", NamespaceSuffix: "orcha", Values: "orchestration.yaml"},
		},
	}
	if err := top.Validate("ctx", dir, t.TempDir()); err == nil {
		t.Fatal("expected error for missing values files")
	}
}

func TestTopologyValidate_MissingIdentityLayer(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "management.yaml")
	top := &Topology{
		Name: "bad-identity",
		Releases: []TopologyRelease{
			{Role: "management", NamespaceSuffix: "mgmt", Values: "management.yaml", Identity: "does-not-exist"},
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
			{Role: "management", NamespaceSuffix: "mgmt", Values: "orchestration.yaml", Persistence: "does-not-exist"},
		},
	}
	if err := top.Validate("ctx", dir, t.TempDir()); err == nil {
		t.Fatal("expected error for missing persistence layer file")
	}
}

func TestTopologyValidate_MissingDependency(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "management.yaml")
	top := &Topology{
		Name: "bad-dep",
		Releases: []TopologyRelease{
			{Role: "management", NamespaceSuffix: "mgmt", Values: "management.yaml", Dependencies: []string{"does-not-exist"}},
		},
	}
	if err := top.Validate("ctx", dir, filepath.Join(t.TempDir(), "dependencies")); err == nil {
		t.Fatal("expected error for missing dependency file")
	}
}

func TestTopologyValidate_NoManagementRole(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "orchestration.yaml")
	top := &Topology{
		Name: "no-mgmt",
		Releases: []TopologyRelease{
			{Role: "orchestration", NamespaceSuffix: "orcha", Values: "orchestration.yaml"},
			{Role: "orchestration", NamespaceSuffix: "orchb", Values: "orchestration.yaml"},
		},
	}
	if err := top.Validate("ctx", dir, t.TempDir()); err == nil {
		t.Fatal("expected error for missing management role")
	}
}

func TestTopologyValidate_TwoManagementRoles(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "management.yaml")
	top := &Topology{
		Name: "two-mgmt",
		Releases: []TopologyRelease{
			{Role: "management", NamespaceSuffix: "mgmta", Values: "management.yaml"},
			{Role: "management", NamespaceSuffix: "mgmtb", Values: "management.yaml"},
		},
	}
	if err := top.Validate("ctx", dir, t.TempDir()); err == nil {
		t.Fatal("expected error for two management roles")
	}
}

func TestTopologyValidate_DuplicateNamespaceSuffix(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "management.yaml")
	writeValuesFile(t, dir, "orchestration.yaml")
	top := &Topology{
		Name: "dup-suffix",
		Releases: []TopologyRelease{
			{Role: "management", NamespaceSuffix: "a", Values: "management.yaml"},
			{Role: "orchestration", NamespaceSuffix: "a", Values: "orchestration.yaml"},
		},
	}
	if err := top.Validate("ctx", dir, t.TempDir()); err == nil {
		t.Fatal("expected error for duplicate namespace-suffix")
	}
}

func TestTopologyValidate_DependsOnUnknownRole(t *testing.T) {
	dir := t.TempDir()
	writeValuesFile(t, dir, "management.yaml")
	writeValuesFile(t, dir, "orchestration.yaml")
	top := &Topology{
		Name: "bad-depends-on",
		Releases: []TopologyRelease{
			{Role: "management", NamespaceSuffix: "mgmt", Values: "management.yaml"},
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
