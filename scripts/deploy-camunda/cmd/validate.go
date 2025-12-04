package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"scripts/camunda-core/pkg/logging"
	"scripts/deploy-camunda/config"
	"scripts/prepare-helm-values/pkg/env"
	"strings"

	"github.com/jwalton/gchalk"
	"github.com/spf13/cobra"
)

// newValidateCommand creates the validate subcommand.
func newValidateCommand() *cobra.Command {
	var validateFlags struct {
		checkScenarios bool
		checkChart     bool
		verbose        bool
	}

	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate configuration without deploying",
		Long: `Validate the deploy-camunda configuration file and check that all required
resources exist. This includes:
  - Configuration file syntax and required fields
  - Scenario files existence (with --check-scenarios)
  - Chart path validity (with --check-chart)

This command does not make any changes to your cluster.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Setup logging
			if err := logging.Setup(logging.Options{
				LevelString:  flags.LogLevel,
				ColorEnabled: logging.IsTerminal(os.Stdout.Fd()),
			}); err != nil {
				return err
			}

			// Load .env file
			if flags.EnvFile != "" {
				_ = env.Load(flags.EnvFile)
			} else {
				_ = env.Load(".env")
			}

			// Load config
			cfgPath, err := config.ResolvePath(configFile)
			if err != nil {
				return fmt.Errorf("failed to resolve config path: %w", err)
			}

			rc, err := config.Read(cfgPath, true)
			if err != nil {
				return fmt.Errorf("failed to read config: %w", err)
			}

			// Run validation
			results := validateConfiguration(rc, &flags, validateFlags.checkScenarios, validateFlags.checkChart, validateFlags.verbose)

			// Print results
			printValidationResults(results, cfgPath)

			if results.hasErrors() {
				return fmt.Errorf("validation failed with %d error(s)", results.errorCount())
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&validateFlags.checkScenarios, "check-scenarios", true, "Verify scenario files exist")
	cmd.Flags().BoolVar(&validateFlags.checkChart, "check-chart", true, "Verify chart path exists")
	cmd.Flags().BoolVar(&validateFlags.verbose, "verbose", false, "Show detailed validation output")

	return cmd
}

// ValidationResults holds the results of configuration validation.
type ValidationResults struct {
	ConfigPath string
	Checks     []ValidationCheck
}

// ValidationCheck represents a single validation check result.
type ValidationCheck struct {
	Name    string
	Status  ValidationStatus
	Message string
	Details []string
}

// ValidationStatus represents the status of a validation check.
type ValidationStatus int

const (
	StatusPass ValidationStatus = iota
	StatusWarn
	StatusFail
)

func (v *ValidationResults) hasErrors() bool {
	for _, c := range v.Checks {
		if c.Status == StatusFail {
			return true
		}
	}
	return false
}

func (v *ValidationResults) errorCount() int {
	count := 0
	for _, c := range v.Checks {
		if c.Status == StatusFail {
			count++
		}
	}
	return count
}

func (v *ValidationResults) addCheck(name string, status ValidationStatus, message string, details ...string) {
	v.Checks = append(v.Checks, ValidationCheck{
		Name:    name,
		Status:  status,
		Message: message,
		Details: details,
	})
}

// validateConfiguration performs all validation checks.
func validateConfiguration(rc *config.RootConfig, flags *config.RuntimeFlags, checkScenarios, checkChart, verbose bool) *ValidationResults {
	results := &ValidationResults{
		ConfigPath: rc.FilePath,
	}

	// Check 1: Config file exists and is readable
	if rc.FilePath != "" {
		if _, err := os.Stat(rc.FilePath); err == nil {
			results.addCheck("Config file", StatusPass, fmt.Sprintf("Found at %s", rc.FilePath))
		} else {
			results.addCheck("Config file", StatusWarn, "Using defaults (no config file found)")
		}
	}

	// Check 2: Apply deployment and validate merged config
	if err := config.ApplyActiveDeployment(rc, rc.Current, flags); err != nil {
		results.addCheck("Active deployment", StatusFail, err.Error())
	} else if rc.Current != "" {
		results.addCheck("Active deployment", StatusPass, fmt.Sprintf("Using '%s'", rc.Current))
	} else if len(rc.Deployments) > 0 {
		results.addCheck("Active deployment", StatusWarn, "No deployment selected (use 'config use <name>')")
	} else {
		results.addCheck("Active deployment", StatusPass, "Using root-level defaults")
	}

	// Check 3: Required fields
	if err := config.Validate(flags); err != nil {
		// Parse the error to provide more specific feedback
		errMsg := err.Error()
		if strings.Contains(errMsg, "namespace") {
			results.addCheck("Namespace", StatusFail, "Not set - provide via --namespace or config")
		}
		if strings.Contains(errMsg, "release") {
			results.addCheck("Release", StatusFail, "Not set - provide via --release or config")
		}
		if strings.Contains(errMsg, "scenario") {
			results.addCheck("Scenario", StatusFail, "Not set - provide via --scenario or config")
		}
		if strings.Contains(errMsg, "chart") {
			results.addCheck("Chart", StatusFail, "Neither --chart-path nor --chart is set")
		}
	} else {
		results.addCheck("Required fields", StatusPass, "All required fields are set")
	}

	// Check 4: Chart path (if requested)
	if checkChart && strings.TrimSpace(flags.ChartPath) != "" {
		if fi, err := os.Stat(flags.ChartPath); err == nil && fi.IsDir() {
			results.addCheck("Chart path", StatusPass, flags.ChartPath)

			// Check for Chart.yaml
			chartYaml := filepath.Join(flags.ChartPath, "Chart.yaml")
			if _, err := os.Stat(chartYaml); err == nil {
				results.addCheck("Chart.yaml", StatusPass, "Found")
			} else {
				results.addCheck("Chart.yaml", StatusFail, "Not found in chart path")
			}
		} else {
			results.addCheck("Chart path", StatusFail, fmt.Sprintf("Directory not found: %s", flags.ChartPath))
		}
	}

	// Check 5: Scenario files (if requested)
	if checkScenarios && len(flags.Scenarios) > 0 {
		scenarioDir := flags.ScenarioPath
		if scenarioDir == "" && flags.ChartPath != "" {
			scenarioDir = filepath.Join(flags.ChartPath, "test/integration/scenarios/chart-full-setup")
		}

		if scenarioDir != "" {
			for _, scenario := range flags.Scenarios {
				filename := fmt.Sprintf("values-integration-test-ingress-%s.yaml", scenario)
				fullPath := filepath.Join(scenarioDir, filename)
				if _, err := os.Stat(fullPath); err == nil {
					results.addCheck(fmt.Sprintf("Scenario '%s'", scenario), StatusPass, "Found")
				} else {
					results.addCheck(fmt.Sprintf("Scenario '%s'", scenario), StatusFail,
						fmt.Sprintf("Not found: %s", filename),
						fmt.Sprintf("Expected at: %s", fullPath))
				}
			}
		}
	}

	// Check 6: Deployments summary
	if len(rc.Deployments) > 0 {
		var depNames []string
		for name := range rc.Deployments {
			depNames = append(depNames, name)
		}
		results.addCheck("Configured deployments", StatusPass, fmt.Sprintf("%d deployment(s)", len(rc.Deployments)), depNames...)
	}

	return results
}

// printValidationResults outputs the validation results in a formatted way.
func printValidationResults(results *ValidationResults, configPath string) {
	styleOk := func(s string) string { return logging.Emphasize(s, gchalk.Green) }
	styleWarn := func(s string) string { return logging.Emphasize(s, gchalk.Yellow) }
	styleErr := func(s string) string { return logging.Emphasize(s, gchalk.Red) }
	styleHead := func(s string) string { return logging.Emphasize(s, gchalk.Bold) }
	styleKey := func(s string) string { return logging.Emphasize(s, gchalk.Cyan) }

	var out strings.Builder
	out.WriteString(styleHead("Configuration Validation Results"))
	out.WriteString("\n")
	out.WriteString(fmt.Sprintf("Config: %s\n\n", styleKey(configPath)))

	passCount, warnCount, failCount := 0, 0, 0
	for _, check := range results.Checks {
		var statusIcon, statusStyle func(string) string
		switch check.Status {
		case StatusPass:
			statusIcon = func(s string) string { return styleOk("✓") }
			statusStyle = styleOk
			passCount++
		case StatusWarn:
			statusIcon = func(s string) string { return styleWarn("⚠") }
			statusStyle = styleWarn
			warnCount++
		case StatusFail:
			statusIcon = func(s string) string { return styleErr("✗") }
			statusStyle = styleErr
			failCount++
		}

		fmt.Fprintf(&out, "%s %s: %s\n", statusIcon(""), styleKey(check.Name), statusStyle(check.Message))
		for _, detail := range check.Details {
			fmt.Fprintf(&out, "    %s\n", detail)
		}
	}

	out.WriteString("\n")
	out.WriteString(styleHead("Summary: "))
	fmt.Fprintf(&out, "%s passed", styleOk(fmt.Sprintf("%d", passCount)))
	if warnCount > 0 {
		fmt.Fprintf(&out, ", %s warnings", styleWarn(fmt.Sprintf("%d", warnCount)))
	}
	if failCount > 0 {
		fmt.Fprintf(&out, ", %s failed", styleErr(fmt.Sprintf("%d", failCount)))
	}
	out.WriteString("\n")

	logging.Logger.Info().Msg(out.String())
}

