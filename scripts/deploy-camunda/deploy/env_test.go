package deploy

import (
	"os"
	"sync"
	"testing"
)

func TestEnvScope_Apply(t *testing.T) {
	// Set initial values
	os.Setenv("TEST_VAR_1", "original1")
	os.Setenv("TEST_VAR_2", "original2")
	defer func() {
		os.Unsetenv("TEST_VAR_1")
		os.Unsetenv("TEST_VAR_2")
	}()

	scope := NewEnvScope([]string{"TEST_VAR_1", "TEST_VAR_2"})

	cleanup := scope.Apply(func() {
		os.Setenv("TEST_VAR_1", "modified1")
		os.Setenv("TEST_VAR_2", "modified2")
	})

	// Values should be modified
	if got := os.Getenv("TEST_VAR_1"); got != "modified1" {
		t.Errorf("TEST_VAR_1 = %q, want %q", got, "modified1")
	}
	if got := os.Getenv("TEST_VAR_2"); got != "modified2" {
		t.Errorf("TEST_VAR_2 = %q, want %q", got, "modified2")
	}

	// Call cleanup
	cleanup()

	// Values should be restored
	if got := os.Getenv("TEST_VAR_1"); got != "original1" {
		t.Errorf("After cleanup: TEST_VAR_1 = %q, want %q", got, "original1")
	}
	if got := os.Getenv("TEST_VAR_2"); got != "original2" {
		t.Errorf("After cleanup: TEST_VAR_2 = %q, want %q", got, "original2")
	}
}

func TestEnvScope_RestoresEmptyVar(t *testing.T) {
	// Ensure var is unset
	os.Unsetenv("TEST_NEW_VAR")

	scope := NewEnvScope([]string{"TEST_NEW_VAR"})

	cleanup := scope.Apply(func() {
		os.Setenv("TEST_NEW_VAR", "new-value")
	})
	defer cleanup()

	// Var should be set
	if got := os.Getenv("TEST_NEW_VAR"); got != "new-value" {
		t.Errorf("TEST_NEW_VAR = %q, want %q", got, "new-value")
	}

	// After cleanup
	cleanup()

	// Var should be unset again
	if got := os.Getenv("TEST_NEW_VAR"); got != "" {
		t.Errorf("After cleanup: TEST_NEW_VAR = %q, want empty", got)
	}
}

func TestEnvScope_PanicsOnDoubleApply(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic on double Apply")
		}
	}()

	scope := NewEnvScope([]string{"TEST_VAR"})
	_ = scope.Apply(func() {})
	_ = scope.Apply(func() {}) // Should panic
}

func TestEnvScope_CleanupIdempotent(t *testing.T) {
	os.Setenv("TEST_IDEM_VAR", "original")
	defer os.Unsetenv("TEST_IDEM_VAR")

	scope := NewEnvScope([]string{"TEST_IDEM_VAR"})
	cleanup := scope.Apply(func() {
		os.Setenv("TEST_IDEM_VAR", "modified")
	})

	// Call cleanup multiple times - should not panic
	cleanup()
	cleanup()
	cleanup()

	if got := os.Getenv("TEST_IDEM_VAR"); got != "original" {
		t.Errorf("TEST_IDEM_VAR = %q, want %q", got, "original")
	}
}

func TestWithEnv(t *testing.T) {
	os.Setenv("TEST_WITH_VAR", "original")
	defer os.Unsetenv("TEST_WITH_VAR")

	result, err := WithEnv(
		[]string{"TEST_WITH_VAR"},
		func() { os.Setenv("TEST_WITH_VAR", "modified") },
		func() (string, error) {
			return os.Getenv("TEST_WITH_VAR"), nil
		},
	)

	if err != nil {
		t.Fatalf("WithEnv error = %v", err)
	}

	if result != "modified" {
		t.Errorf("Result = %q, want %q", result, "modified")
	}

	// Should be restored
	if got := os.Getenv("TEST_WITH_VAR"); got != "original" {
		t.Errorf("After WithEnv: TEST_WITH_VAR = %q, want %q", got, "original")
	}
}

func TestDeploymentEnvKeys(t *testing.T) {
	keys := DeploymentEnvKeys()

	if len(keys) == 0 {
		t.Error("DeploymentEnvKeys() returned empty slice")
	}

	// Check for expected keys
	expected := []string{
		"KEYCLOAK_REALM",
		"OPTIMIZE_INDEX_PREFIX",
		"ORCHESTRATION_INDEX_PREFIX",
		"FLOW",
	}

	for _, exp := range expected {
		found := false
		for _, k := range keys {
			if k == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected key %q not found in DeploymentEnvKeys()", exp)
		}
	}
}

func TestCaptureEnv(t *testing.T) {
	os.Setenv("CAPTURE_VAR_1", "value1")
	os.Setenv("CAPTURE_VAR_2", "value2")
	os.Unsetenv("CAPTURE_VAR_3")
	defer func() {
		os.Unsetenv("CAPTURE_VAR_1")
		os.Unsetenv("CAPTURE_VAR_2")
	}()

	captured := captureEnv([]string{"CAPTURE_VAR_1", "CAPTURE_VAR_2", "CAPTURE_VAR_3"})

	if captured["CAPTURE_VAR_1"] != "value1" {
		t.Errorf("CAPTURE_VAR_1 = %q, want %q", captured["CAPTURE_VAR_1"], "value1")
	}
	if captured["CAPTURE_VAR_2"] != "value2" {
		t.Errorf("CAPTURE_VAR_2 = %q, want %q", captured["CAPTURE_VAR_2"], "value2")
	}
	if captured["CAPTURE_VAR_3"] != "" {
		t.Errorf("CAPTURE_VAR_3 = %q, want empty", captured["CAPTURE_VAR_3"])
	}
}

func TestRestoreEnv(t *testing.T) {
	os.Setenv("RESTORE_VAR_1", "modified")
	os.Setenv("RESTORE_VAR_2", "modified")
	defer func() {
		os.Unsetenv("RESTORE_VAR_1")
		os.Unsetenv("RESTORE_VAR_2")
	}()

	captured := map[string]string{
		"RESTORE_VAR_1": "original",
		"RESTORE_VAR_2": "", // Should unset
	}

	restoreEnv(captured)

	if got := os.Getenv("RESTORE_VAR_1"); got != "original" {
		t.Errorf("RESTORE_VAR_1 = %q, want %q", got, "original")
	}
	if got := os.Getenv("RESTORE_VAR_2"); got != "" {
		t.Errorf("RESTORE_VAR_2 = %q, want empty (unset)", got)
	}
}

func TestEnvScope_ConcurrentAccess(t *testing.T) {
	// Test that concurrent env modifications are serialized
	os.Setenv("CONCURRENT_VAR", "initial")
	defer os.Unsetenv("CONCURRENT_VAR")

	var wg sync.WaitGroup
	results := make([]string, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			scope := NewEnvScope([]string{"CONCURRENT_VAR"})
			expectedValue := string('A' + byte(idx))

			cleanup := scope.Apply(func() {
				os.Setenv("CONCURRENT_VAR", expectedValue)
			})

			// Read the value while we hold the lock
			results[idx] = os.Getenv("CONCURRENT_VAR")
			cleanup()
		}(i)
	}

	wg.Wait()

	// Each goroutine should have seen its own value
	for i, r := range results {
		expected := string('A' + byte(i))
		if r != expected {
			t.Errorf("Goroutine %d: got %q, want %q", i, r, expected)
		}
	}
}

