package deployer

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"scripts/camunda-core/pkg/kube"
	"scripts/camunda-core/pkg/logging"
	"strings"

	"gopkg.in/yaml.v3"
)

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

func applyInfraTestCredentials(ctx context.Context, kubeClient *kube.Client, namespace string) error {
	b64 := strings.TrimSpace(os.Getenv("INTEGRATION_TEST"))
	if b64 == "" {
		logging.Logger.Debug().Str("namespace", namespace).Msg("skipping infra-credentials (env not present)")
		return nil
	}
	
	decoded, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return fmt.Errorf("failed to decode INTEGRATION_TEST: %w", err)
	}
	decoded, err = replaceNameFieldInYAML(decoded, "infra-credentials")
	if err != nil {
		return fmt.Errorf("failed to update name field in decoded manifest: %w", err)
	}


	// Use generic method to apply manifest
	return kubeClient.ApplyManifest(ctx, namespace, decoded)
}

func replaceNameFieldInYAML(data []byte, name string) ([]byte, error) {
	var obj map[string]interface{}
	err := yaml.Unmarshal(data, &obj)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML: %w", err)
	}

	metadata, ok := obj["metadata"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("failed to find metadata field in YAML")
	}
	metadata["name"] = name

	return yaml.Marshal(obj)
}