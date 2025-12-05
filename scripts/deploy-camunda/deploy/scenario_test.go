package deploy

import (
	"os"
	"path/filepath"
	"scripts/deploy-camunda/config"
	"scripts/deploy-camunda/internal/util"
	"strings"
	"testing"
)

func TestGenerateRandomSuffix(t *testing.T) {
	// Test that suffix has correct length
	suffix := util.GenerateRandomSuffix()
	if len(suffix) != util.RandomSuffixLength {
		t.Errorf("GenerateRandomSuffix() length = %d, want %d", len(suffix), util.RandomSuffixLength)
	}

	// Test that suffix contains only valid characters
	validChars := "abcdefghijklmnopqrstuvwxyz0123456789"
	for _, c := range suffix {
		if !strings.ContainsRune(validChars, c) {
			t.Errorf("GenerateRandomSuffix() contains invalid char %q", c)
		}
	}

	// Test uniqueness (generate multiple and check they're different)
	suffixes := make(map[string]bool)
	for i := 0; i < 100; i++ {
		s := util.GenerateRandomSuffix()
		if suffixes[s] {
			t.Errorf("GenerateRandomSuffix() generated duplicate: %s", s)
		}
		suffixes[s] = true
	}
}

func TestGenerateCompactRealmName(t *testing.T) {
	tests := []struct {
		name           string
		namespace      string
		scenario       string
		suffix         string
		wantMaxLen     int
		wantContains   string
	}{
		{
			name:       "short scenario",
			namespace:  "test",
			scenario:   "keycloak",
			suffix:     "abc12345",
			wantMaxLen: MaxRealmNameLength,
		},
		{
			name:       "long scenario name gets truncated",
			namespace:  "test",
			scenario:   "very-long-scenario-name-that-exceeds-limit",
			suffix:     "abc12345",
			wantMaxLen: MaxRealmNameLength,
		},
		{
			name:         "simple format when short enough",
			namespace:    "ns",
			scenario:     "kc",
			suffix:       "12345678",
			wantMaxLen:   MaxRealmNameLength,
			wantContains: "kc-",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateCompactRealmName(tt.namespace, tt.scenario, tt.suffix)

			if len(result) > tt.wantMaxLen {
				t.Errorf("generateCompactRealmName() length = %d, want <= %d", len(result), tt.wantMaxLen)
			}

			if tt.wantContains != "" && !strings.Contains(result, tt.wantContains) {
				t.Errorf("generateCompactRealmName() = %q, want to contain %q", result, tt.wantContains)
			}
		})
	}
}

func TestGenerateScenarioContext(t *testing.T) {
	tests := []struct {
		name     string
		scenario string
		flags    *config.RuntimeFlags
		check    func(*testing.T, *ScenarioContext)
	}{
		{
			name:     "single scenario uses provided values",
			scenario: "keycloak",
			flags: &config.RuntimeFlags{
				Scenarios:      []string{"keycloak"},
				Namespace:      "test-ns",
				Release:        "test-release",
				KeycloakRealm:  "my-realm",
				OptimizeIndexPrefix: "opt-prefix",
			},
			check: func(t *testing.T, ctx *ScenarioContext) {
				if ctx.ScenarioName != "keycloak" {
					t.Errorf("ScenarioName = %q, want %q", ctx.ScenarioName, "keycloak")
				}
				if ctx.Namespace != "test-ns" {
					t.Errorf("Namespace = %q, want %q", ctx.Namespace, "test-ns")
				}
				if ctx.KeycloakRealm != "my-realm" {
					t.Errorf("KeycloakRealm = %q, want %q", ctx.KeycloakRealm, "my-realm")
				}
				if ctx.OptimizeIndexPrefix != "opt-prefix" {
					t.Errorf("OptimizeIndexPrefix = %q, want %q", ctx.OptimizeIndexPrefix, "opt-prefix")
				}
				// Release should be default
				if ctx.Release != DefaultReleaseName {
					t.Errorf("Release = %q, want %q", ctx.Release, DefaultReleaseName)
				}
			},
		},
		{
			name:     "multi-scenario generates unique namespace",
			scenario: "keycloak-mt",
			flags: &config.RuntimeFlags{
				Scenarios: []string{"keycloak", "keycloak-mt"},
				Namespace: "base-ns",
			},
			check: func(t *testing.T, ctx *ScenarioContext) {
				// Namespace should include scenario name
				if !strings.Contains(ctx.Namespace, "base-ns-keycloak-mt") {
					t.Errorf("Namespace = %q, want to contain %q", ctx.Namespace, "base-ns-keycloak-mt")
				}
				// Auto-generated realm should be set
				if ctx.KeycloakRealm == "" {
					t.Error("KeycloakRealm should be auto-generated")
				}
				// Auto-generated prefixes should be set
				if ctx.OptimizeIndexPrefix == "" {
					t.Error("OptimizeIndexPrefix should be auto-generated")
				}
			},
		},
		{
			name:     "auto-generates all identifiers when not provided",
			scenario: "saas",
			flags: &config.RuntimeFlags{
				Scenarios: []string{"saas"},
				Namespace: "test",
			},
			check: func(t *testing.T, ctx *ScenarioContext) {
				// All identifiers should be auto-generated
				if ctx.KeycloakRealm == "" {
					t.Error("KeycloakRealm should be auto-generated")
				}
				if ctx.OptimizeIndexPrefix == "" {
					t.Error("OptimizeIndexPrefix should be auto-generated")
				}
				if ctx.OrchestrationIndexPrefix == "" {
					t.Error("OrchestrationIndexPrefix should be auto-generated")
				}
				if ctx.TasklistIndexPrefix == "" {
					t.Error("TasklistIndexPrefix should be auto-generated")
				}
				if ctx.OperateIndexPrefix == "" {
					t.Error("OperateIndexPrefix should be auto-generated")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := generateScenarioContext(tt.scenario, tt.flags)
			tt.check(t, ctx)
		})
	}
}

func TestListAvailableScenarios(t *testing.T) {
	// Create temp directory with scenario files
	tmpDir := t.TempDir()

	// Create some scenario files
	scenarios := []string{"keycloak", "keycloak-mt", "saas"}
	for _, s := range scenarios {
		filename := ScenarioFilePrefix + s + ScenarioFileSuffix
		filepath := filepath.Join(tmpDir, filename)
		if err := os.WriteFile(filepath, []byte("# test"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}

	// Create a non-scenario file that should be ignored
	if err := os.WriteFile(filepath.Join(tmpDir, "other-file.yaml"), []byte("# test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create a directory that should be ignored
	if err := os.Mkdir(filepath.Join(tmpDir, ScenarioFilePrefix+"dir"+ScenarioFileSuffix), 0755); err != nil {
		t.Fatalf("Failed to create test dir: %v", err)
	}

	// Test listing
	result := listAvailableScenarios(tmpDir)

	if len(result) != len(scenarios) {
		t.Errorf("listAvailableScenarios() returned %d scenarios, want %d", len(result), len(scenarios))
	}

	// Check that all expected scenarios are found
	resultSet := make(map[string]bool)
	for _, s := range result {
		resultSet[s] = true
	}
	for _, expected := range scenarios {
		if !resultSet[expected] {
			t.Errorf("listAvailableScenarios() missing expected scenario %q", expected)
		}
	}
}

func TestListAvailableScenarios_NonexistentDir(t *testing.T) {
	result := listAvailableScenarios("/nonexistent/path")
	if result != nil {
		t.Errorf("listAvailableScenarios() on nonexistent dir = %v, want nil", result)
	}
}

func TestEnhanceScenarioError(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		scenario     string
		scenarioPath string
		chartPath    string
		wantNil      bool
		wantContains string
	}{
		{
			name:    "nil error returns nil",
			err:     nil,
			wantNil: true,
		},
		{
			name:         "not found error gets enhanced",
			err:          os.ErrNotExist,
			scenario:     "missing-scenario",
			scenarioPath: "/some/path",
			chartPath:    "/chart",
			wantContains: "missing-scenario",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := enhanceScenarioError(tt.err, tt.scenario, tt.scenarioPath, tt.chartPath)

			if tt.wantNil && result != nil {
				t.Errorf("enhanceScenarioError() = %v, want nil", result)
				return
			}

			if !tt.wantNil && result == nil {
				t.Error("enhanceScenarioError() = nil, want error")
				return
			}

			if tt.wantContains != "" {
				// Check if it's a DeployError
				if de, ok := result.(*DeployError); ok {
					if !strings.Contains(de.Message, tt.wantContains) {
						t.Errorf("enhanceScenarioError() message = %q, want to contain %q", de.Message, tt.wantContains)
					}
				}
			}
		})
	}
}

func TestValidateScenarios(t *testing.T) {
	// Create temp directory with scenario files
	tmpDir := t.TempDir()
	scenarioDir := filepath.Join(tmpDir, "scenarios")
	if err := os.MkdirAll(scenarioDir, 0755); err != nil {
		t.Fatalf("Failed to create scenario dir: %v", err)
	}

	// Create keycloak scenario file
	keycloakFile := filepath.Join(scenarioDir, ScenarioFilePrefix+"keycloak"+ScenarioFileSuffix)
	if err := os.WriteFile(keycloakFile, []byte("# test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tests := []struct {
		name      string
		flags     *config.RuntimeFlags
		wantError bool
	}{
		{
			name: "valid scenario passes",
			flags: &config.RuntimeFlags{
				Scenarios:    []string{"keycloak"},
				ScenarioPath: scenarioDir,
			},
			wantError: false,
		},
		{
			name: "missing scenario fails",
			flags: &config.RuntimeFlags{
				Scenarios:    []string{"nonexistent"},
				ScenarioPath: scenarioDir,
			},
			wantError: true,
		},
		{
			name: "mixed valid and invalid fails",
			flags: &config.RuntimeFlags{
				Scenarios:    []string{"keycloak", "nonexistent"},
				ScenarioPath: scenarioDir,
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateScenarios(tt.flags)
			if (err != nil) != tt.wantError {
				t.Errorf("validateScenarios() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}
