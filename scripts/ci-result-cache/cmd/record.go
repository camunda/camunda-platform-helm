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
	"fmt"
	"time"

	"scripts/ci-result-cache/pkg/cache"
	"scripts/ci-result-cache/pkg/hash"

	"github.com/spf13/cobra"
)

var recordCmd = &cobra.Command{
	Use:   "record",
	Short: "Record a passing scenario result as a commit status",
	Long: `Record writes a GitHub commit status to the PR HEAD commit indicating that
a scenario passed. The status includes a content hash and timestamp so that
future cache checks can verify validity.

The status is written to the PR HEAD commit (not the merge queue commit) so
it persists across queue ejections.`,
	RunE: runRecord,
}

var (
	recordSHA             string
	recordVersion         string
	recordShortname       string
	recordFlow            string
	recordPlatform        string
	recordRepoRoot        string
	recordChartVersions   string
	recordE2ESuiteVersion string
	recordRunnerImage     string
	recordTargetURL       string
)

func init() {
	recordCmd.Flags().StringVar(&recordSHA, "sha", "", "PR HEAD commit SHA to record the status on (required)")
	recordCmd.Flags().StringVar(&recordVersion, "version", "", "Chart version (e.g., 8.9) (required)")
	recordCmd.Flags().StringVar(&recordShortname, "shortname", "", "Scenario shortname (e.g., oske) (required)")
	recordCmd.Flags().StringVar(&recordFlow, "flow", "", "Flow name (e.g., install, upgrade-minor) (required)")
	recordCmd.Flags().StringVar(&recordPlatform, "platform", "", "Platform the scenario ran on (e.g., gke, eks) (required)")
	recordCmd.Flags().StringVar(&recordRepoRoot, "repo-root", ".", "Repository root directory")
	recordCmd.Flags().StringVar(&recordChartVersions, "chart-versions", "", "Comma-separated chart versions the scenario deploys (e.g., 8.10,8.9) (required)")
	recordCmd.Flags().StringVar(&recordE2ESuiteVersion, "e2e-test-suite-version", "", "@camunda/e2e-test-suite version the scenario runs (required)")
	recordCmd.Flags().StringVar(&recordRunnerImage, "playwright-runner-image", "", "Digest-pinned Playwright runner image the scenario ran (required)")
	recordCmd.Flags().StringVar(&recordTargetURL, "target-url", "", "URL to the CI run (optional, shown in GitHub UI)")

	_ = recordCmd.MarkFlagRequired("sha")
	_ = recordCmd.MarkFlagRequired("version")
	_ = recordCmd.MarkFlagRequired("shortname")
	_ = recordCmd.MarkFlagRequired("flow")
	_ = recordCmd.MarkFlagRequired("platform")
	_ = recordCmd.MarkFlagRequired("chart-versions")
	_ = recordCmd.MarkFlagRequired("e2e-test-suite-version")
	_ = recordCmd.MarkFlagRequired("playwright-runner-image")
}

func runRecord(cmd *cobra.Command, args []string) error {
	contentHash, err := hash.Compute(recordRepoRoot, hash.ChartVersionsFromCSV(recordChartVersions), recordE2ESuiteVersion, recordRunnerImage)
	if err != nil {
		return fmt.Errorf("computing content hash: %w", err)
	}

	client, err := cache.NewGitHubClient()
	if err != nil {
		return fmt.Errorf("creating GitHub client: %w", err)
	}

	context := cache.StatusContext(recordVersion, recordShortname, recordFlow, recordPlatform)
	description := cache.FormatDescription(contentHash, time.Now())

	if err := client.SetStatus(recordSHA, "success", context, description, recordTargetURL); err != nil {
		return fmt.Errorf("setting commit status: %w", err)
	}

	fmt.Printf("Recorded: %s = %s (hash: %s)\n", context, "success", contentHash[:12])
	return nil
}
