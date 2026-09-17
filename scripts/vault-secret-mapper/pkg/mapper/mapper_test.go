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

package mapper

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"scripts/camunda-core/pkg/logging"
	"testing"
)

func TestRequiredEnvVars(t *testing.T) {
	tests := []struct {
		name    string
		mapping string
		want    []string
	}{
		{
			name:    "empty mapping",
			mapping: "",
			want:    []string{},
		},
		{
			name:    "single entry single var",
			mapping: "ci/path OPENAI_API_KEY;",
			want:    []string{"OPENAI_API_KEY"},
		},
		{
			name:    "semicolon separated",
			mapping: "ci/path A;ci/path B;ci/path C;",
			want:    []string{"A", "B", "C"},
		},
		{
			name:    "newline separated",
			mapping: "ci/path A\nci/path B\n",
			want:    []string{"A", "B"},
		},
		{
			name:    "dedupes repeated vars",
			mapping: "ci/path A;ci/other A;ci/path B;",
			want:    []string{"A", "B"},
		},
		{
			name:    "uses aliases when present",
			mapping: "ci/path KEY1 | ALIAS1;",
			want:    []string{"ALIAS1"},
		},
		{
			name:    "comma separated keys",
			mapping: "ci/path KEY1,KEY2;",
			want:    []string{"KEY1", "KEY2"},
		},
		{
			name:    "skips comments and blanks",
			mapping: "# a comment\nci/path A;\n\nci/path B;",
			want:    []string{"A", "B"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RequiredEnvVars(tt.mapping)
			if len(got) == 0 && len(tt.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("RequiredEnvVars(%q) = %v, want %v", tt.mapping, got, tt.want)
			}
		})
	}
}

func TestGenerateStrictFailsOnMissing(t *testing.T) {
	out := filepath.Join(t.TempDir(), "secret.yaml")
	overrides := map[string]string{"PRESENT": "v"} // MISSING is absent

	// Non-strict: omits MISSING, succeeds, writes a file.
	if err := Generate("ci/path PRESENT,MISSING;", "s", out, overrides); err != nil {
		t.Fatalf("Generate (non-strict) should succeed, got %v", err)
	}
	if _, err := os.Stat(out); err != nil {
		t.Errorf("non-strict should write the secret file: %v", err)
	}

	// Strict: fails because MISSING is unset.
	if err := GenerateStrict("ci/path PRESENT,MISSING;", "s", out, overrides); err == nil {
		t.Error("GenerateStrict should fail when a mapped var is unset")
	}

	// Strict with everything present: succeeds.
	full := map[string]string{"PRESENT": "v", "MISSING": "w"}
	if err := GenerateStrict("ci/path PRESENT,MISSING;", "s", out, full); err != nil {
		t.Errorf("GenerateStrict should succeed when all vars set, got %v", err)
	}
}

// perVarMissingWarnMsg is the message currently emitted by generate() for
// EVERY unset/empty mapped variable (mapper.go ~L95). The planned fix demotes
// this to Debug and folds the detail into the existing summary event instead.
const perVarMissingWarnMsg = "Environment variable empty or missing, omitting from secret"

type capturedLogEvent struct {
	Level       string   `json:"level"`
	Message     string   `json:"message"`
	Var         string   `json:"var"`
	Missing     *int     `json:"missing"`
	MissingVars []string `json:"missingVars"`
}

func decodeLogEvents(t *testing.T, raw []byte) []capturedLogEvent {
	t.Helper()
	var events []capturedLogEvent
	for _, line := range bytes.Split(raw, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var ev capturedLogEvent
		if err := json.Unmarshal(line, &ev); err != nil {
			t.Fatalf("decode log line %q: %v", line, err)
		}
		events = append(events, ev)
	}
	return events
}

func containsAll(haystack, want []string) bool {
	for _, w := range want {
		found := false
		for _, h := range haystack {
			if h == w {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// TestMapper_MissingVarsSummarisedNotPerVariable locks the intended fix for the
// per-variable Warn log spam in generate() (mapper.go ~L95): at default/Info
// level, missing variables must be reported ONCE via the existing summary
// event (mapper.go ~L103), carrying both a `missing` count and a `missingVars`
// list, with per-variable events demoted to Debug (still visible when Debug
// is explicitly enabled) and no warn summary at all when nothing is missing.
func TestMapper_MissingVarsSummarisedNotPerVariable(t *testing.T) {
	const mapping = "ci/path PRESENT_VAR,MISSING_VAR_1,MISSING_VAR_2;"
	overrides := map[string]string{"PRESENT_VAR": "value"}
	expectedMissing := []string{"MISSING_VAR_1", "MISSING_VAR_2"}

	t.Run("default level summarises missing vars without per-variable warns", func(t *testing.T) {
		var buf bytes.Buffer
		if err := logging.Setup(logging.Options{Writer: &buf, UseJSON: true, ColorEnabled: false}); err != nil {
			t.Fatalf("logging.Setup: %v", err)
		}

		out := filepath.Join(t.TempDir(), "secret.yaml")
		if err := Generate(mapping, "s", out, overrides); err != nil {
			t.Fatalf("Generate: %v", err)
		}

		events := decodeLogEvents(t, buf.Bytes())

		for _, ev := range events {
			if ev.Message == perVarMissingWarnMsg {
				t.Errorf("unexpected per-variable warn event at default level: %+v", ev)
			}
		}

		var warnEvents []capturedLogEvent
		for _, ev := range events {
			if ev.Level == "warn" {
				warnEvents = append(warnEvents, ev)
			}
		}
		if len(warnEvents) != 1 {
			t.Fatalf("expected exactly one warn event, got %d: %+v", len(warnEvents), warnEvents)
		}

		summary := warnEvents[0]
		if summary.Missing == nil || *summary.Missing != len(expectedMissing) {
			t.Errorf("summary warn event field 'missing' = %v, want %d", summary.Missing, len(expectedMissing))
		}
		if !containsAll(summary.MissingVars, expectedMissing) {
			t.Errorf("summary warn event field 'missingVars' = %v, want to contain %v", summary.MissingVars, expectedMissing)
		}
	})

	t.Run("debug level still exposes per-variable detail", func(t *testing.T) {
		var buf bytes.Buffer
		if err := logging.Setup(logging.Options{Writer: &buf, UseJSON: true, ColorEnabled: false, LevelString: "debug"}); err != nil {
			t.Fatalf("logging.Setup: %v", err)
		}

		out := filepath.Join(t.TempDir(), "secret.yaml")
		if err := Generate(mapping, "s", out, overrides); err != nil {
			t.Fatalf("Generate: %v", err)
		}

		events := decodeLogEvents(t, buf.Bytes())

		var perVarVars []string
		for _, ev := range events {
			if ev.Message == perVarMissingWarnMsg {
				perVarVars = append(perVarVars, ev.Var)
			}
		}
		if len(perVarVars) != len(expectedMissing) {
			t.Fatalf("expected %d per-variable debug events, got %d: %v", len(expectedMissing), len(perVarVars), perVarVars)
		}
		if !containsAll(perVarVars, expectedMissing) {
			t.Errorf("per-variable debug events var= %v, want to contain %v", perVarVars, expectedMissing)
		}
	})

	t.Run("nothing missing emits no warn summary", func(t *testing.T) {
		var buf bytes.Buffer
		if err := logging.Setup(logging.Options{Writer: &buf, UseJSON: true, ColorEnabled: false}); err != nil {
			t.Fatalf("logging.Setup: %v", err)
		}

		out := filepath.Join(t.TempDir(), "secret.yaml")
		allPresent := map[string]string{"PRESENT_VAR": "value"}
		if err := Generate("ci/path PRESENT_VAR;", "s", out, allPresent); err != nil {
			t.Fatalf("Generate: %v", err)
		}

		events := decodeLogEvents(t, buf.Bytes())
		for _, ev := range events {
			if ev.Level == "warn" {
				t.Errorf("unexpected warn event when nothing is missing: %+v", ev)
			}
		}
	})
}
