package deployer

import (
	"fmt"
	"os"
	"path/filepath"
	"scripts/camunda-core/pkg/scenarios"
	"strings"
)

type ScenarioMeta struct {
	Name string
	Path string
	Desc string
}

// ResolveScenarioFiles resolves scenario names to their values file paths.
// It supports both layered values (values/ directory) and legacy single-file approach.
func ResolveScenarioFiles(scenarioDir string, scenarioNames []string) ([]string, error) {
	return ResolveScenarioFilesWithConfig(scenarioDir, scenarioNames, nil)
}

// ResolveScenarioFilesWithConfig resolves scenario names to their values file paths,
// using the provided deployment config for layered values resolution.
// This allows passing ChartVersion and Flow for migrator detection.
func ResolveScenarioFilesWithConfig(scenarioDir string, scenarioNames []string, config *scenarios.DeploymentConfig) ([]string, error) {
	if len(scenarioNames) == 0 {
		return nil, nil
	}

	// Check if this scenario directory uses layered values
	if scenarios.HasLayeredValues(scenarioDir) {
		return resolveLayeredScenarioFiles(scenarioDir, scenarioNames, config)
	}

	// Fall back to legacy single-file approach
	return resolveLegacyScenarioFiles(scenarioDir, scenarioNames)
}

// resolveLayeredScenarioFiles resolves scenarios using the layered values structure.
// For layered values, we combine all layer files for each scenario.
func resolveLayeredScenarioFiles(scenarioDir string, scenarioNames []string, baseConfig *scenarios.DeploymentConfig) ([]string, error) {
	var allFiles []string
	var missing []string

	for _, s := range scenarioNames {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}

		// Start with the provided config or derive from scenario name
		var config *scenarios.DeploymentConfig
		if baseConfig != nil {
			// Use the provided config (which includes ChartVersion and Flow)
			config = baseConfig
		}

		files, err := scenarios.ResolveLayeredPaths(scenarioDir, s, config)
		if err != nil {
			missing = append(missing, s)
			continue
		}

		allFiles = append(allFiles, files...)
	}

	if len(missing) > 0 {
		var errMsgs []string
		for _, miss := range missing {
			errMsgs = append(errMsgs, fmt.Sprintf("could not resolve layered values for scenario %q", miss))
		}
		return nil, fmt.Errorf("%s", strings.Join(errMsgs, "; "))
	}

	return allFiles, nil
}

// resolveLegacyScenarioFiles resolves scenarios using the legacy single-file approach.
func resolveLegacyScenarioFiles(scenarioDir string, scenarioNames []string) ([]string, error) {
	var files []string
	var missing []string

	for _, s := range scenarioNames {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		p := filepath.Join(scenarioDir, fmt.Sprintf("values-integration-test-ingress-%s.yaml", s))
		if _, err := os.Stat(p); err != nil {
			missing = append(missing, s)
			continue
		}
		files = append(files, p)
	}

	if len(missing) > 0 {
		var errMsgs []string
		for _, miss := range missing {
			filePath := filepath.Join(scenarioDir, fmt.Sprintf("values-integration-test-ingress-%s.yaml", miss))
			errMsgs = append(errMsgs, fmt.Sprintf("missing scenario values file: %s", filePath))
		}
		return nil, fmt.Errorf("%s", strings.Join(errMsgs, "; "))
	}

	return files, nil
}
