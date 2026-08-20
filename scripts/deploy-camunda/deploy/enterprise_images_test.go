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
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeValuesFile drops a values file into a temp dir and returns its path.
func writeValuesFile(t *testing.T, name, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("writing %s: %v", name, err)
	}
	return path
}

// The shape of charts/camunda-platform-8.7/values-enterprise.yaml, trimmed.
const enterpriseValues = `
global:
  security:
    allowInsecureImages: true
identityPostgresql:
  image:
    registry: registry.camunda.cloud
    repository: vendor-ee/postgresql
    tag: 15.18.0-debian-12-r17
  volumePermissions:
    image:
      registry: registry.camunda.cloud
      repository: vendor-ee/os-shell
postgresql:
  image:
    registry: registry.camunda.cloud
    repository: vendor-ee/postgresql
    tag: 14.23.0-debian-12-r19
elasticsearch:
  image:
    registry: registry.camunda.cloud
    repository: vendor-ee/elasticsearch
    tag: 8.19.20
`

func TestCollectPinnedImages(t *testing.T) {
	t.Parallel()

	t.Run("collects only fully-pinned triplets, deduplicated and sorted", func(t *testing.T) {
		t.Parallel()
		path := writeValuesFile(t, "values-enterprise.yaml", enterpriseValues)
		got := collectPinnedImages([]string{path})

		want := []string{
			"registry.camunda.cloud/vendor-ee/elasticsearch:8.19.20",
			"registry.camunda.cloud/vendor-ee/postgresql:14.23.0-debian-12-r19",
			"registry.camunda.cloud/vendor-ee/postgresql:15.18.0-debian-12-r17",
		}
		if len(got) != len(want) {
			t.Fatalf("got %d images %v, want %d", len(got), got, len(want))
		}
		for i, w := range want {
			if got[i].Ref != w {
				t.Errorf("image[%d] = %q, want %q", i, got[i].Ref, w)
			}
		}
		// os-shell pins registry+repository but no tag, exactly like the real file;
		// the bash check skips it and so must we.
		for _, img := range got {
			if strings.Contains(img.Ref, "os-shell") {
				t.Errorf("os-shell has no tag and must not be collected, got %q", img.Ref)
			}
		}
	})

	t.Run("ordinary scenario values yield nothing", func(t *testing.T) {
		t.Parallel()
		path := writeValuesFile(t, "base.yaml", "orchestration:\n  enabled: true\nidentity:\n  fullURL: http://x\n")
		if got := collectPinnedImages([]string{path}); len(got) != 0 {
			t.Fatalf("expected no pinned images, got %v", got)
		}
	})

	t.Run("unreadable and empty layers are skipped", func(t *testing.T) {
		t.Parallel()
		empty := writeValuesFile(t, "empty.yaml", "")
		got := collectPinnedImages([]string{empty, "/nonexistent/values.yaml"})
		if len(got) != 0 {
			t.Fatalf("expected no images, got %v", got)
		}
	})

	t.Run("numeric tags are accepted", func(t *testing.T) {
		t.Parallel()
		path := writeValuesFile(t, "n.yaml", "c:\n  image:\n    registry: r.io\n    repository: repo/img\n    tag: 8.19\n")
		got := collectPinnedImages([]string{path})
		if len(got) != 1 || got[0].Ref != "r.io/repo/img:8.19" {
			t.Fatalf("got %v, want r.io/repo/img:8.19", got)
		}
	})
}

func TestRepositoryOf(t *testing.T) {
	t.Parallel()
	tests := []struct{ ref, want string }{
		{"registry.camunda.cloud/vendor-ee/postgresql:15.18.0", "registry.camunda.cloud/vendor-ee/postgresql"},
		{"localhost:5000/repo/img:1.0", "localhost:5000/repo/img"},
		// A registry port with no tag must not be mistaken for a tag separator.
		{"localhost:5000/repo/img", "localhost:5000/repo/img"},
		{"nginx", "nginx"},
	}
	for _, tt := range tests {
		if got := repositoryOf(tt.ref); got != tt.want {
			t.Errorf("repositoryOf(%q) = %q, want %q", tt.ref, got, tt.want)
		}
	}
}

const multiArchIndex = `{"manifests":[
  {"digest":"sha256:amd64digest","platform":{"os":"linux","architecture":"amd64"}},
  {"digest":"sha256:arm64digest","platform":{"os":"linux","architecture":"arm64"}},
  {"digest":"sha256:attestation","platform":{"os":"unknown","architecture":"unknown"}}
]}`

func TestResolveImageForPlatform(t *testing.T) {
	t.Run("index resolves but the amd64 child 404s", func(t *testing.T) {
		// The exact 2026-08-19 shape: index OK, child missing. An index-only probe
		// calls this healthy; this is the regression the whole check exists for.
		var calls []string
		orig := dockerManifestInspect
		dockerManifestInspect = func(_ context.Context, ref string) ([]byte, error) {
			calls = append(calls, ref)
			if strings.Contains(ref, "@sha256:amd64digest") {
				return nil, errors.New("manifest unknown")
			}
			return []byte(multiArchIndex), nil
		}
		defer func() { dockerManifestInspect = orig }()

		res := resolveImageForPlatform(context.Background(), "reg/vendor-ee/postgresql:15.18.0", "linux/amd64")
		if res.Err == nil {
			t.Fatal("expected the missing child to be reported")
		}
		if !strings.Contains(res.Err.Error(), "child manifest") {
			t.Errorf("error should point at the child, got %v", res.Err)
		}
		if len(calls) != 2 {
			t.Fatalf("expected an index call then a child call, got %v", calls)
		}
		if calls[1] != "reg/vendor-ee/postgresql@sha256:amd64digest" {
			t.Errorf("child fetched by the wrong ref: %q", calls[1])
		}
	})

	t.Run("index and child both resolve", func(t *testing.T) {
		orig := dockerManifestInspect
		dockerManifestInspect = func(_ context.Context, _ string) ([]byte, error) {
			return []byte(multiArchIndex), nil
		}
		defer func() { dockerManifestInspect = orig }()

		res := resolveImageForPlatform(context.Background(), "reg/img:1", "linux/amd64")
		if res.Err != nil {
			t.Fatalf("expected success, got %v", res.Err)
		}
		if res.Digest != "sha256:amd64digest" {
			t.Errorf("digest = %q", res.Digest)
		}
	})

	t.Run("single-platform image has no children to check", func(t *testing.T) {
		calls := 0
		orig := dockerManifestInspect
		dockerManifestInspect = func(_ context.Context, _ string) ([]byte, error) {
			calls++
			return []byte(`{"schemaVersion":2,"config":{}}`), nil
		}
		defer func() { dockerManifestInspect = orig }()

		if res := resolveImageForPlatform(context.Background(), "reg/img:1", "linux/amd64"); res.Err != nil {
			t.Fatalf("expected success, got %v", res.Err)
		}
		if calls != 1 {
			t.Errorf("expected a single call, got %d", calls)
		}
	})

	t.Run("platform not built is not a failure", func(t *testing.T) {
		orig := dockerManifestInspect
		dockerManifestInspect = func(_ context.Context, _ string) ([]byte, error) {
			return []byte(multiArchIndex), nil
		}
		defer func() { dockerManifestInspect = orig }()

		if res := resolveImageForPlatform(context.Background(), "reg/img:1", "linux/s390x"); res.Err != nil {
			t.Fatalf("an image never built for a platform is not broken, got %v", res.Err)
		}
	})

	t.Run("unresolvable index is reported", func(t *testing.T) {
		orig := dockerManifestInspect
		dockerManifestInspect = func(_ context.Context, _ string) ([]byte, error) {
			return nil, errors.New("manifest unknown")
		}
		defer func() { dockerManifestInspect = orig }()

		res := resolveImageForPlatform(context.Background(), "reg/img:nope", "linux/amd64")
		if res.Err == nil || !strings.Contains(res.Err.Error(), "index does not resolve") {
			t.Fatalf("expected an index failure, got %v", res.Err)
		}
	})
}

func TestCheckPinnedImages(t *testing.T) {
	t.Run("no pinned images is OK and makes no registry calls", func(t *testing.T) {
		orig := dockerManifestInspect
		dockerManifestInspect = func(_ context.Context, _ string) ([]byte, error) {
			t.Fatal("must not touch the registry when nothing is pinned")
			return nil, nil
		}
		defer func() { dockerManifestInspect = orig }()

		path := writeValuesFile(t, "base.yaml", "orchestration:\n  enabled: true\n")
		got := checkPinnedImages(context.Background(), []string{path}, "linux/amd64")
		if got.Status != StatusOK {
			t.Fatalf("status = %v, detail = %q", got.Status, got.Detail)
		}
	})

	t.Run("a broken child manifest fails the preflight", func(t *testing.T) {
		orig := dockerManifestInspect
		dockerManifestInspect = func(_ context.Context, ref string) ([]byte, error) {
			if strings.Contains(ref, "postgresql@") {
				return nil, errors.New("manifest unknown")
			}
			return []byte(multiArchIndex), nil
		}
		defer func() { dockerManifestInspect = orig }()

		path := writeValuesFile(t, "values-enterprise.yaml", enterpriseValues)
		got := checkPinnedImages(context.Background(), []string{path}, "linux/amd64")
		if got.Status != StatusFail {
			t.Fatalf("status = %v, want fail; detail = %q", got.Status, got.Detail)
		}
		if !strings.Contains(got.Detail, "vendor-ee/postgresql") {
			t.Errorf("detail should name the broken image, got %q", got.Detail)
		}
		if !strings.Contains(got.Detail, "values-enterprise.yaml") {
			t.Errorf("detail should name the source layer, got %q", got.Detail)
		}
		if !strings.Contains(got.Remediation, "6804") {
			t.Errorf("remediation should point at the tracking issue, got %q", got.Remediation)
		}
		// elasticsearch resolves fine; only the two postgresql refs are broken.
		if !strings.Contains(got.Detail, "2 of 3") {
			t.Errorf("detail should count the failures, got %q", got.Detail)
		}
	})

	t.Run("all images pullable is OK", func(t *testing.T) {
		orig := dockerManifestInspect
		dockerManifestInspect = func(_ context.Context, _ string) ([]byte, error) {
			return []byte(multiArchIndex), nil
		}
		defer func() { dockerManifestInspect = orig }()

		path := writeValuesFile(t, "values-enterprise.yaml", enterpriseValues)
		got := checkPinnedImages(context.Background(), []string{path}, "linux/amd64")
		if got.Status != StatusOK {
			t.Fatalf("status = %v, detail = %q", got.Status, got.Detail)
		}
		if !strings.Contains(got.Detail, "3 image(s)") {
			t.Errorf("detail should report the count, got %q", got.Detail)
		}
	})
}

func TestEnvFlagEnabled(t *testing.T) {
	const name = "DEPLOY_CAMUNDA_TEST_FLAG"
	for _, v := range []string{"1", "true", "TRUE", "yes", "on"} {
		t.Setenv(name, v)
		if !envFlagEnabled(name) {
			t.Errorf("%q should enable", v)
		}
	}
	for _, v := range []string{"", "0", "false", "off", "maybe"} {
		t.Setenv(name, v)
		if envFlagEnabled(name) {
			t.Errorf("%q should not enable", v)
		}
	}
}
