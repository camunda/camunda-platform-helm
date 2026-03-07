package cmd

import (
	"context"
	"fmt"
	"os"
	"scripts/camunda-core/pkg/logging"
	"scripts/deploy-camunda/entra"
	"scripts/prepare-helm-values/pkg/env"

	"github.com/spf13/cobra"
)

// newEntraCommand creates the "entra" parent command with ensure-venom-app and cleanup-venom-app subcommands.
func newEntraCommand() *cobra.Command {
	entraCmd := &cobra.Command{
		Use:   "entra",
		Short: "Manage Microsoft Entra ID app registrations for OIDC integration tests",
	}

	entraCmd.AddCommand(newEntraEnsureCommand())
	entraCmd.AddCommand(newEntraCleanupCommand())
	entraCmd.AddCommand(newEntraUpdateRedirectURIsCommand())

	return entraCmd
}

// newEntraEnsureCommand creates the "entra ensure-venom-app" subcommand.
// It provisions an Entra app, creates a K8s secret, and prints
// VENOM_CLIENT_ID=<id> and CONNECTORS_CLIENT_ID=<id> to stdout
// so the CI workflow can capture and export them.
func newEntraEnsureCommand() *cobra.Command {
	var (
		namespace   string
		kubeContext string
		logLevel    string
		envFile     string
	)

	cmd := &cobra.Command{
		Use:   "ensure-venom-app",
		Short: "Provision a venom Entra app registration and create the K8s secret",
		Long: `Provision a venom Entra app registration for OIDC integration tests.

This command:
1. Authenticates to Microsoft Graph API using parent app credentials.
2. Finds or creates an app registration named "venom-test-<namespace>".
3. Rotates the client secret.
4. Ensures a service principal exists.
5. Creates/updates the venom-entra-credentials K8s secret.
6. Prints VENOM_CLIENT_ID=<id> and CONNECTORS_CLIENT_ID=<id> to stdout.

Environment variables (or set in .env file):
  ENTRA_APP_DIRECTORY_ID  - Entra tenant/directory ID
  ENTRA_APP_CLIENT_ID     - Parent app client ID (for Graph API auth)
  ENTRA_APP_CLIENT_SECRET - Parent app client secret`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := setupEntraLogging(logLevel); err != nil {
				return err
			}
			loadEnvFile(envFile)

			opts := entra.Options{
				Namespace:   namespace,
				KubeContext: kubeContext,
			}

			app, err := entra.EnsureVenomApp(context.Background(), opts)
			if err != nil {
				return err
			}

			// Print to stdout so CI can capture with >> $GITHUB_ENV.
			fmt.Fprintf(os.Stdout, "VENOM_CLIENT_ID=%s\n", app.AppID)
			fmt.Fprintf(os.Stdout, "CONNECTORS_CLIENT_ID=%s\n", os.Getenv("ENTRA_APP_CLIENT_ID"))

			return nil
		},
	}

	f := cmd.Flags()
	f.StringVarP(&namespace, "namespace", "n", "", "Kubernetes namespace (required)")
	f.StringVar(&kubeContext, "kube-context", "", "Kubernetes context (uses default if empty)")
	f.StringVarP(&logLevel, "log-level", "l", "info", "Log level (debug, info, warn, error)")
	f.StringVar(&envFile, "env-file", "", "Path to .env file (defaults to .env in current dir)")
	_ = cmd.MarkFlagRequired("namespace")

	return cmd
}

// newEntraUpdateRedirectURIsCommand creates the "entra update-redirect-uris" subcommand.
// It replaces the bash-based Taskfile task that was accumulating redirect URIs
// unboundedly. This Go implementation prunes stale CI URIs before adding the
// current deployment's URIs, preventing the Microsoft Graph API limit from
// being exceeded.
func newEntraUpdateRedirectURIsCommand() *cobra.Command {
	var (
		ingressHost string
		objectID    string
		logLevel    string
		envFile     string
	)

	cmd := &cobra.Command{
		Use:   "update-redirect-uris",
		Short: "Update redirect URIs on the parent Entra app registration",
		Long: `Update redirect URIs on the parent Entra app registration.

This command:
1. Authenticates to Microsoft Graph API using parent app credentials.
2. Fetches the current web and SPA redirect URIs from the app registration.
3. Removes stale CI redirect URIs (those matching *.ci.distro.ultrawombat.com).
4. Removes malformed URIs (e.g., trailing commas from data corruption).
5. Adds the redirect URIs for the current deployment's ingress host.
6. PATCHes the updated URI lists back to the app registration.

Environment variables (or set in .env file):
  ENTRA_APP_DIRECTORY_ID  - Entra tenant/directory ID
  ENTRA_APP_CLIENT_ID     - Parent app client ID (for Graph API auth)
  ENTRA_APP_CLIENT_SECRET - Parent app client secret
  ENTRA_APP_OBJECT_ID     - Parent app object ID (target for redirect URIs)`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := setupEntraLogging(logLevel); err != nil {
				return err
			}
			loadEnvFile(envFile)

			opts := entra.RedirectURIOptions{
				IngressHost: ingressHost,
				ObjectID:    objectID,
			}

			return entra.UpdateRedirectURIs(context.Background(), opts)
		},
	}

	f := cmd.Flags()
	f.StringVar(&ingressHost, "ingress-host", "", "Ingress hostname for the current deployment (required)")
	f.StringVar(&objectID, "object-id", "", "Entra app object ID (falls back to ENTRA_APP_OBJECT_ID env var)")
	f.StringVarP(&logLevel, "log-level", "l", "info", "Log level (debug, info, warn, error)")
	f.StringVar(&envFile, "env-file", "", "Path to .env file (defaults to .env in current dir)")
	_ = cmd.MarkFlagRequired("ingress-host")

	return cmd
}

// newEntraCleanupCommand creates the "entra cleanup-venom-app" subcommand.
func newEntraCleanupCommand() *cobra.Command {
	var (
		namespace string
		logLevel  string
		envFile   string
	)

	cmd := &cobra.Command{
		Use:   "cleanup-venom-app",
		Short: "Delete the per-namespace venom Entra app registration",
		Long: `Delete the venom Entra app registration for a namespace.
This is a best-effort cleanup — errors are logged but do not cause a non-zero exit.

Environment variables (or set in .env file):
  ENTRA_APP_DIRECTORY_ID  - Entra tenant/directory ID
  ENTRA_APP_CLIENT_ID     - Parent app client ID
  ENTRA_APP_CLIENT_SECRET - Parent app client secret`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := setupEntraLogging(logLevel); err != nil {
				return err
			}
			loadEnvFile(envFile)

			opts := entra.Options{
				Namespace: namespace,
			}

			// CleanupVenomApp is best-effort — it logs warnings but doesn't return errors.
			entra.CleanupVenomApp(context.Background(), opts)
			return nil
		},
	}

	f := cmd.Flags()
	f.StringVarP(&namespace, "namespace", "n", "", "Kubernetes namespace (required)")
	f.StringVarP(&logLevel, "log-level", "l", "info", "Log level (debug, info, warn, error)")
	f.StringVar(&envFile, "env-file", "", "Path to .env file (defaults to .env in current dir)")
	_ = cmd.MarkFlagRequired("namespace")

	return cmd
}

// setupEntraLogging initialises the logger for entra subcommands.
func setupEntraLogging(logLevel string) error {
	return logging.Setup(logging.Options{
		LevelString:  logLevel,
		ColorEnabled: logging.IsTerminal(os.Stdout.Fd()),
	})
}

// loadEnvFile loads a .env file, falling back to ".env" when path is empty.
func loadEnvFile(path string) {
	if path == "" {
		path = ".env"
	}
	if err := env.Load(path); err != nil {
		logging.Logger.Debug().Err(err).Str("envFile", path).Msg("Could not load env file (non-fatal)")
	}
}
