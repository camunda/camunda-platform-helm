package cmd

import (
	"fmt"
	"strings"

	"scripts/ci-result-cache/pkg/cache"

	"github.com/spf13/cobra"
)

var invalidateCmd = &cobra.Command{
	Use:   "invalidate",
	Short: "Invalidate cached results for a scenario or version",
	Long: `Invalidate overwrites cached commit statuses with a "pending" state,
causing future cache checks to treat them as misses.

Granularity:
  --version + --shortname + --flow  →  invalidate one scenario
  --version                         →  invalidate all scenarios for that version
  (no filter)                       →  invalidate ALL cached results on the commit`,
	RunE: runInvalidate,
}

var (
	invalidateSHA       string
	invalidateVersion   string
	invalidateShortname string
	invalidateFlow      string
)

func init() {
	invalidateCmd.Flags().StringVar(&invalidateSHA, "sha", "", "PR HEAD commit SHA (required)")
	invalidateCmd.Flags().StringVar(&invalidateVersion, "version", "", "Chart version to invalidate (optional, narrows scope)")
	invalidateCmd.Flags().StringVar(&invalidateShortname, "shortname", "", "Scenario shortname to invalidate (optional)")
	invalidateCmd.Flags().StringVar(&invalidateFlow, "flow", "", "Flow to invalidate (optional)")

	_ = invalidateCmd.MarkFlagRequired("sha")
}

func runInvalidate(cmd *cobra.Command, args []string) error {
	client, err := cache.NewGitHubClient()
	if err != nil {
		return fmt.Errorf("creating GitHub client: %w", err)
	}

	statuses, err := client.GetStatuses(invalidateSHA)
	if err != nil {
		return fmt.Errorf("fetching statuses: %w", err)
	}

	// Build the context prefix to match.
	var contextFilter string
	switch {
	case invalidateVersion != "" && invalidateShortname != "" && invalidateFlow != "":
		contextFilter = cache.StatusContext(invalidateVersion, invalidateShortname, invalidateFlow)
	case invalidateVersion != "":
		contextFilter = fmt.Sprintf("ci-cache/%s/", invalidateVersion)
	default:
		contextFilter = "ci-cache/"
	}

	// Deduplicate by context — GitHub returns all status entries (append-only)
	// in reverse chronological order, so the same context may appear multiple
	// times. We only need to invalidate each unique context once (the new
	// "pending" status becomes the latest). We also skip contexts whose most
	// recent entry is already "pending" (already invalidated).
	seen := make(map[string]bool)
	invalidated := 0
	for _, s := range statuses {
		if seen[s.Context] {
			continue // already processed this context (either invalidated or already pending)
		}

		// Mark as seen on first encounter regardless of state.
		// Since GitHub returns in reverse chronological order, the first
		// entry for each context is the most recent.
		seen[s.Context] = true

		if s.State == "pending" {
			continue // most recent status is already invalidated
		}

		// For exact match (all three specified), match exactly.
		// For prefix match (version only or all), use HasPrefix.
		match := false
		if invalidateVersion != "" && invalidateShortname != "" && invalidateFlow != "" {
			match = s.Context == contextFilter
		} else {
			match = strings.HasPrefix(s.Context, contextFilter)
		}

		if !match {
			continue
		}

		if err := client.SetStatus(invalidateSHA, "pending", s.Context, "invalidated", ""); err != nil {
			return fmt.Errorf("invalidating %s: %w", s.Context, err)
		}
		invalidated++
		fmt.Printf("Invalidated: %s\n", s.Context)
	}

	if invalidated == 0 {
		fmt.Println("No matching cached results found to invalidate.")
	} else {
		fmt.Printf("Invalidated %d cached result(s).\n", invalidated)
	}

	return nil
}
