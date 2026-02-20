package scenarios

import (
	"os"
	"path/filepath"
	"testing"
)

// createScenarioFile is a test helper that creates a scenario values file
// with the standard naming convention.
func createScenarioFile(t *testing.T, dir, scenario string) string {
	t.Helper()
	filename := ValuesFilePrefix + scenario + ".yaml"
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, []byte("# "+scenario), 0644); err != nil {
		t.Fatalf("failed to create scenario file %s: %v", path, err)
	}
	return path
}

func TestResolvePath(t *testing.T) {
	tests := []struct {
		name      string
		scenario  string
		setup     func(t *testing.T, dir string) string // returns expected path
		wantErr   bool
		errSubstr string
	}{
		{
			name:     "resolves short scenario name",
			scenario: "keycloak",
			setup: func(t *testing.T, dir string) string {
				return createScenarioFile(t, dir, "keycloak")
			},
		},
		{
			name:     "resolves full filename",
			scenario: "values-integration-test-ingress-keycloak.yaml",
			setup: func(t *testing.T, dir string) string {
				return createScenarioFile(t, dir, "keycloak")
			},
		},
		{
			name:     "resolves scenario with hyphens",
			scenario: "keycloak-mt",
			setup: func(t *testing.T, dir string) string {
				return createScenarioFile(t, dir, "keycloak-mt")
			},
		},
		{
			name:     "resolves scenario with multiple hyphens",
			scenario: "qa-elasticsearch",
			setup: func(t *testing.T, dir string) string {
				return createScenarioFile(t, dir, "qa-elasticsearch")
			},
		},
		{
			name:     "resolves gateway-keycloak scenario",
			scenario: "gateway-keycloak",
			setup: func(t *testing.T, dir string) string {
				return createScenarioFile(t, dir, "gateway-keycloak")
			},
		},
		{
			name:      "returns error for nonexistent scenario",
			scenario:  "nonexistent",
			setup:     func(t *testing.T, dir string) string { return "" },
			wantErr:   true,
			errSubstr: "not found",
		},
		{
			name:      "returns error for empty scenario name",
			scenario:  "",
			setup:     func(t *testing.T, dir string) string { return "" },
			wantErr:   true,
			errSubstr: "not found",
		},
		{
			name:      "returns error for nonexistent directory",
			scenario:  "keycloak",
			setup:     nil, // don't create the dir
			wantErr:   true,
			errSubstr: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var dir string
			if tt.setup != nil {
				dir = t.TempDir()
				expectedPath := tt.setup(t, dir)
				got, err := ResolvePath(dir, tt.scenario)
				if tt.wantErr {
					if err == nil {
						t.Errorf("ResolvePath() expected error, got nil")
					} else if tt.errSubstr != "" && !containsStr(err.Error(), tt.errSubstr) {
						t.Errorf("ResolvePath() error = %q, want substring %q", err.Error(), tt.errSubstr)
					}
					return
				}
				if err != nil {
					t.Fatalf("ResolvePath() unexpected error: %v", err)
				}
				if got != expectedPath {
					t.Errorf("ResolvePath() = %q, want %q", got, expectedPath)
				}
			} else {
				// Use a nonexistent directory
				got, err := ResolvePath("/nonexistent/dir", tt.scenario)
				if !tt.wantErr {
					t.Fatalf("expected wantErr for nil setup")
				}
				if err == nil {
					t.Errorf("ResolvePath() expected error for nonexistent dir, got path %q", got)
				}
			}
		})
	}
}

func TestResolvePathBackwardCompatibility(t *testing.T) {
	// This test validates that the old naming convention (values-integration-test-ingress-<name>.yaml)
	// is preserved. This is critical because CI workflows, the Taskfile, and the deploy-camunda CLI
	// all rely on this naming convention.

	dir := t.TempDir()

	// Create files matching the old convention
	oldScenarios := []string{
		"keycloak",
		"elasticsearch",
		"opensearch",
		"oidc",
		"qa-license",
		"qa-elasticsearch",
		"keycloak-mt",
		"gateway-keycloak",
		"keycloak-rdbms",
		"hybrid",
		"infra",
	}

	for _, s := range oldScenarios {
		createScenarioFile(t, dir, s)
	}

	// Verify each scenario can be resolved by short name
	for _, s := range oldScenarios {
		t.Run("resolve_"+s, func(t *testing.T) {
			path, err := ResolvePath(dir, s)
			if err != nil {
				t.Fatalf("ResolvePath(%q) failed: %v", s, err)
			}
			expectedFile := filepath.Join(dir, ValuesFilePrefix+s+".yaml")
			if path != expectedFile {
				t.Errorf("ResolvePath(%q) = %q, want %q", s, path, expectedFile)
			}
		})
	}

	// Verify each scenario can be resolved by full filename
	for _, s := range oldScenarios {
		t.Run("resolve_full_"+s, func(t *testing.T) {
			fullName := ValuesFilePrefix + s + ".yaml"
			path, err := ResolvePath(dir, fullName)
			if err != nil {
				t.Fatalf("ResolvePath(%q) failed: %v", fullName, err)
			}
			expectedFile := filepath.Join(dir, fullName)
			if path != expectedFile {
				t.Errorf("ResolvePath(%q) = %q, want %q", fullName, path, expectedFile)
			}
		})
	}
}

func TestList(t *testing.T) {
	t.Run("lists all scenarios", func(t *testing.T) {
		dir := t.TempDir()
		createScenarioFile(t, dir, "keycloak")
		createScenarioFile(t, dir, "elasticsearch")
		createScenarioFile(t, dir, "opensearch")

		// Also create non-scenario files that should be ignored
		os.WriteFile(filepath.Join(dir, "values-integration-test.yaml"), []byte("# common"), 0644)
		os.WriteFile(filepath.Join(dir, "values-enterprise.yaml"), []byte("# enterprise"), 0644)
		os.WriteFile(filepath.Join(dir, "README.md"), []byte("# readme"), 0644)
		os.Mkdir(filepath.Join(dir, "subdir"), 0755)

		got, err := List(dir)
		if err != nil {
			t.Fatalf("List() unexpected error: %v", err)
		}

		want := []string{"elasticsearch", "keycloak", "opensearch"}
		if !strSliceEqual(got, want) {
			t.Errorf("List() = %v, want %v", got, want)
		}
	})

	t.Run("returns empty for directory with no scenarios", func(t *testing.T) {
		dir := t.TempDir()
		// Create files that don't match the scenario prefix
		os.WriteFile(filepath.Join(dir, "values-integration-test.yaml"), []byte("# common"), 0644)
		os.WriteFile(filepath.Join(dir, "values-enterprise.yaml"), []byte("# enterprise"), 0644)

		got, err := List(dir)
		if err != nil {
			t.Fatalf("List() unexpected error: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("List() = %v, want empty slice", got)
		}
	})

	t.Run("returns empty for empty directory", func(t *testing.T) {
		dir := t.TempDir()
		got, err := List(dir)
		if err != nil {
			t.Fatalf("List() unexpected error: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("List() = %v, want empty slice", got)
		}
	})

	t.Run("returns error for nonexistent directory", func(t *testing.T) {
		_, err := List("/nonexistent/dir")
		if err == nil {
			t.Error("List() expected error for nonexistent directory")
		}
	})

	t.Run("ignores directories matching prefix pattern", func(t *testing.T) {
		dir := t.TempDir()
		createScenarioFile(t, dir, "keycloak")
		// Create a directory that matches the naming pattern (should be ignored)
		os.Mkdir(filepath.Join(dir, ValuesFilePrefix+"fake-dir.yaml"), 0755)

		got, err := List(dir)
		if err != nil {
			t.Fatalf("List() unexpected error: %v", err)
		}

		want := []string{"keycloak"}
		if !strSliceEqual(got, want) {
			t.Errorf("List() = %v, want %v", got, want)
		}
	})

	t.Run("ignores .yml extension files", func(t *testing.T) {
		dir := t.TempDir()
		createScenarioFile(t, dir, "keycloak")
		// Create a .yml file matching the prefix (should be ignored — convention is .yaml only)
		os.WriteFile(filepath.Join(dir, ValuesFilePrefix+"invalid.yml"), []byte("# yml"), 0644)

		got, err := List(dir)
		if err != nil {
			t.Fatalf("List() unexpected error: %v", err)
		}

		want := []string{"keycloak"}
		if !strSliceEqual(got, want) {
			t.Errorf("List() = %v, want %v", got, want)
		}
	})
}

func TestValuesFilePrefix(t *testing.T) {
	// Ensure the prefix constant hasn't changed — this is a backward compatibility invariant
	expected := "values-integration-test-ingress-"
	if ValuesFilePrefix != expected {
		t.Errorf("ValuesFilePrefix = %q, want %q — changing this breaks backward compatibility", ValuesFilePrefix, expected)
	}
}

// strSliceEqual compares two sorted string slices.
func strSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
