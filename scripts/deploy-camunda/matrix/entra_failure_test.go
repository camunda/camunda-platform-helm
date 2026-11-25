// Copyright 2025 Camunda Services GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package matrix

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// fakeEntraGraph serves the Graph and token endpoints the qa-entra provisioning
// calls. The venom app already exists; the login app exists when loginExists
// is set, else POST /applications creates it. Configuring the login app fails
// with 403.
type fakeEntraGraph struct {
	mu          sync.Mutex
	loginExists bool
	deleted     []string
}

const (
	fakeVenomObjectID = "venom-obj-id"
	fakeLoginObjectID = "login-obj-id"
)

func (f *fakeEntraGraph) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	path := strings.TrimPrefix(r.URL.Path, "/v1.0")
	switch {
	case strings.HasSuffix(path, "/oauth2/v2.0/token"):
		writeJSON(w, http.StatusOK, map[string]string{"access_token": "graph-token"})
	case r.Method == http.MethodGet && path == "/applications":
		filter := r.URL.Query().Get("$filter")
		var apps []map[string]string
		switch {
		case strings.Contains(filter, "'venom-test-"):
			apps = append(apps, map[string]string{"appId": "venom-app-id", "id": fakeVenomObjectID})
		case strings.Contains(filter, "'camunda-oidc-") && f.loginExists:
			apps = append(apps, map[string]string{"appId": "login-app-id", "id": fakeLoginObjectID})
		}
		writeJSON(w, http.StatusOK, map[string]any{"value": apps})
	case r.Method == http.MethodPost && path == "/applications":
		f.loginExists = true
		writeJSON(w, http.StatusCreated, map[string]string{"appId": "login-app-id", "id": fakeLoginObjectID})
	case r.Method == http.MethodGet && path == "/applications/"+fakeVenomObjectID:
		writeJSON(w, http.StatusOK, map[string]any{"passwordCredentials": []any{}})
	case r.Method == http.MethodPost && path == "/applications/"+fakeVenomObjectID+"/addPassword":
		writeJSON(w, http.StatusOK, map[string]string{"secretText": "venom-secret"})
	case r.Method == http.MethodPatch && strings.HasPrefix(path, "/servicePrincipals("):
		w.WriteHeader(http.StatusNoContent)
	case r.Method == http.MethodPatch && path == "/applications/"+fakeLoginObjectID:
		writeJSON(w, http.StatusForbidden, map[string]any{"error": map[string]string{"code": "Authorization_RequestDenied"}})
	case r.Method == http.MethodDelete && strings.HasPrefix(path, "/applications/"):
		f.deleted = append(f.deleted, strings.TrimPrefix(path, "/applications/"))
		w.WriteHeader(http.StatusNoContent)
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": r.Method + " " + path})
	}
}

func (f *fakeEntraGraph) deletedObjects() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.deleted...)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// redirectTransport sends every request to target, keeping its path and query.
type redirectTransport struct {
	target *url.URL
	next   http.RoundTripper
}

func (t redirectTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.URL.Scheme, r.URL.Host, r.Host = t.target.Scheme, t.target.Host, t.target.Host
	return t.next.RoundTrip(r)
}

// routeDefaultTransport points http.DefaultTransport, which the entra package
// uses when Options.HTTPClient is nil, at handler for the rest of the test.
func routeDefaultTransport(t *testing.T, handler http.Handler) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	target, err := url.Parse(srv.URL)
	require.NoError(t, err)
	orig := http.DefaultTransport
	http.DefaultTransport = redirectTransport{target: target, next: srv.Client().Transport}
	t.Cleanup(func() { http.DefaultTransport = orig })
}

func TestExecuteEntryCleansUpEntraAppsWhenLoginAppFails(t *testing.T) {
	for _, tc := range []struct {
		name        string
		loginExists bool
		wantDeleted []string
	}{
		{name: "new login app", loginExists: false, wantDeleted: []string{fakeVenomObjectID, fakeLoginObjectID}},
		{name: "reused login app", loginExists: true, wantDeleted: []string{fakeVenomObjectID}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			graph := &fakeEntraGraph{loginExists: tc.loginExists}
			routeDefaultTransport(t, graph)
			t.Setenv("ENTRA_APP_DIRECTORY_ID", "test-tenant")
			t.Setenv("ENTRA_APP_CLIENT_ID", "parent-app-id")
			t.Setenv("ENTRA_APP_CLIENT_SECRET", "parent-secret")
			t.Setenv("CAMUNDA_HOSTNAME", "test-ns.example.com")

			result := executeEntry(context.Background(), Entry{Version: "8.10", Identity: qaEntraIdentity, Auth: "oidc"}, RunOptions{
				NamespaceOverride: "test-ns",
			})

			require.ErrorContains(t, result.Error, "entra: provision login app")
			require.Equal(t, tc.wantDeleted, graph.deletedObjects())
		})
	}
}
