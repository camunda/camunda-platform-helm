package deployer

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"scripts/camunda-core/pkg/kube"
	"scripts/camunda-core/pkg/logging"
	"strings"
)

// loadKeycloakRealmConfigMap creates a Keycloak realm ConfigMap from a realm.json file.
// This is Camunda-specific test infrastructure.
func loadKeycloakRealmConfigMap(ctx context.Context, kubeClient *kube.Client, chartPath, realmName, namespace string) error {
	logging.Logger.Debug().
		Str("chartPath", chartPath).
		Str("realmName", realmName).
		Str("namespace", namespace).
		Msg("loading Keycloak realm ConfigMap")

	// Locate the realm.json file
	realmJSONPath := filepath.Join(chartPath, "test", "integration", "realm.json")
	if _, err := os.Stat(realmJSONPath); err != nil {
		logging.Logger.Debug().Str("path", realmJSONPath).Msg("realm.json not found")
		return fmt.Errorf("realm.json not found at %s: %w", realmJSONPath, err)
	}

	logging.Logger.Debug().Str("path", realmJSONPath).Msg("found realm.json")

	// Read and parse the realm.json
	data, err := os.ReadFile(realmJSONPath)
	if err != nil {
		return fmt.Errorf("failed to read realm.json: %w", err)
	}

	var realmConfig map[string]any
	if err := json.Unmarshal(data, &realmConfig); err != nil {
		return fmt.Errorf("failed to parse realm.json: %w", err)
	}

	logging.Logger.Debug().Msg("updating realm configuration")

	// Update realm configuration with the specified realm name
	realmConfig["id"] = realmName
	realmConfig["realm"] = realmName

	// Update realm roles if they exist
	if roles, ok := realmConfig["roles"].(map[string]any); ok {
		if realmRoles, ok := roles["realm"].([]any); ok {
			for _, role := range realmRoles {
				if roleMap, ok := role.(map[string]any); ok {
					roleMap["containerId"] = realmName
				}
			}
		}
	}

	// Update defaultRole if it exists
	if defaultRole, ok := realmConfig["defaultRole"].(map[string]any); ok {
		defaultRole["containerId"] = realmName
	}

	// Marshal the modified config
	modifiedData, err := json.MarshalIndent(realmConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal modified realm config: %w", err)
	}

	// Apply using generic ConfigMap method
	if err := kubeClient.ApplyConfigMap(ctx, namespace, "realm-json", map[string]string{
		"realm.json": string(modifiedData),
	}); err != nil {
		return fmt.Errorf("failed to apply Keycloak realm ConfigMap: %w", err)
	}

	logging.Logger.Debug().
		Str("configMap", "realm-json").
		Str("namespace", namespace).
		Msg("Keycloak realm ConfigMap applied successfully")

	return nil
}

// labelAndAnnotateNamespace adds Camunda/GitHub-specific labels and annotations
func labelAndAnnotateNamespace(ctx context.Context, kubeClient *kube.Client, namespace, identifier, flow, ttl string, ghRunID string, ghJobID string, ghOrg string, ghRepo string, workflowURL string) error {
	// Build labels map
	labels := make(map[string]string)
	if strings.TrimSpace(identifier) != "" {
		labels["github-id"] = identifier
	}
	if strings.TrimSpace(flow) != "" {
		labels["test-flow"] = flow
	}
	if strings.TrimSpace(ghRunID) != "" {
		labels["github-run-id"] = ghRunID
	}
	if strings.TrimSpace(ghJobID) != "" {
		labels["github-job-id"] = ghJobID
	}
	if strings.TrimSpace(ghOrg) != "" {
		labels["github-org"] = ghOrg
	}
	if strings.TrimSpace(ghRepo) != "" {
		labels["github-repo"] = ghRepo
	}

	// Build annotations map
	if strings.TrimSpace(ttl) == "" {
		ttl = "1h"
	}
	annotations := map[string]string{
		"cleaner/ttl":             ttl,
		"janitor/ttl":             ttl,
		"camunda.cloud/ephemeral": "true",
	}
	if strings.TrimSpace(workflowURL) != "" {
		annotations["github-workflow-run-url"] = workflowURL
	}

	// Use generic method to apply
	return kubeClient.SetLabelsAndAnnotations(ctx, namespace, labels, annotations)
}

// applyIntegrationTestCredentials applies integration test credentials from environment variable
func applyIntegrationTestCredentials(ctx context.Context, kubeClient *kube.Client, namespace string) error {
	b64 := strings.TrimSpace(os.Getenv("INTEGRATION_TEST_CREDENTIALS"))
	if b64 == "" {
		logging.Logger.Debug().Str("namespace", namespace).Msg("skipping integration-test credentials (env not present)")
		return nil
	}

	logging.Logger.Debug().Str("namespace", namespace).Msg("applying integration-test credentials from env")

	decoded, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return fmt.Errorf("failed to decode INTEGRATION_TEST_CREDENTIALS: %w", err)
	}

	// Use generic method to apply manifest
	return kubeClient.ApplyManifest(ctx, namespace, decoded)
}

