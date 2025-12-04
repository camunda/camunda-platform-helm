package config

import (
	"strings"
	"testing"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name        string
		flags       RuntimeFlags
		expectError string
	}{
		{
			name: "valid config",
			flags: RuntimeFlags{
				ChartPath: "/path/to/chart",
				Namespace: "test-ns",
				Release:   "test-release",
				Scenario:  "keycloak",
			},
			expectError: "",
		},
		{
			name: "missing chart and chart-path",
			flags: RuntimeFlags{
				Namespace: "test-ns",
				Release:   "test-release",
				Scenario:  "keycloak",
			},
			expectError: "either --chart-path or --chart must be provided",
		},
		{
			name: "missing namespace",
			flags: RuntimeFlags{
				ChartPath: "/path/to/chart",
				Release:   "test-release",
				Scenario:  "keycloak",
			},
			expectError: "namespace not set",
		},
		{
			name: "missing release",
			flags: RuntimeFlags{
				ChartPath: "/path/to/chart",
				Namespace: "test-ns",
				Scenario:  "keycloak",
			},
			expectError: "release not set",
		},
		{
			name: "missing scenario",
			flags: RuntimeFlags{
				ChartPath: "/path/to/chart",
				Namespace: "test-ns",
				Release:   "test-release",
			},
			expectError: "scenario not set",
		},
		{
			name: "version without chart",
			flags: RuntimeFlags{
				ChartPath:    "/path/to/chart",
				ChartVersion: "1.0.0",
				Namespace:    "test-ns",
				Release:      "test-release",
				Scenario:     "keycloak",
			},
			expectError: "--version requires --chart",
		},
		{
			name: "chart with version is valid",
			flags: RuntimeFlags{
				Chart:        "camunda-platform",
				ChartVersion: "8.8.0",
				Namespace:    "test-ns",
				Release:      "test-release",
				Scenario:     "keycloak",
			},
			expectError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(&tt.flags)

			if tt.expectError == "" {
				if err != nil {
					t.Errorf("Validate() unexpected error = %v", err)
				}
			} else {
				if err == nil {
					t.Errorf("Validate() expected error containing %q, got nil", tt.expectError)
				} else if !strings.Contains(err.Error(), tt.expectError) {
					t.Errorf("Validate() error = %v, want containing %q", err, tt.expectError)
				}
			}
		})
	}
}

func TestValidate_ParsesScenarios(t *testing.T) {
	flags := RuntimeFlags{
		ChartPath: "/path/to/chart",
		Namespace: "test-ns",
		Release:   "test-release",
		Scenario:  "keycloak,keycloak-mt,saas",
	}

	err := Validate(&flags)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	if len(flags.Scenarios) != 3 {
		t.Errorf("Scenarios length = %d, want 3", len(flags.Scenarios))
	}

	expected := []string{"keycloak", "keycloak-mt", "saas"}
	for i, s := range expected {
		if flags.Scenarios[i] != s {
			t.Errorf("Scenarios[%d] = %q, want %q", i, flags.Scenarios[i], s)
		}
	}
}

func TestApplyActiveDeployment(t *testing.T) {
	boolPtr := func(b bool) *bool { return &b }

	rc := &RootConfig{
		RepoRoot: "/global/repo",
		Platform: "gke",
		LogLevel: "info",
		Deployments: map[string]DeploymentConfig{
			"test-dep": {
				Chart:           "camunda-platform-8.8",
				Namespace:       "from-deployment",
				Release:         "test-release",
				Scenario:        "keycloak",
				Platform:        "eks",
				ExternalSecrets: boolPtr(true),
			},
		},
	}

	flags := &RuntimeFlags{}

	err := ApplyActiveDeployment(rc, "test-dep", flags)
	if err != nil {
		t.Fatalf("ApplyActiveDeployment() error = %v", err)
	}

	// Check deployment-specific values
	if flags.Chart != "camunda-platform-8.8" {
		t.Errorf("Chart = %q, want %q", flags.Chart, "camunda-platform-8.8")
	}
	if flags.Namespace != "from-deployment" {
		t.Errorf("Namespace = %q, want %q", flags.Namespace, "from-deployment")
	}

	// Deployment platform should override root
	if flags.Platform != "eks" {
		t.Errorf("Platform = %q, want %q (from deployment)", flags.Platform, "eks")
	}

	// Root value should be used when deployment doesn't specify
	if flags.RepoRoot != "/global/repo" {
		t.Errorf("RepoRoot = %q, want %q (from root)", flags.RepoRoot, "/global/repo")
	}

	// Boolean pointer handling
	if !flags.ExternalSecrets {
		t.Error("ExternalSecrets = false, want true (from deployment)")
	}
}

func TestApplyActiveDeployment_NotFound(t *testing.T) {
	rc := &RootConfig{
		Deployments: map[string]DeploymentConfig{
			"existing": {Chart: "test"},
		},
	}

	flags := &RuntimeFlags{}
	err := ApplyActiveDeployment(rc, "nonexistent", flags)

	if err == nil {
		t.Error("ApplyActiveDeployment() expected error for nonexistent deployment")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Error = %v, want containing 'not found'", err)
	}
}

func TestApplyActiveDeployment_AutoSelectsSingleDeployment(t *testing.T) {
	rc := &RootConfig{
		Deployments: map[string]DeploymentConfig{
			"only-one": {
				Chart:     "camunda-platform-8.8",
				Namespace: "auto-selected",
			},
		},
	}

	flags := &RuntimeFlags{}
	err := ApplyActiveDeployment(rc, "", flags) // Empty active name

	if err != nil {
		t.Fatalf("ApplyActiveDeployment() error = %v", err)
	}

	if flags.Namespace != "auto-selected" {
		t.Errorf("Namespace = %q, want %q (auto-selected)", flags.Namespace, "auto-selected")
	}
}

func TestApplyActiveDeployment_FlagsNotOverwritten(t *testing.T) {
	rc := &RootConfig{
		Platform: "gke",
		Deployments: map[string]DeploymentConfig{
			"test": {
				Namespace: "from-config",
				Platform:  "eks",
			},
		},
	}

	flags := &RuntimeFlags{
		Namespace: "from-cli", // Already set by CLI flag
		Platform:  "rosa",     // Already set by CLI flag
	}

	err := ApplyActiveDeployment(rc, "test", flags)
	if err != nil {
		t.Fatalf("ApplyActiveDeployment() error = %v", err)
	}

	// CLI values should be preserved
	if flags.Namespace != "from-cli" {
		t.Errorf("Namespace = %q, want %q (from CLI)", flags.Namespace, "from-cli")
	}
	if flags.Platform != "rosa" {
		t.Errorf("Platform = %q, want %q (from CLI)", flags.Platform, "rosa")
	}
}

