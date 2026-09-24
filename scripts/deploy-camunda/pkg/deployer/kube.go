// Copyright 2026 Camunda Services GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

	existing, err := kubeClient.NamespaceAnnotations(ctx, namespace)
	if err != nil {
		return err
	}
	annotations := lifecycleAnnotations(existing, ttl)
	if strings.TrimSpace(workflowURL) != "" {
		annotations["github-workflow-run-url"] = workflowURL
	}

	// Use generic method to apply
	return kubeClient.SetLabelsAndAnnotations(ctx, namespace, labels, annotations)
}

// lifecycleAnnotations returns the cleaner/janitor annotations to stamp on a
// namespace a deploy is about to use. A namespace `topology persist` marked
// long-lived (camunda.cloud/ephemeral=false) keeps its current values: the
// cleaner measures a TTL from the namespace's creation, so stamping a short
// TTL on an old namespace makes it expire at once. The values are re-applied
// rather than omitted because an omitted server-side-apply field this manager
// owns is removed, which would drop the persisted marker itself.
func lifecycleAnnotations(existing map[string]string, ttl string) map[string]string {
	if existing["camunda.cloud/ephemeral"] == "false" {
		kept := map[string]string{"camunda.cloud/ephemeral": "false"}
		for _, k := range []string{"cleaner/ttl", "janitor/ttl"} {
			if v, ok := existing[k]; ok {
				kept[k] = v
			}
		}
		return kept
	}
	if strings.TrimSpace(ttl) == "" {
		ttl = "1h"
	}
	return map[string]string{
		"cleaner/ttl":             ttl,
		"janitor/ttl":             ttl,
		"camunda.cloud/ephemeral": "true",
	}
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
