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
	"time"

	"gopkg.in/yaml.v3"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// labelAndAnnotateNamespace adds Camunda/GitHub-specific labels and annotations
func labelAndAnnotateNamespace(ctx context.Context, kubeClient *kube.Client, namespace string, existingAnnotations map[string]string, identifier, flow, ttl string, ghRunID string, ghJobID string, ghOrg string, ghRepo string, workflowURL string) error {
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

	annotations := lifecycleAnnotations(existingAnnotations, ttl)
	if strings.TrimSpace(workflowURL) != "" {
		annotations["github-workflow-run-url"] = workflowURL
	}

	// Use generic method to apply
	return kubeClient.SetLabelsAndAnnotations(ctx, namespace, labels, annotations)
}

// readNamespaceAnnotations returns a namespace's annotations. A namespace that
// does not exist yet reads as (nil, nil); any other failure is returned, so the
// caller can leave lifecycle metadata alone rather than guess the namespace is new.
func readNamespaceAnnotations(ctx context.Context, kubeClient *kube.Client, namespace string) (map[string]string, error) {
	exists, err := kubeClient.NamespaceExists(ctx, namespace)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil
	}
	return kubeClient.NamespaceAnnotations(ctx, namespace)
}

// decideLifecycle settles which annotations lifecycleAnnotations should see and
// whether to stamp at all. A namespace this deploy created is new whatever a
// read says. Otherwise the decision uses reread, a read taken right before
// stamping, so a namespace persisted by someone else since the deploy started
// is seen as persisted; the initial read (taken before EnsureNamespace) is only
// the fallback when that read fails, and only if it showed the namespace persisted. A Forbidden result means the lifecycle
// state is unknowable: stamp nothing rather than risk a short TTL on a
// persisted namespace. Any other error is returned.
func decideLifecycle(created bool, initial map[string]string, initialErr error, reread func() (map[string]string, error)) (map[string]string, bool, error) {
	if created {
		return nil, true, nil
	}
	annotations, err := reread()
	if err != nil && initialErr == nil && initial["camunda.cloud/ephemeral"] == "false" {
		// Only a persisted initial read is safe to reuse: it cannot have become
		// "more persisted" since, whereas an ephemeral one may have been persisted.
		annotations, err = initial, nil
	}
	switch {
	case err == nil:
		return annotations, true, nil
	case apierrors.IsForbidden(err):
		return nil, false, nil
	default:
		return nil, false, err
	}
}

// retryNamespaceAnnotations retries readNamespaceAnnotations, returning the
// last error once attempts run out.
func retryNamespaceAnnotations(ctx context.Context, kubeClient *kube.Client, namespace string, attempts int, wait time.Duration) (map[string]string, error) {
	var err error
	for i := 0; i < attempts; i++ {
		var annotations map[string]string
		if annotations, err = readNamespaceAnnotations(ctx, kubeClient, namespace); err == nil {
			return annotations, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(wait):
		}
	}
	return nil, err
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
