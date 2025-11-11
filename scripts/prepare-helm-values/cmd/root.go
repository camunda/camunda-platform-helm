package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"scripts/prepare-helm-values/internal/logging"
	"scripts/prepare-helm-values/internal/values"
)

func Execute() {
	var (
		chartPath    string
		scenario     string
		valuesConfig string
		licenseKey   string
		output       string
		outputDir    string
		verbose      bool
		noColor      bool
	)

	root := &cobra.Command{
		Use:   "prepare-helm-values",
		Short: "Prepare Helm values file by substituting placeholders and injecting license key",
		Long:  "Reads a scenario values file, validates required environment variables, substitutes $VAR and ${VAR} placeholders, and optionally injects .global.license.key.",
		Example: `
  prepare-helm-values \
    --chart-path ./charts/camunda-platform-8.8 \
    --scenario keycloak-original \
    --values-config '{}' \
    --license-key "$E2E_TESTS_LICENSE_KEY" \
    --output-dir /tmp/prepared-values \
    --verbose`,
		SilenceUsage:  true,
		SilenceErrors: true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			// Cobra will enforce required flags via MarkFlagRequired, this is just for nicer logs
			log := logging.Logger{Verbose: verbose, Color: !noColor}
			if chartPath == "" || scenario == "" {
				log.Errorf("Required flags missing: --chart-path and --scenario must be set")
				return fmt.Errorf("missing required flags")
			}
			if output != "" && outputDir != "" {
				log.Errorf("Cannot specify both --output and --output-dir")
				return fmt.Errorf("conflicting output flags")
			}
			log.Debugf("Flags OK: chart-path=%s scenario=%s", chartPath, scenario)
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			log := logging.Logger{Verbose: verbose, Color: !noColor}

			opts := values.Options{
				ChartPath:    chartPath,
				Scenario:     scenario,
				ValuesConfig: valuesConfig,
				LicenseKey:   licenseKey,
				Output:       output,
				OutputDir:    outputDir,
			}

			valuesFile, err := values.ResolveValuesFile(opts, log)
			if err != nil {
				log.Errorf("Values file not found or inaccessible: %v", err)
				return err
			}

			log.Infof("Using chart path: %s", chartPath)
			log.Infof("Scenario: %s", scenario)
			log.Debugf("Source values file: %s", valuesFile)

			outputPath, content, err := values.Process(valuesFile, opts, log)
			if err != nil {
				if missing, names := values.IsMissingEnv(err); missing {
					log.Errorf("Missing required environment variables for substitution:")
					for _, v := range names {
						fmt.Printf("%s   - %s\n", log.Tag("[ERR ]"), v)
					}
					os.Exit(3)
				}
				log.Errorf("Processing failed: %v", err)
				return err
			}

			log.Okf("Prepared values file: %s", outputPath)
			if outputDir == "" {
				// Only print to stdout if not writing to output-dir
				fmt.Print(content)
			}
			return nil
		},
	}

	root.Flags().StringVar(&chartPath, "chart-path", "", "Root chart path used to resolve scenarios dir (required)")
	root.Flags().StringVar(&scenario, "scenario", "", "Scenario name (required)")
	root.Flags().StringVar(&valuesConfig, "values-config", "", "JSON config string for env injection; \"{}\" or empty = skip")
	root.Flags().StringVar(&licenseKey, "license-key", os.Getenv("E2E_TESTS_LICENSE_KEY"), "License key to inject; defaults to $E2E_TESTS_LICENSE_KEY")
	root.Flags().StringVar(&output, "output", "", "Output file path (defaults to scenario values file in-place)")
	root.Flags().StringVar(&outputDir, "output-dir", "", "Output directory path (writes with scenario-based filename)")
	root.Flags().BoolVar(&verbose, "verbose", false, "Enable verbose logging")
	root.Flags().BoolVar(&noColor, "no-color", false, "Disable colored output")
	_ = root.MarkFlagRequired("chart-path")
	_ = root.MarkFlagRequired("scenario")

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}


