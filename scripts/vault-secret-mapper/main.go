package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

func main() {
	mapping := flag.String("mapping", "", "Vault secret mapping content (multi-line, semicolon-terminated entries)")
	secretName := flag.String("secret-name", "vault-mapped-secrets", "Kubernetes Secret name to generate")
	outputPath := flag.String("output", "", "Path to write the generated Secret YAML")
	flag.Parse()

	if *outputPath == "" {
		exitWithError("missing required flag: --output")
	}
	if *mapping == "" {
		exitWithError("missing required flag: --mapping")
	}

	// Parse the mapping and derive env var names to include in the Secret
	envVarNames := parseMapping(*mapping)
	envVarNames = dedupePreserveOrder(envVarNames)

	// Collect non-empty env vars from environment
	stringData := make([][2]string, 0, len(envVarNames))
	for _, name := range envVarNames {
		if name == "" {
			continue
		}
		val := os.Getenv(name)
		if val == "" {
			// Skip empty values to avoid creating empty keys
			continue
		}
		stringData = append(stringData, [2]string{name, val})
	}

	// Build YAML
	yaml := buildSecretYAML(*secretName, stringData)

	// Write to file
	if err := os.WriteFile(*outputPath, []byte(yaml), 0o600); err != nil {
		exitWithError("write output: %v", err)
	}
}

func exitWithError(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "vault-secret-mapper error: %s\n", msg)
	os.Exit(1)
}

// parseMapping extracts the env var names from the provided mapping.
// Input format examples (one per line; trailing ';' optional):
//   path/to/secret KEY1 | ALIAS1;
//   path/to/secret KEY1,KEY2 | ALIAS1,ALIAS2;
//   path/to/secret KEY1;
// When alias list is present after '|', we use aliases; otherwise use key names.
func parseMapping(mapping string) []string {
	lines := strings.Split(mapping, "\n")
	var names []string
	for _, raw := range lines {
		line := trimSpaceAndCR(raw)
		if line == "" {
			continue
		}
		// Remove trailing semicolon
		if strings.HasSuffix(line, ";") {
			line = strings.TrimSuffix(line, ";")
		}
		// Skip comments
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		// Split first space: path and remainder
		parts := splitOnce(line, " ")
		if len(parts) != 2 {
			// malformed; skip
			continue
		}
		rest := strings.TrimSpace(parts[1])
		if rest == "" {
			continue
		}

		if idx := strings.Index(rest, "|"); idx >= 0 {
			aliasPart := strings.TrimSpace(rest[idx+1:])
			if aliasPart == "" {
				continue
			}
			for _, n := range splitCSVOrSpace(aliasPart) {
				n = strings.TrimSpace(n)
				if n != "" {
					names = append(names, n)
				}
			}
		} else {
			// Use original keys (before alias)
			keysPart := strings.TrimSpace(rest)
			for _, n := range splitCSVOrSpace(keysPart) {
				n = strings.TrimSpace(n)
				if n != "" {
					names = append(names, n)
				}
			}
		}
	}
	return names
}

func trimSpaceAndCR(s string) string {
	s = strings.TrimRight(s, "\r")
	return strings.TrimSpace(s)
}

func splitOnce(s, sep string) []string {
	i := strings.Index(s, sep)
	if i < 0 {
		return []string{s}
	}
	return []string{s[:i], s[i+len(sep):]}
}

func splitCSVOrSpace(s string) []string {
	// Replace commas with spaces, then split on spaces
	s = strings.ReplaceAll(s, ",", " ")
	fields := strings.Fields(s)
	return fields
}

func dedupePreserveOrder(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, v := range in {
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func yamlQuote(s string) string {
	// Use single-quoted YAML string; escape single quotes by doubling them
	s = strings.ReplaceAll(s, "'", "''")
	return "'" + s + "'"
}

func buildSecretYAML(secretName string, data [][2]string) string {
	var b strings.Builder
	b.WriteString("apiVersion: v1\n")
	b.WriteString("kind: Secret\n")
	b.WriteString("metadata:\n")
	b.WriteString("  name: " + escapeYAMLKey(secretName) + "\n")

	// Optional labels
	jobID := os.Getenv("GITHUB_WORKFLOW_JOB_ID")
	b.WriteString("  labels:\n")
	b.WriteString("    managed-by: test-integration-runner\n")
	if jobID != "" {
		b.WriteString("    github-id: " + escapeYAMLKey(jobID) + "\n")
	}

	b.WriteString("type: Opaque\n")
	b.WriteString("stringData:\n")
	if len(data) == 0 {
		// Create an empty key to ensure valid YAML (Kubernetes allows empty stringData map)
		// but we still keep it empty for clarity.
	}
	for _, kv := range data {
		key := kv[0]
		val := kv[1]
		b.WriteString("  " + escapeYAMLKey(key) + ": " + yamlQuote(val) + "\n")
	}
	return b.String()
}

// escapeYAMLKey ensures the key is a valid simple YAML key; if it contains unsafe chars, quote it.
func escapeYAMLKey(s string) string {
	if s == "" {
		return "''"
	}
	// Simple heuristic: if only [A-Za-z0-9_.-], keep as-is; else single-quote
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'A' && c <= 'Z') ||
			(c >= 'a' && c <= 'z') ||
			(c >= '0' && c <= '9') ||
			c == '_' || c == '.' || c == '-' {
			continue
		}
		return yamlQuote(s)
	}
	return s
}


