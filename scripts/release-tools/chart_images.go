package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"scripts/camunda-core/pkg/chartmeta"
)

// runChartImages derives the chart's declared image set from its values.yaml
// (plus the chart-full-setup scenario layers) and prints it one fully-qualified
// reference per line, e.g. to record as the camunda.io/chart-images annotation:
//
//	release-tools chart-images --chart "$chart_dir" > /tmp/chart-images.txt
//	yq -i '.annotations."camunda.io/chart-images" = load_str("/tmp/chart-images.txt")' "$chart_dir/Chart.yaml"
//
// It fails loud on an empty result rather than recording an empty set: a valid
// chart always declares images.
func runChartImages(args []string) error {
	fs := flag.NewFlagSet("chart-images", flag.ContinueOnError)
	var chartDir string
	fs.StringVar(&chartDir, "chart", "", "chart directory (the finalized chart, e.g. charts/camunda-platform-<v>)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if chartDir == "" {
		return fmt.Errorf("--chart is required")
	}

	images, err := chartmeta.ImageSet(chartDir)
	if err != nil {
		return fmt.Errorf("derive image set: %w", err)
	}
	if len(images) == 0 {
		return fmt.Errorf("no images declared in %s/values.yaml; refusing to record an empty chart-images set", chartDir)
	}

	_, err = fmt.Fprintln(os.Stdout, strings.Join(images, "\n"))
	return err
}
