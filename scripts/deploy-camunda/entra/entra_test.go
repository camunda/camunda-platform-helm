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

package entra

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// newTestServer creates an httptest server that handles Graph API and login
// endpoints. The handlers map is keyed by "METHOD path" (e.g., "GET /applications").
// Any unhandled path returns 404.
func newTestServer(t *testing.T, handlers map[string]http.HandlerFunc) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Strip query string for handler lookup.
		path := r.URL.Path
		key := r.Method + " " + path
		if h, ok := handlers[key]; ok {
			h(w, r)
			return
		}
		// Try prefix matching for paths with dynamic segments (e.g., /applications/<id>).
		for k, h := range handlers {
			parts := strings.SplitN(k, " ", 2)
			if len(parts) == 2 && r.Method == parts[0] && strings.HasPrefix(path, parts[1]) {
				h(w, r)
				return
			}
		}
		t.Logf("unhandled request: %s %s", r.Method, r.URL.String())
		http.NotFound(w, r)
	}))
}

func jsonResponse(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// noRedirectVerifyWait removes the wait before the redirect URI read-back.
func noRedirectVerifyWait(t *testing.T) {
	t.Helper()
	origDelay, origJitter := redirectVerifyDelay, redirectVerifyJitter
	redirectVerifyDelay, redirectVerifyJitter = 0, 0
	t.Cleanup(func() { redirectVerifyDelay, redirectVerifyJitter = origDelay, origJitter })
}

// fakeRedirectApp is a Graph application whose GET returns what the last
// PATCH wrote. afterPatch, when set, runs after each PATCH is stored.
// getStatusAfterPatch, when non-zero, is the status every GET returns once a
// PATCH has been stored. staleReads GETs after each PATCH return the
// pre-PATCH state, like a lagging Graph replica.
type fakeRedirectApp struct {
	mu                  sync.Mutex
	app                 redirectURIApp
	prePatch            redirectURIApp
	patches             int
	afterPatch          func(f *fakeRedirectApp)
	getStatusAfterPatch int
	staleReads          int
	staleLeft           int
}

func (f *fakeRedirectApp) handlers() map[string]http.HandlerFunc {
	return map[string]http.HandlerFunc{
		"POST /test-tenant/oauth2/v2.0/token": tokenHandler(),
		"GET /applications/parent-obj-id": func(w http.ResponseWriter, r *http.Request) {
			f.mu.Lock()
			defer f.mu.Unlock()
			if f.patches > 0 && f.getStatusAfterPatch != 0 {
				jsonResponse(w, f.getStatusAfterPatch, map[string]any{
					"error": map[string]string{"code": "serviceNotAvailable"},
				})
				return
			}
			if f.staleLeft > 0 {
				f.staleLeft--
				jsonResponse(w, 200, f.prePatch)
				return
			}
			jsonResponse(w, 200, f.app)
		},
		"PATCH /applications/parent-obj-id": func(w http.ResponseWriter, r *http.Request) {
			var patch redirectURIApp
			_ = json.NewDecoder(r.Body).Decode(&patch)
			f.mu.Lock()
			defer f.mu.Unlock()
			f.prePatch = f.app
			f.staleLeft = f.staleReads
			f.app.Web, f.app.Spa = patch.Web, patch.Spa
			f.patches++
			if f.afterPatch != nil {
				f.afterPatch(f)
			}
			w.WriteHeader(204)
		},
	}
}

// redirectTestHost is the ingress host runUpdateRedirectURIs registers.
const redirectTestHost = "new-ns" + ciDomainSuffix

func runUpdateRedirectURIs(t *testing.T, f *fakeRedirectApp) error {
	t.Helper()
	noRedirectVerifyWait(t)
	srv := newTestServer(t, f.handlers())
	t.Cleanup(srv.Close)
	origGraph, origLogin := graphBaseURL, loginBaseURL
	graphBaseURL, loginBaseURL = srv.URL, srv.URL
	t.Cleanup(func() { graphBaseURL, loginBaseURL = origGraph, origLogin })
	return UpdateRedirectURIs(context.Background(), RedirectURIOptions{
		ObjectID:     "parent-obj-id",
		IngressHost:  redirectTestHost,
		DirectoryID:  "test-tenant",
		ClientID:     "parent-client",
		ClientSecret: "parent-secret",
		HTTPClient:   srv.Client(),
	})
}

// tokenHandler returns a handler that responds with a valid access token.
func tokenHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, 200, map[string]string{
			"access_token": "test-bearer-token",
		})
	}
}

func TestIsOIDCEntry(t *testing.T) {
	tests := []struct {
		auth, identity string
		want           bool
	}{
		{"oidc", "", true},
		{"", "oidc", true},
		{"oidc", "oidc", true},
		{"keycloak", "", false},
		{"", "keycloak", false},
		{"", "", false},
		// auth0 entries reuse auth=="oidc" as the legacy "external OIDC"
		// signal but must not trigger Entra provisioning.
		{"oidc", "auth0", false},
		{"", "auth0", false},
	}
	for _, tc := range tests {
		name := fmt.Sprintf("auth=%q identity=%q", tc.auth, tc.identity)
		t.Run(name, func(t *testing.T) {
			if got := IsOIDCEntry(tc.auth, tc.identity); got != tc.want {
				t.Errorf("IsOIDCEntry(%q, %q) = %v, want %v", tc.auth, tc.identity, got, tc.want)
			}
		})
	}
}

func TestResolveOpts_MissingRequired(t *testing.T) {
	tests := []struct {
		name string
		opts Options
		want string // substring expected in error
	}{
		{
			name: "missing directory ID",
			opts: Options{Namespace: "ns", ClientID: "cid", ClientSecret: "cs"},
			want: "ENTRA_APP_DIRECTORY_ID",
		},
		{
			name: "missing client ID",
			opts: Options{Namespace: "ns", DirectoryID: "did", ClientSecret: "cs"},
			want: "ENTRA_APP_CLIENT_ID",
		},
		{
			name: "missing client secret",
			opts: Options{Namespace: "ns", DirectoryID: "did", ClientID: "cid"},
			want: "ENTRA_APP_CLIENT_SECRET",
		},
		{
			name: "missing namespace",
			opts: Options{DirectoryID: "did", ClientID: "cid", ClientSecret: "cs"},
			want: "namespace is required",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Clear env vars to avoid accidental resolution.
			t.Setenv("ENTRA_APP_DIRECTORY_ID", "")
			t.Setenv("ENTRA_APP_CLIENT_ID", "")
			t.Setenv("ENTRA_APP_CLIENT_SECRET", "")

			err := resolveOpts(&tc.opts)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not contain %q", err, tc.want)
			}
		})
	}
}

func TestAppDisplayName(t *testing.T) {
	if got := appDisplayName("my-ns"); got != "venom-test-my-ns" {
		t.Errorf("appDisplayName(%q) = %q, want %q", "my-ns", got, "venom-test-my-ns")
	}
}

func TestAcquireBearerToken(t *testing.T) {
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"POST /test-tenant/oauth2/v2.0/token": tokenHandler(),
	})
	defer srv.Close()

	origLogin := loginBaseURL
	loginBaseURL = srv.URL
	defer func() { loginBaseURL = origLogin }()

	opts := &Options{
		DirectoryID:  "test-tenant",
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		Namespace:    "ns",
		HTTPClient:   srv.Client(),
	}

	token, err := acquireBearerToken(context.Background(), opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "test-bearer-token" {
		t.Errorf("got token %q, want %q", token, "test-bearer-token")
	}
}

func TestAcquireBearerToken_Error(t *testing.T) {
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"POST /test-tenant/oauth2/v2.0/token": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 400, map[string]string{
				"error":             "invalid_client",
				"error_description": "bad credentials",
			})
		},
	})
	defer srv.Close()

	origLogin := loginBaseURL
	loginBaseURL = srv.URL
	defer func() { loginBaseURL = origLogin }()

	opts := &Options{
		DirectoryID:  "test-tenant",
		ClientID:     "test-client",
		ClientSecret: "bad-secret",
		Namespace:    "ns",
		HTTPClient:   srv.Client(),
	}

	_, err := acquireBearerToken(context.Background(), opts)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid_client") {
		t.Errorf("error %q does not mention invalid_client", err)
	}
}

func TestFindApp_Found(t *testing.T) {
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"GET /applications": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]any{
				"value": []map[string]string{
					{"appId": "found-app-id", "id": "found-object-id"},
				},
			})
		},
	})
	defer srv.Close()

	origGraph := graphBaseURL
	graphBaseURL = srv.URL
	defer func() { graphBaseURL = origGraph }()

	appID, objectID, err := findApp(context.Background(), srv.Client(), "token", "venom-test-ns")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if appID != "found-app-id" {
		t.Errorf("appID = %q, want %q", appID, "found-app-id")
	}
	if objectID != "found-object-id" {
		t.Errorf("objectID = %q, want %q", objectID, "found-object-id")
	}
}

func TestFindApp_NotFound(t *testing.T) {
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"GET /applications": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]any{
				"value": []any{},
			})
		},
	})
	defer srv.Close()

	origGraph := graphBaseURL
	graphBaseURL = srv.URL
	defer func() { graphBaseURL = origGraph }()

	appID, objectID, err := findApp(context.Background(), srv.Client(), "token", "venom-test-ns")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if appID != "" || objectID != "" {
		t.Errorf("expected empty strings, got appID=%q objectID=%q", appID, objectID)
	}
}

func TestCreateApp(t *testing.T) {
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"POST /applications": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 201, map[string]string{
				"appId": "new-app-id",
				"id":    "new-object-id",
			})
		},
	})
	defer srv.Close()

	origGraph := graphBaseURL
	graphBaseURL = srv.URL
	defer func() { graphBaseURL = origGraph }()

	appID, objectID, err := createApp(context.Background(), srv.Client(), "token", "venom-test-ns", "parent-client-id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if appID != "new-app-id" {
		t.Errorf("appID = %q, want %q", appID, "new-app-id")
	}
	if objectID != "new-object-id" {
		t.Errorf("objectID = %q, want %q", objectID, "new-object-id")
	}
}

func TestCreateApp_Error(t *testing.T) {
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"POST /applications": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 400, map[string]any{
				"error": map[string]string{
					"code":    "BadRequest",
					"message": "invalid payload",
				},
			})
		},
	})
	defer srv.Close()

	origGraph := graphBaseURL
	graphBaseURL = srv.URL
	defer func() { graphBaseURL = origGraph }()

	_, _, err := createApp(context.Background(), srv.Client(), "token", "venom-test-ns", "parent-client-id")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestRotateCredentials(t *testing.T) {
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"GET /applications/obj-123": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]any{
				"passwordCredentials": []map[string]string{
					{"keyId": "old-key-1"},
				},
			})
		},
		"POST /applications/obj-123/removePassword": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(204)
		},
		"POST /applications/obj-123/addPassword": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]string{
				"secretText": "new-secret-value",
			})
		},
	})
	defer srv.Close()

	origGraph := graphBaseURL
	graphBaseURL = srv.URL
	defer func() { graphBaseURL = origGraph }()

	secret, err := rotateCredentials(context.Background(), srv.Client(), "token", "obj-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if secret != "new-secret-value" {
		t.Errorf("secret = %q, want %q", secret, "new-secret-value")
	}
}

// shrinkRetryTimings scales the graph retry knobs down to millisecond scale
// so retry tests complete in milliseconds, not minutes.
func shrinkRetryTimings(t *testing.T) {
	t.Helper()
	origBase, origMax, origAttempts := retryBaseBackoff, retryMaxBackoff, retryMaxAttempts
	retryBaseBackoff = time.Millisecond
	retryMaxBackoff = 4 * time.Millisecond
	retryMaxAttempts = 8
	t.Cleanup(func() {
		retryBaseBackoff = origBase
		retryMaxBackoff = origMax
		retryMaxAttempts = origAttempts
	})
}

// TestRotateCredentials_RetriesOn404ThenSucceeds verifies that addPassword
// is retried when Graph returns 404 for a newly-created object id (eventual
// consistency), and that the rotated secret is returned once Graph recovers.
func TestRotateCredentials_RetriesOn404ThenSucceeds(t *testing.T) {
	shrinkRetryTimings(t)

	var addAttempts atomic.Int32
	const flakyAttempts int32 = 3

	srv := newTestServer(t, map[string]http.HandlerFunc{
		"GET /applications/obj-flaky": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]any{
				"passwordCredentials": []map[string]string{},
			})
		},
		"POST /applications/obj-flaky/addPassword": func(w http.ResponseWriter, r *http.Request) {
			n := addAttempts.Add(1)
			if n <= flakyAttempts {
				jsonResponse(w, 404, map[string]any{
					"error": map[string]string{
						"code":    "Request_ResourceNotFound",
						"message": "Resource 'obj-flaky' does not exist",
					},
				})
				return
			}
			jsonResponse(w, 201, map[string]string{
				"secretText": "fresh-secret",
			})
		},
	})
	defer srv.Close()

	origGraph := graphBaseURL
	graphBaseURL = srv.URL
	defer func() { graphBaseURL = origGraph }()

	secret, err := rotateCredentials(context.Background(), srv.Client(), "token", "obj-flaky")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if secret != "fresh-secret" {
		t.Errorf("secret = %q, want %q", secret, "fresh-secret")
	}
	if got := addAttempts.Load(); got != flakyAttempts+1 {
		t.Errorf("addPassword attempts = %d, want %d (retry loop did not retry through 404s)", got, flakyAttempts+1)
	}
}

// TestRotateCredentials_RetriesCredentialGetOn404 verifies that the initial
// credential read is retried while a newly-created app returns 404.
func TestRotateCredentials_RetriesCredentialGetOn404(t *testing.T) {
	shrinkRetryTimings(t)

	var getAttempts atomic.Int32
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"GET /applications/obj-new": func(w http.ResponseWriter, r *http.Request) {
			if getAttempts.Add(1) == 1 {
				jsonResponse(w, 404, map[string]any{
					"error": map[string]string{"code": "Request_ResourceNotFound"},
				})
				return
			}
			jsonResponse(w, 200, map[string]any{
				"passwordCredentials": []map[string]string{{"keyId": "old-key"}},
			})
		},
		"POST /applications/obj-new/removePassword": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(204)
		},
		"POST /applications/obj-new/addPassword": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 201, map[string]string{"secretText": "fresh-secret"})
		},
	})
	defer srv.Close()

	origGraph := graphBaseURL
	graphBaseURL = srv.URL
	defer func() { graphBaseURL = origGraph }()

	secret, err := rotateCredentials(context.Background(), srv.Client(), "token", "obj-new")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if secret != "fresh-secret" {
		t.Errorf("secret = %q, want %q", secret, "fresh-secret")
	}
	if got := getAttempts.Load(); got != 2 {
		t.Errorf("credential GET attempts = %d, want 2", got)
	}
}

// TestRotateCredentials_GivesUpAfterMaxAttempts verifies the retry loop
// terminates with an error that references the last status and body when
// Graph persistently returns 404.
func TestRotateCredentials_GivesUpAfterMaxAttempts(t *testing.T) {
	shrinkRetryTimings(t)

	var addAttempts atomic.Int32
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"GET /applications/obj-stuck": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]any{
				"passwordCredentials": []map[string]string{},
			})
		},
		"POST /applications/obj-stuck/addPassword": func(w http.ResponseWriter, r *http.Request) {
			addAttempts.Add(1)
			jsonResponse(w, 404, map[string]any{
				"error": map[string]string{
					"code":    "Request_ResourceNotFound",
					"message": "Resource 'obj-stuck' does not exist",
				},
			})
		},
	})
	defer srv.Close()

	origGraph := graphBaseURL
	graphBaseURL = srv.URL
	defer func() { graphBaseURL = origGraph }()

	_, err := rotateCredentials(context.Background(), srv.Client(), "token", "obj-stuck")
	if err == nil {
		t.Fatal("expected error when addPassword never succeeds, got nil")
	}
	if !strings.Contains(err.Error(), "gave up") {
		t.Errorf("error should mention give-up, got: %v", err)
	}
	if got := addAttempts.Load(); got != int32(retryMaxAttempts) {
		t.Errorf("addPassword attempts = %d, want %d (retry loop did not use full budget)", got, retryMaxAttempts)
	}
}

// TestRotateCredentials_RetriesOn409ThenSucceeds verifies that addPassword
// is retried when Graph returns 409 Directory_ConcurrencyViolation (a
// transient error caused by concurrent tenant mutations), and that the
// rotated secret is returned once the conflict clears.
func TestRotateCredentials_RetriesOn409ThenSucceeds(t *testing.T) {
	shrinkRetryTimings(t)

	var addAttempts atomic.Int32
	const flakyAttempts int32 = 2

	srv := newTestServer(t, map[string]http.HandlerFunc{
		"GET /applications/obj-conflict": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]any{
				"passwordCredentials": []map[string]string{},
			})
		},
		"POST /applications/obj-conflict/addPassword": func(w http.ResponseWriter, r *http.Request) {
			n := addAttempts.Add(1)
			if n <= flakyAttempts {
				jsonResponse(w, 409, map[string]any{
					"error": map[string]string{
						"code":    "Directory_ConcurrencyViolation",
						"message": "Error due to concurrent requests being made to the tenant.",
					},
				})
				return
			}
			jsonResponse(w, 201, map[string]string{
				"secretText": "conflict-resolved-secret",
			})
		},
	})
	defer srv.Close()

	origGraph := graphBaseURL
	graphBaseURL = srv.URL
	defer func() { graphBaseURL = origGraph }()

	secret, err := rotateCredentials(context.Background(), srv.Client(), "token", "obj-conflict")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if secret != "conflict-resolved-secret" {
		t.Errorf("secret = %q, want %q", secret, "conflict-resolved-secret")
	}
	if got := addAttempts.Load(); got != flakyAttempts+1 {
		t.Errorf("addPassword attempts = %d, want %d (retry loop did not retry through 409s)", got, flakyAttempts+1)
	}
}

// TestRotateCredentials_DoesNotRetry401 verifies terminal auth errors fail
// fast rather than burning the retry budget.
func TestRotateCredentials_DoesNotRetry401(t *testing.T) {
	shrinkRetryTimings(t)

	var addAttempts atomic.Int32
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"GET /applications/obj-unauthorized": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]any{
				"passwordCredentials": []map[string]string{},
			})
		},
		"POST /applications/obj-unauthorized/addPassword": func(w http.ResponseWriter, r *http.Request) {
			addAttempts.Add(1)
			jsonResponse(w, 401, map[string]any{
				"error": map[string]string{
					"code":    "InvalidAuthenticationToken",
					"message": "Access token has expired",
				},
			})
		},
	})
	defer srv.Close()

	origGraph := graphBaseURL
	graphBaseURL = srv.URL
	defer func() { graphBaseURL = origGraph }()

	_, err := rotateCredentials(context.Background(), srv.Client(), "token", "obj-unauthorized")
	if err == nil {
		t.Fatal("expected error on 401, got nil")
	}
	if !strings.Contains(err.Error(), "non-retryable") {
		t.Errorf("error should be non-retryable, got: %v", err)
	}
	if got := addAttempts.Load(); got != 1 {
		t.Errorf("addPassword attempts = %d, want 1 (should not retry on 401)", got)
	}
}

func TestEnsureServicePrincipal_AlreadyExists(t *testing.T) {
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"PATCH /servicePrincipals(appId='app-id')": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		},
	})
	defer srv.Close()

	origGraph := graphBaseURL
	graphBaseURL = srv.URL
	defer func() { graphBaseURL = origGraph }()

	err := ensureServicePrincipal(context.Background(), srv.Client(), "token", "app-id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEnsureServicePrincipal_Creates(t *testing.T) {
	created := false
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"PATCH /servicePrincipals(appId='app-id')": func(w http.ResponseWriter, r *http.Request) {
			created = true
			jsonResponse(w, 201, map[string]string{
				"id": "new-sp-id",
			})
		},
	})
	defer srv.Close()

	origGraph := graphBaseURL
	graphBaseURL = srv.URL
	defer func() { graphBaseURL = origGraph }()

	err := ensureServicePrincipal(context.Background(), srv.Client(), "token", "app-id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !created {
		t.Error("expected service principal to be created")
	}
}

func TestEnsureServicePrincipal_RepeatedUpsert(t *testing.T) {
	var requests atomic.Int32
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"PATCH /servicePrincipals(appId='app-id')": func(w http.ResponseWriter, r *http.Request) {
			if got := r.Header.Get("Prefer"); got != "create-if-missing" {
				t.Errorf("Prefer = %q, want create-if-missing", got)
			}
			if got := r.Header.Get("Authorization"); got != "Bearer token" {
				t.Errorf("Authorization = %q, want Bearer token", got)
			}
			if got := r.Header.Get("Content-Type"); got != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", got)
			}
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Errorf("decode upsert payload: %v", err)
			}
			if payload == nil || len(payload) != 0 {
				t.Errorf("upsert payload = %#v, want empty object", payload)
			}
			if requests.Add(1) == 1 {
				jsonResponse(w, http.StatusCreated, map[string]string{"id": "sp-id", "appId": "app-id"})
			} else {
				w.WriteHeader(http.StatusNoContent)
			}
		},
	})
	defer srv.Close()
	origGraph := graphBaseURL
	graphBaseURL = srv.URL
	defer func() { graphBaseURL = origGraph }()

	for range 3 {
		if err := ensureServicePrincipal(context.Background(), srv.Client(), "token", "app-id"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if got := requests.Load(); got != 3 {
		t.Errorf("upsert requests = %d, want 3", got)
	}
}

func TestEnsureServicePrincipal_RetriesOn400RequestBadRequest(t *testing.T) {
	shrinkRetryTimings(t)

	var patchAttempts atomic.Int32
	const flakyAttempts int32 = 2

	srv := newTestServer(t, map[string]http.HandlerFunc{
		"PATCH /servicePrincipals(appId='abc-123')": func(w http.ResponseWriter, r *http.Request) {
			attempt := patchAttempts.Add(1)
			if attempt <= flakyAttempts {
				jsonResponse(w, 400, map[string]any{
					"error": map[string]string{
						"code":    "Request_BadRequest",
						"message": "The appId 'abc-123' of the service principal does not reference a valid application object.",
					},
				})
				return
			}
			jsonResponse(w, 201, map[string]string{"id": "sp-recovered"})
		},
	})
	defer srv.Close()

	origGraph := graphBaseURL
	graphBaseURL = srv.URL
	defer func() { graphBaseURL = origGraph }()

	if err := ensureServicePrincipal(context.Background(), srv.Client(), "token", "abc-123"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := patchAttempts.Load(); got != flakyAttempts+1 {
		t.Errorf("PATCH attempts = %d, want %d", got, flakyAttempts+1)
	}
}

// TestEnsureServicePrincipal_DoesNotRetryGeneric400 verifies the SP retry
// predicate is narrow — a 400 that is NOT Request_BadRequest with the appId
// message must fail fast.
func TestEnsureServicePrincipal_DoesNotRetryGeneric400(t *testing.T) {
	shrinkRetryTimings(t)

	var patchAttempts atomic.Int32
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"PATCH /servicePrincipals(appId='app-id')": func(w http.ResponseWriter, r *http.Request) {
			patchAttempts.Add(1)
			jsonResponse(w, 400, map[string]any{
				"error": map[string]string{
					"code":    "Request_BadRequest",
					"message": "Some other validation failure unrelated to propagation.",
				},
			})
		},
	})
	defer srv.Close()

	origGraph := graphBaseURL
	graphBaseURL = srv.URL
	defer func() { graphBaseURL = origGraph }()

	err := ensureServicePrincipal(context.Background(), srv.Client(), "token", "app-id")
	if err == nil {
		t.Fatal("expected error on generic 400, got nil")
	}
	if got := patchAttempts.Load(); got != 1 {
		t.Errorf("PATCH attempts = %d, want 1 (generic 400 should not be retried)", got)
	}
}

func TestEnsureServicePrincipal_Errors(t *testing.T) {
	shrinkRetryTimings(t)
	for _, testCase := range []struct {
		name         string
		statusCode   int
		wantAttempts int32
	}{
		{"Unauthorized", http.StatusUnauthorized, 1},
		{"Forbidden", http.StatusForbidden, 1},
		{"UnexpectedSuccess", http.StatusOK, 1},
		{"RetryBudget", http.StatusServiceUnavailable, int32(retryMaxAttempts)},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			var attempts atomic.Int32
			srv := newTestServer(t, map[string]http.HandlerFunc{
				"PATCH /servicePrincipals(appId='app-id')": func(w http.ResponseWriter, r *http.Request) {
					attempts.Add(1)
					jsonResponse(w, testCase.statusCode, map[string]any{
						"error": map[string]string{"code": "Request_BadRequest", "message": "unrelated failure"},
					})
				},
			})
			defer srv.Close()
			origGraph := graphBaseURL
			graphBaseURL = srv.URL
			defer func() { graphBaseURL = origGraph }()

			err := ensureServicePrincipal(context.Background(), srv.Client(), "token", "app-id")
			if err == nil || !strings.Contains(err.Error(), fmt.Sprint(testCase.statusCode)) {
				t.Fatalf("error = %v, want status %d", err, testCase.statusCode)
			}
			if got := attempts.Load(); got != testCase.wantAttempts {
				t.Errorf("PATCH attempts = %d, want %d", got, testCase.wantAttempts)
			}
		})
	}
}

func TestCleanupVenomApp_AppExists(t *testing.T) {
	deleted := false
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"POST /test-tenant/oauth2/v2.0/token": tokenHandler(),
		"GET /applications": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]any{
				"value": []map[string]string{
					{"appId": "app-id", "id": "obj-to-delete"},
				},
			})
		},
		"DELETE /applications/obj-to-delete": func(w http.ResponseWriter, r *http.Request) {
			deleted = true
			w.WriteHeader(204)
		},
	})
	defer srv.Close()

	origGraph := graphBaseURL
	origLogin := loginBaseURL
	graphBaseURL = srv.URL
	loginBaseURL = srv.URL
	defer func() {
		graphBaseURL = origGraph
		loginBaseURL = origLogin
	}()

	CleanupVenomApp(context.Background(), Options{
		Namespace:    "test-ns",
		DirectoryID:  "test-tenant",
		ClientID:     "parent-client",
		ClientSecret: "parent-secret",
		HTTPClient:   srv.Client(),
	})

	if !deleted {
		t.Error("expected app to be deleted")
	}
}

func TestCleanupVenomApp_AppNotFound(t *testing.T) {
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"POST /test-tenant/oauth2/v2.0/token": tokenHandler(),
		"GET /applications": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]any{
				"value": []any{},
			})
		},
	})
	defer srv.Close()

	origGraph := graphBaseURL
	origLogin := loginBaseURL
	graphBaseURL = srv.URL
	loginBaseURL = srv.URL
	defer func() {
		graphBaseURL = origGraph
		loginBaseURL = origLogin
	}()

	// Should not panic or error — just logs "nothing to clean up".
	CleanupVenomApp(context.Background(), Options{
		Namespace:    "test-ns",
		DirectoryID:  "test-tenant",
		ClientID:     "parent-client",
		ClientSecret: "parent-secret",
		HTTPClient:   srv.Client(),
	})
}

func TestCleanupVenomApp_InvalidOpts(t *testing.T) {
	// Clear env vars to ensure validation fails.
	t.Setenv("ENTRA_APP_DIRECTORY_ID", "")
	t.Setenv("ENTRA_APP_CLIENT_ID", "")
	t.Setenv("ENTRA_APP_CLIENT_SECRET", "")

	// Should not panic — just logs and returns.
	CleanupVenomApp(context.Background(), Options{})
}

// TestEnsureVenomApp_NewApp tests the full happy-path flow when no existing app is found.
// This tests everything except the K8s secret creation (which requires a real cluster).
func TestEnsureVenomApp_NewApp(t *testing.T) {
	spCreated := false
	srv := newTestServer(t, map[string]http.HandlerFunc{
		// Token endpoint.
		"POST /test-tenant/oauth2/v2.0/token": tokenHandler(),
		// findApp: no existing app.
		"GET /applications": func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.URL.RawQuery, "filter") {
				jsonResponse(w, 200, map[string]any{
					"value": []any{},
				})
				return
			}
			// getApp for rotateCredentials (fetching password credentials).
			jsonResponse(w, 200, map[string]any{
				"passwordCredentials": []any{},
			})
		},
		// createApp.
		"POST /applications": func(w http.ResponseWriter, r *http.Request) {
			// Check if this is a removePassword/addPassword call.
			if strings.Contains(r.URL.Path, "/addPassword") || strings.Contains(r.URL.Path, "/removePassword") {
				return // handled by prefix match below
			}
			jsonResponse(w, 201, map[string]string{
				"appId": "new-venom-app-id",
				"id":    "new-venom-obj-id",
			})
		},
		// rotateCredentials: get existing app details.
		"GET /applications/new-venom-obj-id": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]any{
				"passwordCredentials": []any{},
			})
		},
		// rotateCredentials: addPassword.
		"POST /applications/new-venom-obj-id/addPassword": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]string{
				"secretText": "generated-secret",
			})
		},
		"PATCH /servicePrincipals(appId='new-venom-app-id')": func(w http.ResponseWriter, r *http.Request) {
			spCreated = true
			jsonResponse(w, 201, map[string]string{"id": "sp-id"})
		},
	})
	defer srv.Close()

	origGraph := graphBaseURL
	origLogin := loginBaseURL
	origPropSleep := propagationSleepDuration
	graphBaseURL = srv.URL
	loginBaseURL = srv.URL
	propagationSleepDuration = 0
	defer func() {
		graphBaseURL = origGraph
		loginBaseURL = origLogin
		propagationSleepDuration = origPropSleep
	}()

	// Override createVenomK8sSecret to avoid requiring a real K8s cluster.
	origCreateSecret := createVenomK8sSecretFunc
	createVenomK8sSecretFunc = func(ctx context.Context, kubeContext, namespace, venomClientID, venomClientSecret, audience string) error {
		if namespace != "test-ns" {
			t.Errorf("secret namespace = %q, want %q", namespace, "test-ns")
		}
		if venomClientID != "new-venom-app-id" {
			t.Errorf("venomClientID = %q, want %q", venomClientID, "new-venom-app-id")
		}
		if venomClientSecret != "generated-secret" {
			t.Errorf("venomClientSecret = %q, want %q", venomClientSecret, "generated-secret")
		}
		if audience != "parent-client-id" {
			t.Errorf("audience = %q, want %q", audience, "parent-client-id")
		}
		return nil
	}
	defer func() { createVenomK8sSecretFunc = origCreateSecret }()

	app, err := EnsureVenomApp(context.Background(), Options{
		Namespace:    "test-ns",
		DirectoryID:  "test-tenant",
		ClientID:     "parent-client-id",
		ClientSecret: "parent-secret",
		HTTPClient:   srv.Client(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if app.AppID != "new-venom-app-id" {
		t.Errorf("AppID = %q, want %q", app.AppID, "new-venom-app-id")
	}
	if app.ObjectID != "new-venom-obj-id" {
		t.Errorf("ObjectID = %q, want %q", app.ObjectID, "new-venom-obj-id")
	}
	if app.ClientSecret != "generated-secret" {
		t.Errorf("ClientSecret = %q, want %q", app.ClientSecret, "generated-secret")
	}
	if !spCreated {
		t.Error("expected service principal to be created")
	}
}

// TestEnsureVenomApp_ExistingApp tests the flow when an existing app is found.
func TestEnsureVenomApp_ExistingApp(t *testing.T) {
	appCreateCalled := false
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"POST /test-tenant/oauth2/v2.0/token": tokenHandler(),
		// findApp: existing app found.
		"GET /applications": func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.URL.RawQuery, "filter") {
				jsonResponse(w, 200, map[string]any{
					"value": []map[string]string{
						{"appId": "existing-app-id", "id": "existing-obj-id"},
					},
				})
				return
			}
			http.NotFound(w, r)
		},
		// createApp: should NOT be called.
		"POST /applications": func(w http.ResponseWriter, r *http.Request) {
			if !strings.Contains(r.URL.Path, "/") || r.URL.Path == "/applications" {
				appCreateCalled = true
			}
			// Still need to handle addPassword.
			if strings.Contains(r.URL.Path, "addPassword") {
				jsonResponse(w, 200, map[string]string{"secretText": "rotated-secret"})
				return
			}
			http.NotFound(w, r)
		},
		// rotateCredentials.
		"GET /applications/existing-obj-id": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]any{
				"passwordCredentials": []map[string]string{
					{"keyId": "old-key"},
				},
			})
		},
		"POST /applications/existing-obj-id/removePassword": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(204)
		},
		"POST /applications/existing-obj-id/addPassword": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]string{"secretText": "rotated-secret"})
		},
		// ensureServicePrincipal: already exists.
		"PATCH /servicePrincipals(appId='existing-app-id')": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		},
	})
	defer srv.Close()

	origGraph := graphBaseURL
	origLogin := loginBaseURL
	graphBaseURL = srv.URL
	loginBaseURL = srv.URL
	defer func() {
		graphBaseURL = origGraph
		loginBaseURL = origLogin
	}()

	origCreateSecret := createVenomK8sSecretFunc
	createVenomK8sSecretFunc = func(ctx context.Context, kubeContext, namespace, venomClientID, venomClientSecret, audience string) error {
		return nil
	}
	defer func() { createVenomK8sSecretFunc = origCreateSecret }()

	app, err := EnsureVenomApp(context.Background(), Options{
		Namespace:    "test-ns",
		DirectoryID:  "test-tenant",
		ClientID:     "parent-client-id",
		ClientSecret: "parent-secret",
		HTTPClient:   srv.Client(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if app.AppID != "existing-app-id" {
		t.Errorf("AppID = %q, want %q", app.AppID, "existing-app-id")
	}
	if app.ClientSecret != "rotated-secret" {
		t.Errorf("ClientSecret = %q, want %q", app.ClientSecret, "rotated-secret")
	}
	if appCreateCalled {
		t.Error("createApp should not have been called for existing app")
	}
}

// TestEnsureVenomApp_SkipK8sSecret verifies that when SkipK8sSecret is true,
// the K8s secret creation is skipped but the Entra provisioning completes.
func TestEnsureVenomApp_SkipK8sSecret(t *testing.T) {
	secretCreated := false
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"POST /test-tenant/oauth2/v2.0/token": tokenHandler(),
		"GET /applications": func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.URL.RawQuery, "filter") {
				jsonResponse(w, 200, map[string]any{
					"value": []map[string]string{
						{"appId": "skip-secret-app-id", "id": "skip-secret-obj-id"},
					},
				})
				return
			}
			http.NotFound(w, r)
		},
		"GET /applications/skip-secret-obj-id": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]any{
				"passwordCredentials": []any{},
			})
		},
		"POST /applications/skip-secret-obj-id/addPassword": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]string{"secretText": "skip-secret-value"})
		},
		"PATCH /servicePrincipals(appId='skip-secret-app-id')": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		},
	})
	defer srv.Close()

	origGraph := graphBaseURL
	origLogin := loginBaseURL
	graphBaseURL = srv.URL
	loginBaseURL = srv.URL
	defer func() {
		graphBaseURL = origGraph
		loginBaseURL = origLogin
	}()

	origCreateSecret := createVenomK8sSecretFunc
	createVenomK8sSecretFunc = func(ctx context.Context, kubeContext, namespace, venomClientID, venomClientSecret, audience string) error {
		secretCreated = true
		return nil
	}
	defer func() { createVenomK8sSecretFunc = origCreateSecret }()

	app, err := EnsureVenomApp(context.Background(), Options{
		Namespace:     "test-ns",
		DirectoryID:   "test-tenant",
		ClientID:      "parent-client-id",
		ClientSecret:  "parent-secret",
		HTTPClient:    srv.Client(),
		SkipK8sSecret: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if secretCreated {
		t.Error("K8s secret should NOT have been created when SkipK8sSecret=true")
	}
	if app.AppID != "skip-secret-app-id" {
		t.Errorf("AppID = %q, want %q", app.AppID, "skip-secret-app-id")
	}
	if app.ClientSecret != "skip-secret-value" {
		t.Errorf("ClientSecret = %q, want %q", app.ClientSecret, "skip-secret-value")
	}
}

// TestCreateVenomK8sSecret_Deferred verifies that CreateVenomK8sSecret calls
// the underlying secret creation function with the correct parameters.
func TestCreateVenomK8sSecret_Deferred(t *testing.T) {
	var gotNS, gotClientID, gotClientSecret, gotAudience string
	origCreateSecret := createVenomK8sSecretFunc
	createVenomK8sSecretFunc = func(ctx context.Context, kubeContext, namespace, venomClientID, venomClientSecret, audience string) error {
		gotNS = namespace
		gotClientID = venomClientID
		gotClientSecret = venomClientSecret
		gotAudience = audience
		return nil
	}
	defer func() { createVenomK8sSecretFunc = origCreateSecret }()

	app := &VenomApp{
		AppID:        "deferred-app-id",
		ObjectID:     "deferred-obj-id",
		ClientSecret: "deferred-secret",
	}

	err := CreateVenomK8sSecret(context.Background(), "", "target-ns", app, "audience-client-id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotNS != "target-ns" {
		t.Errorf("namespace = %q, want %q", gotNS, "target-ns")
	}
	if gotClientID != "deferred-app-id" {
		t.Errorf("venomClientID = %q, want %q", gotClientID, "deferred-app-id")
	}
	if gotClientSecret != "deferred-secret" {
		t.Errorf("venomClientSecret = %q, want %q", gotClientSecret, "deferred-secret")
	}
	if gotAudience != "audience-client-id" {
		t.Errorf("audience = %q, want %q", gotAudience, "audience-client-id")
	}
}

// TestCreateVenomK8sSecret_Error verifies that errors from the underlying function propagate.
func TestCreateVenomK8sSecret_Error(t *testing.T) {
	origCreateSecret := createVenomK8sSecretFunc
	createVenomK8sSecretFunc = func(ctx context.Context, kubeContext, namespace, venomClientID, venomClientSecret, audience string) error {
		return fmt.Errorf("simulated failure")
	}
	defer func() { createVenomK8sSecretFunc = origCreateSecret }()

	app := &VenomApp{AppID: "id", ClientSecret: "secret"}
	err := CreateVenomK8sSecret(context.Background(), "", "ns", app, "aud")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "simulated failure") {
		t.Errorf("error %q does not contain 'simulated failure'", err)
	}
}

// --- Tests for redirect URI functionality ---

func TestIsValidURI(t *testing.T) {
	tests := []struct {
		uri  string
		want bool
	}{
		{"https://example.com/callback", true},
		{"https://ns.ci.distro.ultrawombat.com/identity/auth/login-callback", true},
		{"", false},
		{"https://example.com/callback,", false},  // trailing comma = corruption
		{"https://example.com/callback,,", false}, // double trailing comma
	}
	for _, tc := range tests {
		t.Run(tc.uri, func(t *testing.T) {
			if got := isValidURI(tc.uri); got != tc.want {
				t.Errorf("isValidURI(%q) = %v, want %v", tc.uri, got, tc.want)
			}
		})
	}
}

func TestBuildRedirectURIs(t *testing.T) {
	web, spa := buildRedirectURIs("my-ns.ci.distro.ultrawombat.com")

	expectedWeb := []string{
		"https://my-ns.ci.distro.ultrawombat.com/identity/auth/login-callback",
		"https://my-ns.ci.distro.ultrawombat.com/operate/identity-callback",
		"https://my-ns.ci.distro.ultrawombat.com/optimize/api/authentication/callback",
		"https://my-ns.ci.distro.ultrawombat.com/tasklist/identity-callback",
		"https://my-ns.ci.distro.ultrawombat.com/orchestration/sso-callback",
	}
	expectedSpa := []string{
		"https://my-ns.ci.distro.ultrawombat.com/modeler/login-callback",
		"https://my-ns.ci.distro.ultrawombat.com/",
	}

	if len(web) != len(expectedWeb) {
		t.Fatalf("web URI count = %d, want %d", len(web), len(expectedWeb))
	}
	for i, uri := range web {
		if uri != expectedWeb[i] {
			t.Errorf("web[%d] = %q, want %q", i, uri, expectedWeb[i])
		}
	}

	if len(spa) != len(expectedSpa) {
		t.Fatalf("spa URI count = %d, want %d", len(spa), len(expectedSpa))
	}
	for i, uri := range spa {
		if uri != expectedSpa[i] {
			t.Errorf("spa[%d] = %q, want %q", i, uri, expectedSpa[i])
		}
	}
}

func TestFilterRedirectURIs(t *testing.T) {
	existing := []string{
		// Non-CI URI — should be preserved.
		"https://production.example.com/callback",
		// Another live CI deployment — should be preserved.
		"https://other-ns.ci.distro.ultrawombat.com/identity/auth/login-callback",
		// Current host — should be replaced by newURIs.
		"https://new-ns.ci.distro.ultrawombat.com/stale-callback",
		// Malformed URI with trailing comma — should be removed.
		"https://broken.ci.distro.ultrawombat.com/callback,",
		// Empty string — should be removed.
		"",
		// Another non-CI URI.
		"https://staging.distribution.aws.camunda.cloud/callback",
		// Duplicate non-CI URI — should be deduplicated.
		"https://production.example.com/callback",
	}

	newURIs := []string{
		"https://new-ns.ci.distro.ultrawombat.com/identity/auth/login-callback",
		"https://new-ns.ci.distro.ultrawombat.com/operate/identity-callback",
	}

	result := filterRedirectURIs(existing, newURIs, "new-ns.ci.distro.ultrawombat.com")

	expected := []string{
		"https://production.example.com/callback",
		"https://other-ns.ci.distro.ultrawombat.com/identity/auth/login-callback",
		"https://staging.distribution.aws.camunda.cloud/callback",
		"https://new-ns.ci.distro.ultrawombat.com/identity/auth/login-callback",
		"https://new-ns.ci.distro.ultrawombat.com/operate/identity-callback",
	}

	if len(result) != len(expected) {
		t.Fatalf("filterRedirectURIs count = %d, want %d\ngot:  %v\nwant: %v", len(result), len(expected), result, expected)
	}
	for i, uri := range result {
		if uri != expected[i] {
			t.Errorf("result[%d] = %q, want %q", i, uri, expected[i])
		}
	}
}

func TestCapRedirectURIs_SharedBudgetWithPublicClient(t *testing.T) {
	host := "new-ns" + ciDomainSuffix
	var web, spa []string
	for i := range 120 {
		web = append(web, fmt.Sprintf("https://web-%d%s/callback", i, ciDomainSuffix))
	}
	for i := range 118 {
		spa = append(spa, fmt.Sprintf("https://spa-%d%s/callback", i, ciDomainSuffix))
	}
	for i := range 17 {
		web = append(web, fmt.Sprintf("https://prod-%d.example.com/callback", i))
	}
	newWeb, newSpa := buildRedirectURIs(host)
	web = filterRedirectURIs(web, newWeb, host)
	spa = filterRedirectURIs(spa, newSpa, host)
	const publicClient = 1

	web, spa = capRedirectURIs(web, spa, publicClient, host)

	if got := len(web) + len(spa) + publicClient; got > maxRedirectURIs {
		t.Errorf("total redirect URIs = %d, want <= %d", got, maxRedirectURIs)
	}
	for _, uri := range append(newWeb, newSpa...) {
		if !slices.Contains(web, uri) && !slices.Contains(spa, uri) {
			t.Errorf("current host URI dropped: %s", uri)
		}
	}
	for i := range 17 {
		if uri := fmt.Sprintf("https://prod-%d.example.com/callback", i); !slices.Contains(web, uri) {
			t.Errorf("non-CI URI dropped: %s", uri)
		}
	}
	if slices.Contains(web, "https://web-0"+ciDomainSuffix+"/callback") {
		t.Error("oldest CI web URI was not pruned first")
	}
}

func TestCapRedirectURIs_EvictsOldestDeploymentWhole(t *testing.T) {
	var web, spa []string
	for i := 1; i <= 40; i++ {
		host := fmt.Sprintf("ns-%d%s", i, ciDomainSuffix)
		newWeb, newSpa := buildRedirectURIs(host)
		web = filterRedirectURIs(web, newWeb, host)
		spa = filterRedirectURIs(spa, newSpa, host)
		web, spa = capRedirectURIs(web, spa, 0, host)
	}

	if got := len(web) + len(spa); got > maxRedirectURIs {
		t.Fatalf("total redirect URIs = %d, want <= %d", got, maxRedirectURIs)
	}
	perHost := len(webRedirectPaths) + len(spaRedirectPaths)
	keep := maxRedirectURIs / perHost
	for i := 1; i <= 40; i++ {
		host := fmt.Sprintf("ns-%d%s", i, ciDomainSuffix)
		wantWeb, wantSpa := buildRedirectURIs(host)
		n := 0
		for _, uri := range append(wantWeb, wantSpa...) {
			if slices.Contains(web, uri) || slices.Contains(spa, uri) {
				n++
			}
		}
		retained := i > 40-keep
		if retained && n != perHost {
			t.Errorf("host %d kept %d of %d URIs, want all", i, n, perHost)
		}
		if !retained && n != 0 {
			t.Errorf("host %d kept %d URIs, want it evicted whole", i, n)
		}
	}
}

func TestCapRedirectURIs_MatchesOnParsedHostname(t *testing.T) {
	host := "new-ns" + ciDomainSuffix
	queryOnly := "https://old-ns" + ciDomainSuffix + "?flow=oidc"
	pathMention := "https://production.example.com/auth" + ciDomainSuffix + "/callback"
	web := []string{queryOnly, pathMention}
	for i := range maxRedirectURIs {
		web = append(web, fmt.Sprintf("https://keep-%d.example.com/cb", i))
	}
	newWeb, newSpa := buildRedirectURIs(host)
	web = filterRedirectURIs(web, newWeb, host)
	spa := filterRedirectURIs(nil, newSpa, host)

	done := make(chan struct{})
	go func() {
		web, spa = capRedirectURIs(web, spa, 0, host)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("capRedirectURIs did not terminate")
	}

	if slices.Contains(web, queryOnly) {
		t.Errorf("query-only CI URI was not evicted: %s", queryOnly)
	}
	if !slices.Contains(web, pathMention) {
		t.Errorf("non-CI host with CI domain in its path was evicted: %s", pathMention)
	}
}

func TestCapRedirectURIs_UnderLimitUnchanged(t *testing.T) {
	web := []string{"https://a" + ciDomainSuffix + "/cb"}
	spa := []string{"https://b" + ciDomainSuffix + "/cb"}
	gotWeb, gotSpa := capRedirectURIs(web, spa, 0, "new-ns"+ciDomainSuffix)
	if !slices.Equal(gotWeb, web) || !slices.Equal(gotSpa, spa) {
		t.Errorf("lists changed under the limit: web=%v spa=%v", gotWeb, gotSpa)
	}
}

func TestFilterRedirectURIs_EmptyExisting(t *testing.T) {
	newURIs := []string{
		"https://ns.ci.distro.ultrawombat.com/callback",
	}

	result := filterRedirectURIs(nil, newURIs, "ns.ci.distro.ultrawombat.com")
	if len(result) != 1 {
		t.Fatalf("expected 1 URI, got %d: %v", len(result), result)
	}
	if result[0] != newURIs[0] {
		t.Errorf("result[0] = %q, want %q", result[0], newURIs[0])
	}
}

func TestFilterRedirectURIs_DeduplicatesNewURIs(t *testing.T) {
	newURIs := []string{
		"https://ns.ci.distro.ultrawombat.com/callback",
		"https://ns.ci.distro.ultrawombat.com/callback", // duplicate
	}

	result := filterRedirectURIs(nil, newURIs, "ns.ci.distro.ultrawombat.com")
	if len(result) != 1 {
		t.Fatalf("expected 1 URI after dedup, got %d: %v", len(result), result)
	}
}

func TestResolveRedirectOpts_MissingRequired(t *testing.T) {
	tests := []struct {
		name string
		opts RedirectURIOptions
		want string
	}{
		{
			name: "missing directory ID",
			opts: RedirectURIOptions{ObjectID: "oid", IngressHost: "host", ClientID: "cid", ClientSecret: "cs"},
			want: "ENTRA_APP_DIRECTORY_ID",
		},
		{
			name: "missing client ID",
			opts: RedirectURIOptions{ObjectID: "oid", IngressHost: "host", DirectoryID: "did", ClientSecret: "cs"},
			want: "ENTRA_APP_CLIENT_ID",
		},
		{
			name: "missing client secret",
			opts: RedirectURIOptions{ObjectID: "oid", IngressHost: "host", DirectoryID: "did", ClientID: "cid"},
			want: "ENTRA_APP_CLIENT_SECRET",
		},
		{
			name: "missing object ID",
			opts: RedirectURIOptions{IngressHost: "host", DirectoryID: "did", ClientID: "cid", ClientSecret: "cs"},
			want: "ENTRA_APP_OBJECT_ID",
		},
		{
			name: "missing ingress host",
			opts: RedirectURIOptions{ObjectID: "oid", DirectoryID: "did", ClientID: "cid", ClientSecret: "cs"},
			want: "ingress host is required",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("ENTRA_APP_DIRECTORY_ID", "")
			t.Setenv("ENTRA_APP_CLIENT_ID", "")
			t.Setenv("ENTRA_APP_CLIENT_SECRET", "")
			t.Setenv("ENTRA_APP_OBJECT_ID", "")

			err := resolveRedirectOpts(&tc.opts)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not contain %q", err, tc.want)
			}
		})
	}
}

func TestResolveRedirectOpts_EnvFallback(t *testing.T) {
	t.Setenv("ENTRA_APP_DIRECTORY_ID", "env-dir-id")
	t.Setenv("ENTRA_APP_CLIENT_ID", "env-client-id")
	t.Setenv("ENTRA_APP_CLIENT_SECRET", "env-secret")
	t.Setenv("ENTRA_APP_OBJECT_ID", "env-object-id")

	opts := RedirectURIOptions{IngressHost: "my-host.example.com"}
	if err := resolveRedirectOpts(&opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.DirectoryID != "env-dir-id" {
		t.Errorf("DirectoryID = %q, want %q", opts.DirectoryID, "env-dir-id")
	}
	if opts.ClientID != "env-client-id" {
		t.Errorf("ClientID = %q, want %q", opts.ClientID, "env-client-id")
	}
	if opts.ClientSecret != "env-secret" {
		t.Errorf("ClientSecret = %q, want %q", opts.ClientSecret, "env-secret")
	}
	if opts.ObjectID != "env-object-id" {
		t.Errorf("ObjectID = %q, want %q", opts.ObjectID, "env-object-id")
	}
}

func TestUpdateRedirectURIs_HappyPath(t *testing.T) {
	noRedirectVerifyWait(t)
	var patchReceived map[string]any

	srv := newTestServer(t, map[string]http.HandlerFunc{
		"POST /test-tenant/oauth2/v2.0/token": tokenHandler(),
		"GET /applications/parent-obj-id": func(w http.ResponseWriter, r *http.Request) {
			if patchReceived != nil {
				jsonResponse(w, 200, patchReceived)
				return
			}
			jsonResponse(w, 200, map[string]any{
				"web": map[string]any{
					"redirectUris": []string{
						"https://production.example.com/callback",
						"https://old-ns.ci.distro.ultrawombat.com/identity/auth/login-callback",
						"https://old-ns.ci.distro.ultrawombat.com/operate/identity-callback",
						"https://corrupt.ci.distro.ultrawombat.com/callback,",
					},
				},
				"spa": map[string]any{
					"redirectUris": []string{
						"https://production.example.com/spa-callback",
						"https://old-ns.ci.distro.ultrawombat.com/modeler/login-callback",
					},
				},
			})
		},
		"PATCH /applications/parent-obj-id": func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewDecoder(r.Body).Decode(&patchReceived)
			w.WriteHeader(204)
		},
	})
	defer srv.Close()

	origGraph := graphBaseURL
	origLogin := loginBaseURL
	graphBaseURL = srv.URL
	loginBaseURL = srv.URL
	defer func() {
		graphBaseURL = origGraph
		loginBaseURL = origLogin
	}()

	err := UpdateRedirectURIs(context.Background(), RedirectURIOptions{
		ObjectID:     "parent-obj-id",
		IngressHost:  "new-ns.ci.distro.ultrawombat.com",
		DirectoryID:  "test-tenant",
		ClientID:     "parent-client",
		ClientSecret: "parent-secret",
		HTTPClient:   srv.Client(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if patchReceived == nil {
		t.Fatal("expected PATCH request, got none")
	}

	// Verify web URIs: production and other CI hosts preserved, new added.
	webData, ok := patchReceived["web"].(map[string]any)
	if !ok {
		t.Fatal("missing 'web' in PATCH payload")
	}
	webURIs, ok := webData["redirectUris"].([]any)
	if !ok {
		t.Fatal("missing 'redirectUris' in web payload")
	}

	webStrings := make([]string, len(webURIs))
	for i, u := range webURIs {
		webStrings[i] = u.(string)
	}

	// Should contain production + 2 other-host CI URIs + 5 new web URIs = 8.
	// Only the corrupt URI should be removed.
	if len(webStrings) != 8 {
		t.Errorf("web URI count = %d, want 8, got: %v", len(webStrings), webStrings)
	}

	// Verify production URI is preserved.
	found := slices.Contains(webStrings, "https://production.example.com/callback")
	if !found {
		t.Error("production URI was not preserved in web URIs")
	}

	// Verify another CI deployment's URIs were preserved.
	for _, want := range []string{
		"https://old-ns.ci.distro.ultrawombat.com/identity/auth/login-callback",
		"https://old-ns.ci.distro.ultrawombat.com/operate/identity-callback",
	} {
		if !slices.Contains(webStrings, want) {
			t.Errorf("other CI host URI should have been preserved: %s", want)
		}
	}

	// Verify corrupt URI was removed.
	for _, u := range webStrings {
		if strings.HasSuffix(u, ",") {
			t.Errorf("corrupt URI with trailing comma should have been removed: %s", u)
		}
	}

	// Verify SPA URIs.
	spaData, ok := patchReceived["spa"].(map[string]any)
	if !ok {
		t.Fatal("missing 'spa' in PATCH payload")
	}
	spaURIs, ok := spaData["redirectUris"].([]any)
	if !ok {
		t.Fatal("missing 'redirectUris' in spa payload")
	}

	// Should contain production + 1 other-host CI URI + 2 new SPA URIs = 4.
	if len(spaURIs) != 4 {
		spaStrings := make([]string, len(spaURIs))
		for i, u := range spaURIs {
			spaStrings[i] = u.(string)
		}
		t.Errorf("spa URI count = %d, want 4, got: %v", len(spaURIs), spaStrings)
	}
}

func TestUpdateRedirectURIs_OverLimitReservesPublicClient(t *testing.T) {
	var web, spa []string
	for i := range 36 {
		w, sp := buildRedirectURIs(fmt.Sprintf("ns-%d%s", i, ciDomainSuffix))
		web = append(web, w...)
		spa = append(spa, sp...)
	}
	for i := range 4 {
		web = append(web, fmt.Sprintf("https://prod-%d.example.com/callback", i))
	}
	publicClient := []string{"https://login.microsoftonline.com/common/oauth2/nativeclient"}
	var patchReceived struct {
		Web struct {
			RedirectURIs []string `json:"redirectUris"`
		} `json:"web"`
		Spa struct {
			RedirectURIs []string `json:"redirectUris"`
		} `json:"spa"`
	}

	noRedirectVerifyWait(t)
	patched := false
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"POST /test-tenant/oauth2/v2.0/token": tokenHandler(),
		"GET /applications/parent-obj-id": func(w http.ResponseWriter, r *http.Request) {
			if patched {
				web, spa = patchReceived.Web.RedirectURIs, patchReceived.Spa.RedirectURIs
			}
			jsonResponse(w, 200, map[string]any{
				"web":          map[string]any{"redirectUris": web},
				"spa":          map[string]any{"redirectUris": spa},
				"publicClient": map[string]any{"redirectUris": publicClient},
			})
		},
		"PATCH /applications/parent-obj-id": func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewDecoder(r.Body).Decode(&patchReceived)
			patched = true
			w.WriteHeader(204)
		},
	})
	defer srv.Close()

	origGraph, origLogin := graphBaseURL, loginBaseURL
	graphBaseURL, loginBaseURL = srv.URL, srv.URL
	defer func() { graphBaseURL, loginBaseURL = origGraph, origLogin }()

	host := "new-ns" + ciDomainSuffix
	err := UpdateRedirectURIs(context.Background(), RedirectURIOptions{
		ObjectID:     "parent-obj-id",
		IngressHost:  host,
		DirectoryID:  "test-tenant",
		ClientID:     "parent-client",
		ClientSecret: "parent-secret",
		HTTPClient:   srv.Client(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := len(patchReceived.Web.RedirectURIs) + len(patchReceived.Spa.RedirectURIs) + len(publicClient)
	if got > maxRedirectURIs {
		t.Errorf("web+spa+publicClient = %d, want <= %d", got, maxRedirectURIs)
	}
	newWeb, newSpa := buildRedirectURIs(host)
	for _, uri := range newWeb {
		if !slices.Contains(patchReceived.Web.RedirectURIs, uri) {
			t.Errorf("current host web URI missing from PATCH: %s", uri)
		}
	}
	for _, uri := range newSpa {
		if !slices.Contains(patchReceived.Spa.RedirectURIs, uri) {
			t.Errorf("current host SPA URI missing from PATCH: %s", uri)
		}
	}
	oldWeb, _ := buildRedirectURIs("ns-0" + ciDomainSuffix)
	if slices.Contains(patchReceived.Web.RedirectURIs, oldWeb[0]) {
		t.Error("oldest CI deployment was not evicted")
	}
}

func TestUpdateRedirectURIs_RewritesAfterConcurrentOverwrite(t *testing.T) {
	host := redirectTestHost
	otherWeb, otherSpa := buildRedirectURIs("other-ns" + ciDomainSuffix)
	f := &fakeRedirectApp{}
	f.afterPatch = func(f *fakeRedirectApp) {
		if f.patches == 1 {
			f.app.Web.RedirectURIs = slices.Clone(otherWeb)
			f.app.Spa.RedirectURIs = slices.Clone(otherSpa)
		}
	}

	if err := runUpdateRedirectURIs(t, f); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if f.patches != 2 {
		t.Errorf("PATCH count = %d, want 2", f.patches)
	}
	if !hasHostRedirectURIs(f.app, host) {
		t.Error("current host URIs missing after rewrite")
	}
	if !hasHostRedirectURIs(f.app, "other-ns"+ciDomainSuffix) {
		t.Error("concurrent writer's URIs dropped by the rewrite")
	}
}

func TestUpdateRedirectURIs_WaitsOutStaleReadsWithoutRewriting(t *testing.T) {
	host := redirectTestHost
	otherWeb, otherSpa := buildRedirectURIs("other-ns" + ciDomainSuffix)
	f := &fakeRedirectApp{staleReads: redirectVerifyReads - 1}
	f.app.Web.RedirectURIs, f.app.Spa.RedirectURIs = otherWeb, otherSpa

	if err := runUpdateRedirectURIs(t, f); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if f.patches != 1 {
		t.Errorf("PATCH count = %d, want 1", f.patches)
	}
	if !hasHostRedirectURIs(f.app, host) || !hasHostRedirectURIs(f.app, "other-ns"+ciDomainSuffix) {
		t.Error("redirect URIs of a host are missing after stale reads")
	}
}

func TestUpdateRedirectURIs_FailsWhenOverwrittenEveryAttempt(t *testing.T) {
	f := &fakeRedirectApp{}
	f.afterPatch = func(f *fakeRedirectApp) {
		f.app.Web.RedirectURIs, f.app.Spa.RedirectURIs = nil, nil
	}

	err := runUpdateRedirectURIs(t, f)

	if err == nil || !strings.Contains(err.Error(), "concurrent update") {
		t.Fatalf("error = %v, want concurrent update failure", err)
	}
	if f.patches != redirectVerifyAttempts {
		t.Errorf("PATCH count = %d, want %d", f.patches, redirectVerifyAttempts)
	}
}

func TestUpdateRedirectURIs_FailsWhenVerificationReadFails(t *testing.T) {
	f := &fakeRedirectApp{getStatusAfterPatch: 503}

	err := runUpdateRedirectURIs(t, f)

	if err == nil || !strings.Contains(err.Error(), "status=503") {
		t.Fatalf("error = %v, want verification read failure with status=503", err)
	}
	if f.patches != 1 {
		t.Errorf("PATCH count = %d, want 1", f.patches)
	}
}

func TestUpdateRedirectURIs_PatchFails(t *testing.T) {
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"POST /test-tenant/oauth2/v2.0/token": tokenHandler(),
		"GET /applications/parent-obj-id": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]any{
				"web": map[string]any{"redirectUris": []string{}},
				"spa": map[string]any{"redirectUris": []string{}},
			})
		},
		"PATCH /applications/parent-obj-id": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 400, map[string]any{
				"error": map[string]string{
					"code":    "Request_BadRequest",
					"message": "too many redirect URIs",
				},
			})
		},
	})
	defer srv.Close()

	origGraph := graphBaseURL
	origLogin := loginBaseURL
	graphBaseURL = srv.URL
	loginBaseURL = srv.URL
	defer func() {
		graphBaseURL = origGraph
		loginBaseURL = origLogin
	}()

	err := UpdateRedirectURIs(context.Background(), RedirectURIOptions{
		ObjectID:     "parent-obj-id",
		IngressHost:  "ns.ci.distro.ultrawombat.com",
		DirectoryID:  "test-tenant",
		ClientID:     "parent-client",
		ClientSecret: "parent-secret",
		HTTPClient:   srv.Client(),
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "status=400") {
		t.Errorf("error %q does not mention status=400", err)
	}
}

func TestUpdateRedirectURIs_InvalidOpts(t *testing.T) {
	t.Setenv("ENTRA_APP_DIRECTORY_ID", "")
	t.Setenv("ENTRA_APP_CLIENT_ID", "")
	t.Setenv("ENTRA_APP_CLIENT_SECRET", "")
	t.Setenv("ENTRA_APP_OBJECT_ID", "")

	err := UpdateRedirectURIs(context.Background(), RedirectURIOptions{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGraphPatch(t *testing.T) {
	var receivedBody map[string]any
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"PATCH /applications/test-id": func(w http.ResponseWriter, r *http.Request) {
			if got := r.Header.Get("Prefer"); got != "" {
				t.Errorf("Prefer = %q, want no upsert header", got)
			}
			_ = json.NewDecoder(r.Body).Decode(&receivedBody)
			w.WriteHeader(204)
		},
	})
	defer srv.Close()

	origGraph := graphBaseURL
	graphBaseURL = srv.URL
	defer func() { graphBaseURL = origGraph }()

	payload := map[string]string{"key": "value"}
	_, statusCode, err := graphPatch(context.Background(), srv.Client(), "token", "/applications/test-id", payload, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if statusCode != 204 {
		t.Errorf("statusCode = %d, want 204", statusCode)
	}
	if receivedBody == nil {
		t.Fatal("expected PATCH body, got nil")
	}
	if receivedBody["key"] != "value" {
		t.Errorf("body key = %q, want %q", receivedBody["key"], "value")
	}
}

// TestEnsureLoginApp_ReplacesRedirectURIs asserts the deployment's own app is
// created for the namespace and that its redirect URIs are written as a full
// replacement holding only this host's URIs — the property that makes a
// concurrent deployment unable to drop them.
func TestEnsureLoginApp_ReplacesRedirectURIs(t *testing.T) {
	const host = "gke-qaen-1234-8-10-qaen.ci.distro.ultrawombat.com"

	patched := map[string]any{}
	var grants []map[string]any
	var roleAssignment map[string]any
	spCreated := false
	created := false
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"POST /test-tenant/oauth2/v2.0/token": tokenHandler(),
		"GET /applications": func(w http.ResponseWriter, r *http.Request) {
			if !created {
				jsonResponse(w, 200, map[string]any{"value": []any{}})
				return
			}
			jsonResponse(w, 200, map[string]any{"value": []any{
				map[string]string{"appId": "login-app-id", "id": "login-obj-id"},
			}})
		},
		"POST /applications": func(w http.ResponseWriter, r *http.Request) {
			created = true
			jsonResponse(w, 201, map[string]string{
				"appId": "login-app-id",
				"id":    "login-obj-id",
			})
		},
		"GET /applications/login-obj-id": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]any{"passwordCredentials": []any{}})
		},
		"PATCH /applications/login-obj-id": func(w http.ResponseWriter, r *http.Request) {
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decode PATCH body: %v", err)
			}
			mergePatch(patched, body)
			w.WriteHeader(http.StatusNoContent)
		},
		"POST /applications/login-obj-id/addPassword": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]string{"secretText": "login-secret"})
		},
		"POST /oauth2PermissionGrants": func(w http.ResponseWriter, r *http.Request) {
			var grant map[string]any
			if err := json.NewDecoder(r.Body).Decode(&grant); err != nil {
				t.Errorf("decode grant body: %v", err)
			}
			grants = append(grants, grant)
			jsonResponse(w, 201, map[string]string{"id": "grant-id"})
		},
		"PATCH /servicePrincipals(appId='login-app-id')": func(w http.ResponseWriter, r *http.Request) {
			spCreated = true
			jsonResponse(w, 201, map[string]string{"id": "sp-id"})
		},
		"GET /servicePrincipals(appId='login-app-id')": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]string{"id": "sp-id"})
		},
		"GET /servicePrincipals(appId='00000003-0000-0000-c000-000000000000')": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]string{"id": "graph-sp-id"})
		},
		"GET /servicePrincipals(appId='parent-client-id')": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]string{"id": "parent-sp-id"})
		},
		"POST /servicePrincipals/sp-id/appRoleAssignedTo": func(w http.ResponseWriter, r *http.Request) {
			if err := json.NewDecoder(r.Body).Decode(&roleAssignment); err != nil {
				t.Errorf("decode app role assignment body: %v", err)
			}
			jsonResponse(w, 201, map[string]string{"id": "assignment-id"})
		},
	})
	defer srv.Close()

	origGraph, origLogin, origSleep := graphBaseURL, loginBaseURL, propagationSleepDuration
	graphBaseURL, loginBaseURL, propagationSleepDuration = srv.URL, srv.URL, 0
	defer func() {
		graphBaseURL, loginBaseURL, propagationSleepDuration = origGraph, origLogin, origSleep
	}()

	app, err := EnsureLoginApp(context.Background(), Options{
		Namespace:    "test-ns",
		DirectoryID:  "test-tenant",
		ClientID:     "parent-client-id",
		ClientSecret: "parent-secret",
		HTTPClient:   srv.Client(),
	}, host)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if app.AppID != "login-app-id" || app.ClientSecret != "login-secret" {
		t.Errorf("app = %+v, want appId login-app-id and secret login-secret", app)
	}
	if !spCreated {
		t.Error("expected a service principal for the login app")
	}

	if got, _ := patched["identifierUris"].([]any); len(got) != 1 || got[0] != "api://login-app-id" {
		t.Errorf("identifierUris = %v, want [api://login-app-id]", patched["identifierUris"])
	}
	api, ok := patched["api"].(map[string]any)
	if !ok {
		t.Fatalf("PATCH body has no api block: %v", patched)
	}
	if v := api["requestedAccessTokenVersion"]; v != float64(2) {
		t.Errorf("requestedAccessTokenVersion = %v, want 2 (a v1 token's issuer fails validation)", v)
	}
	scopes, _ := api["oauth2PermissionScopes"].([]any)
	if len(scopes) != 1 {
		t.Fatalf("oauth2PermissionScopes = %v, want exactly one; without a scope of its own the app is not a valid resource for \"<clientId>/.default\" and authorize fails with AADSTS650057", api["oauth2PermissionScopes"])
	}
	scope, _ := scopes[0].(map[string]any)
	scopeID, _ := scope["id"].(string)
	if scopeID == "" {
		t.Fatalf("scope = %v, want an id", scope)
	}
	if scope["type"] != "User" {
		t.Errorf("scope type = %v, want User", scope["type"])
	}

	required, _ := patched["requiredResourceAccess"].([]any)
	if len(required) != 1 {
		t.Fatalf("requiredResourceAccess = %v, want the app's own scope; authorize reads the valid-resource list off the client, not off the resource", required)
	}
	entry, _ := required[0].(map[string]any)
	if entry["resourceAppId"] != "login-app-id" {
		t.Errorf("resourceAppId = %v, want login-app-id", entry["resourceAppId"])
	}
	access, _ := entry["resourceAccess"].([]any)
	if len(access) != 1 {
		t.Fatalf("resourceAccess = %v, want one entry", access)
	}
	if got, _ := access[0].(map[string]any); got["id"] != scopeID || got["type"] != "Scope" {
		t.Errorf("resourceAccess[0] = %v, want the exposed scope id %s as a Scope", got, scopeID)
	}

	if len(grants) != 2 {
		t.Fatalf("oauth2PermissionGrants = %v, want two; consent lives on the service principal, and a run-created one starts with an empty ledger", grants)
	}
	for _, grant := range grants {
		if grant["consentType"] != "AllPrincipals" {
			t.Errorf("consentType = %v, want AllPrincipals; a per-user grant leaves every other pool account prompted", grant["consentType"])
		}
		if grant["clientId"] != "sp-id" {
			t.Errorf("clientId = %v, want sp-id", grant["clientId"])
		}
	}
	if grants[0]["resourceId"] != "graph-sp-id" || grants[0]["scope"] != "openid profile email" {
		t.Errorf("first grant = %v, want the Microsoft Graph sign-in scopes", grants[0])
	}
	if grants[1]["resourceId"] != "sp-id" || grants[1]["scope"] != "user_impersonation" {
		t.Errorf("second grant = %v, want the app's own scope", grants[1])
	}

	roles, ok := patched["appRoles"].([]any)
	if !ok || len(roles) != 1 {
		t.Fatalf("appRoles = %v, want exactly one; without it Entra refuses the parent app a token for this deployment", patched["appRoles"])
	}
	role, _ := roles[0].(map[string]any)
	if role["value"] != loginAppRoleValue {
		t.Errorf("app role value = %v, want %s", role["value"], loginAppRoleValue)
	}
	if members, _ := role["allowedMemberTypes"].([]any); len(members) != 1 || members[0] != "Application" {
		t.Errorf("allowedMemberTypes = %v, want [Application]; a User role cannot back client credentials", role["allowedMemberTypes"])
	}
	if role["id"] != loginAppRoleID("login-app-id") {
		t.Errorf("app role id = %v, want the id derived from the app id so re-deploys keep the assignment valid", role["id"])
	}
	if roleAssignment == nil {
		t.Fatalf("no appRoleAssignedTo POST; the parent app would still mint tokens with its own audience")
	}
	if roleAssignment["principalId"] != "parent-sp-id" || roleAssignment["resourceId"] != "sp-id" {
		t.Errorf("assignment = %v, want the parent app's principal on the login app", roleAssignment)
	}
	if roleAssignment["appRoleId"] != loginAppRoleID("login-app-id") {
		t.Errorf("assignment appRoleId = %v, want the role configureLoginApp wrote", roleAssignment["appRoleId"])
	}

	wantWeb, wantSpa := buildRedirectURIs(host)
	gotWeb := patchedURIs(t, patched, "web")
	gotSpa := patchedURIs(t, patched, "spa")
	if !slices.Equal(gotWeb, wantWeb) {
		t.Errorf("web redirectUris = %v, want exactly %v", gotWeb, wantWeb)
	}
	if !slices.Equal(gotSpa, wantSpa) {
		t.Errorf("spa redirectUris = %v, want exactly %v", gotSpa, wantSpa)
	}
}

// TestEnsureLoginApp_ReprovisionsReusedApp deploys twice to one namespace: the
// second run reuses the app, so it must keep the scope id the first run exposed
// and treat the consent grants and role assignment that already exist as done.
func TestEnsureLoginApp_ReprovisionsReusedApp(t *testing.T) {
	const host = "gke-qaen-1234-8-10-qaen.ci.distro.ultrawombat.com"

	created := false
	var scopeIDs []string
	grants := map[string]bool{}
	assigned := false
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"POST /test-tenant/oauth2/v2.0/token": tokenHandler(),
		"GET /applications": func(w http.ResponseWriter, r *http.Request) {
			if !created {
				jsonResponse(w, 200, map[string]any{"value": []any{}})
				return
			}
			jsonResponse(w, 200, map[string]any{"value": []any{
				map[string]string{"appId": "login-app-id", "id": "login-obj-id"},
			}})
		},
		"POST /applications": func(w http.ResponseWriter, r *http.Request) {
			created = true
			jsonResponse(w, 201, map[string]string{"appId": "login-app-id", "id": "login-obj-id"})
		},
		"GET /applications/login-obj-id": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]any{"passwordCredentials": []any{}})
		},
		"PATCH /applications/login-obj-id": func(w http.ResponseWriter, r *http.Request) {
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decode PATCH body: %v", err)
			}
			if api, ok := body["api"].(map[string]any); ok {
				scopes, _ := api["oauth2PermissionScopes"].([]any)
				if len(scopes) == 1 {
					scope, _ := scopes[0].(map[string]any)
					id, _ := scope["id"].(string)
					scopeIDs = append(scopeIDs, id)
				}
			}
			w.WriteHeader(http.StatusNoContent)
		},
		"POST /applications/login-obj-id/addPassword": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]string{"secretText": "login-secret"})
		},
		"POST /oauth2PermissionGrants": func(w http.ResponseWriter, r *http.Request) {
			var grant map[string]any
			if err := json.NewDecoder(r.Body).Decode(&grant); err != nil {
				t.Errorf("decode grant body: %v", err)
			}
			key := fmt.Sprintf("%v|%v", grant["resourceId"], grant["scope"])
			if grants[key] {
				jsonResponse(w, 409, map[string]any{"error": map[string]string{
					"code":    "Request_MultipleObjectsWithSameKeyValue",
					"message": "Permission entry already exists.",
				}})
				return
			}
			grants[key] = true
			jsonResponse(w, 201, map[string]string{"id": "grant-id"})
		},
		"PATCH /servicePrincipals(appId='login-app-id')": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 201, map[string]string{"id": "sp-id"})
		},
		"GET /servicePrincipals(appId='login-app-id')": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]string{"id": "sp-id"})
		},
		"GET /servicePrincipals(appId='00000003-0000-0000-c000-000000000000')": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]string{"id": "graph-sp-id"})
		},
		"GET /servicePrincipals(appId='parent-client-id')": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]string{"id": "parent-sp-id"})
		},
		"POST /servicePrincipals/sp-id/appRoleAssignedTo": func(w http.ResponseWriter, r *http.Request) {
			if assigned {
				jsonResponse(w, 400, map[string]any{"error": map[string]string{
					"code":    "Request_BadRequest",
					"message": "Permission being assigned already exists on the object",
				}})
				return
			}
			assigned = true
			jsonResponse(w, 201, map[string]string{"id": "assignment-id"})
		},
	})
	defer srv.Close()

	origGraph, origLogin, origSleep := graphBaseURL, loginBaseURL, propagationSleepDuration
	graphBaseURL, loginBaseURL, propagationSleepDuration = srv.URL, srv.URL, 0
	defer func() {
		graphBaseURL, loginBaseURL, propagationSleepDuration = origGraph, origLogin, origSleep
	}()

	opts := Options{
		Namespace:    "test-ns",
		DirectoryID:  "test-tenant",
		ClientID:     "parent-client-id",
		ClientSecret: "parent-secret",
		HTTPClient:   srv.Client(),
	}
	for run := 1; run <= 2; run++ {
		if _, err := EnsureLoginApp(context.Background(), opts, host); err != nil {
			t.Fatalf("run %d: unexpected error: %v", run, err)
		}
	}

	if len(scopeIDs) != 2 {
		t.Fatalf("scope ids written = %v, want one per run", scopeIDs)
	}
	if scopeIDs[0] != scopeIDs[1] {
		t.Errorf("scope id changed between runs (%s -> %s); Entra refuses to drop an enabled scope", scopeIDs[0], scopeIDs[1])
	}
	if scopeIDs[0] != loginAppScopeID("login-app-id") {
		t.Errorf("scope id = %s, want the id derived from the app id", scopeIDs[0])
	}
}

// TestEnsureLoginApp_RequiresIngressHost guards against provisioning an app
// with no redirect URI, which would fail every sign-in with AADSTS50011.
func TestEnsureLoginApp_RequiresIngressHost(t *testing.T) {
	_, err := EnsureLoginApp(context.Background(), Options{
		Namespace:    "test-ns",
		DirectoryID:  "test-tenant",
		ClientID:     "parent-client-id",
		ClientSecret: "parent-secret",
	}, "")
	if err == nil {
		t.Fatal("expected an error when the ingress host is empty")
	}
}

// mergePatch applies one PATCH body the way Graph does: a complex type such as
// api keeps the members the body leaves out, so successive writes accumulate.
func mergePatch(into, body map[string]any) {
	for key, value := range body {
		nested, isMap := value.(map[string]any)
		existing, wasMap := into[key].(map[string]any)
		if isMap && wasMap {
			mergePatch(existing, nested)
			continue
		}
		into[key] = value
	}
}

func patchedURIs(t *testing.T, patched map[string]any, platform string) []string {
	t.Helper()
	block, ok := patched[platform].(map[string]any)
	if !ok {
		t.Fatalf("PATCH body has no %q block: %v", platform, patched)
	}
	raw, ok := block["redirectUris"].([]any)
	if !ok {
		t.Fatalf("PATCH %q block has no redirectUris: %v", platform, block)
	}
	uris := make([]string, 0, len(raw))
	for _, u := range raw {
		uris = append(uris, u.(string))
	}
	return uris
}

func TestPermissionGrantRetryable(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		want   bool
	}{
		{"unreplicated service principal", http.StatusBadRequest, `{"error":{"code":"Request_BadRequest","message":"Specified resourceId was not found."}}`, true},
		{"duplicate grant", http.StatusBadRequest, `{"error":{"message":"Permission entry already exists."}}`, false},
		{"duplicate grant conflict", http.StatusConflict, `{"error":{"message":"Permission entry already exists."}}`, false},
		{"generic bad request", http.StatusBadRequest, `{"error":{"code":"Request_BadRequest","message":"Invalid scope"}}`, false},
		{"forbidden", http.StatusForbidden, `{"error":{"code":"Authorization_RequestDenied"}}`, false},
		{"throttled", http.StatusTooManyRequests, ``, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := permissionGrantRetryable(tc.status, []byte(tc.body), nil); got != tc.want {
				t.Errorf("permissionGrantRetryable(%d) = %v, want %v", tc.status, got, tc.want)
			}
		})
	}
}
