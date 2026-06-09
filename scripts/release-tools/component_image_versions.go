package main

import (
	"flag"
	"fmt"
	"os"

	"scripts/camunda-core/pkg/chartmeta"
)

// runComponentImageVersions builds the component-image-versions annotation block
// from the chart's values.yaml and prints it to stdout, e.g.:
//
//	release-tools component-image-versions --chart "$dir" --camunda-version 8.10 > /tmp/image-versions.yaml
//	yq -i '.annotations."camunda.io/component-image-versions" = load_str("/tmp/image-versions.yaml")' Chart.yaml
//
// The component set is version-gated (8.8+ orchestration vs 8.6–8.7 classic).
func runComponentImageVersions(args []string) error {
	fs := flag.NewFlagSet("component-image-versions", flag.ContinueOnError)
	var (
		chartDir       string
		camundaVersion string
	)
	fs.StringVar(&chartDir, "chart", "", "chart directory (e.g. charts/camunda-platform-<v>)")
	fs.StringVar(&camundaVersion, "camunda-version", "", "Camunda minor line, e.g. 8.10")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if chartDir == "" || camundaVersion == "" {
		return fmt.Errorf("--chart and --camunda-version are required")
	}

	block, err := chartmeta.ComponentImageVersions(chartDir, camundaVersion)
	if err != nil {
		return fmt.Errorf("build component-image-versions: %w", err)
	}
	if block == "" {
		return fmt.Errorf("empty component-image-versions for %s; refusing to record", chartDir)
	}
	_, err = fmt.Fprint(os.Stdout, block)
	return err
}
