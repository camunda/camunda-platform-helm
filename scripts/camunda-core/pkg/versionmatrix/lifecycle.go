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

package versionmatrix

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	BucketAlpha           = "alpha"
	BucketSupportStandard = "supportStandard"
	BucketSupportExtended = "supportExtended"
	BucketEndOfLife       = "endOfLife"
)

// Lifecycle holds the per-minor support-lifecycle facts from the
// camundaSupportLifecycle block of charts/chart-versions.yaml. Dates are
// ISO 8601 (YYYY-MM-DD).
type Lifecycle struct {
	Released        string `yaml:"released"`
	StdSupportUntil string `yaml:"stdSupportUntil"`
	EOLSince        string `yaml:"eolSince"`
	LatestChart     string `yaml:"latestChart"`
	Note            string `yaml:"note"`
}

type ChartAutomation struct {
	RoutineVersions []string `yaml:"routineVersions"`
}

// ChartVersionsConfig is the parsed charts/chart-versions.yaml.
type ChartVersionsConfig struct {
	ChartAutomation         ChartAutomation      `yaml:"chartAutomation"`
	CamundaSupportLifecycle map[string]Lifecycle `yaml:"camundaSupportLifecycle"`
}

// ChartVersionsPath returns the charts/chart-versions.yaml path under repoRoot.
func ChartVersionsPath(repoRoot string) string {
	return filepath.Join(repoRoot, "charts", "chart-versions.yaml")
}

// LoadChartVersionsConfig reads and validates charts/chart-versions.yaml.
func LoadChartVersionsConfig(path string) (*ChartVersionsConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var cfg ChartVersionsConfig
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &cfg, nil
}

// BucketOf returns the bucket name a minor is classified under, or "".
func (c *ChartVersionsConfig) BucketOf(minor string) string {
	lifecycle, ok := c.CamundaSupportLifecycle[minor]
	if !ok {
		return ""
	}
	switch {
	case lifecycle.EOLSince != "":
		return BucketEndOfLife
	case lifecycle.Released == "":
		return BucketAlpha
	case lifecycle.StdSupportUntil != "":
		return BucketSupportStandard
	default:
		return BucketSupportExtended
	}
}

func (c *ChartVersionsConfig) AllMinors() []string {
	minors := make([]string, 0, len(c.CamundaSupportLifecycle))
	for minor := range c.CamundaSupportLifecycle {
		minors = append(minors, minor)
	}
	return SortAppVersionsDescending(minors)
}

func (c *ChartVersionsConfig) MinorsInBucket(bucket string) []string {
	var minors []string
	for _, minor := range c.AllMinors() {
		if c.BucketOf(minor) == bucket {
			minors = append(minors, minor)
		}
	}
	return minors
}

func (c *ChartVersionsConfig) ActiveVersions() []string {
	return slices.Clone(c.ChartAutomation.RoutineVersions)
}

func (c *ChartVersionsConfig) LatestStable() (string, error) {
	for _, minor := range c.AllMinors() {
		if c.CamundaSupportLifecycle[minor].Released != "" {
			return minor, nil
		}
	}
	return "", fmt.Errorf("camundaSupportLifecycle contains no released minor")
}

func (c *ChartVersionsConfig) Validate() error {
	var errs []string
	if len(c.ChartAutomation.RoutineVersions) == 0 {
		errs = append(errs, "chartAutomation.routineVersions must not be empty")
	}
	if len(c.CamundaSupportLifecycle) == 0 {
		errs = append(errs, "camundaSupportLifecycle must not be empty")
	}
	seen := map[string]bool{}
	for _, minor := range c.ChartAutomation.RoutineVersions {
		if seen[minor] {
			errs = append(errs, fmt.Sprintf("minor %s is repeated in chartAutomation.routineVersions", minor))
		}
		seen[minor] = true
		if _, ok := c.CamundaSupportLifecycle[minor]; !ok {
			errs = append(errs, fmt.Sprintf("minor %s has no camundaSupportLifecycle entry", minor))
		} else if c.BucketOf(minor) == BucketEndOfLife {
			errs = append(errs, fmt.Sprintf("end-of-life minor %s cannot have routine automation", minor))
		}
	}
	for _, minor := range c.AllMinors() {
		lifecycle := c.CamundaSupportLifecycle[minor]
		if lifecycle.Released == "" && (lifecycle.StdSupportUntil != "" || lifecycle.EOLSince != "" || lifecycle.LatestChart != "") {
			errs = append(errs, fmt.Sprintf("minor %s has release metadata but is missing released", minor))
		}
		errs = append(errs, lifecycle.validateDates(minor)...)
	}
	if len(errs) > 0 {
		return fmt.Errorf("chart-versions lifecycle validation failed:\n  - %s", joinLines(errs))
	}
	return nil
}

// validateDates checks that every set date parses as ISO 8601 and that the
// end-of-support/EOL dates come after the release date.
func (lc Lifecycle) validateDates(minor string) []string {
	var errs []string
	parse := func(field, value string) (time.Time, bool) {
		if value == "" {
			return time.Time{}, false
		}
		t, err := time.Parse("2006-01-02", value)
		if err != nil {
			errs = append(errs, fmt.Sprintf("minor %s: %s %q is not a valid YYYY-MM-DD date", minor, field, value))
			return time.Time{}, false
		}
		return t, true
	}
	released, hasReleased := parse("released", lc.Released)
	if until, ok := parse("stdSupportUntil", lc.StdSupportUntil); ok && hasReleased && !until.After(released) {
		errs = append(errs, fmt.Sprintf("minor %s: stdSupportUntil %s is not after released %s", minor, lc.StdSupportUntil, lc.Released))
	}
	if eol, ok := parse("eolSince", lc.EOLSince); ok && hasReleased && !eol.After(released) {
		errs = append(errs, fmt.Sprintf("minor %s: eolSince %s is not after released %s", minor, lc.EOLSince, lc.Released))
	}
	return errs
}

func joinLines(lines []string) string {
	out := ""
	for i, l := range lines {
		if i > 0 {
			out += "\n  - "
		}
		out += l
	}
	return out
}
