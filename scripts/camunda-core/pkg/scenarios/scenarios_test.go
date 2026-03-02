package scenarios

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMapScenarioToConfig(t *testing.T) {
	tests := []struct {
		name            string
		scenario        string
		wantIdentity    string
		wantPersistence string
		wantPlatform    string
		wantFeatures    []string
		wantQA          bool
		wantUpgrade     bool
	}{
		// Well-known composite scenarios
		{
			name:            "keycloak-original maps to external keycloak + external elasticsearch",
			scenario:        "keycloak-original",
			wantIdentity:    "keycloak-external",
			wantPersistence: "elasticsearch-external",
			wantPlatform:    "gke",
		},
		{
			name:            "keycloak-original case insensitive",
			scenario:        "Keycloak-Original",
			wantIdentity:    "keycloak-external",
			wantPersistence: "elasticsearch-external",
			wantPlatform:    "gke",
		},

		// Standard identity derivation
		{
			name:            "elasticsearch defaults to keycloak + elasticsearch",
			scenario:        "elasticsearch",
			wantIdentity:    "keycloak",
			wantPersistence: "elasticsearch",
			wantPlatform:    "gke",
		},
		{
			name:            "keycloak-mt maps to keycloak-external with multitenancy",
			scenario:        "keycloak-mt",
			wantIdentity:    "keycloak-external",
			wantPersistence: "elasticsearch",
			wantPlatform:    "gke",
			wantFeatures:    []string{"multitenancy"},
		},
		{
			name:            "multitenancy maps to keycloak-external with multitenancy",
			scenario:        "multitenancy",
			wantIdentity:    "keycloak-external",
			wantPersistence: "elasticsearch",
			wantPlatform:    "gke",
			wantFeatures:    []string{"multitenancy"},
		},
		{
			name:            "oidc maps to oidc identity",
			scenario:        "oidc",
			wantIdentity:    "oidc",
			wantPersistence: "elasticsearch",
			wantPlatform:    "gke",
		},
		{
			name:            "elasticsearch-basic maps to basic identity",
			scenario:        "elasticsearch-basic",
			wantIdentity:    "basic",
			wantPersistence: "elasticsearch",
			wantPlatform:    "gke",
		},
		{
			name:            "hybrid maps to hybrid identity",
			scenario:        "hybrid",
			wantIdentity:    "hybrid",
			wantPersistence: "elasticsearch",
			wantPlatform:    "gke",
		},

		// Persistence derivation
		{
			name:            "opensearch maps to opensearch persistence",
			scenario:        "opensearch",
			wantIdentity:    "keycloak",
			wantPersistence: "opensearch",
			wantPlatform:    "gke",
		},
		{
			name:            "rdbms maps to rdbms persistence",
			scenario:        "rdbms",
			wantIdentity:    "keycloak",
			wantPersistence: "rdbms",
			wantPlatform:    "gke",
		},
		{
			name:            "rdbms-oracle maps to rdbms-oracle persistence",
			scenario:        "rdbms-oracle",
			wantIdentity:    "keycloak",
			wantPersistence: "rdbms-oracle",
			wantPlatform:    "gke",
		},

		// Platform derivation
		{
			name:            "eks maps to eks platform",
			scenario:        "elasticsearch-eks",
			wantIdentity:    "keycloak",
			wantPersistence: "elasticsearch",
			wantPlatform:    "eks",
		},
		{
			name:            "openshift maps to openshift platform",
			scenario:        "elasticsearch-openshift",
			wantIdentity:    "keycloak",
			wantPersistence: "elasticsearch",
			wantPlatform:    "openshift",
		},
		{
			name:            "rosa maps to openshift platform",
			scenario:        "elasticsearch-rosa",
			wantIdentity:    "keycloak",
			wantPersistence: "elasticsearch",
			wantPlatform:    "openshift",
		},

		// Feature derivation
		{
			name:            "keycloak-rba maps to rba feature",
			scenario:        "keycloak-rba",
			wantIdentity:    "keycloak",
			wantPersistence: "elasticsearch",
			wantPlatform:    "gke",
			wantFeatures:    []string{"rba"},
		},
		{
			name:            "documentstore maps to documentstore feature",
			scenario:        "documentstore",
			wantIdentity:    "keycloak",
			wantPersistence: "elasticsearch",
			wantPlatform:    "gke",
			wantFeatures:    []string{"documentstore"},
		},

		// QA and upgrade modifiers
		{
			name:            "qa- prefix enables QA mode",
			scenario:        "qa-elasticsearch",
			wantIdentity:    "keycloak",
			wantPersistence: "elasticsearch",
			wantPlatform:    "gke",
			wantQA:          true,
		},
		{
			name:            "upgrade in name enables upgrade mode",
			scenario:        "upgrade-migration",
			wantIdentity:    "keycloak",
			wantPersistence: "elasticsearch",
			wantPlatform:    "gke",
			wantUpgrade:     true,
		},

		// Additional coverage for triggers not exercised above
		{
			name:            "entra maps to oidc identity",
			scenario:        "entra",
			wantIdentity:    "oidc",
			wantPersistence: "elasticsearch",
			wantPlatform:    "gke",
		},
		{
			name:            "-mt- trigger maps to keycloak-external with multitenancy",
			scenario:        "foo-mt-bar",
			wantIdentity:    "keycloak-external",
			wantPersistence: "elasticsearch",
			wantPlatform:    "gke",
			wantFeatures:    []string{"multitenancy"},
		},
		{
			name:            "-upg trigger enables upgrade mode",
			scenario:        "elasticsearch-upg",
			wantIdentity:    "keycloak",
			wantPersistence: "elasticsearch",
			wantPlatform:    "gke",
			wantUpgrade:     true,
		},
		{
			name:            "combined multi-feature: mt + documentstore",
			scenario:        "keycloak-mt-document",
			wantIdentity:    "keycloak-external",
			wantPersistence: "elasticsearch",
			wantPlatform:    "gke",
			wantFeatures:    []string{"multitenancy", "documentstore"},
		},
		{
			name:            "combined qa + opensearch + eks",
			scenario:        "qa-opensearch-eks",
			wantIdentity:    "keycloak",
			wantPersistence: "opensearch",
			wantPlatform:    "eks",
			wantQA:          true,
		},
		{
			name:            "combined upgrade + openshift + rdbms",
			scenario:        "upgrade-rdbms-openshift",
			wantIdentity:    "keycloak",
			wantPersistence: "rdbms",
			wantPlatform:    "openshift",
			wantUpgrade:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := MapScenarioToConfig(tt.scenario)

			if config.Identity != tt.wantIdentity {
				t.Errorf("Identity = %q, want %q", config.Identity, tt.wantIdentity)
			}
			if config.Persistence != tt.wantPersistence {
				t.Errorf("Persistence = %q, want %q", config.Persistence, tt.wantPersistence)
			}
			if config.Platform != tt.wantPlatform {
				t.Errorf("Platform = %q, want %q", config.Platform, tt.wantPlatform)
			}
			if config.QA != tt.wantQA {
				t.Errorf("QA = %v, want %v", config.QA, tt.wantQA)
			}
			if config.Upgrade != tt.wantUpgrade {
				t.Errorf("Upgrade = %v, want %v", config.Upgrade, tt.wantUpgrade)
			}

			// Compare features
			if len(config.Features) != len(tt.wantFeatures) {
				t.Errorf("Features = %v, want %v", config.Features, tt.wantFeatures)
			} else {
				for i, f := range config.Features {
					if f != tt.wantFeatures[i] {
						t.Errorf("Features[%d] = %q, want %q", i, f, tt.wantFeatures[i])
					}
				}
			}
		})
	}
}

func TestDeploymentConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  DeploymentConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: DeploymentConfig{
				Identity:    "keycloak",
				Persistence: "elasticsearch",
				Platform:    "gke",
			},
		},
		{
			name: "valid config with elasticsearch-external",
			config: DeploymentConfig{
				Identity:    "keycloak-external",
				Persistence: "elasticsearch-external",
				Platform:    "gke",
			},
		},
		{
			name: "missing identity",
			config: DeploymentConfig{
				Persistence: "elasticsearch",
				Platform:    "gke",
			},
			wantErr: true,
		},
		{
			name: "invalid persistence",
			config: DeploymentConfig{
				Identity:    "keycloak",
				Persistence: "mongodb",
				Platform:    "gke",
			},
			wantErr: true,
		},
		{
			name: "multitenancy and rba conflict",
			config: DeploymentConfig{
				Identity:    "keycloak",
				Persistence: "elasticsearch",
				Platform:    "gke",
				Features:    []string{"multitenancy", "rba"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDeploymentConfigResolvePaths(t *testing.T) {
	// Create a temporary directory structure to test path resolution
	tmpDir := t.TempDir()

	// Create the expected directory structure
	dirs := []string{
		filepath.Join(tmpDir, "values", "identity"),
		filepath.Join(tmpDir, "values", "persistence"),
		filepath.Join(tmpDir, "values", "platform"),
		filepath.Join(tmpDir, "values", "features"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}

	// Create the files
	files := []string{
		filepath.Join(tmpDir, "values", "base.yaml"),
		filepath.Join(tmpDir, "values", "identity", "keycloak.yaml"),
		filepath.Join(tmpDir, "values", "identity", "keycloak-external.yaml"),
		filepath.Join(tmpDir, "values", "persistence", "elasticsearch.yaml"),
		filepath.Join(tmpDir, "values", "persistence", "elasticsearch-external.yaml"),
		filepath.Join(tmpDir, "values", "platform", "gke.yaml"),
		filepath.Join(tmpDir, "values", "features", "multitenancy.yaml"),
	}
	for _, f := range files {
		if err := os.WriteFile(f, []byte("# test"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("keycloak-original config resolves to keycloak-external + elasticsearch-external", func(t *testing.T) {
		config := MapScenarioToConfig("keycloak-original")
		paths, err := config.ResolvePaths(tmpDir)
		if err != nil {
			t.Fatalf("ResolvePaths() error = %v", err)
		}

		// Should contain: base.yaml, keycloak-external.yaml, elasticsearch-external.yaml, gke.yaml
		if len(paths) != 4 {
			t.Fatalf("Expected 4 paths, got %d: %v", len(paths), paths)
		}

		expectedSuffixes := []string{
			"values/base.yaml",
			"values/identity/keycloak-external.yaml",
			"values/persistence/elasticsearch-external.yaml",
			"values/platform/gke.yaml",
		}
		for i, suffix := range expectedSuffixes {
			if !containsSuffix(paths[i], suffix) {
				t.Errorf("paths[%d] = %q, want suffix %q", i, paths[i], suffix)
			}
		}
	})

	t.Run("standard keycloak config resolves normally", func(t *testing.T) {
		config := MapScenarioToConfig("elasticsearch")
		paths, err := config.ResolvePaths(tmpDir)
		if err != nil {
			t.Fatalf("ResolvePaths() error = %v", err)
		}

		// Should contain: base.yaml, keycloak.yaml, elasticsearch.yaml, gke.yaml
		if len(paths) != 4 {
			t.Fatalf("Expected 4 paths, got %d: %v", len(paths), paths)
		}

		expectedSuffixes := []string{
			"values/base.yaml",
			"values/identity/keycloak.yaml",
			"values/persistence/elasticsearch.yaml",
			"values/platform/gke.yaml",
		}
		for i, suffix := range expectedSuffixes {
			if !containsSuffix(paths[i], suffix) {
				t.Errorf("paths[%d] = %q, want suffix %q", i, paths[i], suffix)
			}
		}
	})

	t.Run("multitenancy config includes feature", func(t *testing.T) {
		config := MapScenarioToConfig("keycloak-mt")
		paths, err := config.ResolvePaths(tmpDir)
		if err != nil {
			t.Fatalf("ResolvePaths() error = %v", err)
		}

		// Should contain: base.yaml, keycloak-external.yaml, elasticsearch.yaml, gke.yaml, multitenancy.yaml
		if len(paths) != 5 {
			t.Fatalf("Expected 5 paths, got %d: %v", len(paths), paths)
		}

		expectedSuffixes := []string{
			"values/base.yaml",
			"values/identity/keycloak-external.yaml",
			"values/persistence/elasticsearch.yaml",
			"values/platform/gke.yaml",
			"values/features/multitenancy.yaml",
		}
		for i, suffix := range expectedSuffixes {
			if !containsSuffix(paths[i], suffix) {
				t.Errorf("paths[%d] = %q, want suffix %q", i, paths[i], suffix)
			}
		}
	})
}

func TestExpandImports(t *testing.T) {
	t.Run("basic import expansion", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create a.yaml that imports b
		writeYAML(t, filepath.Join(tmpDir, "a.yaml"), "imports:\n  - b\nkey: from-a\n")
		writeYAML(t, filepath.Join(tmpDir, "b.yaml"), "key: from-b\n")

		result, err := ExpandImports([]string{filepath.Join(tmpDir, "a.yaml")})
		if err != nil {
			t.Fatal(err)
		}
		if len(result) != 2 {
			t.Fatalf("expected 2 files, got %d: %v", len(result), result)
		}
		// b.yaml should come before a.yaml
		if !containsSuffix(result[0], "b.yaml") {
			t.Errorf("result[0] = %q, want b.yaml", result[0])
		}
		if !containsSuffix(result[1], "a.yaml") {
			t.Errorf("result[1] = %q, want a.yaml", result[1])
		}
	})

	t.Run("transitive imports (depth-first)", func(t *testing.T) {
		tmpDir := t.TempDir()

		// a imports b, b imports c
		writeYAML(t, filepath.Join(tmpDir, "a.yaml"), "imports:\n  - b\nkey: from-a\n")
		writeYAML(t, filepath.Join(tmpDir, "b.yaml"), "imports:\n  - c\nkey: from-b\n")
		writeYAML(t, filepath.Join(tmpDir, "c.yaml"), "key: from-c\n")

		result, err := ExpandImports([]string{filepath.Join(tmpDir, "a.yaml")})
		if err != nil {
			t.Fatal(err)
		}
		if len(result) != 3 {
			t.Fatalf("expected 3 files, got %d: %v", len(result), result)
		}
		// Order: c, b, a (depth-first, imports before importer)
		wantSuffixes := []string{"c.yaml", "b.yaml", "a.yaml"}
		for i, want := range wantSuffixes {
			if !containsSuffix(result[i], want) {
				t.Errorf("result[%d] = %q, want suffix %q", i, result[i], want)
			}
		}
	})

	t.Run("circular import detection", func(t *testing.T) {
		tmpDir := t.TempDir()

		// a imports b, b imports a
		writeYAML(t, filepath.Join(tmpDir, "a.yaml"), "imports:\n  - b\n")
		writeYAML(t, filepath.Join(tmpDir, "b.yaml"), "imports:\n  - a\n")

		_, err := ExpandImports([]string{filepath.Join(tmpDir, "a.yaml")})
		if err == nil {
			t.Fatal("expected circular import error, got nil")
		}
		if !contains([]string{err.Error()}, "circular import detected: "+filepath.Join(tmpDir, "a.yaml")) {
			// Check that the error message mentions circular import
			if !containsSubstring(err.Error(), "circular import") {
				t.Errorf("expected error about circular import, got: %v", err)
			}
		}
	})

	t.Run("missing import error", func(t *testing.T) {
		tmpDir := t.TempDir()

		writeYAML(t, filepath.Join(tmpDir, "a.yaml"), "imports:\n  - nonexistent\n")

		_, err := ExpandImports([]string{filepath.Join(tmpDir, "a.yaml")})
		if err == nil {
			t.Fatal("expected missing import error, got nil")
		}
		if !containsSubstring(err.Error(), "not found") {
			t.Errorf("expected error about not found, got: %v", err)
		}
	})

	t.Run("deduplication across multiple importers", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Both a and b import c; input list is [a, b]
		writeYAML(t, filepath.Join(tmpDir, "a.yaml"), "imports:\n  - c\nkey: from-a\n")
		writeYAML(t, filepath.Join(tmpDir, "b.yaml"), "imports:\n  - c\nkey: from-b\n")
		writeYAML(t, filepath.Join(tmpDir, "c.yaml"), "key: from-c\n")

		result, err := ExpandImports([]string{
			filepath.Join(tmpDir, "a.yaml"),
			filepath.Join(tmpDir, "b.yaml"),
		})
		if err != nil {
			t.Fatal(err)
		}
		// c should appear only once, before a and b
		if len(result) != 3 {
			t.Fatalf("expected 3 files (c deduplicated), got %d: %v", len(result), result)
		}
		wantSuffixes := []string{"c.yaml", "a.yaml", "b.yaml"}
		for i, want := range wantSuffixes {
			if !containsSuffix(result[i], want) {
				t.Errorf("result[%d] = %q, want suffix %q", i, result[i], want)
			}
		}
	})

	t.Run("no imports passes through unchanged", func(t *testing.T) {
		tmpDir := t.TempDir()

		writeYAML(t, filepath.Join(tmpDir, "a.yaml"), "key: value\n")
		writeYAML(t, filepath.Join(tmpDir, "b.yaml"), "key: value\n")

		input := []string{filepath.Join(tmpDir, "a.yaml"), filepath.Join(tmpDir, "b.yaml")}
		result, err := ExpandImports(input)
		if err != nil {
			t.Fatal(err)
		}
		if len(result) != 2 {
			t.Fatalf("expected 2 files, got %d: %v", len(result), result)
		}
		for i := range input {
			if result[i] != input[i] {
				t.Errorf("result[%d] = %q, want %q", i, result[i], input[i])
			}
		}
	})
}

func TestDeploymentConfigResolvePathsWithImports(t *testing.T) {
	tmpDir := t.TempDir()

	// Create the expected directory structure
	dirs := []string{
		filepath.Join(tmpDir, "values", "identity"),
		filepath.Join(tmpDir, "values", "persistence"),
		filepath.Join(tmpDir, "values", "platform"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}

	// Create files — hybrid.yaml imports keycloak-external
	writeYAML(t, filepath.Join(tmpDir, "values", "base.yaml"), "base: true\n")
	writeYAML(t, filepath.Join(tmpDir, "values", "identity", "keycloak-external.yaml"), "identity: keycloak-external\n")
	writeYAML(t, filepath.Join(tmpDir, "values", "identity", "hybrid.yaml"),
		"imports:\n  - keycloak-external\nidentity: hybrid\n")
	writeYAML(t, filepath.Join(tmpDir, "values", "persistence", "elasticsearch.yaml"), "persistence: elasticsearch\n")
	writeYAML(t, filepath.Join(tmpDir, "values", "platform", "gke.yaml"), "platform: gke\n")

	config := &DeploymentConfig{
		Identity:    "hybrid",
		Persistence: "elasticsearch",
		Platform:    "gke",
	}
	paths, err := config.ResolvePaths(tmpDir)
	if err != nil {
		t.Fatalf("ResolvePaths() error = %v", err)
	}

	// Expected: base, keycloak-external (imported), hybrid, elasticsearch, gke
	if len(paths) != 5 {
		t.Fatalf("expected 5 paths, got %d: %v", len(paths), paths)
	}

	expectedSuffixes := []string{
		"values/base.yaml",
		"values/identity/keycloak-external.yaml",
		"values/identity/hybrid.yaml",
		"values/persistence/elasticsearch.yaml",
		"values/platform/gke.yaml",
	}
	for i, suffix := range expectedSuffixes {
		if !containsSuffix(paths[i], suffix) {
			t.Errorf("paths[%d] = %q, want suffix %q", i, paths[i], suffix)
		}
	}
}

// writeYAML is a test helper that writes content to a file.
func writeYAML(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

// containsSubstring checks if s contains substr.
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && strings.Contains(s, substr)
}

// containsSuffix checks if s ends with suffix (using filepath separator).
func containsSuffix(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}
