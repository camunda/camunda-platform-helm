// Copyright 2026 Camunda Services GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Command validate-values-schema reports values keys that are not described by
// a chart's values.schema.json.
//
// It performs the same unknown-key detection used by the downstream InfraEx
// internal-helm-deprecation-check, so chart CI catches schema/values drift
// before it reaches consumers. Detection strictifies a copy of the schema
// in memory (it does NOT add additionalProperties:false to the shipped
// schema) — so this is a verification gate, not install-time enforcement.
//
// Strictification is non-destructive: object schemas that already declare
// additionalProperties (free-form override blocks, annotation/label string
// maps) keep their carve-out, so those are not reported as unknown.
//
// Top-level sub-chart roots (the chart's Helm dependencies, e.g. elasticsearch,
// identityKeycloak) are skipped: their internals are described by the
// sub-chart's own schema, not the umbrella schema.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

func main() {
	schemaPath := flag.String("schema", "", "path to values.schema.json")
	chartDir := flag.String("chart-dir", "", "chart directory; its Chart.yaml dependencies are treated as pass-through sub-chart roots")
	var ignoreRoots []string
	flag.Func("ignore-root", "additional top-level key to skip (repeatable)", func(v string) error {
		ignoreRoots = append(ignoreRoots, v)
		return nil
	})
	flag.Parse()

	if *schemaPath == "" || flag.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "usage: validate-values-schema --schema <schema.json> [--chart-dir <dir>] [--ignore-root <key>]... <values.yaml>...")
		os.Exit(2)
	}

	schema, err := loadJSON(*schemaPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: reading schema: %v\n", err)
		os.Exit(2)
	}
	strict := strictify(schema)

	ignore := map[string]bool{}
	for _, r := range ignoreRoots {
		ignore[r] = true
	}
	if *chartDir != "" {
		deps, err := chartDependencyRoots(filepath.Join(*chartDir, "Chart.yaml"))
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: reading Chart.yaml: %v\n", err)
			os.Exit(2)
		}
		for _, d := range deps {
			ignore[d] = true
		}
	}

	failures := 0
	for _, valuesPath := range flag.Args() {
		values, err := loadYAML(valuesPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: reading %s: %v\n", valuesPath, err)
			os.Exit(2)
		}
		vmap, ok := values.(map[string]any)
		if !ok {
			continue
		}
		for root := range ignore {
			delete(vmap, root)
		}
		unknown := findUnknownKeys(strict, vmap, "")
		if len(unknown) > 0 {
			failures += len(unknown)
			sort.Strings(unknown)
			fmt.Printf("::error::%s defines %d key(s) not described by the schema:\n", valuesPath, len(unknown))
			for _, k := range unknown {
				fmt.Printf("  - %s\n", k)
			}
		}
	}

	if failures > 0 {
		fmt.Fprintf(os.Stderr, "\nAdd the missing keys to values.schema.extra.json and run 'make helm.schema-update', "+
			"or remove the dead keys from the values file. See https://github.com/camunda/camunda-platform-helm/issues/4564\n")
		os.Exit(1)
	}
}

// strictify recursively sets additionalProperties:false on object schemas that
// do not already declare additionalProperties. Existing values (true, false, or
// a sub-schema) are preserved so intentional carve-outs keep working.
func strictify(node any) any {
	m, ok := node.(map[string]any)
	if !ok {
		return node
	}
	_, hasProps := m["properties"]
	_, hasPattern := m["patternProperties"]
	if m["type"] == "object" || hasProps || hasPattern {
		if _, ok := m["additionalProperties"]; !ok {
			m["additionalProperties"] = false
		}
	}
	if props, ok := m["properties"].(map[string]any); ok {
		for k, v := range props {
			props[k] = strictify(v)
		}
	}
	if pp, ok := m["patternProperties"].(map[string]any); ok {
		for k, v := range pp {
			pp[k] = strictify(v)
		}
	}
	switch items := m["items"].(type) {
	case map[string]any:
		m["items"] = strictify(items)
	case []any:
		for i, it := range items {
			items[i] = strictify(it)
		}
	}
	for _, comb := range []string{"allOf", "anyOf", "oneOf"} {
		if arr, ok := m[comb].([]any); ok {
			for i, sub := range arr {
				arr[i] = strictify(sub)
			}
		}
	}
	if not, ok := m["not"].(map[string]any); ok {
		m["not"] = strictify(not)
	}
	return m
}

// findUnknownKeys walks values against the strictified schema and returns dotted
// paths for keys that the schema does not allow.
func findUnknownKeys(schema any, values map[string]any, path string) []string {
	var unknown []string
	smap, ok := schema.(map[string]any)
	if !ok {
		return unknown
	}
	props, _ := smap["properties"].(map[string]any)
	pattern, _ := smap["patternProperties"].(map[string]any)

	additionalFalse := false
	if av, ok := smap["additionalProperties"]; ok {
		if b, isBool := av.(bool); isBool && !b {
			additionalFalse = true
		}
	}
	// Open object (no declared properties and additional allowed): nothing to check.
	if len(props) == 0 && !additionalFalse {
		return unknown
	}

	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		full := k
		if path != "" {
			full = path + "." + k
		}
		if sub, ok := props[k]; ok {
			unknown = append(unknown, recurseValue(sub, values[k], full)...)
			continue
		}
		if len(pattern) > 0 {
			matched := false
			for pat, sub := range pattern {
				if re, err := regexp.Compile(pat); err == nil && re.MatchString(k) {
					matched = true
					unknown = append(unknown, recurseValue(sub, values[k], full)...)
					break
				}
			}
			if matched {
				continue
			}
		}
		if additionalFalse {
			unknown = append(unknown, full)
		}
	}
	return unknown
}

func recurseValue(schema any, value any, path string) []string {
	switch v := value.(type) {
	case map[string]any:
		return findUnknownKeys(schema, v, path)
	case []any:
		smap, _ := schema.(map[string]any)
		items, _ := smap["items"].(map[string]any)
		if items == nil {
			return nil
		}
		var unknown []string
		for i, it := range v {
			if im, ok := it.(map[string]any); ok {
				unknown = append(unknown, findUnknownKeys(items, im, fmt.Sprintf("%s[%d]", path, i))...)
			}
		}
		return unknown
	}
	return nil
}

func chartDependencyRoots(chartYAML string) ([]string, error) {
	raw, err := os.ReadFile(chartYAML)
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Dependencies []struct {
			Name  string `yaml:"name"`
			Alias string `yaml:"alias"`
		} `yaml:"dependencies"`
	}
	if err := yaml.Unmarshal(raw, &parsed); err != nil {
		return nil, err
	}
	roots := make([]string, 0, len(parsed.Dependencies))
	for _, d := range parsed.Dependencies {
		name := strings.TrimSpace(d.Alias)
		if name == "" {
			name = strings.TrimSpace(d.Name)
		}
		if name != "" {
			roots = append(roots, name)
		}
	}
	return roots, nil
}

func loadJSON(path string) (any, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func loadYAML(path string) (any, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out any
	if err := yaml.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}
