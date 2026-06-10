package deploy

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// helmGetValuesTimeout is the maximum time allowed for a `helm get values` call.
const helmGetValuesTimeout = 30 * time.Second

// InstalledPrefixes holds index prefix values read from a live Helm release.
// Zero-value fields indicate the prefix was not found or not set.
type InstalledPrefixes struct {
	OrchestrationIndexPrefix string
	OperateIndexPrefix       string
	OptimizeIndexPrefix      string
	TasklistIndexPrefix      string
}

// GetInstalledValues runs `helm get values <release> -n <ns> -o yaml` and
// returns the parsed user-supplied values as a nested map. Returns a nil map
// (no error) if the release does not exist.
func GetInstalledValues(ctx context.Context, namespace, release, kubeContext string) (map[string]interface{}, error) {
	callCtx, cancel := context.WithTimeout(ctx, helmGetValuesTimeout)
	defer cancel()

	args := []string{"get", "values", release, "-n", namespace, "-o", "yaml"}
	if kubeContext != "" {
		args = append(args, "--kube-context", kubeContext)
	}

	cmd := exec.CommandContext(callCtx, "helm", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		stderrStr := strings.TrimSpace(stderr.String())
		// Release not found is not an error — return nil map.
		// Match Helm's specific error messages to avoid false positives from
		// unrelated errors that happen to contain "not found".
		if strings.Contains(stderrStr, "release: not found") ||
			strings.Contains(stderrStr, fmt.Sprintf("release %q not found", release)) {
			return nil, nil
		}
		return nil, fmt.Errorf("helm get values %s -n %s failed: %w; stderr: %s",
			release, namespace, err, truncateStr(stderrStr, 500))
	}

	out := bytes.TrimSpace(stdout.Bytes())
	if len(out) == 0 {
		return nil, nil
	}

	var vals map[string]interface{}
	if err := yaml.Unmarshal(out, &vals); err != nil {
		return nil, fmt.Errorf("failed to parse helm values YAML: %w", err)
	}

	return vals, nil
}

// ReadInstalledPrefixes extracts index prefix values from a live Helm release.
// It reads `orchestration.index.prefix`, walks `orchestration.env[]` for the
// operate secondary-storage prefix, and reads `optimize.database.opensearch.prefix`.
// Returns a zero-value struct if the release doesn't exist or fields are absent.
func ReadInstalledPrefixes(ctx context.Context, namespace, release, kubeContext string) (InstalledPrefixes, error) {
	vals, err := GetInstalledValues(ctx, namespace, release, kubeContext)
	if err != nil {
		return InstalledPrefixes{}, err
	}
	if vals == nil {
		return InstalledPrefixes{}, nil
	}

	return readPrefixesFromMap(vals), nil
}

// readPrefixesFromMap extracts index prefix values from a parsed Helm values map.
// Factored out from ReadInstalledPrefixes for direct unit testing.
func readPrefixesFromMap(vals map[string]interface{}) InstalledPrefixes {
	var result InstalledPrefixes

	// orchestration.index.prefix
	result.OrchestrationIndexPrefix = getNestedString(vals, "orchestration", "index", "prefix")

	// optimize.database.opensearch.prefix (also uses the orchestration prefix in OS scenarios)
	result.OptimizeIndexPrefix = getNestedString(vals, "optimize", "database", "opensearch", "prefix")

	// orchestration.env[] — find CAMUNDA_DATA_SECONDARYSTORAGE_OPENSEARCH_INDEXPREFIX
	result.OperateIndexPrefix = findEnvValue(vals, []string{"orchestration", "env"}, "CAMUNDA_DATA_SECONDARYSTORAGE_OPENSEARCH_INDEXPREFIX")

	// orchestration.env[] — find CAMUNDA_TASKLIST_OPENSEARCH_INDEXPREFIX (if present)
	if tp := findEnvValue(vals, []string{"orchestration", "env"}, "CAMUNDA_TASKLIST_OPENSEARCH_INDEXPREFIX"); tp != "" {
		result.TasklistIndexPrefix = tp
	}

	return result
}

// getNestedString traverses a nested map by successive keys and returns the
// leaf value as a string. Returns "" if any key is missing or the leaf is not
// a string.
func getNestedString(m map[string]interface{}, keys ...string) string {
	current := m
	for i, key := range keys {
		v, ok := current[key]
		if !ok {
			return ""
		}
		if i == len(keys)-1 {
			// Leaf — convert to string
			switch s := v.(type) {
			case string:
				return s
			default:
				return ""
			}
		}
		// Intermediate — must be a map
		next, ok := v.(map[string]interface{})
		if !ok {
			return ""
		}
		current = next
	}
	return ""
}

// findEnvValue searches an env-style array ([]map with "name"/"value" keys)
// at the given path in the nested map, returning the "value" for the entry
// whose "name" matches envName. Returns "" if not found.
func findEnvValue(m map[string]interface{}, path []string, envName string) string {
	if len(path) == 0 {
		return ""
	}
	// Navigate to the array
	current := m
	for _, key := range path[:len(path)-1] {
		v, ok := current[key]
		if !ok {
			return ""
		}
		next, ok := v.(map[string]interface{})
		if !ok {
			return ""
		}
		current = next
	}

	// Get the array
	arrKey := path[len(path)-1]
	arrRaw, ok := current[arrKey]
	if !ok {
		return ""
	}

	arr, ok := arrRaw.([]interface{})
	if !ok {
		return ""
	}

	// Search for the env var by name
	for _, item := range arr {
		entry, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		name, _ := entry["name"].(string)
		if name == envName {
			val, _ := entry["value"].(string)
			return val
		}
	}

	return ""
}

// truncateStr truncates a string to maxLen characters, appending "..." if truncated.
func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
