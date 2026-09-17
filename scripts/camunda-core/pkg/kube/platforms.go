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

package kube

import (
	"context"
	"fmt"
	"path/filepath"
	"scripts/camunda-core/pkg/logging"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/wait"
)

type PlatformSecretsProvider interface {
	Apply(ctx context.Context, client *Client, namespace string) error
}

type GKESecretsProvider struct {
	RepoRoot             string
	ChartPath            string
	ExternalSecretsStore string
}

func (p *GKESecretsProvider) Apply(ctx context.Context, client *Client, namespace string) error {
	return applyExternalSecretsForGKERosa(ctx, client, p.RepoRoot, p.ChartPath, namespace, p.ExternalSecretsStore)
}

type ROSASecretsProvider struct {
	RepoRoot             string
	ChartPath            string
	ExternalSecretsStore string
}

func (p *ROSASecretsProvider) Apply(ctx context.Context, client *Client, namespace string) error {
	return applyExternalSecretsForGKERosa(ctx, client, p.RepoRoot, p.ChartPath, namespace, p.ExternalSecretsStore)
}

type EKSSecretsProvider struct {
	RepoRoot             string
	ChartPath            string
	ExternalSecretsStore string
}

func (p *EKSSecretsProvider) Apply(ctx context.Context, client *Client, namespace string) error {
	return applySecretsForEKS(ctx, client, p.RepoRoot, p.ChartPath, namespace, p.ExternalSecretsStore)
}

func NewPlatformSecretsProvider(platform, repoRoot, chartPath, externalSecretsStore string) (PlatformSecretsProvider, error) {
	switch platform {
	case platformGKE:
		return &GKESecretsProvider{
			RepoRoot:             repoRoot,
			ChartPath:            chartPath,
			ExternalSecretsStore: externalSecretsStore,
		}, nil
	case platformROSA:
		return &ROSASecretsProvider{
			RepoRoot:             repoRoot,
			ChartPath:            chartPath,
			ExternalSecretsStore: externalSecretsStore,
		}, nil
	case platformEKS:
		return &EKSSecretsProvider{
			RepoRoot:             repoRoot,
			ChartPath:            chartPath,
			ExternalSecretsStore: externalSecretsStore,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported platform %q (supported: gke, rosa, eks)", platform)
	}
}

func applyExternalSecretsForGKERosa(ctx context.Context, client *Client, repoRoot, chartPath, namespace, externalSecretsStore string) error {
	if err := applyExternalSecretsCertificates(ctx, client, repoRoot, namespace); err != nil {
		return err
	}

	if err := applyExternalSecretsOther(ctx, client, repoRoot, chartPath, namespace, externalSecretsStore); err != nil {
		return err
	}

	logging.Logger.Debug().Str("namespace", namespace).Msg("waiting for ExternalSecrets to become Ready")
	if err := waitExternalSecretsReady(ctx, client, namespace, externalSecretsReadyTimeout); err != nil {
		return fmt.Errorf("wait for ExternalSecrets ready: %w", err)
	}

	return nil
}

func applyExternalSecretsCertificates(ctx context.Context, client *Client, repoRoot, namespace string) error {
	externalSecretDir := filepath.Join(repoRoot, ".github", "config", "external-secret")

	if err := applyManifestIfExists(ctx, client, namespace,
		filepath.Join(externalSecretDir, "external-secret-certificates.yaml"),
		"certificates external-secret"); err != nil {
		return fmt.Errorf("apply certificates: %w", err)
	}

	return nil
}

func applyExternalSecretsOther(ctx context.Context, client *Client, repoRoot, chartPath, namespace, externalSecretsStore string) error {
	externalSecretDir := filepath.Join(repoRoot, ".github", "config", "external-secret")

	// Determine suffix for vault-backed secrets
	vaultSuffix := ""
	if externalSecretsStore == "vault-backend" {
		vaultSuffix = "-vault"
		logging.Logger.Debug().Msg("using vault-backed external secrets")
	}

	// Apply credentials secrets
	credentialsSecretFile := fmt.Sprintf("external-secret-credentials%s.yaml", vaultSuffix)
	if err := applyManifestIfExists(ctx, client, namespace,
		filepath.Join(externalSecretDir, credentialsSecretFile),
		"credentials external-secret"); err != nil {
		return fmt.Errorf("apply credentials secrets: %w", err)
	}

	// Determine which integration test credentials file to use based on external secrets store
	integrationCredsFile := fmt.Sprintf("external-secret-integration-test-credentials%s.yaml", vaultSuffix)

	chartSpecific := filepath.Join(chartPath, "test", "integration", "external-secrets", integrationCredsFile)
	fallback := filepath.Join(externalSecretDir, integrationCredsFile)

	if fileExists(chartSpecific) {
		if err := applyManifestFile(ctx, client, namespace, chartSpecific); err != nil {
			return fmt.Errorf("apply chart-specific integration-test credentials: %w", err)
		}
		logging.Logger.Debug().Str("file", chartSpecific).Msg("applied chart-specific integration-test external-secret")
	} else if fileExists(fallback) {
		if err := applyManifestFile(ctx, client, namespace, fallback); err != nil {
			return fmt.Errorf("apply fallback integration-test credentials: %w", err)
		}
		logging.Logger.Debug().Str("file", fallback).Msg("applied fallback integration-test external-secret")
	} else {
		logging.Logger.Debug().Msg("no integration-test external-secret manifest found (optional, continuing)")
	}

	return nil
}

func applySecretsForEKS(ctx context.Context, client *Client, repoRoot, chartPath, namespace, externalSecretsStore string) error {
	stub := filepath.Join(repoRoot, ".github", "config", "replicate-from", "replicate-from-eks-tls.yaml")

	if err := deleteExternalSecretsTargeting(ctx, client, namespace, secretNameTLS); err != nil {
		return err
	}

	if err := applyManifestIfExists(ctx, client, namespace, stub, "EKS TLS replicate-from stub"); err != nil {
		return fmt.Errorf("apply EKS TLS replicate-from stub: %w", err)
	}

	if err := waitForReplicatedSecret(ctx, client, namespace, secretNameTLS, "tls.crt", "tls.key"); err != nil {
		return fmt.Errorf("%w (source is the replicate-from annotation in %s)", err, stub)
	}

	if err := applyExternalSecretsOther(ctx, client, repoRoot, chartPath, namespace, externalSecretsStore); err != nil {
		return err
	}

	logging.Logger.Debug().Str("namespace", namespace).Msg("waiting for ExternalSecrets to become Ready")
	if err := waitExternalSecretsReady(ctx, client, namespace, externalSecretsReadyTimeout); err != nil {
		return fmt.Errorf("wait for ExternalSecrets ready: %w", err)
	}
	return nil
}

const (
	externalSecretsReadyTimeout = 600 * time.Second

	replicatedSecretInterval = 5 * time.Second
	replicatedSecretTimeout  = 300 * time.Second
)

// The stub is applied with empty tls.crt/tls.key, so existence is not enough: the secret is
// only usable once the replicator has copied values in.
func waitForReplicatedSecret(ctx context.Context, client *Client, namespace, secretName string, keys ...string) error {
	var missing []string

	err := wait.PollUntilContextTimeout(ctx, replicatedSecretInterval, replicatedSecretTimeout, true,
		func(ctx context.Context) (bool, error) {
			secret, err := client.clientset.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
			if err != nil {
				if apierrors.IsNotFound(err) {
					missing = keys
					return false, nil
				}
				return false, err
			}

			missing = emptyKeys(secret.Data, keys)
			return len(missing) == 0, nil
		})
	if err != nil {
		return fmt.Errorf("secret %q in namespace %q was not populated by the replicator (still empty: %v): %w",
			secretName, namespace, missing, err)
	}

	logging.Logger.Debug().
		Str("namespace", namespace).
		Str("secret", secretName).
		Msg("secret populated by the replicator")
	return nil
}

// A namespace carried over from before the move to replication still holds the ExternalSecret
// that reconciled the Vault snapshot into the same Secret. Left in place it would overwrite the
// replicated certificate, or race the replicator for it.
func deleteExternalSecretsTargeting(ctx context.Context, client *Client, namespace, targetSecret string) error {
	list, err := client.dynamicClient.Resource(externalSecretGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("list ExternalSecrets in %q: %w", namespace, err)
	}

	for _, item := range list.Items {
		name, found, err := unstructured.NestedString(item.Object, "spec", "target", "name")
		if err != nil || !found || name != targetSecret {
			continue
		}

		logging.Logger.Debug().
			Str("namespace", namespace).
			Str("externalSecret", item.GetName()).
			Str("target", targetSecret).
			Msg("removing stale ExternalSecret so it cannot overwrite the replicated secret")

		err = client.dynamicClient.Resource(externalSecretGVR).Namespace(namespace).
			Delete(ctx, item.GetName(), metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("delete stale ExternalSecret %q in %q: %w", item.GetName(), namespace, err)
		}
	}

	return nil
}

func emptyKeys(data map[string][]byte, keys []string) []string {
	var missing []string
	for _, key := range keys {
		if len(data[key]) == 0 {
			missing = append(missing, key)
		}
	}
	return missing
}
