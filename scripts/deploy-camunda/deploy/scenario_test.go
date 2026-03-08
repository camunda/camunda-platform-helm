package deploy

import (
	"testing"

	"scripts/deploy-camunda/config"
)

func TestPinScenarioPrefixes_SetsAllFields(t *testing.T) {
	flags := &config.RuntimeFlags{
		Deployment: config.DeploymentFlags{
			Namespace: "matrix-89-eske-upgm",
			Scenarios: []string{"elasticsearch"},
		},
	}

	err := PinScenarioPrefixes("elasticsearch", flags)
	if err != nil {
		t.Fatalf("PinScenarioPrefixes returned error: %v", err)
	}

	// All index prefixes should be set.
	if flags.Index.OptimizeIndexPrefix == "" {
		t.Error("OptimizeIndexPrefix should be set, got empty")
	}
	if flags.Index.OrchestrationIndexPrefix == "" {
		t.Error("OrchestrationIndexPrefix should be set, got empty")
	}
	if flags.Index.TasklistIndexPrefix == "" {
		t.Error("TasklistIndexPrefix should be set, got empty")
	}
	if flags.Index.OperateIndexPrefix == "" {
		t.Error("OperateIndexPrefix should be set, got empty")
	}
	if flags.Auth.KeycloakRealm == "" {
		t.Error("KeycloakRealm should be set, got empty")
	}

	// Verify prefix naming conventions.
	if got := flags.Index.OptimizeIndexPrefix; len(got) < 4 {
		t.Errorf("OptimizeIndexPrefix too short: %q", got)
	}
	if got := flags.Index.OrchestrationIndexPrefix; len(got) < 5 {
		t.Errorf("OrchestrationIndexPrefix too short: %q", got)
	}

	t.Logf("Pinned prefixes: orch=%s opt=%s task=%s op=%s realm=%s",
		flags.Index.OrchestrationIndexPrefix,
		flags.Index.OptimizeIndexPrefix,
		flags.Index.TasklistIndexPrefix,
		flags.Index.OperateIndexPrefix,
		flags.Auth.KeycloakRealm)
}

func TestPinScenarioPrefixes_DoesNotOverrideExisting(t *testing.T) {
	flags := &config.RuntimeFlags{
		Deployment: config.DeploymentFlags{
			Namespace: "matrix-89-eske-upgm",
			Scenarios: []string{"elasticsearch"},
		},
		Index: config.IndexPrefixFlags{
			OrchestrationIndexPrefix: "existing-orch",
			OptimizeIndexPrefix:      "existing-opt",
			TasklistIndexPrefix:      "existing-task",
			OperateIndexPrefix:       "existing-op",
		},
		Auth: config.AuthFlags{
			KeycloakRealm: "existing-realm",
		},
	}

	err := PinScenarioPrefixes("elasticsearch", flags)
	if err != nil {
		t.Fatalf("PinScenarioPrefixes returned error: %v", err)
	}

	// Existing values should NOT be overwritten.
	if got := flags.Index.OrchestrationIndexPrefix; got != "existing-orch" {
		t.Errorf("OrchestrationIndexPrefix should not change: want %q, got %q", "existing-orch", got)
	}
	if got := flags.Index.OptimizeIndexPrefix; got != "existing-opt" {
		t.Errorf("OptimizeIndexPrefix should not change: want %q, got %q", "existing-opt", got)
	}
	if got := flags.Index.TasklistIndexPrefix; got != "existing-task" {
		t.Errorf("TasklistIndexPrefix should not change: want %q, got %q", "existing-task", got)
	}
	if got := flags.Index.OperateIndexPrefix; got != "existing-op" {
		t.Errorf("OperateIndexPrefix should not change: want %q, got %q", "existing-op", got)
	}
	if got := flags.Auth.KeycloakRealm; got != "existing-realm" {
		t.Errorf("KeycloakRealm should not change: want %q, got %q", "existing-realm", got)
	}
}

func TestPinScenarioPrefixes_Idempotent(t *testing.T) {
	flags := &config.RuntimeFlags{
		Deployment: config.DeploymentFlags{
			Namespace: "matrix-89-eske-upgm",
			Scenarios: []string{"elasticsearch"},
		},
	}

	// First call pins the values.
	err := PinScenarioPrefixes("elasticsearch", flags)
	if err != nil {
		t.Fatalf("First PinScenarioPrefixes returned error: %v", err)
	}

	// Save the pinned values.
	orchPrefix := flags.Index.OrchestrationIndexPrefix
	optPrefix := flags.Index.OptimizeIndexPrefix
	taskPrefix := flags.Index.TasklistIndexPrefix
	opPrefix := flags.Index.OperateIndexPrefix
	realm := flags.Auth.KeycloakRealm

	// Second call should NOT change the values (they're already set).
	err = PinScenarioPrefixes("elasticsearch", flags)
	if err != nil {
		t.Fatalf("Second PinScenarioPrefixes returned error: %v", err)
	}

	if flags.Index.OrchestrationIndexPrefix != orchPrefix {
		t.Errorf("OrchestrationIndexPrefix changed on second call: %q -> %q", orchPrefix, flags.Index.OrchestrationIndexPrefix)
	}
	if flags.Index.OptimizeIndexPrefix != optPrefix {
		t.Errorf("OptimizeIndexPrefix changed on second call: %q -> %q", optPrefix, flags.Index.OptimizeIndexPrefix)
	}
	if flags.Index.TasklistIndexPrefix != taskPrefix {
		t.Errorf("TasklistIndexPrefix changed on second call: %q -> %q", taskPrefix, flags.Index.TasklistIndexPrefix)
	}
	if flags.Index.OperateIndexPrefix != opPrefix {
		t.Errorf("OperateIndexPrefix changed on second call: %q -> %q", opPrefix, flags.Index.OperateIndexPrefix)
	}
	if flags.Auth.KeycloakRealm != realm {
		t.Errorf("KeycloakRealm changed on second call: %q -> %q", realm, flags.Auth.KeycloakRealm)
	}
}

func TestPinScenarioPrefixes_SharedBetweenClones(t *testing.T) {
	// Simulate the two-step upgrade pattern: pin on base flags, then clone.
	flags := &config.RuntimeFlags{
		Deployment: config.DeploymentFlags{
			Namespace: "matrix-89-eske-upgm",
			Scenarios: []string{"elasticsearch"},
		},
	}

	err := PinScenarioPrefixes("elasticsearch", flags)
	if err != nil {
		t.Fatalf("PinScenarioPrefixes returned error: %v", err)
	}

	// Clone for Step 1 and Step 2 (shallow copy like runner.go does).
	step1 := *flags
	step2 := *flags

	// Both clones should have the same pinned values.
	if step1.Index.OrchestrationIndexPrefix != step2.Index.OrchestrationIndexPrefix {
		t.Errorf("Step 1 and Step 2 OrchestrationIndexPrefix differ: %q vs %q",
			step1.Index.OrchestrationIndexPrefix, step2.Index.OrchestrationIndexPrefix)
	}
	if step1.Index.OptimizeIndexPrefix != step2.Index.OptimizeIndexPrefix {
		t.Errorf("Step 1 and Step 2 OptimizeIndexPrefix differ: %q vs %q",
			step1.Index.OptimizeIndexPrefix, step2.Index.OptimizeIndexPrefix)
	}
	if step1.Auth.KeycloakRealm != step2.Auth.KeycloakRealm {
		t.Errorf("Step 1 and Step 2 KeycloakRealm differ: %q vs %q",
			step1.Auth.KeycloakRealm, step2.Auth.KeycloakRealm)
	}

	t.Logf("Step1 orch=%s realm=%s", step1.Index.OrchestrationIndexPrefix, step1.Auth.KeycloakRealm)
	t.Logf("Step2 orch=%s realm=%s", step2.Index.OrchestrationIndexPrefix, step2.Auth.KeycloakRealm)
}

func TestNormalizeIdentifierPart(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"elasticsearch", "elasticsearch"},
		{"keycloak-mt", "keycloak-mt"},
		{"UPPER", "upper"},
		{"with spaces", "with-spaces"},
		{"special!@#chars", "special---chars"},
		{"--leading-dashes--", "leading-dashes"},
		{"", "x"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeIdentifierPart(tt.input)
			if got != tt.want {
				t.Errorf("normalizeIdentifierPart(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestGenerateCompactRealmName(t *testing.T) {
	t.Run("short names fit directly", func(t *testing.T) {
		name := generateCompactRealmName("ns", "eske", "abcd1234")
		if len(name) > 36 {
			t.Errorf("realm name too long: %q (%d chars)", name, len(name))
		}
		if name != "eske-abcd1234" {
			t.Errorf("expected simple format, got %q", name)
		}
	})

	t.Run("long names are truncated and hashed", func(t *testing.T) {
		longScenario := "this-is-a-very-long-scenario-name-that-exceeds-limits"
		name := generateCompactRealmName("namespace", longScenario, "abcd1234")
		if len(name) > 36 {
			t.Errorf("realm name too long: %q (%d chars)", name, len(name))
		}
	})

	t.Run("36-char limit enforced", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			name := generateCompactRealmName("some-long-namespace", "some-long-scenario", "abcd1234")
			if len(name) > 36 {
				t.Fatalf("realm name exceeds 36 chars: %q (%d)", name, len(name))
			}
		}
	})
}
