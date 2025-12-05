package deploy

import (
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"scripts/camunda-core/pkg/logging"
	"scripts/deploy-camunda/config"
	"scripts/deploy-camunda/internal/util"
	"strings"
)

// ScenarioContext holds scenario-specific deployment configuration.
type ScenarioContext struct {
	ScenarioName             string
	Namespace                string
	Release                  string
	IngressHostname          string
	KeycloakRealm            string
	OptimizeIndexPrefix      string
	OrchestrationIndexPrefix string
	TasklistIndexPrefix      string
	OperateIndexPrefix       string
	TempDir                  string
}

// ScenarioResult holds the result of a scenario deployment.
type ScenarioResult struct {
	Scenario                 string
	Namespace                string
	Release                  string
	IngressHostname          string
	KeycloakRealm            string
	OptimizeIndexPrefix      string
	OrchestrationIndexPrefix string
	FirstUserPassword        string
	SecondUserPassword       string
	ThirdUserPassword        string
	KeycloakClientsSecret    string
	Error                    error
}

// generateCompactRealmName creates a realm name that fits within Keycloak's character limit.
// Format: {prefix}-{hash} where hash is derived from namespace+scenario+suffix.
func generateCompactRealmName(namespace, scenario, suffix string) string {
	const maxLength = MaxRealmNameLength

	// Try simple format first: scenario-suffix (e.g., "keycloak-mt-a8x9z3k1")
	simple := fmt.Sprintf("%s-%s", scenario, suffix)
	if len(simple) <= maxLength {
		return simple
	}

	// If scenario name is too long, truncate it and add a short hash for uniqueness
	// Format: {truncated-scenario}-{short-hash}
	// Reserve 9 characters for "-" + 8 char hash
	maxScenarioLen := maxLength - 9

	if len(scenario) > maxScenarioLen {
		scenario = scenario[:maxScenarioLen]
	}

	// Create a short hash from the full identifier for uniqueness
	fullId := fmt.Sprintf("%s-%s-%s", namespace, scenario, suffix)
	hash := fmt.Sprintf("%x", big.NewInt(0).SetBytes([]byte(fullId)).Int64())
	if len(hash) > 8 {
		hash = hash[:8]
	}

	result := fmt.Sprintf("%s-%s", scenario, hash)

	// Final safety check - truncate if still too long
	if len(result) > maxLength {
		result = result[:maxLength]
	}

	return result
}

// buildIngressHostname constructs the full ingress hostname based on configuration.
// Priority: 1) IngressHostname (full override), 2) IngressSubdomain + base domain
// For multi-scenario deployments, the scenario name is prepended to the subdomain.
func buildIngressHostname(scenario string, flags *config.RuntimeFlags, isMultiScenario bool) string {
	// Full hostname override takes highest priority
	if flags.IngressHostname != "" {
		return flags.IngressHostname
	}

	// If no subdomain provided, return empty
	if flags.IngressSubdomain == "" {
		return ""
	}

	// Build hostname from subdomain + base domain
	subdomain := flags.IngressSubdomain
	if isMultiScenario {
		// For multi-scenario, prepend scenario name to subdomain
		subdomain = fmt.Sprintf("%s-%s", scenario, flags.IngressSubdomain)
	}

	return fmt.Sprintf("%s.%s", subdomain, DefaultIngressBaseDomain)
}

// generateScenarioContext creates a scenario-specific deployment context.
func generateScenarioContext(scenario string, flags *config.RuntimeFlags) *ScenarioContext {
	suffix := util.GenerateRandomSuffix()

	// Generate unique identifiers for this scenario
	var realmName, optimizePrefix, orchestrationPrefix, tasklistPrefix, operatePrefix string
	var namespace, release string

	// Use provided values or generate unique ones
	if flags.KeycloakRealm != "" && len(flags.Scenarios) == 1 {
		realmName = flags.KeycloakRealm
	} else {
		// Keycloak realm name has a maximum length of 36 characters
		// Generate a compact name that fits within this limit
		realmName = generateCompactRealmName(flags.Namespace, scenario, suffix)
	}

	if flags.OptimizeIndexPrefix != "" && len(flags.Scenarios) == 1 {
		optimizePrefix = flags.OptimizeIndexPrefix
	} else {
		optimizePrefix = fmt.Sprintf("opt-%s-%s", scenario, suffix)
	}

	if flags.OrchestrationIndexPrefix != "" && len(flags.Scenarios) == 1 {
		orchestrationPrefix = flags.OrchestrationIndexPrefix
	} else {
		orchestrationPrefix = fmt.Sprintf("orch-%s-%s", scenario, suffix)
	}

	if flags.TasklistIndexPrefix != "" && len(flags.Scenarios) == 1 {
		tasklistPrefix = flags.TasklistIndexPrefix
	} else {
		tasklistPrefix = fmt.Sprintf("task-%s-%s", scenario, suffix)
	}

	if flags.OperateIndexPrefix != "" && len(flags.Scenarios) == 1 {
		operatePrefix = flags.OperateIndexPrefix
	} else {
		operatePrefix = fmt.Sprintf("op-%s-%s", scenario, suffix)
	}

	// Determine if this is a multi-scenario deployment
	isMultiScenario := len(flags.Scenarios) > 1

	// Generate unique namespace for multi-scenario, but always use "integration" as release name
	// since we never have multiple deployments in the same namespace
	if isMultiScenario {
		namespace = fmt.Sprintf("%s-%s", flags.Namespace, scenario)
	} else {
		namespace = flags.Namespace
	}

	// Build the ingress hostname
	ingressHostname := buildIngressHostname(scenario, flags, isMultiScenario)

	// Always use default release name
	release = DefaultReleaseName

	return &ScenarioContext{
		ScenarioName:             scenario,
		Namespace:                namespace,
		Release:                  release,
		IngressHostname:          ingressHostname,
		KeycloakRealm:            realmName,
		OptimizeIndexPrefix:      optimizePrefix,
		OrchestrationIndexPrefix: orchestrationPrefix,
		TasklistIndexPrefix:      tasklistPrefix,
		OperateIndexPrefix:       operatePrefix,
	}
}

// enhanceScenarioError wraps scenario resolution errors with helpful context.
func enhanceScenarioError(err error, scenario, scenarioPath, chartPath string) error {
	if err == nil {
		return nil
	}

	// Check if it's a "not found" type error
	errStr := err.Error()
	if !strings.Contains(errStr, "not found") && !strings.Contains(errStr, "no such file") {
		return err
	}

	// Try to list available scenarios
	scenarioDir := scenarioPath
	if scenarioDir == "" {
		scenarioDir = filepath.Join(chartPath, DefaultScenarioSubdir)
	}

	availableScenarios := listAvailableScenarios(scenarioDir)
	return ErrScenarioNotFoundError(scenario, scenarioDir, availableScenarios).WithCause(err)
}

// listAvailableScenarios returns a list of scenario names found in the given directory.
func listAvailableScenarios(scenarioDir string) []string {
	entries, err := os.ReadDir(scenarioDir)
	if err != nil {
		return nil
	}

	var scenarios []string
	for _, e := range entries {
		name := e.Name()
		if !e.IsDir() && strings.HasPrefix(name, ScenarioFilePrefix) && strings.HasSuffix(name, ScenarioFileSuffix) {
			scenarioName := strings.TrimPrefix(name, ScenarioFilePrefix)
			scenarioName = strings.TrimSuffix(scenarioName, ScenarioFileSuffix)
			scenarios = append(scenarios, scenarioName)
		}
	}
	return scenarios
}

// validateScenarios checks that all scenario files exist before deployment.
func validateScenarios(flags *config.RuntimeFlags) error {
	scenarioDir := flags.ScenarioPath
	if scenarioDir == "" {
		scenarioDir = filepath.Join(flags.ChartPath, DefaultScenarioSubdir)
	}

	for _, scenario := range flags.Scenarios {
		var filename string
		if strings.HasPrefix(scenario, ScenarioFilePrefix) && strings.HasSuffix(scenario, ScenarioFileSuffix) {
			filename = scenario
		} else {
			filename = fmt.Sprintf("%s%s%s", ScenarioFilePrefix, scenario, ScenarioFileSuffix)
		}

		sourceValuesFile := filepath.Join(scenarioDir, filename)
		if _, err := os.Stat(sourceValuesFile); err != nil {
			return enhanceScenarioError(err, scenario, flags.ScenarioPath, flags.ChartPath)
		}
	}

	logging.Logger.Info().Msg("All scenarios validated successfully")
	return nil
}

