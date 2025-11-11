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

	"scripts/prepare-helm-values/internal/logging"
	"scripts/prepare-helm-values/internal/placeholders"
)

type Options struct {
	ChartPath    string
	Scenario     string
	ValuesConfig string
	LicenseKey   string
	Output       string
	OutputDir    string
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

// ResolveValuesFile determines the source values file from options.
func ResolveValuesFile(opts Options, log logging.Logger) (string, error) {
	scenariosDir := filepath.Join(opts.ChartPath, "test", "integration", "scenarios", "chart-full-setup")
	sourceValuesFile := filepath.Join(scenariosDir, fmt.Sprintf("values-integration-test-ingress-%s.yaml", opts.Scenario))
	
	log.Debugf("Resolving values file from scenarios dir: %s", scenariosDir)
	log.Debugf("Looking for values file: %s", sourceValuesFile)
	
	if _, err := os.Stat(sourceValuesFile); err != nil {
		log.Debugf("Values file not found: %v", err)
		return "", err
	}
	
	log.Debugf("Found values file: %s", sourceValuesFile)
	return sourceValuesFile, nil
}

// computeOutputPath determines where to write the processed values file.
func computeOutputPath(sourceValuesFile string, opts Options, log logging.Logger) (string, error) {
	if opts.Output != "" {
		log.Debugf("Using explicit output file: %s", opts.Output)
		return opts.Output, nil
	}
	if opts.OutputDir != "" {
		log.Debugf("Creating output directory if needed: %s", opts.OutputDir)
		// Ensure output directory exists
		if err := os.MkdirAll(opts.OutputDir, 0o755); err != nil {
			return "", fmt.Errorf("create output directory: %w", err)
		}
		// Use the original filename in the output directory
		filename := filepath.Base(sourceValuesFile)
		outputPath := filepath.Join(opts.OutputDir, filename)
		log.Debugf("Output path will be: %s", outputPath)
		return outputPath, nil
	}
	// Default: write in-place
	log.Debugf("Writing in-place to: %s", sourceValuesFile)
	return sourceValuesFile, nil
}

// Process performs substitution and optional license injection, writing once to disk and
// returning the output path and final content as a string.
func Process(valuesFile string, opts Options, log logging.Logger) (string, string, error) {
	log.Debugf("Starting values processing for: %s", valuesFile)
	
	// Build overlay env from JSON config (stringified)
	configEnv := map[string]string{}
	if opts.ValuesConfig != "" && opts.ValuesConfig != "{}" {
		log.Debugf("Parsing values-config JSON")
		var m map[string]interface{}
		if err := json.Unmarshal([]byte(opts.ValuesConfig), &m); err != nil {
			log.Debugf("Failed to parse values-config: %v", err)
			return "", "", err
		}
		for k, v := range m {
			configEnv[k] = fmt.Sprintf("%v", v)
		}
		log.Debugf("Loaded %d config values from values-config", len(configEnv))
	}

	log.Debugf("Reading values file: %s", valuesFile)
	content, err := readFile(valuesFile)
	if err != nil {
		log.Debugf("Failed to read values file: %v", err)
		return "", "", err
	}
	log.Debugf("Read %d bytes from values file", len(content))

	// Find required placeholders and validate presence (unset is an error; empty is allowed)
	log.Debugf("Scanning for placeholders in values file")
	ph := placeholders.Find(content)
	log.Debugf("Found %d unique placeholders to substitute", len(ph))
	
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
		log.Debugf("Missing %d required environment variables", len(missing))
		return "", "", MissingEnvError{Missing: missing}
	}
	log.Debugf("All required environment variables are present")

	// Perform substitution using os.Expand to support both $VAR and ${VAR} consistently
	log.Debugf("Performing placeholder substitution")
	content = os.Expand(content, func(name string) string {
		if v, ok := getVal(name); ok {
			return v
		}
		// This should not happen (validated above), but keep safe fallback
		return ""
	})
	log.Debugf("Placeholder substitution complete")

	// Optional license injection performed in-memory on substituted content
	if opts.LicenseKey != "" {
		log.Debugf("Injecting license key into global.license.key")
		var doc map[string]interface{}
		if err := yaml.Unmarshal([]byte(content), &doc); err != nil {
			log.Debugf("Failed to unmarshal YAML for license injection: %v", err)
			return "", "", err
		}
		global := ensureMap(doc, "global")
		license := ensureMap(global, "license")
		license["key"] = opts.LicenseKey
		out, err := yaml.Marshal(doc)
		if err != nil {
			log.Debugf("Failed to marshal YAML after license injection: %v", err)
			return "", "", err
		}
		content = string(out)
		log.Debugf("License key injected successfully")
	} else {
		log.Debugf("No license key provided, skipping injection")
	}

	// Determine output path
	log.Debugf("Determining output path")
	outputPath, err := computeOutputPath(valuesFile, opts, log)
	if err != nil {
		log.Debugf("Failed to compute output path: %v", err)
		return "", "", err
	}

	// Single write to disk at the end
	log.Debugf("Writing processed values to: %s", outputPath)
	if err := writeFile(outputPath, content); err != nil {
		log.Debugf("Failed to write output file: %v", err)
		return "", "", err
	}
	log.Debugf("Successfully wrote %d bytes to %s", len(content), outputPath)
	return outputPath, content, nil
}

// IsMissingEnv returns (true, names) if err is a MissingEnvError.
func IsMissingEnv(err error) (bool, []string) {
	var me MissingEnvError
	if errors.As(err, &me) {
		return true, me.Missing
	}
	return false, nil
}


