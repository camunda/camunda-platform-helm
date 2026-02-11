package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"scripts/camunda-core/pkg/logging"
	"scripts/camunda-core/pkg/scenarios"
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

	// Raw debug flags (parsed into flags.DebugComponents in PreRunE)
	debugFlagsRaw []string
)

// isUsageError checks if an error is a usage/validation error that should show help.
func isUsageError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	// Check for common usage error patterns
	usagePatterns := []string{
		"unknown flag",
		"unknown shorthand flag",
		"required flag",
		"invalid argument",
		"accepts",
		"unknown command",
		"flag needs an argument",
		"invalid value",
	}
	for _, pattern := range usagePatterns {
		if strings.Contains(strings.ToLower(errStr), pattern) {
			return true
		}
	}
	return false
}

// NewRootCommand creates the root command.
func NewRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:           "deploy-camunda",
		Short:         "Deploy Camunda Platform with prepared Helm values",
		SilenceUsage:  true, // Don't show usage for runtime errors, only for usage errors
		SilenceErrors: true, // Handle all error printing ourselves
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

			// Load config and merge with flags first to get envFile from config
			if _, err := config.LoadAndMerge(configFile, true, &flags); err != nil {
				return err
			}

			// Load .env file - use config value if set, otherwise default to .env
			envFileToLoad := flags.EnvFile
			if envFileToLoad == "" {
				envFileToLoad = ".env"
			}
			logging.Logger.Debug().
				Str("envFile", envFileToLoad).
				Str("source", func() string {
					if flags.EnvFile != "" {
						return "config/flag"
					}
					return "default"
				}()).
				Msg("Loading environment file")
			_ = env.Load(envFileToLoad)

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

			// Parse debug flags into the DebugComponents map
			if len(debugFlagsRaw) > 0 {
				flags.DebugComponents = make(map[string]config.DebugConfig)
				for _, raw := range debugFlagsRaw {
					component, port, err := config.ParseDebugFlag(raw, flags.DebugPort)
					if err != nil {
						return fmt.Errorf("invalid --debug flag %q: %w", raw, err)
					}
					flags.DebugComponents[component] = config.DebugConfig{Port: port}
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
	f.StringVar(&flags.NamespacePrefix, "namespace-prefix", "", "Prefix to prepend to namespace (e.g., 'distribution' for EKS results in 'distribution-<namespace>')")
	f.StringVarP(&flags.Release, "release", "r", "", "Helm release name")
	f.StringVarP(&flags.Scenario, "scenario", "s", "", "The name of the scenario to deploy (comma-separated for parallel deployment)")
	f.StringVar(&flags.ScenarioPath, "scenario-path", "", "Path to scenario files")
	f.StringVar(&flags.Auth, "auth", "keycloak", "Auth scenario")
	f.StringVar(&flags.Platform, "platform", "gke", "Target platform: gke, rosa, eks")
	f.StringVarP(&flags.LogLevel, "log-level", "l", "info", "Log level")
	f.BoolVar(&flags.SkipDependencyUpdate, "skip-dependency-update", true, "Skip Helm dependency update")
	f.BoolVar(&flags.ExternalSecrets, "external-secrets", true, "Enable external secrets")
	f.StringVar(&flags.ExternalSecretsStore, "external-secrets-store", "", "External secrets store type (e.g., vault-backend)")
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
	f.StringVar(&flags.IngressSubdomain, "ingress-subdomain", "", "Ingress subdomain (requires --ingress-base-domain)")
	f.StringVar(&flags.IngressBaseDomain, "ingress-base-domain", "", "Base domain for ingress (ci.distro.ultrawombat.com or distribution.aws.camunda.cloud)")
	f.StringVar(&flags.IngressHostname, "ingress-hostname", "", "Full ingress hostname (overrides --ingress-subdomain)")
	f.IntVar(&flags.Timeout, "timeout", 5, "Timeout in minutes for Helm deployment")
	f.StringSliceVar(&debugFlagsRaw, "debug", nil, "Enable JVM remote debugging for component (repeatable, e.g., --debug orchestration:5005 --debug connectors:5006)")
	f.IntVar(&flags.DebugPort, "debug-port", 5005, "Default JVM debug port (used when no port specified in --debug)")
	f.BoolVar(&flags.DebugSuspend, "debug-suspend", false, "Suspend JVM on startup until debugger attaches")
	f.BoolVar(&flags.OutputTestEnv, "output-test-env", false, "Generate a .env file for E2E tests after deployment")
	f.StringVar(&flags.OutputTestEnvPath, "output-test-env-path", ".env.test", "Path for the test .env file output (for multi-scenario: used as base, e.g., .env.test.{scenario})")

	// Test execution flags
	f.BoolVar(&flags.RunIntegrationTests, "test-it", false, "Run integration tests after deployment")
	f.BoolVar(&flags.RunE2ETests, "test-e2e", false, "Run e2e tests after deployment")
	f.BoolVar(&flags.RunAllTests, "test-all", false, "Run both integration and e2e tests after deployment")
	f.StringVar(&flags.KubeContext, "kube-context", "", "Kubernetes context to use for deployment")
	f.BoolVar(&flags.UseVaultBackedSecrets, "use-vault-backed-secrets", false, "Use vault-backed external secrets (selects -vault.yaml suffix files)")
	f.BoolVar(&flags.RunTestsIT, "run-tests-it", false, "Run integration tests after deployment (runs against each deployed scenario)")
	f.BoolVar(&flags.RunTestsE2E, "run-tests-e2e", false, "Run e2e tests after deployment (runs against each deployed scenario)")

	// Register completions using config-aware completion function
	registerScenarioCompletion(rootCmd, "scenario")
	registerScenarioCompletion(rootCmd, "auth")
	registerKubeContextCompletion(rootCmd)
	registerPlatformCompletion(rootCmd)
	registerIngressBaseDomainCompletion(rootCmd)

	return rootCmd
}

// registerScenarioCompletion adds tab completion for scenario-related flags.
// It loads the config file and merges with CLI flags to resolve the scenario path.
// Supports comma-separated multi-select for the scenario flag.
func registerScenarioCompletion(cmd *cobra.Command, flagName string) {
	_ = cmd.RegisterFlagCompletionFunc(flagName, func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		// First check CLI flags
		scenarioPath, _ := cmd.Flags().GetString("scenario-path")
		if scenarioPath == "" {
			// Fall back to config file - create temporary flags with CLI values and merge config
			var tempFlags config.RuntimeFlags
			tempFlags.ScenarioPath, _ = cmd.Flags().GetString("scenario-path")
			tempFlags.ChartPath, _ = cmd.Flags().GetString("chart-path")

			if _, err := config.LoadAndMerge(configFile, false, &tempFlags); err == nil {
				scenarioPath = tempFlags.ScenarioPath
			}
		}

		if scenarioPath == "" {
			return cobra.AppendActiveHelp(nil, "Please specify --scenario-path or configure scenarioRoot in your deployment config"), cobra.ShellCompDirectiveNoFileComp
		}

		list, err := scenarios.List(scenarioPath)
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}

		// Handle comma-separated multi-select
		// Parse already selected scenarios and filter them out
		var prefix string
		var alreadySelected []string
		if idx := strings.LastIndex(toComplete, ","); idx >= 0 {
			prefix = toComplete[:idx+1]
			alreadySelected = strings.Split(toComplete[:idx], ",")
		}

		// Build set of already selected scenarios for fast lookup
		selected := make(map[string]bool)
		for _, s := range alreadySelected {
			selected[strings.TrimSpace(s)] = true
		}

		// Filter out already selected and prepend prefix
		var completions []string
		for _, s := range list {
			if !selected[s] {
				completions = append(completions, prefix+s)
			}
		}

		// Use NoSpace directive to allow continuing with comma for multi-select
		return completions, cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace
	})
}

// registerKubeContextCompletion adds tab completion for the --kube-context flag.
func registerKubeContextCompletion(cmd *cobra.Command) {
	_ = cmd.RegisterFlagCompletionFunc("kube-context", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		contexts, err := getKubeContexts()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		var completions []string
		for _, ctx := range contexts {
			if toComplete == "" || strings.HasPrefix(ctx, toComplete) {
				completions = append(completions, ctx)
			}
		}
		return completions, cobra.ShellCompDirectiveNoFileComp
	})
}

// registerPlatformCompletion adds tab completion for the --platform flag.
func registerPlatformCompletion(cmd *cobra.Command) {
	_ = cmd.RegisterFlagCompletionFunc("platform", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		platforms := []string{"gke", "eks", "rosa"}
		var completions []string
		for _, p := range platforms {
			if toComplete == "" || strings.HasPrefix(p, toComplete) {
				completions = append(completions, p)
			}
		}
		return completions, cobra.ShellCompDirectiveNoFileComp
	})
}

// registerIngressBaseDomainCompletion adds tab completion for the --ingress-base-domain flag.
func registerIngressBaseDomainCompletion(cmd *cobra.Command) {
	_ = cmd.RegisterFlagCompletionFunc("ingress-base-domain", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		var completions []string
		for _, d := range config.ValidIngressBaseDomains {
			if toComplete == "" || strings.HasPrefix(d, toComplete) {
				completions = append(completions, d)
			}
		}
		return completions, cobra.ShellCompDirectiveNoFileComp
	})
}

// getKubeContexts returns available kubectl contexts.
func getKubeContexts() ([]string, error) {
	out, err := exec.Command("kubectl", "config", "get-contexts", "-o", "name").Output()
	if err != nil {
		return nil, err
	}

	var contexts []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		ctx := strings.TrimSpace(line)
		if ctx != "" {
			contexts = append(contexts, ctx)
		}
	}
	return contexts, nil
}

// Execute runs the root command.
func Execute() error {
	rootCmd := NewRootCommand()
	rootCmd.AddCommand(newCompletionCommand(rootCmd))
	rootCmd.AddCommand(newConfigCommand())

	err := rootCmd.Execute()
	if err != nil {
		// Only show usage/help for usage errors, not runtime errors
		if isUsageError(err) {
			// Print error and then show usage
			fmt.Fprintf(os.Stderr, "Error: %v\n\n", err)
			_ = rootCmd.Usage()
		} else {
			// For runtime errors, just print the error without usage
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
	}
	return err
}
