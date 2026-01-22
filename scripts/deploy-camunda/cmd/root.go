package cmd

import (
	"context"
	"fmt"
	"os"
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
			if _, err := config.LoadAndMerge(configFile, true, &flags); err != nil {
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

	// Selection + composition model (new - preferred over deprecated --scenario)
	f.StringVar(&flags.Identity, "identity", "", "Identity selection: keycloak, keycloak-external, oidc, basic, hybrid")
	f.StringVar(&flags.Persistence, "persistence", "", "Persistence selection: elasticsearch, opensearch, rdbms, rdbms-oracle")
	f.StringVar(&flags.TestPlatform, "test-platform", "", "Test platform selection: gke, eks, openshift")
	f.StringSliceVar(&flags.Features, "features", nil, "Feature selections (comma-separated): multitenancy, rba, documentstore")
	f.BoolVar(&flags.QA, "qa", false, "Enable QA configuration (test users, etc.)")
	f.BoolVar(&flags.ImageTags, "image-tags", false, "Enable image tag overrides from env vars")
	f.BoolVar(&flags.UpgradeFlow, "upgrade-flow", false, "Enable upgrade flow configuration")

	// Deprecated layered values flags (kept for backward compatibility)
	f.StringVar(&flags.ValuesAuth, "values-auth", "", "DEPRECATED: use --identity instead")
	f.StringVar(&flags.ValuesBackend, "values-backend", "", "DEPRECATED: use --persistence instead")
	f.StringSliceVar(&flags.ValuesFeatures, "values-features", nil, "DEPRECATED: use --features instead")
	f.BoolVar(&flags.ValuesQA, "values-qa", false, "DEPRECATED: use --qa instead")
	f.StringVar(&flags.ValuesInfra, "values-infra", "", "DEPRECATED: use --test-platform instead")

	// Mark deprecated flags as hidden (they still work but won't show in help)
	_ = f.MarkHidden("values-auth")
	_ = f.MarkHidden("values-backend")
	_ = f.MarkHidden("values-features")
	_ = f.MarkHidden("values-qa")
	_ = f.MarkHidden("values-infra")

	// Register completions using config-aware completion function
	registerScenarioCompletion(rootCmd, "scenario")
	registerScenarioCompletion(rootCmd, "auth")
	registerSelectionCompletion(rootCmd)
	registerLayeredValuesCompletion(rootCmd)

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

// registerSelectionCompletion adds tab completion for the new selection + composition flags.
func registerSelectionCompletion(cmd *cobra.Command) {
	// Identity completion
	_ = cmd.RegisterFlagCompletionFunc("identity", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		scenarioPath := resolveScenarioPath(cmd)
		defaultIdentities := []string{"keycloak", "keycloak-external", "oidc", "basic", "hybrid"}

		if scenarioPath == "" {
			return defaultIdentities, cobra.ShellCompDirectiveNoFileComp
		}

		identities, err := scenarios.ListIdentities(scenarioPath)
		if err != nil || len(identities) == 0 {
			return defaultIdentities, cobra.ShellCompDirectiveNoFileComp
		}
		return identities, cobra.ShellCompDirectiveNoFileComp
	})

	// Persistence completion
	_ = cmd.RegisterFlagCompletionFunc("persistence", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		scenarioPath := resolveScenarioPath(cmd)
		defaultPersistence := []string{"elasticsearch", "opensearch", "rdbms", "rdbms-oracle"}

		if scenarioPath == "" {
			return defaultPersistence, cobra.ShellCompDirectiveNoFileComp
		}

		persistence, err := scenarios.ListPersistence(scenarioPath)
		if err != nil || len(persistence) == 0 {
			return defaultPersistence, cobra.ShellCompDirectiveNoFileComp
		}
		return persistence, cobra.ShellCompDirectiveNoFileComp
	})

	// Test platform completion
	_ = cmd.RegisterFlagCompletionFunc("test-platform", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		scenarioPath := resolveScenarioPath(cmd)
		defaultPlatforms := []string{"gke", "eks", "openshift"}

		if scenarioPath == "" {
			return defaultPlatforms, cobra.ShellCompDirectiveNoFileComp
		}

		platforms, err := scenarios.ListPlatforms(scenarioPath)
		if err != nil || len(platforms) == 0 {
			return defaultPlatforms, cobra.ShellCompDirectiveNoFileComp
		}
		return platforms, cobra.ShellCompDirectiveNoFileComp
	})

	// Features completion (supports comma-separated multi-select)
	_ = cmd.RegisterFlagCompletionFunc("features", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		scenarioPath := resolveScenarioPath(cmd)
		defaultFeatures := []string{"multitenancy", "rba", "documentstore"}

		var features []string
		if scenarioPath != "" {
			var err error
			features, err = scenarios.ListFeatures(scenarioPath)
			if err != nil || len(features) == 0 {
				features = defaultFeatures
			}
		} else {
			features = defaultFeatures
		}

		// Handle comma-separated multi-select
		var prefix string
		var alreadySelected []string
		if idx := strings.LastIndex(toComplete, ","); idx >= 0 {
			prefix = toComplete[:idx+1]
			alreadySelected = strings.Split(toComplete[:idx], ",")
		}

		selected := make(map[string]bool)
		for _, s := range alreadySelected {
			selected[strings.TrimSpace(s)] = true
		}

		var completions []string
		for _, f := range features {
			if !selected[f] {
				completions = append(completions, prefix+f)
			}
		}

		return completions, cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace
	})
}

// registerLayeredValuesCompletion adds tab completion for layered values flags.
func registerLayeredValuesCompletion(cmd *cobra.Command) {
	// Auth types completion
	_ = cmd.RegisterFlagCompletionFunc("values-auth", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		scenarioPath := resolveScenarioPath(cmd)
		if scenarioPath == "" {
			// Return default auth types
			return []string{"keycloak", "keycloak-external", "oidc", "basic", "hybrid"}, cobra.ShellCompDirectiveNoFileComp
		}

		authTypes, err := scenarios.ListLayeredAuthTypes(scenarioPath)
		if err != nil || len(authTypes) == 0 {
			return []string{"keycloak", "keycloak-external", "oidc", "basic", "hybrid"}, cobra.ShellCompDirectiveNoFileComp
		}
		return authTypes, cobra.ShellCompDirectiveNoFileComp
	})

	// Backend types completion
	_ = cmd.RegisterFlagCompletionFunc("values-backend", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		scenarioPath := resolveScenarioPath(cmd)
		if scenarioPath == "" {
			return []string{"elasticsearch", "opensearch"}, cobra.ShellCompDirectiveNoFileComp
		}

		backends, err := scenarios.ListLayeredBackends(scenarioPath)
		if err != nil || len(backends) == 0 {
			return []string{"elasticsearch", "opensearch"}, cobra.ShellCompDirectiveNoFileComp
		}
		return backends, cobra.ShellCompDirectiveNoFileComp
	})

	// Feature types completion
	_ = cmd.RegisterFlagCompletionFunc("values-features", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		scenarioPath := resolveScenarioPath(cmd)
		defaultFeatures := []string{"multitenancy", "rba", "documentstore", "rdbms", "rdbms-oracle", "upgrade"}

		if scenarioPath == "" {
			return defaultFeatures, cobra.ShellCompDirectiveNoFileComp
		}

		features, err := scenarios.ListLayeredFeatures(scenarioPath)
		if err != nil || len(features) == 0 {
			return defaultFeatures, cobra.ShellCompDirectiveNoFileComp
		}

		// Handle comma-separated multi-select
		var prefix string
		var alreadySelected []string
		if idx := strings.LastIndex(toComplete, ","); idx >= 0 {
			prefix = toComplete[:idx+1]
			alreadySelected = strings.Split(toComplete[:idx], ",")
		}

		selected := make(map[string]bool)
		for _, s := range alreadySelected {
			selected[strings.TrimSpace(s)] = true
		}

		var completions []string
		for _, f := range features {
			if !selected[f] {
				completions = append(completions, prefix+f)
			}
		}

		return completions, cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace
	})

	// Infra types completion
	_ = cmd.RegisterFlagCompletionFunc("values-infra", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"eks"}, cobra.ShellCompDirectiveNoFileComp
	})
}

// resolveScenarioPath resolves the scenario path from CLI flags or config.
func resolveScenarioPath(cmd *cobra.Command) string {
	scenarioPath, _ := cmd.Flags().GetString("scenario-path")
	if scenarioPath == "" {
		var tempFlags config.RuntimeFlags
		tempFlags.ScenarioPath, _ = cmd.Flags().GetString("scenario-path")
		tempFlags.ChartPath, _ = cmd.Flags().GetString("chart-path")

		if _, err := config.LoadAndMerge(configFile, false, &tempFlags); err == nil {
			scenarioPath = tempFlags.ScenarioPath
		}
	}
	return scenarioPath
}

// Execute runs the root command.
func Execute() error {
	rootCmd := NewRootCommand()
	rootCmd.AddCommand(newCompletionCommand(rootCmd))
	rootCmd.AddCommand(newConfigCommand())
	return rootCmd.Execute()
}
