package format

import (
	"encoding/json"
	"fmt"
	"os"
)

// DeploymentResult represents the result of a deployment in a structured format.
type DeploymentResult struct {
	Status      string               `json:"status"` // success, failed
	Scenario    string               `json:"scenario"`
	Namespace   string               `json:"namespace"`
	Release     string               `json:"release"`
	IngressHostname string               `json:"ingressHostname,omitempty"`
	Identifiers DeploymentIdentifiers `json:"identifiers"`
	Credentials DeploymentCredentials `json:"credentials,omitempty"`
	Error       string               `json:"error,omitempty"`
}

// DeploymentIdentifiers holds the unique identifiers generated for a deployment.
type DeploymentIdentifiers struct {
	KeycloakRealm            string `json:"keycloakRealm"`
	OptimizeIndexPrefix      string `json:"optimizeIndexPrefix"`
	OrchestrationIndexPrefix string `json:"orchestrationIndexPrefix"`
	TasklistIndexPrefix      string `json:"tasklistIndexPrefix,omitempty"`
	OperateIndexPrefix       string `json:"operateIndexPrefix,omitempty"`
}

// DeploymentCredentials holds the generated credentials for a deployment.
type DeploymentCredentials struct {
	FirstUserPassword     string `json:"firstUserPassword,omitempty"`
	SecondUserPassword    string `json:"secondUserPassword,omitempty"`
	ThirdUserPassword     string `json:"thirdUserPassword,omitempty"`
	KeycloakClientsSecret string `json:"keycloakClientsSecret,omitempty"`
}

// MultiDeploymentResult represents the result of multiple parallel deployments.
type MultiDeploymentResult struct {
	Status       string             `json:"status"` // success, partial, failed
	TotalCount   int                `json:"totalCount"`
	SuccessCount int                `json:"successCount"`
	FailedCount  int                `json:"failedCount"`
	Deployments  []DeploymentResult `json:"deployments"`
}

// ValidationResult represents the result of a validation check.
type ValidationResult struct {
	Status     string            `json:"status"` // pass, fail
	ConfigFile string            `json:"configFile,omitempty"`
	Checks     []ValidationCheck `json:"checks"`
	Summary    ValidationSummary `json:"summary"`
}

// ValidationCheck represents a single validation check.
type ValidationCheck struct {
	Name    string   `json:"name"`
	Status  string   `json:"status"` // pass, warn, fail
	Message string   `json:"message"`
	Details []string `json:"details,omitempty"`
}

// ValidationSummary provides counts of validation check results.
type ValidationSummary struct {
	PassCount int `json:"passCount"`
	WarnCount int `json:"warnCount"`
	FailCount int `json:"failCount"`
}

// ConfigListResult represents the result of listing configurations.
type ConfigListResult struct {
	ConfigFile  string          `json:"configFile"`
	Current     string          `json:"current,omitempty"`
	Deployments []ConfigSummary `json:"deployments"`
}

// ConfigSummary provides a summary of a deployment configuration.
type ConfigSummary struct {
	Name      string `json:"name"`
	Chart     string `json:"chart,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Scenario  string `json:"scenario,omitempty"`
	IsCurrent bool   `json:"isCurrent"`
}

// PrintJSON outputs any value as formatted JSON to stdout.
func PrintJSON(v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	fmt.Fprintln(os.Stdout, string(data))
	return nil
}

// MustPrintJSON outputs any value as formatted JSON to stdout, panicking on error.
func MustPrintJSON(v interface{}) {
	if err := PrintJSON(v); err != nil {
		panic(err)
	}
}

// NewSuccessResult creates a successful deployment result.
func NewSuccessResult(scenario, namespace, release, ingressHostname string) *DeploymentResult {
	return &DeploymentResult{
		Status:          "success",
		Scenario:        scenario,
		Namespace:       namespace,
		Release:         release,
		IngressHostname: ingressHostname,
	}
}

// NewFailedResult creates a failed deployment result.
func NewFailedResult(scenario, namespace, release string, err error) *DeploymentResult {
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}
	return &DeploymentResult{
		Status:    "failed",
		Scenario:  scenario,
		Namespace: namespace,
		Release:   release,
		Error:     errMsg,
	}
}

// WithIdentifiers adds identifiers to a deployment result.
func (r *DeploymentResult) WithIdentifiers(realm, optimizePrefix, orchestrationPrefix, tasklistPrefix, operatePrefix string) *DeploymentResult {
	r.Identifiers = DeploymentIdentifiers{
		KeycloakRealm:            realm,
		OptimizeIndexPrefix:      optimizePrefix,
		OrchestrationIndexPrefix: orchestrationPrefix,
		TasklistIndexPrefix:      tasklistPrefix,
		OperateIndexPrefix:       operatePrefix,
	}
	return r
}

// WithCredentials adds credentials to a deployment result.
func (r *DeploymentResult) WithCredentials(firstPwd, secondPwd, thirdPwd, clientSecret string) *DeploymentResult {
	r.Credentials = DeploymentCredentials{
		FirstUserPassword:     firstPwd,
		SecondUserPassword:    secondPwd,
		ThirdUserPassword:     thirdPwd,
		KeycloakClientsSecret: clientSecret,
	}
	return r
}

// NewMultiDeploymentResult creates a multi-deployment result from individual results.
func NewMultiDeploymentResult(results []*DeploymentResult) *MultiDeploymentResult {
	successCount := 0
	failedCount := 0
	deployments := make([]DeploymentResult, len(results))

	for i, r := range results {
		deployments[i] = *r
		if r.Status == "success" {
			successCount++
		} else {
			failedCount++
		}
	}

	status := "success"
	if failedCount == len(results) {
		status = "failed"
	} else if failedCount > 0 {
		status = "partial"
	}

	return &MultiDeploymentResult{
		Status:       status,
		TotalCount:   len(results),
		SuccessCount: successCount,
		FailedCount:  failedCount,
		Deployments:  deployments,
	}
}

