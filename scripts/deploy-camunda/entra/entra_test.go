package entra

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
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

func jsonResponse(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body) //nolint:errcheck
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
			jsonResponse(w, 200, map[string]interface{}{
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
			jsonResponse(w, 200, map[string]interface{}{
				"value": []interface{}{},
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
			jsonResponse(w, 400, map[string]interface{}{
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
			jsonResponse(w, 200, map[string]interface{}{
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

	var addAttempts int32
	const flakyAttempts int32 = 3

	srv := newTestServer(t, map[string]http.HandlerFunc{
		"GET /applications/obj-flaky": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"passwordCredentials": []map[string]string{},
			})
		},
		"POST /applications/obj-flaky/addPassword": func(w http.ResponseWriter, r *http.Request) {
			n := atomic.AddInt32(&addAttempts, 1)
			if n <= flakyAttempts {
				jsonResponse(w, 404, map[string]interface{}{
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
	if got := atomic.LoadInt32(&addAttempts); got != flakyAttempts+1 {
		t.Errorf("addPassword attempts = %d, want %d (retry loop did not retry through 404s)", got, flakyAttempts+1)
	}
}

// TestRotateCredentials_GivesUpAfterMaxAttempts verifies the retry loop
// terminates with an error that references the last status and body when
// Graph persistently returns 404.
func TestRotateCredentials_GivesUpAfterMaxAttempts(t *testing.T) {
	shrinkRetryTimings(t)

	var addAttempts int32
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"GET /applications/obj-stuck": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"passwordCredentials": []map[string]string{},
			})
		},
		"POST /applications/obj-stuck/addPassword": func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&addAttempts, 1)
			jsonResponse(w, 404, map[string]interface{}{
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
	if got := atomic.LoadInt32(&addAttempts); got != int32(retryMaxAttempts) {
		t.Errorf("addPassword attempts = %d, want %d (retry loop did not use full budget)", got, retryMaxAttempts)
	}
}

// TestRotateCredentials_DoesNotRetry401 verifies terminal auth errors fail
// fast rather than burning the retry budget.
func TestRotateCredentials_DoesNotRetry401(t *testing.T) {
	shrinkRetryTimings(t)

	var addAttempts int32
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"GET /applications/obj-unauthorized": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"passwordCredentials": []map[string]string{},
			})
		},
		"POST /applications/obj-unauthorized/addPassword": func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&addAttempts, 1)
			jsonResponse(w, 401, map[string]interface{}{
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
	if got := atomic.LoadInt32(&addAttempts); got != 1 {
		t.Errorf("addPassword attempts = %d, want 1 (should not retry on 401)", got)
	}
}

func TestEnsureServicePrincipal_AlreadyExists(t *testing.T) {
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"GET /servicePrincipals": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"value": []map[string]string{
					{"id": "sp-123"},
				},
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
}

func TestEnsureServicePrincipal_Creates(t *testing.T) {
	created := false
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"GET /servicePrincipals": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"value": []interface{}{},
			})
		},
		"POST /servicePrincipals": func(w http.ResponseWriter, r *http.Request) {
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

func TestCleanupVenomApp_AppExists(t *testing.T) {
	deleted := false
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"POST /test-tenant/oauth2/v2.0/token": tokenHandler(),
		"GET /applications": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
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
			jsonResponse(w, 200, map[string]interface{}{
				"value": []interface{}{},
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
				jsonResponse(w, 200, map[string]interface{}{
					"value": []interface{}{},
				})
				return
			}
			// getApp for rotateCredentials (fetching password credentials).
			jsonResponse(w, 200, map[string]interface{}{
				"passwordCredentials": []interface{}{},
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
			jsonResponse(w, 200, map[string]interface{}{
				"passwordCredentials": []interface{}{},
			})
		},
		// rotateCredentials: addPassword.
		"POST /applications/new-venom-obj-id/addPassword": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]string{
				"secretText": "generated-secret",
			})
		},
		// ensureServicePrincipal: not found → create.
		"GET /servicePrincipals": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"value": []interface{}{},
			})
		},
		"POST /servicePrincipals": func(w http.ResponseWriter, r *http.Request) {
			spCreated = true
			jsonResponse(w, 201, map[string]string{"id": "sp-id"})
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
				jsonResponse(w, 200, map[string]interface{}{
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
			jsonResponse(w, 200, map[string]interface{}{
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
		"GET /servicePrincipals": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"value": []map[string]string{{"id": "sp-existing"}},
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
				jsonResponse(w, 200, map[string]interface{}{
					"value": []map[string]string{
						{"appId": "skip-secret-app-id", "id": "skip-secret-obj-id"},
					},
				})
				return
			}
			http.NotFound(w, r)
		},
		"GET /applications/skip-secret-obj-id": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"passwordCredentials": []interface{}{},
			})
		},
		"POST /applications/skip-secret-obj-id/addPassword": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]string{"secretText": "skip-secret-value"})
		},
		"GET /servicePrincipals": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"value": []map[string]string{{"id": "sp-existing"}},
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

func TestIsCIDomainURI(t *testing.T) {
	tests := []struct {
		uri  string
		want bool
	}{
		{"https://my-ns.ci.distro.ultrawombat.com/identity/auth/login-callback", true},
		{"https://other.ci.distro.ultrawombat.com/operate/identity-callback", true},
		{"https://production.example.com/callback", false},
		{"https://staging.distribution.aws.camunda.cloud/callback", false},
		{"", false},
	}
	for _, tc := range tests {
		t.Run(tc.uri, func(t *testing.T) {
			if got := isCIDomainURI(tc.uri); got != tc.want {
				t.Errorf("isCIDomainURI(%q) = %v, want %v", tc.uri, got, tc.want)
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
		// Stale CI URI — should be removed.
		"https://old-ns.ci.distro.ultrawombat.com/identity/auth/login-callback",
		"https://old-ns.ci.distro.ultrawombat.com/operate/identity-callback",
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

	result := filterRedirectURIs(existing, newURIs)

	expected := []string{
		"https://production.example.com/callback",
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

func TestFilterRedirectURIs_EmptyExisting(t *testing.T) {
	newURIs := []string{
		"https://ns.ci.distro.ultrawombat.com/callback",
	}

	result := filterRedirectURIs(nil, newURIs)
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

	result := filterRedirectURIs(nil, newURIs)
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
	var patchReceived map[string]interface{}

	srv := newTestServer(t, map[string]http.HandlerFunc{
		"POST /test-tenant/oauth2/v2.0/token": tokenHandler(),
		"GET /applications/parent-obj-id": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"web": map[string]interface{}{
					"redirectUris": []string{
						"https://production.example.com/callback",
						"https://old-ns.ci.distro.ultrawombat.com/identity/auth/login-callback",
						"https://old-ns.ci.distro.ultrawombat.com/operate/identity-callback",
						"https://corrupt.ci.distro.ultrawombat.com/callback,",
					},
				},
				"spa": map[string]interface{}{
					"redirectUris": []string{
						"https://production.example.com/spa-callback",
						"https://old-ns.ci.distro.ultrawombat.com/modeler/login-callback",
					},
				},
			})
		},
		"PATCH /applications/parent-obj-id": func(w http.ResponseWriter, r *http.Request) {
			json.NewDecoder(r.Body).Decode(&patchReceived) //nolint:errcheck
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

	// Verify web URIs: production preserved, stale CI removed, new added.
	webData, ok := patchReceived["web"].(map[string]interface{})
	if !ok {
		t.Fatal("missing 'web' in PATCH payload")
	}
	webURIs, ok := webData["redirectUris"].([]interface{})
	if !ok {
		t.Fatal("missing 'redirectUris' in web payload")
	}

	webStrings := make([]string, len(webURIs))
	for i, u := range webURIs {
		webStrings[i] = u.(string)
	}

	// Should contain production + 5 new web URIs = 6 total.
	// Old CI URIs and corrupt URI should be removed.
	if len(webStrings) != 6 {
		t.Errorf("web URI count = %d, want 6, got: %v", len(webStrings), webStrings)
	}

	// Verify production URI is preserved.
	found := false
	for _, u := range webStrings {
		if u == "https://production.example.com/callback" {
			found = true
			break
		}
	}
	if !found {
		t.Error("production URI was not preserved in web URIs")
	}

	// Verify stale CI URI was removed.
	for _, u := range webStrings {
		if strings.Contains(u, "old-ns.ci.distro.ultrawombat.com") {
			t.Errorf("stale CI URI should have been removed: %s", u)
		}
	}

	// Verify corrupt URI was removed.
	for _, u := range webStrings {
		if strings.HasSuffix(u, ",") {
			t.Errorf("corrupt URI with trailing comma should have been removed: %s", u)
		}
	}

	// Verify SPA URIs.
	spaData, ok := patchReceived["spa"].(map[string]interface{})
	if !ok {
		t.Fatal("missing 'spa' in PATCH payload")
	}
	spaURIs, ok := spaData["redirectUris"].([]interface{})
	if !ok {
		t.Fatal("missing 'redirectUris' in spa payload")
	}

	// Should contain production + 2 new SPA URIs = 3 total.
	if len(spaURIs) != 3 {
		spaStrings := make([]string, len(spaURIs))
		for i, u := range spaURIs {
			spaStrings[i] = u.(string)
		}
		t.Errorf("spa URI count = %d, want 3, got: %v", len(spaURIs), spaStrings)
	}
}

func TestUpdateRedirectURIs_PatchFails(t *testing.T) {
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"POST /test-tenant/oauth2/v2.0/token": tokenHandler(),
		"GET /applications/parent-obj-id": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]interface{}{
				"web": map[string]interface{}{"redirectUris": []string{}},
				"spa": map[string]interface{}{"redirectUris": []string{}},
			})
		},
		"PATCH /applications/parent-obj-id": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 400, map[string]interface{}{
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
	var receivedBody map[string]interface{}
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"PATCH /applications/test-id": func(w http.ResponseWriter, r *http.Request) {
			json.NewDecoder(r.Body).Decode(&receivedBody) //nolint:errcheck
			w.WriteHeader(204)
		},
	})
	defer srv.Close()

	origGraph := graphBaseURL
	graphBaseURL = srv.URL
	defer func() { graphBaseURL = origGraph }()

	payload := map[string]string{"key": "value"}
	_, statusCode, err := graphPatch(context.Background(), srv.Client(), "token", "/applications/test-id", payload)
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
