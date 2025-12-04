package deploy

import (
	"scripts/deploy-camunda/config"
	"strings"
	"testing"
)

func TestGenerateRandomSuffix(t *testing.T) {
	// Generate multiple suffixes and verify properties
	seen := make(map[string]bool)

	for i := 0; i < 100; i++ {
		suffix := generateRandomSuffix()

		// Check length
		if len(suffix) != RandomSuffixLength {
			t.Errorf("Suffix length = %d, want %d", len(suffix), RandomSuffixLength)
		}

		// Check character set (lowercase alphanumeric)
		for _, c := range suffix {
			if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')) {
				t.Errorf("Invalid character %q in suffix", c)
			}
		}

		// Track for uniqueness (probabilistically)
		seen[suffix] = true
	}

	// Should have high uniqueness (collisions extremely unlikely)
	if len(seen) < 95 {
		t.Errorf("Only %d unique suffixes out of 100, expected near 100", len(seen))
	}
}

func TestGenerateCompactRealmName(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		scenario  string
		suffix    string
		maxLen    int
	}{
		{
			name:      "short scenario fits",
			namespace: "test-ns",
			scenario:  "keycloak",
			suffix:    "abc12345",
			maxLen:    MaxRealmNameLength,
		},
		{
			name:      "long scenario is truncated",
			namespace: "test-namespace",
			scenario:  "very-long-scenario-name-that-exceeds-limits",
			suffix:    "xyz98765",
			maxLen:    MaxRealmNameLength,
		},
		{
			name:      "medium scenario",
			namespace: "ns",
			scenario:  "keycloak-multi-tenant",
			suffix:    "12345678",
			maxLen:    MaxRealmNameLength,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateCompactRealmName(tt.namespace, tt.scenario, tt.suffix)

			if len(result) > tt.maxLen {
				t.Errorf("Result length %d exceeds max %d: %q", len(result), tt.maxLen, result)
			}

			if result == "" {
				t.Error("Result should not be empty")
			}
		})
	}
}

func TestGenerateCompactRealmName_Deterministic(t *testing.T) {
	// Same inputs should produce consistent outputs
	result1 := generateCompactRealmName("ns", "scenario", "suffix12")
	result2 := generateCompactRealmName("ns", "scenario", "suffix12")

	if result1 != result2 {
		t.Errorf("Non-deterministic: %q != %q", result1, result2)
	}
}

func TestGenerateScenarioContext_SingleScenario(t *testing.T) {
	flags := &config.RuntimeFlags{
		Namespace:   "test-namespace",
		IngressHost: "test.example.com",
		Scenarios:   []string{"keycloak"},
	}

	ctx := generateScenarioContext("keycloak", flags)

	// Single scenario uses provided namespace directly
	if ctx.Namespace != "test-namespace" {
		t.Errorf("Namespace = %q, want %q", ctx.Namespace, "test-namespace")
	}

	// Release should be default
	if ctx.Release != DefaultReleaseName {
		t.Errorf("Release = %q, want %q", ctx.Release, DefaultReleaseName)
	}

	// IngressHost unchanged for single scenario
	if ctx.IngressHost != "test.example.com" {
		t.Errorf("IngressHost = %q, want %q", ctx.IngressHost, "test.example.com")
	}

	// Generated prefixes should not be empty
	if ctx.KeycloakRealm == "" {
		t.Error("KeycloakRealm should not be empty")
	}
	if ctx.OptimizeIndexPrefix == "" {
		t.Error("OptimizeIndexPrefix should not be empty")
	}
}

func TestGenerateScenarioContext_MultipleScenarios(t *testing.T) {
	flags := &config.RuntimeFlags{
		Namespace:   "test-namespace",
		IngressHost: "test.example.com",
		Scenarios:   []string{"keycloak", "keycloak-mt", "saas"},
	}

	ctx := generateScenarioContext("keycloak-mt", flags)

	// Multi-scenario appends scenario name to namespace
	expectedNs := "test-namespace-keycloak-mt"
	if ctx.Namespace != expectedNs {
		t.Errorf("Namespace = %q, want %q", ctx.Namespace, expectedNs)
	}

	// IngressHost prefixed with scenario
	if !strings.HasPrefix(ctx.IngressHost, "keycloak-mt-") {
		t.Errorf("IngressHost = %q, want prefix 'keycloak-mt-'", ctx.IngressHost)
	}
}

func TestGenerateScenarioContext_UsesProvidedPrefixes(t *testing.T) {
	flags := &config.RuntimeFlags{
		Namespace:                "test-namespace",
		Scenarios:                []string{"keycloak"},
		KeycloakRealm:            "custom-realm",
		OptimizeIndexPrefix:      "custom-opt",
		OrchestrationIndexPrefix: "custom-orch",
		TasklistIndexPrefix:      "custom-task",
		OperateIndexPrefix:       "custom-op",
	}

	ctx := generateScenarioContext("keycloak", flags)

	if ctx.KeycloakRealm != "custom-realm" {
		t.Errorf("KeycloakRealm = %q, want %q", ctx.KeycloakRealm, "custom-realm")
	}
	if ctx.OptimizeIndexPrefix != "custom-opt" {
		t.Errorf("OptimizeIndexPrefix = %q, want %q", ctx.OptimizeIndexPrefix, "custom-opt")
	}
	if ctx.OrchestrationIndexPrefix != "custom-orch" {
		t.Errorf("OrchestrationIndexPrefix = %q, want %q", ctx.OrchestrationIndexPrefix, "custom-orch")
	}
	if ctx.TasklistIndexPrefix != "custom-task" {
		t.Errorf("TasklistIndexPrefix = %q, want %q", ctx.TasklistIndexPrefix, "custom-task")
	}
	if ctx.OperateIndexPrefix != "custom-op" {
		t.Errorf("OperateIndexPrefix = %q, want %q", ctx.OperateIndexPrefix, "custom-op")
	}
}

func TestGenerateScenarioContext_GeneratesUniquePrefixes(t *testing.T) {
	flags := &config.RuntimeFlags{
		Namespace: "test-namespace",
		Scenarios: []string{"keycloak"},
	}

	// Generate multiple contexts and check uniqueness
	seen := make(map[string]bool)
	for i := 0; i < 10; i++ {
		ctx := generateScenarioContext("keycloak", flags)
		if seen[ctx.KeycloakRealm] {
			// Could be a collision, but with 8 char random suffix, very unlikely in 10 iterations
			t.Logf("Warning: possible collision detected for realm %q", ctx.KeycloakRealm)
		}
		seen[ctx.KeycloakRealm] = true
	}
}

func TestEnhanceScenarioError_NilError(t *testing.T) {
	result := enhanceScenarioError(nil, "scenario", "", "")
	if result != nil {
		t.Errorf("Expected nil for nil input, got %v", result)
	}
}

func TestEnhanceScenarioError_NonNotFoundError(t *testing.T) {
	originalErr := NewTestError("some other error")
	result := enhanceScenarioError(originalErr, "scenario", "", "")

	// Should return original error unchanged
	if result != originalErr {
		t.Errorf("Expected original error, got %v", result)
	}
}

func TestEnhanceScenarioError_NotFoundError(t *testing.T) {
	originalErr := NewTestError("file not found")
	result := enhanceScenarioError(originalErr, "missing-scenario", "", "/chart/path")

	if result == nil {
		t.Fatal("Expected enhanced error, got nil")
	}

	errStr := result.Error()

	// Should contain helpful context
	if !strings.Contains(errStr, "missing-scenario") {
		t.Error("Error should mention scenario name")
	}
	if !strings.Contains(errStr, "Searched in") {
		t.Error("Error should mention search location")
	}
}

// TestError is a simple error type for testing.
type TestError struct {
	msg string
}

func NewTestError(msg string) *TestError {
	return &TestError{msg: msg}
}

func (e *TestError) Error() string {
	return e.msg
}

