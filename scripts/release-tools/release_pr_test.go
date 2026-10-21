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
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleasePRSelectsPredecessorForRequestedVersion(t *testing.T) {
	for _, tc := range []struct {
		name       string
		releaseAs  string
		historical bool
		newer      bool
	}{
		{"next dev version", "", false, false},
		{"unpublished RC", "14.11.2", false, false},
		{"published RC rehearsal", "14.11.1", true, false},
		{"historical RC after a newer release", "14.11.1", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newReleaseSourceFixture(t)
			digest, err := fileSHA256(f.currentPackage)
			if err != nil {
				t.Fatal(err)
			}
			f.releases = append(f.releases, sourceTestPublished("camunda-platform-8.9-14.11.1", "14.11.1", digest, 2))
			if tc.newer {
				f.releases = append(f.releases, sourceTestPublished("camunda-platform-8.9-14.11.2", "14.11.2", "sha256:"+strings.Repeat("c", 64), 3))
			}
			f.writeReleases(t)
			payloadFile := filepath.Join(t.TempDir(), "release-pr-input.json")
			t.Setenv("SOURCE_RELEASE_PR_INPUT", payloadFile)
			t.Setenv("SOURCE_NPM_ROOT", t.TempDir())
			t.Setenv("GH_TOKEN", "test-token")
			t.Setenv("GITHUB_REPOSITORY", "camunda/camunda-platform-helm")
			sourceTestWrite(t, filepath.Join(f.bin, "npm"), []byte("#!/bin/sh\n[ \"$1\" = root ] && [ \"$2\" = -g ] || exit 1\nprintf '%s\\n' \"$SOURCE_NPM_ROOT\"\n"), 0o755)
			sourceTestWrite(t, filepath.Join(f.bin, "node"), []byte("#!/bin/sh\ncat > \"$SOURCE_RELEASE_PR_INPUT\"\n"), 0o755)
			source, previous, tag := f.c, f.b, "camunda-platform-8.9-14.11.1"
			if tc.historical {
				source, previous, tag = f.b, f.a, "camunda-platform-8.9-14.11.0"
			}
			args := []string{"--chart-dir", "charts/camunda-platform-8.9", "--source", source, "--dry-run"}
			if tc.releaseAs != "" {
				args = append(args, "--release-as", tc.releaseAs)
			}
			if err := runReleasePR(args); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(payloadFile)
			if err != nil {
				t.Fatal(err)
			}
			var input releasePRInput
			if err := json.Unmarshal(data, &input); err != nil {
				t.Fatal(err)
			}
			if input.PreviousSource != previous || input.PreviousTag != tag {
				t.Fatalf("predecessor=%s/%s, want %s/%s", input.PreviousTag, input.PreviousSource, tag, previous)
			}
			if len(input.Commits) != 1 || input.Commits[0].SHA != source {
				t.Fatalf("wrong source-bound commits: %+v", input.Commits)
			}
			if !input.DryRun || input.ReleaseAs != tc.releaseAs {
				t.Fatalf("wrong rehearsal arguments: %+v", input)
			}
		})
	}
}

func TestReleasePRCommitInputsAreBounded(t *testing.T) {
	f := newReleaseSourceFixture(t)
	commits, err := releaseInputCommits(context.Background(), "charts/camunda-platform-8.9", f.a, f.b)
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 1 || commits[0].SHA != f.b || !strings.Contains(commits[0].Message, "#101") {
		t.Fatalf("wrong release-please input: %+v", commits)
	}
	if len(commits[0].Files) != 1 || commits[0].Files[0] != "charts/camunda-platform-8.9/templates/change.yaml" {
		t.Fatalf("wrong commit paths: %v", commits[0].Files)
	}
}

func TestReleasePleaseUsesArtifactHistory(t *testing.T) {
	moduleRoot := sourceTestCommand(t, "npm", "root", "-g")
	program := releaseSourcePluginJS + `
(async () => {
  const root = process.argv[1];
  const {Manifest} = require(root);
  const {ManifestPlugin} = require(root + "/build/src/plugin");
  const {TagName} = require(root + "/build/src/util/tag-name");
  const {Version} = require(root + "/build/src/version");
  const path = "charts/camunda-platform-8.9";
  const previous = "a".repeat(40), late = "b".repeat(40);
  const publishing = "c".repeat(40), current = "d".repeat(40);
  const oldTag = "camunda-platform-8.9-14.11.1";
  let headFeature = false;
  const fix = {sha: late, message: "fix: late change missed by old artifact (#7456)",
    files: [path + "/templates/configmap.yaml"]};
  const github = {
    repository: {owner: "camunda", repo: "camunda-platform-helm", defaultBranch: "main"},
    releaseIterator: async function* () {
      yield {name: oldTag, tagName: oldTag, sha: publishing, notes: ""};
    },
    mergeCommitIterator: async function* () {
      yield {sha: current, message: headFeature ? "feat: beyond the selected artifact" : "chore: release bookkeeping",
        files: [path + "/Chart.yaml"]};
      yield {sha: publishing, message: "ci: publishing checkout", files: [".github/workflows/release.yaml"]};
      yield fix;
      yield {sha: previous, message: "fix: earlier release", files: [path + "/Chart.yaml"]};
    },
    getFileContents: async () => ({
      parsedContent: "name: camunda-platform\nversion: 14.11.1\nappVersion: 8.9.x\n"
    })
  };
  const config = {[path]: {
    releaseType: "helm", component: "camunda-platform-8.9", includeVInTag: false
  }};
  const manifest = new Manifest(github, "main", config,
    {[path]: Version.parse("14.11.1")}, {separatePullRequests: true});
  const broken = await manifest.buildPullRequests();
  if (broken.length !== 0) throw new Error("fixture did not reproduce the incorrect-tag history gap");
  bindArtifactSource(manifest, {chartDir: path, previousTag: oldTag,
    previousSource: previous, commits: [fix]}, github, ManifestPlugin, TagName);
  const fixed = await manifest.buildPullRequests();
  if (fixed.length !== 1 || fixed[0].version.toString() !== "14.11.2")
    throw new Error("late change did not bump the next release");
  if (!fixed[0].body.toString().includes("#7456"))
    throw new Error("late PR is missing from release-please's release body");
  headFeature = true;
  const pinned = new Manifest(github, "main", config,
    {[path]: Version.parse("14.11.1")}, {separatePullRequests: true});
  bindArtifactSource(pinned, {chartDir: path, previousTag: oldTag,
    previousSource: previous, commits: []}, github, ManifestPlugin, TagName);
  if ((await pinned.buildPullRequests()).length !== 0)
    throw new Error("release-please leaked commits outside the supplied artifact range");
  const forced = new Manifest(github, "main", {[path]: {...config[path], releaseAs: "14.12.3"}},
    {[path]: Version.parse("14.11.1")}, {separatePullRequests: true});
  bindArtifactSource(forced, {chartDir: path, previousTag: oldTag,
    previousSource: previous, commits: [fix]}, github, ManifestPlugin, TagName);
  if ((await forced.buildPullRequests())[0].version.toString() !== "14.12.3")
    throw new Error("promotion ignored the selected package version");
})().catch(error => { console.error(error.stack); process.exitCode = 1; });
`
	output, err := exec.Command("node", "-e", program, filepath.Join(moduleRoot, "release-please")).CombinedOutput()
	if err != nil {
		t.Fatalf("release-please contract failed: %v\n%s", err, output)
	}
}
