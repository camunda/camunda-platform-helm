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

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Check if a scenario result is cached and valid",
	Long: `Check verifies whether a scenario has a valid cached result by:

1. Computing the current content hash for the chart version
2. Looking up the commit status on the PR HEAD commit
3. Verifying the hash matches and the result is within TTL

Exit codes:
  0 — cached (valid result found)
  1 — not cached (no result, hash mismatch, or TTL expired)`,
	RunE: runCheck,
}

var (
	checkSHA       string
	checkVersion   string
	checkShortname string
	checkFlow      string
	checkRepoRoot  string
	checkTTL       time.Duration
)

func init() {
	checkCmd.Flags().StringVar(&checkSHA, "sha", "", "PR HEAD commit SHA to check (required)")
	checkCmd.Flags().StringVar(&checkVersion, "version", "", "Chart version (e.g., 8.9) (required)")
	checkCmd.Flags().StringVar(&checkShortname, "shortname", "", "Scenario shortname (e.g., oske) (required)")
	checkCmd.Flags().StringVar(&checkFlow, "flow", "", "Flow name (e.g., install, upgrade-minor) (required)")
	checkCmd.Flags().StringVar(&checkRepoRoot, "repo-root", ".", "Repository root directory")
	checkCmd.Flags().DurationVar(&checkTTL, "ttl", cache.DefaultTTL, "Maximum age of cached results (e.g., 24h, 12h, 0 to disable)")

	_ = checkCmd.MarkFlagRequired("sha")
	_ = checkCmd.MarkFlagRequired("version")
	_ = checkCmd.MarkFlagRequired("shortname")
	_ = checkCmd.MarkFlagRequired("flow")
}

func runCheck(cmd *cobra.Command, args []string) error {
	contentHash, err := hash.Compute(checkRepoRoot, checkVersion)
	if err != nil {
		return fmt.Errorf("computing content hash: %w", err)
	}

	client, err := cache.NewGitHubClient()
	if err != nil {
		return fmt.Errorf("creating GitHub client: %w", err)
	}

	statuses, err := client.GetStatuses(checkSHA)
	if err != nil {
		return fmt.Errorf("fetching statuses: %w", err)
	}

	context := cache.StatusContext(checkVersion, checkShortname, checkFlow)
	cached := cache.Check(statuses, context, contentHash, checkTTL)

	if cached {
		fmt.Printf("CACHED: %s (hash: %s)\n", context, contentHash[:12])
		return nil
	}

	fmt.Printf("NOT CACHED: %s (hash: %s)\n", context, contentHash[:12])
	return ErrNotCached
}
