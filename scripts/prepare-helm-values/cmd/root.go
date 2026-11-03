// Copyright 2026 Camunda Services GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"scripts/camunda-core/pkg/completion"
	"scripts/camunda-core/pkg/logging"
	"scripts/prepare-helm-values/pkg/env"
	"scripts/prepare-helm-values/pkg/values"
)

type rootFlags struct {
	chartPath     string
	scenario      string
	valuesConfig  string
	licenseKey    string
	output        string
	outputDir     string
	envFile       string
	interactive   bool
	logLevel      string
	noColor       bool
	imageTagsFile string
}

func Execute() {
	var f rootFlags

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
    --log-level info`,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return logging.Setup(logging.Options{
				UseJSON:      false,
				LevelString:  f.logLevel,
				ColorEnabled: !f.noColor && logging.IsTerminal(os.Stdout.Fd()),
			})
		},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return f.validate()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return f.run()
		},
	}

	root.Flags().StringVar(&f.chartPath, "chart-path", "", "Root chart path used to resolve scenarios dir (required)")
	root.Flags().StringVar(&f.scenario, "scenario", "", "Scenario name (required)")
	completion.RegisterScenarioCompletion(root, "scenario", "chart-path")
	root.Flags().StringVar(&f.valuesConfig, "values-config", "", "JSON config string for env injection; \"{}\" or empty = skip")
	root.Flags().StringVar(&f.licenseKey, "license-key", os.Getenv("E2E_TESTS_LICENSE_KEY"), "License key to inject; defaults to $E2E_TESTS_LICENSE_KEY")
	root.Flags().StringVar(&f.output, "output", "", "Output file path (defaults to scenario values file in-place)")
	root.Flags().StringVar(&f.outputDir, "output-dir", "", "Output directory path (writes with scenario-based filename)")
	root.Flags().StringVar(&f.envFile, "env-file", "", "Path to .env file (defaults to .env in current dir)")
	root.Flags().StringVar(&f.imageTagsFile, "image-tags-file", "", "Path to image tags values file for substitution (optional)")
	root.Flags().BoolVar(&f.interactive, "interactive", true, "Enable interactive prompts for missing variables")
	root.Flags().StringVar(&f.logLevel, "log-level", "info", "Log level: trace, debug, info, warn, error")
	root.Flags().BoolVar(&f.noColor, "no-color", false, "Disable colored output")
	_ = root.MarkFlagRequired("chart-path")
	_ = root.MarkFlagRequired("scenario")

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func (f *rootFlags) validate() error {
	if f.chartPath == "" || f.scenario == "" {
		logging.Logger.Error().Msg("Required flags missing: --chart-path and --scenario must be set")
		return fmt.Errorf("missing required flags")
	}
	if f.output != "" && f.outputDir != "" {
		logging.Logger.Error().Msg("Cannot specify both --output and --output-dir")
		return fmt.Errorf("conflicting output flags")
	}
	// Try to load .env file
	if f.envFile != "" {
		_ = env.Load(f.envFile)
	} else {
		// Try default locations if not specified
		_ = env.Load(".env")
	}
	logging.Logger.Debug().Str("chart-path", f.chartPath).Str("scenario", f.scenario).Msg("Flags OK")
	return nil
}

func (f *rootFlags) run() error {
	opts := values.Options{
		ChartPath:     f.chartPath,
		Scenario:      f.scenario,
		ScenarioDir:   filepath.Join(f.chartPath, "test", "integration", "scenarios", "chart-full-setup"),
		ValuesConfig:  f.valuesConfig,
		LicenseKey:    f.licenseKey,
		Output:        f.output,
		OutputDir:     f.outputDir,
		Interactive:   f.interactive,
		EnvFile:       f.envFile,
		ImageTagsFile: f.imageTagsFile,
	}

	if opts.EnvFile == "" {
		opts.EnvFile = ".env" // Default for persistence
	}

	// Process image tags file first (if provided)
	imageTagsOutput, err := values.ProcessImageTags(opts)
	if err != nil {
		logging.Logger.Error().Err(err).Msg("Failed to process image tags file")
		return err
	}
	if imageTagsOutput != "" {
		logging.Logger.Info().Str("image-tags-output", imageTagsOutput).Msg("Processed image tags file")
	}

	valuesFile, err := values.ResolveValuesFile(opts)
	if err != nil {
		logging.Logger.Error().Err(err).Msg("Values file not found or inaccessible")
		return err
	}

	logging.Logger.Info().Str("chart-path", f.chartPath).Msg("Using chart path")
	logging.Logger.Info().Str("scenario", f.scenario).Msg("Scenario")
	logging.Logger.Debug().Str("values-file", valuesFile).Msg("Source values file")

	outputPath, content, err := values.Process(context.Background(), valuesFile, opts)
	if err != nil {
		if missing, names := values.IsMissingEnv(err); missing {
			logging.Logger.Error().Msg("Missing required environment variables for substitution:")
			for _, v := range names {
				fmt.Printf("   - %s\n", v)
			}
			os.Exit(3)
		}
		logging.Logger.Error().Err(err).Msg("Processing failed")
		return err
	}

	logging.Logger.Info().Str("output", outputPath).Msg("Prepared values file")
	if f.outputDir == "" {
		// Only print to stdout if not writing to output-dir
		fmt.Print(content)
	}
	return nil
}
