package deployer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// BuildValuesList composes the values file list in precedence order:
// 1) Auth scenario values (if auth is provided, layered before main scenario)
// 2) Scenario values (must exist)
// 3) Optional overlays (enterprise/digest) when present
// 4) User-provided values (last overrides earlier)
func BuildValuesList(scenarioDir string, scenarios []string, auth string, includeEnterprise, includeDigest bool, userValues []string) ([]string, error) {
	var files []string
	
	// Add auth scenario first if provided
	if strings.TrimSpace(auth) != "" {
		authFiles, err := ResolveScenarioFiles(scenarioDir, []string{auth})
		if err != nil {
			return nil, fmt.Errorf("failed to resolve auth scenario %q: %w", auth, err)
		}
		files = append(files, authFiles...)
	}
	
	// Add main scenario values
	scenarioFiles, err := ResolveScenarioFiles(scenarioDir, scenarios)
	if err != nil {
		return nil, err
	}
	files = append(files, scenarioFiles...)
	
	// overlays
	if includeEnterprise {
		if f := overlayIfExists(scenarioDir, "values-enterprise.yaml"); f != "" {
			files = append(files, f)
		}
	}
	if includeDigest {
		if f := overlayIfExists(scenarioDir, "values-digest.yaml"); f != "" {
			files = append(files, f)
		}
	}
	// user last
	files = append(files, userValues...)
	return files, nil
}

func overlayIfExists(scenarioDir, fileName string) string {
	p := filepath.Join(scenarioDir, fileName)
	if _, err := os.Stat(p); err == nil {
		return p
	}
	return ""
}