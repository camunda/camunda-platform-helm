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

func TestReleaseVersionFailIfReleased(t *testing.T) {
	// Real lifecycle file: 8.10 is released, so it is not still-alpha.
	versionsFile, err := filepath.Abs("../../charts/chart-versions.yaml")
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name           string
		current        string // "-alpha1" with a released minor computes the stable version
		tagged         bool
		overrides      bool
		failIfReleased bool
		wantErr        bool
		wantSkip       bool
	}{
		{name: "released fallback skips", current: "15.4.1", tagged: true, failIfReleased: true, wantSkip: true},
		{name: "released computed version fails", current: "15.4.1-alpha1", tagged: true, failIfReleased: true, wantErr: true},
		{name: "released fallback with overrides fails", current: "15.4.1", tagged: true, overrides: true, failIfReleased: true, wantErr: true},
		{name: "unreleased fallback builds", current: "15.4.1", failIfReleased: true},
		{name: "guard off builds", current: "15.4.1", tagged: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := t.TempDir()
			git := func(args ...string) {
				cmd := exec.Command("git", append([]string{"-C", repo, "-c", "user.name=t", "-c", "user.email=t@t", "-c", "commit.gpgsign=false", "-c", "tag.gpgsign=false"}, args...)...)
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("git %v: %v\n%s", args, err, out)
				}
			}
			git("init", "-q")
			git("commit", "-q", "--allow-empty", "-m", "init")
			if tc.tagged {
				git("tag", "camunda-platform-8.10-15.4.1")
			}
			t.Chdir(repo)
			envFile := filepath.Join(t.TempDir(), "env")
			t.Setenv("GITHUB_ENV", envFile)
			overrides := "false"
			if tc.overrides {
				overrides = "true"
			}
			t.Setenv("HAS_IMAGE_OVERRIDES", overrides)

			err := runReleaseVersion([]string{
				"--current-version", tc.current,
				"--chart-version", "8.10",
				"--chart-versions-file", versionsFile,
				"--short-sha", "abc1234",
				"--fail-if-released=" + map[bool]string{true: "true", false: "false"}[tc.failIfReleased],
			})
			if tc.wantErr {
				if err == nil || !strings.Contains(err.Error(), "refusing to build") {
					t.Fatalf("err = %v, want refusing to build", err)
				}
			} else if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			data, _ := os.ReadFile(envFile)
			env := string(data)
			if got := strings.Contains(env, "SKIP_BUILD=true\n"); got != tc.wantSkip {
				t.Errorf("SKIP_BUILD=true written = %v, want %v; env:\n%s", got, tc.wantSkip, env)
			}
			if !tc.wantErr && !strings.Contains(env, "DEV_TAG=15.4.1-dev-abc1234\n") {
				t.Errorf("DEV_TAG not emitted; env:\n%s", env)
			}
		})
	}
}
