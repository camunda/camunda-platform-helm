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

package ciworkflow

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSplitRunPattern(t *testing.T) {
	tests := []struct {
		name string
		run  string
		want [][]string
	}{
		{name: "single element", run: "TestA", want: [][]string{{"TestA"}}},
		{name: "slash separates levels", run: "TestA/TestB", want: [][]string{{"TestA", "TestB"}}},
		{name: "top-level bar separates alternatives", run: "TestA/TestB|TestC", want: [][]string{{"TestA", "TestB"}, {"TestC"}}},
		{name: "bar inside parentheses stays in the element", run: "TestA/^Test(B|C)$/TestD", want: [][]string{{"TestA", "^Test(B|C)$", "TestD"}}},
		{name: "slash and bar inside brackets stay in the element", run: "TestA/[/|]", want: [][]string{{"TestA", "[/|]"}}},
		{name: "escaped slash stays in the element", run: `TestA\/TestB`, want: [][]string{{`TestA\/TestB`}}},
		{name: "trailing slash adds an empty level", run: "TestA/", want: [][]string{{"TestA", ""}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := splitRunPattern(tc.run); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("splitRunPattern(%q) = %q, want %q", tc.run, got, tc.want)
			}
		})
	}
}

func TestCompileRunPatternRejectsInvalidRegexp(t *testing.T) {
	if _, err := compileRunPattern("TestA/(TestB"); err == nil {
		t.Fatal("compileRunPattern: want error for unbalanced parenthesis, got nil")
	}
}

func TestMatchesRunPattern(t *testing.T) {
	tests := []struct {
		name string
		run  string
		test string
		want bool
	}{
		{name: "parent above the selected level", run: "TestA/TestB", test: "TestA", want: false},
		{name: "selected level", run: "TestA/TestB", test: "TestA/TestB", want: true},
		{name: "below the selected level", run: "TestA/TestB", test: "TestA/TestB/TestC", want: true},
		{name: "sibling of the selected level", run: "TestA/TestB", test: "TestA/TestX", want: false},
		{name: "anchored alternation matches", run: "TestA/^TestC(B|D)$", test: "TestA/TestCD", want: true},
		{name: "anchored alternation rejects a longer name", run: "TestA/^TestC(B|D)$", test: "TestA/TestCDE", want: false},
		{name: "second top-level alternative", run: "TestA/TestB|TestC", test: "TestC", want: true},
		{name: "parent of the first top-level alternative", run: "TestA/TestB|TestC", test: "TestA", want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pattern, err := compileRunPattern(tc.run)
			if err != nil {
				t.Fatalf("compileRunPattern: %v", err)
			}
			if got := matchesRunPattern(tc.test, pattern); got != tc.want {
				t.Errorf("matchesRunPattern(%q, %q) = %t, want %t", tc.test, tc.run, got, tc.want)
			}
		})
	}
}

func TestPassedMatches(t *testing.T) {
	longOutput := strings.Repeat("x", 128*1024)
	stream := strings.Join([]string{
		`{"Action":"start","Package":"p"}`,
		`{"Action":"output","Package":"p","Test":"TestA","Output":"=== RUN   TestA\n"}`,
		`{"Action":"output","Package":"p","Test":"TestA/TestB","Output":"` + longOutput + `\n"}`,
		`{"Action":"skip","Package":"p","Test":"TestA/TestSkipped"}`,
		`{"Action":"pass","Package":"p","Test":"TestA/TestB/TestC"}`,
		`{"Action":"pass","Package":"p","Test":"TestA/TestB"}`,
		`{"Action":"pass","Package":"p","Test":"TestA"}`,
		`not json`,
		`{"Action":"pass","Package":"p"}`,
	}, "\n") + "\n"
	pattern, err := compileRunPattern("TestA/TestB|TestA/TestSkipped")
	if err != nil {
		t.Fatalf("compileRunPattern: %v", err)
	}

	var out bytes.Buffer
	matched, err := passedMatches(strings.NewReader(stream), &out, pattern)
	if err != nil {
		t.Fatalf("passedMatches: %v", err)
	}
	if want := []string{"TestA/TestB/TestC", "TestA/TestB"}; !reflect.DeepEqual(matched, want) {
		t.Errorf("matched = %q, want %q", matched, want)
	}
	if want := "=== RUN   TestA\n" + longOutput + "\nnot json\n"; out.String() != want {
		t.Errorf("echoed output has %d bytes, want %d", out.Len(), len(want))
	}
}

const unitTestModuleTest = `package common

import "testing"

func TestTop(t *testing.T) {
	t.Run("TestMethod", func(t *testing.T) {
		t.Run("TestCaseA", func(t *testing.T) {})
		t.Run("TestCaseB", func(t *testing.T) {})
	})
	t.Run("TestBroken", func(t *testing.T) {
		t.Fatal("broken on purpose")
	})
}
`

func writeUnitTestModule(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	chartDir := filepath.Join(root, "charts", "camunda-platform-8.10")
	if err := os.MkdirAll(filepath.Join(chartDir, "test", "unit", "common"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(chartDir, "go.mod"), []byte("module example.com/chart\n\ngo 1.21\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(chartDir, "test", "unit", "common", "common_test.go"), []byte(unitTestModuleTest), 0o644); err != nil {
		t.Fatalf("write test: %v", err)
	}

	helmDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(helmDir, "helm"), []byte("#!/bin/sh\necho v3.10.3\n"), 0o755); err != nil {
		t.Fatalf("write fake helm: %v", err)
	}
	t.Setenv("PATH", helmDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return root
}

func TestRunUnitTest(t *testing.T) {
	root := writeUnitTestModule(t)

	tests := []struct {
		name        string
		run         string
		helmVersion string
		wantErr     string
		wantOut     []string
		wantNotOut  []string
	}{
		{
			name:        "selection runs matching tests",
			run:         "TestTop/TestMethod/^TestCase(A|B)$",
			helmVersion: "3.10.3",
			wantOut: []string{
				"helm v3.10.3 (",
				"=== RUN   TestTop/TestMethod/TestCaseA",
				"--- PASS: TestTop/TestMethod/TestCaseB",
				"2 passed test(s) match",
			},
		},
		{
			name:    "selection matches no test",
			run:     "TestTop/TestMethod/TestMissing",
			wantErr: "passed without running a matching test",
			wantOut: []string{"=== RUN   TestTop/TestMethod"},
		},
		{
			name:    "selected test fails",
			run:     "TestTop/TestBroken",
			wantErr: `go test ./common -run "TestTop/TestBroken"`,
			wantOut: []string{"broken on purpose"},
		},
		{
			name:        "helm on PATH is another version",
			run:         "TestTop/TestMethod",
			helmVersion: "3.22.0",
			wantErr:     "helm on PATH is v3.10.3",
			wantNotOut:  []string{"=== RUN"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			err := RunUnitTest(context.Background(), UnitTestInput{
				ChartDir:    "camunda-platform-8.10",
				Package:     "common",
				Run:         tc.run,
				HelmVersion: tc.helmVersion,
				RepoRoot:    root,
			}, &stdout, &stderr)
			if tc.wantErr == "" && err != nil {
				t.Fatalf("RunUnitTest: %v\n--- stdout ---\n%s\n--- stderr ---\n%s", err, stdout.String(), stderr.String())
			}
			if tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)) {
				t.Fatalf("RunUnitTest error = %v, want it to contain %q", err, tc.wantErr)
			}
			for _, want := range tc.wantOut {
				if !strings.Contains(stdout.String(), want) {
					t.Errorf("stdout missing %q\n--- stdout ---\n%s", want, stdout.String())
				}
			}
			for _, unwanted := range tc.wantNotOut {
				if strings.Contains(stdout.String(), unwanted) {
					t.Errorf("stdout contains %q\n--- stdout ---\n%s", unwanted, stdout.String())
				}
			}
		})
	}
}
