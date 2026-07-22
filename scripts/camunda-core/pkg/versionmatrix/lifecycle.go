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
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// Lifecycle bucket names, mirroring the camundaVersions keys in
// charts/chart-versions.yaml.
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

// Buckets mirrors the camundaVersions block of charts/chart-versions.yaml.
type Buckets struct {
	Alpha           []string `yaml:"alpha"`
	SupportStandard []string `yaml:"supportStandard"`
	SupportExtended []string `yaml:"supportExtended"`
	EndOfLife       []string `yaml:"endOfLife"`
}

// ChartVersionsConfig is the parsed charts/chart-versions.yaml.
type ChartVersionsConfig struct {
	CamundaVersions         Buckets              `yaml:"camundaVersions"`
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
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &cfg, nil
}

// BucketOf returns the bucket name a minor is classified under, or "".
func (c *ChartVersionsConfig) BucketOf(minor string) string {
	for bucket, minors := range map[string][]string{
		BucketAlpha:           c.CamundaVersions.Alpha,
		BucketSupportStandard: c.CamundaVersions.SupportStandard,
		BucketSupportExtended: c.CamundaVersions.SupportExtended,
		BucketEndOfLife:       c.CamundaVersions.EndOfLife,
	} {
		for _, m := range minors {
			if m == minor {
				return bucket
			}
		}
	}
	return ""
}

// AllMinors returns every minor listed in any bucket, in bucket order
// (alpha, supportStandard, supportExtended, endOfLife).
func (c *ChartVersionsConfig) AllMinors() []string {
	var all []string
	all = append(all, c.CamundaVersions.Alpha...)
	all = append(all, c.CamundaVersions.SupportStandard...)
	all = append(all, c.CamundaVersions.SupportExtended...)
	all = append(all, c.CamundaVersions.EndOfLife...)
	return all
}

// Validate enforces the bucket ⟷ lifecycle contract:
//
//   - every minor in a bucket has a camundaSupportLifecycle entry;
//   - every camundaSupportLifecycle key is classified in exactly one bucket;
//   - supportStandard entries carry stdSupportUntil;
//   - endOfLife entries carry eolSince;
//   - supportStandard, supportExtended, and endOfLife entries carry released.
//
// Violations fail loudly so a lifecycle change cannot silently drop a minor
// from the rendered matrix.
func (c *ChartVersionsConfig) Validate() error {
	seen := map[string]int{}
	for _, m := range c.AllMinors() {
		seen[m]++
	}
	var errs []string
	for m, n := range seen {
		if n > 1 {
			errs = append(errs, fmt.Sprintf("minor %s is listed in %d buckets", m, n))
		}
	}
	for _, m := range c.AllMinors() {
		lc, ok := c.CamundaSupportLifecycle[m]
		if !ok {
			errs = append(errs, fmt.Sprintf("minor %s has no camundaSupportLifecycle entry", m))
			continue
		}
		bucket := c.BucketOf(m)
		if bucket != BucketAlpha && lc.Released == "" {
			errs = append(errs, fmt.Sprintf("minor %s (%s) is missing released", m, bucket))
		}
		if bucket == BucketSupportStandard && lc.StdSupportUntil == "" {
			errs = append(errs, fmt.Sprintf("minor %s (supportStandard) is missing stdSupportUntil", m))
		}
		if bucket == BucketEndOfLife && lc.EOLSince == "" {
			errs = append(errs, fmt.Sprintf("minor %s (endOfLife) is missing eolSince", m))
		}
		errs = append(errs, lc.validateDates(m)...)
	}
	for m := range c.CamundaSupportLifecycle {
		if _, ok := seen[m]; !ok {
			errs = append(errs, fmt.Sprintf("camundaSupportLifecycle entry %s is not in any camundaVersions bucket", m))
		}
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
