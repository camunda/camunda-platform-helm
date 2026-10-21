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
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReleaseHistory(t *testing.T) {
	for _, tc := range []struct {
		name, before, app, version, wantTag, wantRef string
		missingSource                                bool
	}{
		{"latest", "", "8.9", "14.11.1", "camunda-platform-8.9-14.11.2", "ccccccc", false},
		{"unpublished", "14.11.3", "8.9", "14.11.1", "camunda-platform-8.9-14.11.2", "ccccccc", false},
		{"historical replay", "14.11.1", "8.9", "14.11.0", "camunda-platform-8.9-14.11.0", "aaaaaaa", false},
		{"bootstrap", "", "8.11", "16.0.0-alpha1", "camunda-platform-8.11-16.0.0-alpha0", "refs/tags/camunda-platform-8.11-16.0.0-alpha0", false},
		{"missing source", "", "8.9", "14.11.1", "", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			chart, releasesFile := filepath.Join(dir, "Chart.yaml"), filepath.Join(dir, "releases.json")
			if err := os.WriteFile(chart, []byte("name: camunda-platform\nversion: "+tc.version+"\nappVersion: "+tc.app+".x\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			var releases []publishedRelease
			for i, version := range []string{"14.11.0", "14.11.1", "14.11.2"} {
				releases = append(releases, publishedRelease{Tag: "camunda-platform-8.9-" + version, PublishedAt: time.Date(2026, 10, i+1, 12, 0, 0, 0, time.UTC)})
			}
			data, err := json.Marshal([][]publishedRelease{releases})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(releasesFile, data, 0o644); err != nil {
				t.Fatal(err)
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ref := filepath.Base(r.URL.Path)
				sha := map[string]string{"14.11.0-rc": "aaaaaaa", "14.11.2-rc": "ccccccc"}[ref]
				if r.Method != http.MethodGet || sha == "" || r.URL.Query().Get("with_tag") != "true" {
					t.Errorf("unexpected predecessor lookup: %s %s", r.Method, r.URL)
				}
				tags := fmt.Sprintf(`[{"name":%q}]`, ref)
				if !tc.missingSource {
					tags = fmt.Sprintf(`[{"name":%q},{"name":%q}]`, ref, strings.TrimSuffix(ref, "-rc")+"-dev-"+sha)
				}
				if _, err := fmt.Fprintf(w, `{"tags":%s}`, tags); err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()
			args := []string{"--chart-yaml", chart, "--releases-file", releasesFile, "--api", server.URL, "--repo", "projects/test/repositories/camunda-platform"}
			if tc.before != "" {
				args = append(args, "--before-version", tc.before)
			}
			out, ref, err := captureToolOutput(t, runReleaseHistory, args)
			if tc.missingSource {
				if err == nil || ref != "" || len(out) != 0 {
					t.Fatalf("missing source was accepted: %v %q %v", out, ref, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if ref != tc.wantRef || out["previous_release_tag"] != tc.wantTag {
				t.Fatalf("predecessor = %v %q, want %s %s", out, ref, tc.wantTag, tc.wantRef)
			}
		})
	}
}
