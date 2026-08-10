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
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// registryRepoRoot is the matrix package's testdata registry, reused here so the
// command test does not depend on the real charts/ tree.
const registryRepoRoot = "../matrix/testdata/registry-good"

type e2eLeg struct {
	Suite      string `json:"suite"`
	Blocking   bool   `json:"blocking"`
	ShardIndex int    `json:"shard_index"`
	ShardTotal int    `json:"shard_total"`
}

func TestCIE2EMatrixWritesRegistryLegs(t *testing.T) {
	repoRoot, err := filepath.Abs(registryRepoRoot)
	if err != nil {
		t.Fatalf("abs: %v", err)
	}

	for _, tc := range []struct {
		name      string
		shortname string
		scenario  string
		want      []e2eLeg
	}{
		{"alpha defaults to blocking smoke", "alp", "alpha", []e2eLeg{{"smoke", true, 1, 1}}},
		{"beta inverts both blocking defaults", "bet", "beta", []e2eLeg{{"smoke", false, 1, 1}, {"full", true, 1, 1}}},
		{"gamma opts into a non-blocking full leg", "gam", "gamma", []e2eLeg{{"smoke", true, 1, 1}, {"full", false, 1, 1}}},
		// Shortname wins: a mismatched scenario name must not override it.
		{"shortname takes precedence", "bet", "alpha", []e2eLeg{{"smoke", false, 1, 1}, {"full", true, 1, 1}}},
		// Unknown shortname falls back to the name lookup.
		{"unknown shortname falls back to name", "zzz", "beta", []e2eLeg{{"smoke", false, 1, 1}, {"full", true, 1, 1}}},
		{"unknown scenario defaults", "", "not-a-scenario", []e2eLeg{{"smoke", true, 1, 1}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			outputPath := filepath.Join(t.TempDir(), "github_output")
			t.Setenv("GITHUB_OUTPUT", outputPath)

			command := newCIE2EMatrixCommand()
			command.SetArgs([]string{
				"--repo-root", repoRoot,
				"--chart-dir", "camunda-platform-99.99",
				"--shortname", tc.shortname,
				"--scenario", tc.scenario,
			})
			if err := command.Execute(); err != nil {
				t.Fatalf("execute e2e-matrix: %v", err)
			}

			line := strings.TrimSpace(readFile(t, outputPath))
			legsJSON, ok := strings.CutPrefix(line, "e2e-matrix=")
			if !ok {
				t.Fatalf("GITHUB_OUTPUT missing e2e-matrix assignment: %q", line)
			}
			var got []e2eLeg
			if err := json.Unmarshal([]byte(legsJSON), &got); err != nil {
				t.Fatalf("decode e2e-matrix output %q: %v", legsJSON, err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("legs = %+v, want %+v", got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Errorf("leg %d = %+v, want %+v", i, got[i], tc.want[i])
				}
			}
		})
	}
}
