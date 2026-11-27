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

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"scripts/camunda-core/pkg/logging"
	"scripts/deploy-camunda/entra"
	"scripts/prepare-helm-values/pkg/env"
	"time"

	"github.com/spf13/cobra"
)

// newEntraCommand creates the "entra" parent command and its subcommands.
func newEntraCommand() *cobra.Command {
	entraCmd := &cobra.Command{
		Use:   "entra",
		Short: "Manage Microsoft Entra ID app registrations for OIDC integration tests",
	}

	entraCmd.AddCommand(newEntraEnsureCommand())
	entraCmd.AddCommand(newEntraCleanupCommand())
	entraCmd.AddCommand(newEntraCleanupLoginAppCommand())
	entraCmd.AddCommand(newEntraSweepLoginAppsCommand())
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
// It replaces the current host's redirect URIs, keeps other deployments' URIs,
// and evicts the oldest CI deployments only when the app would exceed Entra's
// redirect URI limit.
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
3. Replaces the current host's redirect URIs; other deployments' URIs are kept.
4. Removes malformed URIs (e.g., trailing commas from data corruption).
5. Adds the redirect URIs for the current deployment's ingress host.
6. If web, SPA and public-client URIs together exceed Entra's 256-URI limit,
   evicts whole CI deployments (*.ci.distro.ultrawombat.com), oldest first.
7. PATCHes the updated URI lists back to the app registration.
8. Reads the app back up to 6 times, 2-5s apart; if no read contains this
   host's URIs, repeats from step 3, failing after 3 writes.

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
	return newEntraAppCleanupCommand(
		"cleanup-venom-app",
		"Delete the per-namespace venom Entra app registration",
		"Delete the venom Entra app registration for a namespace.",
		entra.CleanupVenomApp,
	)
}

// newEntraCleanupLoginAppCommand creates the "entra cleanup-login-app" subcommand.
func newEntraCleanupLoginAppCommand() *cobra.Command {
	return newEntraAppCleanupCommand(
		"cleanup-login-app",
		"Delete the per-namespace Entra login app registration (camunda-oidc-<namespace>)",
		"Delete the Entra login app registration a qa-entra deployment provisions for itself.",
		entra.CleanupLoginApp,
	)
}

// newEntraSweepLoginAppsCommand creates the "entra sweep-login-apps" subcommand.
func newEntraSweepLoginAppsCommand() *cobra.Command {
	var (
		maxAge     time.Duration
		dryRun     bool
		outputFile string
		logLevel   string
		envFile    string
	)

	cmd := &cobra.Command{
		Use:   "sweep-login-apps",
		Short: "Delete QA Entra login app registrations left behind by finished deployments",
		Long: `Delete the login app registrations of QA Entra namespaces older than --max-age.

Every other app registration in the tenant is left alone. --max-age must stay
above the TTL of the namespaces these apps belong to, so a running deployment
keeps its app; values below 12h are rejected.

Environment variables (or set in .env file):
  ENTRA_APP_DIRECTORY_ID  - Entra tenant/directory ID
  ENTRA_APP_CLIENT_ID     - Parent app client ID
  ENTRA_APP_CLIENT_SECRET - Parent app client secret`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := setupEntraLogging(logLevel); err != nil {
				return err
			}
			loadEnvFile(envFile)

			result, err := entra.SweepLoginApps(context.Background(), entra.SweepOptions{
				MaxAge: maxAge,
				DryRun: dryRun,
			})
			if err != nil {
				return err
			}
			if err := writeSweepResult(outputFile, result); err != nil {
				return err
			}

			logging.Logger.Info().
				Int("scanned", result.Scanned).
				Int("candidates", len(result.Candidates)).
				Int("deleted", len(result.Deleted)).
				Int("failed", len(result.Failed)).
				Bool("dryRun", result.DryRun).
				Msg("Entra login app sweep finished")

			if len(result.Failed) > 0 {
				return fmt.Errorf("failed to delete %d of %d Entra app registrations", len(result.Failed), len(result.Candidates))
			}
			return nil
		},
	}

	f := cmd.Flags()
	f.DurationVar(&maxAge, "max-age", 24*time.Hour, "Keep app registrations younger than this")
	f.BoolVar(&dryRun, "dry-run", true, "List what would be deleted without deleting it")
	f.StringVarP(&outputFile, "output", "o", "", "Write the JSON sweep result to this file")
	f.StringVarP(&logLevel, "log-level", "l", "info", "Log level (debug, info, warn, error)")
	f.StringVar(&envFile, "env-file", "", "Path to .env file (defaults to .env in current dir)")

	return cmd
}

// writeSweepResult writes result as indented JSON to path, skipping an empty path.
func writeSweepResult(path string, result entra.SweepResult) error {
	if path == "" {
		return nil
	}
	payload, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("encode sweep result: %w", err)
	}
	if err := os.WriteFile(path, append(payload, '\n'), 0o600); err != nil {
		return fmt.Errorf("write sweep result to %s: %w", path, err)
	}
	return nil
}

// newEntraAppCleanupCommand builds a best-effort cleanup subcommand that runs
// cleanup for the --namespace flag.
func newEntraAppCleanupCommand(use, short, summary string, cleanup func(context.Context, entra.Options)) *cobra.Command {
	var (
		namespace string
		logLevel  string
		envFile   string
	)

	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Long: summary + `
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
			cleanup(context.Background(), entra.Options{Namespace: namespace})
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
