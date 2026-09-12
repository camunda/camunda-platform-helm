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

package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestHTTPClientRejectsRedirects(t *testing.T) {
	redirectTarget := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("redirect target must not receive credentials")
	}))
	defer redirectTarget.Close()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Redirect(w, &http.Request{}, redirectTarget.URL, http.StatusTemporaryRedirect)
	}))
	defer server.Close()

	req, err := http.NewRequest(http.MethodPost, server.URL, strings.NewReader("client_secret=sensitive"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = do(newHTTPClient(http.DefaultTransport), req)
	if err == nil || !strings.Contains(err.Error(), "HTTP 307") {
		t.Fatalf("expected redirect rejection, got %v", err)
	}
}

func TestAcquireTokenSendsExpectedFormWithoutExposingCredentials(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.Header.Get("Content-Type"))
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		form, err := url.ParseQuery(string(body))
		if err != nil {
			t.Fatal(err)
		}
		if form.Get("client_id") != "id" || form.Get("client_secret") != "secret" || form.Get("scope") != "api/.default" || form.Get("grant_type") != "client_credentials" {
			t.Fatalf("unexpected token form: %v", form)
		}
		_, _ = io.WriteString(w, `{"access_token":"token"}`)
	}))
	defer server.Close()

	encode := func(value string) string { return base64.StdEncoding.EncodeToString([]byte(value)) }
	secret := []byte(`{"data":{"client-id":"` + encode("id") + `","client-secret":"` + encode("secret") + `","audience":"` + encode("api") + `"}}`)
	v := &verifier{
		cfg:        config{release: "integration"},
		httpClient: newHTTPClient(http.DefaultTransport),
		kubectl: func(_ context.Context, _ string, args ...string) ([]byte, error) {
			joined := strings.Join(args, " ")
			if strings.Contains(joined, "get configmap") {
				return []byte(server.URL), nil
			}
			if strings.Contains(joined, "get secret") {
				return secret, nil
			}
			return nil, errors.New("unexpected kubectl call")
		},
	}
	if err := v.acquireToken(context.Background()); err != nil {
		t.Fatal(err)
	}
	if v.token != "token" {
		t.Fatalf("unexpected token %q", v.token)
	}
}

func TestRequestSendsBearerToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer token" {
			t.Fatalf("unexpected authorization header %q", r.Header.Get("Authorization"))
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	v := &verifier{token: "token", httpClient: newHTTPClient(http.DefaultTransport)}
	if err := v.request(context.Background(), http.MethodGet, server.URL, "", nil, nil); err != nil {
		t.Fatal(err)
	}
}

func TestSelectIngress(t *testing.T) {
	raw := []byte(`{"items":[{"spec":{"rules":[{"host":"grpc-camunda.example.com"}]},"status":{"loadBalancer":{"ingress":[{"ip":"1.1.1.1"}]}}},{"spec":{"rules":[{"host":"camunda.example.com"}]},"status":{"loadBalancer":{"ingress":[{"ip":"2.2.2.2"}]}}}]}`)
	host, ip, err := selectIngress(raw)
	if err != nil {
		t.Fatal(err)
	}
	if host != "camunda.example.com" || ip != "2.2.2.2" {
		t.Fatalf("got %s/%s", host, ip)
	}
}

func TestSelectIngressRejectsPendingAddress(t *testing.T) {
	_, _, err := selectIngress([]byte(`{"items":[{"spec":{"rules":[{"host":"camunda.example.com"}]},"status":{"loadBalancer":{"ingress":[]}}}]}`))
	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestAbsenceAssertions(t *testing.T) {
	if err := assertNoItems([]byte(`{"items":[]}`)); err != nil {
		t.Fatal(err)
	}
	if err := assertDataKeyAbsent([]byte(`{"data":{"OTHER":"value"}}`), "CAMUNDA_IDENTITY_BASEURL"); err != nil {
		t.Fatal(err)
	}
	if err := assertNoItems([]byte(`{"items":[{}]}`)); err == nil {
		t.Fatal("expected existing resource error")
	}
	if err := assertDataKeyAbsent([]byte(`{"data":{"CAMUNDA_IDENTITY_BASEURL":"http://identity"}}`), "CAMUNDA_IDENTITY_BASEURL"); err == nil {
		t.Fatal("expected existing key error")
	}
}

func TestParseCredentials(t *testing.T) {
	encode := func(value string) string { return base64.StdEncoding.EncodeToString([]byte(value)) }
	raw := []byte(`{"data":{"client-id":"` + encode("id") + `","client-secret":"` + encode("secret") + `","audience":"` + encode("api") + `"}}`)
	id, secret, audience, err := parseCredentials(raw)
	if err != nil {
		t.Fatal(err)
	}
	if id != "id" || secret != "secret" || audience != "api" {
		t.Fatal("credentials were parsed incorrectly")
	}
}

func TestParseCredentialsErrorDoesNotExposeValues(t *testing.T) {
	raw := []byte(`{"data":{"client-id":"not-base64","client-secret":"sensitive-value","audience":"another-sensitive-value"}}`)
	_, _, _, err := parseCredentials(raw)
	if err == nil {
		t.Fatal("expected an error")
	}
	if strings.Contains(err.Error(), "sensitive") || strings.Contains(err.Error(), "not-base64") {
		t.Fatalf("error exposed credential material: %v", err)
	}
}

func TestSelectLatestDefinition(t *testing.T) {
	raw := []byte(`[{"key":"other","version":"99","tenantId":"x"},{"key":"test","version":"2","tenantId":"tenant"},{"key":"test","version":3,"tenantId":"tenant"}]`)
	definition, err := selectLatestDefinition(raw, "test")
	if err != nil {
		t.Fatal(err)
	}
	if string(definition.Version) != "3" || string(definition.TenantID) != `"tenant"` {
		t.Fatalf("selected wrong definition: %+v", definition)
	}
}

func TestReportDefinitionPreservesJQScalarConversion(t *testing.T) {
	body, err := reportDefinition(processDefinition{Key: "test", Version: json.RawMessage(`2`), TenantID: json.RawMessage(`null`)})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), `"versions":["2"]`) || !strings.Contains(string(body), `"tenantIds":["null"]`) {
		t.Fatalf("unexpected report definition: %s", body)
	}
}

func TestSelectLatestDefinitionErrors(t *testing.T) {
	for _, raw := range []string{`[]`, `[{"key":"test","version":"bad"}]`, `{}`} {
		if _, err := selectLatestDefinition([]byte(raw), "test"); err == nil {
			t.Fatalf("expected error for %s", raw)
		}
	}
}

func TestRetrySucceedsAndBoundsAttempts(t *testing.T) {
	attempts, sleeps := 0, 0
	err := retry(context.Background(), 3, time.Second, func(context.Context, time.Duration) error {
		sleeps++
		return nil
	}, func() error {
		attempts++
		if attempts < 3 {
			return errors.New("temporary")
		}
		return nil
	}, "operation failed")
	if err != nil || attempts != 3 || sleeps != 2 {
		t.Fatalf("err=%v attempts=%d sleeps=%d", err, attempts, sleeps)
	}
}

func TestRetryReturnsLastError(t *testing.T) {
	sentinel := errors.New("last failure")
	attempts := 0
	err := retry(context.Background(), 2, 0, func(context.Context, time.Duration) error { return nil }, func() error {
		attempts++
		return sentinel
	}, "operation failed")
	if !errors.Is(err, sentinel) || attempts != 2 || !strings.Contains(err.Error(), "after 2 attempts") {
		t.Fatalf("err=%v attempts=%d", err, attempts)
	}
}

func TestRetryStopsWhenSleepIsCancelled(t *testing.T) {
	sentinel := errors.New("cancelled")
	attempts := 0
	err := retry(context.Background(), 3, time.Second, func(context.Context, time.Duration) error { return sentinel }, func() error {
		attempts++
		return errors.New("temporary")
	}, "operation failed")
	if !errors.Is(err, sentinel) || attempts != 1 {
		t.Fatalf("err=%v attempts=%d", err, attempts)
	}
}
