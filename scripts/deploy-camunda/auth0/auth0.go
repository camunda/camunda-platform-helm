// Package auth0 provisions and cleans up Auth0 OIDC clients for Camunda
// integration tests. It is the canonical implementation called by both the
// "deploy-camunda auth0" CLI subcommand and (in the future) the matrix
// runner to execute Auth0 OIDC scenarios end-to-end.
//
// Unlike the entra package — which provisions a single "venom" child app —
// auth0 creates one client per Camunda component (identity, orchestration,
// optimize, connectors, Web Modeler, Console). The connectors client is
// non-interactive (M2M); identity/orchestration/optimize are regular_web;
// Web Modeler and Console are SPA. All are first-party, created via the
// Management API (DCR third-party clients cannot use client_credentials,
// which connectors requires).
package auth0

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"scripts/camunda-core/pkg/kube"
	"scripts/camunda-core/pkg/logging"
)

// auth0BaseURL is the Auth0 tenant base URL (no trailing slash). Variable so
// tests can override with an httptest server URL.
var auth0BaseURL = "https://distribution-team.eu.auth0.com"

// Retry parameters for Auth0 Management API calls. Variables (not consts) so
// tests can shrink them to ms scale. The Auth0 tenant has a "global limit"
// that triggers HTTP 429s under burst load; bounded exponential backoff
// recovers without operator intervention.
var (
	retryBaseBackoff = 1 * time.Second
	retryMaxBackoff  = 16 * time.Second
	retryMaxAttempts = 8
)

const (
	// DefaultAudience is the API identifier (resource server) that Camunda
	// services validate tokens against. Override per-deployment via Options.
	DefaultAudience = "distribution-team-oidc"

	// DefaultSecretName is the K8s secret consumed by the chart for OIDC
	// client_secret values. Override per-deployment via Options.
	DefaultSecretName = "client-secret-for-components"

	// Component names. The first four are confidential (with secrets), the
	// last two are public (SPA, no secret).
	ComponentIdentity      = "identity"
	ComponentOrchestration = "orchestration"
	ComponentOptimize      = "optimize"
	ComponentConnectors    = "connectors"
	ComponentWebModeler    = "Web Modeler"
	ComponentConsole       = "Console"
)

// PrivateComponents lists the confidential clients (with client_secret).
var PrivateComponents = []string{
	ComponentIdentity,
	ComponentOrchestration,
	ComponentOptimize,
	ComponentConnectors,
}

// PublicComponents lists the SPA clients (no client_secret).
var PublicComponents = []string{
	ComponentWebModeler,
	ComponentConsole,
}

// Options configures a provisioning or cleanup operation.
type Options struct {
	// Namespace is the Kubernetes namespace where the K8s secret will be
	// created. Also used to scope client display names ("<namespace>-<component>")
	// so multiple integration runs can coexist on one Auth0 tenant.
	Namespace string

	// KubeContext is the Kubernetes context to use for secret creation.
	// May be empty (uses default context).
	KubeContext string

	// IngressHost is the deployment's ingress hostname (e.g.
	// "my-ns.ci.distro.ultrawombat.com"). Used to build redirect URIs.
	IngressHost string

	// Audience is the API audience for client_grants and OIDC tokens.
	// Falls back to DefaultAudience when empty.
	Audience string

	// Domain is the Auth0 tenant base URL (e.g.
	// "https://distribution-team.eu.auth0.com"). Falls back to AUTH0_DOMAIN
	// env var, then to the package default.
	Domain string

	// MgmtToken is a pre-acquired Management API bearer token. Preferred when
	// set. Falls back to AUTH0_MGMT_TOKEN env var.
	MgmtToken string

	// MgmtClientID / MgmtClientSecret are an M2M client_credentials pair used
	// to acquire a Management API token at runtime. Falls back to
	// AUTH0_MGMT_CLIENT_ID / AUTH0_MGMT_CLIENT_SECRET env vars. Ignored when
	// MgmtToken is set.
	MgmtClientID     string
	MgmtClientSecret string

	// SecretName overrides the K8s secret name. Defaults to DefaultSecretName.
	SecretName string

	// SkipK8sSecret skips the creation of the K8s secret during EnsureClients.
	// When true, the caller is responsible for calling CreateK8sSecret later
	// (e.g. via a PreInstallHook after the namespace exists).
	SkipK8sSecret bool

	// PostgresPasswords are extra key=password entries written into the K8s
	// secret alongside the auth0-* client secrets. Used by the chart's
	// bundled postgres deployments. May be nil.
	PostgresPasswords map[string]string

	// HTTPClient is an optional HTTP client. Defaults to http.DefaultClient.
	HTTPClient *http.Client
}

// Client is a single provisioned Auth0 client.
type Client struct {
	Component    string // logical Camunda component (e.g. "orchestration")
	Name         string // Auth0 client name as registered ("<namespace>-<component>")
	ClientID     string
	ClientSecret string // empty for public/SPA clients
	Public       bool
}

// Provisioned is the result of EnsureClients.
type Provisioned struct {
	Private []Client // identity, orchestration, optimize, connectors
	Public  []Client // Web Modeler, Console
}

// All returns Private and Public concatenated, in stable order.
func (p *Provisioned) All() []Client {
	out := make([]Client, 0, len(p.Private)+len(p.Public))
	out = append(out, p.Private...)
	out = append(out, p.Public...)
	return out
}

// ByComponent returns the Client with the given component name, or false.
func (p *Provisioned) ByComponent(component string) (Client, bool) {
	for _, c := range p.All() {
		if c.Component == component {
			return c, true
		}
	}
	return Client{}, false
}

// IsAuth0Identity returns true if the matrix entry uses Auth0 OIDC.
// Mirrors entra.IsOIDCEntry.
func IsAuth0Identity(identity string) bool {
	return identity == "auth0"
}

// resolveOpts fills in empty Options fields from environment variables and
// validates required ones.
func resolveOpts(opts *Options) error {
	if opts.Domain == "" {
		opts.Domain = os.Getenv("AUTH0_DOMAIN")
	}
	if opts.Domain == "" {
		opts.Domain = auth0BaseURL
	}
	if opts.Audience == "" {
		opts.Audience = DefaultAudience
	}
	if opts.SecretName == "" {
		opts.SecretName = DefaultSecretName
	}
	if opts.MgmtToken == "" {
		opts.MgmtToken = os.Getenv("AUTH0_MGMT_TOKEN")
	}
	if opts.MgmtClientID == "" {
		opts.MgmtClientID = os.Getenv("AUTH0_MGMT_CLIENT_ID")
	}
	if opts.MgmtClientSecret == "" {
		opts.MgmtClientSecret = os.Getenv("AUTH0_MGMT_CLIENT_SECRET")
	}
	if opts.MgmtToken == "" && (opts.MgmtClientID == "" || opts.MgmtClientSecret == "") {
		return fmt.Errorf("set AUTH0_MGMT_TOKEN, or AUTH0_MGMT_CLIENT_ID + AUTH0_MGMT_CLIENT_SECRET")
	}
	if opts.Namespace == "" {
		return fmt.Errorf("namespace is required")
	}
	if opts.IngressHost == "" {
		return fmt.Errorf("ingress host is required")
	}
	return nil
}

func httpClientFor(opts *Options) *http.Client {
	if opts.HTTPClient != nil {
		return opts.HTTPClient
	}
	return http.DefaultClient
}

// clientName returns the deterministic Auth0 client name for a given
// namespace/component pair. Used to scope clients per-namespace so multiple
// CI runs don't collide.
func clientName(namespace, component string) string {
	return namespace + "-" + component
}

// redirectURIs returns the OIDC redirect URLs for a given component on a
// given ingress host. Returns nil for connectors (pure M2M, no callbacks).
func redirectURIs(component, ingressHost string) []string {
	switch component {
	case ComponentIdentity:
		return []string{"https://" + ingressHost + "/identity/auth/login-callback"}
	case ComponentOrchestration:
		return []string{
			"https://" + ingressHost + "/orchestration/sso-callback",
			"https://" + ingressHost + "/operate/identity-callback",
			"https://" + ingressHost + "/tasklist/identity-callback",
		}
	case ComponentOptimize:
		return []string{"https://" + ingressHost + "/optimize/api/authentication/callback"}
	case ComponentWebModeler:
		return []string{
			"https://" + ingressHost + "/modeler/login-callback",
			"https://" + ingressHost + "/modeler",
		}
	case ComponentConsole:
		return []string{"https://" + ingressHost + "/"}
	default:
		return nil
	}
}

// EnsureClients provisions every Camunda OIDC client for the given namespace
// and grants each the configured audience. When SkipK8sSecret is false (the
// default) it also writes the K8s secret. Returns the provisioned clients.
//
// The flow:
//  1. Acquire a Management API token (token-first, then M2M client_credentials).
//  2. POST /api/v2/clients per component (first-party + appropriate grant_types).
//  3. POST /api/v2/client-grants per *private* client to grant the audience.
//  4. Optionally write the K8s secret with auth0-<component>=<secret> entries.
func EnsureClients(ctx context.Context, opts Options) (*Provisioned, error) {
	if err := resolveOpts(&opts); err != nil {
		return nil, fmt.Errorf("auth0: %w", err)
	}
	client := httpClientFor(&opts)

	logging.Logger.Info().
		Str("namespace", opts.Namespace).
		Str("audience", opts.Audience).
		Str("ingressHost", opts.IngressHost).
		Msg("Provisioning Auth0 clients")

	token, err := acquireManagementToken(ctx, client, &opts)
	if err != nil {
		return nil, fmt.Errorf("auth0: %w", err)
	}

	prov := &Provisioned{}
	for _, comp := range PrivateComponents {
		c, err := createClient(ctx, client, token, &opts, comp, kindFor(comp))
		if err != nil {
			return prov, fmt.Errorf("auth0: create private client %q: %w", comp, err)
		}
		logging.Logger.Info().
			Str("component", comp).
			Str("clientId", c.ClientID).
			Msg("Created Auth0 private client")
		prov.Private = append(prov.Private, c)
	}
	for _, comp := range PublicComponents {
		c, err := createClient(ctx, client, token, &opts, comp, kindSPA)
		if err != nil {
			return prov, fmt.Errorf("auth0: create public client %q: %w", comp, err)
		}
		logging.Logger.Info().
			Str("component", comp).
			Str("clientId", c.ClientID).
			Msg("Created Auth0 public client")
		prov.Public = append(prov.Public, c)
	}

	for _, c := range prov.Private {
		if err := grantAudience(ctx, client, token, &opts, c.ClientID); err != nil {
			return prov, fmt.Errorf("auth0: grant audience for %s: %w", c.Component, err)
		}
		logging.Logger.Info().
			Str("component", c.Component).
			Str("audience", opts.Audience).
			Msg("Granted audience access")
	}

	if !opts.SkipK8sSecret {
		if err := CreateK8sSecret(ctx, opts.KubeContext, opts.Namespace, opts.SecretName, prov, opts.PostgresPasswords); err != nil {
			return prov, fmt.Errorf("auth0: %w", err)
		}
		logging.Logger.Info().
			Str("namespace", opts.Namespace).
			Str("secret", opts.SecretName).
			Msg("Wrote K8s secret with Auth0 client secrets")
	} else {
		logging.Logger.Info().
			Msg("Skipping K8s secret creation (SkipK8sSecret=true); caller must invoke CreateK8sSecret later")
	}

	return prov, nil
}

// CleanupClients deletes every Auth0 client whose name matches
// "<namespace>-<component>". Best-effort: errors are logged but not returned.
//
// Uses a single paged list of all clients in the tenant (with retry on 429),
// then deletes only the ones we expect for this namespace. This is dramatically
// kinder on the Auth0 rate limit than the previous per-component lookup loop.
func CleanupClients(ctx context.Context, opts Options) {
	if err := resolveOpts(&opts); err != nil {
		logging.Logger.Warn().Err(err).Msg("auth0 cleanup: invalid options, skipping")
		return
	}
	client := httpClientFor(&opts)

	token, err := acquireManagementToken(ctx, client, &opts)
	if err != nil {
		logging.Logger.Warn().Err(err).Msg("auth0 cleanup: failed to acquire management token")
		return
	}

	allClients, err := listAllClients(ctx, client, token, &opts)
	if err != nil {
		logging.Logger.Warn().Err(err).Msg("auth0 cleanup: list failed")
		return
	}

	all := append([]string{}, PrivateComponents...)
	all = append(all, PublicComponents...)
	expected := make(map[string]bool, len(all))
	for _, comp := range all {
		expected[clientName(opts.Namespace, comp)] = true
	}

	// Iterate every client in the tenant and delete any whose name matches
	// an expected "<namespace>-<component>" entry. Iterating the slice
	// (rather than indexing by name) ensures duplicates with the same name
	// — which Auth0 allows — all get cleaned up.
	for _, c := range allClients {
		if !expected[c.Name] {
			continue
		}
		if err := deleteClient(ctx, client, token, &opts, c.ClientID); err != nil {
			logging.Logger.Warn().Err(err).Str("name", c.Name).Str("clientId", c.ClientID).Msg("auth0 cleanup: delete failed")
			continue
		}
		logging.Logger.Info().Str("name", c.Name).Str("clientId", c.ClientID).Msg("Deleted Auth0 client")
	}
}

// CreateK8sSecret creates or updates the K8s secret holding Auth0 private
// client_secrets and any extra postgres passwords. Public clients are
// excluded (SPAs have no usable secret).
func CreateK8sSecret(ctx context.Context, kubeContext, namespace, secretName string, prov *Provisioned, postgresPasswords map[string]string) error {
	if secretName == "" {
		secretName = DefaultSecretName
	}
	k8sClient, err := kube.NewClient("", kubeContext)
	if err != nil {
		return fmt.Errorf("create K8s client: %w", err)
	}

	data := map[string]string{}
	for _, c := range prov.Private {
		data["auth0-"+c.Component] = c.ClientSecret
	}
	for k, v := range postgresPasswords {
		data[k] = v
	}

	if err := k8sClient.EnsureOpaqueSecret(ctx, namespace, secretName, data); err != nil {
		return fmt.Errorf("apply secret %s/%s: %w", namespace, secretName, err)
	}
	return nil
}

// ---- Internals ----

// doWithRetry executes an HTTP request built by buildReq, retrying on 429 and
// 5xx with bounded exponential backoff. The buildReq closure is called once
// per attempt because http.Request bodies are not replayable. On success
// (any 2xx) it returns the body + status. On non-retryable errors (4xx other
// than 429) it returns the body + status with no error so callers can decide
// the meaning (e.g. 404 from a delete is "already gone"). When attempts are
// exhausted it returns the last status/body together with a wrapped error.
func doWithRetry(ctx context.Context, client *http.Client, buildReq func() (*http.Request, error), op string) ([]byte, int, error) {
	var (
		body       []byte
		statusCode int
		lastErr    error
	)
	backoff := retryBaseBackoff
	for attempt := 1; attempt <= retryMaxAttempts; attempt++ {
		req, err := buildReq()
		if err != nil {
			return nil, 0, fmt.Errorf("%s: build request: %w", op, err)
		}
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			statusCode = 0
			body = nil
		} else {
			body, _ = io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			statusCode = resp.StatusCode
			lastErr = nil
			if !isRetryable(statusCode) {
				if attempt > 1 {
					logging.Logger.Info().
						Str("op", op).
						Int("attempt", attempt).
						Int("statusCode", statusCode).
						Msg("Auth0 call succeeded after retry")
				}
				return body, statusCode, nil
			}
		}

		if attempt == retryMaxAttempts {
			break
		}

		// Honour Retry-After if Auth0 set one; otherwise use exponential
		// backoff with jitter.
		wait := backoffWithJitter(backoff)
		if statusCode == http.StatusTooManyRequests {
			if ra := retryAfter(body); ra > 0 {
				wait = ra
			}
		}

		logging.Logger.Warn().
			Str("op", op).
			Int("attempt", attempt).
			Int("statusCode", statusCode).
			Err(lastErr).
			Dur("nextBackoff", wait).
			Msg("Auth0 call retried (rate-limited or transient)")

		select {
		case <-ctx.Done():
			return body, statusCode, ctx.Err()
		case <-time.After(wait):
		}

		if backoff < retryMaxBackoff {
			backoff *= 2
			if backoff > retryMaxBackoff {
				backoff = retryMaxBackoff
			}
		}
	}

	if lastErr != nil {
		return body, statusCode, fmt.Errorf("%s: gave up after %d attempts: %w", op, retryMaxAttempts, lastErr)
	}
	return body, statusCode, fmt.Errorf("%s: gave up after %d attempts (last status=%d body=%s)", op, retryMaxAttempts, statusCode, string(body))
}

// isRetryable returns true for HTTP statuses worth retrying. Transport errors
// are handled by doWithRetry separately (statusCode==0 in that case).
func isRetryable(statusCode int) bool {
	if statusCode == 0 {
		return true // transport-level error
	}
	if statusCode == http.StatusTooManyRequests {
		return true
	}
	if statusCode >= 500 && statusCode <= 599 {
		return true
	}
	return false
}

func backoffWithJitter(base time.Duration) time.Duration {
	if base <= 0 {
		return 0
	}
	return base + time.Duration(rand.Int63n(int64(base/4)+1))
}

// retryAfter parses an Auth0 429 body's "Retry-After" hint if present. Auth0
// usually sets a header but also sometimes echoes the suggested wait in the
// body — header parsing is handled by net/http indirectly; here we look at
// the body's textual hint as a best-effort fallback. Returns 0 when nothing
// usable is found.
func retryAfter(body []byte) time.Duration {
	// Auth0 bodies don't reliably include a parseable retry hint; this is a
	// stub kept for symmetry with the entra package and for future extension
	// (e.g. parsing the X-RateLimit-Reset header). Returns 0 → caller uses
	// exponential backoff.
	_ = body
	_ = strconv.Atoi // keep import live in case future extension parses
	return 0
}

type clientKind int

const (
	kindRegularWeb clientKind = iota // server-side OIDC
	kindM2M                          // non_interactive, client_credentials only
	kindSPA                          // SPA, no client_secret usage
)

func kindFor(component string) clientKind {
	if component == ComponentConnectors {
		return kindM2M
	}
	return kindRegularWeb
}

// acquireManagementToken returns a Management API bearer token. Prefers an
// already-acquired token (Options.MgmtToken / AUTH0_MGMT_TOKEN) and falls
// back to client_credentials with the M2M pair.
func acquireManagementToken(ctx context.Context, client *http.Client, opts *Options) (string, error) {
	if opts.MgmtToken != "" {
		return opts.MgmtToken, nil
	}
	tokenURL := opts.Domain + "/oauth/token"
	payload := map[string]string{
		"grant_type":    "client_credentials",
		"client_id":     opts.MgmtClientID,
		"client_secret": opts.MgmtClientSecret,
		"audience":      opts.Domain + "/api/v2/",
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("encode token request: %w", err)
	}

	body, status, err := doWithRetry(ctx, client, func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		return req, nil
	}, "acquireManagementToken")
	if err != nil {
		return "", err
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("management token endpoint %d: %s", status, string(body))
	}
	var out struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", fmt.Errorf("decode token response: %w", err)
	}
	if out.AccessToken == "" {
		return "", fmt.Errorf("management token endpoint returned empty access_token: %s", string(body))
	}
	return out.AccessToken, nil
}

// createClient creates a single first-party Auth0 client with appropriate
// app_type, grant_types and token_endpoint_auth_method for the kind.
func createClient(ctx context.Context, client *http.Client, token string, opts *Options, component string, kind clientKind) (Client, error) {
	var (
		appType         string
		grantTypes      []string
		tokenAuthMethod string
		public          bool
	)
	switch kind {
	case kindM2M:
		appType = "non_interactive"
		grantTypes = []string{"client_credentials"}
		tokenAuthMethod = "client_secret_post"
	case kindSPA:
		appType = "spa"
		grantTypes = []string{"authorization_code", "refresh_token"}
		tokenAuthMethod = "none"
		public = true
	default:
		appType = "regular_web"
		grantTypes = []string{"authorization_code", "refresh_token", "client_credentials"}
		tokenAuthMethod = "client_secret_post"
	}

	name := clientName(opts.Namespace, component)
	payload := map[string]interface{}{
		"name":                       name,
		"is_first_party":             true,
		"oidc_conformant":            true,
		"app_type":                   appType,
		"grant_types":                grantTypes,
		"token_endpoint_auth_method": tokenAuthMethod,
	}
	if cb := redirectURIs(component, opts.IngressHost); kind != kindM2M && len(cb) > 0 {
		payload["callbacks"] = cb
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return Client{}, fmt.Errorf("encode payload: %w", err)
	}

	body, status, err := doWithRetry(ctx, client, func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, opts.Domain+"/api/v2/clients", bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		return req, nil
	}, "createClient/"+component)
	if err != nil {
		return Client{}, err
	}
	if status != http.StatusCreated && status != http.StatusOK {
		return Client{}, fmt.Errorf("POST /api/v2/clients %d: %s", status, string(body))
	}
	var out struct {
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return Client{}, fmt.Errorf("decode response: %w", err)
	}
	if out.ClientID == "" {
		return Client{}, fmt.Errorf("management API returned empty client_id (body: %s)", string(body))
	}
	return Client{
		Component:    component,
		Name:         name,
		ClientID:     out.ClientID,
		ClientSecret: out.ClientSecret,
		Public:       public,
	}, nil
}

// grantAudience POSTs a client_grants entry tying clientID to opts.Audience.
// Idempotent: 409 Conflict (already granted) is treated as success.
func grantAudience(ctx context.Context, client *http.Client, token string, opts *Options, clientID string) error {
	payload := map[string]interface{}{
		"client_id": clientID,
		"audience":  opts.Audience,
		"scope":     []string{},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode payload: %w", err)
	}

	body, status, err := doWithRetry(ctx, client, func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, opts.Domain+"/api/v2/client-grants", bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		return req, nil
	}, "grantAudience")
	if err != nil {
		return err
	}
	switch status {
	case http.StatusCreated, http.StatusOK, http.StatusConflict:
		return nil
	default:
		return fmt.Errorf("POST /api/v2/client-grants %d: %s", status, string(body))
	}
}

// clientRef is one (name, client_id) pair returned by listAllClients.
type clientRef struct {
	Name     string
	ClientID string
}

// listAllClients returns every Auth0 client in the tenant as a slice of
// (name, client_id) pairs. Pages through the Management API in 100-client
// batches until exhausted. We deliberately return a slice (not a map keyed
// by name) because Auth0 does not enforce client-name uniqueness, and an
// earlier map-based implementation silently dropped duplicates — leaving
// orphans that subsequent cleanup runs couldn't see.
func listAllClients(ctx context.Context, client *http.Client, token string, opts *Options) ([]clientRef, error) {
	const perPage = 100
	var out []clientRef

	for page := 0; ; page++ {
		q := url.Values{}
		q.Set("fields", "client_id,name")
		q.Set("include_fields", "true")
		q.Set("per_page", strconv.Itoa(perPage))
		q.Set("page", strconv.Itoa(page))
		urlStr := opts.Domain + "/api/v2/clients?" + q.Encode()

		body, status, err := doWithRetry(ctx, client, func() (*http.Request, error) {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
			if err != nil {
				return nil, err
			}
			req.Header.Set("Authorization", "Bearer "+token)
			return req, nil
		}, "listClients")
		if err != nil {
			return nil, err
		}
		if status != http.StatusOK {
			return nil, fmt.Errorf("GET /api/v2/clients %d: %s", status, string(body))
		}

		var list []struct {
			ClientID string `json:"client_id"`
			Name     string `json:"name"`
		}
		if err := json.Unmarshal(body, &list); err != nil {
			return nil, fmt.Errorf("decode clients list: %w", err)
		}
		for _, c := range list {
			out = append(out, clientRef{Name: c.Name, ClientID: c.ClientID})
		}
		if len(list) < perPage {
			break // last page
		}
	}
	return out, nil
}

// deleteClient deletes an Auth0 client by client_id via the Management API.
// Retries on 429/5xx; treats 404 as "already gone".
func deleteClient(ctx context.Context, client *http.Client, token string, opts *Options, clientID string) error {
	body, status, err := doWithRetry(ctx, client, func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodDelete, opts.Domain+"/api/v2/clients/"+clientID, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		return req, nil
	}, "deleteClient/"+clientID)
	if err != nil {
		return err
	}
	if status != http.StatusNoContent && status != http.StatusNotFound {
		return fmt.Errorf("DELETE /api/v2/clients/%s %d: %s", clientID, status, string(body))
	}
	return nil
}
