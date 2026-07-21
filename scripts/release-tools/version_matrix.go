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
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"scripts/camunda-core/pkg/versionmatrix"
)

// runVersionMatrix renders version-matrix README files. Modes (exactly one):
//
//	--readme <app>  regenerate version-matrix/camunda-<app>/README.md in full
//	                from its version-matrix.json (summary table + per-version
//	                sections).
//	--index         scan version-matrix/ dirs, write version-matrix/README.md.
//
// Both modes read the lifecycle classification from charts/chart-versions.yaml
// and fail loudly when a minor is missing from it.
func runVersionMatrix(args []string) error {
	fs := flag.NewFlagSet("version-matrix", flag.ContinueOnError)
	var app string
	var index bool
	fs.StringVar(&app, "readme", "", "app version (e.g. 8.7) — regenerates version-matrix/camunda-<app>/README.md")
	fs.BoolVar(&index, "index", false, "render version-matrix/README.md from all camunda-* dirs")
	if err := fs.Parse(args); err != nil {
		return err
	}
	switch {
	case app != "" && index:
		return fmt.Errorf("--readme and --index are mutually exclusive")
	case app != "":
		return renderVersionMatrixReadme(app)
	case index:
		return renderVersionMatrixIndex()
	default:
		return fmt.Errorf("--readme <app> or --index is required")
	}
}

// loadMatrixEntries reads and parses one app's version-matrix.json.
func loadMatrixEntries(app string) ([]versionmatrix.ChartEntry, error) {
	matrixFile := filepath.Join("version-matrix", "camunda-"+app, "version-matrix.json")
	data, err := os.ReadFile(matrixFile)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", matrixFile, err)
	}
	var entries []versionmatrix.ChartEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parse %s: %w", matrixFile, err)
	}
	return entries, nil
}

// renderVersionMatrixReadme regenerates the per-app README in full from the
// app's version-matrix.json and the lifecycle config.
func renderVersionMatrixReadme(app string) error {
	cfg, err := versionmatrix.LoadChartVersionsConfig(versionmatrix.ChartVersionsPath("."))
	if err != nil {
		return err
	}
	bucket := cfg.BucketOf(app)
	if bucket == "" {
		return fmt.Errorf("minor %s is not classified in charts/chart-versions.yaml", app)
	}
	entries, err := loadMatrixEntries(app)
	if err != nil {
		return err
	}

	outPath := filepath.Join("version-matrix", "camunda-"+app, "README.md")
	existing, err := os.ReadFile(outPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read %s: %w", outPath, err)
	}

	readme := versionmatrix.RenderMinorReadme(app, entries, bucket,
		cfg.CamundaSupportLifecycle[app], versionmatrix.ParseReadmeSections(string(existing)))
	if err := os.WriteFile(outPath, []byte(readme), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", outPath, err)
	}
	fmt.Fprintf(os.Stderr, "wrote %s (%d chart versions)\n", outPath, len(entries))
	return nil
}

func renderVersionMatrixIndex() error {
	cfg, err := versionmatrix.LoadChartVersionsConfig(versionmatrix.ChartVersionsPath("."))
	if err != nil {
		return err
	}

	dir := "version-matrix"
	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read %s: %w", dir, err)
	}

	entriesByApp := make(map[string][]versionmatrix.ChartEntry)
	for _, de := range dirEntries {
		if !de.IsDir() || !strings.HasPrefix(de.Name(), "camunda-") {
			continue
		}
		app := strings.TrimPrefix(de.Name(), "camunda-")
		charts, err := loadMatrixEntries(app)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				// Frozen minors (e.g. 8.0–8.2, 1.3) have a README but no JSON.
				continue
			}
			return err
		}
		if cfg.BucketOf(app) == "" {
			return fmt.Errorf("version-matrix/camunda-%s has data but %s is not classified in charts/chart-versions.yaml", app, app)
		}
		entriesByApp[app] = versionmatrix.SortEntriesDescending(charts)
	}

	readme, err := versionmatrix.RenderIndex(cfg, entriesByApp)
	if err != nil {
		return err
	}
	outPath := filepath.Join(dir, "README.md")
	if err := os.WriteFile(outPath, []byte(readme), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", outPath, err)
	}
	fmt.Fprintf(os.Stderr, "wrote %s (%d app versions with data)\n", outPath, len(entriesByApp))
	return nil
}
