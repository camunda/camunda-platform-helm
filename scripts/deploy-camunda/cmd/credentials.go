package cmd

import (
	"bufio"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"scripts/camunda-core/pkg/logging"
	"scripts/deploy-camunda/config"
	"scripts/deploy-camunda/credentials"
	"scripts/deploy-camunda/matrix"
	"scripts/prepare-helm-values/pkg/env"
)

var credentialStore credentials.Store = credentials.KeyringStore{}

type registryCredentialSource struct {
	registry           string
	username, password *string
	required           bool
	envPairs           [][2]string
}

func newCredentialsCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "credentials", Short: "Manage registry credentials in the OS keyring"}
	cmd.AddCommand(newCredentialsConfigureCommand(), newCredentialsStatusCommand(), newCredentialsDeleteCommand())
	return cmd
}

func newCredentialsConfigureCommand() *cobra.Command {
	var registry string
	cmd := &cobra.Command{Use: "configure", Short: "Securely store Harbor or Docker Hub credentials", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		registry, err := canonicalCredentialRegistry(registry)
		if err != nil {
			return err
		}
		reader := bufio.NewReader(cmd.InOrStdin())
		username, err := promptLine(cmd.Context(), cmd.OutOrStdout(), reader, "Username", "")
		if err != nil {
			return err
		}
		password, err := promptSecret(cmd.Context(), cmd.OutOrStdout(), reader, "Password or access token")
		if err != nil {
			return err
		}
		if err := credentialStore.Set(registry, credentials.Credential{Username: username, Password: password}); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Stored %s credentials in the OS keyring.\n", registry)
		return nil
	}}
	cmd.Flags().StringVar(&registry, "registry", "", "Registry: harbor or dockerhub")
	_ = cmd.MarkFlagRequired("registry")
	return cmd
}

func newCredentialsStatusCommand() *cobra.Command {
	return &cobra.Command{Use: "status", Short: "Show which registry credentials are configured", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		for _, registry := range []string{credentials.HarborRegistry, credentials.DockerHubRegistry} {
			credential, found, err := credentialStore.Get(registry)
			if err != nil {
				return err
			}
			if found {
				fmt.Fprintf(cmd.OutOrStdout(), "%s: configured (%s)\n", registry, credential.Username)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "%s: not configured\n", registry)
			}
		}
		return nil
	}}
}

func newCredentialsDeleteCommand() *cobra.Command {
	var registry string
	cmd := &cobra.Command{Use: "delete", Short: "Delete registry credentials from the OS keyring", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		registry, err := canonicalCredentialRegistry(registry)
		if err != nil {
			return err
		}
		if err := credentialStore.Delete(registry); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Deleted %s credentials from the OS keyring.\n", registry)
		return nil
	}}
	cmd.Flags().StringVar(&registry, "registry", "", "Registry: harbor or dockerhub")
	_ = cmd.MarkFlagRequired("registry")
	return cmd
}

func canonicalCredentialRegistry(value string) (string, error) {
	switch value {
	case "harbor", credentials.HarborRegistry:
		return credentials.HarborRegistry, nil
	case "dockerhub", credentials.DockerHubRegistry:
		return credentials.DockerHubRegistry, nil
	default:
		return "", fmt.Errorf("unsupported registry %q; use harbor or dockerhub", value)
	}
}

func resolveRegistryCredentials(docker *config.DockerFlags) error {
	if err := resolveRegistryCredentialsFromEnvironment(docker); err != nil {
		return err
	}
	for _, item := range registryCredentialSources(docker) {
		if !item.required || *item.username != "" {
			continue
		}
		credential, found, err := credentials.GetOptional(credentialStore, item.registry)
		if err != nil {
			return err
		}
		if found {
			*item.username, *item.password = credential.Username, credential.Password
		}
	}
	return nil
}

func resolveRegistryCredentialsFromEnvironment(docker *config.DockerFlags) error {
	for _, item := range registryCredentialSources(docker) {
		if !item.required {
			continue
		}
		if (*item.username == "") != (*item.password == "") {
			return fmt.Errorf("%s flags/config must provide both username and password/token", item.registry)
		}
		if *item.username != "" {
			continue
		}
		for _, pair := range item.envPairs {
			username, password := os.Getenv(pair[0]), os.Getenv(pair[1])
			if username == "" && password == "" {
				continue
			}
			if username == "" || password == "" {
				set, unset := pair[0], pair[1]
				if username == "" {
					set, unset = pair[1], pair[0]
				}
				logging.Logger.Warn().
					Str("registry", item.registry).
					Str("set", set).
					Str("unset", unset).
					Msg("Ignoring half-configured registry credentials from the environment; trying the keyring next")
				continue
			}
			*item.username, *item.password = username, password
			break
		}
		if *item.username != "" {
			continue
		}
	}
	return nil
}

func registryCredentialSources(docker *config.DockerFlags) []registryCredentialSource {
	return []registryCredentialSource{
		{credentials.HarborRegistry, &docker.DockerUsername, &docker.DockerPassword, docker.EnsureDockerRegistry, [][2]string{{"HARBOR_USERNAME", "HARBOR_PASSWORD"}, {"TEST_DOCKER_USERNAME_CAMUNDA_CLOUD", "TEST_DOCKER_PASSWORD_CAMUNDA_CLOUD"}, {"NEXUS_USERNAME", "NEXUS_PASSWORD"}}},
		{credentials.DockerHubRegistry, &docker.DockerHubUsername, &docker.DockerHubPassword, docker.EnsureDockerHub, [][2]string{{"DOCKERHUB_USERNAME", "DOCKERHUB_PASSWORD"}, {"TEST_DOCKER_USERNAME", "TEST_DOCKER_PASSWORD"}}},
	}
}

func resolveRegistryCredentialsFromEnvFiles(docker *config.DockerFlags, entries []matrix.Entry, envFiles map[string]string, fallback string) error {
	if err := resolveRegistryCredentialsFromEnvironment(docker); err != nil {
		return err
	}
	resolveHarborFromFiles := docker.EnsureDockerRegistry && docker.DockerUsername == ""
	resolveDockerHubFromFiles := docker.EnsureDockerHub && docker.DockerHubUsername == ""
	seenPaths := map[string]bool{}
	var paths []string
	for _, entry := range entries {
		path := envFiles[entry.Version]
		if path == "" {
			path = fallback
		}
		if path != "" && !seenPaths[path] {
			seenPaths[path] = true
			paths = append(paths, path)
		}
	}
	for _, path := range paths {
		values, err := env.ReadFile(path)
		if err != nil {
			return err
		}
		if resolveHarborFromFiles {
			if err := mergeCredentialPair("Harbor", values, &docker.DockerUsername, &docker.DockerPassword, [][2]string{{"HARBOR_USERNAME", "HARBOR_PASSWORD"}, {"TEST_DOCKER_USERNAME_CAMUNDA_CLOUD", "TEST_DOCKER_PASSWORD_CAMUNDA_CLOUD"}, {"NEXUS_USERNAME", "NEXUS_PASSWORD"}}); err != nil {
				return fmt.Errorf("%s: %w", path, err)
			}
		}
		if resolveDockerHubFromFiles {
			if err := mergeCredentialPair("Docker Hub", values, &docker.DockerHubUsername, &docker.DockerHubPassword, [][2]string{{"DOCKERHUB_USERNAME", "DOCKERHUB_PASSWORD"}, {"TEST_DOCKER_USERNAME", "TEST_DOCKER_PASSWORD"}}); err != nil {
				return fmt.Errorf("%s: %w", path, err)
			}
		}
	}
	return resolveRegistryCredentials(docker)
}

func mergeCredentialPair(name string, values map[string]string, username, password *string, pairs [][2]string) error {
	for _, pair := range pairs {
		user, pass := values[pair[0]], values[pair[1]]
		if user == "" && pass == "" {
			continue
		}
		if user == "" || pass == "" {
			return fmt.Errorf("%s credential pair %s/%s is incomplete", name, pair[0], pair[1])
		}
		if *username != "" && (*username != user || *password != pass) {
			return fmt.Errorf("conflicting %s credentials", name)
		}
		*username, *password = user, pass
		return nil
	}
	return nil
}
