// Copyright 2024 Camunda Services GmbH
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

package renovateconfigcheck

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/titanous/json5"
	"gopkg.in/yaml.v3"
)

// repoRoot returns the root of the repository by walking up from the test file.
func repoRoot(t *testing.T) string {
	t.Helper()
	// We're at scripts/renovate-config-check/, so repo root is ../../
	dir, err := os.Getwd()
	require.NoError(t, err)
	root := filepath.Join(dir, "..", "..")
	// Verify we found the right place
	_, err = os.Stat(filepath.Join(root, ".github", "renovate.json5"))
	require.NoError(t, err, "could not find .github/renovate.json5 — repo root detection failed")
	return root
}

// RenovateConfig represents the subset of renovate.json5 we need to validate.
type RenovateConfig struct {
	PackageRules   []PackageRule   `json:"packageRules"`
	CustomManagers []CustomManager `json:"customManagers"`
}

// PackageRule represents a single Renovate package rule.
type PackageRule struct {
	Description       string   `json:"description"`
	GroupName         string   `json:"groupName"`
	Versioning        string   `json:"versioning"`
	MatchFileNames    []string `json:"matchFileNames"`
	MatchManagers     []string `json:"matchManagers"`
	MatchPackageNames []string `json:"matchPackageNames"`
	MatchUpdateTypes  []string `json:"matchUpdateTypes"`
	PRCreation        string   `json:"prCreation"`
	MinimumReleaseAge string   `json:"minimumReleaseAge"`
	AddLabels         []string `json:"addLabels"`
	Schedule          []string `json:"schedule"`
	Automerge         *bool    `json:"automerge"`
	PlatformAutomerge *bool    `json:"platformAutomerge"`
	IgnoreTests       *bool    `json:"ignoreTests"`
	AutomergeType     string   `json:"automergeType"`
}

type CustomManager struct {
	FileMatch    []string `json:"fileMatch"`
	MatchStrings []string `json:"matchStrings"`
}

// ChartYAML represents the relevant fields from Chart.yaml.
type ChartYAML struct {
	Version    string `yaml:"version"`
	AppVersion string `yaml:"appVersion"`
}

// chartVersionPattern extracts the chart version number (e.g., "8.9") from a matchFileName path
// like "charts/camunda-platform-8.9/Chart.yaml".
var chartVersionPattern = regexp.MustCompile(`charts/camunda-platform-(\d+\.\d+)/`)

// minSupportedMinor is the oldest 8.x chart minor still covered by the
// camunda-platform-images Renovate rules. Charts at minor 8.6 and below are EOL
// and were dropped from those rules in #6284, so they are exempt from the
// coverage check in TestGAChartsHavePatchUpdatesEnabled.
const minSupportedMinor = 7

// isSupportedChart reports whether a chart version like "8.9" is within the
// supported window (8.7+). Unrecognized forms return true so they are checked
// rather than silently skipped.
func isSupportedChart(version string) bool {
	parts := strings.SplitN(version, ".", 2)
	if len(parts) != 2 {
		return true
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return true
	}
	return minor >= minSupportedMinor
}

// TestAlphaVersioningConsistency validates that Renovate alpha versioning rules
// only reference chart versions that are actually still in alpha.
//
// When a chart version transitions from alpha to GA (the version field in Chart.yaml
// no longer contains "-alpha"), it must be removed from the alpha versioning rules
// in .github/renovate.json5. Failing to do so blocks Renovate from detecting GA
// releases for that chart version.
func TestAlphaVersioningConsistency(t *testing.T) {
	root := repoRoot(t)
	configPath := filepath.Join(root, ".github", "renovate.json5")

	// Read and parse the Renovate config
	configBytes, err := os.ReadFile(configPath)
	require.NoError(t, err, "failed to read renovate.json5")

	var config RenovateConfig
	err = json5.Unmarshal(configBytes, &config)
	require.NoError(t, err, "failed to parse renovate.json5 as JSON5")

	// Find all package rules that enforce alpha versioning
	for _, rule := range config.PackageRules {
		if !strings.Contains(rule.Versioning, "alpha") {
			continue
		}

		// Extract chart versions referenced in this alpha rule
		chartVersions := extractChartVersions(rule.MatchFileNames)
		if len(chartVersions) == 0 {
			continue
		}

		for _, cv := range chartVersions {
			chartYAMLPath := filepath.Join(root, "charts", fmt.Sprintf("camunda-platform-%s", cv), "Chart.yaml")

			// Read Chart.yaml
			chartBytes, err := os.ReadFile(chartYAMLPath)
			if os.IsNotExist(err) {
				// Chart directory doesn't exist — might have been removed
				t.Logf("WARN: chart directory for version %s does not exist, skipping", cv)
				continue
			}
			require.NoError(t, err, "failed to read Chart.yaml for version %s", cv)

			var chart ChartYAML
			err = yaml.Unmarshal(chartBytes, &chart)
			require.NoError(t, err, "failed to parse Chart.yaml for version %s", cv)

			// If the chart version does NOT contain "-alpha", it has gone GA
			isAlpha := strings.Contains(chart.Version, "-alpha") ||
				strings.Contains(chart.Version, "SNAPSHOT")

			assert.True(t, isAlpha,
				"Chart %s (version=%s) has gone GA but is still referenced in a Renovate "+
					"alpha versioning rule.\n\n"+
					"Action: Remove %s files from the alpha versioning rules in "+
					".github/renovate.json5 and move them to the GA patch-only group.\n\n"+
					"Rule description: %q\n"+
					"Rule versioning: %q",
				cv, chart.Version, cv, rule.Description, rule.Versioning,
			)
		}
	}
}

// TestGAChartsHavePatchUpdatesEnabled validates that all GA chart versions
// are included in the patch-only image update group so Renovate can update them.
func TestGAChartsHavePatchUpdatesEnabled(t *testing.T) {
	root := repoRoot(t)
	configPath := filepath.Join(root, ".github", "renovate.json5")

	// Read and parse the Renovate config
	configBytes, err := os.ReadFile(configPath)
	require.NoError(t, err, "failed to read renovate.json5")

	var config RenovateConfig
	err = json5.Unmarshal(configBytes, &config)
	require.NoError(t, err, "failed to parse renovate.json5 as JSON5")

	// Find all GA chart versions
	gaCharts := findGACharts(t, root)

	// Find the patch-only image update rules (rules with groupName "camunda-platform-images"
	// and matchUpdateTypes containing "patch")
	var patchRuleFileNames []string
	for _, rule := range config.PackageRules {
		if containsFile(rule.MatchFileNames, "values-latest.yaml") {
			patchRuleFileNames = append(patchRuleFileNames, rule.MatchFileNames...)
		}
	}

	// For each supported GA chart, verify its values-latest.yaml is referenced in at least one rule
	for _, cv := range gaCharts {
		if !isSupportedChart(cv) {
			continue
		}
		valuesLatestPath := fmt.Sprintf("charts/camunda-platform-%s/values-latest.yaml", cv)
		found := false
		for _, f := range patchRuleFileNames {
			if f == valuesLatestPath {
				found = true
				break
			}
		}
		assert.True(t, found,
			"GA chart %s has no Renovate image update rule covering its values-latest.yaml.\n\n"+
				"Action: Add 'charts/camunda-platform-%s/values-latest.yaml' to the appropriate "+
				"camunda-platform-images group in .github/renovate.json5.",
			cv, cv,
		)
	}
}

func TestGitHubActionsAutomergePolicy(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	configBytes, err := os.ReadFile(filepath.Join(root, ".github", "renovate.json5"))
	require.NoError(t, err, "failed to read renovate.json5")

	var config RenovateConfig
	err = json5.Unmarshal(configBytes, &config)
	require.NoError(t, err, "failed to parse renovate.json5 as JSON5")

	var actionRules []PackageRule
	var manualMajorRule *PackageRule
	var scheduledMajorRule *PackageRule
	for i := range config.PackageRules {
		rule := &config.PackageRules[i]
		if containsString(rule.MatchManagers, "github-actions") {
			actionRules = append(actionRules, *rule)
		}
		switch rule.Description {
		case "Major updates require manual review to prevent breaking changes.":
			manualMajorRule = rule
		case "Schedule major updates for Friday nights.":
			scheduledMajorRule = rule
		}
	}

	require.NotNil(t, manualMajorRule)
	assert.Equal(t, []string{"!github-actions"}, manualMajorRule.MatchManagers)
	require.NotNil(t, scheduledMajorRule)
	assert.Equal(t, []string{"!github-actions"}, scheduledMajorRule.MatchManagers)

	require.Len(t, actionRules, 1, "GitHub Actions policy must be defined by one manager-wide rule")
	actionRule := actionRules[0]
	assert.Empty(t, actionRule.MatchPackageNames)
	assert.Empty(t, actionRule.MatchUpdateTypes)
	assert.Contains(t, actionRule.Schedule, "every weekend")

	require.NotNil(t, actionRule.Automerge)
	assert.True(t, *actionRule.Automerge)
	require.NotNil(t, actionRule.PlatformAutomerge)
	assert.True(t, *actionRule.PlatformAutomerge)
	require.NotNil(t, actionRule.IgnoreTests)
	assert.False(t, *actionRule.IgnoreTests)
	assert.Equal(t, "pr", actionRule.AutomergeType)

	for _, label := range []string{
		"deps/github-actions",
		"automerge",
		"automation/renovatebot",
		"kind/chore",
	} {
		assert.Contains(t, actionRule.AddLabels, label)
	}
}

func TestDigestUpdatePolicy(t *testing.T) {
	config := readRenovateConfig(t)

	for _, rule := range config.PackageRules {
		if rule.GroupName != "camunda-platform-digests" {
			continue
		}
		assert.Contains(t, rule.MatchUpdateTypes, "digest")
		assert.Equal(t, "immediate", rule.PRCreation,
			"moving snapshot digests must not be held by the global not-pending policy")
		assert.Equal(t, "6 hours", rule.MinimumReleaseAge,
			"digest updates need time to detect retracted image hashes")
		return
	}
	t.Fatal("camunda-platform-digests package rule not found")
}

func TestDigestRegexDoesNotCrossImageBlocks(t *testing.T) {
	root := repoRoot(t)
	config := readRenovateConfig(t)
	re := regexp.MustCompile(digestManagerPattern(t, config))

	for _, version := range []string{"8.8", "8.9", "8.10"} {
		path := filepath.Join(root, "charts", "camunda-platform-"+version, "values-digest.yaml")
		contents, err := os.ReadFile(path)
		require.NoError(t, err)

		assert.Equal(t, digestImageBlocks(t, contents), digestRegexMatches(re, string(contents)),
			"%s digest dependencies must stay within their YAML image block", path)
	}
}

func TestDigestRegexSkipsDigestlessBlockWithoutConsumingNextDigest(t *testing.T) {
	config := readRenovateConfig(t)
	re := regexp.MustCompile(digestManagerPattern(t, config))
	contents := `
webModeler:
  restapi:
    image:
      repository: camunda/hub
      tag: SNAPSHOT

orchestration:
  image:
    repository: camunda/camunda
    tag: 8.10-SNAPSHOT
    digest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
`

	assert.Equal(t, map[string]string{
		"camunda/camunda@8.10-SNAPSHOT": "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}, digestRegexMatches(re, contents))
}

func readRenovateConfig(t *testing.T) RenovateConfig {
	t.Helper()
	contents, err := os.ReadFile(filepath.Join(repoRoot(t), ".github", "renovate.json5"))
	require.NoError(t, err)
	var config RenovateConfig
	require.NoError(t, json5.Unmarshal(contents, &config))
	return config
}

func digestManagerPattern(t *testing.T, config RenovateConfig) string {
	t.Helper()
	for _, manager := range config.CustomManagers {
		if containsFile(manager.FileMatch, "values-digest\\.yaml$") {
			require.Len(t, manager.MatchStrings, 1)
			return manager.MatchStrings[0]
		}
	}
	t.Fatal("values-digest custom manager not found")
	return ""
}

func digestImageBlocks(t *testing.T, contents []byte) map[string]string {
	t.Helper()
	var document yaml.Node
	require.NoError(t, yaml.Unmarshal(contents, &document))
	result := map[string]string{}
	var visit func(*yaml.Node)
	visit = func(node *yaml.Node) {
		if node.Kind == yaml.MappingNode {
			fields := map[string]string{}
			for i := 0; i+1 < len(node.Content); i += 2 {
				key, value := node.Content[i], node.Content[i+1]
				if value.Kind == yaml.ScalarNode {
					fields[key.Value] = value.Value
				}
				visit(value)
			}
			if fields["repository"] != "" && fields["tag"] != "" && fields["digest"] != "" {
				result[fields["repository"]+"@"+fields["tag"]] = fields["digest"]
			}
			return
		}
		for _, child := range node.Content {
			visit(child)
		}
	}
	visit(&document)
	return result
}

func digestRegexMatches(re *regexp.Regexp, contents string) map[string]string {
	result := map[string]string{}
	groups := map[string]int{}
	for i, name := range re.SubexpNames() {
		groups[name] = i
	}
	for _, match := range re.FindAllStringSubmatch(contents, -1) {
		result[match[groups["depName"]]+"@"+match[groups["currentValue"]]] = match[groups["currentDigest"]]
	}
	return result
}

// extractChartVersions extracts unique chart version numbers from matchFileNames.
func extractChartVersions(fileNames []string) []string {
	seen := make(map[string]bool)
	var versions []string
	for _, f := range fileNames {
		matches := chartVersionPattern.FindStringSubmatch(f)
		if len(matches) >= 2 {
			v := matches[1]
			if !seen[v] {
				seen[v] = true
				versions = append(versions, v)
			}
		}
	}
	return versions
}

// findGACharts returns chart version strings (e.g., "8.9") for all charts
// whose Chart.yaml version does NOT contain "-alpha" or "SNAPSHOT".
func findGACharts(t *testing.T, root string) []string {
	t.Helper()
	chartsDir := filepath.Join(root, "charts")
	entries, err := os.ReadDir(chartsDir)
	require.NoError(t, err)

	var gaCharts []string
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "camunda-platform-8.") {
			continue
		}

		chartYAMLPath := filepath.Join(chartsDir, entry.Name(), "Chart.yaml")
		chartBytes, err := os.ReadFile(chartYAMLPath)
		if err != nil {
			continue
		}

		var chart ChartYAML
		if err := yaml.Unmarshal(chartBytes, &chart); err != nil {
			continue
		}

		isAlpha := strings.Contains(chart.Version, "-alpha") ||
			strings.Contains(chart.Version, "SNAPSHOT")
		if !isAlpha {
			// Extract version number from directory name
			version := strings.TrimPrefix(entry.Name(), "camunda-platform-")
			gaCharts = append(gaCharts, version)
		}
	}
	return gaCharts
}

// containsFile checks if any entry in the slice contains the given suffix.
func containsFile(fileNames []string, suffix string) bool {
	for _, f := range fileNames {
		if strings.HasSuffix(f, suffix) {
			return true
		}
	}
	return false
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
