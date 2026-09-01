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

package matrix

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type chartDependencyRef struct {
	Name      string `yaml:"name"`
	Condition string `yaml:"condition"`
}

type chartMetaDependencies struct {
	Dependencies []chartDependencyRef `yaml:"dependencies"`
}

func readChartDependencies(chartDir string) ([]chartDependencyRef, error) {
	path := filepath.Join(chartDir, "Chart.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var meta chartMetaDependencies
	if err := yaml.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return meta.Dependencies, nil
}

func hasChartDependency(deps []chartDependencyRef, name string) bool {
	for _, dep := range deps {
		if dep.Name == name {
			return true
		}
	}
	return false
}

func readYAMLMap(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if doc == nil {
		doc = map[string]any{}
	}
	return doc, nil
}

// anyConditionKeyTrue splits on "," because Helm treats a condition list as an
// OR and enables the subchart when any entry resolves true.
func anyConditionKeyTrue(doc map[string]any, condition string) bool {
	for _, path := range strings.Split(condition, ",") {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		if lookupBoolPath(doc, path) {
			return true
		}
	}
	return false
}

func lookupBoolPath(doc map[string]any, dottedPath string) bool {
	segments := strings.Split(dottedPath, ".")
	var current any = doc
	for _, seg := range segments {
		node, ok := current.(map[string]any)
		if !ok {
			return false
		}
		current, ok = node[seg]
		if !ok {
			return false
		}
	}
	enabled, ok := current.(bool)
	return ok && enabled
}
