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

package docker

import (
	"context"
	"fmt"
	"scripts/camunda-core/pkg/executil"
	"scripts/camunda-core/pkg/logging"
	"scripts/camunda-core/pkg/registry"
)

// EnsureDockerHubLogin logs into Docker Hub (index.docker.io) to avoid pull rate limits.
// Falls back to DOCKERHUB_USERNAME/DOCKERHUB_PASSWORD, then TEST_DOCKER_USERNAME/TEST_DOCKER_PASSWORD env vars.
func EnsureDockerHubLogin(ctx context.Context, username, password string) error {
	username, password = registry.DockerHubLogin.ResolveEnv(username, password)
	if username == "" || password == "" {
		return fmt.Errorf("login to Docker Hub failed: credentials not provided (set DOCKERHUB_USERNAME/DOCKERHUB_PASSWORD or TEST_DOCKER_USERNAME/TEST_DOCKER_PASSWORD environment variables, or pass --dockerhub-username/--dockerhub-password flags)")
	}

	logging.Logger.Debug().Str("registry", "docker.io").Msg("ensuring Docker Hub login")

	args := []string{"login", "--username", username, "--password-stdin"}
	return executil.RunCommandWithStdin(ctx, "docker", args, nil, "", []byte(password))
}

// EnsureHarborLogin logs into the Camunda Harbor registry (registry.camunda.cloud).
// Falls back to HARBOR_USERNAME/HARBOR_PASSWORD, then TEST_DOCKER_USERNAME_CAMUNDA_CLOUD/TEST_DOCKER_PASSWORD_CAMUNDA_CLOUD,
// then NEXUS_USERNAME/NEXUS_PASSWORD env vars.
func EnsureHarborLogin(ctx context.Context, username, password string) error {
	return EnsureRegistryLogin(ctx, registry.Harbor, username, password)
}

// EnsureRegistryLogin logs into p.Host, resolving empty credentials through p.Login.
func EnsureRegistryLogin(ctx context.Context, p registry.Provider, username, password string) error {
	username, password = p.Login.ResolveEnv(username, password)
	if username == "" || password == "" {
		return fmt.Errorf("login to Harbor failed: credentials not provided (set HARBOR_USERNAME/HARBOR_PASSWORD, TEST_DOCKER_USERNAME_CAMUNDA_CLOUD/TEST_DOCKER_PASSWORD_CAMUNDA_CLOUD, or NEXUS_USERNAME/NEXUS_PASSWORD environment variables, or pass --docker-username/--docker-password flags)")
	}

	logging.Logger.Debug().Str("registry", p.Host).Msg("ensuring Harbor login")

	args := []string{"login", p.Host, "--username", username, "--password-stdin"}
	return executil.RunCommandWithStdin(ctx, "docker", args, nil, "", []byte(password))
}
