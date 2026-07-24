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
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"scripts/deploy-camunda/config"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// fakeResolver returns a canned result for every LookupHost call, tracking how
// many times it was called so tests can drive a transient-then-success case.
type fakeResolver struct {
	calls int
	// failFor is the number of leading calls that return NXDOMAIN before
	// succeeding. 0 means every call succeeds.
	failFor int
}

func (f *fakeResolver) LookupHost(_ context.Context, host string) ([]string, error) {
	f.calls++
	if f.failFor > 0 && f.calls <= f.failFor {
		return nil, &net.DNSError{Err: "no such host", Name: host, IsNotFound: true}
	}
	return []string{"203.0.113.10"}, nil
}

// alwaysFailResolver always returns NXDOMAIN, for the timeout-path case.
type alwaysFailResolver struct{}

func (alwaysFailResolver) LookupHost(_ context.Context, host string) ([]string, error) {
	return nil, &net.DNSError{Err: "no such host", Name: host, IsNotFound: true}
}

// noSleep replaces the interval wait with a no-op so tests run instantly.
func noSleep(ctx context.Context, _ time.Duration) error {
	return ctx.Err()
}

func TestResolveIngressReadyHostWith(t *testing.T) {
	t.Run("explicit ingress hostname remains the full override", func(t *testing.T) {
		flags := &config.RuntimeFlags{
			Ingress: config.IngressFlags{IngressHostname: "override.example.com"},
		}
		scenarioCtx := &ScenarioContext{Namespace: "test", Release: "integration", IngressHost: "computed.example.com"}
		lookupCalled := false

		got := resolveIngressReadyHostWith(
			context.Background(),
			flags,
			scenarioCtx,
			func(string) string { return "env.example.com" },
			func(context.Context, string, string, string) string {
				lookupCalled = true
				return "rendered.example.com"
			},
		)

		if got != "override.example.com" {
			t.Fatalf("resolveIngressReadyHostWith() = %q, want explicit override", got)
		}
		if lookupCalled {
			t.Fatal("cluster lookup should not run when --ingress-hostname is set")
		}
	})

	t.Run("rendered routing host wins over raw Helm and computed hosts", func(t *testing.T) {
		flags := &config.RuntimeFlags{
			Deployment: config.DeploymentFlags{
				ExtraHelmSets: map[string]string{"global.host": "stale.example.com"},
			},
		}
		scenarioCtx := &ScenarioContext{Namespace: "test", Release: "integration", IngressHost: "computed.example.com"}
		lookupCalled := false

		got := resolveIngressReadyHostWith(
			context.Background(),
			flags,
			scenarioCtx,
			func(string) string { return "env.example.com" },
			func(_ context.Context, _, _, release string) string {
				lookupCalled = true
				if release != "integration" {
					t.Fatalf("lookup called with release %q", release)
				}
				return "rendered.example.com"
			},
		)

		if got != "rendered.example.com" {
			t.Fatalf("resolveIngressReadyHostWith() = %q, want rendered host", got)
		}
		if !lookupCalled {
			t.Fatal("cluster lookup should determine the effective rendered host")
		}
	})

	t.Run("deployed routing host wins over computed and environment hosts", func(t *testing.T) {
		flags := &config.RuntimeFlags{Test: config.TestFlags{KubeContext: "cluster"}}
		scenarioCtx := &ScenarioContext{Namespace: "test", Release: "integration", IngressHost: "computed.example.com"}

		got := resolveIngressReadyHostWith(
			context.Background(),
			flags,
			scenarioCtx,
			func(string) string { return "env.example.com" },
			func(_ context.Context, kubeContext, namespace, release string) string {
				if kubeContext != "cluster" || namespace != "test" {
					t.Fatalf("lookup called with context=%q namespace=%q", kubeContext, namespace)
				}
				if release != "integration" {
					t.Fatalf("lookup called with release=%q", release)
				}
				return "gateway.example.com"
			},
		)

		if got != "gateway.example.com" {
			t.Fatalf("resolveIngressReadyHostWith() = %q, want deployed routing host", got)
		}
	})

	t.Run("templated global host defers to the rendered routing host", func(t *testing.T) {
		flags := &config.RuntimeFlags{
			Deployment: config.DeploymentFlags{
				ExtraHelmSets: map[string]string{"global.host": "{{ .Release.Name }}.example.com"},
			},
		}
		scenarioCtx := &ScenarioContext{Namespace: "test", Release: "integration", IngressHost: "computed.example.com"}
		lookupCalled := false

		got := resolveIngressReadyHostWith(
			context.Background(),
			flags,
			scenarioCtx,
			func(string) string { return "env.example.com" },
			func(context.Context, string, string, string) string {
				lookupCalled = true
				return "rendered.example.com"
			},
		)

		if got != "rendered.example.com" {
			t.Fatalf("resolveIngressReadyHostWith() = %q, want rendered routing host", got)
		}
		if !lookupCalled {
			t.Fatal("cluster lookup should run when global.host contains a Helm template")
		}
	})

	t.Run("environment host wins over computed fallback when no deployed host is found", func(t *testing.T) {
		flags := &config.RuntimeFlags{}
		scenarioCtx := &ScenarioContext{Namespace: "test", Release: "integration", IngressHost: "computed.example.com"}

		got := resolveIngressReadyHostWith(
			context.Background(),
			flags,
			scenarioCtx,
			func(string) string { return "env.example.com" },
			func(context.Context, string, string, string) string { return "" },
		)

		if got != "env.example.com" {
			t.Fatalf("resolveIngressReadyHostWith() = %q, want environment host", got)
		}
	})

	t.Run("concrete configured host remains a fallback when discovery fails", func(t *testing.T) {
		flags := &config.RuntimeFlags{
			Deployment: config.DeploymentFlags{
				ExtraHelmSets: map[string]string{"global.host": "configured.example.com"},
			},
		}
		scenarioCtx := &ScenarioContext{Namespace: "test", Release: "integration", IngressHost: "computed.example.com"}

		got := resolveIngressReadyHostWith(
			context.Background(),
			flags,
			scenarioCtx,
			func(string) string { return "env.example.com" },
			func(context.Context, string, string, string) string { return "" },
		)

		if got != "configured.example.com" {
			t.Fatalf("resolveIngressReadyHostWith() = %q, want configured host", got)
		}
	})

	t.Run("legacy ingress host takes precedence in configured fallbacks", func(t *testing.T) {
		flags := &config.RuntimeFlags{
			Deployment: config.DeploymentFlags{
				ExtraHelmSets: map[string]string{
					"global.host":         "current.example.com",
					"global.ingress.host": "legacy.example.com",
				},
			},
		}
		scenarioCtx := &ScenarioContext{Namespace: "test", Release: "integration"}

		got := resolveIngressReadyHostWith(
			context.Background(),
			flags,
			scenarioCtx,
			func(string) string { return "" },
			func(context.Context, string, string, string) string { return "" },
		)

		if got != "legacy.example.com" {
			t.Fatalf("resolveIngressReadyHostWith() = %q, want legacy host", got)
		}
	})

	t.Run("computed host remains the final fallback", func(t *testing.T) {
		flags := &config.RuntimeFlags{}
		scenarioCtx := &ScenarioContext{Namespace: "test", Release: "integration", IngressHost: "computed.example.com"}

		got := resolveIngressReadyHostWith(
			context.Background(),
			flags,
			scenarioCtx,
			func(string) string { return "" },
			func(context.Context, string, string, string) string { return "" },
		)

		if got != "computed.example.com" {
			t.Fatalf("resolveIngressReadyHostWith() = %q, want computed fallback", got)
		}
	})
}

func TestResolveDeployedRoutingHostWith(t *testing.T) {
	tests := []struct {
		name    string
		outputs map[string][]unstructured.Unstructured
		errors  map[string]error
		want    string
	}{
		{
			name: "classic ingress host wins",
			outputs: map[string][]unstructured.Unstructured{
				"ingresses": {routingObject("integration-http", "integration", map[string]any{
					"rules": []any{
						map[string]any{"host": "grpc-app.example.com"},
						map[string]any{"host": "app.example.com"},
					},
				})},
			},
			want: "app.example.com",
		},
		{
			name: "unrelated routing resources are ignored",
			outputs: map[string][]unstructured.Unstructured{
				"ingresses": {
					routingObject("companion", "companion", map[string]any{"rules": []any{map[string]any{"host": "unrelated.example.com"}}}),
					routingObject("integration-http", "integration", map[string]any{"rules": []any{map[string]any{"host": "app.example.com"}}}),
				},
			},
			want: "app.example.com",
		},
		{
			name: "HTTPRoute host is used when ingress is empty",
			outputs: map[string][]unstructured.Unstructured{
				"httproutes": {routingObject("integration-orchestration", "integration", map[string]any{
					"hostnames": []any{"grpc-app.example.com", "route.example.com"},
				})},
			},
			want: "route.example.com",
		},
		{
			name: "Gateway listener host is used when ingress and HTTPRoute are unavailable",
			outputs: map[string][]unstructured.Unstructured{
				"gateways": {routingObject("integration-camunda-platform", "", map[string]any{
					"listeners": []any{
						map[string]any{"hostname": "grpc-app.example.com"},
						map[string]any{"hostname": "gateway.example.com"},
					},
				})},
			},
			errors: map[string]error{"httproutes": errors.New("resource unavailable")},
			want:   "gateway.example.com",
		},
		{
			name:   "no routing host returns empty",
			errors: map[string]error{"ingresses": errors.New("not found"), "httproutes": errors.New("not found"), "gateways": errors.New("not found")},
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls []string
			listResources := func(
				_ context.Context,
				namespace string,
				resource schema.GroupVersionResource,
			) (*unstructured.UnstructuredList, error) {
				calls = append(calls, resource.Resource)
				if namespace != "test" {
					t.Fatalf("namespace = %q, want test", namespace)
				}
				if err := tt.errors[resource.Resource]; err != nil {
					return nil, err
				}
				return &unstructured.UnstructuredList{Items: tt.outputs[resource.Resource]}, nil
			}

			got := resolveDeployedRoutingHostWith(context.Background(), "test", "integration", listResources)
			if got != tt.want {
				t.Fatalf("resolveDeployedRoutingHostWith() = %q, want %q (calls: %v)", got, tt.want, calls)
			}
		})
	}
}

func routingObject(name, release string, spec map[string]any) unstructured.Unstructured {
	labels := map[string]any{}
	if release != "" {
		labels["app.kubernetes.io/instance"] = release
	}
	return unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{"name": name, "labels": labels},
		"spec":     spec,
	}}
}

func TestSelectPrimaryRoutingHost(t *testing.T) {
	tests := []struct {
		raw  string
		want string
	}{
		{raw: "grpc-app.example.com app.example.com", want: "app.example.com"},
		{raw: "zeebe-app.example.com actuator-app.example.com", want: ""},
		{raw: "app.grpc-company.example", want: "app.grpc-company.example"},
		{raw: "*.example.com app.example.com", want: "app.example.com"},
		{raw: "*.example.com", want: ""},
	}

	for _, tt := range tests {
		if got := selectPrimaryRoutingHost(tt.raw); got != tt.want {
			t.Errorf("selectPrimaryRoutingHost(%q) = %q, want %q", tt.raw, got, tt.want)
		}
	}
}

func TestWaitIngressReadyWithDeps(t *testing.T) {
	t.Run("fast path: resolver and server succeed on first poll", func(t *testing.T) {
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		deps := ingressReadyDeps{
			resolver: &fakeResolver{},
			client:   srv.Client(),
			sleep:    noSleep,
		}

		err := waitIngressReadyWithDeps(context.Background(), deps, hostFromURL(t, srv.URL), time.Minute, time.Millisecond)
		if err != nil {
			t.Fatalf("waitIngressReadyWithDeps() error = %v, want nil", err)
		}
	})

	t.Run("timeout path: resolver always returns NXDOMAIN", func(t *testing.T) {
		deps := ingressReadyDeps{
			resolver: alwaysFailResolver{},
			client:   http.DefaultClient,
			sleep:    noSleep,
		}

		err := waitIngressReadyWithDeps(context.Background(), deps, "never-resolves.example.com", 0, time.Millisecond)
		if err == nil {
			t.Fatal("waitIngressReadyWithDeps() error = nil, want error mentioning DNS")
		}
		if !containsDNSErr(err) {
			t.Errorf("waitIngressReadyWithDeps() error = %v, want it to mention the DNS failure", err)
		}
	})

	t.Run("transient DNS failures then success", func(t *testing.T) {
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		deps := ingressReadyDeps{
			resolver: &fakeResolver{failFor: 2},
			client:   srv.Client(),
			sleep: func(ctx context.Context, _ time.Duration) error {
				return ctx.Err()
			},
		}

		err := waitIngressReadyWithDeps(context.Background(), deps, hostFromURL(t, srv.URL), time.Minute, time.Millisecond)
		if err != nil {
			t.Fatalf("waitIngressReadyWithDeps() error = %v, want nil", err)
		}
	})

	t.Run("3xx redirect counts as reachable", func(t *testing.T) {
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Location", "https://unreachable.invalid/")
			w.WriteHeader(http.StatusFound)
		}))
		defer srv.Close()

		client := srv.Client()
		client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

		deps := ingressReadyDeps{
			resolver: &fakeResolver{},
			client:   client,
			sleep:    noSleep,
		}

		err := waitIngressReadyWithDeps(context.Background(), deps, hostFromURL(t, srv.URL), time.Minute, time.Millisecond)
		if err != nil {
			t.Fatalf("waitIngressReadyWithDeps() error = %v, want nil (3xx is a reachable response)", err)
		}
	})

	t.Run("DNS resolves but HTTP is unreachable", func(t *testing.T) {
		deps := ingressReadyDeps{
			resolver: &fakeResolver{},
			client: &http.Client{
				Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					return nil, errors.New("connection refused")
				}),
			},
			sleep: noSleep,
		}

		err := waitIngressReadyWithDeps(context.Background(), deps, "example.com", 0, time.Millisecond)
		if err == nil {
			t.Fatal("waitIngressReadyWithDeps() error = nil, want error mentioning HTTP")
		}
		if !containsHTTPErr(err) {
			t.Errorf("waitIngressReadyWithDeps() error = %v, want it to mention the HTTP failure", err)
		}
	})

	t.Run("untrusted TLS certificate is not reachable", func(t *testing.T) {
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		deps := ingressReadyDeps{
			resolver: &fakeResolver{},
			client:   &http.Client{},
			sleep:    noSleep,
		}

		err := waitIngressReadyWithDeps(context.Background(), deps, hostFromURL(t, srv.URL), 0, time.Millisecond)
		if err == nil {
			t.Fatal("waitIngressReadyWithDeps() error = nil, want error mentioning HTTP/TLS failure")
		}
		if !containsHTTPErr(err) {
			t.Errorf("waitIngressReadyWithDeps() error = %v, want it to mention the HTTP/TLS failure", err)
		}
	})

	t.Run("deadline expiry during sleep reports timeout with probe detail", func(t *testing.T) {
		deps := ingressReadyDeps{
			resolver: alwaysFailResolver{},
			client:   http.DefaultClient,
			sleep: func(context.Context, time.Duration) error {
				return context.DeadlineExceeded
			},
		}

		err := waitIngressReadyWithDeps(context.Background(), deps, "never-resolves.example.com", time.Minute, time.Millisecond)
		if err == nil {
			t.Fatal("waitIngressReadyWithDeps() error = nil, want timeout error mentioning DNS")
		}
		if !containsDNSErr(err) {
			t.Errorf("waitIngressReadyWithDeps() error = %v, want it to preserve the DNS failure detail", err)
		}
		if strings.Contains(err.Error(), "canceled") {
			t.Errorf("waitIngressReadyWithDeps() error = %v, want a timeout not a 'canceled' message", err)
		}
	})
}

// roundTripFunc adapts a function to http.RoundTripper.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

// hostFromURL extracts the host:port portion of a httptest server URL, since
// waitIngressReadyWithDeps only takes a bare host and builds the https:// URL itself.
func hostFromURL(t *testing.T, rawURL string) string {
	t.Helper()
	const prefix = "https://"
	if len(rawURL) <= len(prefix) || rawURL[:len(prefix)] != prefix {
		t.Fatalf("unexpected test server URL: %s", rawURL)
	}
	return rawURL[len(prefix):]
}

func containsDNSErr(err error) bool {
	return err != nil && strings.Contains(err.Error(), "dns error=") && !strings.Contains(err.Error(), "dns error=<nil>")
}

func containsHTTPErr(err error) bool {
	return err != nil && strings.Contains(err.Error(), "http error=") && !strings.Contains(err.Error(), "http error=<nil>")
}
