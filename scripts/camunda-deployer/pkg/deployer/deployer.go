package deployer

import (
	"context"
	"fmt"
	"os"
	"scripts/camunda-core/pkg/docker"
	"scripts/camunda-core/pkg/helm"
	"scripts/camunda-core/pkg/kube"
	"scripts/camunda-core/pkg/logging"
	"scripts/camunda-core/pkg/utils"
	"scripts/camunda-deployer/pkg/types"
)

func Deploy(ctx context.Context, o types.Options) error {
	// Render-only mode: do not touch the cluster or docker; just render templates
	if o.RenderTemplates {
		return renderTemplates(ctx, o)
	}

	if !o.SkipDockerLogin {
		if o.EnsureDockerHub {
			if err := docker.EnsureDockerHubLogin(ctx, o.DockerHubUsername, o.DockerHubPassword); err != nil {
				return fmt.Errorf("failed to ensure Docker Hub login: %w", err)
			}
		}
		if o.EnsureDockerRegistry {
			if err := docker.EnsureHarborLogin(ctx, o.DockerRegistryUsername, o.DockerRegistryPassword); err != nil {
				return fmt.Errorf("failed to ensure Harbor login: %w", err)
			}
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
		// Non-fatal: namespace labels are CI housekeeping metadata (TTL, GitHub run IDs).
		// On some clusters (e.g., EKS via Teleport) the user may lack namespace PATCH RBAC.
		logging.Logger.Warn().Err(err).Str("namespace", o.Namespace).
			Msg("failed to label/annotate namespace (continuing without CI metadata)")
	}

	if o.EnsureDockerRegistry {
		// Resolve Harbor registry credentials from flags or environment fallbacks
		username := o.DockerRegistryUsername
		password := o.DockerRegistryPassword
		if username == "" {
			username = utils.FirstNonEmpty(os.Getenv("HARBOR_USERNAME"), os.Getenv("TEST_DOCKER_USERNAME_CAMUNDA_CLOUD"), os.Getenv("NEXUS_USERNAME"))
		}
		if password == "" {
			password = utils.FirstNonEmpty(os.Getenv("HARBOR_PASSWORD"), os.Getenv("TEST_DOCKER_PASSWORD_CAMUNDA_CLOUD"), os.Getenv("NEXUS_PASSWORD"))
		}
		if err := kubeClient.EnsureDockerRegistrySecret(ctx, o.Namespace, username, password); err != nil {
			return err
		}
	}

	if o.EnsureDockerHub {
		// Resolve Docker Hub credentials from flags or environment fallbacks
		username := o.DockerHubUsername
		password := o.DockerHubPassword
		if username == "" {
			username = utils.FirstNonEmpty(os.Getenv("DOCKERHUB_USERNAME"), os.Getenv("TEST_DOCKER_USERNAME"))
		}
		if password == "" {
			password = utils.FirstNonEmpty(os.Getenv("DOCKERHUB_PASSWORD"), os.Getenv("TEST_DOCKER_PASSWORD"))
		}
		if err := kubeClient.EnsureDockerHubSecret(ctx, o.Namespace, username, password); err != nil {
			return err
		}
	}

	if o.ExternalSecretsEnabled {
		if err := kube.ApplyExternalSecretsAndCerts(ctx, o.Kubeconfig, o.KubeContext, o.Platform, o.RepoRoot, o.ChartPath, o.Namespace, o.NamespacePrefix, o.ExternalSecretsStore); err != nil {
			return err
		}
	}

	if o.ApplyIntegrationCreds {
		if err := applyIntegrationTestCredentials(ctx, kubeClient, o.Namespace); err != nil {
			return err
		}
		if err := applyInfraTestCredentials(ctx, kubeClient, o.Namespace); err != nil {
			return err
		}
	}

	if o.VaultSecretPath != "" {
		data, err := os.ReadFile(o.VaultSecretPath)
		if err != nil {
			return fmt.Errorf("failed to read vault secret file: %w", err)
		}
		if err := kubeClient.ApplyManifest(ctx, o.Namespace, data); err != nil {
			return fmt.Errorf("failed to apply vault secret: %w", err)
		}
	}

	return upgradeInstall(ctx, o)
}
