package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "ci-result-cache",
	Short: "Progressive CI result caching for merge queue optimization",
	Long: `ci-result-cache manages cached CI test results using GitHub commit statuses.

It enables progressive accumulation of test results across merge queue attempts:
scenarios that already passed (for the same content hash) are skipped on re-queue,
saving CI time and reducing flaky failure impact.

Commands:
  record           Record a passing scenario result as a commit status
  check            Check if a scenario result is cached and valid
  invalidate       Invalidate cached results for a scenario or version
  annotate-matrix  Annotate a CI matrix JSON with cached/uncached flags`,
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(recordCmd)
	rootCmd.AddCommand(checkCmd)
	rootCmd.AddCommand(invalidateCmd)
	rootCmd.AddCommand(annotateMatrixCmd)
}
