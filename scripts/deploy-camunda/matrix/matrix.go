package matrix

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"scripts/camunda-core/pkg/logging"
)

// Entry represents a single matrix entry — one scenario + one flow + one platform combination.
type Entry struct {
	Version   string   `json:"version"`
	ChartPath string   `json:"chartPath"`
	Scenario  string   `json:"scenario"`
	Shortname string   `json:"shortname"`
	Auth      string   `json:"auth"`
	Flow      string   `json:"flow"`
	Platform  string   `json:"platform,omitempty"`
	Exclude   []string `json:"exclude,omitempty"`
	Enabled   bool     `json:"enabled"`
}

// GenerateOptions controls matrix generation.
type GenerateOptions struct {
	// Versions limits the matrix to specific versions. Empty means all active versions.
	Versions []string
	// IncludeDisabled includes disabled scenarios in the output.
	IncludeDisabled bool
}

// FilterOptions controls post-generation filtering.
type FilterOptions struct {
	// ScenarioFilter limits output to scenarios matching one or more substrings (comma-separated).
	ScenarioFilter string
	// FlowFilter limits output to entries with this specific flow.
	FlowFilter string
	// Platform limits output to entries targeting this platform.
	Platform string
}

// Generate builds the full test matrix from CI config files.
// It reads chart-versions.yaml, each version's ci-test-config.yaml, and permitted-flows.yaml,
// then explodes comma-separated flows into separate entries and filters denied flows.
func Generate(repoRoot string, opts GenerateOptions) ([]Entry, error) {
	// Load chart versions
	cv, err := LoadChartVersions(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("loading chart versions: %w", err)
	}

	// Load permitted flows
	pf, err := LoadPermittedFlows(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("loading permitted flows: %w", err)
	}

	// Determine which versions to process
	activeVersions := cv.ActiveVersions()
	var versions []string
	if len(opts.Versions) > 0 {
		// Validate requested versions are active
		activeSet := make(map[string]bool)
		for _, v := range activeVersions {
			activeSet[v] = true
		}
		for _, v := range opts.Versions {
			if !activeSet[v] {
				return nil, fmt.Errorf("requested version %q is not active (active: %v)", v, activeVersions)
			}
			versions = append(versions, v)
		}
	} else {
		versions = activeVersions
	}

	var entries []Entry

	for _, version := range versions {
		chartDir := filepath.Join(repoRoot, "charts", fmt.Sprintf("camunda-platform-%s", version))

		cfg, err := LoadCITestConfig(chartDir)
		if err != nil {
			logging.Logger.Warn().
				Str("version", version).
				Err(err).
				Msg("Skipping version — failed to load ci-test-config.yaml")
			continue
		}

		// Only PR scenarios
		for _, scenario := range cfg.Integration.Case.PR.Scenarios {
			// Skip disabled unless requested
			if !scenario.Enabled && !opts.IncludeDisabled {
				continue
			}

			// Default flow to "install" if not specified
			flowStr := strings.TrimSpace(scenario.Flow)
			if flowStr == "" {
				flowStr = "install"
			}

			// Explode comma-separated flows
			rawFlows := strings.Split(flowStr, ",")
			var flows []string
			for _, f := range rawFlows {
				f = strings.TrimSpace(f)
				if f != "" {
					flows = append(flows, f)
				}
			}

			// Apply permitted-flows filtering
			permittedFlows := FilterFlows(pf, version, flows)
			if len(permittedFlows) == 0 {
				logging.Logger.Debug().
					Str("version", version).
					Str("scenario", scenario.Name).
					Str("shortname", scenario.Shortname).
					Strs("originalFlows", flows).
					Msg("All flows denied by permitted-flows rules — skipping scenario")
				continue
			}

			// Create one entry per permitted flow per platform.
			// If no platforms are specified, create one entry with an empty platform
			// (defaults to "gke" at execution time via resolvePlatform).
			platforms := scenario.Platforms
			if len(platforms) == 0 {
				platforms = []string{""}
			}

			for _, flow := range permittedFlows {
				for _, platform := range platforms {
					entries = append(entries, Entry{
						Version:   version,
						ChartPath: chartDir,
						Scenario:  scenario.Name,
						Shortname: scenario.Shortname,
						Auth:      scenario.Auth,
						Flow:      flow,
						Platform:  platform,
						Exclude:   scenario.Exclude,
						Enabled:   scenario.Enabled,
					})
				}
			}
		}
	}

	return entries, nil
}

// Filter applies post-generation filtering to the matrix entries.
func Filter(entries []Entry, opts FilterOptions) []Entry {
	if opts.ScenarioFilter == "" && opts.FlowFilter == "" && opts.Platform == "" {
		return entries
	}

	// Parse comma-separated scenario filters into individual substrings.
	var scenarioFilters []string
	if opts.ScenarioFilter != "" {
		for _, f := range strings.Split(opts.ScenarioFilter, ",") {
			if t := strings.TrimSpace(f); t != "" {
				scenarioFilters = append(scenarioFilters, t)
			}
		}
	}

	var filtered []Entry
	for _, e := range entries {
		if len(scenarioFilters) > 0 && !matchesAny(e.Scenario, scenarioFilters) {
			continue
		}
		if opts.FlowFilter != "" && e.Flow != opts.FlowFilter {
			continue
		}
		if opts.Platform != "" {
			if e.Platform != "" && e.Platform != opts.Platform {
				continue
			}
		}
		filtered = append(filtered, e)
	}
	return filtered
}

// matchesAny reports whether s contains any of the given substrings.
func matchesAny(s string, substrings []string) bool {
	for _, sub := range substrings {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// Print outputs the matrix entries in the requested format.
func Print(entries []Entry, format string) (string, error) {
	switch format {
	case "json":
		return printJSON(entries)
	case "table":
		return printTable(entries), nil
	default:
		return "", fmt.Errorf("unknown format %q (supported: table, json)", format)
	}
}

// printJSON returns the entries as a JSON array.
func printJSON(entries []Entry) (string, error) {
	if entries == nil {
		entries = []Entry{}
	}
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal matrix to JSON: %w", err)
	}
	return string(data), nil
}

// printTable returns the entries formatted as an ASCII table.
func printTable(entries []Entry) string {
	if len(entries) == 0 {
		return "No matrix entries found."
	}

	var b strings.Builder

	// Header
	fmt.Fprintf(&b, "%-8s %-6s %-25s %-10s %-10s %-16s %-12s %-8s\n",
		"ENABLED", "VER", "SCENARIO", "SHORT", "AUTH", "FLOW", "PLATFORM", "EXCLUDE")
	fmt.Fprintf(&b, "%-8s %-6s %-25s %-10s %-10s %-16s %-12s %-8s\n",
		"-------", "---", "--------", "-----", "----", "----", "--------", "-------")

	for _, e := range entries {
		enabled := "yes"
		if !e.Enabled {
			enabled = "no"
		}
		platform := e.Platform
		if platform == "" {
			platform = "-"
		}
		exclude := strings.Join(e.Exclude, ",")
		if exclude == "" {
			exclude = "-"
		}
		fmt.Fprintf(&b, "%-8s %-6s %-25s %-10s %-10s %-16s %-12s %s\n",
			enabled, e.Version, e.Scenario, e.Shortname, e.Auth, e.Flow, platform, exclude)
	}

	fmt.Fprintf(&b, "\nTotal: %d entries\n", len(entries))
	return b.String()
}

// GroupByVersion groups entries by their version, preserving order.
func GroupByVersion(entries []Entry) map[string][]Entry {
	groups := make(map[string][]Entry)
	for _, e := range entries {
		groups[e.Version] = append(groups[e.Version], e)
	}
	return groups
}

// VersionOrder returns the unique versions from entries in the order they appear.
func VersionOrder(entries []Entry) []string {
	seen := make(map[string]bool)
	var order []string
	for _, e := range entries {
		if !seen[e.Version] {
			seen[e.Version] = true
			order = append(order, e.Version)
		}
	}
	return order
}
