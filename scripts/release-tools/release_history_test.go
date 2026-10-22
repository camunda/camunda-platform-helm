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

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type releaseSourceFixture struct {
	root, bin, log, a, b, c string
}

func sourceTestWrite(t *testing.T, path string, data []byte, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, mode); err != nil {
		t.Fatal(err)
	}
}

func sourceTestCommand(t *testing.T, name string, args ...string) string {
	t.Helper()
	data, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, data)
	}
	return strings.TrimSpace(string(data))
}

func newReleaseSourceFixture(t *testing.T) *releaseSourceFixture {
	t.Helper()
	repo, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	f := &releaseSourceFixture{root: t.TempDir(), bin: t.TempDir(), log: filepath.Join(t.TempDir(), "calls")}
	for _, path := range []string{".tool-versions", ".github/config/cliff.toml", "scripts/set-prs-version-label.sh"} {
		data, err := os.ReadFile(filepath.Join(repo, path))
		if err != nil {
			t.Fatal(err)
		}
		sourceTestWrite(t, filepath.Join(f.root, path), data, 0o644)
	}
	t.Chdir(f.root)
	sourceTestCommand(t, "git", "init", "-b", "main", "--quiet")
	sourceTestCommand(t, "git", "config", "user.name", "Release tests")
	sourceTestCommand(t, "git", "config", "user.email", "release-tests@example.invalid")
	sourceTestCommand(t, "git", "config", "commit.gpgsign", "false")
	sourceTestCommand(t, "git", "config", "tag.gpgsign", "false")
	sourceTestCommand(t, "git", "config", "core.hooksPath", t.TempDir())
	chart := "charts/camunda-platform-8.9/"
	sourceTestWrite(t, chart+"Chart.yaml", []byte("name: camunda-platform\nversion: 14.11.0\nappVersion: 8.9.x\n"), 0o644)
	var commits []string
	for _, message := range []string{"chore: initial chart", "fix: included in RC (#101)", "fix: merged after RC (#102)"} {
		sourceTestWrite(t, chart+"templates/change.yaml", []byte(message+"\n"), 0o644)
		sourceTestCommand(t, "git", "add", ".")
		sourceTestCommand(t, "git", "commit", "--quiet", "-m", message)
		commits = append(commits, sourceTestCommand(t, "git", "rev-parse", "HEAD"))
	}
	f.a, f.b, f.c = commits[0], commits[1], commits[2]
	sourceTestCommand(t, "git", "tag", "camunda-platform-8.9-14.11.0", f.b)
	t.Setenv("PATH", f.bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("SOURCE_CALLS", f.log)
	return f
}

func TestReleaseHistory(t *testing.T) {
	f := newReleaseSourceFixture(t)
	for _, tag := range []string{
		"camunda-platform-8.9-14.11.1", "camunda-platform-8.9-14.11.2",
		"camunda-platform-8.10-15.0.0-alpha4", "camunda-platform-8.10-15.0.0-alpha5",
		"camunda-platform-8.11-16.0.0-alpha0",
	} {
		sourceTestCommand(t, "git", "tag", tag, f.c)
	}
	for _, tc := range []struct{ name, app, version, want string }{
		{"audited predecessor", "8.9", "14.11.2", "5713eb88aaaa65df4ee2e3bfa75ef505bc36a024"},
		{"published version", "8.9", "14.11.1", "9ad9c5302c85bc8d4690bc9c959f47f735d4c864"},
		{"correct tag", "8.9", "14.11.3", "refs/tags/camunda-platform-8.9-14.11.2"},
		{"stable after prerelease", "8.10", "15.0.0", "d8dfa78d08efd336f79741d4b043e823333101ba"},
		{"bootstrap", "8.11", "16.0.0-alpha1", "refs/tags/camunda-platform-8.11-16.0.0-alpha0"},
		{"no predecessor", "8.12", "17.0.0-alpha1", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			chart := filepath.Join(t.TempDir(), "Chart.yaml")
			sourceTestWrite(t, chart, []byte("name: camunda-platform\nversion: "+tc.version+"\nappVersion: "+tc.app+".x\n"), 0o644)
			_, ref, err := captureToolOutput(t, runReleaseHistory, []string{"--chart-yaml", chart})
			if tc.want == "" {
				if err == nil {
					t.Fatalf("expected no predecessor, got %q", ref)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if ref != tc.want {
				t.Fatalf("previous = %q, want %q", ref, tc.want)
			}
		})
	}
}

func TestVersionLabelScriptUsesArtifactRange(t *testing.T) {
	for _, missing := range []string{"", "sourceSHA", "previousSourceSHA", "chartVersion"} {
		t.Run("missing="+missing, func(t *testing.T) {
			f := newReleaseSourceFixture(t)
			sourceTestWrite(t, filepath.Join(f.bin, "gh"), []byte(`#!/usr/bin/env bash
set -euo pipefail
printf 'gh %s\n' "$*" >> "$SOURCE_CALLS"
case "$1" in
  label) test "$2" = create ;;
  api) if [[ "$2" != graphql ]]; then test "$2" = repos/camunda/camunda-platform-helm/pulls/101; fi ;;
  pr) test "$2" = edit; printf 'https://github.com/camunda/camunda-platform-helm/pull/%s\n' "$3" ;;
  *) echo 'unexpected operation' >&2; exit 1 ;;
esac
`), 0o755)
			t.Setenv("chartPath", "charts/camunda-platform-8.9")
			t.Setenv("chartVersion", "14.11.1")
			t.Setenv("sourceSHA", f.b)
			t.Setenv("previousSourceSHA", f.a)
			sourceTestWrite(t, "charts/camunda-platform-8.9/Chart.yaml", []byte("name: camunda-platform\nversion: 99.0.0\nappVersion: 8.10.x\n"), 0o644)
			if missing != "" {
				t.Setenv(missing, "")
			}

			output, err := exec.Command("bash", "scripts/set-prs-version-label.sh").CombinedOutput()
			if missing != "" {
				if err == nil {
					t.Fatalf("missing %s was accepted: %s", missing, output)
				}
				if calls, readErr := os.ReadFile(f.log); readErr == nil && len(calls) != 0 {
					t.Fatalf("invalid inputs caused a GitHub operation: %s", calls)
				}
				return
			}
			if err != nil {
				t.Fatalf("label selector failed: %v\n%s", err, output)
			}
			calls, err := os.ReadFile(f.log)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(calls), "pr edit 101 --add-label version/8.9,version:8.9-14.11.1") || strings.Contains(string(calls), "102") {
				t.Fatalf("wrong PR labels: %s", calls)
			}
		})
	}
}
