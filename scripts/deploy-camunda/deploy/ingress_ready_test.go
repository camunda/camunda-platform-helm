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
