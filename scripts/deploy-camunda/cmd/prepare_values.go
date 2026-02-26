package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"scripts/camunda-core/pkg/logging"
	"scripts/camunda-core/pkg/scenarios"
	"scripts/deploy-camunda/config"
	"scripts/deploy-camunda/deploy"
	"scripts/prepare-helm-values/pkg/env"
	"scripts/prepare-helm-values/pkg/values"

	"github.com/spf13/cobra"
)

// prepareValuesFlags holds the flags specific to the prepare-values subcommand.
// This is a lightweight subset of config.RuntimeFlags — no namespace, release, chart, or
// deployment-related fields required.
type prepareValuesFlags struct {
	scenarioPath string
	chartPath    string
	scenario     string
	identity     string
	persistence  string
	testPlatform string
	platform     string
	features     []string
	qa           bool
	imageTags    bool
	upgradeFlow  bool
	flow         string
	chartVersion string
	infraType    string
	valuesConfig string
	envFile      string
	outputDir    string
	interactive  bool
	logLevel     string
}

// newPrepareValuesCommand creates the "prepare-values" subcommand.
//
// This command resolves layered values files, performs environment variable
// substitution on each one, deep-merges them into a single YAML file, and
// prints the output path to stdout. It does NOT invoke Helm, kube client,
// vault, docker registry, or any deployment logic.
//
// Usage:
//
//	deploy-camunda prepare-values \
//	  --scenario-path /path/to/chart-full-setup \
//	  --identity keycloak-external \
//	  --persistence elasticsearch-external \
//	  --features multitenancy \
//	  --output-dir /tmp/values
func newPrepareValuesCommand() *cobra.Command {
	var pv prepareValuesFlags

	cmd := &cobra.Command{
		Use:   "prepare-values",
		Short: "Resolve, substitute, and merge layered Helm values files",
		Long: `Resolve all layered values files for a scenario, perform environment variable
substitution on each, deep-merge them into a single YAML file, and print the
output path to stdout.

This command is designed for CI and Taskfile integration — it replaces the
bash layer-resolution blocks with a single CLI call that reuses the canonical
Go layer resolution, env var substitution, and name-keyed array merge logic.

Exit code 0 on success; the ONLY line on stdout is the merged file path.
All diagnostic output goes to stderr via the logger.`,
		// Override PersistentPreRunE to skip root's heavy validation
		// (no namespace, release, chart-path, or scenario required).
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPrepareValues(&pv)
		},
	}

	f := cmd.Flags()
	f.StringVar(&pv.scenarioPath, "scenario-path", "", "Path to the scenario directory (e.g., chart-full-setup)")
	f.StringVar(&pv.chartPath, "chart-path", "", "Path to the Camunda chart directory (used to derive scenario-path if not set)")
	f.StringVar(&pv.scenario, "scenario", "chart-full-setup", "Scenario name (used to derive defaults from naming conventions)")
	f.StringVar(&pv.identity, "identity", "", "Identity selection: keycloak, keycloak-external, oidc, basic, hybrid")
	f.StringVar(&pv.persistence, "persistence", "", "Persistence selection: elasticsearch, elasticsearch-external, opensearch, rdbms, rdbms-oracle")
	f.StringVar(&pv.testPlatform, "test-platform", "", "Test platform selection: gke, eks, openshift")
	f.StringVar(&pv.platform, "platform", "gke", "Deploy platform: gke, rosa, eks (fallback for --test-platform)")
	f.StringSliceVar(&pv.features, "features", nil, "Feature selections (comma-separated): multitenancy, rba, documentstore")
	f.BoolVar(&pv.qa, "qa", false, "Enable QA configuration (test users, etc.)")
	f.BoolVar(&pv.imageTags, "image-tags", false, "Enable image tag overrides from env vars")
	f.BoolVar(&pv.upgradeFlow, "upgrade-flow", false, "Enable upgrade flow configuration")
	f.StringVar(&pv.flow, "flow", "install", "Flow type: install, upgrade-patch, upgrade-minor")
	f.StringVar(&pv.chartVersion, "chart-version", "", "Chart version (used to determine if migrator is needed)")
	f.StringVar(&pv.infraType, "infra-type", "", "Infrastructure pool type (preemptible, distroci, standard, arm)")
	f.StringVar(&pv.valuesConfig, "values-config", "", "JSON config string for env var overlay during substitution; \"{}\" or empty = skip")
	f.StringVar(&pv.envFile, "env-file", "", "Path to .env file (defaults to .env in current dir)")
	f.StringVar(&pv.outputDir, "output-dir", "", "Directory for output files (defaults to a temp dir)")
	f.BoolVar(&pv.interactive, "interactive", false, "Enable interactive prompts for missing variables")
	f.StringVarP(&pv.logLevel, "log-level", "l", "info", "Log level")

	// Register completions for selection flags
	_ = cmd.RegisterFlagCompletionFunc("identity", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"keycloak", "keycloak-external", "oidc", "basic", "hybrid"}, cobra.ShellCompDirectiveNoFileComp
	})
	_ = cmd.RegisterFlagCompletionFunc("persistence", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"elasticsearch", "elasticsearch-external", "opensearch", "rdbms", "rdbms-oracle"}, cobra.ShellCompDirectiveNoFileComp
	})
	_ = cmd.RegisterFlagCompletionFunc("test-platform", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return config.TestPlatforms, cobra.ShellCompDirectiveNoFileComp
	})
	_ = cmd.RegisterFlagCompletionFunc("features", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return completeMultiSelect(toComplete, []string{"multitenancy", "rba", "documentstore", "arm", "migrator"})
	})
	_ = cmd.RegisterFlagCompletionFunc("log-level", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return completeLogLevels(toComplete)
	})

	return cmd
}

// runPrepareValues is the main logic for the prepare-values subcommand.
func runPrepareValues(pv *prepareValuesFlags) error {
	// Setup logging — write to stderr so stdout stays clean for the merged file path.
	if err := logging.Setup(logging.Options{
		LevelString:  pv.logLevel,
		ColorEnabled: logging.IsTerminal(os.Stderr.Fd()),
		Writer:       os.Stderr,
	}); err != nil {
		return err
	}

	// Load .env file.
	envFileToLoad := pv.envFile
	if envFileToLoad == "" {
		envFileToLoad = ".env"
	}
	_ = env.Load(envFileToLoad)

	// Resolve scenario path.
	scenarioDir := pv.scenarioPath
	if scenarioDir == "" && pv.chartPath != "" {
		scenarioDir = filepath.Join(pv.chartPath, "test/integration/scenarios/chart-full-setup")
	}
	if scenarioDir == "" {
		return fmt.Errorf("either --scenario-path or --chart-path must be provided")
	}

	// Verify the scenario directory exists.
	if fi, err := os.Stat(scenarioDir); err != nil || !fi.IsDir() {
		return fmt.Errorf("scenario directory %q does not exist or is not a directory", scenarioDir)
	}

	// Check for layered values.
	if !scenarios.HasLayeredValues(scenarioDir) {
		return fmt.Errorf("no layered values found in %q (expected values/base.yaml)", scenarioDir)
	}

	// Resolve the effective platform (--test-platform takes precedence over --platform).
	effectivePlatform := pv.testPlatform
	if effectivePlatform == "" {
		effectivePlatform = pv.platform
	}

	// Build DeploymentConfig using the canonical builder.
	deployConfig := scenarios.BuildDeploymentConfig(pv.scenario, scenarios.BuilderOverrides{
		Identity:     pv.identity,
		Persistence:  pv.persistence,
		Platform:     effectivePlatform,
		Features:     pv.features,
		InfraType:    pv.infraType,
		Flow:         pv.flow,
		QA:           pv.qa,
		ImageTags:    pv.imageTags,
		Upgrade:      pv.upgradeFlow,
		ChartVersion: pv.chartVersion,
	})

	// Validate the deployment config.
	if err := deployConfig.Validate(); err != nil {
		return fmt.Errorf("deployment config validation failed: %w", err)
	}

	// Resolve all layer file paths.
	layeredFiles, err := deployConfig.ResolvePaths(scenarioDir)
	if err != nil {
		return fmt.Errorf("failed to resolve layered values: %w", err)
	}

	if len(layeredFiles) == 0 {
		return fmt.Errorf("no values files resolved for the given configuration")
	}

	// Log resolved layers at INFO level for visibility.
	shortFiles := make([]string, len(layeredFiles))
	for i, f := range layeredFiles {
		if rel, err := filepath.Rel(scenarioDir, f); err == nil {
			shortFiles[i] = rel
		} else {
			shortFiles[i] = filepath.Base(f)
		}
	}
	logging.Logger.Info().
		Str("identity", deployConfig.Identity).
		Str("persistence", deployConfig.Persistence).
		Str("platform", deployConfig.Platform).
		Strs("features", deployConfig.Features).
		Strs("layerFiles", shortFiles).
		Msg("Resolved deployment layers")

	// Create output directory.
	outputDir := pv.outputDir
	if outputDir == "" {
		outputDir, err = os.MkdirTemp("", "camunda-prepare-values-*")
		if err != nil {
			return fmt.Errorf("failed to create temp directory: %w", err)
		}
		// Do NOT clean up — the caller needs the files.
	} else {
		if err := os.MkdirAll(outputDir, 0o755); err != nil {
			return fmt.Errorf("failed to create output directory %q: %w", outputDir, err)
		}
	}

	// Process each layered file (env var substitution) into the output directory.
	var processedFiles []string
	for _, srcFile := range layeredFiles {
		opts := values.Options{
			ChartPath:    pv.chartPath,
			ScenarioDir:  scenarioDir,
			ValuesConfig: pv.valuesConfig,
			OutputDir:    outputDir,
			Interactive:  pv.interactive,
			EnvFile:      pv.envFile,
		}

		outputPath, _, procErr := values.Process(srcFile, opts)
		if procErr != nil {
			return fmt.Errorf("failed to process values file %q: %w", srcFile, procErr)
		}
		processedFiles = append(processedFiles, outputPath)
	}

	logging.Logger.Debug().
		Strs("processedFiles", processedFiles).
		Msg("All layered values files processed")

	// Deep-merge all processed files into a single YAML file.
	// This prevents Helm's shallow array replacement from silently dropping entries.
	mergedFiles, err := deploy.MergeLayeredValues(processedFiles, outputDir)
	if err != nil {
		return fmt.Errorf("failed to merge layered values: %w", err)
	}

	if len(mergedFiles) == 0 {
		return fmt.Errorf("merge produced no output files")
	}

	// Print the merged file path to stdout — this is the ONLY stdout output.
	// The caller (Taskfile / CI runner) captures this path for --values.
	fmt.Fprintln(os.Stdout, mergedFiles[0])

	return nil
}
