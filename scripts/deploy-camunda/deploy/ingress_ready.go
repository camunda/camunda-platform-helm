package deploy

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"scripts/camunda-core/pkg/logging"
	"scripts/deploy-camunda/config"
)

// ingressReadyPollInterval is the wait between reachability polls in executeDeployment.
const ingressReadyPollInterval = 15 * time.Second

const routingHostLookupTimeout = 10 * time.Second

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

type kubectlOutputFunc func(ctx context.Context, args ...string) ([]byte, error)

// resolveIngressReadyHost selects the public host that was actually applied
// to the deployment before falling back to the precomputed scenario host.
func resolveIngressReadyHost(ctx context.Context, flags *config.RuntimeFlags, scenarioCtx *ScenarioContext) string {
	return resolveIngressReadyHostWith(ctx, flags, scenarioCtx, os.Getenv, resolveDeployedRoutingHost)
}

func resolveIngressReadyHostWith(
	ctx context.Context,
	flags *config.RuntimeFlags,
	scenarioCtx *ScenarioContext,
	getenv func(string) string,
	lookupDeployedHost func(context.Context, string, string) string,
) string {
	if host := concreteRoutingHost(flags.Deployment.ExtraHelmSets["global.host"]); host != "" {
		return host
	}

	deployedHost := lookupDeployedHost(ctx, flags.Test.KubeContext, scenarioCtx.Namespace)
	return config.FirstNonEmpty(
		deployedHost,
		scenarioCtx.IngressHost,
		getenv("CAMUNDA_HOSTNAME"),
		getenv("TEST_INGRESS_HOST"),
	)
}

// resolveDeployedRoutingHost discovers the first web hostname exposed by the
// routing resources in a namespace. HTTPRoute is checked before Gateway so an
// exact route hostname wins over a wildcard listener hostname.
func resolveDeployedRoutingHost(ctx context.Context, kubeContext, namespace string) string {
	lookupCtx, cancel := context.WithTimeout(ctx, routingHostLookupTimeout)
	defer cancel()
	return resolveDeployedRoutingHostWith(lookupCtx, kubeContext, namespace, runKubectlOutput)
}

func resolveDeployedRoutingHostWith(
	ctx context.Context,
	kubeContext,
	namespace string,
	kubectlOutput kubectlOutputFunc,
) string {
	queries := []struct {
		resource string
		jsonPath string
	}{
		{resource: "ingress", jsonPath: "{.items[*].spec.rules[*].host}"},
		{resource: "httproute", jsonPath: "{.items[*].spec.hostnames[*]}"},
		{resource: "gateway", jsonPath: "{.items[*].spec.listeners[*].hostname}"},
	}

	for _, query := range queries {
		args := make([]string, 0, 7)
		if kubeContext != "" {
			args = append(args, "--context", kubeContext)
		}
		args = append(args, "-n", namespace, "get", query.resource, "--request-timeout=5s", "-o", "jsonpath="+query.jsonPath)

		out, err := kubectlOutput(ctx, args...)
		if err != nil {
			continue
		}
		if host := selectPrimaryRoutingHost(string(out)); host != "" {
			return host
		}
	}

	return ""
}

func runKubectlOutput(ctx context.Context, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, "kubectl", args...).Output()
}

func selectPrimaryRoutingHost(raw string) string {
	for _, host := range strings.Fields(raw) {
		if concreteRoutingHost(host) == "" {
			continue
		}
		firstLabel := strings.ToLower(strings.SplitN(host, ".", 2)[0])
		if firstLabel == "grpc" || firstLabel == "zeebe" || firstLabel == "actuator" ||
			strings.HasPrefix(firstLabel, "grpc-") ||
			strings.HasPrefix(firstLabel, "zeebe-") ||
			strings.HasPrefix(firstLabel, "actuator-") {
			continue
		}
		return host
	}
	return ""
}

func concreteRoutingHost(raw string) string {
	host := strings.TrimSpace(raw)
	if host == "" || strings.Contains(host, "{{") || strings.Contains(host, "}}") || strings.Contains(host, "*") {
		return ""
	}
	return host
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
