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

package cmd

import (
	"path/filepath"
	"strings"
	"testing"
)

// registryRepoRoot is the matrix package's testdata registry, reused here so the
// command test does not depend on the real charts/ tree.
const registryRepoRoot = "../matrix/testdata/registry-good"

func TestCIE2ESuiteVarsWritesRegistryDeclarations(t *testing.T) {
	repoRoot, err := filepath.Abs(registryRepoRoot)
	if err != nil {
		t.Fatalf("abs: %v", err)
	}

	for _, tc := range []struct {
		scenario string
		want     map[string]string
	}{
		{"beta", map[string]string{"e2e-full-suite": "true", "e2e-non-blocking": "true"}},
		{"alpha", map[string]string{"e2e-full-suite": "false", "e2e-non-blocking": "false"}},
		{"not-a-scenario", map[string]string{"e2e-full-suite": "false", "e2e-non-blocking": "false"}},
	} {
		t.Run(tc.scenario, func(t *testing.T) {
			outputPath := filepath.Join(t.TempDir(), "github_output")
			t.Setenv("GITHUB_OUTPUT", outputPath)

			command := newCIE2ESuiteVarsCommand()
			command.SetArgs([]string{
				"--repo-root", repoRoot,
				"--chart-dir", "camunda-platform-99.99",
				"--scenario", tc.scenario,
			})
			if err := command.Execute(); err != nil {
				t.Fatalf("execute e2e-suite-vars: %v", err)
			}

			got := map[string]string{}
			for _, line := range strings.Split(strings.TrimSpace(readFile(t, outputPath)), "\n") {
				if name, value, ok := strings.Cut(strings.TrimSpace(line), "="); ok {
					got[name] = value
				}
			}
			for name, want := range tc.want {
				if got[name] != want {
					t.Errorf("%s = %q, want %q (all outputs: %v)", name, got[name], want, got)
				}
			}
		})
	}
}
