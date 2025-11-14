package deployer

import (
	"context"
	"fmt"
	"scripts/camunda-core/pkg/helm"
	"scripts/camunda-core/pkg/kube"
	"scripts/camunda-deployer/pkg/types"
	"strings"
)

// Deploy performs the full deployment flow: deps update, ns ensure, labels/annotations,
// optional secrets, and helm apply.
// This is the main entry point for deploying a Camunda chart.
func Deploy(ctx context.Context, o types.Options) error {
	// Ensure docker login before helm dependency update (needed for private registry dependencies)
	if !o.SkipDockerLogin && o.EnsureDockerRegistry {
		if err := kube.EnsureDockerLogin(ctx, o.DockerRegistryUsername, o.DockerRegistryPassword); err != nil {
			return fmt.Errorf("failed to ensure docker login: %w", err)
		}
	}

	// Helm deps update
	if !o.SkipDependencyUpdate {
		if err := helm.DependencyUpdate(ctx, o.ChartPath); err != nil {
			return err
		}
	}

	// Namespace lifecycle and metadata
	if err := kube.EnsureNamespace(ctx, o.Kubeconfig, o.KubeContext, o.Namespace); err != nil {
		return err
	}

	if err := kube.LabelAndAnnotateNamespace(ctx, o.Kubeconfig, o.KubeContext, o.Namespace, o.Identifier, o.CIMetadata.Flow, o.TTL, o.CIMetadata.GithubRunID, o.CIMetadata.GithubJobID, o.CIMetadata.GithubOrg, o.CIMetadata.GithubRepo, o.CIMetadata.WorkflowURL); err != nil {
		return err
	}

	// Keycloak realm ConfigMap
	if o.LoadKeycloakRealm && strings.TrimSpace(o.KeycloakRealmName) != "" {
		if err := kube.LoadKeycloakRealmConfigMap(ctx, o.Kubeconfig, o.KubeContext, o.ChartPath, o.KeycloakRealmName, o.Namespace); err != nil {
			return fmt.Errorf("failed to load Keycloak realm: %w", err)
		}
	}

	// External secrets/certs per platform
	if o.ExternalSecretsEnabled && strings.TrimSpace(o.Platform) != "" {
		if err := kube.ApplyExternalSecretsAndCerts(ctx, o.Kubeconfig, o.KubeContext, o.Platform, o.RepoRoot, o.ChartPath, o.Namespace, o.NamespacePrefix); err != nil {
			return err
		}
	}

	// Required/optional cluster secrets
	if o.EnsureDockerRegistry {
		if err := kube.EnsureDockerRegistrySecretWithCreds(ctx, o.Kubeconfig, o.KubeContext, o.Namespace, o.DockerRegistryUsername, o.DockerRegistryPassword); err != nil {
			return err
		}
	}
	if o.ApplyIntegrationCreds {
		if err := kube.ApplyIntegrationTestCredentialsIfPresent(ctx, o.Kubeconfig, o.KubeContext, o.Namespace); err != nil {
			return err
		}
	}

	// Helm execution - use deployer's opinionated implementation
	return upgradeInstall(ctx, o)
}
