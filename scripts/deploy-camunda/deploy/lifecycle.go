package deploy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"scripts/camunda-core/pkg/kube"
	"scripts/camunda-core/pkg/logging"
)

// lifecycleResourcesDir is the subdirectory under the chart's integration
// scenarios directory that contains lifecycle (pre-install / post-deploy)
// manifest templates.
const lifecycleResourcesDir = "common/resources"

// ApplyLifecycleManifests applies a specific list of manifest files from the
// chart's common/resources/ directory with the given variable substitutions.
// Used by pre-install lifecycle hooks declared in ci-test-config.yaml.
func ApplyLifecycleManifests(ctx context.Context, scenarioCtx *ScenarioContext, chartPath, kubeContext string,
	filenames []string, vars map[string]string) error {
	if len(filenames) == 0 {
		return nil
	}

	resourcesDir := filepath.Join(chartPath, "test", "integration", "scenarios", lifecycleResourcesDir)
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

// manifestVarRe matches $VAR and ${VAR} placeholders. Capture group 1 is the
// braced form, group 2 the bare form. Identifiers follow shell rules:
// `[A-Za-z_][A-Za-z0-9_]*`. Word-boundary semantics fall out of the regex —
// `$NAMESPACE` cannot match inside `$NAMESPACE_TAG` because the engine
// consumes the longest valid identifier.
var manifestVarRe = regexp.MustCompile(`\$(?:\{([A-Za-z_][A-Za-z0-9_]*)\}|([A-Za-z_][A-Za-z0-9_]*))`)

// substituteManifestVars replaces $VAR and ${VAR} placeholders in manifest
// content with values from the supplied map. Placeholders not present in
// vars are left intact (envsubst-style passthrough; we do not support
// `${VAR:-default}` or `${VAR:?error}` extensions).
func substituteManifestVars(content string, vars map[string]string) string {
	return manifestVarRe.ReplaceAllStringFunc(content, func(match string) string {
		sub := manifestVarRe.FindStringSubmatch(match)
		key := sub[1]
		if key == "" {
			key = sub[2]
		}
		if v, ok := vars[key]; ok {
			return v
		}
		return match
	})
}
