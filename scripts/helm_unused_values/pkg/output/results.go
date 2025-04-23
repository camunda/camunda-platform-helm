package output

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"camunda.com/helm-unused-values/pkg/keys"
	"camunda.com/helm-unused-values/pkg/utils"
	"github.com/fatih/color"
)

// ResultSummary represents the analysis results summary
type ResultSummary struct {
	TotalKeys            int `json:"total_keys"`
	UsedKeys             int `json:"used_keys"`
	UnusedKeys           int `json:"unused_keys"`
	UnusedParentKeys     int `json:"unused_parent_keys"`
	UnusedCompletelyKeys int `json:"unused_completely_keys"`
}

// JSONResult represents the JSON output format
type JSONResult struct {
	Timestamp            string        `json:"timestamp"`
	Summary              ResultSummary `json:"summary"`
	UnusedParentKeys     []string      `json:"unused_parent_keys"`
	UnusedCompletelyKeys []string      `json:"unused_completely_keys"`
}

// Reporter handles reporting the analysis results
type Reporter struct {
	Display          *Display
	JSONOutput       bool
	OutputFile       string
	ShowAllKeys      bool
	ShowTestCommands bool
}

// NewReporter creates a new reporter
func NewReporter(display *Display, jsonOutput bool, outputFile string, showAllKeys bool, showTestCommands bool) *Reporter {
	return &Reporter{
		Display:          display,
		JSONOutput:       jsonOutput,
		OutputFile:       outputFile,
		ShowAllKeys:      showAllKeys,
		ShowTestCommands: showTestCommands,
	}
}

// ReportResults reports the analysis results based on the configured format
func (r *Reporter) ReportResults(usages []keys.KeyUsage) error {
	// Calculate summary
	summary := r.calculateSummary(usages)

	// Get keys by usage type
	unusedKeys := filterByUsageType(usages, "unused")
	parentKeys := filterByUsageType(usages, "parent")
	directKeys := filterByUsageType(usages, "direct")
	patternKeys := filterByUsageType(usages, "pattern")

	if r.JSONOutput {
		if r.ShowAllKeys {
			// For JSON output with showAllKeys, include all keys by type
			return r.reportAllKeysJSONResults(summary, directKeys, patternKeys, parentKeys, unusedKeys)
		}
		return r.reportJSONResults(summary, parentKeys, unusedKeys)
	}

	if r.ShowAllKeys {
		// Show all keys, not just unused ones
		return r.reportAllKeysTextResults(summary, directKeys, patternKeys, parentKeys, unusedKeys, usages)
	}

	return r.reportTextResults(summary, unusedKeys, usages)
}

// calculateSummary calculates the result summary
func (r *Reporter) calculateSummary(usages []keys.KeyUsage) ResultSummary {
	var summary ResultSummary

	summary.TotalKeys = len(usages)

	// Count keys by usage type
	for _, usage := range usages {
		switch usage.UsageType {
		case "direct", "pattern":
			summary.UsedKeys++
		case "parent":
			summary.UnusedParentKeys++
		case "unused":
			summary.UnusedCompletelyKeys++
		}
	}

	summary.UnusedKeys = summary.UnusedParentKeys + summary.UnusedCompletelyKeys

	return summary
}

func (r *Reporter) reportJSONResults(summary ResultSummary, parentKeys, unusedKeys []string) error {
	jsonOutput := JSONResult{
		Timestamp:            time.Now().UTC().Format(time.RFC3339),
		Summary:              summary,
		UnusedParentKeys:     parentKeys,
		UnusedCompletelyKeys: unusedKeys,
	}

	jsonData, err := json.MarshalIndent(jsonOutput, "", "  ")
	if err != nil {
		return fmt.Errorf("error generating JSON: %w", err)
	}

	if r.OutputFile != "" {
		if err := os.WriteFile(r.OutputFile, jsonData, 0644); err != nil {
			return fmt.Errorf("error writing to output file: %w", err)
		}
	} else {
		fmt.Println(string(jsonData))
	}

	return nil
}

func (r *Reporter) reportTextResults(summary ResultSummary, unusedKeys []string, usages []keys.KeyUsage) error {
	usageMap := make(map[string]keys.KeyUsage)
	for _, usage := range usages {
		usageMap[usage.Key] = usage
	}

	if summary.UnusedKeys == 0 {
		r.Display.PrintSuccess("No unused keys found in values.yaml.")
		return nil
	}

	r.Display.PrintError("Unused keys found in values.yaml:")
	fmt.Println()

	usedKeys := []string{}
	for _, usage := range usages {
		if usage.UsageType == "direct" || usage.UsageType == "pattern" {
			usedKeys = append(usedKeys, usage.Key)
		}
	}
	sort.Strings(usedKeys)

	if len(usedKeys) > 0 {
		r.Display.PrintBold(fmt.Sprintf("Used keys (%d):", len(usedKeys)))
		for _, key := range usedKeys {
			usage := usageMap[key]

			fmt.Printf("  ")
			green := color.New(color.FgGreen)

			switch usage.UsageType {
			case "direct":
				green.Printf(".Values.%s", key)
				if len(usage.Locations) > 0 {
					location := usage.Locations[0]
					parts := strings.Split(location, ":")
					if len(parts) >= 2 {
						fmt.Printf(" → ")
						cyan := color.New(color.FgCyan)
						cyan.Printf("%s", parts[0])

						bold := color.New(color.Bold)
						bold.Printf(":%s", parts[1])

						// Show location count if more than one
						if len(usage.Locations) > 1 {
							fmt.Printf(" (+%d more)", len(usage.Locations)-1)
						}
					}
				}
			case "pattern":
				green.Printf(".Values.%s ", key)

				cyan := color.New(color.FgCyan)
				cyan.Printf("(via %s)", usage.PatternName)

				// Show the first location if available
				if len(usage.Locations) > 0 {
					location := usage.Locations[0]
					parts := strings.Split(location, ":")
					if len(parts) >= 2 {
						fmt.Printf(" → ")
						// Handle placeholder values that might not be real file:line
						if strings.HasPrefix(parts[0], "[PATTERN MATCH]") {
							// This is a placeholder from a regex match
							magenta := color.New(color.FgMagenta)
							magenta.Printf("Pattern match detected")
						} else {
							// Regular file:line match
							cyan.Printf("%s", parts[0])

							bold := color.New(color.Bold)
							bold.Printf(":%s", parts[1])
						}

						// Show location count if more than one
						if len(usage.Locations) > 1 {
							fmt.Printf(" (+%d more)", len(usage.Locations)-1)
						}
					} else {
						fmt.Printf(" → %s", location)
					}
				}
			}
			fmt.Println()
		}
		fmt.Println()
	}

	if summary.UnusedCompletelyKeys > 0 {
		r.Display.PrintBold(fmt.Sprintf("Completely unused keys (%d):", summary.UnusedCompletelyKeys))
		for _, key := range unusedKeys {
			// Display test command if requested
			if r.ShowTestCommands {
				// Create a pattern that would find the key
				escapedKey := strings.ReplaceAll(key, ".", "\\.")
				pattern := fmt.Sprintf("\\.Values\\.%s", escapedKey)

				// Generate the command
				var cmd string
				if utils.DetectRipgrep() {
					cmd = fmt.Sprintf("rg -F \"%s\" --no-heading --with-filename --line-number templates/", pattern)
				} else {
					cmd = fmt.Sprintf("grep -r -n -F \"%s\" templates/", pattern)
				}

				fmt.Printf("  ")
				red := color.New(color.FgRed)
				red.Printf(".Values.%s", key)

				fmt.Printf("  ")
				gray := color.New(color.FgHiBlack)
				gray.Printf("(Test with: %s)", cmd)
				fmt.Println()
			} else {
				r.Display.PrintError(fmt.Sprintf("  .Values.%s", key))
			}
		}
		fmt.Println()
	}

	r.Display.PrintBold("Usage summary:")
	fmt.Printf("  ")
	cyan := color.New(color.FgCyan)
	cyan.Printf("Total keys: %d", summary.TotalKeys)

	fmt.Printf("  |  ")
	green := color.New(color.FgGreen)
	green.Printf("Used: %d", summary.UsedKeys)

	fmt.Printf("  |  ")
	yellow := color.New(color.FgYellow)
	yellow.Printf("Parent: %d", summary.UnusedParentKeys)

	fmt.Printf("  |  ")
	red := color.New(color.FgRed)
	red.Printf("Unused: %d", summary.UnusedCompletelyKeys)
	fmt.Println()
	fmt.Println()

	return nil
}

func filterByUsageType(usages []keys.KeyUsage, usageType string) []string {
	var keys []string

	for _, usage := range usages {
		if usage.UsageType == usageType {
			keys = append(keys, usage.Key)
		}
	}

	return keys
}

func FilterByUsageType(usages []keys.KeyUsage, usageType string) []string {
	var keys []string

	for _, usage := range usages {
		if usage.UsageType == usageType {
			keys = append(keys, usage.Key)
		}
	}

	return keys
}

// reportAllKeysJSONResults outputs all keys (used and unused) in JSON format
func (r *Reporter) reportAllKeysJSONResults(summary ResultSummary, directKeys, patternKeys, parentKeys, unusedKeys []string) error {
	// Enhanced JSON format to include all key types
	type AllKeysJSONResult struct {
		Timestamp            string        `json:"timestamp"`
		Summary              ResultSummary `json:"summary"`
		DirectlyUsedKeys     []string      `json:"directly_used_keys"`
		PatternUsedKeys      []string      `json:"pattern_used_keys"`
		UnusedParentKeys     []string      `json:"unused_parent_keys"`
		UnusedCompletelyKeys []string      `json:"unused_completely_keys"`
	}

	jsonOutput := AllKeysJSONResult{
		Timestamp:            time.Now().UTC().Format(time.RFC3339),
		Summary:              summary,
		DirectlyUsedKeys:     directKeys,
		PatternUsedKeys:      patternKeys,
		UnusedParentKeys:     parentKeys,
		UnusedCompletelyKeys: unusedKeys,
	}

	jsonData, err := json.MarshalIndent(jsonOutput, "", "  ")
	if err != nil {
		return fmt.Errorf("error generating JSON: %w", err)
	}

	if r.OutputFile != "" {
		if err := os.WriteFile(r.OutputFile, jsonData, 0644); err != nil {
			return fmt.Errorf("error writing to output file: %w", err)
		}
	} else {
		r.Display.PrintInfo(string(jsonData))
	}

	return nil
}

func (r *Reporter) reportAllKeysTextResults(summary ResultSummary, directKeys, patternKeys, parentKeys, unusedKeys []string, usages []keys.KeyUsage) error {
	// Build a map for faster lookups
	usageMap := make(map[string]keys.KeyUsage)
	for _, usage := range usages {
		usageMap[usage.Key] = usage
	}

	fmt.Println()
	r.Display.PrintInfo("All keys in values.yaml:")
	fmt.Println()

	if len(directKeys) > 0 {
		r.Display.PrintBold(fmt.Sprintf("Directly used keys (%d):", len(directKeys)))
		for _, key := range directKeys {
			// Format the locations inline if available
			if usage, ok := usageMap[key]; ok && len(usage.Locations) > 0 {
				// Show only the first location for brevity
				location := usage.Locations[0]
				parts := strings.Split(location, ":")
				if len(parts) >= 2 {
					fmt.Printf("  ")
					green := color.New(color.FgGreen)
					green.Printf(".Values.%s ", key)

					cyan := color.New(color.FgCyan)
					cyan.Printf("→ %s", parts[0])

					bold := color.New(color.Bold)
					bold.Printf(":%s", parts[1])

					// Show location count if more than one
					if len(usage.Locations) > 1 {
						fmt.Printf(" (+%d more)", len(usage.Locations)-1)
					}
					fmt.Println()
				} else {
					r.Display.PrintSuccess(fmt.Sprintf("  .Values.%s → %s", key, location))
				}
			} else {
				r.Display.PrintSuccess(fmt.Sprintf("  .Values.%s", key))
			}
		}
		fmt.Println()
	}

	if len(patternKeys) > 0 {
		r.Display.PrintBold(fmt.Sprintf("Keys used via patterns (%d):", len(patternKeys)))
		for _, key := range patternKeys {
			if usage, ok := usageMap[key]; ok {
				fmt.Printf("  ")
				green := color.New(color.FgGreen)
				green.Printf(".Values.%s ", key)

				cyan := color.New(color.FgCyan)
				cyan.Printf("(via %s)", usage.PatternName)

				// Show the first location if available
				if len(usage.Locations) > 0 {
					location := usage.Locations[0]
					parts := strings.Split(location, ":")
					if len(parts) >= 2 {
						fmt.Printf(" → ")
						// Handle placeholder values that might not be real file:line
						if strings.HasPrefix(parts[0], "[PATTERN MATCH]") {
							// This is a placeholder from a regex match
							magenta := color.New(color.FgMagenta)
							magenta.Printf("Pattern match detected")
						} else {
							// Regular file:line match
							cyan.Printf("%s", parts[0])

							bold := color.New(color.Bold)
							bold.Printf(":%s", parts[1])
						}

						// Show location count if more than one
						if len(usage.Locations) > 1 {
							fmt.Printf(" (+%d more)", len(usage.Locations)-1)
						}
					} else {
						fmt.Printf(" → %s", location)
					}
				}
				fmt.Println()
			} else {
				r.Display.PrintSuccess(fmt.Sprintf("  .Values.%s", key))
			}
		}
		fmt.Println()
	}

	if len(parentKeys) > 0 {
		r.Display.PrintBold(fmt.Sprintf("Parents of used keys (%d):", len(parentKeys)))
		for _, key := range parentKeys {
			if usage, ok := usageMap[key]; ok && len(usage.ChildKeys) > 0 {
				fmt.Printf("  ")
				yellow := color.New(color.FgYellow)
				yellow.Printf(".Values.%s ", key)

				// Show child count
				cyan := color.New(color.FgCyan)
				cyan.Printf("(has %d child keys)", len(usage.ChildKeys))

				// Show the first child for context
				if len(usage.ChildKeys) > 0 {
					fmt.Printf(" e.g., ")
					cyan.Printf(".Values.%s", usage.ChildKeys[0])
				}
				fmt.Println()
			} else {
				r.Display.PrintWarning(fmt.Sprintf("  .Values.%s", key))
			}
		}
		fmt.Println()
	}

	if len(unusedKeys) > 0 {
		r.Display.PrintBold(fmt.Sprintf("Completely unused keys (%d):", len(unusedKeys)))
		for _, key := range unusedKeys {
			// Display test command if requested
			if r.ShowTestCommands {
				// Create a pattern that would find the key
				escapedKey := strings.ReplaceAll(key, ".", "\\.")
				pattern := fmt.Sprintf("\\.Values\\.%s", escapedKey)

				// Generate the command
				var cmd string
				if utils.DetectRipgrep() {
					cmd = fmt.Sprintf("rg -F \"%s\" --no-heading --with-filename --line-number templates/", pattern)
				} else {
					cmd = fmt.Sprintf("grep -r -n -F \"%s\" templates/", pattern)
				}

				fmt.Printf("  ")
				red := color.New(color.FgRed)
				red.Printf(".Values.%s", key)

				fmt.Printf("  ")
				gray := color.New(color.FgHiBlack)
				gray.Printf("(Test with: %s)", cmd)
				fmt.Println()
			} else {
				r.Display.PrintError(fmt.Sprintf("  .Values.%s", key))
			}
		}
		fmt.Println()
	}

	// Display summary with improved formatting
	r.Display.PrintBold("Usage summary:")
	fmt.Printf("  ")
	cyan := color.New(color.FgCyan)
	cyan.Printf("Total keys: %d", summary.TotalKeys)

	fmt.Printf("  |  ")
	green := color.New(color.FgGreen)
	green.Printf("Used: %d", summary.UsedKeys)

	fmt.Printf("  |  ")
	yellow := color.New(color.FgYellow)
	yellow.Printf("Parent: %d", summary.UnusedParentKeys)

	fmt.Printf("  |  ")
	red := color.New(color.FgRed)
	red.Printf("Unused: %d", summary.UnusedCompletelyKeys)
	fmt.Println()
	fmt.Println()

	return nil
}
