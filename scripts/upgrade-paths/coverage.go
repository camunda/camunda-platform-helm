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

	"gopkg.in/yaml.v3"
)

// Coverage status values.
const (
	StatusCovered   = "covered"
	StatusPartial   = "partial"
	StatusUncovered = "uncovered"
)

// CoverageCategory is one class of breaking change and how the harness treats it.
type CoverageCategory struct {
	ID     string `yaml:"id" json:"id"`
	Title  string `yaml:"title" json:"title"`
	Status string `yaml:"status" json:"status"`
	Stage  string `yaml:"stage" json:"stage"`
	Detail string `yaml:"detail" json:"detail"`
}

// Coverage is the parsed manifest.
type Coverage struct {
	Categories []CoverageCategory `yaml:"categories" json:"categories"`
}

// ForStage marks checks owned by another stage as not exercised by this run.
func (c Coverage) ForStage(stage string) Coverage {
	out := Coverage{Categories: make([]CoverageCategory, len(c.Categories))}
	copy(out.Categories, c.Categories)
	for i := range out.Categories {
		cat := &out.Categories[i]
		if cat.Stage != "" && cat.Stage != stage {
			cat.Status = StatusUncovered
			if cat.Stage != "none" {
				cat.Detail = fmt.Sprintf("Checked by the %s stage, not this %s run. %s", cat.Stage, stage, cat.Detail)
			}
		}
	}
	return out
}

// CoveragePath is the manifest location.
func CoveragePath(repoRoot string) string {
	return filepath.Join(repoRoot, "test", "upgrade-paths", "coverage.yaml")
}

// LoadCoverage reads and validates the manifest. A missing manifest is not an
// error; the report then omits the coverage section.
func LoadCoverage(repoRoot string) (Coverage, error) {
	var c Coverage
	b, err := os.ReadFile(CoveragePath(repoRoot))
	if err != nil {
		if os.IsNotExist(err) {
			return c, nil
		}
		return c, fmt.Errorf("read coverage manifest: %w", err)
	}
	if err := yaml.Unmarshal(b, &c); err != nil {
		return c, fmt.Errorf("parse coverage manifest: %w", err)
	}
	seen := map[string]bool{}
	for _, cat := range c.Categories {
		if cat.ID == "" || cat.Title == "" {
			return Coverage{}, fmt.Errorf("coverage manifest: id and title are required")
		}
		if seen[cat.ID] {
			return Coverage{}, fmt.Errorf("coverage manifest: duplicate id %q", cat.ID)
		}
		seen[cat.ID] = true
		switch cat.Status {
		case StatusCovered, StatusPartial, StatusUncovered:
		default:
			return Coverage{}, fmt.Errorf("coverage manifest: %s has unknown status %q", cat.ID, cat.Status)
		}
	}
	return c, nil
}

// Gaps returns the categories this run does not fully check.
func (c Coverage) Gaps() []CoverageCategory {
	var out []CoverageCategory
	for _, cat := range c.Categories {
		if cat.Status != StatusCovered {
			out = append(out, cat)
		}
	}
	return out
}

// Counts tallies categories by status.
func (c Coverage) Counts() map[string]int {
	m := map[string]int{}
	for _, cat := range c.Categories {
		m[cat.Status]++
	}
	return m
}
