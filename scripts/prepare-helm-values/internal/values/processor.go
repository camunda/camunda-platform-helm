package values

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"scripts/prepare-helm-values/internal/placeholders"
)

type Options struct {
	ChartPath    string
	Scenario     string
	ValuesConfig string
	LicenseKey   string
	Output       string
}

type MissingEnvError struct {
	Missing []string
}

func (e MissingEnvError) Error() string {
	return fmt.Sprintf("missing required environment variables: %s", strings.Join(e.Missing, ", "))
}

func readFile(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}

func ensureMap(m map[string]interface{}, k string) map[string]interface{} {
	if v, ok := m[k]; ok {
		if mm, ok := v.(map[string]interface{}); ok {
			return mm
		}
	}
	mm := map[string]interface{}{}
	m[k] = mm
	return mm
}

// ResolveValuesFile determines the target values file from options.
func ResolveValuesFile(opts Options) (string, error) {
	scenariosDir := filepath.Join(opts.ChartPath, "test", "integration", "scenarios", "chart-full-setup")
	defaultValuesFile := filepath.Join(scenariosDir, fmt.Sprintf("values-integration-test-ingress-%s.yaml", opts.Scenario))
	valuesFile := defaultValuesFile
	if opts.Output != "" {
		valuesFile = opts.Output
	}
	if _, err := os.Stat(valuesFile); err != nil {
		return "", err
	}
	return valuesFile, nil
}

// Process performs substitution and optional license injection, writing once to disk and
// returning the final content as a string.
func Process(valuesFile string, opts Options, verbose bool) (string, error) {
	// Build overlay env from JSON config (stringified)
	configEnv := map[string]string{}
	if opts.ValuesConfig != "" && opts.ValuesConfig != "{}" {
		var m map[string]interface{}
		if err := json.Unmarshal([]byte(opts.ValuesConfig), &m); err != nil {
			return "", err
		}
		for k, v := range m {
			configEnv[k] = fmt.Sprintf("%v", v)
		}
	}

	content, err := readFile(valuesFile)
	if err != nil {
		return "", err
	}

	// Find required placeholders and validate presence (unset is an error; empty is allowed)
	ph := placeholders.Find(content)
	var missing []string
	getVal := func(name string) (string, bool) {
		if v, ok := configEnv[name]; ok {
			return v, true
		}
		v, ok := os.LookupEnv(name)
		return v, ok
	}
	for _, p := range ph {
		if _, ok := getVal(p); !ok {
			missing = append(missing, p)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return "", MissingEnvError{Missing: missing}
	}

	// Perform substitution using os.Expand to support both $VAR and ${VAR} consistently
	content = os.Expand(content, func(name string) string {
		if v, ok := getVal(name); ok {
			return v
		}
		// This should not happen (validated above), but keep safe fallback
		return ""
	})

	// Optional license injection performed in-memory on substituted content
	if opts.LicenseKey != "" {
		var doc map[string]interface{}
		if err := yaml.Unmarshal([]byte(content), &doc); err != nil {
			return "", err
		}
		global := ensureMap(doc, "global")
		license := ensureMap(global, "license")
		license["key"] = opts.LicenseKey
		out, err := yaml.Marshal(doc)
		if err != nil {
			return "", err
		}
		content = string(out)
	}

	// Single write to disk at the end
	if err := writeFile(valuesFile, content); err != nil {
		return "", err
	}
	return content, nil
}

// IsMissingEnv returns (true, names) if err is a MissingEnvError.
func IsMissingEnv(err error) (bool, []string) {
	var me MissingEnvError
	if errors.As(err, &me) {
		return true, me.Missing
	}
	return false, nil
}


