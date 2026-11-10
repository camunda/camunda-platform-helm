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
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPublicReleaseResume(t *testing.T) {
	for _, tool := range []string{"bash", "jq", "sha256sum"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s is required: %v", tool, err)
		}
	}
	run := sourceTestWorkflow(t, "chart-release-public.yaml").stepRun(t, "release", "Check existing release")
	digest := fmt.Sprintf("sha256:%x", sha256.Sum256([]byte("package")))
	asset := fmt.Sprintf(`{"name":"chart.tgz","state":"uploaded","digest":%q}`, digest)
	bundle := `,{"name":"bundle","state":"uploaded"}`
	for _, tc := range []struct {
		name, status, body, tagSHA string
		wantOK, exists, bundle     bool
	}{
		{"missing", "404", `{}`, "source", true, false, false},
		{"no bundle", "200", `{"draft":false,"assets":[` + asset + `]}`, "source", true, true, false},
		{"bundle", "200", `{"draft":false,"assets":[` + asset + bundle + `]}`, "source", true, true, true},
		{"different digest", "200", `{"draft":false,"assets":[{"name":"chart.tgz","state":"uploaded","digest":"sha256:wrong"}]}`, "source", false, false, false},
		{"missing asset", "200", `{"draft":false,"assets":[]}`, "source", false, false, false},
		{"draft", "200", `{"draft":true,"assets":[` + asset + `]}`, "source", false, false, false},
		{"different source", "200", `{"draft":false,"assets":[` + asset + `]}`, "other", false, false, false},
		{"server failure", "500", `{}`, "source", false, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := resumeTempDir(t)
			if err := os.Mkdir(filepath.Join(dir, ".cr-release-packages"), 0o755); err != nil {
				t.Fatal(err)
			}
			writeResumeFile(t, filepath.Join(dir, ".cr-release-packages", "chart.tgz"), "package", 0o644)
			writeResumeFile(t, filepath.Join(dir, "curl"), "#!/bin/bash\nwhile [[ $# -gt 0 ]]; do if [[ $1 == -o ]]; then printf '%s' \"$MOCK_BODY\" > \"$2\"; shift; fi; shift; done\nprintf '%s' \"$MOCK_STATUS\"\n", 0o755)
			writeResumeFile(t, filepath.Join(dir, "git"), "#!/bin/bash\nif [[ $1 == rev-parse ]]; then printf '%s\\n' \"$MOCK_TAG_SHA\"; fi\n", 0o755)
			output := filepath.Join(dir, "output")
			cmd := exec.Command("bash", "-e", "-o", "pipefail", "-c", run)
			cmd.Dir = dir
			cmd.Env = append(os.Environ(), "PATH="+dir+":"+os.Getenv("PATH"), "RUNNER_TEMP="+dir,
				"GITHUB_OUTPUT="+output, "GH_TOKEN=test", "GITHUB_API_URL=https://example.test", "GITHUB_REPOSITORY=owner/repo",
				"RELEASE_TAG=tag", "PACKAGE_FILE=chart.tgz", "SOURCE_SHA=source", "COSIGN_BUNDLE=bundle",
				"MOCK_STATUS="+tc.status, "MOCK_BODY="+tc.body, "MOCK_TAG_SHA="+tc.tagSHA)
			result, err := cmd.CombinedOutput()
			if (err == nil) != tc.wantOK {
				t.Fatalf("error = %v, output = %s", err, result)
			}
			data, _ := os.ReadFile(output)
			if strings.Contains(string(data), "exists=true") != tc.exists {
				t.Fatalf("unexpected output: %s", data)
			}
			if tc.exists && string(data) != fmt.Sprintf("exists=true\nbundle=%t\n", tc.bundle) {
				t.Fatalf("unexpected signing outputs: %s", data)
			}
		})
	}
}

func resumeTempDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	versions, err := os.ReadFile("../../.tool-versions")
	if err != nil {
		t.Fatal(err)
	}
	writeResumeFile(t, filepath.Join(dir, ".tool-versions"), string(versions), 0o644)
	return dir
}

func writeResumeFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}

func TestPublicIndexRetriesAfterPushFailure(t *testing.T) {
	run := sourceTestWorkflow(t, "chart-release-public.yaml").stepRun(t, "release", "Run Chart Releaser - Indexing")
	dir := t.TempDir()
	writeResumeFile(t, filepath.Join(dir, "git"), "#!/bin/bash\necho fetch >> \"$CALLS\"\n", 0o755)
	writeResumeFile(t, filepath.Join(dir, "helm-cr"), "#!/bin/bash\necho index >> \"$CALLS\"\n[[ $(grep -c index \"$CALLS\") -ge 2 ]]\n", 0o755)
	writeResumeFile(t, filepath.Join(dir, "sleep"), "#!/bin/bash\n", 0o755)
	calls := filepath.Join(dir, "calls")
	cmd := exec.Command("bash", "-e", "-o", "pipefail", "-c", run)
	cmd.Env = append(os.Environ(), "PATH="+dir+":"+os.Getenv("PATH"), "CALLS="+calls,
		"CHART_RELEASER_CONFIG_FILE=config", "GITHUB_REPOSITORY_OWNER=owner", "GITHUB_REPOSITORY=owner/repo", "RELEASE_TAG=tag")
	if result, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("index retry: %v: %s", err, result)
	}
	data, err := os.ReadFile(calls)
	if err != nil || string(data) != "fetch\nindex\nfetch\nindex\n" {
		t.Fatalf("unexpected calls %q: %v", data, err)
	}
}

func TestPublicReleasePRVersionGuard(t *testing.T) {
	workflow := sourceTestWorkflow(t, "chart-release-public.yaml")
	run := workflow.stepRun(t, "post-release", "Check release-please PR version")
	for _, tc := range []struct {
		name, version, fetchSHA string
		recovery, wantOK, match bool
	}{
		{"matching fresh", "14.12.1", "abc", false, true, true},
		{"matching replay", "14.12.1", "abc", true, true, true},
		{"superseded replay", "14.12.2", "abc", true, true, false},
		{"mismatched fresh", "14.12.2", "abc", false, false, false},
		{"branch changed during lookup", "14.12.1", "def", true, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := resumeTempDir(t)
			writeResumeFile(t, filepath.Join(dir, "gh"), "#!/bin/bash\nprintf '%s\\n' '{\"number\":42,\"headRefOid\":\"abc\"}'\n", 0o755)
			writeResumeFile(t, filepath.Join(dir, "git"), "#!/bin/bash\ncase $1 in rev-parse) echo \"$MOCK_SHA\";; show) echo \"version: $MOCK_VERSION\";; esac\n", 0o755)
			writeResumeFile(t, filepath.Join(dir, "yq"), "#!/bin/bash\nread -r key version\n[[ $key == version: ]]\nprintf '%s\\n' \"$version\"\n", 0o755)
			output := filepath.Join(dir, "output")
			cmd := exec.Command("bash", "-e", "-o", "pipefail", "-c", run)
			cmd.Dir = dir
			cmd.Env = append(os.Environ(), "PATH="+dir+":"+os.Getenv("PATH"), "GITHUB_OUTPUT="+output,
				"CHART_DIR_ID=8.9", "CHART_VERSION=14.12.1", "MOCK_VERSION="+tc.version, "MOCK_SHA="+tc.fetchSHA,
				fmt.Sprintf("EXISTING_RELEASE=%t", tc.recovery))
			result, err := cmd.CombinedOutput()
			if (err == nil) != tc.wantOK {
				t.Fatalf("error = %v, output = %s", err, result)
			}
			data, _ := os.ReadFile(output)
			if tc.match && string(data) != "pr_number=42\nhead_sha=abc\n" {
				t.Fatalf("unexpected output: %s", data)
			}
			if !tc.match && len(data) != 0 {
				t.Fatalf("unexpected outputs for mismatched PR: %s", data)
			}
			if tc.name == "superseded replay" && !strings.Contains(string(result), "::notice::") {
				t.Fatalf("missing notice: %s", result)
			}
			if tc.name == "mismatched fresh" && !strings.Contains(string(result), "::error::") {
				t.Fatalf("missing error: %s", result)
			}
		})
	}
	for _, step := range workflow.Jobs["post-release"].Steps {
		switch step.Name {
		case "Stamp release date on the release-please PR":
			if !strings.Contains(step.If, "steps.release-pr.outputs.pr_number != ''") {
				t.Errorf("%s is not gated by the version check: %s", step.Name, step.If)
			}
		case "Approve and auto-merge release-please PR":
			if !strings.Contains(step.If, "steps.mark-published.outputs.pr_number != ''") ||
				!strings.Contains(step.If, "steps.mark-published.outcome == 'success'") {
				t.Errorf("%s is not gated by the marked PR: %s", step.Name, step.If)
			}
		}
	}
}

func runMarkPublished(t *testing.T, refSHA string) (string, string, []byte) {
	t.Helper()
	run := sourceTestWorkflow(t, "chart-release-public.yaml").stepRun(t, "post-release", "Mark release-please PR as published")
	dir := t.TempDir()
	writeResumeFile(t, filepath.Join(dir, "gh"), "#!/bin/bash\nif [[ $1 == pr && $2 == view ]]; then echo stale; else echo \"$*\" >> \"$CALLS\"; fi\n", 0o755)
	writeResumeFile(t, filepath.Join(dir, "git"), "#!/bin/bash\nif [[ $1 == ls-remote && $3 == refs/heads/release-please--branches--main--components--camunda-platform-8.7 ]]; then printf '%s\\t%s\\n' \"$MOCK_REF_SHA\" \"$3\"; fi\n", 0o755)
	cmd := exec.Command("bash", "-e", "-o", "pipefail", "-c", run)
	cmd.Env = append(os.Environ(), "PATH="+dir+":"+os.Getenv("PATH"), "GITHUB_OUTPUT="+filepath.Join(dir, "output"),
		"PR_NUMBER=42", "PR_SHA=stamped", "CHART_DIR_ID=8.7", "MOCK_REF_SHA="+refSHA, "CALLS="+filepath.Join(dir, "calls"))
	result, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("mark published failed: %v: %s", err, result)
	}
	return filepath.Join(dir, "output"), filepath.Join(dir, "calls"), result
}

func TestPublicReleasePRChangedAfterStamp(t *testing.T) {
	output, calls, result := runMarkPublished(t, "newer")
	if !strings.Contains(string(result), "::notice::") {
		t.Fatalf("expected skip notice: %s", result)
	}
	for _, file := range []string{output, calls} {
		if _, err := os.Stat(file); !os.IsNotExist(err) {
			t.Fatalf("unexpected %s after PR changed: %v", file, err)
		}
	}
}

func TestPublicReleasePRMarkedDespiteStaleAPIHead(t *testing.T) {
	output, calls, result := runMarkPublished(t, "stamped")
	got, err := os.ReadFile(output)
	if err != nil || !strings.Contains(string(got), "pr_number=42") {
		t.Fatalf("expected pr_number output: %v: %s", err, result)
	}
	mutations, err := os.ReadFile(calls)
	if err != nil || !strings.Contains(string(mutations), "--add-label autorelease: published") {
		t.Fatalf("expected published label: %v: %s", err, mutations)
	}
}

func TestPublicReleasePRMissing(t *testing.T) {
	run := sourceTestWorkflow(t, "chart-release-public.yaml").stepRun(t, "post-release", "Check release-please PR version")
	for _, recovery := range []bool{false, true} {
		t.Run(fmt.Sprintf("recovery=%t", recovery), func(t *testing.T) {
			dir := t.TempDir()
			writeResumeFile(t, filepath.Join(dir, "gh"), "#!/bin/bash\nexit 0\n", 0o755)
			cmd := exec.Command("bash", "-e", "-o", "pipefail", "-c", run)
			cmd.Env = append(os.Environ(), "PATH="+dir+":"+os.Getenv("PATH"), "CHART_DIR_ID=8.9",
				fmt.Sprintf("EXISTING_RELEASE=%t", recovery))
			result, err := cmd.CombinedOutput()
			level := "::error::"
			if recovery {
				level = "::notice::"
			}
			if (err == nil) != recovery || !strings.Contains(string(result), level) {
				t.Fatalf("error = %v, output = %s", err, result)
			}
		})
	}
}

func TestPublicRejectsInvalidRekorIndex(t *testing.T) {
	run := sourceTestWorkflow(t, "chart-release-public.yaml").stepRun(t, "release", "Get Cosign Rekor log index")
	run = strings.ReplaceAll(run, "${{ steps.metadata.outputs.cosign_bundle }}", "bundle.json")
	dir := resumeTempDir(t)
	writeResumeFile(t, filepath.Join(dir, "bundle.json"), `{"verificationMaterial":{"tlogEntries":[{"logIndex":"1; echo injected"}]}}`, 0o644)
	cmd := exec.Command("bash", "-e", "-o", "pipefail", "-c", run)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GITHUB_OUTPUT="+filepath.Join(dir, "output"))
	result, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(result), "Invalid Rekor log index") {
		t.Fatalf("expected invalid index rejection, error = %v, output = %s", err, result)
	}
}
