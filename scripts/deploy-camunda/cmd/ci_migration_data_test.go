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

package cmd

import (
	"context"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRequestMigrationToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if r.FormValue("client_id") != "test" || r.FormValue("client_secret") != "secret" {
			t.Errorf("unexpected token form: %v", r.Form)
		}
		_, _ = io.WriteString(w, `{"access_token":"token"}`)
	}))
	defer server.Close()

	token, err := requestMigrationToken(context.Background(), server.URL, "secret")
	if err != nil {
		t.Fatal(err)
	}
	if token != "token" {
		t.Fatalf("token = %q, want token", token)
	}
}

func TestRetryMigrationRequestRetriesUntilSuccess(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if r.Header.Get("Authorization") != "Bearer token" {
			t.Errorf("authorization = %q", r.Header.Get("Authorization"))
		}
		if attempts < 2 {
			http.Error(w, "not ready", http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	if _, err := retryMigrationRequest(context.Background(), server.URL, "token", "application/json", []byte(`{}`), 2, 0, true); err != nil {
		t.Fatal(err)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
}

func TestRetryMigrationRequestDoesNotRetryPermanentFailure(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		http.Error(w, "forbidden", http.StatusForbidden)
	}))
	defer server.Close()

	if _, err := retryMigrationRequest(context.Background(), server.URL, "token", "application/json", nil, 5, 0, false); err == nil {
		t.Fatal("expected permanent failure")
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
}

func TestWaitForMigrationAdminRoleRefreshesTokenAfterAssignment(t *testing.T) {
	tokenRequests := 0
	roleRequests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			tokenRequests++
			_, _ = io.WriteString(w, `{"access_token":"token-`+string(rune('0'+tokenRequests))+`"}`)
		case "/orchestration/v2/roles/admin/clients/test":
			roleRequests++
			if r.Method != http.MethodPut || r.Header.Get("Authorization") != "Bearer token-1" {
				t.Errorf("role request = %s %q", r.Method, r.Header.Get("Authorization"))
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	token, err := waitForMigrationAdminRole(context.Background(), server.URL+"/orchestration", server.URL+"/token", "secret", 1, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if token != "token-2" || tokenRequests != 2 || roleRequests != 1 {
		t.Fatalf("token = %q, token requests = %d, role requests = %d", token, tokenRequests, roleRequests)
	}
}

func TestWaitForMigrationAdminRoleRefreshesTokenForExistingMembership(t *testing.T) {
	tokenRequests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			tokenRequests++
			_, _ = io.WriteString(w, `{"access_token":"token-`+string(rune('0'+tokenRequests))+`"}`)
			return
		}
		if r.Header.Get("Authorization") != "Bearer token-1" {
			t.Errorf("authorization = %q", r.Header.Get("Authorization"))
		}
		w.WriteHeader(http.StatusConflict)
	}))
	defer server.Close()

	propagationDelay := 20 * time.Millisecond
	started := time.Now()
	token, err := waitForMigrationAdminRole(context.Background(), server.URL+"/orchestration", server.URL+"/token", "secret", 1, 0, propagationDelay)
	if err != nil {
		t.Fatal(err)
	}
	if token != "token-2" || tokenRequests != 2 {
		t.Fatalf("token = %q, token requests = %d, want fresh token", token, tokenRequests)
	}
	if elapsed := time.Since(started); elapsed < propagationDelay {
		t.Fatalf("elapsed = %v, want at least propagation delay %v", elapsed, propagationDelay)
	}
}

func TestReadMigrationEnv(t *testing.T) {
	values, err := parseMigrationEnv(strings.NewReader("BASE_URL=https://example.test\nTOKEN=a=b=c\nIGNORED\n"))
	if err != nil {
		t.Fatal(err)
	}
	if values["TOKEN"] != "a=b=c" {
		t.Fatalf("TOKEN = %q", values["TOKEN"])
	}
}

func TestMigrationMultipartBody(t *testing.T) {
	body, contentType, err := migrationMultipartBody("process.bpmn", []byte("<bpmn/>"))
	if err != nil {
		t.Fatal(err)
	}
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		t.Fatal(err)
	}
	reader := multipart.NewReader(strings.NewReader(string(body)), params["boundary"])
	part, err := reader.NextPart()
	if err != nil {
		t.Fatal(err)
	}
	if part.FormName() != "resources" || part.FileName() != "process.bpmn" {
		t.Fatalf("multipart part = %q/%q", part.FormName(), part.FileName())
	}
}
