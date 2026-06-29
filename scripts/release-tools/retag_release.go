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
	"flag"
	"fmt"
	"os"
	"strings"

	"scripts/camunda-core/pkg/retagger"
)

// runRetagRelease moves each chart's release Git tag to targetSHA when the
// existing tag points to a different commit — closing the window between
// helm-cr tag creation and the release-please Chart.yaml version bump.
//
//	retag-release --repo <owner/name> --sha <commit-sha> [--charts-root <path>]
func runRetagRelease(args []string) error {
	fs := flag.NewFlagSet("retag-release", flag.ContinueOnError)
	var (
		repo       string
		sha        string
		chartsRoot string
	)
	fs.StringVar(&repo, "repo", "", "GitHub repo slug (owner/name)")
	fs.StringVar(&sha, "sha", "", "target commit SHA (the release-please merge commit)")
	fs.StringVar(&chartsRoot, "charts-root", ".", "repo root containing charts/")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if repo == "" || sha == "" {
		return fmt.Errorf("--repo and --sha are required")
	}

	client := retagger.NewGitHubClient()
	results, err := retagger.Run(chartsRoot, repo, sha, client)
	if err != nil {
		return err
	}
	return writeRetagSummary(results, sha)
}

func writeRetagSummary(results []retagger.Result, targetSHA string) error {
	var moved, skipped []string
	for _, r := range results {
		if r.Moved {
			moved = append(moved, r.TagName)
		} else {
			skipped = append(skipped, fmt.Sprintf("%s (%s)", r.TagName, r.Reason))
		}
	}

	var sb strings.Builder
	sb.WriteString("## 🏷️ Release Tag Sync\n")
	if len(moved) > 0 {
		shortSHA := targetSHA
		if len(shortSHA) > 7 {
			shortSHA = shortSHA[:7]
		}
		fmt.Fprintf(&sb, "### ✅ Moved to `%s`\n", shortSHA)
		for _, t := range moved {
			fmt.Fprintf(&sb, "- `%s`\n", t)
		}
	}
	if len(skipped) > 0 {
		sb.WriteString("### ⏭️ Skipped\n")
		for _, t := range skipped {
			fmt.Fprintf(&sb, "- %s\n", t)
		}
	}
	summary := sb.String()

	// Always print to stdout for local visibility.
	fmt.Print(summary)

	summaryPath := os.Getenv("GITHUB_STEP_SUMMARY")
	if summaryPath == "" {
		return nil
	}
	f, err := os.OpenFile(summaryPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		return fmt.Errorf("open GITHUB_STEP_SUMMARY: %w", err)
	}
	defer f.Close()
	if _, err := f.WriteString(summary); err != nil {
		return fmt.Errorf("write GITHUB_STEP_SUMMARY: %w", err)
	}
	return nil
}
