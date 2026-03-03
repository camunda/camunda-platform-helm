package entra

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
