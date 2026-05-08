package deploy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"scripts/camunda-core/pkg/kube"
	"scripts/camunda-core/pkg/logging"
	"sort"
	"strings"
)

// lifecycleResourcesDir is the subdirectory under the chart's integration
// scenarios directory that contains lifecycle (pre-install / post-deploy)
// manifest templates.
const lifecycleResourcesDir = "common/resources"

// scenariosRequiringPostDeploy lists scenario names that require post-deploy
// resource application. Resources from common/resources/ are applied for these scenarios.
var scenariosRequiringPostDeploy = map[string]bool{
	"gateway-keycloak": true,
}

// applyPostDeployResources applies post-deploy Kubernetes resources for scenarios
// that require them. It loads every YAML manifest in the chart's
// common/resources/ directory, substitutes $NAMESPACE and $RELEASE_NAME, and
// applies them to the cluster.
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

	vars := map[string]string{
		"NAMESPACE":    scenarioCtx.Namespace,
		"RELEASE_NAME": scenarioCtx.Release,
	}

	manifests, err := loadAndSubstituteManifests(resourcesDir, vars)
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

// ApplyLifecycleManifests applies a specific list of manifest files from the
// chart's common/resources/ directory with the given variable substitutions.
// Used by pre-install lifecycle hooks declared in ci-test-config.yaml.
func ApplyLifecycleManifests(ctx context.Context, scenarioCtx *ScenarioContext, chartPath, kubeContext string,
	filenames []string, vars map[string]string) error {
	if len(filenames) == 0 {
		return nil
	}

	resourcesDir := resolveResourcesDir(chartPath)
	if resourcesDir == "" {
		return fmt.Errorf("could not find lifecycle resources directory in chart %s", chartPath)
	}

	manifests, err := loadSelectedManifests(resourcesDir, filenames, vars)
	if err != nil {
		return fmt.Errorf("failed to load lifecycle manifests: %w", err)
	}

	kubeClient, err := kube.NewClient("", kubeContext)
	if err != nil {
		return fmt.Errorf("failed to create kube client for lifecycle manifests: %w", err)
	}

	for _, m := range manifests {
		logging.Logger.Info().
			Str("scenario", scenarioCtx.ScenarioName).
			Str("file", m.filename).
			Str("namespace", scenarioCtx.Namespace).
			Msg("Applying lifecycle manifest")

		if err := kubeClient.ApplyManifest(ctx, scenarioCtx.Namespace, m.data); err != nil {
			return fmt.Errorf("failed to apply lifecycle manifest %s: %w", m.filename, err)
		}
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
	candidate := filepath.Join(chartPath, "test", "integration", "scenarios", lifecycleResourcesDir)
	info, err := os.Stat(candidate)
	if err == nil && info.IsDir() {
		return candidate
	}
	return ""
}

// loadAndSubstituteManifests reads all YAML files from the resources directory
// and substitutes the supplied variables into each.
func loadAndSubstituteManifests(resourcesDir string, vars map[string]string) ([]manifest, error) {
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

		manifests = append(manifests, manifest{
			filename: name,
			data:     []byte(substituteManifestVars(string(raw), vars)),
		})
	}

	return manifests, nil
}

// loadSelectedManifests reads the named YAML files (in order) from the resources
// directory and substitutes the supplied variables into each. Returns an error
// if any named file is missing.
func loadSelectedManifests(resourcesDir string, filenames []string, vars map[string]string) ([]manifest, error) {
	var manifests []manifest
	for _, name := range filenames {
		filePath := filepath.Join(resourcesDir, name)
		raw, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read manifest file %s: %w", filePath, err)
		}
		manifests = append(manifests, manifest{
			filename: name,
			data:     []byte(substituteManifestVars(string(raw), vars)),
		})
	}
	return manifests, nil
}

// substituteManifestVars replaces $VAR and ${VAR} placeholders in manifest
// content with values from the supplied map. Keys are processed longest-first
// to prevent a shorter key from corrupting a longer one (e.g. $NAMESPACE vs
// $NAMESPACE_TAG). Both ${VAR} and $VAR forms are supported, matching the
// envsubst semantics used by the legacy shell scripts.
func substituteManifestVars(content string, vars map[string]string) string {
	keys := make([]string, 0, len(vars))
	for k := range vars {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return len(keys[i]) > len(keys[j])
	})
	result := content
	for _, k := range keys {
		v := vars[k]
		result = strings.ReplaceAll(result, "${"+k+"}", v)
		result = strings.ReplaceAll(result, "$"+k, v)
	}
	return result
}
