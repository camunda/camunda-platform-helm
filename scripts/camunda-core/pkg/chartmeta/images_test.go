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

package chartmeta

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writeValues(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "values.yaml"), []byte(content), 0o644); err != nil {
		t.Fatalf("write values.yaml: %v", err)
	}
	return dir
}

func TestImageSetComponentsAndGlobalFallback(t *testing.T) {
	dir := writeValues(t, `
global:
  image:
    registry: ""
    tag: ""
orchestration:
  image:
    repository: camunda/camunda
    tag: 8.10.0-alpha1
identity:
  image:
    repository: camunda/identity
    tag: "8.10.0-alpha1"
optimize:
  image:
    repository: camunda/optimize   # no tag → falls back to global.image.tag below
`)
	// give global a tag to prove the fallback resolves
	if err := os.WriteFile(filepath.Join(dir, "values.yaml"), []byte(`
global:
  image:
    registry: ""
    tag: "8.10.0-alpha1"
orchestration:
  image:
    repository: camunda/camunda
    tag: 8.10.0-alpha1
optimize:
  image:
    repository: camunda/optimize
`), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ImageSet(dir)
	if err != nil {
		t.Fatalf("ImageSet: %v", err)
	}
	want := []string{
		"docker.io/camunda/camunda:8.10.0-alpha1",
		"docker.io/camunda/optimize:8.10.0-alpha1", // tag from global fallback
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ImageSet:\n got: %v\nwant: %v", got, want)
	}
}

func TestImageSetWebModelerSharedTag(t *testing.T) {
	dir := writeValues(t, `
webModeler:
  image:
    tag: 8.10.0-alpha1-rc1
  restapi:
    image:
      repository: camunda/hub
  websockets:
    image:
      repository: camunda/hub-websockets
`)
	got, err := ImageSet(dir)
	if err != nil {
		t.Fatalf("ImageSet: %v", err)
	}
	want := []string{
		"docker.io/camunda/hub-websockets:8.10.0-alpha1-rc1",
		"docker.io/camunda/hub:8.10.0-alpha1-rc1",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ImageSet webModeler:\n got: %v\nwant: %v", got, want)
	}
}

func TestImageSetHostQualifiedRegistryNotPrefixed(t *testing.T) {
	// A component with an explicit registry host must not get a docker.io/ prefix.
	dir := writeValues(t, `
orchestration:
  image:
    registry: registry.camunda.cloud
    repository: camunda/camunda
    tag: "8.10.0"
`)
	got, err := ImageSet(dir)
	if err != nil {
		t.Fatalf("ImageSet: %v", err)
	}
	want := []string{"registry.camunda.cloud/camunda/camunda:8.10.0"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ImageSet host-qualified:\n got: %v\nwant: %v", got, want)
	}
}

func TestImageSetDigestOverridesTag(t *testing.T) {
	dir := writeValues(t, `
orchestration:
  image:
    repository: camunda/camunda
    tag: "8.10.0"
    digest: "sha256:abc123"
`)
	got, err := ImageSet(dir)
	if err != nil {
		t.Fatalf("ImageSet: %v", err)
	}
	want := []string{"docker.io/camunda/camunda@sha256:abc123"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ImageSet digest:\n got: %v\nwant: %v", got, want)
	}
}

func TestImageSetSkipsRepoWithoutTagOrDigest(t *testing.T) {
	dir := writeValues(t, `
orchestration:
  image:
    repository: camunda/camunda   # no tag, no digest, no global tag → skipped
`)
	got, err := ImageSet(dir)
	if err != nil {
		t.Fatalf("ImageSet: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty (no resolvable tag), got %v", got)
	}
}

func TestImageSetMissingValuesIsError(t *testing.T) {
	if _, err := ImageSet(t.TempDir()); err == nil {
		t.Error("expected error when values.yaml is missing, got nil")
	}
}

// TestImageSetRealChartContract is the structural contract test from the plan:
// run against the real chart values and assert the result is non-empty and
// every ref is well-formed (no render). Skips if the chart dir is absent.
func TestImageSetRealChartContract(t *testing.T) {
	for _, version := range []string{"8.10", "8.9", "8.8", "8.7"} {
		dir := filepath.Join("..", "..", "..", "..", "charts", "camunda-platform-"+version)
		if _, err := os.Stat(filepath.Join(dir, "values.yaml")); err != nil {
			continue
		}
		t.Run(version, func(t *testing.T) {
			refs, err := ImageSet(dir)
			if err != nil {
				t.Fatalf("ImageSet(%s): %v", version, err)
			}
			if len(refs) == 0 {
				t.Fatalf("ImageSet(%s) returned no images", version)
			}
			for _, r := range refs {
				name, tagOrDigest, ok := splitRef(r)
				if !ok || name == "" || tagOrDigest == "" {
					t.Errorf("malformed ref %q", r)
				}
				if strings.HasSuffix(r, ":") || strings.HasSuffix(r, "@") {
					t.Errorf("ref with empty tag/digest: %q", r)
				}
			}
			t.Logf("%s: %d images: %v", version, len(refs), refs)
		})
	}
}

func splitRef(r string) (name, tagOrDigest string, ok bool) {
	if i := strings.LastIndex(r, "@"); i > 0 {
		return r[:i], r[i+1:], true
	}
	if i := strings.LastIndex(r, ":"); i > 0 {
		return r[:i], r[i+1:], true
	}
	return "", "", false
}
