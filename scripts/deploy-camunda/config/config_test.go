package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolvePath(t *testing.T) {
	tests := []struct {
		name     string
		explicit string
		setup    func() string // returns expected path
		cleanup  func()
	}{
		{
			name:     "explicit path is returned as-is",
			explicit: "/custom/path/config.yaml",
			setup: func() string {
				return "/custom/path/config.yaml"
			},
		},
		{
			name:     "whitespace-only explicit returns local or user path",
			explicit: "   ",
			setup: func() string {
				// Should fall through to defaults
				home, _ := os.UserHomeDir()
				return filepath.Join(home, ".config", "camunda", "deploy.yaml")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expected := tt.setup()
			if tt.cleanup != nil {
				defer tt.cleanup()
			}

			result, err := ResolvePath(tt.explicit)
			if err != nil {
				t.Fatalf("ResolvePath() error = %v", err)
			}

			if result != expected {
				t.Errorf("ResolvePath(%q) = %q, want %q", tt.explicit, result, expected)
			}
		})
	}
}

func TestRead(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.yaml")

	configContent := `
current: test-deployment
repoRoot: /test/repo
platform: eks
logLevel: debug

deployments:
  test-deployment:
    chart: camunda-platform-8.8
    namespace: test-ns
    release: test-release
    scenario: keycloak
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Test reading the config
	rc, err := Read(configPath, false)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}

	// Verify fields
	if rc.Current != "test-deployment" {
		t.Errorf("Current = %q, want %q", rc.Current, "test-deployment")
	}
	if rc.RepoRoot != "/test/repo" {
		t.Errorf("RepoRoot = %q, want %q", rc.RepoRoot, "/test/repo")
	}
	if rc.Platform != "eks" {
		t.Errorf("Platform = %q, want %q", rc.Platform, "eks")
	}
	if rc.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, want %q", rc.LogLevel, "debug")
	}

	// Check deployment
	dep, ok := rc.Deployments["test-deployment"]
	if !ok {
		t.Fatal("Expected deployment 'test-deployment' not found")
	}
	if dep.Chart != "camunda-platform-8.8" {
		t.Errorf("Deployment.Chart = %q, want %q", dep.Chart, "camunda-platform-8.8")
	}
	if dep.Namespace != "test-ns" {
		t.Errorf("Deployment.Namespace = %q, want %q", dep.Namespace, "test-ns")
	}
}

func TestRead_MissingFile(t *testing.T) {
	// Reading a missing file should not error (returns empty config)
	rc, err := Read("/nonexistent/path/config.yaml", false)
	if err != nil {
		t.Fatalf("Read() error = %v, expected nil for missing file", err)
	}

	if rc == nil {
		t.Fatal("Read() returned nil config")
	}
}

func TestRead_WithEnvOverrides(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.yaml")

	configContent := `
platform: gke
logLevel: info
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Set environment variables
	os.Setenv("CAMUNDA_PLATFORM", "rosa")
	os.Setenv("CAMUNDA_LOG_LEVEL", "debug")
	defer func() {
		os.Unsetenv("CAMUNDA_PLATFORM")
		os.Unsetenv("CAMUNDA_LOG_LEVEL")
	}()

	// Test with env overrides
	rc, err := Read(configPath, true)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}

	if rc.Platform != "rosa" {
		t.Errorf("Platform = %q, want %q (env override)", rc.Platform, "rosa")
	}
	if rc.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, want %q (env override)", rc.LogLevel, "debug")
	}

	// Test without env overrides
	rc2, err := Read(configPath, false)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}

	if rc2.Platform != "gke" {
		t.Errorf("Platform = %q, want %q (no env override)", rc2.Platform, "gke")
	}
}

func TestWrite(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "output-config.yaml")

	rc := &RootConfig{
		FilePath: configPath,
		Current:  "my-deployment",
		Platform: "gke",
		Deployments: map[string]DeploymentConfig{
			"my-deployment": {
				Chart:     "camunda-platform-8.8",
				Namespace: "test",
			},
		},
	}

	if err := Write(rc); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("Write() did not create config file")
	}

	// Read it back
	rc2, err := Read(configPath, false)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}

	if rc2.Current != "my-deployment" {
		t.Errorf("Current = %q, want %q", rc2.Current, "my-deployment")
	}
}

func TestWriteCurrentOnly(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "current-only.yaml")

	// Write initial config
	initialContent := `
platform: gke
current: old-deployment
deployments:
  old-deployment:
    chart: test
`
	if err := os.WriteFile(configPath, []byte(initialContent), 0644); err != nil {
		t.Fatalf("Failed to write initial config: %v", err)
	}

	// Update only the current field
	if err := WriteCurrentOnly(configPath, "new-deployment"); err != nil {
		t.Fatalf("WriteCurrentOnly() error = %v", err)
	}

	// Read it back
	rc, err := Read(configPath, false)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}

	if rc.Current != "new-deployment" {
		t.Errorf("Current = %q, want %q", rc.Current, "new-deployment")
	}

	// Platform should still be preserved
	if rc.Platform != "gke" {
		t.Errorf("Platform = %q, want %q (should be preserved)", rc.Platform, "gke")
	}
}

func TestMergeStringField(t *testing.T) {
	tests := []struct {
		name     string
		target   string
		depVal   string
		rootVal  string
		expected string
	}{
		{
			name:     "target empty, dep has value",
			target:   "",
			depVal:   "from-dep",
			rootVal:  "from-root",
			expected: "from-dep",
		},
		{
			name:     "target empty, dep empty, root has value",
			target:   "",
			depVal:   "",
			rootVal:  "from-root",
			expected: "from-root",
		},
		{
			name:     "target has value, not overwritten",
			target:   "original",
			depVal:   "from-dep",
			rootVal:  "from-root",
			expected: "original",
		},
		{
			name:     "all empty",
			target:   "",
			depVal:   "",
			rootVal:  "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := tt.target
			MergeStringField(&target, tt.depVal, tt.rootVal)
			if target != tt.expected {
				t.Errorf("MergeStringField() = %q, want %q", target, tt.expected)
			}
		})
	}
}

func TestMergeBoolField(t *testing.T) {
	boolPtr := func(b bool) *bool { return &b }

	tests := []struct {
		name     string
		target   bool
		depVal   *bool
		rootVal  *bool
		expected bool
	}{
		{
			name:     "dep value overrides",
			target:   false,
			depVal:   boolPtr(true),
			rootVal:  boolPtr(false),
			expected: true,
		},
		{
			name:     "root value used when dep nil",
			target:   false,
			depVal:   nil,
			rootVal:  boolPtr(true),
			expected: true,
		},
		{
			name:     "both nil, target unchanged",
			target:   true,
			depVal:   nil,
			rootVal:  nil,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := tt.target
			MergeBoolField(&target, tt.depVal, tt.rootVal)
			if target != tt.expected {
				t.Errorf("MergeBoolField() = %v, want %v", target, tt.expected)
			}
		})
	}
}

