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
	if len(scenarioNames) == 0 {
		return nil, nil
	}

	// Check if this scenario directory uses layered values
	if scenarios.HasLayeredValues(scenarioDir) {
		return resolveLayeredScenarioFiles(scenarioDir, scenarioNames)
	}

	// Fall back to legacy single-file approach
	return resolveLegacyScenarioFiles(scenarioDir, scenarioNames)
}

// resolveLayeredScenarioFiles resolves scenarios using the layered values structure.
// For layered values, we combine all layer files for each scenario.
func resolveLayeredScenarioFiles(scenarioDir string, scenarioNames []string) ([]string, error) {
	var allFiles []string
	var missing []string

	for _, s := range scenarioNames {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}

		files, err := scenarios.ResolveLayeredPaths(scenarioDir, s, nil)
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

// ResolveScenarioFilesWithConfig resolves scenario files using an explicit layered config.
// This allows callers to specify exactly which layers to use, bypassing auto-detection.
func ResolveScenarioFilesWithConfig(scenarioDir string, config *scenarios.LayeredConfig) ([]string, error) {
	if config == nil {
		return nil, fmt.Errorf("layered config is required")
	}

	// Use an empty scenario name since config is explicit
	return scenarios.ResolveLayeredPaths(scenarioDir, "", config)
}
