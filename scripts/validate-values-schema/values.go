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

package main

import (
	"encoding/json"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

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
