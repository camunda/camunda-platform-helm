package deploy

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"scripts/camunda-core/pkg/logging"
)

// ingressReadyPollInterval is the wait between reachability polls in executeDeployment.
const ingressReadyPollInterval = 15 * time.Second

// hostResolver abstracts DNS resolution so tests can inject a fake without
// touching the network. net.Resolver satisfies this via LookupHost.
type hostResolver interface {
	LookupHost(ctx context.Context, host string) ([]string, error)
}

// ingressReadyDeps bundles the seams waitIngressReady needs to run
// deterministically under test: a resolver for DNS, an HTTP client for the
// reachability probe, and a sleep func so polling never blocks on real
// wall-clock time in unit tests.
type ingressReadyDeps struct {
	resolver hostResolver
	client   *http.Client
	sleep    func(ctx context.Context, d time.Duration) error
}

// waitIngressReady polls host until it is both publicly DNS-resolvable and
// answers an HTTPS request, or returns an error once timeout has elapsed. Any
// HTTP/TLS response — including 4xx/5xx — counts as reachable; only DNS
// failures (e.g. NXDOMAIN), connection failures (e.g. connection refused, no
// route), and TLS verification failures count as not-ready. This mirrors the
// E2E suite's getent + curl smoke probe without coupling to Keycloak token
// issuance.
func waitIngressReady(ctx context.Context, host string, timeout, interval time.Duration) error {
	return waitIngressReadyWithDeps(ctx, ingressReadyDeps{
		resolver: net.DefaultResolver,
		client:   newIngressProbeClient(),
		sleep:    sleepCtx,
	}, host, timeout, interval)
}

// newIngressProbeClient returns an HTTP client tuned for the readiness probe:
// a short per-attempt timeout and Go's default TLS verification, since the
// CI ingress serves a trusted, CA-signed certificate — an invalid, expired,
// or mis-issued cert must fail the probe rather than pass it. Redirects are
// not followed so a 3xx counts as a reachable response.
func newIngressProbeClient() *http.Client {
	return &http.Client{
		Timeout:       10 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

// sleepCtx waits for d, returning early with ctx.Err() if ctx is canceled first.
func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// waitIngressReadyWithDeps is waitIngressReady with injectable dependencies,
// so tests can run the full poll loop without real network calls or sleeps.
func waitIngressReadyWithDeps(ctx context.Context, deps ingressReadyDeps, host string, timeout, interval time.Duration) error {
	start := time.Now()
	deadline := start.Add(timeout)

	// Bound the whole check by the readiness deadline so a stalled resolver or
	// connection cannot run past --ingress-ready-timeout.
	loopCtx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()

	// Per-attempt cap so a single stalled DNS/HTTP call cannot consume the whole
	// budget; WithTimeout still clamps this to whatever remains before deadline.
	const perAttemptCap = 10 * time.Second

	var lastDNSErr, lastHTTPErr error
	attempt := 0

	for {
		attempt++

		lastDNSErr, lastHTTPErr = probeIngressOnce(loopCtx, deps, host, perAttemptCap, attempt, start)
		if lastDNSErr == nil && lastHTTPErr == nil {
			return nil
		}

		logging.Logger.Debug().
			Str("host", host).
			Int("attempt", attempt).
			AnErr("dnsErr", lastDNSErr).
			AnErr("httpErr", lastHTTPErr).
			Msg("⏳ [waitIngressReady] ingress not ready yet, retrying")

		if !time.Now().Before(deadline) {
			return fmt.Errorf("ingress %q not ready after %s (attempt %d): dns error=%v, http error=%v",
				host, time.Since(start).Round(time.Second), attempt, lastDNSErr, lastHTTPErr)
		}

		if err := deps.sleep(loopCtx, interval); err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				return fmt.Errorf("ingress %q not ready after %s (attempt %d): dns error=%v, http error=%v",
					host, time.Since(start).Round(time.Second), attempt, lastDNSErr, lastHTTPErr)
			}
			return fmt.Errorf("ingress %q readiness check canceled after %s: %w", host, time.Since(start).Round(time.Second), err)
		}
	}
}

// resolveDeployedIngressHost reads the primary web host from the Ingress
// objects helm just created in namespace, so the readiness gate probes the
// host actually served (and published to DNS) rather than a computed
// <namespace>.<base-domain> value that CI overrides via global.host. Returns
// "" on any error or when no primary host is found, so callers fall back to
// their computed host.
func resolveDeployedIngressHost(ctx context.Context, kubeContext, namespace string) string {
	args := []string{}
	if kubeContext != "" {
		args = append(args, "--context", kubeContext)
	}
	args = append(args, "-n", namespace, "get", "ingress",
		"-o", "jsonpath={.items[*].spec.rules[*].host}")
	out, err := exec.CommandContext(ctx, "kubectl", args...).Output()
	if err != nil {
		return ""
	}
	return selectPrimaryIngressHost(string(out))
}

// selectPrimaryIngressHost picks the primary web host from whitespace-separated
// ingress host tokens, skipping the gRPC/Zeebe/actuator sub-hosts, and returns
// the first match ("" if none). Kept pure so it is unit-testable without
// shelling out to kubectl.
func selectPrimaryIngressHost(raw string) string {
	for _, h := range strings.Fields(raw) {
		if strings.Contains(h, "zeebe") || strings.Contains(h, "grpc") || strings.Contains(h, "actuator") {
			continue
		}
		return h
	}
	return ""
}

// probeIngressOnce runs one DNS + HTTPS attempt bounded by perAttemptCap.
// It returns (nil, nil) on success; otherwise the DNS and/or HTTP error seen.
func probeIngressOnce(loopCtx context.Context, deps ingressReadyDeps, host string, perAttemptCap time.Duration, attempt int, start time.Time) (dnsErr, httpErr error) {
	attemptCtx, ac := context.WithTimeout(loopCtx, perAttemptCap)
	defer ac()

	if _, err := deps.resolver.LookupHost(attemptCtx, host); err != nil {
		return err, nil
	}

	url := "https://" + host
	req, err := http.NewRequestWithContext(attemptCtx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := deps.client.Do(req)
	if err != nil {
		return nil, err
	}
	resp.Body.Close()
	logging.Logger.Debug().
		Str("host", host).
		Int("attempt", attempt).
		Int("status", resp.StatusCode).
		Dur("elapsed", time.Since(start)).
		Msg("✅ [waitIngressReady] ingress reachable")
	return nil, nil
}
