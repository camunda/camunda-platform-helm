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
	// CredentialsManifest, when set, replaces the chart's integration-test-credentials ExternalSecret.
	CredentialsManifest string
}

func (p *GKESecretsProvider) Apply(ctx context.Context, client *Client, namespace string) error {
	return applyExternalSecretsForGKERosa(ctx, client, p.RepoRoot, p.ChartPath, namespace, p.ExternalSecretsStore, p.CredentialsManifest)
}

type ROSASecretsProvider struct {
	RepoRoot             string
	ChartPath            string
	ExternalSecretsStore string
	// CredentialsManifest, when set, replaces the chart's integration-test-credentials ExternalSecret.
	CredentialsManifest string
}

func (p *ROSASecretsProvider) Apply(ctx context.Context, client *Client, namespace string) error {
	return applyExternalSecretsForGKERosa(ctx, client, p.RepoRoot, p.ChartPath, namespace, p.ExternalSecretsStore, p.CredentialsManifest)
}

type EKSSecretsProvider struct {
	RepoRoot             string
	ChartPath            string
	ExternalSecretsStore string
	// CredentialsManifest, when set, replaces the chart's integration-test-credentials ExternalSecret.
	CredentialsManifest string
}

func (p *EKSSecretsProvider) Apply(ctx context.Context, client *Client, namespace string) error {
	return applySecretsForEKS(ctx, client, p.RepoRoot, p.ChartPath, namespace, p.ExternalSecretsStore, p.CredentialsManifest)
}

func NewPlatformSecretsProvider(platform, repoRoot, chartPath, externalSecretsStore, credentialsManifest string) (PlatformSecretsProvider, error) {
	switch platform {
	case platformGKE:
		return &GKESecretsProvider{
			RepoRoot:             repoRoot,
			ChartPath:            chartPath,
			ExternalSecretsStore: externalSecretsStore,
			CredentialsManifest:  credentialsManifest,
		}, nil
	case platformROSA:
		return &ROSASecretsProvider{
			RepoRoot:             repoRoot,
			ChartPath:            chartPath,
			ExternalSecretsStore: externalSecretsStore,
			CredentialsManifest:  credentialsManifest,
		}, nil
	case platformEKS:
		return &EKSSecretsProvider{
			RepoRoot:             repoRoot,
			ChartPath:            chartPath,
			ExternalSecretsStore: externalSecretsStore,
			CredentialsManifest:  credentialsManifest,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported platform %q (supported: gke, rosa, eks)", platform)
	}
}

func applyExternalSecretsForGKERosa(ctx context.Context, client *Client, repoRoot, chartPath, namespace, externalSecretsStore, credentialsManifest string) error {
	if err := applyExternalSecretsCertificates(ctx, client, repoRoot, namespace); err != nil {
		return err
	}

	if err := applyExternalSecretsOther(ctx, client, repoRoot, chartPath, namespace, externalSecretsStore, credentialsManifest); err != nil {
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

func applyExternalSecretsOther(ctx context.Context, client *Client, repoRoot, chartPath, namespace, externalSecretsStore, credentialsManifest string) error {
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

	file, err := integrationCredentialsManifest(chartPath, externalSecretDir, vaultSuffix, credentialsManifest, fileExists)
	if err != nil {
		return err
	}
	if file == "" {
		logging.Logger.Debug().Msg("no integration-test external-secret manifest found (optional, continuing)")
		return nil
	}
	if err := applyManifestFile(ctx, client, namespace, file); err != nil {
		return fmt.Errorf("apply integration-test credentials %s: %w", file, err)
	}
	logging.Logger.Debug().Str("file", file).Msg("applied integration-test credentials external-secret")
	return nil
}

// integrationCredentialsManifest picks the one manifest that provides a
// namespace's integration-test-credentials: the scenario's own when given (and
// then never the CI one), otherwise the chart-specific file, otherwise the
// shared fallback. An empty result means there is none to apply.
func integrationCredentialsManifest(chartPath, externalSecretDir, vaultSuffix, credentialsManifest string, exists func(string) bool) (string, error) {
	if credentialsManifest != "" {
		if !exists(credentialsManifest) {
			return "", fmt.Errorf("credentials manifest %q does not exist", credentialsManifest)
		}
		return credentialsManifest, nil
	}
	name := fmt.Sprintf("external-secret-integration-test-credentials%s.yaml", vaultSuffix)
	for _, candidate := range []string{
		filepath.Join(chartPath, "test", "integration", "external-secrets", name),
		filepath.Join(externalSecretDir, name),
	} {
		if exists(candidate) {
			return candidate, nil
		}
	}
	return "", nil
}

func applySecretsForEKS(ctx context.Context, client *Client, repoRoot, chartPath, namespace, externalSecretsStore, credentialsManifest string) error {
	stub := filepath.Join(repoRoot, ".github", "config", "replicate-from", "replicate-from-eks-tls.yaml")

	if err := deleteExternalSecretsTargeting(ctx, client, namespace, secretNameTLS); err != nil {
		return err
	}

	if err := deleteSecretIfExists(ctx, client, namespace, secretNameTLS); err != nil {
		return err
	}

	if err := applyManifestFile(ctx, client, namespace, stub); err != nil {
		return fmt.Errorf("apply EKS TLS replicate-from stub: %w", err)
	}

	if err := waitForReplicatedSecret(ctx, client, namespace, secretNameTLS, "tls.crt", "tls.key"); err != nil {
		return fmt.Errorf("%w (source is the replicate-from annotation in %s)", err, stub)
	}

	if err := applyExternalSecretsOther(ctx, client, repoRoot, chartPath, namespace, externalSecretsStore, credentialsManifest); err != nil {
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
	var (
		missing  []string
		notFound bool
	)

	err := wait.PollUntilContextTimeout(ctx, replicatedSecretInterval, replicatedSecretTimeout, true,
		func(ctx context.Context) (bool, error) {
			secret, err := client.clientset.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
			if err != nil {
				if apierrors.IsNotFound(err) {
					notFound = true
					return false, nil
				}
				return false, err
			}

			notFound = false
			missing = emptyKeys(secret.Data, keys)
			return len(missing) == 0, nil
		})
	if err != nil {
		if notFound {
			return fmt.Errorf("secret %q was never created in namespace %q: %w", secretName, namespace, err)
		}
		return fmt.Errorf("secret %q in namespace %q was not populated by the replicator (still empty: %v): %w",
			secretName, namespace, missing, err)
	}

	logging.Logger.Debug().
		Str("namespace", namespace).
		Str("secret", secretName).
		Msg("secret populated by the replicator")
	return nil
}

func deleteSecretIfExists(ctx context.Context, client *Client, namespace, name string) error {
	err := client.clientset.CoreV1().Secrets(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("delete secret %q in %q: %w", name, namespace, err)
	}
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
