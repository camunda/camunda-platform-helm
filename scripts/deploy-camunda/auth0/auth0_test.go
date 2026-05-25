package auth0

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// newTestServer creates an httptest server that handles Auth0 endpoints. The
// handlers map is keyed by "METHOD /path" with prefix matching for paths
// containing dynamic segments (e.g., "DELETE /api/v2/clients/").
func newTestServer(t *testing.T, handlers map[string]http.HandlerFunc) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Method + " " + r.URL.Path
		if h, ok := handlers[key]; ok {
			h(w, r)
			return
		}
		// Prefix match for dynamic segments.
		for k, h := range handlers {
			parts := strings.SplitN(k, " ", 2)
			if len(parts) == 2 && r.Method == parts[0] && strings.HasPrefix(r.URL.Path, parts[1]) {
				h(w, r)
				return
			}
		}
		t.Logf("unhandled %s %s", r.Method, r.URL.Path)
		http.NotFound(w, r)
	}))
}

func tokenHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "test-mgmt-token"})
	}
}

// stubCreateClientHandler returns a handler that echoes back a synthetic
// client_id and client_secret. It records every received name so the test
// can assert kind-specific configuration was sent.
func stubCreateClientHandler(t *testing.T, received *[]map[string]interface{}) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		*received = append(*received, body)
		name, _ := body["name"].(string)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"client_id":     "id-" + name,
			"client_secret": "secret-" + name,
		})
	}
}

func TestIsAuth0Identity(t *testing.T) {
	cases := []struct {
		identity string
		want     bool
	}{
		{"auth0", true},
		{"oidc", false},
		{"keycloak", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := IsAuth0Identity(tc.identity); got != tc.want {
			t.Errorf("IsAuth0Identity(%q) = %v, want %v", tc.identity, got, tc.want)
		}
	}
}

func TestResolveOpts_RequiresMgmtCreds(t *testing.T) {
	t.Setenv("AUTH0_MGMT_TOKEN", "")
	t.Setenv("AUTH0_MGMT_CLIENT_ID", "")
	t.Setenv("AUTH0_MGMT_CLIENT_SECRET", "")
	opts := Options{Namespace: "ns", IngressHost: "host"}
	err := resolveOpts(&opts, true)
	if err == nil {
		t.Fatal("expected error when no mgmt creds set")
	}
	if !strings.Contains(err.Error(), "AUTH0_MGMT_TOKEN") {
		t.Errorf("expected error to mention AUTH0_MGMT_TOKEN, got: %v", err)
	}
}

func TestResolveOpts_DefaultsApplied(t *testing.T) {
	t.Setenv("AUTH0_MGMT_TOKEN", "x")
	opts := Options{Namespace: "ns", IngressHost: "host"}
	if err := resolveOpts(&opts, true); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if opts.Audience != DefaultAudience {
		t.Errorf("Audience default not applied: %q", opts.Audience)
	}
	if opts.SecretName != DefaultSecretName {
		t.Errorf("SecretName default not applied: %q", opts.SecretName)
	}
	if opts.Domain == "" {
		t.Errorf("Domain default not applied")
	}
}

func TestResolveOpts_IngressHostRequiredForEnsureButNotCleanup(t *testing.T) {
	t.Setenv("AUTH0_MGMT_TOKEN", "x")

	ensureOpts := Options{Namespace: "ns"}
	if err := resolveOpts(&ensureOpts, true); err == nil {
		t.Fatal("expected ensure-path resolveOpts to require IngressHost, got nil")
	} else if !strings.Contains(err.Error(), "ingress host") {
		t.Errorf("expected error to mention ingress host, got: %v", err)
	}

	cleanupOpts := Options{Namespace: "ns"}
	if err := resolveOpts(&cleanupOpts, false); err != nil {
		t.Fatalf("cleanup-path resolveOpts must not require IngressHost, got: %v", err)
	}
}

func TestParseRetryAfter(t *testing.T) {
	now := time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name   string
		header string
		want   time.Duration
	}{
		{"empty", "", 0},
		{"whitespace", "   ", 0},
		{"seconds", "30", 30 * time.Second},
		{"seconds-with-padding", "  90  ", 90 * time.Second},
		{"zero-seconds", "0", 0},
		{"negative-seconds", "-5", 0},
		{"http-date-future", now.Add(45 * time.Second).UTC().Format(http.TimeFormat), 45 * time.Second},
		{"http-date-past", now.Add(-1 * time.Minute).UTC().Format(http.TimeFormat), 0},
		{"garbage", "soon", 0},
	}
	for _, tc := range cases {
		got := parseRetryAfter(tc.header, now)
		// http-date round-trip drops sub-second precision; allow ±1s slack.
		diff := got - tc.want
		if diff < -time.Second || diff > time.Second {
			t.Errorf("%s: parseRetryAfter(%q) = %v, want ~%v", tc.name, tc.header, got, tc.want)
		}
	}
}

func TestRedirectURIs(t *testing.T) {
	host := "test.example.com"
	cases := []struct {
		component    string
		wantCount    int
		wantContains string
	}{
		{ComponentIdentity, 1, "/identity/auth/login-callback"},
		{ComponentOrchestration, 3, "/orchestration/sso-callback"},
		{ComponentOptimize, 1, "/optimize/api/authentication/callback"},
		{ComponentConnectors, 0, ""},
		{ComponentWebModeler, 2, "/modeler/login-callback"},
		{ComponentConsole, 1, "/"},
	}
	for _, tc := range cases {
		got := redirectURIs(tc.component, host)
		if len(got) != tc.wantCount {
			t.Errorf("%s: got %d URIs, want %d (%v)", tc.component, len(got), tc.wantCount, got)
			continue
		}
		if tc.wantContains != "" {
			found := false
			for _, u := range got {
				if strings.Contains(u, tc.wantContains) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("%s: expected URI containing %q, got %v", tc.component, tc.wantContains, got)
			}
		}
	}
}

func TestEnsureClients_FullFlow(t *testing.T) {
	var (
		createReceived []map[string]interface{}
		grantReceived  []map[string]interface{}
	)
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"POST /oauth/token":    tokenHandler(),
		"POST /api/v2/clients": stubCreateClientHandler(t, &createReceived),
		"POST /api/v2/client-grants": func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			grantReceived = append(grantReceived, body)
			w.WriteHeader(http.StatusCreated)
		},
	})
	defer srv.Close()

	opts := Options{
		Namespace:        "test-ns",
		IngressHost:      "host.example.com",
		Domain:           srv.URL,
		MgmtClientID:     "mgmt-id",
		MgmtClientSecret: "mgmt-secret",
		HTTPClient:       srv.Client(),
		SkipK8sSecret:    true,
	}

	prov, err := EnsureClients(context.Background(), opts)
	if err != nil {
		t.Fatalf("EnsureClients: %v", err)
	}

	// 4 private + 2 public = 6 client creations.
	if len(createReceived) != 6 {
		t.Errorf("got %d create requests, want 6", len(createReceived))
	}
	// Audience grants only for the 4 private clients.
	if len(grantReceived) != 4 {
		t.Errorf("got %d grant requests, want 4", len(grantReceived))
	}
	if len(prov.Private) != 4 || len(prov.Public) != 2 {
		t.Errorf("Provisioned: got Private=%d Public=%d, want 4/2", len(prov.Private), len(prov.Public))
	}

	// Spot-check the connectors client got non_interactive + client_credentials only.
	for _, body := range createReceived {
		if body["name"] == "test-ns-connectors" {
			if got := body["app_type"]; got != "non_interactive" {
				t.Errorf("connectors app_type = %v, want non_interactive", got)
			}
			gt, _ := body["grant_types"].([]interface{})
			if len(gt) != 1 || gt[0] != "client_credentials" {
				t.Errorf("connectors grant_types = %v, want [client_credentials]", gt)
			}
			if _, hasCallbacks := body["callbacks"]; hasCallbacks {
				t.Errorf("connectors should not have callbacks (M2M)")
			}
		}
		if body["name"] == "test-ns-Web Modeler" {
			if got := body["app_type"]; got != "spa" {
				t.Errorf("Web Modeler app_type = %v, want spa", got)
			}
			if got := body["token_endpoint_auth_method"]; got != "none" {
				t.Errorf("Web Modeler token_endpoint_auth_method = %v, want none", got)
			}
		}
	}

	// All grants target the configured audience.
	for _, g := range grantReceived {
		if g["audience"] != DefaultAudience {
			t.Errorf("grant audience = %v, want %v", g["audience"], DefaultAudience)
		}
	}
}

func TestEnsureClients_RetriesOn429(t *testing.T) {
	// Shrink retry timings so the test runs in milliseconds.
	origBase, origMax := retryBaseBackoff, retryMaxBackoff
	retryBaseBackoff = 1 * time.Millisecond
	retryMaxBackoff = 4 * time.Millisecond
	t.Cleanup(func() { retryBaseBackoff, retryMaxBackoff = origBase, origMax })

	// First two POST /api/v2/clients calls return 429, third succeeds.
	// Subsequent calls always succeed (so we get all six clients created).
	var clientCalls int32
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"POST /oauth/token": tokenHandler(),
		"POST /api/v2/clients": func(w http.ResponseWriter, r *http.Request) {
			n := atomic.AddInt32(&clientCalls, 1)
			if n <= 2 {
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"statusCode":429,"error":"Too Many Requests"}`))
				return
			}
			var body map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			name, _ := body["name"].(string)
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"client_id":     "id-" + name,
				"client_secret": "secret-" + name,
			})
		},
		"POST /api/v2/client-grants": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusCreated)
		},
	})
	defer srv.Close()

	opts := Options{
		Namespace:        "ns",
		IngressHost:      "h",
		Domain:           srv.URL,
		MgmtClientID:     "x",
		MgmtClientSecret: "y",
		HTTPClient:       srv.Client(),
		SkipK8sSecret:    true,
	}
	prov, err := EnsureClients(context.Background(), opts)
	if err != nil {
		t.Fatalf("EnsureClients with retry: %v", err)
	}
	if got := atomic.LoadInt32(&clientCalls); got < 3 {
		t.Errorf("expected at least 3 POST /clients calls (2 failed + 1 succeeded), got %d", got)
	}
	if len(prov.Private)+len(prov.Public) != 6 {
		t.Errorf("expected 6 provisioned clients, got %d", len(prov.Private)+len(prov.Public))
	}
}

func TestCleanupClients_SinglePageList(t *testing.T) {
	// Shrink retry timings.
	origBase, origMax := retryBaseBackoff, retryMaxBackoff
	retryBaseBackoff = 1 * time.Millisecond
	retryMaxBackoff = 4 * time.Millisecond
	t.Cleanup(func() { retryBaseBackoff, retryMaxBackoff = origBase, origMax })

	// The new cleanup must do exactly ONE GET /api/v2/clients call (paged
	// fetch terminates when a page is shorter than perPage).
	var listCalls int32
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"POST /oauth/token": tokenHandler(),
		"GET /api/v2/clients": func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&listCalls, 1)
			_ = json.NewEncoder(w).Encode([]map[string]string{
				{"client_id": "id-A", "name": "ns-identity"},
				{"client_id": "id-B", "name": "ns-orchestration"},
			})
		},
		"DELETE /api/v2/clients/": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		},
	})
	defer srv.Close()

	opts := Options{
		Namespace:        "ns",
		IngressHost:      "irrelevant",
		Domain:           srv.URL,
		MgmtClientID:     "x",
		MgmtClientSecret: "y",
		HTTPClient:       srv.Client(),
	}
	CleanupClients(context.Background(), opts)

	if got := atomic.LoadInt32(&listCalls); got != 1 {
		t.Errorf("expected exactly 1 GET /api/v2/clients call, got %d", got)
	}
}

func TestCleanupClients_DeletesByName(t *testing.T) {
	// Shrink retry timings.
	origBase, origMax := retryBaseBackoff, retryMaxBackoff
	retryBaseBackoff = 1 * time.Millisecond
	retryMaxBackoff = 4 * time.Millisecond
	t.Cleanup(func() { retryBaseBackoff, retryMaxBackoff = origBase, origMax })

	deleted := map[string]bool{}
	listed := []map[string]string{
		{"client_id": "id-A", "name": "ns1-identity"},
		{"client_id": "id-B", "name": "ns1-orchestration"},
		// Duplicate-named clients must ALL be cleaned up. Auth0 does not
		// enforce name uniqueness; a previous map-keyed implementation
		// silently lost everything but the first.
		{"client_id": "id-A2", "name": "ns1-identity"},
		{"client_id": "id-X", "name": "other-ns-orchestration"}, // must not be touched
	}
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"POST /oauth/token": tokenHandler(),
		"GET /api/v2/clients": func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(listed)
		},
		"DELETE /api/v2/clients/": func(w http.ResponseWriter, r *http.Request) {
			id := strings.TrimPrefix(r.URL.Path, "/api/v2/clients/")
			deleted[id] = true
			w.WriteHeader(http.StatusNoContent)
		},
	})
	defer srv.Close()

	opts := Options{
		Namespace:        "ns1",
		IngressHost:      "irrelevant",
		Domain:           srv.URL,
		MgmtClientID:     "x",
		MgmtClientSecret: "y",
		HTTPClient:       srv.Client(),
	}

	CleanupClients(context.Background(), opts)

	if !deleted["id-A"] || !deleted["id-B"] || !deleted["id-A2"] {
		t.Errorf("expected id-A, id-B, AND duplicate id-A2 to be deleted, got %v", deleted)
	}
	if deleted["id-X"] {
		t.Error("must not delete clients from other namespaces")
	}
}
