package deployer

import (
	"context"
	"fmt"
	"scripts/camunda-core/pkg/docker"
	"scripts/camunda-core/pkg/helm"
	"scripts/camunda-core/pkg/kube"
	"scripts/camunda-deployer/pkg/types"
	"strings"
)

func Deploy(ctx context.Context, o types.Options) error {
	if !o.SkipDockerLogin && o.EnsureDockerRegistry {
		if err := docker.EnsureDockerLogin(ctx, o.DockerRegistryUsername, o.DockerRegistryPassword); err != nil {
			return fmt.Errorf("failed to ensure docker login: %w", err)
		}
	}

	if !o.SkipDependencyUpdate {
		if err := helm.DependencyUpdate(ctx, o.ChartPath); err != nil {
			return err
		}
	}

	kubeClient, err := kube.NewClient(o.Kubeconfig, o.KubeContext)
	if err != nil {
		return fmt.Errorf("failed to create kube client: %w", err)
	}

	if err := kubeClient.EnsureNamespace(ctx, o.Namespace); err != nil {
		return err
	}

	if err := labelAndAnnotateNamespace(ctx, kubeClient, o.Namespace, o.Identifier, o.CIMetadata.Flow, o.TTL, o.CIMetadata.GithubRunID, o.CIMetadata.GithubJobID, o.CIMetadata.GithubOrg, o.CIMetadata.GithubRepo, o.CIMetadata.WorkflowURL); err != nil {
		return err
	}

	if o.LoadKeycloakRealm && strings.TrimSpace(o.KeycloakRealmName) != "" {
		if err := loadKeycloakRealmConfigMap(ctx, kubeClient, o.ChartPath, o.KeycloakRealmName, o.Namespace); err != nil {
			return fmt.Errorf("failed to load Keycloak realm: %w", err)
		}
	}

	if o.EnsureDockerRegistry {
		if err := kubeClient.EnsureDockerRegistrySecret(ctx, o.Namespace, o.DockerRegistryUsername, o.DockerRegistryPassword); err != nil {
			return err
		}
	}
	
	if o.ExternalSecretsEnabled {
		if err := kube.ApplyExternalSecretsAndCerts(ctx, o.Kubeconfig, o.KubeContext, o.Platform, o.RepoRoot, o.ChartPath, o.Namespace, o.NamespacePrefix); err != nil {
			return err
		}
	}

	if o.ApplyIntegrationCreds {
		if err := applyIntegrationTestCredentials(ctx, kubeClient, o.Namespace); err != nil {
			return err
		}
	}

	return upgradeInstall(ctx, o)
}
