package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"scripts/camunda-core/pkg/logging"
	"scripts/deploy-camunda/auth0"
	"scripts/prepare-helm-values/pkg/env"

	"github.com/spf13/cobra"
)

// newAuth0Command creates the "auth0" parent command with ensure-clients and
// cleanup-clients subcommands. Mirrors the entra command's shape.
func newAuth0Command() *cobra.Command {
	auth0Cmd := &cobra.Command{
		Use:   "auth0",
		Short: "Manage Auth0 OIDC clients for integration tests",
	}

	auth0Cmd.AddCommand(newAuth0EnsureCommand())
	auth0Cmd.AddCommand(newAuth0CleanupCommand())

	return auth0Cmd
}

// newAuth0EnsureCommand creates the "auth0 ensure-clients" subcommand.
// Provisions one OIDC client per Camunda component, grants each the
// audience, writes the K8s secret, and prints
// AUTH0_<COMPONENT>_CLIENT_ID=<id> to stdout for CI to capture.
func newAuth0EnsureCommand() *cobra.Command {
	var (
		namespace      string
		kubeContext    string
		ingressHost    string
		audience       string
		domain         string
		secretName     string
		logLevel       string
		envFile        string
		skipK8sSecret  bool
		valuesFragment string
	)

	cmd := &cobra.Command{
		Use:   "ensure-clients",
		Short: "Provision Auth0 OIDC clients for a Camunda integration test deployment",
		Long: `Provision Auth0 OIDC clients for a Camunda integration test deployment.

Creates one first-party Auth0 client per Camunda component:

  identity, orchestration, optimize          regular_web (auth_code + refresh + client_credentials)
  connectors                                 non_interactive (M2M only)
  Web Modeler, Console                       spa (auth_code + refresh, no secret)

Then grants each private client access to the configured API audience and
writes a Kubernetes secret containing the client_secret values for chart
consumption.

Clients are named "<namespace>-<component>" so multiple parallel CI runs can
coexist on a single Auth0 tenant. Cleanup looks them up by the same name.

Environment variables (or set in .env file):
  AUTH0_DOMAIN              Auth0 tenant base URL (e.g. https://distribution-team.eu.auth0.com)
  AUTH0_MGMT_TOKEN          Pre-acquired Management API token (preferred)
  AUTH0_MGMT_CLIENT_ID      M2M client_id used to acquire a Management API token
  AUTH0_MGMT_CLIENT_SECRET  M2M client_secret used to acquire a Management API token`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := setupAuth0Logging(logLevel); err != nil {
				return err
			}
			loadAuth0EnvFile(envFile)

			opts := auth0.Options{
				Namespace:     namespace,
				KubeContext:   kubeContext,
				IngressHost:   ingressHost,
				Audience:      audience,
				Domain:        domain,
				SecretName:    secretName,
				SkipK8sSecret: skipK8sSecret,
			}

			prov, err := auth0.EnsureClients(context.Background(), opts)
			if err != nil {
				return err
			}

			// Stable env-var output for CI capture (>> $GITHUB_ENV).
			// Format: AUTH0_<COMPONENT_UPPER>_CLIENT_ID=<id>, plus *_CLIENT_SECRET
			// for confidential clients only.
			for _, c := range prov.All() {
				prefix := "AUTH0_" + envify(c.Component)
				fmt.Fprintf(os.Stdout, "%s_CLIENT_ID=%s\n", prefix, c.ClientID)
				if !c.Public {
					fmt.Fprintf(os.Stdout, "%s_CLIENT_SECRET=%s\n", prefix, c.ClientSecret)
				}
			}

			if valuesFragment != "" {
				if err := writeAuth0ValuesFragment(valuesFragment, prov); err != nil {
					return fmt.Errorf("write values fragment: %w", err)
				}
				logging.Logger.Info().
					Str("path", valuesFragment).
					Msg("Wrote Helm values fragment with provisioned client_ids")
			}

			return nil
		},
	}

	f := cmd.Flags()
	f.StringVarP(&namespace, "namespace", "n", "", "Kubernetes namespace (required)")
	f.StringVar(&kubeContext, "kube-context", "", "Kubernetes context (uses default if empty)")
	f.StringVar(&ingressHost, "ingress-host", "", "Ingress hostname for redirect URIs (required)")
	f.StringVar(&audience, "audience", "", "Auth0 API audience (defaults to "+auth0.DefaultAudience+")")
	f.StringVar(&domain, "domain", "", "Auth0 tenant base URL (falls back to AUTH0_DOMAIN env var)")
	f.StringVar(&secretName, "secret-name", "", "K8s secret name (defaults to "+auth0.DefaultSecretName+")")
	f.BoolVar(&skipK8sSecret, "skip-k8s-secret", false, "Skip K8s secret creation; caller must invoke separately later")
	f.StringVar(&valuesFragment, "values-output", "", "Optional path to write a Helm values overlay containing the provisioned client_ids")
	f.StringVarP(&logLevel, "log-level", "l", "info", "Log level (debug, info, warn, error)")
	f.StringVar(&envFile, "env-file", "", "Path to .env file (defaults to .env in current dir)")
	_ = cmd.MarkFlagRequired("namespace")
	_ = cmd.MarkFlagRequired("ingress-host")

	return cmd
}

// newAuth0CleanupCommand creates the "auth0 cleanup-clients" subcommand.
// Looks up clients named "<namespace>-<component>" via the Management API and
// deletes them. Best-effort — errors are logged but do not produce a non-zero
// exit code.
func newAuth0CleanupCommand() *cobra.Command {
	var (
		namespace string
		domain    string
		logLevel  string
		envFile   string
	)

	cmd := &cobra.Command{
		Use:   "cleanup-clients",
		Short: "Delete the per-namespace Auth0 OIDC clients",
		Long: `Delete the Auth0 OIDC clients created by ensure-clients for a namespace.

Clients are looked up by name pattern "<namespace>-<component>" via the
Management API. This is a best-effort cleanup — errors are logged but do not
cause a non-zero exit.

Environment variables (or set in .env file):
  AUTH0_DOMAIN              Auth0 tenant base URL
  AUTH0_MGMT_TOKEN          Pre-acquired Management API token (preferred)
  AUTH0_MGMT_CLIENT_ID      M2M client_id used to acquire a Management API token
  AUTH0_MGMT_CLIENT_SECRET  M2M client_secret used to acquire a Management API token`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := setupAuth0Logging(logLevel); err != nil {
				return err
			}
			loadAuth0EnvFile(envFile)

			// IngressHost is unused on cleanup but resolveOpts requires it; pass
			// a placeholder. Audience also unused for cleanup.
			opts := auth0.Options{
				Namespace:   namespace,
				Domain:      domain,
				IngressHost: "cleanup.invalid",
			}

			auth0.CleanupClients(context.Background(), opts)
			return nil
		},
	}

	f := cmd.Flags()
	f.StringVarP(&namespace, "namespace", "n", "", "Kubernetes namespace (required)")
	f.StringVar(&domain, "domain", "", "Auth0 tenant base URL (falls back to AUTH0_DOMAIN env var)")
	f.StringVarP(&logLevel, "log-level", "l", "info", "Log level (debug, info, warn, error)")
	f.StringVar(&envFile, "env-file", "", "Path to .env file (defaults to .env in current dir)")
	_ = cmd.MarkFlagRequired("namespace")

	return cmd
}

// setupAuth0Logging initialises the logger for auth0 subcommands.
func setupAuth0Logging(logLevel string) error {
	return logging.Setup(logging.Options{
		LevelString:  logLevel,
		ColorEnabled: logging.IsTerminal(os.Stdout.Fd()),
	})
}

// loadAuth0EnvFile loads a .env file, defaulting to ".env".
func loadAuth0EnvFile(path string) {
	if path == "" {
		path = ".env"
	}
	if err := env.Load(path); err != nil {
		logging.Logger.Debug().Err(err).Str("envFile", path).Msg("Could not load env file (non-fatal)")
	}
}

// envify converts a component name to an env-var-friendly suffix:
// "Web Modeler" → "WEB_MODELER", "connectors" → "CONNECTORS".
func envify(component string) string {
	s := strings.ToUpper(component)
	s = strings.ReplaceAll(s, " ", "_")
	s = strings.ReplaceAll(s, "-", "_")
	return s
}

// writeAuth0ValuesFragment emits a Helm values overlay containing the freshly
// provisioned client_ids in every location the chart references them. Apply
// it with "-f <path>" AFTER the base values so the deep merge replaces stale
// clientIds.
func writeAuth0ValuesFragment(path string, prov *auth0.Provisioned) error {
	get := func(component string) string {
		c, ok := prov.ByComponent(component)
		if !ok {
			return ""
		}
		return c.ClientID
	}
	identity := get(auth0.ComponentIdentity)
	orchestration := get(auth0.ComponentOrchestration)
	optimize := get(auth0.ComponentOptimize)
	connectors := get(auth0.ComponentConnectors)
	webModeler := get(auth0.ComponentWebModeler)
	console := get(auth0.ComponentConsole)

	for name, id := range map[string]string{
		"identity":      identity,
		"orchestration": orchestration,
		"optimize":      optimize,
		"connectors":    connectors,
		"Web Modeler":   webModeler,
		"Console":       console,
	} {
		if id == "" {
			return fmt.Errorf("missing client_id for %q", name)
		}
	}

	body := fmt.Sprintf(`# Generated by 'deploy-camunda auth0 ensure-clients'.
# Apply AFTER your base values:  helm upgrade -f base.yaml -f %s ...
global:
  identity:
    auth:
      identity:
        clientId: %s
      orchestration:
        clientId: %s
      optimize:
        clientId: %s
      connectors:
        clientId: %s
      console:
        clientId: %s
      webModeler:
        clientId: %s

orchestration:
  security:
    authentication:
      oidc:
        clientId: %s
    initialization:
      defaultRoles:
        connectors:
          clients:
            - %s

connectors:
  security:
    authentication:
      oidc:
        clientId: %s
`,
		path,
		identity, orchestration, optimize, connectors, console, webModeler,
		orchestration, connectors, connectors,
	)
	return os.WriteFile(path, []byte(body), 0o644)
}
