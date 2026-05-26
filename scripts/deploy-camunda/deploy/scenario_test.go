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

func TestNamespaceDerivedSuffix_Deterministic(t *testing.T) {
	// Same namespace always produces the same suffix.
	s1 := namespaceDerivedSuffix("matrix-88-qaosupg-inst-gke")
	s2 := namespaceDerivedSuffix("matrix-88-qaosupg-inst-gke")
	if s1 != s2 {
		t.Errorf("same namespace produced different suffixes: %q vs %q", s1, s2)
	}
	if len(s1) != 8 {
		t.Errorf("suffix should be 8 chars, got %d: %q", len(s1), s1)
	}
}

func TestNamespaceDerivedSuffix_UniquenessAcrossNamespaces(t *testing.T) {
	// Different namespaces produce different suffixes.
	namespaces := []string{
		"matrix-88-qaosupg-inst-gke",
		"matrix-89-qaosupg-inst-gke",
		"matrix-88-qaelupg-inst-gke",
		"matrix-810-keyco-inst-gke",
	}
	seen := make(map[string]string)
	for _, ns := range namespaces {
		s := namespaceDerivedSuffix(ns)
		if prev, ok := seen[s]; ok {
			t.Errorf("collision: %q and %q both produce suffix %q", prev, ns, s)
		}
		seen[s] = ns
	}
}

func TestCrossJobPrefixConsistency(t *testing.T) {
	// Simulate cross-job upgrade: install and upgrade are independent calls
	// with the same namespace but fresh RuntimeFlags. Both should produce
	// identical prefixes because the suffix is derived from the namespace.
	namespace := "matrix-88-qaosupg-inst-gke"

	// Job 1: install
	installFlags := &config.RuntimeFlags{
		Deployment: config.DeploymentFlags{
			Namespace: namespace,
			Scenarios: []string{"qa-opensearch"},
		},
	}
	installCtx, err := generateScenarioContext("qa-opensearch", installFlags)
	if err != nil {
		t.Fatalf("install generateScenarioContext: %v", err)
	}

	// Job 2: upgrade (completely fresh flags, same namespace)
	upgradeFlags := &config.RuntimeFlags{
		Deployment: config.DeploymentFlags{
			Namespace: namespace,
			Scenarios: []string{"qa-opensearch"},
		},
	}
	upgradeCtx, err := generateScenarioContext("qa-opensearch", upgradeFlags)
	if err != nil {
		t.Fatalf("upgrade generateScenarioContext: %v", err)
	}

	// Prefixes must match — this is the core property that fixes cross-job upgrades.
	if installCtx.OrchestrationIndexPrefix != upgradeCtx.OrchestrationIndexPrefix {
		t.Errorf("OrchestrationIndexPrefix mismatch: install=%q upgrade=%q",
			installCtx.OrchestrationIndexPrefix, upgradeCtx.OrchestrationIndexPrefix)
	}
	if installCtx.OperateIndexPrefix != upgradeCtx.OperateIndexPrefix {
		t.Errorf("OperateIndexPrefix mismatch: install=%q upgrade=%q",
			installCtx.OperateIndexPrefix, upgradeCtx.OperateIndexPrefix)
	}
	if installCtx.OptimizeIndexPrefix != upgradeCtx.OptimizeIndexPrefix {
		t.Errorf("OptimizeIndexPrefix mismatch: install=%q upgrade=%q",
			installCtx.OptimizeIndexPrefix, upgradeCtx.OptimizeIndexPrefix)
	}
	if installCtx.TasklistIndexPrefix != upgradeCtx.TasklistIndexPrefix {
		t.Errorf("TasklistIndexPrefix mismatch: install=%q upgrade=%q",
			installCtx.TasklistIndexPrefix, upgradeCtx.TasklistIndexPrefix)
	}
	if installCtx.KeycloakRealm != upgradeCtx.KeycloakRealm {
		t.Errorf("KeycloakRealm mismatch: install=%q upgrade=%q",
			installCtx.KeycloakRealm, upgradeCtx.KeycloakRealm)
	}
}

func TestPrefixKeyOverridesScenarioName(t *testing.T) {
	// Simulate the cross-version upgrade issue: 8.8 uses scenario name
	// "qa-opensearch-tasklist-v1" while 8.9 uses "qa-opensearch-upg".
	// When prefix-key is used to pin both to "qa-opensearch-upg", the
	// prefixes must match despite different scenario names.
	namespace := "matrix-88-qaosupg-inst-gke"

	// Job 1: install on 8.8 (scenario="qa-opensearch-tasklist-v1", prefix-key="qa-opensearch-upg")
	installFlags := &config.RuntimeFlags{
		Deployment: config.DeploymentFlags{
			Namespace: namespace,
			Scenarios: []string{"qa-opensearch-tasklist-v1"},
		},
	}
	// PinScenarioPrefixes called with prefix-key value, not scenario name
	if err := PinScenarioPrefixes("qa-opensearch-upg", installFlags); err != nil {
		t.Fatalf("PinScenarioPrefixes (install): %v", err)
	}

	// Job 2: upgrade on 8.9 (scenario="qa-opensearch-upg", prefix-key="qa-opensearch-upg")
	upgradeFlags := &config.RuntimeFlags{
		Deployment: config.DeploymentFlags{
			Namespace: namespace,
			Scenarios: []string{"qa-opensearch-upg"},
		},
	}
	if err := PinScenarioPrefixes("qa-opensearch-upg", upgradeFlags); err != nil {
		t.Fatalf("PinScenarioPrefixes (upgrade): %v", err)
	}

	if installFlags.Index.OrchestrationIndexPrefix != upgradeFlags.Index.OrchestrationIndexPrefix {
		t.Errorf("OrchestrationIndexPrefix mismatch: install=%q upgrade=%q",
			installFlags.Index.OrchestrationIndexPrefix, upgradeFlags.Index.OrchestrationIndexPrefix)
	}
	if installFlags.Index.OptimizeIndexPrefix != upgradeFlags.Index.OptimizeIndexPrefix {
		t.Errorf("OptimizeIndexPrefix mismatch: install=%q upgrade=%q",
			installFlags.Index.OptimizeIndexPrefix, upgradeFlags.Index.OptimizeIndexPrefix)
	}
	if installFlags.Auth.KeycloakRealm != upgradeFlags.Auth.KeycloakRealm {
		t.Errorf("KeycloakRealm mismatch: install=%q upgrade=%q",
			installFlags.Auth.KeycloakRealm, upgradeFlags.Auth.KeycloakRealm)
	}
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

func TestComputeExpectedOrchestrationPrefix_UsesExistingFlag(t *testing.T) {
	flags := &config.RuntimeFlags{
		Deployment: config.DeploymentFlags{
			Namespace: "matrix-89-eske-upgm",
		},
		Index: config.IndexPrefixFlags{
			OrchestrationIndexPrefix: "already-pinned-prefix",
		},
	}

	got := ComputeExpectedOrchestrationPrefix("some-scenario", flags)
	if got != "already-pinned-prefix" {
		t.Errorf("expected pinned prefix %q, got %q", "already-pinned-prefix", got)
	}
}

func TestComputeExpectedOrchestrationPrefix_ComputesFromScenario(t *testing.T) {
	flags := &config.RuntimeFlags{
		Deployment: config.DeploymentFlags{
			Namespace: "matrix-89-eske-upgm",
		},
	}

	got := ComputeExpectedOrchestrationPrefix("elasticsearch", flags)
	if got == "" {
		t.Fatal("expected non-empty prefix, got empty")
	}
	if !startsWith(got, "orch-") {
		t.Errorf("expected prefix to start with 'orch-', got %q", got)
	}
}

func TestComputeExpectedOrchestrationPrefix_MatchesPinned(t *testing.T) {
	// ComputeExpected should produce the same value as PinScenarioPrefixes.
	namespace := "matrix-89-qaosupg-inst-gke"
	scenario := "qa-opensearch-upg"

	flags := &config.RuntimeFlags{
		Deployment: config.DeploymentFlags{
			Namespace: namespace,
		},
	}
	if err := PinScenarioPrefixes(scenario, flags); err != nil {
		t.Fatalf("PinScenarioPrefixes: %v", err)
	}

	freshFlags := &config.RuntimeFlags{
		Deployment: config.DeploymentFlags{
			Namespace: namespace,
		},
	}
	computed := ComputeExpectedOrchestrationPrefix(scenario, freshFlags)

	if computed != flags.Index.OrchestrationIndexPrefix {
		t.Errorf("ComputeExpected=%q does not match Pinned=%q",
			computed, flags.Index.OrchestrationIndexPrefix)
	}
}

func startsWith(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
