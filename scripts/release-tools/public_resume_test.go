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
	for _, tc := range []struct {
		name, status, body, tagSHA string
		wantOK, exists             bool
	}{
		{"missing", "404", `{}`, "source", true, false},
		{"matching", "200", `{"draft":false,"assets":[` + asset + `]}`, "source", true, true},
		{"different digest", "200", `{"draft":false,"assets":[{"name":"chart.tgz","state":"uploaded","digest":"sha256:wrong"}]}`, "source", false, false},
		{"missing asset", "200", `{"draft":false,"assets":[]}`, "source", false, false},
		{"draft", "200", `{"draft":true,"assets":[` + asset + `]}`, "source", false, false},
		{"different source", "200", `{"draft":false,"assets":[` + asset + `]}`, "other", false, false},
		{"server failure", "500", `{}`, "source", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
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
				"RELEASE_TAG=tag", "PACKAGE_FILE=chart.tgz", "SOURCE_SHA=source", "COSIGN_BUNDLE=bundle", "COSIGN_VERIFY=verify",
				"MOCK_STATUS="+tc.status, "MOCK_BODY="+tc.body, "MOCK_TAG_SHA="+tc.tagSHA)
			result, err := cmd.CombinedOutput()
			if (err == nil) != tc.wantOK {
				t.Fatalf("error = %v, output = %s", err, result)
			}
			data, _ := os.ReadFile(output)
			if strings.Contains(string(data), "exists=true") != tc.exists {
				t.Fatalf("unexpected output: %s", data)
			}
		})
	}
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
