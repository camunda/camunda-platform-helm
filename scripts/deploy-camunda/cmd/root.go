package cmd

import (
	"context"
	"fmt"
	"os"
	"scripts/camunda-core/pkg/completion"
	"scripts/camunda-core/pkg/logging"
	"scripts/deploy-camunda/config"
	"scripts/deploy-camunda/deploy"
	"scripts/deploy-camunda/format"
	"scripts/prepare-helm-values/pkg/env"
	"strings"

	"github.com/spf13/cobra"
)

var (
	// Global flags
	configFile string
	flags      config.RuntimeFlags
)

// NewRootCommand creates the root command.
func NewRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "deploy-camunda",
		Short: "Deploy Camunda Platform with prepared Helm values",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Skip for config and completion subcommands
			if cmd != nil {
				if cmd.Name() == "config" || (cmd.Parent() != nil && cmd.Parent().Name() == "config") {
					return nil
				}
				if cmd.Name() == "completion" ||
					cmd.Name() == cobra.ShellCompRequestCmd ||
					cmd.Name() == cobra.ShellCompNoDescRequestCmd {
					return nil
				}
			}

			// Load .env file
			if flags.EnvFile != "" {
				_ = env.Load(flags.EnvFile)
			} else {
				_ = env.Load(".env")
			}

			// Load config and merge with flags
			cfgPath, err := config.ResolvePath(configFile)
			if err != nil {
				return err
			}
			rc, err := config.Read(cfgPath, true)
			if err != nil {
				return err
			}

			// Apply active deployment defaults
			if err := config.ApplyActiveDeployment(rc, rc.Current, &flags); err != nil {
				return err
			}

			// Validate merged configuration
			if err := config.Validate(&flags); err != nil {
				return err
			}

			// Validate chartPath exists
			if strings.TrimSpace(flags.ChartPath) != "" {
				if fi, err := os.Stat(flags.ChartPath); err != nil || !fi.IsDir() {
					return fmt.Errorf("resolved chart path %q does not exist or is not a directory; set --repo-root/--chart/--version or --chart-path explicitly", flags.ChartPath)
				}
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Setup logging
			if err := logging.Setup(logging.Options{
				LevelString:  flags.LogLevel,
				ColorEnabled: logging.IsTerminal(os.Stdout.Fd()),
			}); err != nil {
				return err
			}

			// Log flags
			format.PrintFlags(cmd.Flags())

			// Execute deployment
			return deploy.Execute(context.Background(), &flags)
		},
	}

	// Persistent flags
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "F", "", "Path to config file (.camunda-deploy.yaml or ~/.config/camunda/deploy.yaml)")

	// Deployment flags
	f := rootCmd.Flags()
	f.StringVar(&flags.ChartPath, "chart-path", "", "Path to the Camunda chart directory")
	f.StringVarP(&flags.Chart, "chart", "c", "", "Chart name")
	f.StringVarP(&flags.ChartVersion, "version", "v", "", "Chart version (only valid with --chart; not allowed with --chart-path)")
	f.StringVarP(&flags.Namespace, "namespace", "n", "", "Kubernetes namespace")
	f.StringVarP(&flags.Release, "release", "r", "", "Helm release name")
	f.StringVarP(&flags.Scenario, "scenario", "s", "", "The name of the scenario to deploy (comma-separated for parallel deployment)")
	f.StringVar(&flags.ScenarioPath, "scenario-path", "", "Path to scenario files")
	f.StringVar(&flags.Auth, "auth", "keycloak", "Auth scenario")
	f.StringVar(&flags.Platform, "platform", "gke", "Target platform: gke, rosa, eks")
	f.StringVarP(&flags.LogLevel, "log-level", "l", "info", "Log level")
	f.BoolVar(&flags.SkipDependencyUpdate, "skip-dependency-update", true, "Skip Helm dependency update")
	f.BoolVar(&flags.ExternalSecrets, "external-secrets", true, "Enable external secrets")
	f.StringVar(&flags.KeycloakHost, "keycloak-host", "keycloak-24-9-0.ci.distro.ultrawombat.com", "Keycloak external host")
	f.StringVar(&flags.KeycloakProtocol, "keycloak-protocol", "https", "Keycloak protocol")
	f.StringVar(&flags.KeycloakRealm, "keycloak-realm", "", "Keycloak realm name (auto-generated if not specified)")
	f.StringVar(&flags.OptimizeIndexPrefix, "optimize-index-prefix", "", "Optimize Elasticsearch index prefix (auto-generated if not specified)")
	f.StringVar(&flags.OrchestrationIndexPrefix, "orchestration-index-prefix", "", "Orchestration Elasticsearch index prefix (auto-generated if not specified)")
	f.StringVar(&flags.TasklistIndexPrefix, "tasklist-index-prefix", "", "Tasklist Elasticsearch index prefix (auto-generated if not specified)")
	f.StringVar(&flags.OperateIndexPrefix, "operate-index-prefix", "", "Operate Elasticsearch index prefix (auto-generated if not specified)")
	f.StringVar(&flags.RepoRoot, "repo-root", "", "Repository root path")
	f.StringVar(&flags.Flow, "flow", "install", "Flow type")
	f.StringVar(&flags.EnvFile, "env-file", "", "Path to .env file (defaults to .env in current dir)")
	f.BoolVar(&flags.Interactive, "interactive", true, "Enable interactive prompts for missing variables")
	f.StringVar(&flags.VaultSecretMapping, "vault-secret-mapping", "", "Vault secret mapping content")
	f.BoolVar(&flags.AutoGenerateSecrets, "auto-generate-secrets", false, "Auto-generate certain secrets for testing purposes")
	f.BoolVar(&flags.DeleteNamespaceFirst, "delete-namespace", false, "Delete the namespace first, then deploy")
	f.StringVar(&flags.DockerUsername, "docker-username", "", "Docker registry username")
	f.StringVar(&flags.DockerPassword, "docker-password", "", "Docker registry password")
	f.BoolVar(&flags.EnsureDockerRegistry, "ensure-docker-registry", false, "Ensure Docker registry secret is created")
	f.BoolVar(&flags.RenderTemplates, "render-templates", false, "Render manifests to a directory instead of installing")
	f.StringVar(&flags.RenderOutputDir, "render-output-dir", "", "Output directory for rendered manifests (defaults to ./rendered/<release>)")
	f.StringSliceVar(&flags.ExtraValues, "extra-values", nil, "Additional Helm values files to apply last (comma-separated or repeatable)")
	f.StringVar(&flags.ValuesPreset, "values-preset", "", "Shortcut to append values-<preset>.yaml from chartPath if present (e.g. latest, enterprise)")
	f.StringVar(&flags.IngressSubdomain, "ingress-subdomain", "", "Ingress subdomain (appended to ."+config.DefaultIngressBaseDomain+")")
	f.StringVar(&flags.IngressHostname, "ingress-hostname", "", "Full ingress hostname (overrides --ingress-subdomain)")
	f.IntVar(&flags.Timeout, "timeout", 5, "Timeout in minutes for Helm deployment")

	// Register completions
	completion.RegisterScenarioCompletion(rootCmd, "scenario", "chart-path")
	completion.RegisterScenarioCompletion(rootCmd, "auth", "chart-path")

	return rootCmd
}

// Execute runs the root command.
func Execute() error {
	rootCmd := NewRootCommand()
	rootCmd.AddCommand(newCompletionCommand(rootCmd))
	rootCmd.AddCommand(newConfigCommand())
	return rootCmd.Execute()
}
