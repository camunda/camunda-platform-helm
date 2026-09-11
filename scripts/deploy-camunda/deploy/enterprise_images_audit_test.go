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
	"strings"
	"sync"
	"testing"
	"time"
)

func stubManifestInspect(t *testing.T, fn func(ref string) ([]byte, error)) {
	t.Helper()
	orig := dockerManifestInspect
	var mu sync.Mutex
	dockerManifestInspect = func(_ context.Context, ref string) ([]byte, error) {
		mu.Lock()
		defer mu.Unlock()
		return fn(ref)
	}
	t.Cleanup(func() { dockerManifestInspect = orig })
}

func shortenAuditRetryDelay(t *testing.T) {
	t.Helper()
	orig := enterpriseImageAuditRetryDelay
	enterpriseImageAuditRetryDelay = time.Millisecond
	t.Cleanup(func() { enterpriseImageAuditRetryDelay = orig })
}

func auditResults(t *testing.T, body string) []EnterpriseImageResult {
	t.Helper()
	got, err := AuditEnterpriseImages(context.Background(), entLayer(t, body), "linux/amd64")
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	return got
}

func TestAuditEnterpriseImages(t *testing.T) {
	t.Run("every image pullable", func(t *testing.T) {
		shortenAuditRetryDelay(t)
		stubManifestInspect(t, func(string) ([]byte, error) { return []byte(multiArchIndex), nil })

		got := auditResults(t, enterpriseValues)
		if len(got) != 3 {
			t.Fatalf("got %d results, want 3: %v", len(got), got)
		}
		for _, r := range got {
			if !r.OK || r.Detail != "" {
				t.Errorf("%s should be OK, got %+v", r.Ref, r)
			}
		}
	})

	t.Run("index resolves but the amd64 child 404s", func(t *testing.T) {
		shortenAuditRetryDelay(t)
		stubManifestInspect(t, func(ref string) ([]byte, error) {
			if strings.Contains(ref, "elasticsearch@") {
				return nil, errors.New("manifest unknown")
			}
			return []byte(multiArchIndex), nil
		})

		var broken *EnterpriseImageResult
		for _, r := range auditResults(t, enterpriseValues) {
			if !r.OK {
				if broken != nil {
					t.Fatalf("only elasticsearch should fail, also got %s", r.Ref)
				}
				r := r
				broken = &r
			}
		}
		if broken == nil {
			t.Fatal("the missing child must be reported as a failure")
		}
		for _, want := range []string{
			"vendor-ee/elasticsearch:8.19.20",
			"linux/amd64",
			"sha256:amd64digest",
		} {
			if !strings.Contains(broken.Detail, want) {
				t.Errorf("detail %q must name %q", broken.Detail, want)
			}
		}
	})

	t.Run("an unresolvable index fails the audit", func(t *testing.T) {
		shortenAuditRetryDelay(t)
		stubManifestInspect(t, func(string) ([]byte, error) { return nil, errors.New("manifest unknown") })

		got := auditResults(t, enterpriseValues)
		if len(got) != 3 {
			t.Fatalf("got %d results, want 3", len(got))
		}
		for _, r := range got {
			if r.OK {
				t.Errorf("%s: checkPinnedImages warns for the deploy path, but the guardrail must fail", r.Ref)
			}
			if !strings.Contains(r.Detail, "could not be verified") {
				t.Errorf("detail should say the image was not verified, got %q", r.Detail)
			}
		}
	})

	t.Run("a single-platform manifest has no child to assert", func(t *testing.T) {
		shortenAuditRetryDelay(t)
		stubManifestInspect(t, func(string) ([]byte, error) { return []byte(`{"schemaVersion":2,"config":{}}`), nil })
		for _, r := range auditResults(t, enterpriseValues) {
			if !r.OK {
				t.Errorf("single-platform %s must not fail: %s", r.Ref, r.Detail)
			}
		}

	})

	t.Run("an index advertising no child for the target fails and names what it carries", func(t *testing.T) {
		shortenAuditRetryDelay(t)
		stubManifestInspect(t, func(string) ([]byte, error) { return []byte(multiArchIndex), nil })

		got, err := AuditEnterpriseImages(context.Background(), entLayer(t, enterpriseValues), "linux/s390x")
		if err != nil {
			t.Fatalf("audit: %v", err)
		}
		for _, r := range got {
			if r.OK {
				t.Fatalf("%s resolved only the index; the guardrail must not report that as verified", r.Ref)
			}
			for _, want := range []string{"advertises no linux/s390x child", "linux/amd64", "linux/arm64"} {
				if !strings.Contains(r.Detail, want) {
					t.Errorf("detail %q must contain %q", r.Detail, want)
				}
			}
		}
	})

	t.Run("arm64 variants are normalized in both directions", func(t *testing.T) {
		shortenAuditRetryDelay(t)
		// registry.camunda.cloud advertises arm64 without a variant; a hand-built
		// index may carry the explicit v8. containerd treats the two as the same
		// platform, so every combination below has to agree.
		indexes := map[string]string{
			"descriptor omits the variant": `{"manifests":[
			  {"digest":"sha256:arm64","platform":{"os":"linux","architecture":"arm64"}}
			]}`,
			"descriptor names v8": `{"manifests":[
			  {"digest":"sha256:arm64v8","platform":{"os":"linux","architecture":"arm64","variant":"v8"}}
			]}`,
		}
		for name, index := range indexes {
			for _, platform := range []string{"linux/arm64", "linux/arm64/v8", "linux/aarch64"} {
				t.Run(name+" / "+platform, func(t *testing.T) {
					stubManifestInspect(t, func(string) ([]byte, error) { return []byte(index), nil })
					got, err := AuditEnterpriseImages(context.Background(), entLayer(t, enterpriseValues), platform)
					if err != nil {
						t.Fatalf("audit: %v", err)
					}
					for _, r := range got {
						if !r.OK {
							t.Errorf("%s must select the arm64 child: %s", platform, r.Detail)
						}
					}
				})
			}

			t.Run(name+" / linux/arm64/v7 selects nothing", func(t *testing.T) {
				stubManifestInspect(t, func(string) ([]byte, error) { return []byte(index), nil })
				got, err := AuditEnterpriseImages(context.Background(), entLayer(t, enterpriseValues), "linux/arm64/v7")
				if err != nil {
					t.Fatalf("audit: %v", err)
				}
				for _, r := range got {
					if r.OK {
						t.Errorf("a genuinely different variant must not be reported as verified: %s", r.Ref)
					}
				}
			})
		}
	})

	t.Run("a bare os is rejected rather than resolved against the host", func(t *testing.T) {
		// platforms.Parse("linux") succeeds by filling in the running host's
		// architecture, which would mean different things on an amd64 runner and
		// an arm64 workstation.
		if err := ValidateImagePlatform("linux"); err == nil {
			t.Fatal("a bare os must not be accepted")
		}
	})

	t.Run("a values file that cannot be parsed fails instead of reporting zero images", func(t *testing.T) {
		got, err := AuditEnterpriseImages(context.Background(),
			entLayer(t, "identityPostgresql:\n  image:\n   registry: \"unterminated\n"), "linux/amd64")
		if err == nil {
			t.Fatalf("a malformed overlay must not pass as an empty image set, got %v", got)
		}
		if got != nil {
			t.Errorf("no image results should be reported alongside a load failure, got %v", got)
		}
	})

	t.Run("a values file that cannot be read fails", func(t *testing.T) {
		if _, err := AuditEnterpriseImages(context.Background(), "/nonexistent/values-enterprise.yaml", "linux/amd64"); err == nil {
			t.Fatal("an unreadable overlay must fail the chart")
		}
	})

	t.Run("a malformed platform is rejected before any registry call", func(t *testing.T) {
		stubManifestInspect(t, func(ref string) ([]byte, error) {
			t.Errorf("unexpected registry call for %s", ref)
			return nil, nil
		})
		for _, platform := range []string{"", "linux", "linux/", "/amd64", "linux/arm64/v8/extra"} {
			if _, err := AuditEnterpriseImages(context.Background(), entLayer(t, enterpriseValues), platform); err == nil {
				t.Errorf("platform %q must be rejected", platform)
			}
		}
	})

	t.Run("a values file with no pinned images makes no registry calls", func(t *testing.T) {
		stubManifestInspect(t, func(ref string) ([]byte, error) {
			t.Errorf("unexpected registry call for %s", ref)
			return nil, nil
		})
		if got := auditResults(t, "orchestration:\n  enabled: true\n"); got != nil {
			t.Fatalf("expected no results, got %v", got)
		}
	})

	t.Run("a transient failure clears on retry", func(t *testing.T) {
		shortenAuditRetryDelay(t)
		attempts := 0
		stubManifestInspect(t, func(ref string) ([]byte, error) {
			if strings.Contains(ref, "elasticsearch@") {
				attempts++
				if attempts == 1 {
					return nil, errors.New("manifest unknown")
				}
			}
			return []byte(multiArchIndex), nil
		})

		for _, r := range auditResults(t, enterpriseValues) {
			if !r.OK {
				t.Errorf("%s should have cleared on retry: %s", r.Ref, r.Detail)
			}
		}
		if attempts != 2 {
			t.Errorf("the child should be re-probed exactly once, got %d attempts", attempts)
		}
	})

	t.Run("retries re-probe only the failing images and preserve report order", func(t *testing.T) {
		shortenAuditRetryDelay(t)
		var calls []string
		stubManifestInspect(t, func(ref string) ([]byte, error) {
			calls = append(calls, ref)
			if strings.Contains(ref, "elasticsearch@") {
				return nil, errors.New("manifest unknown")
			}
			return []byte(multiArchIndex), nil
		})

		got := auditResults(t, enterpriseValues)
		want := []string{
			"registry.camunda.cloud/vendor-ee/elasticsearch:8.19.20",
			"registry.camunda.cloud/vendor-ee/postgresql:14.23.0-debian-12-r19",
			"registry.camunda.cloud/vendor-ee/postgresql:15.18.0-debian-12-r17",
		}
		for i, w := range want {
			if got[i].Ref != w {
				t.Errorf("result[%d] = %q, want %q", i, got[i].Ref, w)
			}
		}
		if got[0].OK {
			t.Error("elasticsearch must be reported failing after every attempt")
		}

		postgres := 0
		for _, c := range calls {
			if strings.Contains(c, "postgresql") {
				postgres++
			}
		}
		if postgres != 4 {
			t.Errorf("the two healthy images must be probed once each (index + child), got %d calls", postgres)
		}
		if len(calls) != 4+enterpriseImageAuditAttempts*2 {
			t.Errorf("elasticsearch must be re-probed on every attempt, got calls %v", calls)
		}
	})

	t.Run("a cancelled context stops the retry loop", func(t *testing.T) {
		orig := enterpriseImageAuditRetryDelay
		enterpriseImageAuditRetryDelay = time.Hour
		t.Cleanup(func() { enterpriseImageAuditRetryDelay = orig })
		stubManifestInspect(t, func(string) ([]byte, error) { return nil, errors.New("manifest unknown") })

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		got, err := AuditEnterpriseImages(ctx, entLayer(t, enterpriseValues), "linux/amd64")
		if err != nil {
			t.Fatalf("audit: %v", err)
		}
		if len(got) != 3 {
			t.Fatalf("got %d results, want 3", len(got))
		}
	})
}
