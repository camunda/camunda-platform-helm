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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
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

func TestIngressReadyHostFallback(t *testing.T) {
	env := func(string) string { return "env.example.com" }
	noEnv := func(string) string { return "" }
	tests := []struct {
		name         string
		flags        *config.RuntimeFlags
		getenv       func(string) string
		deployedHost string
		computedHost string
		want         string
	}{
		{
			name:         "deployed routing host wins over explicit ingress hostname",
			flags:        &config.RuntimeFlags{Ingress: config.IngressFlags{IngressHostname: "override.example.com"}},
			getenv:       env,
			deployedHost: "rendered.example.com",
			computedHost: "computed.example.com",
			want:         "rendered.example.com",
		},
		{
			name:         "explicit ingress hostname remains a fallback",
			flags:        &config.RuntimeFlags{Ingress: config.IngressFlags{IngressHostname: "override.example.com"}},
			getenv:       env,
			computedHost: "computed.example.com",
			want:         "override.example.com",
		},
		{
			name: "rendered routing host wins over raw Helm host",
			flags: &config.RuntimeFlags{Deployment: config.DeploymentFlags{ExtraHelmSets: map[string]string{
				"global.host": "stale.example.com",
			}}},
			getenv:       env,
			deployedHost: "rendered.example.com",
			computedHost: "computed.example.com",
			want:         "rendered.example.com",
		},
		{
			name: "templated global host defers to the rendered routing host",
			flags: &config.RuntimeFlags{Deployment: config.DeploymentFlags{ExtraHelmSets: map[string]string{
				"global.host": "{{ .Release.Name }}.example.com",
			}}},
			getenv:       env,
			deployedHost: "rendered.example.com",
			computedHost: "computed.example.com",
			want:         "rendered.example.com",
		},
		{
			name:         "environment host wins over computed fallback when no deployed host is found",
			flags:        &config.RuntimeFlags{},
			getenv:       env,
			computedHost: "computed.example.com",
			want:         "env.example.com",
		},
		{
			name: "extra Helm host wins over explicit ingress fallback",
			flags: &config.RuntimeFlags{
				Deployment: config.DeploymentFlags{ExtraHelmSets: map[string]string{"global.host": "configured.example.com"}},
				Ingress:    config.IngressFlags{IngressHostname: "override.example.com"},
			},
			getenv:       env,
			computedHost: "computed.example.com",
			want:         "configured.example.com",
		},
		{
			name: "legacy ingress host takes precedence in configured fallbacks",
			flags: &config.RuntimeFlags{Deployment: config.DeploymentFlags{ExtraHelmSets: map[string]string{
				"global.host":         "current.example.com",
				"global.ingress.host": "legacy.example.com",
			}}},
			getenv: noEnv,
			want:   "legacy.example.com",
		},
		{
			name:         "computed host remains the final fallback",
			flags:        &config.RuntimeFlags{},
			getenv:       noEnv,
			computedHost: "computed.example.com",
			want:         "computed.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ingressReadyHostFallback(tt.flags, &ScenarioContext{IngressHost: tt.computedHost}, tt.getenv, tt.deployedHost)
			if got != tt.want {
				t.Fatalf("ingressReadyHostFallback() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveIngressReadyHostAfterLookup(t *testing.T) {
	forbiddenErr := apierrors.NewForbidden(
		schema.GroupResource{Group: "networking.k8s.io", Resource: "ingresses"},
		"",
		errors.New("denied"),
	)
	tests := []struct {
		name          string
		flags         *config.RuntimeFlags
		getenv        func(string) string
		computedHost  string
		lookupErr     error
		want          string
		wantForbidden bool
		wantErr       string
	}{
		{
			name: "configured host survives forbidden discovery",
			flags: &config.RuntimeFlags{Deployment: config.DeploymentFlags{ExtraHelmSets: map[string]string{
				"global.host": "configured.example.com",
			}}},
			getenv:       func(string) string { return "env.example.com" },
			computedHost: "computed.example.com",
			lookupErr:    forbiddenErr,
			want:         "configured.example.com",
		},
		{
			name:         "explicit ingress host survives forbidden discovery",
			flags:        &config.RuntimeFlags{Ingress: config.IngressFlags{IngressHostname: "override.example.com"}},
			getenv:       func(string) string { return "env.example.com" },
			computedHost: "computed.example.com",
			lookupErr:    forbiddenErr,
			want:         "override.example.com",
		},
		{
			name:         "environment host survives forbidden discovery",
			flags:        &config.RuntimeFlags{},
			getenv:       func(string) string { return "env.example.com" },
			computedHost: "computed.example.com",
			lookupErr:    forbiddenErr,
			want:         "env.example.com",
		},
		{
			name:          "forbidden discovery wins over computed fallback",
			flags:         &config.RuntimeFlags{},
			getenv:        func(string) string { return "" },
			computedHost:  "computed.example.com",
			lookupErr:     forbiddenErr,
			wantForbidden: true,
		},
		{
			name: "unexpected discovery error does not use configured fallback",
			flags: &config.RuntimeFlags{Deployment: config.DeploymentFlags{ExtraHelmSets: map[string]string{
				"global.host": "configured.example.com",
			}}},
			getenv:       func(string) string { return "env.example.com" },
			computedHost: "computed.example.com",
			lookupErr:    errors.New("connection reset"),
			wantErr:      "connection reset",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveIngressReadyHostAfterLookup(
				tt.flags,
				&ScenarioContext{IngressHost: tt.computedHost},
				tt.getenv,
				"",
				tt.lookupErr,
			)
			if tt.wantForbidden {
				if !apierrors.IsForbidden(err) {
					t.Fatalf("resolveIngressReadyHostAfterLookup() error = %v, want Forbidden", err)
				}
				return
			}
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("resolveIngressReadyHostAfterLookup() error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveIngressReadyHostAfterLookup() unexpected error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("resolveIngressReadyHostAfterLookup() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveDeployedRoutingHostWith(t *testing.T) {
	tests := []struct {
		name          string
		outputs       map[string][]unstructured.Unstructured
		errors        map[string]error
		want          string
		wantForbidden bool
		wantErr       string
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
					routingObject("integration-companion", "", map[string]any{"rules": []any{map[string]any{"host": "prefix.example.com"}}}),
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
			name: "owned route wins despite forbidden lookup for another resource type",
			outputs: map[string][]unstructured.Unstructured{
				"httproutes": {routingObject("integration-orchestration", "integration", map[string]any{
					"hostnames": []any{"route.example.com"},
				})},
			},
			errors: map[string]error{
				"ingresses": apierrors.NewForbidden(schema.GroupResource{Group: "networking.k8s.io", Resource: "ingresses"}, "", errors.New("denied")),
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
			name: "missing routing APIs return empty",
			errors: map[string]error{
				"ingresses":  apierrors.NewNotFound(schema.GroupResource{Group: "networking.k8s.io", Resource: "ingresses"}, ""),
				"httproutes": apierrors.NewNotFound(schema.GroupResource{Group: "gateway.networking.k8s.io", Resource: "httproutes"}, ""),
				"gateways":   apierrors.NewNotFound(schema.GroupResource{Group: "gateway.networking.k8s.io", Resource: "gateways"}, ""),
			},
			want: "",
		},
		{
			name: "unexpected discovery failure takes precedence over forbidden",
			errors: map[string]error{
				"ingresses":  apierrors.NewForbidden(schema.GroupResource{Group: "networking.k8s.io", Resource: "ingresses"}, "", errors.New("denied")),
				"httproutes": errors.New("connection reset"),
			},
			wantErr: "connection reset",
		},
		{
			name: "forbidden discovery without an owned route returns an error",
			errors: map[string]error{
				"ingresses": apierrors.NewForbidden(schema.GroupResource{Group: "networking.k8s.io", Resource: "ingresses"}, "", errors.New("denied")),
			},
			wantForbidden: true,
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

			got, err := resolveDeployedRoutingHostWith(context.Background(), "test", "integration", listResources)
			if tt.wantForbidden {
				if !apierrors.IsForbidden(err) {
					t.Fatalf("resolveDeployedRoutingHostWith() error = %v, want Forbidden", err)
				}
				return
			}
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("resolveDeployedRoutingHostWith() error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveDeployedRoutingHostWith() unexpected error = %v", err)
			}
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
