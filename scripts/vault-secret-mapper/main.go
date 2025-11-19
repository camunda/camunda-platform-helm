package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Secret represents a Kubernetes Secret
type Secret struct {
	APIVersion string            `yaml:"apiVersion"`
	Kind       string            `yaml:"kind"`
	Metadata   Metadata          `yaml:"metadata"`
	Type       string            `yaml:"type"`
	StringData map[string]string `yaml:"stringData"`
}

type Metadata struct {
	Name   string            `yaml:"name"`
	Labels map[string]string `yaml:"labels"`
}

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
	stringData := make(map[string]string)
	for _, name := range envVarNames {
		if name == "" {
			continue
		}
		val := os.Getenv(name)
		if val == "" {
			// Skip empty values to avoid creating empty keys
			continue
		}
		stringData[name] = val
	}

	// Build Labels
	labels := map[string]string{
		"managed-by": "test-integration-runner",
	}
	if jobID := os.Getenv("GITHUB_WORKFLOW_JOB_ID"); jobID != "" {
		labels["github-id"] = jobID
	}

	secret := Secret{
		APIVersion: "v1",
		Kind:       "Secret",
		Metadata: Metadata{
			Name:   *secretName,
			Labels: labels,
		},
		Type:       "Opaque",
		StringData: stringData,
	}

	// Marshal to YAML
	yamlBytes, err := yaml.Marshal(&secret)
	if err != nil {
		exitWithError("marshal YAML: %v", err)
	}

	// Write to file
	if err := os.WriteFile(*outputPath, yamlBytes, 0o600); err != nil {
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
		line = strings.TrimSuffix(line, ";")

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
