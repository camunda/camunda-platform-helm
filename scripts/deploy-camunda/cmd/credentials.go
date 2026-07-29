package cmd

import (
	"bufio"
	"fmt"

	"github.com/spf13/cobra"

	"scripts/deploy-camunda/credentials"
)

var credentialStore credentials.Store = credentials.KeyringStore{}

func registerRootCommands(root *cobra.Command) {
	root.AddCommand(newCompletionCommand(root), newConfigCommand(), newCredentialsCommand(), newMatrixCommand(), newPrepareValuesCommand(), newEntraCommand(), newWatchCommand(), newAuth0Command(), newDoctorCommand(), newTriageCommand(), newDiagnosticsCommand(), newCICommand(), newE2EEnvCommand(), newTopologyCommand())
}

func newCredentialsCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "credentials", Short: "Manage registry credentials in the OS keyring"}
	cmd.AddCommand(newCredentialsConfigureCommand(), newCredentialsStatusCommand(), newCredentialsDeleteCommand())
	return cmd
}

func newCredentialsConfigureCommand() *cobra.Command {
	var registry string
	cmd := &cobra.Command{
		Use:   "configure",
		Short: "Securely store Harbor or Docker Hub credentials",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
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
		},
	}
	cmd.Flags().StringVar(&registry, "registry", "", "Registry: harbor or dockerhub")
	_ = cmd.MarkFlagRequired("registry")
	return cmd
}

func newCredentialsStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use: "status", Short: "Show which registry credentials are configured", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
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
		},
	}
}

func newCredentialsDeleteCommand() *cobra.Command {
	var registry string
	cmd := &cobra.Command{
		Use: "delete", Short: "Delete registry credentials from the OS keyring", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			registry, err := canonicalCredentialRegistry(registry)
			if err != nil {
				return err
			}
			if err := credentialStore.Delete(registry); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted %s credentials from the OS keyring.\n", registry)
			return nil
		},
	}
	cmd.Flags().StringVar(&registry, "registry", "", "Registry: harbor or dockerhub")
	_ = cmd.MarkFlagRequired("registry")
	return cmd
}

func canonicalCredentialRegistry(value string) (string, error) {
	switch value {
	case "harbor", credentials.HarborRegistry:
		return credentials.HarborRegistry, nil
	case "dockerhub", "docker.io":
		return credentials.DockerHubRegistry, nil
	default:
		return "", fmt.Errorf("unsupported registry %q; use harbor or dockerhub", value)
	}
}

func resolveKeyringCredentialPairs(harborUser, harborPassword, hubUser, hubPassword *string, requireHarbor, requireHub bool) error {
	for _, item := range []struct {
		registry           string
		username, password *string
		required           bool
	}{
		{credentials.HarborRegistry, harborUser, harborPassword, requireHarbor}, {credentials.DockerHubRegistry, hubUser, hubPassword, requireHub},
	} {
		if !item.required || *item.username != "" || *item.password != "" {
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
