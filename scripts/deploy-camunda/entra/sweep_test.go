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
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"
)

// sweepNow is the fixed clock the sweep tests compare ages against.
var sweepNow = time.Date(2026, 11, 27, 12, 0, 0, 0, time.UTC)

// sweepApp builds one Graph application listing entry.
func sweepApp(displayName string, ageHours int) map[string]string {
	return map[string]string{
		"id":              "obj-" + displayName,
		"appId":           "app-" + displayName,
		"displayName":     displayName,
		"createdDateTime": sweepNow.Add(-time.Duration(ageHours) * time.Hour).Format(time.RFC3339),
	}
}

// useTestServer points the Graph and login base URLs at srv for one test.
func useTestServer(t *testing.T, srv *httptest.Server) {
	t.Helper()
	origGraph, origLogin := graphBaseURL, loginBaseURL
	graphBaseURL, loginBaseURL = srv.URL, srv.URL
	t.Cleanup(func() { graphBaseURL, loginBaseURL = origGraph, origLogin })
}

// sweepTestOptions returns options wired to srv with the fixed clock.
func sweepTestOptions(srv *httptest.Server, dryRun bool) SweepOptions {
	creds := Options{
		DirectoryID:  "test-tenant",
		ClientID:     "parent-client",
		ClientSecret: "parent-secret",
		HTTPClient:   srv.Client(),
	}
	return SweepOptions{
		Options: creds,
		MaxAge:  24 * time.Hour,
		DryRun:  dryRun,
		Now:     sweepNow,
	}
}

// names returns the display names of apps.
func names(apps []SweptApp) []string {
	out := make([]string, 0, len(apps))
	for _, app := range apps {
		out = append(out, app.DisplayName)
	}
	return out
}

func TestSweepLoginApps_DeletesOnlyOldQAEntraApps(t *testing.T) {
	listing := []map[string]string{
		sweepApp("camunda-oidc-qa-id-qaen-ce20-8-10-qaen", 48),
		sweepApp("camunda-oidc-qa-id-qaen-ab12-8-11-qaen", 25),
		sweepApp("camunda-oidc-qa-id-qaen-cd34-8-11-qaen", 2),
		sweepApp("camunda-oidc-qa-id-qaen-ef78-8-11-qaen", 24),
		sweepApp("camunda-oidc-qa-id-qaelupg-ef56-8-10", 48),
		sweepApp("camunda-oidc-distro-pr-1234-qaen", 48),
	}

	var deleted []string
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"POST /test-tenant/oauth2/v2.0/token": tokenHandler(),
		"GET /applications": func(w http.ResponseWriter, r *http.Request) {
			filter := r.URL.Query().Get("$filter")
			if !strings.Contains(filter, sweepPrefix) {
				t.Errorf("unexpected filter %q", filter)
			}
			jsonResponse(w, 200, map[string]any{"value": listing})
		},
		"DELETE /applications/": func(w http.ResponseWriter, r *http.Request) {
			deleted = append(deleted, strings.TrimPrefix(r.URL.Path, "/applications/obj-"))
			w.WriteHeader(http.StatusNoContent)
		},
	})
	defer srv.Close()
	useTestServer(t, srv)

	result, err := SweepLoginApps(context.Background(), sweepTestOptions(srv, false))
	if err != nil {
		t.Fatalf("SweepLoginApps: %v", err)
	}

	want := []string{
		"camunda-oidc-qa-id-qaen-ce20-8-10-qaen",
		"camunda-oidc-qa-id-qaen-ab12-8-11-qaen",
	}
	if !slices.Equal(names(result.Candidates), want) {
		t.Errorf("candidates = %v, want %v", names(result.Candidates), want)
	}
	if !slices.Equal(deleted, want) {
		t.Errorf("deleted = %v, want %v", deleted, want)
	}
	if !slices.Equal(names(result.Deleted), want) {
		t.Errorf("result.Deleted = %v, want %v", names(result.Deleted), want)
	}
	if len(result.Failed) != 0 {
		t.Errorf("Failed = %v, want none", names(result.Failed))
	}
	for _, app := range result.Candidates {
		if strings.Contains(app.DisplayName, "-ef78-") {
			t.Error("an app aged exactly --max-age must be kept")
		}
	}
	if result.Scanned != len(listing) {
		t.Errorf("Scanned = %d, want %d", result.Scanned, len(listing))
	}
}

func TestSweepLoginApps_DryRunDeletesNothing(t *testing.T) {
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"POST /test-tenant/oauth2/v2.0/token": tokenHandler(),
		"GET /applications": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]any{
				"value": []map[string]string{sweepApp("camunda-oidc-qa-id-qaen-ce20-8-10-qaen", 48)},
			})
		},
		"DELETE /applications/": func(w http.ResponseWriter, r *http.Request) {
			t.Error("dry run must not delete")
			w.WriteHeader(http.StatusNoContent)
		},
	})
	defer srv.Close()
	useTestServer(t, srv)

	result, err := SweepLoginApps(context.Background(), sweepTestOptions(srv, true))
	if err != nil {
		t.Fatalf("SweepLoginApps: %v", err)
	}
	if len(result.Candidates) != 1 {
		t.Fatalf("Candidates = %v, want one", names(result.Candidates))
	}
	if len(result.Deleted) != 0 || len(result.Failed) != 0 {
		t.Errorf("dry run reported deletions: deleted=%v failed=%v", names(result.Deleted), names(result.Failed))
	}
	if !result.DryRun {
		t.Error("result.DryRun = false, want true")
	}
}

func TestSweepLoginApps_FollowsPaging(t *testing.T) {
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"POST /test-tenant/oauth2/v2.0/token": tokenHandler(),
		"GET /applications": func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("$skiptoken") == "" {
				jsonResponse(w, 200, map[string]any{
					"value":           []map[string]string{sweepApp("camunda-oidc-qa-id-qaen-page1-8-10-qaen", 48)},
					"@odata.nextLink": graphBaseURL + "/applications?$skiptoken=page2",
				})
				return
			}
			jsonResponse(w, 200, map[string]any{
				"value": []map[string]string{sweepApp("camunda-oidc-qa-id-qaen-page2-8-10-qaen", 48)},
			})
		},
		"DELETE /applications/": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		},
	})
	defer srv.Close()
	useTestServer(t, srv)

	result, err := SweepLoginApps(context.Background(), sweepTestOptions(srv, false))
	if err != nil {
		t.Fatalf("SweepLoginApps: %v", err)
	}
	if len(result.Deleted) != 2 {
		t.Errorf("deleted = %v, want both pages", names(result.Deleted))
	}
}

func TestSweepLoginApps_ReportsFailedDeletes(t *testing.T) {
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"POST /test-tenant/oauth2/v2.0/token": tokenHandler(),
		"GET /applications": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]any{
				"value": []map[string]string{
					sweepApp("camunda-oidc-qa-id-qaen-gone-8-10-qaen", 48),
					sweepApp("camunda-oidc-qa-id-qaen-bad-8-10-qaen", 48),
				},
			})
		},
		"DELETE /applications/": func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.URL.Path, "-bad-") {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		},
	})
	defer srv.Close()
	useTestServer(t, srv)

	result, err := SweepLoginApps(context.Background(), sweepTestOptions(srv, false))
	if err != nil {
		t.Fatalf("SweepLoginApps: %v", err)
	}
	if !slices.Equal(names(result.Deleted), []string{"camunda-oidc-qa-id-qaen-gone-8-10-qaen"}) {
		t.Errorf("Deleted = %v, want the 404 to count as deleted", names(result.Deleted))
	}
	if !slices.Equal(names(result.Failed), []string{"camunda-oidc-qa-id-qaen-bad-8-10-qaen"}) {
		t.Errorf("Failed = %v, want the 403", names(result.Failed))
	}
	if result.Failed[0].Error == "" {
		t.Error("failed app carries no error")
	}
}

func TestSweepLoginApps_SkipsUnreadableCreationTime(t *testing.T) {
	broken := sweepApp("camunda-oidc-qa-id-qaen-broken-8-10-qaen", 48)
	broken["createdDateTime"] = "not-a-timestamp"

	srv := newTestServer(t, map[string]http.HandlerFunc{
		"POST /test-tenant/oauth2/v2.0/token": tokenHandler(),
		"GET /applications": func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, map[string]any{"value": []map[string]string{broken}})
		},
		"DELETE /applications/": func(w http.ResponseWriter, r *http.Request) {
			t.Error("app with unreadable creation time must not be deleted")
			w.WriteHeader(http.StatusNoContent)
		},
	})
	defer srv.Close()
	useTestServer(t, srv)

	result, err := SweepLoginApps(context.Background(), sweepTestOptions(srv, false))
	if err != nil {
		t.Fatalf("SweepLoginApps: %v", err)
	}
	if len(result.Candidates) != 0 {
		t.Errorf("Candidates = %v, want none", names(result.Candidates))
	}
}

func TestSweepLoginApps_InvalidOptions(t *testing.T) {
	t.Setenv("ENTRA_APP_DIRECTORY_ID", "")
	t.Setenv("ENTRA_APP_CLIENT_ID", "")
	t.Setenv("ENTRA_APP_CLIENT_SECRET", "")

	if _, err := SweepLoginApps(context.Background(), SweepOptions{MaxAge: 24 * time.Hour}); err == nil {
		t.Error("expected an error without credentials")
	}

	srv := newTestServer(t, map[string]http.HandlerFunc{})
	defer srv.Close()
	for _, maxAge := range []time.Duration{0, minSweepMaxAge - time.Minute} {
		opts := sweepTestOptions(srv, true)
		opts.MaxAge = maxAge
		if _, err := SweepLoginApps(context.Background(), opts); err == nil {
			t.Errorf("max age %s: expected an error", maxAge)
		}
	}
}

func TestSweepLoginApps_ListingError(t *testing.T) {
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"POST /test-tenant/oauth2/v2.0/token": tokenHandler(),
		"GET /applications": func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "boom", http.StatusInternalServerError)
		},
	})
	defer srv.Close()
	useTestServer(t, srv)

	if _, err := SweepLoginApps(context.Background(), sweepTestOptions(srv, true)); err == nil {
		t.Error("expected the listing failure to surface")
	}
}

func TestSweepNamePattern(t *testing.T) {
	cases := map[string]bool{
		"camunda-oidc-qa-id-qaen-ce20-8-10-qaen": true,
		"camunda-oidc-qa-id-qaen-ab12-8-11-qaen": true,
		"camunda-oidc-qa-id-qaelupg-ef56-8-10":   false,
		"camunda-oidc-distro-pr-1234-qaen":       false,
		"camunda-oidc-qa-id-rba-8-10":            false,
		"venom-test-qa-id-qaen-ce20-8-10-qaen":   false,
		"camunda-oidc-production-qaen":           false,
	}
	for name, want := range cases {
		if got := sweepNamePattern.MatchString(name); got != want {
			t.Errorf("%s: match = %v, want %v", name, got, want)
		}
	}
}
