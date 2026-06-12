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

// Package releaseplease computes the dev-build release version: it detects
// prerelease/alpha→stable transitions from the chart's current version, scrapes
// the next stable version out of a `release-please --dry-run --trace` log when
// needed, and derives the dev tag + chart major. It is a pure transform of those
// inputs; the release-please CLI invocation lives in the caller.
package releaseplease

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

var (
	// Current version carries a prerelease suffix: -(alpha|beta|rc)<n>.
	prereleaseRe = regexp.MustCompile(`-(alpha|beta|rc)([0-9]+)$`)
	// A trace line introducing a version (grep `^+.*version:`).
	traceVersionLineRe = regexp.MustCompile(`^\+.*version:`)
	// Prefix stripped from a matched version line, leaving the version value.
	traceVersionStripRe = regexp.MustCompile(`^\+.*version:\s*`)
)

// Result is the computed version state, emitted to $GITHUB_ENV by the workflow.
type Result struct {
	ReleaseVersion string
	IsPrerelease   bool
	Computed       bool
	DevTag         string
	ChartMajor     string
}

// Compute derives the release version, prerelease flag and dev tag.
//
//   - currentVersion: Chart.yaml .version.
//   - stillAlpha: whether chartVersion is still listed under camundaVersions.alpha
//     (see StillAlpha) — distinguishes a prerelease bump from an alpha→stable cut.
//   - traceLog: the release-please dry-run trace (only consulted for a stable
//     release where the version must be scraped).
//   - chartDir: the chart path (matrix.chart.directory), used by the trace fallback.
//   - shortSHA: the dev commit's short SHA, for the dev tag.
func Compute(currentVersion string, stillAlpha bool, traceLog, chartDir, shortSHA string) Result {
	var r Result
	if m := prereleaseRe.FindStringSubmatch(currentVersion); m != nil {
		prereleaseType := m[1]
		num, _ := strconv.Atoi(m[2])
		base := currentVersion
		if i := strings.Index(base, "-"); i >= 0 {
			base = base[:i] // ${CURRENT_VERSION%%-*}
		}
		if stillAlpha {
			r.IsPrerelease = true
			r.ReleaseVersion = fmt.Sprintf("%s-%s%d", base, prereleaseType, num+1)
		} else {
			// alpha→stable: drop the prerelease suffix.
			r.ReleaseVersion = base
		}
		r.Computed = true
	}

	// Stable release: scrape the next version from the release-please trace.
	if !r.IsPrerelease && !r.Computed {
		if v := ScrapeTraceVersion(traceLog, chartDir); v != "" {
			r.ReleaseVersion = v
			r.Computed = true
		}
	}

	if r.Computed || r.ReleaseVersion != "" {
		r.DevTag = r.ReleaseVersion + "-dev-" + shortSHA
	} else {
		// Fallback to the current version when release-please couldn't compute.
		r.ReleaseVersion = currentVersion
		r.DevTag = currentVersion + "-dev-" + shortSHA
	}
	r.ChartMajor = r.ReleaseVersion
	if i := strings.Index(r.ChartMajor, "."); i >= 0 {
		r.ChartMajor = r.ChartMajor[:i] // ${RELEASE_VERSION%%.*}
	}
	return r
}

// ReleaseTag returns the git tag that marks a published chart release:
// camunda-platform-<minor>-<version> (e.g. camunda-platform-8.7-12.10.0). minor
// is the Camunda minor (matrix.chart.version); version is the chart version.
func ReleaseTag(minor, version string) string {
	return fmt.Sprintf("camunda-platform-%s-%s", minor, version)
}

// ScrapeTraceVersion extracts the release version from a release-please trace:
// the first `^+.*version:` line (prefix stripped), else the
// `"<chartDir>": "<version>"` manifest entry.
func ScrapeTraceVersion(traceLog, chartDir string) string {
	for _, line := range strings.Split(traceLog, "\n") {
		if traceVersionLineRe.MatchString(line) {
			return strings.TrimSpace(traceVersionStripRe.ReplaceAllString(line, ""))
		}
	}
	re := regexp.MustCompile(`"` + regexp.QuoteMeta(chartDir) + `":\s*"([^"]+)"`)
	if m := re.FindStringSubmatch(traceLog); m != nil {
		return m[1]
	}
	return ""
}

// StillAlpha reports whether chartVersion is listed under
// .camundaVersions.alpha[] in chart-versions.yaml.
func StillAlpha(chartVersionsPath, chartVersion string) (bool, error) {
	data, err := os.ReadFile(chartVersionsPath)
	if err != nil {
		return false, fmt.Errorf("read %s: %w", chartVersionsPath, err)
	}
	var cv struct {
		CamundaVersions struct {
			Alpha []string `yaml:"alpha"`
		} `yaml:"camundaVersions"`
	}
	if err := yaml.Unmarshal(data, &cv); err != nil {
		return false, fmt.Errorf("parse %s: %w", chartVersionsPath, err)
	}
	for _, v := range cv.CamundaVersions.Alpha {
		if v == chartVersion {
			return true, nil
		}
	}
	return false, nil
}
