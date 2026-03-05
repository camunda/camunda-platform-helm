package deploy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"scripts/camunda-core/pkg/kube"
	"scripts/camunda-core/pkg/logging"
	"strings"
)

// postDeployResourcesDir is the subdirectory under the common scenarios directory
// that contains post-deploy resource templates.
const postDeployResourcesDir = "common/resources"

// scenariosRequiringPostDeploy lists scenario names that require post-deploy
// resource application. Resources from common/resources/ are applied for these scenarios.
var scenariosRequiringPostDeploy = map[string]bool{
	"gateway-keycloak": true,
}

// applyPostDeployResources applies post-deploy Kubernetes resources for scenarios
// that require them. It looks for YAML manifest templates in the common/resources/
// directory relative to the scenario path, substitutes environment variable placeholders
// ($NAMESPACE, $RELEASE_NAME), and applies them to the cluster.
func applyPostDeployResources(ctx context.Context, scenarioCtx *ScenarioContext, chartPath, kubeContext string) error {
	if !scenariosRequiringPostDeploy[scenarioCtx.ScenarioName] {
		logging.Logger.Debug().
			Str("scenario", scenarioCtx.ScenarioName).
			Msg("Scenario does not require post-deploy resources - skipping")
		return nil
	}

	resourcesDir := resolveResourcesDir(chartPath)
	if resourcesDir == "" {
		logging.Logger.Warn().
			Str("scenario", scenarioCtx.ScenarioName).
			Str("chartPath", chartPath).
			Msg("Could not find post-deploy resources directory - skipping")
		return nil
	}

	manifests, err := loadAndSubstituteManifests(resourcesDir, scenarioCtx.Namespace, scenarioCtx.Release)
	if err != nil {
		return fmt.Errorf("failed to load post-deploy manifests: %w", err)
	}

	if len(manifests) == 0 {
		logging.Logger.Debug().
			Str("scenario", scenarioCtx.ScenarioName).
			Str("resourcesDir", resourcesDir).
			Msg("No post-deploy manifests found")
		return nil
	}

	kubeClient, err := kube.NewClient("", kubeContext)
	if err != nil {
		return fmt.Errorf("failed to create kube client for post-deploy resources: %w", err)
	}

	for _, m := range manifests {
		logging.Logger.Info().
			Str("scenario", scenarioCtx.ScenarioName).
			Str("file", m.filename).
			Str("namespace", scenarioCtx.Namespace).
			Msg("Applying post-deploy resource")

		if err := kubeClient.ApplyManifest(ctx, scenarioCtx.Namespace, m.data); err != nil {
			return fmt.Errorf("failed to apply post-deploy resource %s: %w", m.filename, err)
		}

		logging.Logger.Info().
			Str("scenario", scenarioCtx.ScenarioName).
			Str("file", m.filename).
			Msg("Post-deploy resource applied successfully")
	}

	return nil
}

// manifest holds a loaded and substituted manifest ready for application.
type manifest struct {
	filename string
	data     []byte
}

// resolveResourcesDir finds the common/resources/ directory relative to the chart path.
// It checks the standard integration test scenarios path within the chart directory.
func resolveResourcesDir(chartPath string) string {
	if chartPath == "" {
		return ""
	}
	candidate := filepath.Join(chartPath, "test", "integration", "scenarios", postDeployResourcesDir)
	info, err := os.Stat(candidate)
	if err == nil && info.IsDir() {
		return candidate
	}
	return ""
}

// loadAndSubstituteManifests reads all YAML files from the resources directory
// and substitutes $NAMESPACE and $RELEASE_NAME placeholders with actual values.
func loadAndSubstituteManifests(resourcesDir, namespace, releaseName string) ([]manifest, error) {
	entries, err := os.ReadDir(resourcesDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read resources directory %s: %w", resourcesDir, err)
	}

	var manifests []manifest
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}

		filePath := filepath.Join(resourcesDir, name)
		raw, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read manifest file %s: %w", filePath, err)
		}

		substituted := substituteManifestVars(string(raw), namespace, releaseName)
		manifests = append(manifests, manifest{
			filename: name,
			data:     []byte(substituted),
		})
	}

	return manifests, nil
}

// substituteManifestVars replaces $NAMESPACE and $RELEASE_NAME placeholders
// in manifest content with the provided values.
func substituteManifestVars(content, namespace, releaseName string) string {
	result := strings.ReplaceAll(content, "$NAMESPACE", namespace)
	result = strings.ReplaceAll(result, "$RELEASE_NAME", releaseName)
	return result
}
