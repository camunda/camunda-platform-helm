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
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type coverageClass string

const (
	deprecated  coverageClass = "deprecated"
	removed     coverageClass = "removed"
	allowlisted coverageClass = "allowlisted"
	uncovered   coverageClass = "UNCOVERED"
)

type upgradeCoverage struct {
	keys      map[string]coverageClass
	allowlist map[string]string
}

var (
	helmComments    = regexp.MustCompile(`(?s)\{\{-?\s*/\*.*?\*/\s*-?\}\}`)
	coverageInclude = regexp.MustCompile(`(?s)include\s+"camundaPlatform\.(keyDeprecated|keyRemoved|keyRenamed)"\s*\(dict(.*?)\)\s*-?\}\}`)
	coverageOldName = regexp.MustCompile(`"oldName"\s+"([^"]+)"`)
	warningBlock    = regexp.MustCompile(`(?s)\$warningMessage\s*:?=\s*printf\s+(.*?)\s*-?\}\}`)
	warningKey      = regexp.MustCompile(`DEPRECATION:\s+(?:\\")?([a-zA-Z][a-zA-Z0-9]*(?:\.[a-zA-Z0-9*]+)*)(?:\\"|\s)`)
	chartMinor      = regexp.MustCompile(`^camunda-platform-([0-9]+)\.([0-9]+)$`)
	errChartVersion = errors.New("expected camunda-platform-<major>.<minor> with minor > 0")
)

func parseUpgradeCoverage(template, allowlistSource string) (upgradeCoverage, error) {
	c := upgradeCoverage{keys: map[string]coverageClass{}, allowlist: map[string]string{}}
	executable := helmComments.ReplaceAllString(template, "")
	for _, block := range coverageInclude.FindAllStringSubmatch(executable, -1) {
		if key := coverageOldName.FindStringSubmatch(block[2]); key != nil {
			switch block[1] {
			case "keyDeprecated":
				c.keys[key[1]] = deprecated
			case "keyRemoved", "keyRenamed":
				c.keys[key[1]] = removed
			}
		}
	}
	for _, block := range warningBlock.FindAllStringSubmatch(executable, -1) {
		if !strings.Contains(block[1], `"[camunda][warning]"`) {
			continue
		}
		if key := warningKey.FindStringSubmatch(block[1]); key != nil {
			c.keys[key[1]] = deprecated
		}
	}
	if allowlistSource == "" {
		return c, nil
	}
	if err := yaml.Unmarshal([]byte(allowlistSource), &c.allowlist); err != nil {
		return c, fmt.Errorf("parse deprecation allowlist: %w", err)
	}
	return c, nil
}

func (c upgradeCoverage) classify(key string) coverageClass {
	if _, ok := c.allowlist[key]; ok {
		return allowlisted
	}
	for ancestor := key; ancestor != ""; {
		if class, ok := c.keys[ancestor]; ok {
			return class
		}
		if class, ok := c.keys[ancestor+".*"]; ok {
			return class
		}
		index := strings.LastIndexAny(ancestor, ".[")
		if index < 0 {
			break
		}
		ancestor = ancestor[:index]
	}
	return uncovered
}

func (c upgradeCoverage) findUnknownKeys(schema any, values map[string]any) []string {
	unknown := findUnknownKeys(schema, values, "")
	var result []string
	for _, key := range unknown {
		var value any = values
		for _, part := range strings.Split(key, ".") {
			object, ok := value.(map[string]any)
			if !ok {
				value = nil
				break
			}
			value = object[part]
		}
		result = append(result, c.expandUnknown(key, value)...)
	}
	return result
}

func (c upgradeCoverage) expandUnknown(key string, value any) []string {
	expand := false
	for allowed := range c.allowlist {
		expand = expand || strings.HasPrefix(allowed, key+".")
	}
	if c.classify(key) == uncovered {
		for covered := range c.keys {
			expand = expand || strings.HasPrefix(covered, key+".")
		}
	}
	object, ok := value.(map[string]any)
	if !expand || !ok || len(object) == 0 {
		return []string{key}
	}
	keys := make([]string, 0, len(object))
	for child := range object {
		keys = append(keys, child)
	}
	sort.Strings(keys)
	var result []string
	for _, child := range keys {
		result = append(result, c.expandUnknown(key+"."+child, object[child])...)
	}
	return result
}

func (c upgradeCoverage) report(output io.Writer, valuesPath string, unknown []string) int {
	counts := map[coverageClass]int{}
	for _, key := range unknown {
		class := c.classify(key)
		counts[class]++
		switch class {
		case deprecated, removed, allowlisted:
			fmt.Fprintf(output, "INFO %s: %s: %s\n", valuesPath, class, key)
		case uncovered:
			fmt.Fprintf(output, "::error::%s: UNCOVERED: %s\n", valuesPath, key)
		}
	}
	fmt.Fprintf(output, "Upgrade coverage %s: deprecated=%d removed=%d allowlisted=%d uncovered=%d\n",
		valuesPath, counts[deprecated], counts[removed], counts[allowlisted], counts[uncovered])
	return counts[uncovered]
}

func previousMinorChart(chartDir string) (string, error) {
	chartDir = filepath.Clean(chartDir)
	match := chartMinor.FindStringSubmatch(filepath.Base(chartDir))
	if match == nil {
		return "", errChartVersion
	}
	minor, err := strconv.Atoi(match[2])
	if err != nil || minor == 0 {
		return "", errChartVersion
	}
	return filepath.Join(filepath.Dir(chartDir), fmt.Sprintf("camunda-platform-%s.%d", match[1], minor-1)), nil
}

func loadUpgrade(chartDir string) (upgradeCoverage, []string, error) {
	previous, err := previousMinorChart(chartDir)
	if err != nil {
		return upgradeCoverage{}, nil, err
	}
	if _, err := os.Stat(filepath.Join(previous, "values.schema.json")); errors.Is(err, os.ErrNotExist) {
		fmt.Printf("INFO %s: previous minor has no schema baseline, skipping upgrade coverage\n", previous)
		return upgradeCoverage{}, nil, nil
	} else if err != nil {
		return upgradeCoverage{}, nil, fmt.Errorf("previous minor baseline: %w", err)
	}
	template, err := os.ReadFile(filepath.Join(chartDir, "templates/common/constraints.tpl"))
	if err != nil {
		return upgradeCoverage{}, nil, fmt.Errorf("read upgrade constraints: %w", err)
	}
	allowlist, err := os.ReadFile(filepath.Join(chartDir, "test/unit/deprecation/allowlist.yaml"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return upgradeCoverage{}, nil, fmt.Errorf("read upgrade allowlist: %w", err)
	}
	coverage, err := parseUpgradeCoverage(string(template), string(allowlist))
	if err != nil {
		return coverage, nil, err
	}
	files := []string{filepath.Join(previous, "values.yaml")}
	enterprise := filepath.Join(previous, "values-enterprise.yaml")
	if _, err := os.Stat(enterprise); err == nil {
		files = append(files, enterprise)
	} else if !errors.Is(err, os.ErrNotExist) {
		return coverage, nil, fmt.Errorf("previous enterprise values: %w", err)
	}
	return coverage, files, nil
}
