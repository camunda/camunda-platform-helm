package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

// extractParamPaths reads values.yaml and extracts unique @param and @skip paths.
func extractParamPaths(filename string) (map[string]bool, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	paramPaths := make(map[string]bool)
	paramRegex := regexp.MustCompile(`## @(param|skip) ([\w\.]+)`) // Matches both @param and @skip
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		matches := paramRegex.FindStringSubmatch(line)
		if len(matches) > 2 {
			paramPaths[matches[2]] = true // Ensure uniqueness
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return paramPaths, nil
}

// extractSetValues scans .go files and extracts any map keys in SetValues or testCase values.
func extractSetValues(folder string) (map[string]bool, error) {
	setValues := make(map[string]bool)
	mapKeyRegex := regexp.MustCompile(`"([\w\.]+)"\s*:`) // Matches "some.key":
    
	err := filepath.Walk(folder, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || filepath.Ext(path) != ".go" {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		inSetValues := false
		inValuesMap := false

		for scanner.Scan() {
			line := scanner.Text()

			// Detect start of SetValues or values maps
			if regexp.MustCompile(`SetValues:\s*map\[string\]string{`).MatchString(line) {
				inSetValues = true
			}
			if regexp.MustCompile(`values:\s*map\[string\]string{`).MatchString(line) {
				inValuesMap = true
			}

			// Extract keys inside the detected maps
			if inSetValues || inValuesMap {
				matches := mapKeyRegex.FindAllStringSubmatch(line, -1)
				for _, match := range matches {
					if _, exists := setValues[match[1]]; !exists {
						setValues[match[1]] = true // Ensure uniqueness
					}
				}
			}

			// Detect end of the maps
			if inSetValues && regexp.MustCompile(`},`).MatchString(line) {
				inSetValues = false
			}
			if inValuesMap && regexp.MustCompile(`},`).MatchString(line) {
				inValuesMap = false
			}
		}

		if err := scanner.Err(); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return setValues, nil
}

// removeDuplicates removes matching entries and returns remaining unmatched items.
func removeDuplicates(paramPaths, setValues map[string]bool) (map[string]bool, map[string]bool, int) {
	removedCount := 0

	// Remove exact matches
	for path := range setValues {
		if paramPaths[path] {
			delete(setValues, path)
			delete(paramPaths, path)
			removedCount++
		}
	}

	return paramPaths, setValues, removedCount
}

// calculateCoverage computes the percentage of tested params.
func calculateCoverage(setValuesCount, paramCount int) float64 {
	if paramCount == 0 {
		return 0.0 // Avoid division by zero
	}
	return (float64(setValuesCount) / float64(paramCount)) * 100
}

func main() {
	valuesFile := "values.yaml" // Change if needed
	unitFolder := "test/unit"        // Folder containing .go test filesz

	// Extract unique @param and @skip paths from values.yaml
	paramPaths, err := extractParamPaths(valuesFile)
	if err != nil {
		fmt.Println("Error reading values.yaml:", err)
		return
	}

	// Extract unique SetValues and testCase values from Go files
	setValues, err := extractSetValues(unitFolder)
	if err != nil {
		fmt.Println("Error reading Go files:", err)
		return
	}

	// Remove duplicates and validate
	remainingParamPaths, remainingSetValues, removedCount := removeDuplicates(paramPaths, setValues)
	
	// Calculate coverage
	coverage := calculateCoverage(removedCount, len(paramPaths)+removedCount)

	// Print remaining paths if any
	if len(remainingParamPaths) > 0 {
		fmt.Println("❌ Unmatched @param + @skip paths:", keysFromMap(remainingParamPaths))
	} else {
		fmt.Println("✅ All @param + @skip paths are tested!")
	}
	
	if len(remainingSetValues) > 0 {
		fmt.Println("❌ Unmatched SetValues & testCase values:", keysFromMap(remainingSetValues))
	} else {
		fmt.Println("✅ All SetValues & testCase values are covered!")
	}

	// Output results
	fmt.Println()
    fmt.Println("📊 Results:")
    fmt.Printf("📄 Total configs in values.yaml (based on @param + @skip): %d\n", len(paramPaths)+removedCount)
    fmt.Printf("🧪 Total tested config keys in test files (unique): %d\n", len(setValues)+removedCount)
    fmt.Printf("🔄 Matching entries: %d\n", removedCount)
    fmt.Printf("❌ Unmatched configs in values.yaml: %d\n", len(remainingParamPaths))
    fmt.Printf("❌ Unmatched configs in test files: %d\n", len(remainingSetValues))
    fmt.Printf("📈 --> Unit Test Coverage: %.2f%%\n", coverage)
}

// keysFromMap converts a map's keys into a slice.
func keysFromMap(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
