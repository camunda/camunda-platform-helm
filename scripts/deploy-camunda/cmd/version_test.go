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

package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestVersionCommandOutput(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		contains []string
		lines    int
	}{
		{
			name:     "full stamp",
			args:     nil,
			contains: []string{"revision:", "committed:", "modified:", "go:"},
			lines:    4,
		},
		{
			name:     "short prints revision only",
			args:     []string{"--short"},
			contains: nil,
			lines:    1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newVersionCommand()
			out := &bytes.Buffer{}
			cmd.SetOut(out)
			cmd.SetErr(out)
			cmd.SetArgs(tc.args)

			if err := cmd.Execute(); err != nil {
				t.Fatalf("version command failed: %v", err)
			}

			got := out.String()
			for _, want := range tc.contains {
				if !strings.Contains(got, want) {
					t.Errorf("output missing %q:\n%s", want, got)
				}
			}
			lines := strings.Split(strings.TrimSpace(got), "\n")
			if len(lines) != tc.lines {
				t.Errorf("expected %d line(s), got %d:\n%s", tc.lines, len(lines), got)
			}
			for _, line := range lines {
				if strings.TrimSpace(line) == "" {
					t.Errorf("empty line in output:\n%s", got)
				}
			}
		})
	}
}

func TestReadBuildStampNeverEmpty(t *testing.T) {
	stamp := readBuildStamp()
	fields := map[string]string{
		"revision":  stamp.Revision,
		"committed": stamp.Committed,
		"modified":  stamp.Modified,
		"go":        stamp.GoVersion,
	}
	for name, value := range fields {
		if value == "" {
			t.Errorf("%s is empty; expected a value or %q", name, unknownBuildValue)
		}
	}
}
