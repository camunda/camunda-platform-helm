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
	return AuditEnterpriseImages(context.Background(), entLayer(t, body), "linux/amd64")
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

	t.Run("single-platform images and platforms never built stay OK", func(t *testing.T) {
		shortenAuditRetryDelay(t)
		stubManifestInspect(t, func(string) ([]byte, error) { return []byte(`{"schemaVersion":2,"config":{}}`), nil })
		for _, r := range auditResults(t, enterpriseValues) {
			if !r.OK {
				t.Errorf("single-platform %s must not fail: %s", r.Ref, r.Detail)
			}
		}

		stubManifestInspect(t, func(string) ([]byte, error) { return []byte(multiArchIndex), nil })
		for _, r := range AuditEnterpriseImages(context.Background(), entLayer(t, enterpriseValues), "linux/s390x") {
			if !r.OK {
				t.Errorf("platform never built for %s must not fail: %s", r.Ref, r.Detail)
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
		got := AuditEnterpriseImages(ctx, entLayer(t, enterpriseValues), "linux/amd64")
		if len(got) != 3 {
			t.Fatalf("got %d results, want 3", len(got))
		}
	})
}
