package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"scripts/camunda-core/pkg/logging"
	"scripts/deploy-camunda/config"
	"scripts/deploy-camunda/matrix"
	"scripts/prepare-helm-values/pkg/env"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// newMatrixCommand creates the matrix parent command with list and run subcommands.
func newMatrixCommand() *cobra.Command {
	matrixCmd := &cobra.Command{
		Use:   "matrix",
		Short: "Generate and run the CI test matrix across all active chart versions",
	}

	matrixCmd.AddCommand(newMatrixListCommand())
	matrixCmd.AddCommand(newMatrixRunCommand())

	return matrixCmd
}

// newMatrixListCommand creates the "matrix list" subcommand.
func newMatrixListCommand() *cobra.Command {
	var (
		versions        []string
		includeDisabled bool
		scenarioFilter  string
		shortnameFilter string
		flowFilter      string
		outputFormat    string
		platform        string
		repoRoot        string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List the CI test matrix for all active chart versions",
		Long: `List the full CI test matrix generated from chart-versions.yaml,
ci-test-config.yaml (PR scenarios only), and permitted-flows.yaml.

This command does not require cluster access.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Track which CLI flags were explicitly set so config merging
			// does not overwrite them.
			changedFlags := make(map[string]bool)
			cmd.Flags().Visit(func(f *pflag.Flag) {
				changedFlags[f.Name] = true
			})

			// Load config file and merge matrix/root config into local flags.
			if rc, err := config.LoadMatrixConfig(configFile); err == nil {
				config.ApplyMatrixListConfig(rc, changedFlags, &config.MatrixListFlags{
					Versions:        &versions,
					IncludeDisabled: &includeDisabled,
					ScenarioFilter:  &scenarioFilter,
					ShortnameFilter: &shortnameFilter,
					FlowFilter:      &flowFilter,
					OutputFormat:    &outputFormat,
					Platform:        &platform,
					RepoRoot:        &repoRoot,
				})
			}

			if repoRoot == "" {
				detected, err := config.DetectRepoRoot()
				if err != nil {
					return err
				}
				repoRoot = detected
			}
			if repoRoot == "" {
				return fmt.Errorf("--repo-root is required (or set repoRoot in config, or run from within the repo)")
			}

			entries, err := matrix.Generate(repoRoot, matrix.GenerateOptions{
				Versions:        versions,
				IncludeDisabled: includeDisabled,
			})
			if err != nil {
				return err
			}

			entries = matrix.Filter(entries, matrix.FilterOptions{
				ScenarioFilter:  scenarioFilter,
				ShortnameFilter: shortnameFilter,
				FlowFilter:      flowFilter,
				Platform:        platform,
			})

			output, err := matrix.Print(entries, outputFormat)
			if err != nil {
				return err
			}
			fmt.Fprintln(os.Stdout, output)
			return nil
		},
	}

	f := cmd.Flags()
	f.StringSliceVar(&versions, "versions", nil, "Limit to specific chart versions (comma-separated, e.g., 8.8,8.9)")
	f.BoolVar(&includeDisabled, "include-disabled", false, "Include disabled scenarios in the output")
	f.StringVar(&scenarioFilter, "scenario-filter", "", "Filter scenarios by substring match (comma-separated for multiple, e.g. elasticsearch,opensearch)")
	f.StringVar(&shortnameFilter, "shortname-filter", "", "Filter entries by shortname substring match (comma-separated for multiple, e.g. eske,eshy)")
	f.StringVar(&flowFilter, "flow-filter", "", "Filter entries by exact flow name")
	f.StringVar(&outputFormat, "format", "table", "Output format: table, json")
	f.StringVar(&platform, "platform", "", "Filter entries to those supporting this platform")
	f.StringVar(&repoRoot, "repo-root", "", "Repository root path (or set repoRoot in config)")

	registerMatrixShortnameCompletion(cmd)
	registerMatrixVersionsCompletion(cmd)
	registerMatrixFlowCompletion(cmd)

	return cmd
}

// newMatrixRunCommand creates the "matrix run" subcommand.
func newMatrixRunCommand() *cobra.Command {
	var (
		versions                 []string
		includeDisabled          bool
		scenarioFilter           string
		shortnameFilter          string
		flowFilter               string
		platform                 string
		repoRoot                 string
		dryRun                   bool
		coverage                 bool
		testIT                   bool
		testE2E                  bool
		testAll                  bool
		stopOnFailure            bool
		namespacePrefix          string
		cleanup                  bool
		deleteNamespace          bool
		kubeContext              string
		kubeContextGKE           string
		kubeContextEKS           string
		ingressBaseDomain        string
		ingressBaseDomainGKE     string
		ingressBaseDomainEKS     string
		maxParallel              int
		envFile                  string
		envFile86                string
		envFile87                string
		envFile88                string
		envFile89                string
		logLevel                 string
		skipDependencyUpdate     bool
		useVaultBackedSecrets    bool
		useVaultBackedSecretsGKE bool
		useVaultBackedSecretsEKS bool
		keycloakHost             string
		keycloakProtocol         string
		upgradeFromVersion       string
		helmTimeout              int
		dockerUsername           string
		dockerPassword           string
		ensureDockerRegistry     bool
		dockerHubUsername        string
		dockerHubPassword        string
		ensureDockerHub          bool
		useLatest                bool
		useQA                    bool
		yes                      bool
	)

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run the CI test matrix against a live cluster",
		Long: `Run the full CI test matrix, deploying each scenario + flow combination sequentially.
Each entry gets its own namespace (<prefix>-<version>-<shortname>).

Use --cleanup to automatically delete each entry's namespace after its deployment and tests complete.

This command calls deploy.Execute() for each matrix entry.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Create a signal-aware context so that Ctrl+C (SIGINT) and
			// SIGTERM cancel the context, which propagates through
			// matrix.Run → deploy.Execute → executeScript, cleanly
			// terminating the entire subprocess tree.
			ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			// Track which CLI flags were explicitly set so config merging
			// does not overwrite them.
			changedFlags := make(map[string]bool)
			cmd.Flags().Visit(func(f *pflag.Flag) {
				changedFlags[f.Name] = true
			})

			// Build per-platform/per-version maps from CLI flags BEFORE config
			// merging, so that CLI-provided map entries take precedence.
			kubeContexts := make(map[string]string)
			if kubeContextGKE != "" {
				kubeContexts["gke"] = kubeContextGKE
			}
			if kubeContextEKS != "" {
				kubeContexts["eks"] = kubeContextEKS
			}

			envFiles := make(map[string]string)
			for version, path := range map[string]string{
				"8.6": envFile86,
				"8.7": envFile87,
				"8.8": envFile88,
				"8.9": envFile89,
			} {
				if path != "" {
					envFiles[version] = path
				}
			}

			vaultBackedSecrets := make(map[string]bool)
			if cmd.Flags().Changed("use-vault-backed-secrets-gke") {
				vaultBackedSecrets["gke"] = useVaultBackedSecretsGKE
			}
			if cmd.Flags().Changed("use-vault-backed-secrets-eks") {
				vaultBackedSecrets["eks"] = useVaultBackedSecretsEKS
			}

			ingressBaseDomains := make(map[string]string)
			if ingressBaseDomainGKE != "" {
				ingressBaseDomains["gke"] = ingressBaseDomainGKE
			}
			if ingressBaseDomainEKS != "" {
				ingressBaseDomains["eks"] = ingressBaseDomainEKS
			}

			// Load config file and merge matrix/root config into local flags.
			// Config values fill in anything not explicitly set on the CLI.
			if rc, err := config.LoadMatrixConfig(configFile); err == nil {
				config.ApplyMatrixRunConfig(rc, changedFlags, &config.MatrixRunFlags{
					// Filtering & generation
					Versions:        &versions,
					IncludeDisabled: &includeDisabled,
					ScenarioFilter:  &scenarioFilter,
					ShortnameFilter: &shortnameFilter,
					FlowFilter:      &flowFilter,
					Platform:        &platform,
					RepoRoot:        &repoRoot,
					// Execution
					DryRun:               &dryRun,
					Coverage:             &coverage,
					StopOnFailure:        &stopOnFailure,
					Cleanup:              &cleanup,
					DeleteNamespace:      &deleteNamespace,
					NamespacePrefix:      &namespacePrefix,
					MaxParallel:          &maxParallel,
					LogLevel:             &logLevel,
					SkipDependencyUpdate: &skipDependencyUpdate,
					HelmTimeout:          &helmTimeout,
					// Tests
					TestIT:  &testIT,
					TestE2E: &testE2E,
					TestAll: &testAll,
					// Kube contexts
					KubeContext:    &kubeContext,
					KubeContextGKE: &kubeContextGKE,
					KubeContextEKS: &kubeContextEKS,
					KubeContexts:   kubeContexts,
					// Ingress
					IngressBaseDomain:    &ingressBaseDomain,
					IngressBaseDomainGKE: &ingressBaseDomainGKE,
					IngressBaseDomainEKS: &ingressBaseDomainEKS,
					IngressBaseDomains:   ingressBaseDomains,
					// Vault
					UseVaultBackedSecrets:    &useVaultBackedSecrets,
					UseVaultBackedSecretsGKE: &useVaultBackedSecretsGKE,
					UseVaultBackedSecretsEKS: &useVaultBackedSecretsEKS,
					VaultBackedSecrets:       vaultBackedSecrets,
					// Env files
					EnvFile:   &envFile,
					EnvFile86: &envFile86,
					EnvFile87: &envFile87,
					EnvFile88: &envFile88,
					EnvFile89: &envFile89,
					EnvFiles:  envFiles,
					// Docker
					DockerUsername:       &dockerUsername,
					DockerPassword:       &dockerPassword,
					EnsureDockerRegistry: &ensureDockerRegistry,
					DockerHubUsername:    &dockerHubUsername,
					DockerHubPassword:    &dockerHubPassword,
					EnsureDockerHub:      &ensureDockerHub,
					// Keycloak
					KeycloakHost:     &keycloakHost,
					KeycloakProtocol: &keycloakProtocol,
					// Upgrade
					UpgradeFromVersion: &upgradeFromVersion,
				})
			}

			// Setup logging (after config merge so log-level from config takes effect)
			if err := logging.Setup(logging.Options{
				LevelString:  logLevel,
				ColorEnabled: logging.IsTerminal(os.Stdout.Fd()),
			}); err != nil {
				return err
			}

			// Load .env file — use flag/config value if set, otherwise default to .env.
			envFileToLoad := envFile
			if envFileToLoad == "" {
				envFileToLoad = ".env"
			}
			logging.Logger.Debug().
				Str("envFile", envFileToLoad).
				Msg("Loading environment file")
			if err := env.Load(envFileToLoad); err != nil {
				logging.Logger.Warn().Err(err).Str("envFile", envFileToLoad).Msg("Failed to load environment file")
			}

			if repoRoot == "" {
				detected, err := config.DetectRepoRoot()
				if err != nil {
					return err
				}
				repoRoot = detected
			}
			if repoRoot == "" {
				return fmt.Errorf("--repo-root is required (or set repoRoot in config, or run from within the repo)")
			}

			// Validate ingress base domains early so the user gets immediate feedback.
			if ingressBaseDomain != "" {
				if !config.IsValidIngressBaseDomain(ingressBaseDomain) {
					return fmt.Errorf("--ingress-base-domain must be one of: %s", strings.Join(config.ValidIngressBaseDomains, ", "))
				}
			}
			if ingressBaseDomainGKE != "" {
				if !config.IsValidIngressBaseDomain(ingressBaseDomainGKE) {
					return fmt.Errorf("--ingress-base-domain-gke must be one of: %s", strings.Join(config.ValidIngressBaseDomains, ", "))
				}
			}
			if ingressBaseDomainEKS != "" {
				if !config.IsValidIngressBaseDomain(ingressBaseDomainEKS) {
					return fmt.Errorf("--ingress-base-domain-eks must be one of: %s", strings.Join(config.ValidIngressBaseDomains, ", "))
				}
			}

			entries, err := matrix.Generate(repoRoot, matrix.GenerateOptions{
				Versions:        versions,
				IncludeDisabled: includeDisabled,
			})
			if err != nil {
				return err
			}

			entries = matrix.Filter(entries, matrix.FilterOptions{
				ScenarioFilter:  scenarioFilter,
				ShortnameFilter: shortnameFilter,
				FlowFilter:      flowFilter,
				Platform:        platform,
			})

			if len(entries) == 0 {
				fmt.Fprintln(os.Stdout, "No matrix entries matched the filters.")
				return nil
			}

			// Block e2e runs with many entries — Playwright spawns a browser per test
			// which can exhaust machine resources fast.
			const e2eWarnThreshold = 5
			if (testE2E || testAll) && len(entries) > e2eWarnThreshold && !yes {
				logging.Logger.Warn().
					Int("entries", len(entries)).
					Int("threshold", e2eWarnThreshold).
					Msg("Running e2e tests on many entries — Playwright spawns a browser per test which can exhaust machine resources. Consider using --scenario-filter or --shortname-filter to reduce the set.")
				fmt.Fprintf(os.Stderr, "\nProceed with e2e tests on %d entries? [y/N] ", len(entries))
				reader := bufio.NewReader(os.Stdin)
				answer, _ := reader.ReadString('\n')
				answer = strings.TrimSpace(strings.ToLower(answer))
				if answer != "y" && answer != "yes" {
					return fmt.Errorf("aborted: e2e run with %d entries not confirmed (use --yes to skip this prompt)", len(entries))
				}
			}

			// Show what will be run (only for non-dry-run/non-coverage; those modes print their own detailed output)
			if !dryRun && !coverage {
				output, _ := matrix.Print(entries, "table")
				fmt.Fprintln(os.Stdout, output)
			}

			runStart := time.Now()
			results, err := matrix.Run(ctx, entries, matrix.RunOptions{
				DryRun:                dryRun,
				Coverage:              coverage,
				StopOnFailure:         stopOnFailure,
				Cleanup:               cleanup,
				DeleteNamespaceFirst:  deleteNamespace,
				KubeContexts:          kubeContexts,
				KubeContext:           kubeContext,
				NamespacePrefix:       namespacePrefix,
				Platform:              platform,
				MaxParallel:           maxParallel,
				TestIT:                testIT,
				TestE2E:               testE2E,
				TestAll:               testAll,
				RepoRoot:              repoRoot,
				EnvFiles:              envFiles,
				EnvFile:               envFile,
				IngressBaseDomains:    ingressBaseDomains,
				IngressBaseDomain:     ingressBaseDomain,
				LogLevel:              logLevel,
				SkipDependencyUpdate:  skipDependencyUpdate,
				VaultBackedSecrets:    vaultBackedSecrets,
				UseVaultBackedSecrets: useVaultBackedSecrets,
				KeycloakHost:          keycloakHost,
				KeycloakProtocol:      keycloakProtocol,
				UpgradeFromVersion:    upgradeFromVersion,
				HelmTimeout:           helmTimeout,
				DockerUsername:        dockerUsername,
				DockerPassword:        dockerPassword,
				EnsureDockerRegistry:  ensureDockerRegistry,
				DockerHubUsername:     dockerHubUsername,
				DockerHubPassword:     dockerHubPassword,
				EnsureDockerHub:       ensureDockerHub,
				UseLatest:             useLatest,
				UseQA:                 useQA,
			})

			// Print summary (skip for dry-run/coverage since they print their own output)
			if !dryRun && !coverage {
				fmt.Fprintln(os.Stdout, matrix.PrintRunSummary(results, time.Since(runStart)))
			}

			return err
		},
	}

	f := cmd.Flags()
	f.StringSliceVar(&versions, "versions", nil, "Limit to specific chart versions (comma-separated, e.g., 8.8,8.9)")
	f.BoolVar(&includeDisabled, "include-disabled", false, "Include disabled scenarios in the output")
	f.StringVar(&scenarioFilter, "scenario-filter", "", "Filter scenarios by substring match (comma-separated for multiple, e.g. elasticsearch,opensearch)")
	f.StringVar(&shortnameFilter, "shortname-filter", "", "Filter entries by shortname substring match (comma-separated for multiple, e.g. eske,eshy)")
	f.StringVar(&flowFilter, "flow-filter", "", "Filter entries by exact flow name")
	f.StringVar(&platform, "platform", "", "Filter entries to those supporting this platform (also sets deploy platform)")
	f.StringVar(&repoRoot, "repo-root", "", "Repository root path (or set repoRoot in config)")
	f.BoolVar(&dryRun, "dry-run", false, "Log what would be deployed without actually deploying")
	f.BoolVar(&coverage, "coverage", false, "Show a layer-breakdown report of what is tested in the matrix (no deployment)")
	f.BoolVar(&testIT, "test-it", false, "Run integration tests after each deployment")
	f.BoolVar(&testE2E, "test-e2e", false, "Run e2e tests after each deployment")
	f.BoolVar(&testAll, "test-all", false, "Run both integration and e2e tests after each deployment")
	f.BoolVar(&stopOnFailure, "stop-on-failure", false, "Stop the run on the first failure")
	f.StringVar(&namespacePrefix, "namespace-prefix", "matrix", "Prefix for generated namespaces")
	f.BoolVar(&cleanup, "cleanup", false, "Delete each entry's namespace after its deployment and tests complete")
	f.BoolVar(&deleteNamespace, "delete-namespace", false, "Delete the namespace before deploying each entry (clean-slate deployment)")
	f.StringVar(&kubeContext, "kube-context", "", "Default Kubernetes context for all platforms (overridden by --kube-context-gke/--kube-context-eks)")
	f.StringVar(&kubeContextGKE, "kube-context-gke", "", "Kubernetes context for GKE entries")
	f.StringVar(&kubeContextEKS, "kube-context-eks", "", "Kubernetes context for EKS entries")
	f.StringVar(&ingressBaseDomain, "ingress-base-domain", "", "Fallback base domain for ingress hosts (overridden by --ingress-base-domain-gke/--ingress-base-domain-eks)")
	f.StringVar(&ingressBaseDomainGKE, "ingress-base-domain-gke", "", "Ingress base domain for GKE entries (e.g., ci.distro.ultrawombat.com)")
	f.StringVar(&ingressBaseDomainEKS, "ingress-base-domain-eks", "", "Ingress base domain for EKS entries (e.g., distribution.aws.camunda.cloud)")
	f.IntVar(&maxParallel, "max-parallel", 1, "Maximum number of entries to run concurrently (1 = sequential)")
	f.StringVar(&envFile, "env-file", "", "Default .env file for all versions (overridden by --env-file-X.Y)")
	f.StringVar(&envFile86, "env-file-8.6", "", "Path to .env file for 8.6 entries")
	f.StringVar(&envFile87, "env-file-8.7", "", "Path to .env file for 8.7 entries")
	f.StringVar(&envFile88, "env-file-8.8", "", "Path to .env file for 8.8 entries")
	f.StringVar(&envFile89, "env-file-8.9", "", "Path to .env file for 8.9 entries")
	f.StringVarP(&logLevel, "log-level", "l", "info", "Log level (debug, info, warn, error)")
	f.BoolVar(&skipDependencyUpdate, "skip-dependency-update", false, "Skip helm dependency update before deploying")
	f.BoolVar(&useVaultBackedSecrets, "use-vault-backed-secrets", false, "Use vault-backed external secrets for all platforms (overridden by --use-vault-backed-secrets-gke/--use-vault-backed-secrets-eks)")
	f.BoolVar(&useVaultBackedSecretsGKE, "use-vault-backed-secrets-gke", false, "Use vault-backed external secrets for GKE entries")
	f.BoolVar(&useVaultBackedSecretsEKS, "use-vault-backed-secrets-eks", false, "Use vault-backed external secrets for EKS entries")
	f.StringVar(&keycloakHost, "keycloak-host", "", "Keycloak external host (defaults to "+config.DefaultKeycloakHost+")")
	f.StringVar(&keycloakProtocol, "keycloak-protocol", "", "Keycloak protocol (defaults to "+config.DefaultKeycloakProtocol+")")
	f.StringVar(&upgradeFromVersion, "upgrade-from-version", "", "Override the auto-resolved 'from' chart version for upgrade flows (e.g., 13.5.0)")
	f.IntVar(&helmTimeout, "timeout", 10, "Timeout in minutes for Helm deployment (applies to all entries)")
	f.StringVar(&dockerUsername, "docker-username", "", "Harbor registry username (defaults to HARBOR_USERNAME, TEST_DOCKER_USERNAME_CAMUNDA_CLOUD, or NEXUS_USERNAME env var)")
	f.StringVar(&dockerPassword, "docker-password", "", "Harbor registry password (defaults to HARBOR_PASSWORD, TEST_DOCKER_PASSWORD_CAMUNDA_CLOUD, or NEXUS_PASSWORD env var)")
	f.BoolVar(&ensureDockerRegistry, "ensure-docker-registry", false, "Ensure Harbor registry pull secret is created in each entry's namespace")
	f.StringVar(&dockerHubUsername, "dockerhub-username", "", "Docker Hub registry username (defaults to DOCKERHUB_USERNAME or TEST_DOCKER_USERNAME env var)")
	f.StringVar(&dockerHubPassword, "dockerhub-password", "", "Docker Hub registry password (defaults to DOCKERHUB_PASSWORD or TEST_DOCKER_PASSWORD env var)")
	f.BoolVar(&ensureDockerHub, "ensure-docker-hub", false, "Ensure Docker Hub registry pull secret is created in each entry's namespace")
	f.BoolVar(&useLatest, "use-latest", false, "Use values-latest.yaml from each chart root instead of values-digest.yaml")
	f.BoolVar(&useQA, "use-qa", false, "Force the base-qa layer to be included for all entries, regardless of per-scenario qa config")
	f.BoolVarP(&yes, "yes", "y", false, "Skip confirmation prompts (e.g., e2e threshold warning)")

	registerMatrixShortnameCompletion(cmd)
	registerMatrixVersionsCompletion(cmd)
	registerMatrixFlowCompletion(cmd)
	registerIngressBaseDomainCompletion(cmd)
	registerIngressBaseDomainCompletionForFlag(cmd, "ingress-base-domain-gke")
	registerIngressBaseDomainCompletionForFlag(cmd, "ingress-base-domain-eks")
	registerKubeContextCompletion(cmd)
	registerKubeContextCompletionForFlag(cmd, "kube-context-gke")
	registerKubeContextCompletionForFlag(cmd, "kube-context-eks")
	_ = cmd.RegisterFlagCompletionFunc("log-level", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return completeLogLevels(toComplete)
	})

	return cmd
}

// registerMatrixShortnameCompletion adds tab completion for the --shortname-filter flag.
// It generates the matrix from config files and offers unique shortnames, supporting
// comma-separated multi-select.
func registerMatrixShortnameCompletion(cmd *cobra.Command) {
	_ = cmd.RegisterFlagCompletionFunc("shortname-filter", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		repoRoot, _ := cmd.Flags().GetString("repo-root")
		repoRoot = resolveRepoRoot(repoRoot)
		if repoRoot == "" {
			return cobra.AppendActiveHelp(nil, "Please specify --repo-root or configure repoRoot in your deployment config"), cobra.ShellCompDirectiveNoFileComp
		}

		entries, err := matrix.Generate(repoRoot, matrix.GenerateOptions{})
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}

		// Collect unique shortnames in order of appearance.
		seen := make(map[string]bool)
		var shortnames []string
		for _, e := range entries {
			if e.Shortname != "" && !seen[e.Shortname] {
				seen[e.Shortname] = true
				shortnames = append(shortnames, e.Shortname)
			}
		}

		return completeMultiSelect(toComplete, shortnames)
	})
}

// registerMatrixVersionsCompletion adds tab completion for the --versions flag.
// It reads chart-versions.yaml and offers active versions (alpha + supportStandard).
func registerMatrixVersionsCompletion(cmd *cobra.Command) {
	_ = cmd.RegisterFlagCompletionFunc("versions", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		repoRoot, _ := cmd.Flags().GetString("repo-root")
		repoRoot = resolveRepoRoot(repoRoot)
		if repoRoot == "" {
			return cobra.AppendActiveHelp(nil, "Please specify --repo-root or configure repoRoot in your deployment config"), cobra.ShellCompDirectiveNoFileComp
		}

		cv, err := matrix.LoadChartVersions(repoRoot)
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}

		return cv.ActiveVersions(), cobra.ShellCompDirectiveNoFileComp
	})
}

// registerMatrixFlowCompletion adds tab completion for the --flow-filter flag.
// It reads permitted-flows.yaml and offers the default flows list.
func registerMatrixFlowCompletion(cmd *cobra.Command) {
	_ = cmd.RegisterFlagCompletionFunc("flow-filter", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		repoRoot, _ := cmd.Flags().GetString("repo-root")
		repoRoot = resolveRepoRoot(repoRoot)
		if repoRoot == "" {
			return cobra.AppendActiveHelp(nil, "Please specify --repo-root or configure repoRoot in your deployment config"), cobra.ShellCompDirectiveNoFileComp
		}

		pf, err := matrix.LoadPermittedFlows(repoRoot)
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}

		return pf.Defaults.Flows, cobra.ShellCompDirectiveNoFileComp
	})
}

// registerKubeContextCompletionForFlag adds tab completion for a named kube-context flag.
func registerKubeContextCompletionForFlag(cmd *cobra.Command, flagName string) {
	_ = cmd.RegisterFlagCompletionFunc(flagName, func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		contexts, err := getKubeContexts()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return filterByPrefix(contexts, toComplete), cobra.ShellCompDirectiveNoFileComp
	})
}

// resolveRepoRoot resolves the repository root from the flag, config file,
// or auto-detection via git.
func resolveRepoRoot(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}

	// Try to resolve from config file
	var tempFlags config.RuntimeFlags
	if _, _, err := config.LoadAndMerge(configFile, false, &tempFlags); err == nil {
		if tempFlags.Chart.RepoRoot != "" {
			return tempFlags.Chart.RepoRoot
		}
	}

	// Fall back to auto-detection from CWD (errors silently swallowed —
	// completion should never crash the shell).
	if detected, _ := config.DetectRepoRoot(); detected != "" {
		return detected
	}

	return ""
}
