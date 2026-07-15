// Copyright 2026 Camunda Services GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ciworkflow

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// FilterIntegrationMatrix loads .github/config/test-integration-matrix.yaml
// and keeps only the distro entries whose platform and the scenario entries
// whose flow appear in the given comma-separated lists (case-insensitive).
// It returns the filtered `matrix` object as compact JSON for use as a GHA
// job matrix.
func FilterIntegrationMatrix(configPath, platformsCSV, flowsCSV string) (string, error) {
	platforms, err := csvSet(platformsCSV, "platforms")
	if err != nil {
		return "", err
	}
	flows, err := csvSet(flowsCSV, "flows")
	if err != nil {
		return "", err
	}

	raw, err := os.ReadFile(configPath)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", configPath, err)
	}
	var doc struct {
		Matrix struct {
			Distro   []map[string]any `yaml:"distro"`
			Scenario []map[string]any `yaml:"scenario"`
		} `yaml:"matrix"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return "", fmt.Errorf("parse %s: %w", configPath, err)
	}

	distro := make([]map[string]any, 0, len(doc.Matrix.Distro))
	for _, d := range doc.Matrix.Distro {
		if platform, _ := d["platform"].(string); platforms[platform] {
			distro = append(distro, d)
		}
	}
	scenario := make([]map[string]any, 0, len(doc.Matrix.Scenario))
	for _, s := range doc.Matrix.Scenario {
		if flow, _ := s["flow"].(string); flows[flow] {
			scenario = append(scenario, s)
		}
	}

	b, err := json.Marshal(map[string]any{"distro": distro, "scenario": scenario})
	if err != nil {
		return "", fmt.Errorf("marshal matrix: %w", err)
	}
	return string(b), nil
}

// CompactJSON validates and compacts a caller-provided matrix JSON override.
func CompactJSON(input string) (string, error) {
	var v any
	if err := json.Unmarshal([]byte(input), &v); err != nil {
		return "", fmt.Errorf("invalid matrix JSON: %w", err)
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func csvSet(csv, name string) (map[string]bool, error) {
	set := map[string]bool{}
	for _, item := range strings.Split(csv, ",") {
		item = strings.ToLower(strings.TrimSpace(item))
		if item != "" {
			set[item] = true
		}
	}
	if len(set) == 0 {
		return nil, fmt.Errorf("%s input must not be empty", name)
	}
	return set, nil
}
