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
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHarborTagSource(t *testing.T) {
	for _, missingSource := range []bool{false, true} {
		t.Run(fmt.Sprint(missingSource), func(t *testing.T) {
			tags := `[{"name":"14-rc-latest"},{"name":"14.11.1-rc"},{"name":"14.11.1-dev-abc1234"}]`
			if missingSource {
				tags = `[{"name":"14-rc-latest"},{"name":"14.11.1-rc"}]`
			}
			digest := "sha256:" + strings.Repeat("a", 64)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != "/projects/test/repositories/camunda-platform/artifacts/14-rc-latest" || r.URL.Query().Get("with_tag") != "true" {
					t.Errorf("unexpected artifact request: %s %s", r.Method, r.URL)
				}
				if _, err := fmt.Fprintf(w, `{"digest":%q,"tags":%s}`, digest, tags); err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()
			out, sha, err := captureToolOutput(t, runHarborTag, []string{"source", "--ref", "14-rc-latest", "--api", server.URL, "--repo", "projects/test/repositories/camunda-platform"})
			if missingSource {
				if err == nil || len(out) != 0 || sha != "" {
					t.Fatalf("missing provenance emitted a source: %v %q %v", out, sha, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if sha != "abc1234" || out["resolved_tag"] != "14.11.1-rc" || out["version"] != "14.11.1" || out["dev_tag"] != "14.11.1-dev-abc1234" || out["artifact_digest"] != digest {
				t.Fatalf("incorrect artifact source: %v %q", out, sha)
			}
		})
	}
}
