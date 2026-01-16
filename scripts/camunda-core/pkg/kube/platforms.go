package kube

import (
	"context"
	"fmt"
	"path/filepath"
	"scripts/camunda-core/pkg/logging"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1apply "k8s.io/client-go/applyconfigurations/core/v1"
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
	NamespacePrefix      string
	RepoRoot             string
	ChartPath            string
	ExternalSecretsStore string
}

func (p *EKSSecretsProvider) Apply(ctx context.Context, client *Client, namespace string) error {
	return applySecretsForEKS(ctx, client, p.RepoRoot, p.ChartPath, namespace, p.NamespacePrefix, p.ExternalSecretsStore)
}

func NewPlatformSecretsProvider(platform, repoRoot, chartPath, namespacePrefix, externalSecretsStore string) (PlatformSecretsProvider, error) {
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
			NamespacePrefix:      namespacePrefix,
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

	// Apply infra secrets
	infraSecretFile := fmt.Sprintf("external-secret-infra%s.yaml", vaultSuffix)
	if err := applyManifestIfExists(ctx, client, namespace,
		filepath.Join(externalSecretDir, infraSecretFile),
		"infra-secrets external-secret"); err != nil {
		return fmt.Errorf("apply infra secrets: %w", err)
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

func applySecretsForEKS(ctx context.Context, client *Client, repoRoot, chartPath, namespace, namespacePrefix, externalSecretsStore string) error {
	srcNamespace := computeEKSSourceNamespace(namespacePrefix)

	logging.Logger.Debug().
		Str("srcNamespace", srcNamespace).
		Str("destNamespace", namespace).
		Str("secret", secretNameTLS).
		Msg("copying TLS secret for EKS")

	if err := copySecretBetweenNamespaces(ctx, client, srcNamespace, secretNameTLS, namespace); err != nil {
		return fmt.Errorf("copy TLS secret from %s to %s: %w", srcNamespace, namespace, err)
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

const externalSecretsReadyTimeout = 300 * time.Second

func copySecretBetweenNamespaces(ctx context.Context, client *Client, srcNamespace, secretName, destNamespace string) error {
	logging.Logger.Debug().
		Str("srcNamespace", srcNamespace).
		Str("destNamespace", destNamespace).
		Str("secret", secretName).
		Msg("copying secret between namespaces")

	secret, err := client.clientset.CoreV1().Secrets(srcNamespace).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get secret %s in namespace %s: %w", secretName, srcNamespace, err)
	}

	logging.Logger.Debug().
		Str("secret", secretName).
		Str("destNamespace", destNamespace).
		Msg("applying copied secret to destination namespace")

	secretApply := corev1apply.Secret(secretName, destNamespace).
		WithLabels(secret.Labels).
		WithAnnotations(secret.Annotations).
		WithType(secret.Type).
		WithData(secret.Data)

	_, err = client.clientset.CoreV1().Secrets(destNamespace).Apply(
		ctx,
		secretApply,
		defaultApplyOptions(),
	)
	if err != nil {
		// Check if error is due to namespace termination
		if strings.Contains(err.Error(), "is being terminated") || strings.Contains(err.Error(), "because it is being terminated") {
			return fmt.Errorf("failed to apply copied secret %q to namespace %q: namespace is currently being deleted, please wait for deletion to complete or use a different namespace: %w", secretName, destNamespace, err)
		}
		return fmt.Errorf("failed to apply copied secret %q to namespace %q: %w", secretName, destNamespace, err)
	}

	return nil
}
