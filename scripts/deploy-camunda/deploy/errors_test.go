package deploy

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestDeployError_Error(t *testing.T) {
	err := NewDeployError(ErrScenarioNotFound, "test message")
	if err.Error() != "test message" {
		t.Errorf("Error() = %q, want %q", err.Error(), "test message")
	}
}

func TestDeployError_Unwrap(t *testing.T) {
	cause := errors.New("root cause")
	err := NewDeployError(ErrDeploymentFailed, "wrapper").WithCause(cause)

	if errors.Unwrap(err) != cause {
		t.Error("Unwrap() did not return the cause")
	}

	// Test errors.Is
	if !errors.Is(err, cause) {
		t.Error("errors.Is() should find the cause")
	}
}

func TestDeployError_JSON(t *testing.T) {
	err := NewDeployError(ErrConfigInvalid, "invalid config").
		WithSuggestion("fix your config").
		WithDetail("field", "namespace")

	jsonStr := err.JSON()

	// Verify it's valid JSON
	var parsed map[string]interface{}
	if parseErr := json.Unmarshal([]byte(jsonStr), &parsed); parseErr != nil {
		t.Fatalf("JSON() output is not valid JSON: %v", parseErr)
	}

	// Verify fields
	if parsed["code"] != string(ErrConfigInvalid) {
		t.Errorf("JSON code = %v, want %v", parsed["code"], ErrConfigInvalid)
	}
	if parsed["message"] != "invalid config" {
		t.Errorf("JSON message = %v, want %v", parsed["message"], "invalid config")
	}
	if parsed["suggestion"] != "fix your config" {
		t.Errorf("JSON suggestion = %v, want %v", parsed["suggestion"], "fix your config")
	}
}

func TestDeployError_Format(t *testing.T) {
	err := NewDeployError(ErrScenarioNotFound, "scenario not found").
		WithSuggestion("use a valid scenario").
		WithDetail("scenario", "missing").
		WithCause(errors.New("file not found"))

	formatted := err.Format()

	// Should contain the message
	if !strings.Contains(formatted, "scenario not found") {
		t.Error("Format() should contain the message")
	}

	// Should contain the error code
	if !strings.Contains(formatted, string(ErrScenarioNotFound)) {
		t.Error("Format() should contain the error code")
	}

	// Should contain the suggestion
	if !strings.Contains(formatted, "use a valid scenario") {
		t.Error("Format() should contain the suggestion")
	}

	// Should contain the detail
	if !strings.Contains(formatted, "scenario") {
		t.Error("Format() should contain the detail key")
	}

	// Should contain the cause
	if !strings.Contains(formatted, "file not found") {
		t.Error("Format() should contain the cause")
	}
}

func TestDeployError_Builder(t *testing.T) {
	// Test fluent builder pattern
	err := NewDeployError(ErrChartNotFound, "chart not found").
		WithSuggestion("check the path").
		WithDetail("path", "/some/path").
		WithDetail("expected", "Chart.yaml")

	if err.Code != ErrChartNotFound {
		t.Errorf("Code = %v, want %v", err.Code, ErrChartNotFound)
	}
	if err.Message != "chart not found" {
		t.Errorf("Message = %q, want %q", err.Message, "chart not found")
	}
	if err.Suggestion != "check the path" {
		t.Errorf("Suggestion = %q, want %q", err.Suggestion, "check the path")
	}
	if len(err.Details) != 2 {
		t.Errorf("Details length = %d, want 2", len(err.Details))
	}
	if err.Details["path"] != "/some/path" {
		t.Errorf("Details[path] = %q, want %q", err.Details["path"], "/some/path")
	}
}

func TestErrScenarioNotFoundError(t *testing.T) {
	available := []string{"keycloak", "saas"}
	err := ErrScenarioNotFoundError("missing", "/path/to/scenarios", available)

	if err.Code != ErrScenarioNotFound {
		t.Errorf("Code = %v, want %v", err.Code, ErrScenarioNotFound)
	}
	if !strings.Contains(err.Message, "missing") {
		t.Errorf("Message should contain scenario name")
	}
	if !strings.Contains(err.Suggestion, "keycloak") {
		t.Errorf("Suggestion should list available scenarios")
	}
}

func TestErrChartNotFoundError(t *testing.T) {
	err := ErrChartNotFoundError("/bad/path")

	if err.Code != ErrChartNotFound {
		t.Errorf("Code = %v, want %v", err.Code, ErrChartNotFound)
	}
	if err.Details["chartPath"] != "/bad/path" {
		t.Errorf("Details[chartPath] = %q, want %q", err.Details["chartPath"], "/bad/path")
	}
}

func TestErrDeploymentFailedError(t *testing.T) {
	cause := errors.New("helm error")
	err := ErrDeploymentFailedError("test-ns", "test-release", cause)

	if err.Code != ErrDeploymentFailed {
		t.Errorf("Code = %v, want %v", err.Code, ErrDeploymentFailed)
	}
	if err.Details["namespace"] != "test-ns" {
		t.Errorf("Details[namespace] = %q, want %q", err.Details["namespace"], "test-ns")
	}
	if err.Cause != cause {
		t.Error("Cause should be set")
	}
	if !strings.Contains(err.Suggestion, "kubectl logs") {
		t.Error("Suggestion should include kubectl command")
	}
}

func TestIsDeployError(t *testing.T) {
	// Test with DeployError
	de := NewDeployError(ErrConfigInvalid, "test")
	result, ok := IsDeployError(de)
	if !ok {
		t.Error("IsDeployError should return true for DeployError")
	}
	if result != de {
		t.Error("IsDeployError should return the same error")
	}

	// Test with regular error
	stdErr := errors.New("standard error")
	result, ok = IsDeployError(stdErr)
	if ok {
		t.Error("IsDeployError should return false for regular error")
	}
	if result != nil {
		t.Error("IsDeployError should return nil for regular error")
	}
}

func TestWrapError(t *testing.T) {
	// Test wrapping a regular error
	stdErr := errors.New("standard error")
	wrapped := WrapError(ErrDeploymentFailed, "deployment failed", stdErr)

	if wrapped.Code != ErrDeploymentFailed {
		t.Errorf("Code = %v, want %v", wrapped.Code, ErrDeploymentFailed)
	}
	if wrapped.Cause != stdErr {
		t.Error("Cause should be the original error")
	}

	// Test wrapping a DeployError (should return as-is)
	de := NewDeployError(ErrConfigInvalid, "already wrapped")
	wrapped = WrapError(ErrDeploymentFailed, "should not rewrap", de)

	if wrapped.Code != ErrConfigInvalid {
		t.Errorf("WrapError should not rewrap DeployError, Code = %v, want %v", wrapped.Code, ErrConfigInvalid)
	}
}

