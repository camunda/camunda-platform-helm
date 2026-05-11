package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"scripts/ci-result-cache/pkg/cache"
	"scripts/ci-result-cache/pkg/hash"

	"github.com/spf13/cobra"
)

var annotateMatrixCmd = &cobra.Command{
	Use:   "annotate-matrix",
	Short: "Annotate a CI matrix JSON with cached/uncached flags",
	Long: `Annotate-matrix reads the matrix JSON from stdin (as produced by
generate-chart-matrix.sh), checks each entry against cached results,
and outputs an annotated matrix with a "cached" field on each entry.

Entries where cached=true can be routed to a fast-path job that skips
the full GKE deploy+test cycle.

Usage:
  generate-chart-matrix.sh ... | ci-result-cache annotate-matrix --sha <SHA> --repo-root .`,
	RunE: runAnnotateMatrix,
}

var (
	annotateSHA      string
	annotateRepoRoot string
	annotateTTL      time.Duration
)

func init() {
	annotateMatrixCmd.Flags().StringVar(&annotateSHA, "sha", "", "PR HEAD commit SHA to check cached results against (required)")
	annotateMatrixCmd.Flags().StringVar(&annotateRepoRoot, "repo-root", ".", "Repository root directory")
	annotateMatrixCmd.Flags().DurationVar(&annotateTTL, "ttl", cache.DefaultTTL, "Maximum age of cached results")

	_ = annotateMatrixCmd.MarkFlagRequired("sha")
}

// matrixJSON represents the top-level matrix structure: {"include": [...]}
type matrixJSON struct {
	Include []matrixEntry `json:"include"`
}

// matrixEntry represents one entry in the matrix. We preserve all fields
// using json.RawMessage and only add/modify the "cached" field.
type matrixEntry map[string]interface{}

func runAnnotateMatrix(cmd *cobra.Command, args []string) error {
	// Read matrix JSON from stdin.
	var matrix matrixJSON
	decoder := json.NewDecoder(os.Stdin)
	if err := decoder.Decode(&matrix); err != nil {
		return fmt.Errorf("reading matrix JSON from stdin: %w", err)
	}

	if len(matrix.Include) == 0 {
		// Empty matrix — pass through unchanged.
		return json.NewEncoder(os.Stdout).Encode(matrix)
	}

	// Create GitHub client.
	client, err := cache.NewGitHubClient()
	if err != nil {
		// If we can't connect to GitHub (e.g., running locally), pass through uncached.
		fmt.Fprintf(os.Stderr, "Warning: cannot create GitHub client (%v), marking all entries as uncached\n", err)
		for i := range matrix.Include {
			matrix.Include[i]["cached"] = "false"
		}
		return json.NewEncoder(os.Stdout).Encode(matrix)
	}

	// Fetch all ci-cache statuses for this SHA (one API call).
	statuses, err := client.GetStatuses(annotateSHA)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: cannot fetch statuses (%v), marking all entries as uncached\n", err)
		for i := range matrix.Include {
			matrix.Include[i]["cached"] = "false"
		}
		return json.NewEncoder(os.Stdout).Encode(matrix)
	}

	// Pre-compute content hashes per version (avoid recomputing for each entry).
	hashCache := make(map[string]string)

	cachedCount := 0
	for i, entry := range matrix.Include {
		version, _ := entry["version"].(string)
		shortname, _ := entry["shortname"].(string)
		flow, _ := entry["flow"].(string)

		if version == "" || shortname == "" || flow == "" {
			matrix.Include[i]["cached"] = "false"
			continue
		}

		// Get or compute the content hash for this version.
		contentHash, ok := hashCache[version]
		if !ok {
			var hashErr error
			contentHash, hashErr = hash.Compute(annotateRepoRoot, version)
			if hashErr != nil {
				fmt.Fprintf(os.Stderr, "Warning: cannot compute hash for version %s (%v), marking as uncached\n", version, hashErr)
				matrix.Include[i]["cached"] = "false"
				continue
			}
			hashCache[version] = contentHash
		}

		context := cache.StatusContext(version, shortname, flow)
		if cache.Check(statuses, context, contentHash, annotateTTL) {
			matrix.Include[i]["cached"] = "true"
			cachedCount++
		} else {
			matrix.Include[i]["cached"] = "false"
		}
	}

	fmt.Fprintf(os.Stderr, "Cache check: %d/%d entries cached\n", cachedCount, len(matrix.Include))

	// Output annotated matrix as compact JSON.
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(matrix)
}
