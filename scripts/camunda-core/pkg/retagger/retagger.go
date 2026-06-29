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

// Package retagger aligns release Git tags to the release-please merge commit
// that bumps Chart.yaml, fixing the window where helm-cr creates the tag
// before the version bump is committed.
package retagger

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"scripts/camunda-core/pkg/chartmeta"
)

// Chart is a release tag candidate derived from a Chart.yaml.
type Chart struct {
	Dir     string // e.g. "charts/camunda-platform-8.7"
	TagName string // e.g. "camunda-platform-8.7-12.12.1"
	Version string // chart version, e.g. "12.12.1"
}

// ListCharts scans chartsRoot for active chart directories and returns one
// Chart per non-pre-release chart found. Charts are expected at
// chartsRoot/charts/camunda-platform-8.*/.
func ListCharts(chartsRoot string) ([]Chart, error) {
	pattern := filepath.Join(chartsRoot, "charts", "camunda-platform-8.*")
	dirs, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("glob %s: %w", pattern, err)
	}
	var charts []Chart
	for _, dir := range dirs {
		chartYAML := filepath.Join(dir, "Chart.yaml")
		if _, err := os.Stat(chartYAML); err != nil {
			continue
		}
		meta, err := chartmeta.ReadPackageMetadata(chartYAML, "")
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", chartYAML, err)
		}
		// Skip alpha/pre-release versions (contain a hyphen, e.g. "15.0.0-alpha1").
		if strings.Contains(meta.Version, "-") {
			continue
		}
		charts = append(charts, Chart{
			Dir:     dir,
			TagName: meta.ReleaseTag,
			Version: meta.Version,
		})
	}
	return charts, nil
}

// GitHubClient performs the minimal GitHub tag operations needed for retagging.
type GitHubClient interface {
	// CommitSHA returns the commit SHA the named tag currently points to, or ""
	// if the tag does not exist. Annotated tags are resolved transparently.
	CommitSHA(repo, tag string) (string, error)
	// MoveTag force-updates tag to point at sha.
	MoveTag(repo, tag, sha string) error
}

// Result is the outcome of processing one chart's release tag.
type Result struct {
	TagName string
	Moved   bool
	Reason  string
}

// Run checks and retags all charts under chartsRoot. For each chart's release
// tag it compares the current tag target to targetSHA and force-moves the tag
// when they differ.
func Run(chartsRoot, repo, targetSHA string, client GitHubClient) ([]Result, error) {
	charts, err := ListCharts(chartsRoot)
	if err != nil {
		return nil, err
	}
	results := make([]Result, 0, len(charts))
	for _, ch := range charts {
		current, err := client.CommitSHA(repo, ch.TagName)
		if err != nil {
			return nil, fmt.Errorf("get SHA for %s: %w", ch.TagName, err)
		}
		if current == "" {
			results = append(results, Result{TagName: ch.TagName, Moved: false, Reason: "tag does not exist yet"})
			continue
		}
		if current == targetSHA {
			results = append(results, Result{TagName: ch.TagName, Moved: false, Reason: "already correct"})
			continue
		}
		if err := client.MoveTag(repo, ch.TagName, targetSHA); err != nil {
			return nil, fmt.Errorf("move tag %s: %w", ch.TagName, err)
		}
		results = append(results, Result{
			TagName: ch.TagName,
			Moved:   true,
			Reason:  fmt.Sprintf("moved from %s to %s", shortSHA(current), shortSHA(targetSHA)),
		})
	}
	return results, nil
}

func shortSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}
