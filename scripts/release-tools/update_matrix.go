package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"scripts/camunda-core/pkg/chartmeta"
	"scripts/camunda-core/pkg/versionmatrix"
)

// enterpriseRegistryPrefix selects the enterprise (registry.camunda.cloud)
// replacements out of an image set rendered with values-enterprise.yaml.
const enterpriseRegistryPrefix = "registry.camunda.cloud/"

// runUpdateMatrix upserts a version-matrix.json entry for a chart version. Two
// input modes (exactly one):
//
//	--chart-yaml <Chart.yaml>  read the recorded camunda.io/chart-images annotation
//	                           (Promote-RC: the artifact's baked-in image set).
//	--chart <chart-dir>        derive the image set from the chart's values
//	                           (chores/source-sync). chart_enterprise_images is
//	                           derived automatically when values-enterprise.yaml
//	                           exists (its registry.camunda.cloud images).
//
// --dry-run prints the would-be file to stdout and writes nothing.
func runUpdateMatrix(args []string) error {
	fs := flag.NewFlagSet("update-matrix", flag.ContinueOnError)
	var (
		chartYAML    string
		chartDir     string
		chartVersion string
		matrixFile   string
		dryRun       bool
	)
	fs.StringVar(&chartYAML, "chart-yaml", "", "path to a pulled package's Chart.yaml (reads the camunda.io/chart-images annotation)")
	fs.StringVar(&chartDir, "chart", "", "chart directory to derive the image set from (alternative to --chart-yaml)")
	fs.StringVar(&chartVersion, "chart-version", "", "chart version key for the matrix entry (e.g. 13.4.0)")
	fs.StringVar(&matrixFile, "matrix-file", "", "path to version-matrix.json to update")
	fs.BoolVar(&dryRun, "dry-run", false, "print the updated version-matrix.json to stdout instead of writing it")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if chartVersion == "" || matrixFile == "" {
		return fmt.Errorf("--chart-version and --matrix-file are required")
	}
	if (chartYAML == "") == (chartDir == "") {
		return fmt.Errorf("exactly one of --chart-yaml or --chart is required")
	}

	var images, enterpriseImages []string
	var err error
	switch {
	case chartYAML != "":
		// Read the image set the artifact recorded at build time.
		if images, err = chartmeta.ChartImages(chartYAML); err != nil {
			return err
		}
		if len(images) == 0 {
			return fmt.Errorf("%s annotation in %s is empty or missing; it must be recorded at build time before promote", chartmeta.ChartImagesAnnotation, chartYAML)
		}
	default:
		// Derive from the chart's values (same tooling the build embeds with).
		if images, err = chartmeta.ImageSet(chartDir); err != nil {
			return fmt.Errorf("derive image set: %w", err)
		}
		if len(images) == 0 {
			return fmt.Errorf("no images derived from %s/values.yaml", chartDir)
		}
		if enterpriseImages, err = enterpriseImageSet(chartDir); err != nil {
			return err
		}
	}

	existing, err := os.ReadFile(matrixFile)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("read matrix file %s: %w", matrixFile, err)
		}
		existing = []byte("[]")
	}

	updated, err := versionmatrix.UpsertImages(existing, chartVersion, images, enterpriseImages)
	if err != nil {
		return err
	}

	if dryRun {
		_, err := os.Stdout.Write(updated)
		return err
	}
	if err := os.MkdirAll(filepath.Dir(matrixFile), 0o755); err != nil {
		return fmt.Errorf("create matrix dir: %w", err)
	}
	if err := os.WriteFile(matrixFile, updated, 0o644); err != nil {
		return fmt.Errorf("write matrix file %s: %w", matrixFile, err)
	}
	fmt.Fprintf(os.Stderr, "updated %s (entry %s, %d images, %d enterprise)\n", matrixFile, chartVersion, len(images), len(enterpriseImages))
	return nil
}

// enterpriseImageSet derives the chart's enterprise images: the chart's image
// set rendered with values-enterprise.yaml overlaid, kept to the
// registry.camunda.cloud replacements. Returns nil when the chart has no
// values-enterprise.yaml.
func enterpriseImageSet(chartDir string) ([]string, error) {
	entPath := filepath.Join(chartDir, "values-enterprise.yaml")
	if _, err := os.Stat(entPath); err != nil {
		return nil, nil
	}
	full, err := chartmeta.ImageSet(chartDir, entPath)
	if err != nil {
		return nil, fmt.Errorf("derive enterprise image set: %w", err)
	}
	var enterprise []string
	for _, ref := range full {
		if strings.HasPrefix(ref, enterpriseRegistryPrefix) {
			enterprise = append(enterprise, ref)
		}
	}
	return enterprise, nil
}
