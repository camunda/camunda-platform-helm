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

package deploy

import (
	"strings"
	"testing"
)

const pinnedOverlay = `
global:
  image:
    tag:
orchestration:
  image:
    repository: camunda/camunda
    tag: 8.10.0
    digest: "sha256:orchestration"
connectors:
  image:
    repository: camunda/connectors-bundle
    tag: 8.10.0
    digest: "sha256:connectors"
webModeler:
  restapi:
    image:
      repository: camunda/hub
      tag: 8.10.0
      digest: "sha256:restapi"
`

func imageAt(t *testing.T, doc map[string]any, path string) map[string]any {
	t.Helper()
	for _, key := range strings.Split(path, ".") {
		doc, _ = doc[key].(map[string]any)
	}
	img, ok := doc["image"].(map[string]any)
	if !ok {
		t.Fatalf("%s has no image block", path)
	}
	return img
}

func TestResolveImages(t *testing.T) {
	for _, tc := range []struct {
		name        string
		later       string
		allowShadow bool
		want        map[string]string
		wantErr     string
	}{
		{name: "pins stay without overrides", later: "global: {ingress: {enabled: true}}", want: map[string]string{"orchestration": "camunda/camunda@sha256:orchestration", "connectors": "camunda/connectors-bundle@sha256:connectors"}},
		{name: "full override replaces the pin", later: "orchestration: {image: {registry: registry.camunda.cloud, repository: team-camunda/camunda, tag: snapshot-a1}}", want: map[string]string{"orchestration": "registry.camunda.cloud/team-camunda/camunda:snapshot-a1", "connectors": "camunda/connectors-bundle@sha256:connectors"}},
		{name: "tag override replaces the pin", later: "connectors: {image: {tag: snapshot-a1}}", want: map[string]string{"connectors": "camunda/connectors-bundle:snapshot-a1", "orchestration": "camunda/camunda@sha256:orchestration"}},
		{name: "nested override replaces only that pin", later: "webModeler: {restapi: {image: {repository: team-camunda/hub, tag: snapshot-a1}}}", want: map[string]string{"webModeler.restapi": "team-camunda/hub:snapshot-a1", "orchestration": "camunda/camunda@sha256:orchestration"}},
		{name: "restated coordinates keep the pin", later: "webModeler: {restapi: {image: {repository: camunda/hub, pullSecrets: [{name: index-docker-io}]}}}", want: map[string]string{"webModeler.restapi": "camunda/hub@sha256:restapi"}},
		{name: "own digest wins", later: "orchestration: {image: {registry: registry.camunda.cloud, repository: team-camunda/camunda, digest: 'sha256:mine'}}", want: map[string]string{"orchestration": "registry.camunda.cloud/team-camunda/camunda@sha256:mine"}},
		{name: "image override record replaces the pin", later: "orchestration: {image: {registry: '', repository: camunda/camunda, tag: snapshot-a1, digest: ''}}", want: map[string]string{"orchestration": "camunda/camunda:snapshot-a1"}},
		{name: "registry change without tag is rejected", later: "orchestration: {image: {registry: mirror.example.com}}", wantErr: "changes registry/repository without a tag"},
		{name: "allow-digest-shadow keeps the pin", later: "orchestration: {image: {registry: mirror.example.com}}", allowShadow: true, want: map[string]string{"orchestration": "mirror.example.com/camunda/camunda@sha256:orchestration"}},
		{name: "empty tag is rejected", later: "orchestration: {image: {registry: registry.camunda.cloud, repository: team-camunda/camunda, tag: ''}}", wantErr: "orchestration: "},
		{name: "null tag is rejected", later: "connectors: {image: {repository: team-camunda/connectors-bundle, tag: null}}", wantErr: "connectors: "},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			chain := []string{writeTempYAML(t, dir, "values-digest.yaml", pinnedOverlay), writeTempYAML(t, dir, "later.yaml", tc.later)}
			path, err := resolveImages(chain, 1, tc.allowShadow, dir)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) || !strings.Contains(err.Error(), "later.yaml") {
					t.Fatalf("resolveImages() error = %v, want one naming later.yaml and %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveImages(): %v", err)
			}
			resolved := readYAMLMap(t, path)
			if _, echoed := resolved["global"]; echoed {
				t.Errorf("blank global.image.tag was written to the resolved values")
			}
			for component, want := range tc.want {
				img := imageAt(t, resolved, component)
				if got := imageRef(img); got != want {
					t.Errorf("%s = %q, want %q", component, got, want)
				}
				if digest, set := img["digest"]; !strings.Contains(want, "@") && (!set || digest != nil && digest != "") {
					t.Errorf("%s digest = %v, want it cleared so Helm cannot restore the pin", component, digest)
				}
			}
		})
	}
}

func TestWriteImageOverrides(t *testing.T) {
	chart := t.TempDir()
	writeTempYAML(t, chart, "values.yaml", "orchestration: {image: {repository: camunda/camunda}}\nwebModeler: {restapi: {image: {repository: camunda/hub}}}\n")
	path, err := writeImageOverrides([]string{
		"orchestration=registry.camunda.cloud/team-camunda/camunda:8.10.0-SNAPSHOT-a1",
		"webModeler.restapi=localhost:5000/hub@sha256:abc",
	}, chart, t.TempDir())
	if err != nil {
		t.Fatalf("writeImageOverrides(): %v", err)
	}
	doc := readYAMLMap(t, path)
	for component, want := range map[string]string{
		"orchestration":      "registry.camunda.cloud/team-camunda/camunda:8.10.0-SNAPSHOT-a1",
		"webModeler.restapi": "localhost:5000/hub@sha256:abc",
	} {
		if got := imageRef(imageAt(t, doc, component)); got != want {
			t.Errorf("%s = %q, want %q", component, got, want)
		}
	}
	for _, bad := range []string{"orchestration=camunda/camunda", "orchestration", "=camunda/camunda:1", "console=camunda/console:1"} {
		if _, err := writeImageOverrides([]string{bad}, chart, t.TempDir()); err == nil {
			t.Errorf("writeImageOverrides(%q) succeeded, want an error", bad)
		}
	}
}

func TestChartRootOverlayFiles(t *testing.T) {
	chart := t.TempDir()
	digest := writeTempYAML(t, chart, "values-digest.yaml", "{}")
	if got := ChartRootOverlayFiles(chart, []string{"enterprise", "digest"}); len(got) != 1 || got[0] != digest {
		t.Fatalf("ChartRootOverlayFiles() = %v, want [%s]", got, digest)
	}
}
