package deploy

import (
	"encoding/json"
	"fmt"
	"strings"

	"scripts/camunda-core/pkg/logging"

	"github.com/jwalton/gchalk"
)

// ErrorCode represents a machine-readable error identifier.
type ErrorCode string

// Error codes for deploy-camunda operations.
const (
	ErrScenarioNotFound    ErrorCode = "SCENARIO_NOT_FOUND"
	ErrConfigInvalid       ErrorCode = "CONFIG_INVALID"
	ErrChartNotFound       ErrorCode = "CHART_NOT_FOUND"
	ErrNamespaceRequired   ErrorCode = "NAMESPACE_REQUIRED"
	ErrReleaseRequired     ErrorCode = "RELEASE_REQUIRED"
	ErrDeploymentFailed    ErrorCode = "DEPLOYMENT_FAILED"
	ErrValuesPreparation   ErrorCode = "VALUES_PREPARATION_FAILED"
	ErrSecretsGeneration   ErrorCode = "SECRETS_GENERATION_FAILED"
	ErrNamespaceDeletion   ErrorCode = "NAMESPACE_DELETION_FAILED"
	ErrKeycloakSetup       ErrorCode = "KEYCLOAK_SETUP_FAILED"
	ErrHelmTimeout         ErrorCode = "HELM_TIMEOUT"
	ErrInvalidFlag         ErrorCode = "INVALID_FLAG"
	ErrConfigNotFound      ErrorCode = "CONFIG_NOT_FOUND"
	ErrDeploymentNotFound  ErrorCode = "DEPLOYMENT_NOT_FOUND"
	ErrParallelDeployment  ErrorCode = "PARALLEL_DEPLOYMENT_FAILED"
)

// DeployError represents a structured error with actionable guidance.
type DeployError struct {
	Code       ErrorCode         `json:"code"`
	Message    string            `json:"message"`
	Suggestion string            `json:"suggestion,omitempty"`
	Details    map[string]string `json:"details,omitempty"`
	Cause      error             `json:"-"`
}

// Error implements the error interface.
func (e *DeployError) Error() string {
	return e.Message
}

// Unwrap returns the underlying cause for errors.Is/As support.
func (e *DeployError) Unwrap() error {
	return e.Cause
}

// JSON returns the error as a JSON string for machine-readable output.
func (e *DeployError) JSON() string {
	data, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return fmt.Sprintf(`{"code": "%s", "message": "%s"}`, e.Code, e.Message)
	}
	return string(data)
}

// Format returns a human-readable formatted string with colors.
func (e *DeployError) Format() string {
	styleErr := func(s string) string { return logging.Emphasize(s, gchalk.Red) }
	styleCode := func(s string) string { return logging.Emphasize(s, gchalk.Dim) }
	styleHint := func(s string) string { return logging.Emphasize(s, gchalk.Yellow) }
	styleKey := func(s string) string { return logging.Emphasize(s, gchalk.Cyan) }

	var b strings.Builder
	b.WriteString(styleErr("Error: "))
	b.WriteString(e.Message)
	b.WriteString(" ")
	b.WriteString(styleCode(fmt.Sprintf("[%s]", e.Code)))
	b.WriteString("\n")

	if len(e.Details) > 0 {
		b.WriteString("\nDetails:\n")
		for k, v := range e.Details {
			fmt.Fprintf(&b, "  %s: %s\n", styleKey(k), v)
		}
	}

	if e.Suggestion != "" {
		b.WriteString("\n")
		b.WriteString(styleHint("Suggestion: "))
		b.WriteString(e.Suggestion)
		b.WriteString("\n")
	}

	if e.Cause != nil {
		b.WriteString("\n")
		b.WriteString(styleCode("Caused by: "))
		b.WriteString(e.Cause.Error())
		b.WriteString("\n")
	}

	return b.String()
}

// NewDeployError creates a new DeployError with the given code and message.
func NewDeployError(code ErrorCode, message string) *DeployError {
	return &DeployError{
		Code:    code,
		Message: message,
		Details: make(map[string]string),
	}
}

// WithSuggestion adds a suggestion to the error.
func (e *DeployError) WithSuggestion(suggestion string) *DeployError {
	e.Suggestion = suggestion
	return e
}

// WithDetail adds a detail key-value pair to the error.
func (e *DeployError) WithDetail(key, value string) *DeployError {
	if e.Details == nil {
		e.Details = make(map[string]string)
	}
	e.Details[key] = value
	return e
}

// WithCause sets the underlying cause of the error.
func (e *DeployError) WithCause(err error) *DeployError {
	e.Cause = err
	return e
}

// Common error constructors for frequently used error types.

// ErrScenarioNotFoundError creates a scenario not found error with helpful context.
func ErrScenarioNotFoundError(scenario, searchPath string, available []string) *DeployError {
	err := NewDeployError(ErrScenarioNotFound, fmt.Sprintf("Scenario %q not found", scenario)).
		WithDetail("scenario", scenario).
		WithDetail("searchPath", searchPath)

	if len(available) > 0 {
		err.WithDetail("availableScenarios", strings.Join(available, ", "))
		err.WithSuggestion(fmt.Sprintf("Use one of the available scenarios: %s", strings.Join(available, ", ")))
	} else {
		err.WithSuggestion("Check that --chart-path or --scenario-path points to a directory containing scenario files")
	}

	return err
}

// ErrChartNotFoundError creates a chart not found error.
func ErrChartNotFoundError(chartPath string) *DeployError {
	return NewDeployError(ErrChartNotFound, fmt.Sprintf("Chart path %q does not exist or is not a directory", chartPath)).
		WithDetail("chartPath", chartPath).
		WithSuggestion("Verify the path exists and contains a valid Helm chart with Chart.yaml")
}

// ErrConfigInvalidError creates a configuration validation error.
func ErrConfigInvalidError(field, reason string) *DeployError {
	return NewDeployError(ErrConfigInvalid, fmt.Sprintf("Invalid configuration: %s", reason)).
		WithDetail("field", field).
		WithSuggestion(fmt.Sprintf("Provide %s via flag, environment variable, or config file", field))
}

// ErrNamespaceRequiredError creates a namespace required error.
func ErrNamespaceRequiredError() *DeployError {
	return NewDeployError(ErrNamespaceRequired, "Namespace is required but not set").
		WithSuggestion("Provide namespace via --namespace flag, config file, or CAMUNDA_NAMESPACE env var")
}

// ErrReleaseRequiredError creates a release required error.
func ErrReleaseRequiredError() *DeployError {
	return NewDeployError(ErrReleaseRequired, "Release name is required but not set").
		WithSuggestion("Provide release via --release flag or config file")
}

// ErrDeploymentFailedError creates a deployment failure error.
func ErrDeploymentFailedError(namespace, release string, cause error) *DeployError {
	return NewDeployError(ErrDeploymentFailed, "Helm deployment failed").
		WithDetail("namespace", namespace).
		WithDetail("release", release).
		WithCause(cause).
		WithSuggestion("Check pod logs with: kubectl logs -n " + namespace + " -l app.kubernetes.io/instance=" + release)
}

// ErrValuesPreparationError creates a values preparation error.
func ErrValuesPreparationError(scenario string, cause error) *DeployError {
	return NewDeployError(ErrValuesPreparation, fmt.Sprintf("Failed to prepare values for scenario %q", scenario)).
		WithDetail("scenario", scenario).
		WithCause(cause).
		WithSuggestion("Check that all required environment variables are set and the scenario file is valid YAML")
}

// ErrHelmTimeoutError creates a Helm timeout error.
func ErrHelmTimeoutError(namespace, release string, timeout string) *DeployError {
	return NewDeployError(ErrHelmTimeout, "Helm deployment timed out waiting for resources").
		WithDetail("namespace", namespace).
		WithDetail("release", release).
		WithDetail("timeout", timeout).
		WithSuggestion("Increase timeout with --timeout flag or check pod status with: kubectl get pods -n " + namespace)
}

// ErrParallelDeploymentError creates a parallel deployment error.
func ErrParallelDeploymentError(failed, total int) *DeployError {
	return NewDeployError(ErrParallelDeployment, fmt.Sprintf("%d of %d scenarios failed deployment", failed, total)).
		WithDetail("failed", fmt.Sprintf("%d", failed)).
		WithDetail("total", fmt.Sprintf("%d", total)).
		WithSuggestion("Check individual scenario logs above for specific failures")
}

// ErrDeploymentNotFoundError creates a deployment not found error.
func ErrDeploymentNotFoundError(name string, available []string) *DeployError {
	err := NewDeployError(ErrDeploymentNotFound, fmt.Sprintf("Deployment profile %q not found in config", name)).
		WithDetail("requested", name)

	if len(available) > 0 {
		err.WithDetail("available", strings.Join(available, ", "))
		err.WithSuggestion(fmt.Sprintf("Use one of: %s, or create a new deployment with 'deploy-camunda init'", strings.Join(available, ", ")))
	} else {
		err.WithSuggestion("Create a configuration with 'deploy-camunda init'")
	}

	return err
}

// IsDeployError checks if an error is a DeployError and returns it.
func IsDeployError(err error) (*DeployError, bool) {
	if de, ok := err.(*DeployError); ok {
		return de, true
	}
	return nil, false
}

// WrapError wraps a standard error into a DeployError if it isn't one already.
func WrapError(code ErrorCode, message string, err error) *DeployError {
	if de, ok := IsDeployError(err); ok {
		return de
	}
	return NewDeployError(code, message).WithCause(err)
}

