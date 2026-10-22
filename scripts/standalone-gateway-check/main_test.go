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
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const ingressFixture = `{"items":[
 {"metadata":{"name":"integration-camunda-platform-http"},
  "spec":{"rules":[{"host":"ns.example.com","http":{"paths":[
    {"path":"/identity","backend":{"service":{"name":"integration-identity"}}},
    {"path":"/orchestration","backend":{"service":{"name":"integration-zeebe-gateway"}}}]}}]},
  "status":{"loadBalancer":{"ingress":[{"ip":"10.0.0.1"}]}}},
 {"metadata":{"name":"integration-camunda-platform-grpc"},
  "spec":{"rules":[{"host":"grpc-ns.example.com","http":{"paths":[
    {"path":"/","backend":{"service":{"name":"integration-zeebe-standalone-gateway"}}}]}}]},
  "status":{"loadBalancer":{"ingress":[{"ip":"10.0.0.1"}]}}},
 {"metadata":{"name":"integration-camunda-platform-standalone-gateway-rest"},
  "spec":{"rules":[{"host":"api-ns.example.com","http":{"paths":[
    {"path":"/","backend":{"service":{"name":"integration-zeebe-standalone-gateway"}}}]}}]},
  "status":{"loadBalancer":{"ingress":[{"ip":"10.0.0.2"}]}}}
]}`

func parseIngresses(t *testing.T, raw string) ingressList {
	t.Helper()
	var list ingressList
	if err := json.Unmarshal([]byte(raw), &list); err != nil {
		t.Fatal(err)
	}
	return list
}

func TestSelectEndpoints(t *testing.T) {
	web, rest, err := selectEndpoints(parseIngresses(t, ingressFixture), "integration-camunda-platform", "integration-zeebe-gateway", "integration-zeebe-standalone-gateway")
	if err != nil {
		t.Fatal(err)
	}
	if web != (endpoint{host: "ns.example.com", ip: "10.0.0.1", contextPath: "/orchestration"}) {
		t.Fatalf("unexpected web endpoint %+v", web)
	}
	if rest != (endpoint{host: "api-ns.example.com", ip: "10.0.0.2"}) {
		t.Fatalf("unexpected REST endpoint %+v", rest)
	}
}

func TestSelectEndpointsRejectsGRPCIngressOnBrokers(t *testing.T) {
	raw := strings.Replace(ingressFixture,
		`"grpc-ns.example.com","http":{"paths":[
    {"path":"/","backend":{"service":{"name":"integration-zeebe-standalone-gateway"}}}`,
		`"grpc-ns.example.com","http":{"paths":[
    {"path":"/","backend":{"service":{"name":"integration-zeebe-gateway"}}}`, 1)
	_, _, err := selectEndpoints(parseIngresses(t, raw), "integration-camunda-platform", "integration-zeebe-gateway", "integration-zeebe-standalone-gateway")
	if err == nil || !strings.Contains(err.Error(), "gRPC ingress must target") {
		t.Fatalf("expected gRPC backend error, got %v", err)
	}
}

func TestSelectEndpointsRejectsWebIngressOnGateway(t *testing.T) {
	raw := strings.Replace(ingressFixture,
		`{"path":"/orchestration","backend":{"service":{"name":"integration-zeebe-gateway"}}}`,
		`{"path":"/orchestration","backend":{"service":{"name":"integration-zeebe-standalone-gateway"}}}`, 1)
	_, _, err := selectEndpoints(parseIngresses(t, raw), "integration-camunda-platform", "integration-zeebe-gateway", "integration-zeebe-standalone-gateway")
	if err == nil || !strings.Contains(err.Error(), "must not target the standalone gateway") {
		t.Fatalf("expected web ingress error, got %v", err)
	}
}

func TestSelectEndpointsRequiresRESTIngress(t *testing.T) {
	raw := strings.Replace(ingressFixture, "integration-camunda-platform-standalone-gateway-rest", "other", 1)
	_, _, err := selectEndpoints(parseIngresses(t, raw), "integration-camunda-platform", "integration-zeebe-gateway", "integration-zeebe-standalone-gateway")
	if err == nil || !strings.Contains(err.Error(), "standalone-gateway-rest not found") {
		t.Fatalf("expected missing REST ingress error, got %v", err)
	}
}

func TestParseForwardedPort(t *testing.T) {
	port, ok := parseForwardedPort("Forwarding from 127.0.0.1:41234 -> 26500")
	if !ok || port != 41234 {
		t.Fatalf("got %d %v", port, ok)
	}
	if _, ok := parseForwardedPort("Forwarding from [::1]:41234 -> 26500"); ok {
		t.Fatal("IPv6 line must not match")
	}
}

func TestParseCount(t *testing.T) {
	for raw, expected := range map[string]int{"": 0, "3\n": 3, " 4 ": 4} {
		count, err := parseCount([]byte(raw))
		if err != nil || count != expected {
			t.Fatalf("parseCount(%q) = %d, %v", raw, count, err)
		}
	}
	if _, err := parseCount([]byte("x")); err == nil {
		t.Fatal("expected error")
	}
}

func TestProcessInstanceKey(t *testing.T) {
	for _, raw := range []string{`{"processInstanceKey":"2251799813685249"}`, `{"processInstanceKey": 2251799813685249, "bpmnProcessId": "x"}`} {
		key, err := processInstanceKey([]byte(raw))
		if err != nil || key != "2251799813685249" {
			t.Fatalf("processInstanceKey(%s) = %q, %v", raw, key, err)
		}
	}
	if _, err := processInstanceKey([]byte(`{}`)); err == nil {
		t.Fatal("expected error for missing key")
	}
}

func TestAssertBrokerCount(t *testing.T) {
	raw := []byte(`{"brokers":[{"nodeId":0}],"clusterSize":1}`)
	if err := assertBrokerCount(raw, 1); err != nil {
		t.Fatal(err)
	}
	if err := assertBrokerCount(raw, 3); err == nil {
		t.Fatal("expected mismatch error")
	}
}

func TestRetry(t *testing.T) {
	calls := 0
	noSleep := func(context.Context, time.Duration) error { return nil }
	err := retry(context.Background(), 3, time.Millisecond, noSleep, func() error {
		calls++
		if calls < 3 {
			return errors.New("not yet")
		}
		return nil
	}, "action")
	if err != nil || calls != 3 {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
	err = retry(context.Background(), 2, time.Millisecond, noSleep, func() error { return errors.New("boom") }, "action")
	if err == nil || !strings.Contains(err.Error(), "action failed after 2 attempts: boom") {
		t.Fatalf("unexpected error %v", err)
	}
}

func TestVerifyREST(t *testing.T) {
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		if r.Header.Get("Authorization") != "Bearer token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.Method + " " + r.URL.Path {
		case "GET /orchestration/v2/topology":
			_, _ = io.WriteString(w, `{"brokers":[{"nodeId":0}]}`)
		case "POST /orchestration/v2/deployments":
			if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = io.WriteString(w, `{"deployments":[]}`)
		case "POST /orchestration/v2/process-instances":
			body, _ := io.ReadAll(r.Body)
			if !strings.Contains(string(body), `"processDefinitionId":"it-test-process"`) {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = io.WriteString(w, `{"processInstanceKey":"42"}`)
		case "POST /orchestration/v2/process-instances/search":
			body, _ := io.ReadAll(r.Body)
			if !strings.Contains(string(body), `"processInstanceKey":"42"`) {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = io.WriteString(w, `{"items":[{"processInstanceKey":"42"}]}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	bpmn := filepath.Join(t.TempDir(), "process.bpmn")
	if err := os.WriteFile(bpmn, []byte("<bpmn/>"), 0o600); err != nil {
		t.Fatal(err)
	}
	c := &checker{
		cfg:         config{bpmnFile: bpmn, processID: "it-test-process"},
		httpClient:  server.Client(),
		sleep:       func(context.Context, time.Duration) error { return nil },
		token:       "token",
		restBaseURL: server.URL + "/orchestration/v2",
	}
	if err := c.verifyREST(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	expected := []string{
		"GET /orchestration/v2/topology",
		"POST /orchestration/v2/deployments",
		"POST /orchestration/v2/process-instances",
		"POST /orchestration/v2/process-instances/search",
	}
	if strings.Join(requests, ",") != strings.Join(expected, ",") {
		t.Fatalf("unexpected requests %v", requests)
	}
}

func TestLoadClientSecret(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte("s3cret"))
	var gotArgs []string
	c := &checker{
		cfg: config{namespace: "ns", context: "ctx"},
		kubectl: func(_ context.Context, _ string, args ...string) ([]byte, error) {
			gotArgs = args
			return []byte(encoded), nil
		},
	}
	if err := c.loadClientSecret(context.Background()); err != nil {
		t.Fatal(err)
	}
	if c.clientSecret != "s3cret" {
		t.Fatalf("got %q", c.clientSecret)
	}
	if strings.Join(gotArgs, " ") != "--context ctx -n ns get secret integration-test-credentials -o jsonpath={.data.identity-admin-client-password}" {
		t.Fatalf("unexpected kubectl args %v", gotArgs)
	}
}

func TestReadyReplicas(t *testing.T) {
	c := &checker{
		cfg: config{namespace: "ns", release: "integration"},
		kubectl: func(_ context.Context, _ string, args ...string) ([]byte, error) {
			switch args[3] {
			case "statefulset/integration-zeebe":
				return []byte("1"), nil
			case "deployment/integration-zeebe-standalone-gateway":
				return []byte("3"), nil
			}
			return nil, errors.New("unexpected resource " + args[3])
		},
	}
	brokers, gateways, err := c.readyReplicas(context.Background())
	if err != nil || brokers != 1 || gateways != 3 {
		t.Fatalf("brokers=%d gateways=%d err=%v", brokers, gateways, err)
	}
}
