package main

import (
	"fmt"
	"os"
	"path/filepath"

	"camunda.com/helm-unused-values/pkg/config"
	"camunda.com/helm-unused-values/pkg/output"
	"camunda.com/helm-unused-values/pkg/patterns"
	"camunda.com/helm-unused-values/pkg/search"
	"camunda.com/helm-unused-values/pkg/values"

	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
)

func main() {
	// Initialize configuration
	cfg := config.New()

	// Create root command
	rootCmd := &cobra.Command{
		Use:   "helm-unused-values [templates_dir]",
		Short: "Check for unused values in Helm charts",
		Long: `A tool to identify values defined in values.yaml that are not used in templates.
Performance note: If ripgrep (rg) is installed, it will be used for faster searching.
Progress indicators are displayed during long-running operations.`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			// Set templates directory
			cfg.TemplatesDir = args[0]

			// Run the analyzer
			if err := run(cfg); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		},
	}

	// Add command line flags
	rootCmd.Flags().BoolVar(&cfg.NoColors, "no-colors", false, "Disable colored output")
	rootCmd.Flags().BoolVar(&cfg.ShowAllKeys, "show-all-keys", false, "Show all keys (used and unused), not just unused ones")
	rootCmd.Flags().BoolVar(&cfg.JSONOutput, "json", false, "Output results in JSON format (useful for CI)")
	rootCmd.Flags().StringVar(&cfg.OutputFile, "output-file", "", "Write results to the specified file")
	rootCmd.Flags().IntVar(&cfg.ExitCodeOnUnused, "exit-code", 0, "Set exit code when unused values are found (default: 0)")
	rootCmd.Flags().BoolVar(&cfg.QuietMode, "quiet", false, "Suppress all output except results and errors")
	rootCmd.Flags().StringVar(&cfg.FilterPattern, "filter", "", "Only show keys that match the specified pattern (works with --show-all-keys)")
	rootCmd.Flags().BoolVar(&cfg.Debug, "debug", false, "Enable verbose debug logging")
	rootCmd.Flags().IntVar(&cfg.GrepTimeout, "grep-timeout", 5, "Timeout for grep/ripgrep in seconds")
	rootCmd.Flags().StringVar(&cfg.SearchTool, "search-tool", "", "Search tool to use: 'ripgrep' or 'grep' (default: use ripgrep if available)")
	rootCmd.Flags().BoolVar(&cfg.UseShell, "use-shell", false, "Use shell for executing search commands (troubleshooting)")
	rootCmd.Flags().BoolVar(&cfg.ShowTestCommands, "show-test-commands", false, "Show test commands for unused keys that you can run on the terminal")
	rootCmd.Flags().IntVar(&cfg.Parallelism, "parallelism", 0, "Number of parallel workers (0 = auto based on CPU cores)")

	// Execute command
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// run executes the main application logic
func run(cfg *config.Config) error {
	// Create output display
	display := output.NewDisplay(cfg.NoColors, cfg.QuietMode)

	// Set progress visibility (show progress bars except in quiet/JSON modes)
	showProgress := !cfg.QuietMode && !cfg.JSONOutput

	// Validate templates directory
	if err := values.ValidateDirectory(cfg.TemplatesDir); err != nil {
		return fmt.Errorf("invalid templates directory: %w", err)
	}

	// Check dependencies
	depOk, missing := search.CheckDependencies()
	if !depOk {
		display.PrintError(fmt.Sprintf("Missing required dependencies: %v", missing))
		return fmt.Errorf("missing required dependencies")
	}

	// Check for ripgrep and determine search tool to use
	ripgrepAvailable := search.DetectRipgrep()

	// Set UseRipgrep based on SearchTool preference or auto-detection
	if cfg.SearchTool == "grep" {
		cfg.UseRipgrep = false
		display.PrintInfo("Using grep as specified")
	} else if cfg.SearchTool == "ripgrep" {
		if !ripgrepAvailable {
			display.PrintWarning("Ripgrep was specified but not found, falling back to grep")
			cfg.UseRipgrep = false
		} else {
			cfg.UseRipgrep = true
			display.PrintSuccess("Using ripgrep as specified")
		}
	} else {
		// Auto-detect
		cfg.UseRipgrep = ripgrepAvailable
		if cfg.UseRipgrep {
			display.PrintSuccess("Using ripgrep for faster searching")
		} else {
			display.PrintWarning("Ripgrep not found, using grep instead")
		}
	}

	// Create pattern registry
	patternRegistry := patterns.New(cfg.Debug)
	if err := patternRegistry.RegisterBuiltins(); err != nil {
		return fmt.Errorf("failed to register patterns: %w", err)
	}
	defer patternRegistry.CleanUp()

	// Set up key extractor
	keyExtractor := values.NewExtractor(cfg.Debug)

	// Define values.yaml path
	valuesFile := filepath.Join(cfg.TemplatesDir, "..", "values.yaml")
	if err := values.ValidateFile(valuesFile); err != nil {
		return fmt.Errorf("invalid values file: %w", err)
	}

	// Extract keys from values.yaml with progress bar
	if !cfg.QuietMode {
		display.PrintInfo("Extracting keys from values.yaml...")
	}

	var keys []string
	var err error

	if showProgress {
		// Create a progress bar for parsing
		bar := progressbar.NewOptions(1,
			progressbar.OptionEnableColorCodes(true),
			progressbar.OptionSetWidth(50),
			progressbar.OptionSetDescription("Parsing YAML file..."),
			progressbar.OptionShowCount(),
			progressbar.OptionUseANSICodes(true),
			progressbar.OptionSetPredictTime(false),
			progressbar.OptionSpinnerType(14),
			progressbar.OptionSetTheme(progressbar.Theme{
				Saucer:        "[blue]=[reset]",
				SaucerHead:    "[blue]>[reset]",
				SaucerPadding: " ",
				BarStart:      "[",
				BarEnd:        "]",
			}))

		// Start the key extraction (with progress reporting)
		keys, err = keyExtractor.ExtractKeysWithProgress(valuesFile, bar)
	} else {
		// Start the key extraction (without progress reporting)
		keys, err = keyExtractor.ExtractKeys(valuesFile)
	}

	if err != nil {
		return fmt.Errorf("extract values keys: %w", err)
	}

	// Apply filter if specified
	if cfg.FilterPattern != "" {
		display.PrintInfo(fmt.Sprintf("Filtering results to only show keys matching: %s", cfg.FilterPattern))
		keys = keyExtractor.FilterKeys(keys, cfg.FilterPattern)
	}

	// Report total keys found
	display.PrintWarning(fmt.Sprintf("Total keys found: %d", len(keys)))
	fmt.Println()

	// Create finder
	finder := search.NewFinder(cfg.TemplatesDir, patternRegistry, cfg.UseRipgrep, cfg.Debug)

	// Set parallelism if configured
	if cfg.Parallelism > 0 {
		display.PrintInfo(fmt.Sprintf("Using %d parallel workers", cfg.Parallelism))
		finder.Parallelism = cfg.Parallelism
	}

	// Analyze key usage
	display.PrintInfo("Analyzing key usage:")
	usages, err := finder.FindUnusedKeys(keys, showProgress)
	if err != nil {
		return fmt.Errorf("find unused keys: %w", err)
	}

	// Create reporter
	reporter := output.NewReporter(display, cfg.JSONOutput, cfg.OutputFile, cfg.ShowAllKeys, cfg.ShowTestCommands)

	// Report results
	if err := reporter.ReportResults(usages); err != nil {
		return fmt.Errorf("report results: %w", err)
	}

	// Calculate unused keys
	unusedKeys := output.FilterByUsageType(usages, "unused")
	parentKeys := output.FilterByUsageType(usages, "parent")
	totalUnused := len(unusedKeys) + len(parentKeys)

	// Exit with appropriate code if unused keys found
	if totalUnused > 0 && cfg.ExitCodeOnUnused != 0 {
		display.DebugLog(cfg.Debug, fmt.Sprintf("Exiting with code %d (unused keys found)", cfg.ExitCodeOnUnused))
		os.Exit(cfg.ExitCodeOnUnused)
	}

	return nil
}
