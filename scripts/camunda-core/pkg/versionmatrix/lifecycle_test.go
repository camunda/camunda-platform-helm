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
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func validYAML() string {
	return `
camundaVersions:
  alpha:
    - "8.10"
  supportStandard:
    - "8.9"
  supportExtended:
    - "8.6"
  endOfLife:
    - "8.2"
camundaSupportLifecycle:
  "8.10": { note: "hub pointer" }
  "8.9":  { released: "2026-04-14", stdSupportUntil: "2027-10-13" }
  "8.6":  { released: "2024-10-08" }
  "8.2":  { released: "2022-10-11", eolSince: "2024-10-08", latestChart: "8.2.34" }
`
}

func loadFromString(t *testing.T, content string) (*ChartVersionsConfig, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "chart-versions.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return LoadChartVersionsConfig(path)
}

func TestLoadChartVersionsConfigValid(t *testing.T) {
	cfg, err := loadFromString(t, validYAML())
	if err != nil {
		t.Fatalf("LoadChartVersionsConfig: %v", err)
	}
	if got := cfg.BucketOf("8.9"); got != BucketSupportStandard {
		t.Errorf("BucketOf(8.9)=%q", got)
	}
	if got := cfg.BucketOf("7.0"); got != "" {
		t.Errorf("BucketOf(7.0)=%q want empty", got)
	}
	if lc := cfg.CamundaSupportLifecycle["8.2"]; lc.LatestChart != "8.2.34" {
		t.Errorf("lifecycle 8.2 latestChart=%q", lc.LatestChart)
	}
	all := cfg.AllMinors()
	if len(all) != 4 || all[0] != "8.10" || all[3] != "8.2" {
		t.Errorf("AllMinors=%v", all)
	}
}

func TestValidateFailures(t *testing.T) {
	cases := map[string]struct {
		mutate string
		want   string
	}{
		"bucket minor without lifecycle entry": {
			mutate: strings.Replace(validYAML(), `  "8.6":  { released: "2024-10-08" }`+"\n", "", 1),
			want:   "8.6 has no camundaSupportLifecycle entry",
		},
		"lifecycle entry without bucket": {
			mutate: validYAML() + `  "7.9": { released: "2020-01-01" }` + "\n",
			want:   "7.9 is not in any camundaVersions bucket",
		},
		"supportStandard missing stdSupportUntil": {
			mutate: strings.Replace(validYAML(),
				`"8.9":  { released: "2026-04-14", stdSupportUntil: "2027-10-13" }`,
				`"8.9":  { released: "2026-04-14" }`, 1),
			want: "8.9 (supportStandard) is missing stdSupportUntil",
		},
		"endOfLife missing eolSince": {
			mutate: strings.Replace(validYAML(),
				`"8.2":  { released: "2022-10-11", eolSince: "2024-10-08", latestChart: "8.2.34" }`,
				`"8.2":  { released: "2022-10-11" }`, 1),
			want: "8.2 (endOfLife) is missing eolSince",
		},
		"non-alpha missing released": {
			mutate: strings.Replace(validYAML(),
				`"8.6":  { released: "2024-10-08" }`,
				`"8.6":  { note: "x" }`, 1),
			want: "8.6 (supportExtended) is missing released",
		},
		"minor in two buckets": {
			mutate: strings.Replace(validYAML(),
				"  supportExtended:\n    - \"8.6\"",
				"  supportExtended:\n    - \"8.6\"\n    - \"8.9\"", 1),
			want: "8.9 is listed in 2 buckets",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := loadFromString(t, tc.mutate)
			if err == nil {
				t.Fatal("want validation error, got nil")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not contain %q", err, tc.want)
			}
		})
	}
}

func TestRepoChartVersionsFileIsValid(t *testing.T) {
	// The checked-in charts/chart-versions.yaml must always satisfy the
	// lifecycle contract — this is the loud guard for lifecycle chores.
	path := ChartVersionsPath(repoRootFromTest(t))
	if _, err := LoadChartVersionsConfig(path); err != nil {
		t.Fatalf("checked-in chart-versions.yaml invalid: %v", err)
	}
}

// repoRootFromTest walks up from the package dir to the repo root (the dir
// containing charts/chart-versions.yaml).
func repoRootFromTest(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "charts", "chart-versions.yaml")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("repo root with charts/chart-versions.yaml not found")
		}
		dir = parent
	}
}

func TestValidateDateSemantics(t *testing.T) {
	cases := map[string]struct {
		mutate string
		want   string
	}{
		"malformed date": {
			mutate: strings.Replace(validYAML(), `released: "2024-10-08"`, `released: "08.10.2024"`, 1),
			want:   `released "08.10.2024" is not a valid YYYY-MM-DD date`,
		},
		"support ends before release": {
			mutate: strings.Replace(validYAML(),
				`"8.9":  { released: "2026-04-14", stdSupportUntil: "2027-10-13" }`,
				`"8.9":  { released: "2026-04-14", stdSupportUntil: "2026-04-14" }`, 1),
			want: "stdSupportUntil 2026-04-14 is not after released 2026-04-14",
		},
		"eol before release": {
			mutate: strings.Replace(validYAML(),
				`"8.2":  { released: "2022-10-11", eolSince: "2024-10-08", latestChart: "8.2.34" }`,
				`"8.2":  { released: "2022-10-11", eolSince: "2021-01-01", latestChart: "8.2.34" }`, 1),
			want: "eolSince 2021-01-01 is not after released 2022-10-11",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := loadFromString(t, tc.mutate)
			if err == nil {
				t.Fatal("want validation error, got nil")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not contain %q", err, tc.want)
			}
		})
	}
}
