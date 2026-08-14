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
	"os"
	"path/filepath"
	"reflect"
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
