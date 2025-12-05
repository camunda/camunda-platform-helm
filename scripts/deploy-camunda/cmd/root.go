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
	"github.com/spf13/pflag"
)

var (
	// Global flags
	configFile string
	flags      config.RuntimeFlags
	explain    bool
	rootConfig *config.RootConfig
	flagsSet   map[string]bool
)

// NewRootCommand creates the root command.
func NewRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "deploy-camunda",
		Short: "Deploy Camunda Platform with prepared Helm values",
		Long: `Deploy Camunda Platform to Kubernetes with automated Helm values preparation.

This tool automates the deployment of Camunda Platform by:
  - Loading configuration from .camunda-deploy.yaml or ~/.config/camunda/deploy.yaml
  - Preparing scenario-specific Helm values files
  - Managing Keycloak realms and Elasticsearch index prefixes
  - Supporting parallel deployment of multiple scenarios

CONFIGURATION:
  Configuration can be provided via:
    1. CLI flags (highest priority)
    2. Environment variables (CAMUNDA_*)
    3. Config file deployments (selected via 'config use <name>')
    4. Config file root-level defaults

EXAMPLES:
  # Deploy using active config profile
  deploy-camunda

  # Deploy a specific scenario
  deploy-camunda --scenario keycloak --namespace my-ns --release integration

  # Deploy multiple scenarios in parallel
  deploy-camunda --scenario keycloak,keycloak-mt,saas --namespace integration

  # Preview deployment without executing
  deploy-camunda --dry-run

  # Show where each config value came from
  deploy-camunda --explain

  # Use a specific config file
  deploy-camunda -F /path/to/config.yaml

  # Validate configuration
  deploy-camunda validate`,
		Example: `  # Basic deployment
  deploy-camunda -n camunda -r integration -s keycloak --chart-path ./charts/camunda-platform-8.8

  # Deploy with external secrets and auto-generated test credentials
  deploy-camunda -n test -r integration -s keycloak --auto-generate-secrets --external-secrets

  # Render manifests without deploying
  deploy-camunda -n test -r integration -s keycloak --render-templates --render-output-dir ./output`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Skip for config, completion, and validate subcommands
			if cmd != nil {
				if cmd.Name() == "config" || (cmd.Parent() != nil && cmd.Parent().Name() == "config") {
					return nil
				}
				if cmd.Name() == "completion" ||
					cmd.Name() == cobra.ShellCompRequestCmd ||
					cmd.Name() == cobra.ShellCompNoDescRequestCmd {
					return nil
				}
				if cmd.Name() == "validate" {
					return nil
				}
			}

			// Track which flags were explicitly set
			flagsSet = make(map[string]bool)
			cmd.Flags().Visit(func(f *pflag.Flag) {
				flagsSet[f.Name] = true
			})

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
			rootConfig = rc

			// Apply active deployment defaults
			if err := config.ApplyActiveDeployment(rc, rc.Current, &flags); err != nil {
				return err
			}

			// For --explain mode, skip validation to show current state
			if explain {
				// Still parse scenarios if provided
				if flags.Scenario != "" {
					flags.Scenarios = strings.Split(flags.Scenario, ",")
					for i, s := range flags.Scenarios {
						flags.Scenarios[i] = strings.TrimSpace(s)
					}
				}
				return nil
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

			// Handle --explain mode
			if explain {
				exp := config.ExplainConfig(rootConfig, &flags, flagsSet)
				fmt.Println(exp.Format())
				return nil
			}

			// Log flags
			format.PrintFlags(cmd.Flags())

			// Execute deployment
			return deploy.Execute(context.Background(), &flags)
		},
	}

	// Persistent flags (available to all subcommands)
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "F", "",
		"Path to config file. Searches .camunda-deploy.yaml in current dir, then ~/.config/camunda/deploy.yaml")

	// Deployment flags
	f := rootCmd.Flags()

	// Chart source flags (mutually exclusive approaches)
	f.StringVar(&flags.ChartPath, "chart-path", "",
		"Local path to the Camunda chart directory (e.g., ./charts/camunda-platform-8.8)")
	f.StringVarP(&flags.Chart, "chart", "c", "",
		"Chart name for remote chart (e.g., camunda-platform). Use with --version")
	f.StringVarP(&flags.ChartVersion, "version", "v", "",
		"Chart version when using --chart for remote charts (not valid with --chart-path)")

	// Core deployment identifiers
	f.StringVarP(&flags.Namespace, "namespace", "n", "",
		"Kubernetes namespace for deployment. Created if it doesn't exist")
	f.StringVarP(&flags.Release, "release", "r", "",
		"Helm release name (e.g., integration, camunda)")
	f.StringVarP(&flags.Scenario, "scenario", "s", "",
		"Scenario name(s) to deploy. Use comma-separated values for parallel deployment (e.g., keycloak,keycloak-mt)")

	// Scenario configuration
	f.StringVar(&flags.ScenarioPath, "scenario-path", "",
		"Custom path to scenario values files. Default: <chart-path>/test/integration/scenarios/chart-full-setup")
	f.StringVar(&flags.Auth, "auth", "keycloak",
		"Authentication scenario to apply (e.g., keycloak, saas)")

	// Platform and environment
	f.StringVar(&flags.Platform, "platform", "gke",
		"Target Kubernetes platform. Affects platform-specific configurations (gke, rosa, eks)")
	f.StringVarP(&flags.LogLevel, "log-level", "l", "info",
		"Logging verbosity level (debug, info, warn, error)")
	f.StringVar(&flags.EnvFile, "env-file", "",
		"Path to .env file for environment variables. Default: .env in current directory")

	// Keycloak configuration
	f.StringVar(&flags.KeycloakHost, "keycloak-host", "keycloak-24-9-0.ci.distro.ultrawombat.com",
		"External Keycloak hostname for authentication")
	f.StringVar(&flags.KeycloakProtocol, "keycloak-protocol", "https",
		"Protocol for Keycloak connection (http, https)")
	f.StringVar(&flags.KeycloakRealm, "keycloak-realm", "",
		"Keycloak realm name. Auto-generated from scenario if not specified (max 36 chars)")

	// Elasticsearch index prefixes (for multi-tenancy isolation)
	f.StringVar(&flags.OptimizeIndexPrefix, "optimize-index-prefix", "",
		"Optimize Elasticsearch index prefix. Auto-generated if not specified")
	f.StringVar(&flags.OrchestrationIndexPrefix, "orchestration-index-prefix", "",
		"Orchestration Elasticsearch index prefix. Auto-generated if not specified")
	f.StringVar(&flags.TasklistIndexPrefix, "tasklist-index-prefix", "",
		"Tasklist Elasticsearch index prefix. Auto-generated if not specified")
	f.StringVar(&flags.OperateIndexPrefix, "operate-index-prefix", "",
		"Operate Elasticsearch index prefix. Auto-generated if not specified")

	// Repository and values configuration
	f.StringVar(&flags.RepoRoot, "repo-root", "",
		"Root path of the camunda-platform-helm repository")
	f.StringVar(&flags.ValuesPreset, "values-preset", "",
		"Append values-<preset>.yaml from chart path (e.g., latest, enterprise, local)")
	f.StringSliceVar(&flags.ExtraValues, "extra-values", nil,
		"Additional Helm values files applied last. Comma-separated or use multiple times")
	f.StringVar(&flags.IngressSubdomain, "ingress-subdomain", "",
		"Ingress subdomain prefix (combined with ci.distro.ultrawombat.com base domain)")
	f.StringVar(&flags.IngressHostname, "ingress-hostname", "",
		"Full ingress hostname override (bypasses subdomain + base domain construction)")

	// Deployment behavior
	f.StringVar(&flags.Flow, "flow", "install",
		"Deployment flow type (install, upgrade)")
	f.IntVar(&flags.Timeout, "timeout", 5,
		"Helm deployment timeout in minutes")
	f.BoolVar(&flags.SkipDependencyUpdate, "skip-dependency-update", true,
		"Skip 'helm dependency update' before deployment")
	f.BoolVar(&flags.DeleteNamespaceFirst, "delete-namespace", false,
		"Delete and recreate namespace before deployment (destructive)")
	f.BoolVar(&flags.Interactive, "interactive", true,
		"Enable interactive prompts for missing configuration values")

	// Secrets and authentication
	f.BoolVar(&flags.ExternalSecrets, "external-secrets", true,
		"Enable External Secrets Operator integration for secret management")
	f.StringVar(&flags.VaultSecretMapping, "vault-secret-mapping", "",
		"Vault secret mapping specification for External Secrets")
	f.BoolVar(&flags.AutoGenerateSecrets, "auto-generate-secrets", false,
		"Generate random test secrets (IDENTITY passwords, Keycloak secrets)")
	f.StringVar(&flags.DockerUsername, "docker-username", "",
		"Docker registry username for private image pulls")
	f.StringVar(&flags.DockerPassword, "docker-password", "",
		"Docker registry password (consider using env var for security)")
	f.BoolVar(&flags.EnsureDockerRegistry, "ensure-docker-registry", false,
		"Create Docker registry secret in namespace if credentials provided")

	// Output modes
	f.BoolVar(&flags.RenderTemplates, "render-templates", false,
		"Render Helm templates to files instead of deploying. Use for manifest review/GitOps")
	f.StringVar(&flags.RenderOutputDir, "render-output-dir", "",
		"Output directory for rendered manifests. Default: ./rendered/<release>")
	f.BoolVar(&flags.DryRun, "dry-run", false,
		"Preview deployment configuration and helm commands without executing")
	f.BoolVar(&explain, "explain", false,
		"Show where each configuration value came from (flag, env, config file, default)")

	var outputFormat string
	f.StringVarP(&outputFormat, "output", "o", "text",
		"Output format: text (default) or json for machine-readable output")

	// Custom flag parsing to convert string to OutputFormat
	rootCmd.PreRun = func(cmd *cobra.Command, args []string) {
		switch outputFormat {
		case "json":
			flags.OutputFormat = config.OutputFormatJSON
		default:
			flags.OutputFormat = config.OutputFormatText
		}
	}

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
	rootCmd.AddCommand(newValidateCommand())
	rootCmd.AddCommand(newInitCommand())
	rootCmd.AddCommand(newStatusCommand())
	rootCmd.AddCommand(newUninstallCommand())
	return rootCmd.Execute()
}
