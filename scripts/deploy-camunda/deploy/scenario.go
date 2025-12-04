package deploy

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"scripts/camunda-core/pkg/logging"
	"scripts/deploy-camunda/config"
	"strings"
)

// ScenarioContext holds scenario-specific deployment configuration.
type ScenarioContext struct {
	ScenarioName             string
	Namespace                string
	Release                  string
	IngressHost              string
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
	IngressHost              string
	KeycloakRealm            string
	OptimizeIndexPrefix      string
	OrchestrationIndexPrefix string
	FirstUserPassword        string
	SecondUserPassword       string
	ThirdUserPassword        string
	KeycloakClientsSecret    string
	Error                    error
}

// generateRandomSuffix creates a random string of RandomSuffixLength characters.
func generateRandomSuffix() string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	result := make([]byte, RandomSuffixLength)
	for i := range result {
		num, _ := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		result[i] = chars[num.Int64()]
	}
	return string(result)
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

// generateScenarioContext creates a scenario-specific deployment context.
func generateScenarioContext(scenario string, flags *config.RuntimeFlags) *ScenarioContext {
	suffix := generateRandomSuffix()

	// Generate unique identifiers for this scenario
	var realmName, optimizePrefix, orchestrationPrefix, tasklistPrefix, operatePrefix string
	var namespace, release, ingressHost string

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

	// Generate unique namespace for multi-scenario, but always use "integration" as release name
	// since we never have multiple deployments in the same namespace
	if len(flags.Scenarios) > 1 {
		namespace = fmt.Sprintf("%s-%s", flags.Namespace, scenario)
		if flags.IngressHost != "" {
			ingressHost = fmt.Sprintf("%s-%s", scenario, flags.IngressHost)
		} else {
			ingressHost = flags.IngressHost
		}
	} else {
		namespace = flags.Namespace
		ingressHost = flags.IngressHost
	}

	// Always use default release name
	release = DefaultReleaseName

	return &ScenarioContext{
		ScenarioName:             scenario,
		Namespace:                namespace,
		Release:                  release,
		IngressHost:              ingressHost,
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

	var helpMsg strings.Builder
	fmt.Fprintf(&helpMsg, "Scenario %q not found\n\n", scenario)
	fmt.Fprintf(&helpMsg, "Searched in: %s\n", scenarioDir)
	fmt.Fprintf(&helpMsg, "Expected file: %s%s%s\n\n", ScenarioFilePrefix, scenario, ScenarioFileSuffix)

	// Try to list available scenarios
	entries, readErr := os.ReadDir(scenarioDir)
	if readErr != nil {
		fmt.Fprintf(&helpMsg, "Could not list available scenarios: %v\n\n", readErr)
		fmt.Fprintf(&helpMsg, "Please check:\n")
		fmt.Fprintf(&helpMsg, "  1. The scenario directory exists: %s\n", scenarioDir)
		fmt.Fprintf(&helpMsg, "  2. You have permission to read it\n")
		fmt.Fprintf(&helpMsg, "  3. The --chart-path or --scenario-path flags are set correctly\n")
	} else {
		var availableScenarios []string
		for _, e := range entries {
			name := e.Name()
			if !e.IsDir() && strings.HasPrefix(name, ScenarioFilePrefix) && strings.HasSuffix(name, ScenarioFileSuffix) {
				// Extract scenario name
				scenarioName := strings.TrimPrefix(name, ScenarioFilePrefix)
				scenarioName = strings.TrimSuffix(scenarioName, ScenarioFileSuffix)
				availableScenarios = append(availableScenarios, scenarioName)
			}
		}

		if len(availableScenarios) == 0 {
			fmt.Fprintf(&helpMsg, "No scenario files found in: %s\n\n", scenarioDir)
			fmt.Fprintf(&helpMsg, "Expected files matching pattern: %s*%s\n", ScenarioFilePrefix, ScenarioFileSuffix)
		} else {
			fmt.Fprintf(&helpMsg, "Available scenarios (%d found):\n", len(availableScenarios))
			for _, s := range availableScenarios {
				fmt.Fprintf(&helpMsg, "  - %s\n", s)
			}
			fmt.Fprintf(&helpMsg, "\nHint: Use --scenario <name> or --scenario <name1>,<name2> for multiple scenarios\n")
		}
	}

	fmt.Fprintf(&helpMsg, "\nDocumentation: Check the chart's test/integration/scenarios/ directory\n")
	fmt.Fprintf(&helpMsg, "for available scenario configurations.\n")

	return fmt.Errorf("%s\n%s", helpMsg.String(), err)
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

