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
	"fmt"
	"os"
	"path/filepath"
)

func runPreviousMinor(schemaPath, chartDir string, ignoreRoots []string) int {
	schema, err := loadJSON(schemaPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: reading schema: %v\n", err)
		return 2
	}
	strict := strictify(schema)
	deps, err := chartDependencyRoots(filepath.Join(chartDir, "Chart.yaml"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: reading Chart.yaml: %v\n", err)
		return 2
	}
	coverage, files, err := loadUpgrade(chartDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: upgrade coverage: %v\n", err)
		return 2
	}
	failures := 0
	for _, valuesPath := range files {
		values, err := loadYAML(valuesPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: reading %s: %v\n", valuesPath, err)
			return 2
		}
		vmap, ok := values.(map[string]any)
		if !ok {
			fmt.Fprintf(os.Stderr, "error: %s must contain a values object\n", valuesPath)
			return 2
		}
		for _, root := range deps {
			delete(vmap, root)
		}
		for _, root := range ignoreRoots {
			delete(vmap, root)
		}
		reportPath, err := filepath.Rel(filepath.Dir(filepath.Dir(chartDir)), valuesPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: upgrade report path: %v\n", err)
			return 2
		}
		failures += coverage.report(os.Stdout, reportPath, coverage.findUnknownKeys(strict, vmap))
	}
	if failures > 0 {
		fmt.Fprintln(os.Stderr, "Provide deprecation or removal coverage for uncovered previous-minor keys before enforcing a strict schema.")
		return 1
	}
	return 0
}
