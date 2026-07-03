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

package chartmeta

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeValuesFile(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "values.yaml"), []byte(body), 0o644); err != nil {
		t.Fatalf("write values.yaml: %v", err)
	}
	return dir
}

func TestComponentImageVersionsOrchestration(t *testing.T) {
	dir := writeValuesFile(t, `
orchestration:
  image:
    tag: 8.8.1
identity:
  image:
    tag: 8.8.2
optimize:
  image:
    tag: 8.8.3
webModeler:
  image:
    tag: 8.8.4
connectors:
  image:
    tag: 8.8.5
console:
  image:
    tag: 8.8.6
`)
	got, err := ComponentImageVersions(dir, "8.8")
	if err != nil {
		t.Fatalf("ComponentImageVersions: %v", err)
	}
	want := "camunda: 8.8.1\nmanagementIdentity: 8.8.2\noptimize: 8.8.3\nwebModeler: 8.8.4\nconnectors: 8.8.5\nconsole: 8.8.6\n"
	if got != want {
		t.Errorf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestComponentImageVersions810IsOrchestration(t *testing.T) {
	dir := writeValuesFile(t, "orchestration:\n  image:\n    tag: 8.10.0\n")
	got, err := ComponentImageVersions(dir, "8.10")
	if err != nil {
		t.Fatalf("ComponentImageVersions: %v", err)
	}
	if !strings.HasPrefix(got, "camunda: 8.10.0\n") {
		t.Errorf("8.10 must use orchestration set, got:\n%s", got)
	}
}

func TestComponentImageVersionsClassicAndNA(t *testing.T) {
	// 8.7 classic; missing tags render N/A.
	dir := writeValuesFile(t, "zeebe:\n  image:\n    tag: 8.7.1\noperate:\n  image:\n    tag: 8.7.2\n")
	got, err := ComponentImageVersions(dir, "8.7")
	if err != nil {
		t.Fatalf("ComponentImageVersions: %v", err)
	}
	want := "zeebe: 8.7.1\noperate: 8.7.2\ntasklist: N/A\nidentity: N/A\noptimize: N/A\nwebModeler: N/A\nconnectors: N/A\nconsole: N/A\n"
	if got != want {
		t.Errorf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestCamundaMinorAtLeast(t *testing.T) {
	cases := []struct {
		v    string
		min  int
		want bool
	}{
		{"8.8", 8, true}, {"8.10", 8, true}, {"8.7", 8, false},
		{"8.6", 8, false}, {"8.9", 8, true},
	}
	for _, c := range cases {
		if got := camundaMinorAtLeast(c.v, c.min); got != c.want {
			t.Errorf("camundaMinorAtLeast(%q,%d)=%v want %v", c.v, c.min, got, c.want)
		}
	}
}

func TestImageOverrides(t *testing.T) {
	block, has := ImageOverrides([]ImageOverride{
		{"orchestration", "8.8-custom"},
		{"zeebe", ""},
		{"connectors", "1.2.3"},
		{"identity", ""},
	})
	if !has {
		t.Error("has should be true when any override is non-empty")
	}
	want := "orchestration: 8.8-custom\nconnectors: 1.2.3\n"
	if block != want {
		t.Errorf("block:\n%q\nwant:\n%q", block, want)
	}

	block, has = ImageOverrides([]ImageOverride{{"orchestration", ""}, {"zeebe", ""}})
	if has || block != "" {
		t.Errorf("no overrides → empty block + has=false, got %q/%v", block, has)
	}
}

// TestComponentImageVersionsRealCharts asserts the annotation builds non-empty,
// well-formed `label: tag` lines for each active chart's real values.yaml.
func TestComponentImageVersionsRealCharts(t *testing.T) {
	for _, v := range []string{"8.7", "8.8", "8.9", "8.10"} {
		dir := filepath.Join("..", "..", "..", "..", "charts", "camunda-platform-"+v)
		if _, err := os.Stat(filepath.Join(dir, "values.yaml")); err != nil {
			t.Logf("skip %s (no values.yaml)", v)
			continue
		}
		got, err := ComponentImageVersions(dir, v)
		if err != nil {
			t.Fatalf("%s: %v", v, err)
		}
		lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
		if len(lines) == 0 {
			t.Fatalf("%s: empty annotation", v)
		}
		for _, line := range lines {
			if !strings.Contains(line, ": ") {
				t.Errorf("%s: malformed line %q", v, line)
			}
		}
	}
}
