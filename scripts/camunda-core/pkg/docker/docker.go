package docker

import (
	"context"
	"os"
	"scripts/camunda-core/pkg/executil"
	"scripts/camunda-core/pkg/logging"
	"scripts/camunda-core/pkg/utils"
)

// EnsureDockerLogin logs into Docker Hub using the provided credentials.
// Falls back to HARBOR_USERNAME/HARBOR_PASSWORD, then TEST_DOCKER_USERNAME_CAMUNDA_CLOUD/TEST_DOCKER_PASSWORD_CAMUNDA_CLOUD,
// then NEXUS_USERNAME/NEXUS_PASSWORD env vars if username/password are empty.
// Deprecated: Use EnsureDockerHubLogin or EnsureHarborLogin for explicit registry targeting.
func EnsureDockerLogin(ctx context.Context, username, password string) error {
	if username == "" {
		username = utils.FirstNonEmpty(os.Getenv("HARBOR_USERNAME"), os.Getenv("TEST_DOCKER_USERNAME_CAMUNDA_CLOUD"), os.Getenv("NEXUS_USERNAME"))
	}
	if password == "" {
		password = utils.FirstNonEmpty(os.Getenv("HARBOR_PASSWORD"), os.Getenv("TEST_DOCKER_PASSWORD_CAMUNDA_CLOUD"), os.Getenv("NEXUS_PASSWORD"))
	}
	if username == "" || password == "" {
		logging.Logger.Debug().Msg("skipping docker login (credentials not provided)")
		return nil
	}

	logging.Logger.Debug().Str("registry", "docker.io").Msg("ensuring docker login")

	args := []string{"login", "--username", username, "--password-stdin"}
	return executil.RunCommandWithStdin(ctx, "docker", args, nil, "", []byte(password))
}

// EnsureDockerHubLogin logs into Docker Hub (index.docker.io) to avoid pull rate limits.
// Falls back to DOCKERHUB_USERNAME/DOCKERHUB_PASSWORD, then TEST_DOCKER_USERNAME/TEST_DOCKER_PASSWORD env vars.
func EnsureDockerHubLogin(ctx context.Context, username, password string) error {
	if username == "" {
		username = utils.FirstNonEmpty(os.Getenv("DOCKERHUB_USERNAME"), os.Getenv("TEST_DOCKER_USERNAME"))
	}
	if password == "" {
		password = utils.FirstNonEmpty(os.Getenv("DOCKERHUB_PASSWORD"), os.Getenv("TEST_DOCKER_PASSWORD"))
	}
	if username == "" || password == "" {
		logging.Logger.Debug().Msg("skipping Docker Hub login (credentials not provided)")
		return nil
	}

	logging.Logger.Debug().Str("registry", "docker.io").Msg("ensuring Docker Hub login")

	args := []string{"login", "--username", username, "--password-stdin"}
	return executil.RunCommandWithStdin(ctx, "docker", args, nil, "", []byte(password))
}

// EnsureHarborLogin logs into the Camunda Harbor registry (registry.camunda.cloud).
// Falls back to HARBOR_USERNAME/HARBOR_PASSWORD, then TEST_DOCKER_USERNAME_CAMUNDA_CLOUD/TEST_DOCKER_PASSWORD_CAMUNDA_CLOUD,
// then NEXUS_USERNAME/NEXUS_PASSWORD env vars.
func EnsureHarborLogin(ctx context.Context, username, password string) error {
	if username == "" {
		username = utils.FirstNonEmpty(os.Getenv("HARBOR_USERNAME"), os.Getenv("TEST_DOCKER_USERNAME_CAMUNDA_CLOUD"), os.Getenv("NEXUS_USERNAME"))
	}
	if password == "" {
		password = utils.FirstNonEmpty(os.Getenv("HARBOR_PASSWORD"), os.Getenv("TEST_DOCKER_PASSWORD_CAMUNDA_CLOUD"), os.Getenv("NEXUS_PASSWORD"))
	}
	if username == "" || password == "" {
		logging.Logger.Debug().Msg("skipping Harbor login (credentials not provided)")
		return nil
	}

	logging.Logger.Debug().Str("registry", "registry.camunda.cloud").Msg("ensuring Harbor login")

	args := []string{"login", "registry.camunda.cloud", "--username", username, "--password-stdin"}
	return executil.RunCommandWithStdin(ctx, "docker", args, nil, "", []byte(password))
}
