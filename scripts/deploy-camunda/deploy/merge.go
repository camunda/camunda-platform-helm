package deploy

import (
	"fmt"
	"os"
	"path/filepath"
	"scripts/camunda-core/pkg/logging"

	"gopkg.in/yaml.v3"
)

// MergeYAMLFiles reads multiple YAML values files (in order) and deep-merges
// them into a single document. Maps are recursively merged (matching Helm's
// behaviour). Arrays of objects that contain a "name" key (Kubernetes env var
// style) are merged by name — later layers override entries with the same name,
// and new entries are appended. All other arrays are concatenated.
//
// The merged output is written to outputPath. If the input slice has zero or
// one file the function short-circuits (no merge needed).
func MergeYAMLFiles(files []string, outputPath string) (string, error) {
	if len(files) == 0 {
		return "", fmt.Errorf("MergeYAMLFiles: no input files")
	}
	if len(files) == 1 {
		// Nothing to merge — return original file as-is.
		return files[0], nil
	}

	var merged map[string]any

	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return "", fmt.Errorf("MergeYAMLFiles: reading %q: %w", f, err)
		}
		var doc map[string]any
		if err := yaml.Unmarshal(data, &doc); err != nil {
			return "", fmt.Errorf("MergeYAMLFiles: unmarshalling %q: %w", f, err)
		}
		if doc == nil {
			continue // empty document
		}
		if merged == nil {
			merged = doc
		} else {
			merged = deepMergeMaps(merged, doc)
		}
	}

	if merged == nil {
		merged = map[string]any{}
	}

	out, err := yaml.Marshal(merged)
	if err != nil {
		return "", fmt.Errorf("MergeYAMLFiles: marshalling merged document: %w", err)
	}
	if err := os.WriteFile(outputPath, out, 0o644); err != nil {
		return "", fmt.Errorf("MergeYAMLFiles: writing %q: %w", outputPath, err)
	}

	logging.Logger.Info().
		Int("inputFiles", len(files)).
		Str("outputPath", outputPath).
		Msg("Merged layered YAML values into single file")

	return outputPath, nil
}

// deepMergeMaps recursively merges src into dst and returns the result.
// dst is mutated. src values take precedence for scalar conflicts.
func deepMergeMaps(dst, src map[string]any) map[string]any {
	for key, srcVal := range src {
		dstVal, exists := dst[key]
		if !exists {
			dst[key] = srcVal
			continue
		}

		// Both sides exist — decide merge strategy by type.
		switch dstTyped := dstVal.(type) {
		case map[string]any:
			if srcTyped, ok := srcVal.(map[string]any); ok {
				dst[key] = deepMergeMaps(dstTyped, srcTyped)
			} else {
				// Type mismatch — src wins.
				dst[key] = srcVal
			}
		case []any:
			if srcTyped, ok := srcVal.([]any); ok {
				dst[key] = mergeArrays(dstTyped, srcTyped)
			} else {
				dst[key] = srcVal
			}
		default:
			// Scalar — src wins (standard override).
			dst[key] = srcVal
		}
	}
	return dst
}

// mergeArrays merges two YAML arrays. If both contain objects with a "name"
// key (Kubernetes env-var / volume style), they are merged by name — later
// entries override earlier ones with the same name, and new entries are
// appended. Otherwise the arrays are concatenated.
func mergeArrays(dst, src []any) []any {
	if isNameKeyedArray(dst) && isNameKeyedArray(src) {
		return mergeByName(dst, src)
	}
	// Fallback: concatenate (skip duplicates by deep equality).
	return appendUnique(dst, src)
}

// isNameKeyedArray returns true if every element in the slice is a map
// containing a "name" key (possibly with zero-length slices returning true
// to avoid false negatives in the merge decision).
func isNameKeyedArray(arr []any) bool {
	if len(arr) == 0 {
		return true // vacuously true — safe for either merge strategy
	}
	for _, elem := range arr {
		m, ok := elem.(map[string]any)
		if !ok {
			return false
		}
		if _, hasName := m["name"]; !hasName {
			return false
		}
	}
	return true
}

// mergeByName merges two arrays of named objects. Entries from src with the
// same "name" as an entry in dst replace the dst entry (preserving order).
// New entries are appended at the end.
func mergeByName(dst, src []any) []any {
	// Build index of dst entries by name for O(1) lookup.
	type entry struct {
		index int
		obj   map[string]any
	}
	dstIndex := make(map[string]entry, len(dst))
	for i, elem := range dst {
		if m, ok := elem.(map[string]any); ok {
			if name, ok := m["name"].(string); ok {
				dstIndex[name] = entry{index: i, obj: m}
			}
		}
	}

	// Apply src entries.
	for _, elem := range src {
		m, ok := elem.(map[string]any)
		if !ok {
			dst = append(dst, elem) // non-map element — just append
			continue
		}
		name, ok := m["name"].(string)
		if !ok {
			dst = append(dst, elem) // no name — just append
			continue
		}
		if existing, found := dstIndex[name]; found {
			// Override in place — deep merge the map entries so that
			// partial overrides work (e.g., adding "valueFrom" alongside "value").
			dst[existing.index] = deepMergeMaps(existing.obj, m)
		} else {
			dst = append(dst, elem)
			dstIndex[name] = entry{index: len(dst) - 1, obj: m}
		}
	}
	return dst
}

// appendUnique appends elements from src to dst, skipping any that are
// already present (by deep YAML equality via marshalling).
func appendUnique(dst, src []any) []any {
	existing := make(map[string]bool, len(dst))
	for _, elem := range dst {
		key := yamlFingerprint(elem)
		existing[key] = true
	}
	for _, elem := range src {
		key := yamlFingerprint(elem)
		if !existing[key] {
			dst = append(dst, elem)
			existing[key] = true
		}
	}
	return dst
}

// yamlFingerprint returns a deterministic string representation of a value
// for equality comparison. Uses YAML marshalling which handles maps/slices.
func yamlFingerprint(v any) string {
	b, err := yaml.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(b)
}

// MergeLayeredValues is the entry point called from prepareScenarioValues.
// It takes the list of processed scenario value files, merges them into a
// single file in tempDir, and returns a slice containing just that one file.
// If merging is not needed (0 or 1 files), the original slice is returned.
func MergeLayeredValues(scenarioValueFiles []string, tempDir string) ([]string, error) {
	if len(scenarioValueFiles) <= 1 {
		return scenarioValueFiles, nil
	}

	outputPath := filepath.Join(tempDir, "merged-scenario-values.yaml")
	merged, err := MergeYAMLFiles(scenarioValueFiles, outputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to merge layered values: %w", err)
	}
	return []string{merged}, nil
}
