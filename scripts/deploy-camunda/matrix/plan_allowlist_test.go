// Copyright 2026 Camunda Services GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package matrix

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

var localUses = regexp.MustCompile(`uses:\s*\./\.github/(workflows/[\w.-]+\.ya?ml|actions/[\w.-]+)`)

var scriptsReference = regexp.MustCompile(`scripts/([A-Za-z0-9_.-]+)`)

var chartCIScriptWaivers = map[string]string{
	"list-chart-image-commits.sh": "diagnostic output only: both call sites in test-integration-runner.yaml are `if: always()` with `continue-on-error: true`, so it cannot change a chart-CI result",
}

func chartCIUsesClosure(t *testing.T, repoRoot string) map[string]bool {
	t.Helper()
	reachable := map[string]bool{}
	queue := []string{"workflows/test-chart-version.yaml"}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if reachable[current] {
			continue
		}
		reachable[current] = true

		candidates := []string{filepath.Join(repoRoot, ".github", current)}
		if !filepath.IsAbs(current) && filepath.Ext(current) == "" {
			candidates = []string{
				filepath.Join(repoRoot, ".github", current, "action.yaml"),
				filepath.Join(repoRoot, ".github", current, "action.yml"),
			}
		}
		for _, candidate := range candidates {
			content, err := os.ReadFile(candidate)
			if err != nil {
				continue
			}
			for _, match := range localUses.FindAllStringSubmatch(string(content), -1) {
				queue = append(queue, match[1])
			}
		}
	}
	return reachable
}

func TestChartCIWorkflowAllowlistCoversUsesClosure(t *testing.T) {
	repoRoot := findRepoRoot(t)
	pattern := buildAllTriggers[0].Pattern
	for reached := range chartCIUsesClosure(t, repoRoot) {
		if filepath.Ext(reached) == "" {
			continue
		}
		path := ".github/" + reached
		if !pattern.MatchString(path) {
			t.Errorf("%s is reachable from test-chart-version.yaml but is not in chartCIWorkflows; "+
				"a change to it would no longer build any chart version", path)
		}
	}
}

func TestChartCIWorkflowAllowlistEntriesExist(t *testing.T) {
	repoRoot := findRepoRoot(t)
	for _, name := range chartCIWorkflows {
		path := filepath.Join(repoRoot, ".github", "workflows", name+".yaml")
		if _, err := os.Stat(path); err != nil {
			t.Errorf("chartCIWorkflows entry %q has no workflow at %s", name, path)
		}
	}
}

func TestDeployRelevantScriptAllowlistEntriesExist(t *testing.T) {
	repoRoot := findRepoRoot(t)
	for _, name := range deployRelevantScriptDirs {
		info, err := os.Stat(filepath.Join(repoRoot, "scripts", name))
		if err != nil || !info.IsDir() {
			t.Errorf("deployRelevantScriptDirs entry %q is not a directory under scripts/", name)
		}
	}
	for _, name := range deployRelevantScriptFiles {
		if _, err := os.Stat(filepath.Join(repoRoot, "scripts", name)); err != nil {
			t.Errorf("deployRelevantScriptFiles entry %q does not exist under scripts/", name)
		}
	}
}

func TestDeployRelevantScriptAllowlistCoversChartCIReferences(t *testing.T) {
	repoRoot := findRepoRoot(t)
	allowed := map[string]bool{}
	for _, name := range deployRelevantScriptDirs {
		allowed[name] = true
	}
	for _, name := range deployRelevantScriptFiles {
		allowed[name] = true
	}

	for reached := range chartCIUsesClosure(t, repoRoot) {
		candidates := []string{filepath.Join(repoRoot, ".github", reached)}
		if filepath.Ext(reached) == "" {
			candidates = []string{
				filepath.Join(repoRoot, ".github", reached, "action.yaml"),
				filepath.Join(repoRoot, ".github", reached, "action.yml"),
			}
		}
		for _, candidate := range candidates {
			content, err := os.ReadFile(candidate)
			if err != nil {
				continue
			}
			for _, match := range scriptsReference.FindAllStringSubmatch(string(content), -1) {
				segment := match[1]
				if allowed[segment] {
					continue
				}
				if reason, waived := chartCIScriptWaivers[segment]; waived {
					t.Logf("scripts/%s is referenced by .github/%s but deliberately not build-all: %s", segment, reached, reason)
					continue
				}
				t.Errorf("scripts/%s is referenced by .github/%s but is neither in the build-all allowlist nor in "+
					"chartCIScriptWaivers; a change to it would no longer build any chart version", segment, reached)
			}
		}
	}
}
