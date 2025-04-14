package values

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/schollz/progressbar/v3"
)

// Extractor handles extracting and filtering keys from values.yaml
type Extractor struct {
	Debug bool
}

// NewExtractor creates a new keys extractor
func NewExtractor(debug bool) *Extractor {
	return &Extractor{
		Debug: debug,
	}
}

// ExtractKeys extracts all keys from the values.yaml file
func (e *Extractor) ExtractKeys(valuesFile string) ([]string, error) {
	// Validate input
	if _, err := os.Stat(valuesFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("values file %s not found: %w", valuesFile, err)
	}

	// Use yq to convert YAML to JSON
	yqCmd := exec.Command("yq", "eval", valuesFile, "-o", "json")
	yqOutput, err := yqCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run yq: %w", err)
	}

	// Use jq to extract paths
	jqCmd := exec.Command("jq", "[paths(scalars) as $p | {($p | join(\".\")): getpath($p)}] | add | keys[]")
	jqCmd.Stdin = strings.NewReader(string(yqOutput))
	jqOutput, err := jqCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to parse JSON with jq: %w", err)
	}

	// Process the output
	var keys []string
	scanner := bufio.NewScanner(strings.NewReader(string(jqOutput)))
	for scanner.Scan() {
		key := scanner.Text()
		// Remove quotes from the key
		key = strings.Trim(key, "\"")
		if key != "" {
			keys = append(keys, key)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading jq output: %w", err)
	}

	if len(keys) == 0 {
		return nil, fmt.Errorf("no keys extracted from values.yaml")
	}

	if e.Debug {
		fmt.Printf("Extracted %d keys from values.yaml\n", len(keys))
	}

	return keys, nil
}

// FilterKeys filters the keys array based on the filter pattern
func (e *Extractor) FilterKeys(keys []string, pattern string) []string {
	if pattern == "" {
		return keys
	}

	var filtered []string
	for _, key := range keys {
		if strings.Contains(key, pattern) {
			filtered = append(filtered, key)
		}
	}

	if e.Debug {
		fmt.Printf("Filtered keys: %d/%d match pattern '%s'\n",
			len(filtered), len(keys), pattern)
	}

	return filtered
}

// ValidateFile checks if a file exists and is readable
func ValidateFile(file string) error {
	info, err := os.Stat(file)
	if err != nil {
		return fmt.Errorf("file %s not found: %w", file, err)
	}
	if info.IsDir() {
		return fmt.Errorf("%s is a directory, not a file", file)
	}
	return nil
}

// ValidateDirectory checks if a directory exists and is readable
func ValidateDirectory(dir string) error {
	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("directory %s not found: %w", dir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", dir)
	}
	return nil
}

// ExtractKeysWithProgress extracts all keys from the values.yaml file with progress reporting
func (e *Extractor) ExtractKeysWithProgress(valuesFile string, bar *progressbar.ProgressBar) ([]string, error) {
	// Validate input
	if _, err := os.Stat(valuesFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("values file %s not found: %w", valuesFile, err)
	}

	// Update progress bar description
	bar.Describe("Running yq to parse YAML...")

	// Use yq to convert YAML to JSON
	yqCmd := exec.Command("yq", "eval", valuesFile, "-o", "json")
	yqOutput, err := yqCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run yq: %w", err)
	}

	// Update progress
	bar.Describe("Extracting paths with jq...")

	// Use jq to extract paths
	jqCmd := exec.Command("jq", "[paths(scalars) as $p | {($p | join(\".\")): getpath($p)}] | add | keys[]")
	jqCmd.Stdin = strings.NewReader(string(yqOutput))
	jqOutput, err := jqCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to parse JSON with jq: %w", err)
	}

	// Update progress
	bar.Describe("Processing key list...")

	// Process the output
	var keys []string
	scanner := bufio.NewScanner(strings.NewReader(string(jqOutput)))
	for scanner.Scan() {
		key := scanner.Text()
		// Remove quotes from the key
		key = strings.Trim(key, "\"")
		if key != "" {
			keys = append(keys, key)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading jq output: %w", err)
	}

	if len(keys) == 0 {
		return nil, fmt.Errorf("no keys extracted from values.yaml")
	}

	// Complete the progress
	bar.Describe(fmt.Sprintf("Found %d keys", len(keys)))
	bar.Finish()

	if e.Debug {
		fmt.Printf("Extracted %d keys from values.yaml\n", len(keys))
	}

	return keys, nil
}
