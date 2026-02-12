package cmd

import (
	"context"
	"fmt"
	"os"
	"scripts/camunda-core/pkg/logging"
	"scripts/deploy-camunda/config"
	"scripts/deploy-camunda/matrix"
	"scripts/prepare-helm-values/pkg/env"
	"strings"

	"github.com/spf13/cobra"
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
			repoRoot = resolveRepoRoot(repoRoot)
			if repoRoot == "" {
				return fmt.Errorf("--repo-root is required (or set repoRoot in config)")
			}

			entries, err := matrix.Generate(repoRoot, matrix.GenerateOptions{
				Versions:        versions,
				IncludeDisabled: includeDisabled,
			})
			if err != nil {
				return err
			}

			entries = matrix.Filter(entries, matrix.FilterOptions{
				ScenarioFilter: scenarioFilter,
				FlowFilter:     flowFilter,
				Platform:       platform,
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
	f.StringVar(&scenarioFilter, "scenario-filter", "", "Filter scenarios by substring match")
	f.StringVar(&flowFilter, "flow-filter", "", "Filter entries by exact flow name")
	f.StringVar(&outputFormat, "format", "table", "Output format: table, json")
	f.StringVar(&platform, "platform", "", "Filter entries to those supporting this platform")
	f.StringVar(&repoRoot, "repo-root", "", "Repository root path (or set repoRoot in config)")

	return cmd
}

// newMatrixRunCommand creates the "matrix run" subcommand.
func newMatrixRunCommand() *cobra.Command {
	var (
		versions          []string
		includeDisabled   bool
		scenarioFilter    string
		flowFilter        string
		platform          string
		repoRoot          string
		dryRun            bool
		testIT            bool
		testE2E           bool
		testAll           bool
		stopOnFailure     bool
		namespacePrefix   string
		cleanup           bool
		kubeContext       string
		kubeContextGKE    string
		kubeContextEKS    string
		ingressBaseDomain string
		maxParallel       int
		envFile           string
		envFile86         string
		envFile87         string
		envFile88         string
		envFile89         string
		logLevel          string
	)

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run the CI test matrix against a live cluster",
		Long: `Run the full CI test matrix, deploying each scenario + flow combination sequentially.
Each entry gets its own namespace (<prefix>-<version>-<shortname>).

Use --cleanup to automatically delete all created namespaces after the run finishes.
Cleanup runs regardless of whether entries succeeded or failed.

This command calls deploy.Execute() for each matrix entry.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Setup logging
			if err := logging.Setup(logging.Options{
				LevelString:  logLevel,
				ColorEnabled: logging.IsTerminal(os.Stdout.Fd()),
			}); err != nil {
				return err
			}

			// Load .env file — use flag value if set, otherwise default to .env.
			// This loads the fallback env file for vars shared across all versions.
			envFileToLoad := envFile
			if envFileToLoad == "" {
				envFileToLoad = ".env"
			}
			logging.Logger.Debug().
				Str("envFile", envFileToLoad).
				Msg("Loading environment file")
			_ = env.Load(envFileToLoad)

			repoRoot = resolveRepoRoot(repoRoot)
			if repoRoot == "" {
				return fmt.Errorf("--repo-root is required (or set repoRoot in config)")
			}

			// Validate ingress base domain early so the user gets immediate feedback.
			if ingressBaseDomain != "" {
				valid := false
				for _, d := range config.ValidIngressBaseDomains {
					if d == ingressBaseDomain {
						valid = true
						break
					}
				}
				if !valid {
					return fmt.Errorf("--ingress-base-domain must be one of: %s", strings.Join(config.ValidIngressBaseDomains, ", "))
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
				ScenarioFilter: scenarioFilter,
				FlowFilter:     flowFilter,
				Platform:       platform,
			})

			if len(entries) == 0 {
				fmt.Fprintln(os.Stdout, "No matrix entries matched the filters.")
				return nil
			}

			// Show what will be run
			output, _ := matrix.Print(entries, "table")
			fmt.Fprintln(os.Stdout, output)

			// Build platform-to-context map from per-platform flags
			kubeContexts := make(map[string]string)
			if kubeContextGKE != "" {
				kubeContexts["gke"] = kubeContextGKE
			}
			if kubeContextEKS != "" {
				kubeContexts["eks"] = kubeContextEKS
			}

			// Build version-to-env-file map from per-version flags
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

			results, err := matrix.Run(context.Background(), entries, matrix.RunOptions{
				DryRun:            dryRun,
				StopOnFailure:     stopOnFailure,
				Cleanup:           cleanup,
				KubeContexts:      kubeContexts,
				KubeContext:       kubeContext,
				NamespacePrefix:   namespacePrefix,
				Platform:          platform,
				MaxParallel:       maxParallel,
				TestIT:            testIT,
				TestE2E:           testE2E,
				TestAll:           testAll,
				RepoRoot:          repoRoot,
				EnvFiles:          envFiles,
				EnvFile:           envFile,
				IngressBaseDomain: ingressBaseDomain,
				LogLevel:          logLevel,
			})

			fmt.Fprintln(os.Stdout, matrix.PrintRunSummary(results))

			return err
		},
	}

	f := cmd.Flags()
	f.StringSliceVar(&versions, "versions", nil, "Limit to specific chart versions (comma-separated, e.g., 8.8,8.9)")
	f.BoolVar(&includeDisabled, "include-disabled", false, "Include disabled scenarios in the output")
	f.StringVar(&scenarioFilter, "scenario-filter", "", "Filter scenarios by substring match")
	f.StringVar(&flowFilter, "flow-filter", "", "Filter entries by exact flow name")
	f.StringVar(&platform, "platform", "", "Filter entries to those supporting this platform (also sets deploy platform)")
	f.StringVar(&repoRoot, "repo-root", "", "Repository root path (or set repoRoot in config)")
	f.BoolVar(&dryRun, "dry-run", false, "Log what would be deployed without actually deploying")
	f.BoolVar(&testIT, "test-it", false, "Run integration tests after each deployment")
	f.BoolVar(&testE2E, "test-e2e", false, "Run e2e tests after each deployment")
	f.BoolVar(&testAll, "test-all", false, "Run both integration and e2e tests after each deployment")
	f.BoolVar(&stopOnFailure, "stop-on-failure", false, "Stop the run on the first failure")
	f.StringVar(&namespacePrefix, "namespace-prefix", "matrix", "Prefix for generated namespaces")
	f.BoolVar(&cleanup, "cleanup", false, "Delete all created namespaces after the run completes")
	f.StringVar(&kubeContext, "kube-context", "", "Default Kubernetes context for all platforms (overridden by --kube-context-gke/--kube-context-eks)")
	f.StringVar(&kubeContextGKE, "kube-context-gke", "", "Kubernetes context for GKE entries")
	f.StringVar(&kubeContextEKS, "kube-context-eks", "", "Kubernetes context for EKS entries")
	f.StringVar(&ingressBaseDomain, "ingress-base-domain", "", "Base domain for ingress hosts; each entry gets <namespace>.<base-domain>")
	f.IntVar(&maxParallel, "max-parallel", 1, "Maximum number of entries to run concurrently (1 = sequential)")
	f.StringVar(&envFile, "env-file", "", "Default .env file for all versions (overridden by --env-file-X.Y)")
	f.StringVar(&envFile86, "env-file-8.6", "", "Path to .env file for 8.6 entries")
	f.StringVar(&envFile87, "env-file-8.7", "", "Path to .env file for 8.7 entries")
	f.StringVar(&envFile88, "env-file-8.8", "", "Path to .env file for 8.8 entries")
	f.StringVar(&envFile89, "env-file-8.9", "", "Path to .env file for 8.9 entries")
	f.StringVarP(&logLevel, "log-level", "l", "info", "Log level (debug, info, warn, error)")

	registerIngressBaseDomainCompletion(cmd)
	registerKubeContextCompletion(cmd)
	registerKubeContextCompletionForFlag(cmd, "kube-context-gke")
	registerKubeContextCompletionForFlag(cmd, "kube-context-eks")
	_ = cmd.RegisterFlagCompletionFunc("log-level", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return completeLogLevels(toComplete)
	})

	return cmd
}

// registerKubeContextCompletionForFlag adds tab completion for a named kube-context flag.
func registerKubeContextCompletionForFlag(cmd *cobra.Command, flagName string) {
	_ = cmd.RegisterFlagCompletionFunc(flagName, func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
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

// resolveRepoRoot resolves the repository root from the flag or config file.
func resolveRepoRoot(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}

	// Try to resolve from config file
	var tempFlags config.RuntimeFlags
	if _, err := config.LoadAndMerge(configFile, false, &tempFlags); err == nil {
		if tempFlags.RepoRoot != "" {
			return tempFlags.RepoRoot
		}
	}

	return ""
}
