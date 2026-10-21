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
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReleaseHistory(t *testing.T) {
	for _, tc := range []struct {
		name, before, app, version, wantTag, wantRef string
		future                                       bool
	}{
		{"latest audited", "", "8.9", "14.11.1", "camunda-platform-8.9-14.11.2", "0b34a34d2a8ebf671de995c72d90781d03a3fff2", false},
		{"unpublished", "14.11.3", "8.9", "14.11.1", "camunda-platform-8.9-14.11.2", "0b34a34d2a8ebf671de995c72d90781d03a3fff2", false},
		{"historical replay", "14.11.1", "8.9", "14.11.0", "camunda-platform-8.9-14.11.0", "9ad9c5302c85bc8d4690bc9c959f47f735d4c864", false},
		{"bootstrap", "", "8.11", "16.0.0-alpha1", "camunda-platform-8.11-16.0.0-alpha0", "refs/tags/camunda-platform-8.11-16.0.0-alpha0", false},
		{"new release uses Git", "", "8.9", "14.11.3", "camunda-platform-8.9-14.11.3", "refs/tags/camunda-platform-8.9-14.11.3", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HARBOR_API", "http://127.0.0.1:1")
			t.Setenv("HARBOR_REGISTRY_USER", "")
			t.Setenv("HARBOR_REGISTRY_PASSWORD", "")
			dir := t.TempDir()
			chart, releasesFile := filepath.Join(dir, "Chart.yaml"), filepath.Join(dir, "releases.json")
			if err := os.WriteFile(chart, []byte("name: camunda-platform\nversion: "+tc.version+"\nappVersion: "+tc.app+".x\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			var releases []publishedRelease
			for i, version := range []string{"14.11.0", "14.11.1", "14.11.2"} {
				releases = append(releases, publishedRelease{Tag: "camunda-platform-8.9-" + version, PublishedAt: time.Date(2026, 10, i+1, 12, 0, 0, 0, time.UTC)})
			}
			if tc.future {
				releases = append(releases, publishedRelease{Tag: "camunda-platform-8.9-14.11.3", PublishedAt: time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)})
			}
			data, err := json.Marshal([][]publishedRelease{releases})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(releasesFile, data, 0o644); err != nil {
				t.Fatal(err)
			}
			args := []string{"--chart-yaml", chart, "--releases-file", releasesFile}
			if tc.before != "" {
				args = append(args, "--before-version", tc.before)
			}
			out, ref, err := captureToolOutput(t, runReleaseHistory, args)
			if err != nil {
				t.Fatal(err)
			}
			if ref != tc.wantRef || out["previous_release_tag"] != tc.wantTag {
				t.Fatalf("predecessor = %v %q, want %s %s", out, ref, tc.wantTag, tc.wantRef)
			}
		})
	}
}
