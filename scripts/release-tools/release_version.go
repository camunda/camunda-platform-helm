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

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"

	"scripts/camunda-core/pkg/releaseplease"
)

// runReleaseVersion computes the dev-build release version and derived tags. The
// release-please CLI dry-run runs in the caller; this reads its trace file (only
// consulted for a stable release) and emits to $GITHUB_ENV: RELEASE_VERSION,
// IS_PRERELEASE, RELEASE_VERSION_COMPUTED, DEV_TAG and CHART_MAJOR_VERSION.
func runReleaseVersion(args []string) error {
	fs := flag.NewFlagSet("release-version", flag.ContinueOnError)
	var (
		currentVersion    string
		chartVersion      string
		chartVersionsFile string
		chartDir          string
		shortSHA          string
		traceFile         string
		failIfReleased    bool
	)
	fs.StringVar(&currentVersion, "current-version", "", "Chart.yaml .version")
	fs.StringVar(&chartVersion, "chart-version", "", "chart minor (matrix.chart.version), for the still-alpha check")
	fs.StringVar(&chartVersionsFile, "chart-versions-file", "", "path to charts/chart-versions.yaml")
	fs.StringVar(&chartDir, "chart", "", "chart directory (matrix.chart.directory), for the trace fallback")
	fs.StringVar(&shortSHA, "short-sha", "", "dev commit short SHA")
	fs.StringVar(&traceFile, "trace-file", "", "release-please --dry-run --trace log (stable releases scrape this)")
	fs.BoolVar(&failIfReleased, "fail-if-released", false, "hard-fail if the computed version already has a release git tag (set for workflow_dispatch builds)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if currentVersion == "" || chartVersion == "" || chartVersionsFile == "" || shortSHA == "" {
		return fmt.Errorf("--current-version, --chart-version, --chart-versions-file and --short-sha are required")
	}

	stillAlpha, err := releaseplease.StillAlpha(chartVersionsFile, chartVersion)
	if err != nil {
		return err
	}

	var trace string
	if traceFile != "" {
		b, err := os.ReadFile(traceFile)
		if err != nil {
			return fmt.Errorf("read trace file: %w", err)
		}
		trace = string(b)
	}

	r := releaseplease.Compute(currentVersion, stillAlpha, trace, chartDir, shortSHA)

	// Fail if the computed version already has a release tag: rebuilding it would
	// package and push over an immutable published release.
	if failIfReleased {
		tag := releaseplease.ReleaseTag(chartVersion, r.ReleaseVersion)
		existing, err := capture(context.Background(), "git", "tag", "-l", tag)
		if err != nil {
			return fmt.Errorf("check release tag %s: %w", tag, err)
		}
		if existing != "" {
			return fmt.Errorf("refusing to build: chart version %s is already released (git tag %s exists); a workflow_dispatch must not re-mint an immutable released version", r.ReleaseVersion, tag)
		}
	}

	env := newGitHubEnv()
	for _, kv := range [][2]string{
		{"RELEASE_VERSION", r.ReleaseVersion},
		{"IS_PRERELEASE", strconv.FormatBool(r.IsPrerelease)},
		{"RELEASE_VERSION_COMPUTED", strconv.FormatBool(r.Computed)},
		{"DEV_TAG", r.DevTag},
		{"CHART_MAJOR_VERSION", r.ChartMajor},
	} {
		if err := env.set(kv[0], kv[1]); err != nil {
			return err
		}
	}
	fmt.Fprintf(os.Stderr, "::notice::Computed version: %s, dev tag: %s, rolling tag: %s-dev-latest\n", r.ReleaseVersion, r.DevTag, r.ChartMajor)
	return nil
}
