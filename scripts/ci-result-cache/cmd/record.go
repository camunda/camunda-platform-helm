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
	recordSHA       string
	recordVersion   string
	recordShortname string
	recordFlow      string
	recordRepoRoot  string
	recordTargetURL string
)

func init() {
	recordCmd.Flags().StringVar(&recordSHA, "sha", "", "PR HEAD commit SHA to record the status on (required)")
	recordCmd.Flags().StringVar(&recordVersion, "version", "", "Chart version (e.g., 8.9) (required)")
	recordCmd.Flags().StringVar(&recordShortname, "shortname", "", "Scenario shortname (e.g., oske) (required)")
	recordCmd.Flags().StringVar(&recordFlow, "flow", "", "Flow name (e.g., install, upgrade-minor) (required)")
	recordCmd.Flags().StringVar(&recordRepoRoot, "repo-root", ".", "Repository root directory")
	recordCmd.Flags().StringVar(&recordTargetURL, "target-url", "", "URL to the CI run (optional, shown in GitHub UI)")

	_ = recordCmd.MarkFlagRequired("sha")
	_ = recordCmd.MarkFlagRequired("version")
	_ = recordCmd.MarkFlagRequired("shortname")
	_ = recordCmd.MarkFlagRequired("flow")
}

func runRecord(cmd *cobra.Command, args []string) error {
	contentHash, err := hash.Compute(recordRepoRoot, recordVersion)
	if err != nil {
		return fmt.Errorf("computing content hash: %w", err)
	}

	client, err := cache.NewGitHubClient()
	if err != nil {
		return fmt.Errorf("creating GitHub client: %w", err)
	}

	context := cache.StatusContext(recordVersion, recordShortname, recordFlow)
	description := cache.FormatDescription(contentHash, time.Now())

	if err := client.SetStatus(recordSHA, "success", context, description, recordTargetURL); err != nil {
		return fmt.Errorf("setting commit status: %w", err)
	}

	fmt.Printf("Recorded: %s = %s (hash: %s)\n", context, "success", contentHash[:12])
	return nil
}
