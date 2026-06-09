package main

import (
	"flag"
	"fmt"
	"strconv"
	"strings"

	"scripts/camunda-core/pkg/chartmeta"
)

// runChartMetadata reads a pulled artifact's Chart.yaml and emits its metadata to
// $GITHUB_OUTPUT. The .tgz extraction (tar) happens in the caller; this reads the
// already-extracted Chart.yaml.
//
//	chart-metadata --chart-yaml /tmp/Chart.yaml [--chart-versions <chart-versions.yaml>]
//
// Emits: version, app_version, camunda_version (= app_version alias), chart_dir_id,
// prerelease, release_tag, cosign_bundle, cosign_verify, has_image_overrides,
// image_versions (multiline, only when present), and is_latest_stable (only when
// --chart-versions is given). Callers consume the subset they need.
func runChartMetadata(args []string) error {
	fs := flag.NewFlagSet("chart-metadata", flag.ContinueOnError)
	var chartYAML, chartVersions string
	fs.StringVar(&chartYAML, "chart-yaml", "", "path to the extracted Chart.yaml")
	fs.StringVar(&chartVersions, "chart-versions", "", "path to charts/chart-versions.yaml (enables is_latest_stable)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if chartYAML == "" {
		return fmt.Errorf("--chart-yaml is required")
	}

	meta, err := chartmeta.ReadPackageMetadata(chartYAML, chartVersions)
	if err != nil {
		return err
	}

	out := newGitHubOutput()
	pairs := [][2]string{
		{"version", meta.Version},
		{"app_version", meta.AppVersion},
		{"camunda_version", meta.AppVersion},
		{"chart_dir_id", meta.AppVersion},
		{"prerelease", meta.Prerelease},
		{"release_tag", meta.ReleaseTag},
		{"cosign_bundle", meta.CosignBundle},
		{"cosign_verify", meta.CosignVerify},
		{"has_image_overrides", strconv.FormatBool(meta.HasImageOverrides)},
	}
	for _, kv := range pairs {
		if err := out.set(kv[0], kv[1]); err != nil {
			return err
		}
	}
	// Only emit image_versions when the annotation is present; strip trailing newlines.
	if iv := strings.TrimRight(meta.ImageVersions, "\n"); iv != "" {
		if err := out.set("image_versions", iv); err != nil {
			return err
		}
	}
	if meta.IsLatestStable != nil {
		if err := out.set("is_latest_stable", strconv.FormatBool(*meta.IsLatestStable)); err != nil {
			return err
		}
	}
	return nil
}
