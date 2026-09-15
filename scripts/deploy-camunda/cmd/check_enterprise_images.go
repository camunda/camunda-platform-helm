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
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"scripts/deploy-camunda/config"
	"scripts/deploy-camunda/deploy"

	"github.com/spf13/cobra"
)

var (
	auditEnterpriseImages = deploy.AuditEnterpriseImages
	lookPath              = exec.LookPath
)

func newCheckEnterpriseImagesCommand() *cobra.Command {
	var (
		chartVersions []string
		platform      string
		repoRoot      string
	)

	cmd := &cobra.Command{
		Use:   "check-enterprise-images",
		Short: "Assert every values-enterprise.yaml image is pullable, child manifests included",
		Long: `Resolve every image that a chart's values-enterprise.yaml pins completely
(registry + repository + tag) against the registry, and assert both the
multi-arch index and the per-platform child manifest it advertises.

An index-only probe reports success for an image whose linux/amd64 child is not
fetchable, which is the failure mode tracked by
camunda/camunda-platform-helm#6804.

Every chart is visited and every failing image is reported. Exits non-zero if
any chart references an image that is not pullable.`,
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		// Registry auditing needs no chart, namespace or release, so the root's
		// deploy validation is replaced rather than excluded by name.
		PersistentPreRunE: func(*cobra.Command, []string) error { return nil },
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			if err := deploy.ValidateImagePlatform(platform); err != nil {
				return err
			}
			if _, err := lookPath("docker"); err != nil {
				return fmt.Errorf("docker is required to resolve image manifests: %w", err)
			}

			root, err := resolveEnterpriseRepoRoot(repoRoot)
			if err != nil {
				return err
			}
			charts, err := enterpriseChartDirs(root, chartVersions)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			failedCharts, childDenials := 0, 0
			for _, chart := range charts {
				valuesFile := filepath.Join(root, "charts", chart, "values-enterprise.yaml")
				if _, err := os.Stat(valuesFile); err != nil {
					fmt.Fprintf(out, "Skipping %s: no values-enterprise.yaml\n", chart)
					continue
				}

				fmt.Fprintf(out, "\nValidating %s\n", chart)
				results, auditErr := auditEnterpriseImages(ctx, valuesFile, platform)
				if auditErr != nil {
					failedCharts++
					fmt.Fprintf(out, "  ✗ %v\n✗ %s validation failed\n", auditErr, chart)
					continue
				}
				failed := 0
				for _, r := range results {
					if r.OK {
						fmt.Fprintf(out, "  ✓ %s\n", r.Ref)
						continue
					}
					failed++
					if r.ChildDenied {
						childDenials++
					}
					fmt.Fprintf(out, "  ✗ %s\n", r.Detail)
				}
				if failed > 0 {
					failedCharts++
					fmt.Fprintf(out, "✗ %s validation failed (%d of %d images)\n", chart, failed, len(results))
					continue
				}
				fmt.Fprintf(out, "✓ %s validation passed (%d images)\n", chart, len(results))
			}

			if failedCharts > 0 {
				if childDenials > 0 {
					fmt.Fprintln(out, "\nWhen the registry serves an index but denies the child it advertises, "+
						"a pin change will not help — see camunda/camunda-platform-helm#6804.")
				}
				return fmt.Errorf("%d of %d chart(s) failed enterprise image validation on %s",
					failedCharts, len(charts), platform)
			}
			return nil
		},
	}

	f := cmd.Flags()
	f.StringSliceVar(&chartVersions, "chart-version", nil,
		"Chart version(s) to check, e.g. 8.7 (repeatable or comma-separated); "+
			"defaults to every charts/camunda-platform-8.* directory")
	f.StringVar(&platform, "platform", deploy.DefaultImagePlatform,
		"Image platform whose child manifest must resolve")
	f.StringVar(&repoRoot, "repo-root", "", "Repository root (auto-detected when empty)")

	return cmd
}

func resolveEnterpriseRepoRoot(explicit string) (string, error) {
	if root := strings.TrimSpace(explicit); root != "" {
		return root, nil
	}
	detected, err := config.DetectRepoRoot()
	if err != nil {
		return "", err
	}
	if detected == "" {
		return "", fmt.Errorf("could not detect the repository root; pass --repo-root")
	}
	return detected, nil
}

func enterpriseChartDirs(repoRoot string, versions []string) ([]string, error) {
	var out []string
	for _, v := range versions {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, "camunda-platform-"+v)
		}
	}
	if len(out) > 0 {
		return out, nil
	}

	matches, err := filepath.Glob(filepath.Join(repoRoot, "charts", "camunda-platform-8.*"))
	if err != nil {
		return nil, err
	}
	for _, m := range matches {
		if fi, statErr := os.Stat(m); statErr == nil && fi.IsDir() {
			out = append(out, filepath.Base(m))
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no charts/camunda-platform-8.* directories found under %s", repoRoot)
	}
	sort.Strings(out)
	return out, nil
}
