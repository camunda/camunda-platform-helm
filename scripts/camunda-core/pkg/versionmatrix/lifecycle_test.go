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
	"slices"
	"strings"
	"testing"
)

func validYAML() string {
	return `
chartAutomation: { routineVersions: ["8.10", "8.9"] }
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

func TestRoutineAutomationIndependentOfLifecycle(t *testing.T) {
	t.Parallel()
	cfg, err := loadFromString(t, `
chartAutomation:
  routineVersions: ["8.10", "8.9", "8.7"]
camundaSupportLifecycle:
  "8.10": { note: "preview" }
  "8.9": { released: "2026-04-14", stdSupportUntil: "2027-10-13" }
  "8.7": { released: "2025-04-08", stdSupportUntil: "2026-10-13" }
  "8.6": { released: "2024-10-08" }
  "8.2": { released: "2022-10-11", eolSince: "2024-10-08", latestChart: "8.2.34" }
`)
	if err != nil {
		t.Fatalf("LoadChartVersionsConfig: %v", err)
	}
	if got := cfg.ActiveVersions(); !slices.Equal(got, []string{"8.10", "8.9", "8.7"}) {
		t.Fatalf("ActiveVersions = %v", got)
	}
	for minor, want := range map[string]string{
		"8.10": BucketAlpha,
		"8.9":  BucketSupportStandard,
		"8.7":  BucketSupportStandard,
		"8.6":  BucketSupportExtended,
		"8.2":  BucketEndOfLife,
	} {
		if got := cfg.BucketOf(minor); got != want {
			t.Errorf("BucketOf(%s) = %q, want %q", minor, got, want)
		}
	}
	before, err := RenderIndex(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ChartAutomation.RoutineVersions = []string{"8.6"}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	after, err := RenderIndex(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Error("routine automation membership changed the rendered version matrix")
	}
}

func TestLifecycleReleaseSelection(t *testing.T) {
	t.Parallel()
	cfg, err := loadFromString(t, validYAML())
	if err != nil {
		t.Fatal(err)
	}
	cfg.ChartAutomation.RoutineVersions = []string{"8.6"}
	if latest, err := cfg.LatestStable(); err != nil || latest != "8.9" {
		t.Fatalf("LatestStable = %q, %v; want 8.9", latest, err)
	}
	cfg.CamundaSupportLifecycle["8.10"] = Lifecycle{Released: "2026-10-13", StdSupportUntil: "2028-04-12"}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	if latest, err := cfg.LatestStable(); err != nil || latest != "8.10" {
		t.Fatalf("LatestStable after GA = %q, %v; want 8.10", latest, err)
	}
	if got := cfg.BucketOf("8.10"); got != BucketSupportStandard {
		t.Errorf("BucketOf(8.10) after GA = %q", got)
	}
	cfg.ChartAutomation.RoutineVersions = []string{}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("empty routine list: %v", err)
	}
}

func TestValidateFailures(t *testing.T) {
	cases := map[string]struct {
		mutate string
		want   string
	}{
		"routine minor without lifecycle entry": {
			mutate: strings.Replace(validYAML(), `  "8.9":  { released: "2026-04-14", stdSupportUntil: "2027-10-13" }`+"\n", "", 1),
			want:   "8.9 has no camundaSupportLifecycle entry",
		},
		"missing routine list": {
			mutate: strings.Replace(validYAML(), `chartAutomation: { routineVersions: ["8.10", "8.9"] }`, `chartAutomation: {}`, 1),
			want:   "chartAutomation.routineVersions is required",
		},
		"support metadata missing released": {
			mutate: strings.Replace(validYAML(),
				`"8.9":  { released: "2026-04-14", stdSupportUntil: "2027-10-13" }`,
				`"8.9":  { stdSupportUntil: "2027-10-13" }`, 1),
			want: "8.9 has release metadata but is missing released",
		},
		"eol metadata missing released": {
			mutate: strings.Replace(validYAML(),
				`"8.2":  { released: "2022-10-11", eolSince: "2024-10-08", latestChart: "8.2.34" }`,
				`"8.2":  { eolSince: "2024-10-08", latestChart: "8.2.34" }`, 1),
			want: "8.2 has release metadata but is missing released",
		},
		"eol routine minor": {
			mutate: strings.Replace(validYAML(), `["8.10", "8.9"]`, `["8.10", "8.9", "8.2"]`, 1),
			want:   "end-of-life minor 8.2 cannot have routine automation",
		},
		"duplicate routine minor": {
			mutate: strings.Replace(validYAML(), `["8.10", "8.9"]`, `["8.10", "8.9", "8.9"]`, 1),
			want:   "8.9 is repeated in chartAutomation.routineVersions",
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
