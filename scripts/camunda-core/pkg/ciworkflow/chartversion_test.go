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
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fakeGit struct {
	branch        string
	fetchErrs     []error
	fetchCalls    int
	mainContent   []byte
	mainExists    bool
	mainErr       error
	mainCalledFor string
}

func (f *fakeGit) CurrentBranch() (string, error) { return f.branch, nil }

func (f *fakeGit) FetchMain() error {
	f.fetchCalls++
	if len(f.fetchErrs) == 0 {
		return nil
	}
	err := f.fetchErrs[0]
	f.fetchErrs = f.fetchErrs[1:]
	return err
}

func (f *fakeGit) MainChartYaml(chartDir string) ([]byte, bool, error) {
	f.mainCalledFor = chartDir
	return f.mainContent, f.mainExists, f.mainErr
}

func TestResolveChartVersionNonUpgradeFlow(t *testing.T) {
	version, ok, err := ResolveChartVersion(ResolveChartVersionInput{SetupFlow: "install"}, &fakeGit{})
	if err != nil || ok || version != "" {
		t.Fatalf("install flow: got (%q, %t, %v), want no resolution", version, ok, err)
	}
}

func TestResolveChartVersionExplicitOverride(t *testing.T) {
	git := &fakeGit{}
	version, ok, err := ResolveChartVersion(ResolveChartVersionInput{
		SetupFlow: "upgrade-patch", ChartUpgradeVersion: "13.1.0",
	}, git)
	if err != nil || !ok || version != "13.1.0" {
		t.Fatalf("got (%q, %t, %v)", version, ok, err)
	}
	if git.fetchCalls != 0 {
		t.Errorf("explicit version must not fetch, got %d calls", git.fetchCalls)
	}
}

func TestResolveChartVersionFromMain(t *testing.T) {
	git := &fakeGit{branch: "feature", mainContent: []byte("name: camunda-platform\nversion: 13.0.2\n"), mainExists: true}
	version, ok, err := ResolveChartVersion(ResolveChartVersionInput{
		SetupFlow: "upgrade-minor", ChartDir: "camunda-platform-8.10",
		Sleep: func(time.Duration) {},
	}, git)
	if err != nil || !ok || version != "13.0.2" {
		t.Fatalf("got (%q, %t, %v)", version, ok, err)
	}
	if git.fetchCalls != 1 {
		t.Errorf("fetchCalls = %d, want 1", git.fetchCalls)
	}
	if git.mainCalledFor != "camunda-platform-8.10" {
		t.Errorf("mainCalledFor = %q", git.mainCalledFor)
	}
}

func TestResolveChartVersionOnMainSkipsFetch(t *testing.T) {
	git := &fakeGit{branch: "main", mainContent: []byte("version: 12.9.0\n"), mainExists: true}
	version, ok, err := ResolveChartVersion(ResolveChartVersionInput{
		SetupFlow: "modular-upgrade-minor", ChartDir: "camunda-platform-8.9",
	}, git)
	if err != nil || !ok || version != "12.9.0" {
		t.Fatalf("got (%q, %t, %v)", version, ok, err)
	}
	if git.fetchCalls != 0 {
		t.Errorf("on main, fetch must be skipped; got %d calls", git.fetchCalls)
	}
}

func TestResolveChartVersionFetchRetriesWithBackoff(t *testing.T) {
	git := &fakeGit{
		branch:      "feature",
		fetchErrs:   []error{errors.New("500"), errors.New("500")},
		mainContent: []byte("version: 13.0.0\n"),
		mainExists:  true,
	}
	var sleeps []time.Duration
	version, ok, err := ResolveChartVersion(ResolveChartVersionInput{
		SetupFlow: "upgrade-patch", ChartDir: "camunda-platform-8.10",
		Sleep: func(d time.Duration) { sleeps = append(sleeps, d) },
	}, git)
	if err != nil || !ok || version != "13.0.0" {
		t.Fatalf("got (%q, %t, %v)", version, ok, err)
	}
	if git.fetchCalls != 3 {
		t.Errorf("fetchCalls = %d, want 3", git.fetchCalls)
	}
	if len(sleeps) != 2 || sleeps[0] != 10*time.Second || sleeps[1] != 20*time.Second {
		t.Errorf("sleeps = %v, want [10s 20s]", sleeps)
	}
}

func TestResolveChartVersionFetchExhausted(t *testing.T) {
	git := &fakeGit{branch: "feature", fetchErrs: []error{errors.New("a"), errors.New("b"), errors.New("c")}}
	_, _, err := ResolveChartVersion(ResolveChartVersionInput{
		SetupFlow: "upgrade-patch", ChartDir: "camunda-platform-8.10",
		Sleep: func(time.Duration) {},
	}, git)
	if err == nil || !strings.Contains(err.Error(), "all git fetch attempts failed") {
		t.Fatalf("err = %v", err)
	}
	if git.fetchCalls != 3 {
		t.Errorf("fetchCalls = %d, want 3", git.fetchCalls)
	}
}

func TestResolveChartVersionLocalFallback(t *testing.T) {
	repoRoot := t.TempDir()
	dir := filepath.Join(repoRoot, "charts", "camunda-platform-8.11")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Chart.yaml"), []byte("version: 14.0.0-alpha1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git := &fakeGit{branch: "main", mainExists: false}
	version, ok, err := ResolveChartVersion(ResolveChartVersionInput{
		SetupFlow: "upgrade-minor", ChartDir: "camunda-platform-8.11", RepoRoot: repoRoot,
	}, git)
	if err != nil || !ok || version != "14.0.0-alpha1" {
		t.Fatalf("got (%q, %t, %v)", version, ok, err)
	}
}

const stampFixture = `apiVersion: v2
name: camunda-platform
# The chart version comment stays.
version: 13.0.0
dependencies:
  - name: elasticsearch
    version: 21.3.1
    repository: oci://registry-1.docker.io/bitnamicharts
annotations:
  artifacthub.io/changes: |
    - kind: fixed
      description: "something with version: 1.2.3 in it is fine, not column zero"
`

func TestStampChartVersion(t *testing.T) {
	repoRoot := t.TempDir()
	dir := filepath.Join(repoRoot, "charts", "camunda-platform-8.10")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "Chart.yaml")
	if err := os.WriteFile(path, []byte(stampFixture), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := StampChartVersion(repoRoot, "camunda-platform-8.10"); err != nil {
		t.Fatalf("StampChartVersion: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(stampFixture, "version: 13.0.0", "version: 0.0.0-ci-snapshot-8.10", 1)
	if string(got) != want {
		t.Errorf("stamped file differs:\n%s", got)
	}
}

func TestStampChartVersionRejectsAmbiguousFile(t *testing.T) {
	repoRoot := t.TempDir()
	dir := filepath.Join(repoRoot, "charts", "camunda-platform-8.10")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Chart.yaml"), []byte("version: 1\nname: x\nversion: 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := StampChartVersion(repoRoot, "camunda-platform-8.10"); err == nil {
		t.Fatal("expected error on two top-level version lines")
	}
}
