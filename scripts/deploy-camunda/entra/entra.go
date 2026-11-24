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

// Package entra provisions and cleans up Microsoft Entra ID (Azure AD)
// app registrations for OIDC integration tests. This is the canonical
// implementation, called by both the "deploy-camunda entra" CLI subcommand
// and the matrix runner to execute OIDC scenarios end-to-end.
package entra

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
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"scripts/camunda-core/pkg/kube"
	"scripts/camunda-core/pkg/logging"
)

// graphBaseURL is the Microsoft Graph API base URL. It is a variable so tests
// can override it with a local httptest server URL.
var graphBaseURL = "https://graph.microsoft.com/v1.0"

// retryBaseBackoff is the initial backoff for graph retry loops. Exposed as a
// variable so tests can set it to millisecond scale.
var retryBaseBackoff = 1 * time.Second

// retryMaxBackoff caps the per-attempt backoff.
var retryMaxBackoff = 8 * time.Second

// retryMaxAttempts bounds the total number of retries for graph write calls
// that may race against Graph backend propagation (e.g. addPassword, create SP
// right after creating an app registration).
var retryMaxAttempts = 15

// propagationSleepDuration is the sleep between propagation-check attempts after
// creating a new app registration. Exposed as a variable so tests can set it to zero.
var propagationSleepDuration = 2 * time.Second

// loginAppResourceTimeout / loginAppResourceInterval bound the wait for the
// token service to serve the login app as its own resource.
var (
	loginAppResourceTimeout  = 10 * time.Minute
	loginAppResourceInterval = 10 * time.Second
)

// redirectVerifyDelay and redirectVerifyJitter set the wait between writing the
// redirect URIs and reading them back. Variables so tests can set them to zero.
var (
	redirectVerifyDelay  = 2 * time.Second
	redirectVerifyJitter = 3 * time.Second
)

// redirectVerifyAttempts bounds how often UpdateRedirectURIs rewrites the URIs
// when a concurrent registration removed them.
const redirectVerifyAttempts = 3

// redirectVerifyReads is how many read-backs follow each write before the URIs
// count as removed; Graph reads can return a replica's pre-write state.
const redirectVerifyReads = 6

// loginBaseURL is the Microsoft identity platform base URL. It is a variable
// so tests can override it with a local httptest server URL.
var loginBaseURL = "https://login.microsoftonline.com"

// Options configures a venom app provisioning or cleanup operation.
type Options struct {
	// Namespace is the Kubernetes namespace where the venom-entra-credentials
	// secret will be created. Also used to construct the Entra app display name
	// (venom-test-<namespace>).
	Namespace string

	// KubeContext is the Kubernetes context to use for secret creation.
	// May be empty (uses default context).
	KubeContext string

	// DirectoryID (tenant ID) for the Entra directory.
	// Falls back to ENTRA_APP_DIRECTORY_ID env var when empty.
	DirectoryID string

	// ClientID of the parent Entra app used to authenticate to Graph API.
	// Also used as the audience in the venom app's requiredResourceAccess.
	// Falls back to ENTRA_APP_CLIENT_ID env var when empty.
	ClientID string

	// ClientSecret of the parent Entra app for Graph API authentication.
	// Falls back to ENTRA_APP_CLIENT_SECRET env var when empty.
	ClientSecret string

	// HTTPClient is an optional HTTP client for making requests.
	// When nil, http.DefaultClient is used.
	HTTPClient *http.Client

	// SkipK8sSecret skips the creation of the venom-entra-credentials K8s
	// secret during EnsureVenomApp. When true, the caller is responsible for
	// creating the secret later (e.g., via a PreInstallHook after the
	// namespace exists). Use CreateVenomK8sSecret to create it.
	SkipK8sSecret bool
}

// VenomApp holds the result of provisioning a venom Entra app registration.
type VenomApp struct {
	// AppID is the Entra application (client) ID of the venom app.
	AppID string
	// ObjectID is the Entra object ID of the venom app registration.
	ObjectID string
	// ClientSecret is the generated client secret for the venom app.
	ClientSecret string
}

// resolveOpts fills in empty Options fields from environment variables.
func resolveOpts(opts *Options) error {
	if opts.DirectoryID == "" {
		opts.DirectoryID = os.Getenv("ENTRA_APP_DIRECTORY_ID")
	}
	if opts.ClientID == "" {
		opts.ClientID = os.Getenv("ENTRA_APP_CLIENT_ID")
	}
	if opts.ClientSecret == "" {
		opts.ClientSecret = os.Getenv("ENTRA_APP_CLIENT_SECRET")
	}

	if opts.DirectoryID == "" {
		return fmt.Errorf("ENTRA_APP_DIRECTORY_ID is required (set Options.DirectoryID or ENTRA_APP_DIRECTORY_ID env var)")
	}
	if opts.ClientID == "" {
		return fmt.Errorf("ENTRA_APP_CLIENT_ID is required (set Options.ClientID or ENTRA_APP_CLIENT_ID env var)")
	}
	if opts.ClientSecret == "" {
		return fmt.Errorf("ENTRA_APP_CLIENT_SECRET is required (set Options.ClientSecret or ENTRA_APP_CLIENT_SECRET env var)")
	}
	if opts.Namespace == "" {
		return fmt.Errorf("namespace is required")
	}
	return nil
}

func httpClient(opts *Options) *http.Client {
	if opts.HTTPClient != nil {
		return opts.HTTPClient
	}
	return http.DefaultClient
}

// appDisplayName returns the deterministic Entra app display name for a namespace.
func appDisplayName(namespace string) string {
	return "venom-test-" + namespace
}

// entraPortalAppURL returns the Entra admin center URL for an app registration.
func entraPortalAppURL(appID string) string {
	return fmt.Sprintf(
		"https://entra.microsoft.com/#view/Microsoft_AAD_RegisteredApps/ApplicationMenuBlade/~/Overview/appId/%s",
		appID,
	)
}

// acquireBearerToken authenticates to the Microsoft identity platform using
// client_credentials grant and returns a Graph API bearer token.
func acquireBearerToken(ctx context.Context, opts *Options) (string, error) {
	tokenURL := fmt.Sprintf("%s/%s/oauth2/v2.0/token", loginBaseURL, opts.DirectoryID)

	data := url.Values{
		"client_id":     {opts.ClientID},
		"scope":         {"https://graph.microsoft.com/.default"},
		"client_secret": {opts.ClientSecret},
		"grant_type":    {"client_credentials"},
	}

	logging.Logger.Debug().
		Str("tokenURL", tokenURL).
		Str("clientId", opts.ClientID).
		Str("directoryId", opts.DirectoryID).
		Msg("Requesting Graph API bearer token")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := httpClient(opts).Do(req)
	if err != nil {
		return "", fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	logging.Logger.Debug().
		Int("statusCode", resp.StatusCode).
		Msg("Token endpoint responded")

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
		ErrorDesc   string `json:"error_description"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("parse token response: %w (body: %s)", err, string(body))
	}
	if tokenResp.AccessToken == "" {
		return "", fmt.Errorf("failed to obtain bearer token: %s: %s", tokenResp.Error, tokenResp.ErrorDesc)
	}

	logging.Logger.Debug().Msg("Successfully acquired Graph API bearer token")
	return tokenResp.AccessToken, nil
}

// graphGet performs a GET request to the Graph API and rejects non-2xx responses.
func graphGet(ctx context.Context, client *http.Client, token, path string) ([]byte, error) {
	body, statusCode, err := graphGetStatus(ctx, client, token, path)
	if err != nil {
		return nil, err
	}
	if statusCode < 200 || statusCode >= 300 {
		return nil, fmt.Errorf("GET %s failed (status=%d): %s", path, statusCode, string(body))
	}
	return body, nil
}

// graphGetStatus performs a GET request to the Graph API and returns the
// response whatever its status.
func graphGetStatus(ctx context.Context, client *http.Client, token, path string) ([]byte, int, error) {
	reqURL := graphBaseURL + path
	logging.Logger.Debug().Str("method", "GET").Str("url", reqURL).Msg("Graph API request")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	logging.Logger.Debug().Int("statusCode", resp.StatusCode).Int("bodyLen", len(body)).Msg("Graph API GET response")
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return body, resp.StatusCode, nil
}

// retryPredicate decides whether to retry a Graph response. statusCode and
// body come from the last response; err is non-nil for transport-level errors
// (in which case statusCode/body are zero-valued).
type retryPredicate func(statusCode int, body []byte, err error) bool

// defaultRetryable retries on transport errors and the statuses Graph commonly
// returns while propagating a newly-created object or under concurrent
// mutation: 404, 408, 409, 429, and 5xx.
// Terminal statuses (400, 401, 403) are treated as caller errors.
func defaultRetryable(statusCode int, _ []byte, err error) bool {
	if err != nil {
		return true
	}
	switch {
	case statusCode == http.StatusNotFound:
		return true
	case statusCode == http.StatusRequestTimeout:
		return true
	case statusCode == http.StatusConflict: // Directory_ConcurrencyViolation
		return true
	case statusCode == http.StatusTooManyRequests:
		return true
	case statusCode >= 500 && statusCode <= 599:
		return true
	}
	return false
}

func servicePrincipalUpsertRetryable(statusCode int, body []byte, err error) bool {
	if defaultRetryable(statusCode, body, err) {
		return true
	}
	if statusCode == http.StatusBadRequest &&
		bytes.Contains(body, []byte("Request_BadRequest")) &&
		bytes.Contains(body, []byte("does not reference a valid application")) {
		return true
	}
	return false
}

func withGraphRetry(ctx context.Context, op string, retryable retryPredicate, request func() ([]byte, int, error)) ([]byte, int, error) {
	if retryable == nil {
		retryable = defaultRetryable
	}
	var (
		body       []byte
		statusCode int
		lastErr    error
	)
	backoff := retryBaseBackoff
	for attempt := 1; attempt <= retryMaxAttempts; attempt++ {
		body, statusCode, lastErr = request()

		if lastErr == nil && statusCode >= 200 && statusCode < 300 {
			if attempt > 1 {
				logging.Logger.Info().
					Str("op", op).
					Int("attempt", attempt).
					Int("statusCode", statusCode).
					Msg("Graph call succeeded after retry")
			}
			return body, statusCode, nil
		}

		if !retryable(statusCode, body, lastErr) {
			return body, statusCode, fmt.Errorf("%s: non-retryable status=%d body=%s", op, statusCode, string(body))
		}

		if attempt == retryMaxAttempts {
			break
		}

		wait := backoff + time.Duration(rand.Int63n(int64(backoff/4)+1))
		logging.Logger.Warn().
			Str("op", op).
			Int("attempt", attempt).
			Int("statusCode", statusCode).
			Err(lastErr).
			Dur("nextBackoff", wait).
			Msg("Graph call failed, retrying")

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

// graphPost performs a POST request to the Graph API.
func graphPost(ctx context.Context, client *http.Client, token, path string, payload any) ([]byte, int, error) {
	var bodyReader io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, 0, fmt.Errorf("marshal payload: %w", err)
		}
		bodyReader = strings.NewReader(string(data))
	}

	reqURL := graphBaseURL + path
	logging.Logger.Debug().Str("method", "POST").Str("url", reqURL).Msg("Graph API request")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bodyReader)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	logging.Logger.Debug().Int("statusCode", resp.StatusCode).Int("bodyLen", len(body)).Msg("Graph API POST response")
	return body, resp.StatusCode, err
}

// graphPatch performs a PATCH request to the Graph API.
//
//nolint:revive // same arguments as the other graph helpers plus the optional Prefer header
func graphPatch(ctx context.Context, client *http.Client, token, path string, payload any, headers http.Header) ([]byte, int, error) {
	var bodyReader io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, 0, fmt.Errorf("marshal payload: %w", err)
		}
		bodyReader = strings.NewReader(string(data))
	}

	reqURL := graphBaseURL + path
	logging.Logger.Debug().Str("method", "PATCH").Str("url", reqURL).Msg("Graph API request")

	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, reqURL, bodyReader)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	for name, values := range headers {
		for _, value := range values {
			req.Header.Add(name, value)
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	logging.Logger.Debug().Int("statusCode", resp.StatusCode).Int("bodyLen", len(body)).Msg("Graph API PATCH response")
	return body, resp.StatusCode, err
}

// graphDelete performs a DELETE request to the Graph API and returns the HTTP status code.
func graphDelete(ctx context.Context, client *http.Client, token, path string) (int, error) {
	reqURL := graphBaseURL + path
	logging.Logger.Debug().Str("method", "DELETE").Str("url", reqURL).Msg("Graph API request")

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, reqURL, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	logging.Logger.Debug().Int("statusCode", resp.StatusCode).Msg("Graph API DELETE response")
	return resp.StatusCode, nil
}

// findApp searches for an existing app registration by display name.
// Returns (appId, objectId) or empty strings if not found.
func findApp(ctx context.Context, client *http.Client, token, displayName string) (string, string, error) {
	logging.Logger.Debug().Str("displayName", displayName).Msg("Searching for existing Entra app registration")

	path := fmt.Sprintf("/applications?$filter=displayName%%20eq%%20'%s'", url.PathEscape(displayName))
	body, err := graphGet(ctx, client, token, path)
	if err != nil {
		return "", "", fmt.Errorf("search for app %q: %w", displayName, err)
	}

	var result struct {
		Value []struct {
			AppID string `json:"appId"`
			ID    string `json:"id"`
		} `json:"value"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", "", fmt.Errorf("parse app search response: %w", err)
	}

	logging.Logger.Debug().Int("matchCount", len(result.Value)).Msg("App registration search completed")

	if len(result.Value) > 0 {
		return result.Value[0].AppID, result.Value[0].ID, nil
	}
	return "", "", nil
}

// createApp creates a new Entra app registration with the given display name.
// The requiredResourceAccess grants the app access to the parent app's API scope.
func createApp(ctx context.Context, client *http.Client, token, displayName, parentClientID string) (string, string, error) {
	logging.Logger.Debug().
		Str("displayName", displayName).
		Str("parentClientId", parentClientID).
		Str("signInAudience", "AzureADMyOrg").
		Msg("Creating Entra app registration via Graph API")

	payload := map[string]any{
		"displayName":    displayName,
		"signInAudience": "AzureADMyOrg",
		"requiredResourceAccess": []map[string]any{
			{
				"resourceAppId": parentClientID,
				"resourceAccess": []map[string]any{
					{
						"id":   "00000000-0000-0000-0000-000000000000",
						"type": "Scope",
					},
				},
			},
		},
	}

	body, statusCode, err := graphPost(ctx, client, token, "/applications", payload)
	if err != nil {
		return "", "", fmt.Errorf("create app %q: %w", displayName, err)
	}

	var result struct {
		AppID string `json:"appId"`
		ID    string `json:"id"`
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", "", fmt.Errorf("parse create app response: %w (status=%d body=%s)", err, statusCode, string(body))
	}
	if result.AppID == "" {
		return "", "", fmt.Errorf("create app %q failed: %s: %s (status=%d)", displayName, result.Error.Code, result.Error.Message, statusCode)
	}

	logging.Logger.Debug().
		Str("appId", result.AppID).
		Str("objectId", result.ID).
		Str("displayName", displayName).
		Int("statusCode", statusCode).
		Msg("Graph API created app registration")

	return result.AppID, result.ID, nil
}

// rotateCredentials removes all existing password credentials and adds a fresh one.
// Returns the secret text of the new credential.
func rotateCredentials(ctx context.Context, client *http.Client, token, objectID string) (string, error) {
	logging.Logger.Debug().Str("objectId", objectID).Msg("Fetching existing password credentials")

	// Fetch existing credentials.
	appPath := fmt.Sprintf("/applications/%s", objectID)
	body, _, err := withGraphRetry(ctx, "getAppCredentials", nil, func() ([]byte, int, error) {
		return graphGetStatus(ctx, client, token, appPath)
	})
	if err != nil {
		return "", fmt.Errorf("get app credentials: %w", err)
	}

	var appDetails struct {
		PasswordCredentials []struct {
			KeyID string `json:"keyId"`
		} `json:"passwordCredentials"`
	}
	if err := json.Unmarshal(body, &appDetails); err != nil {
		return "", fmt.Errorf("parse app details: %w", err)
	}

	logging.Logger.Debug().
		Int("existingCredentials", len(appDetails.PasswordCredentials)).
		Str("objectId", objectID).
		Msg("Found existing password credentials")

	// Remove existing credentials to avoid accumulation.
	for _, cred := range appDetails.PasswordCredentials {
		logging.Logger.Debug().Str("keyId", cred.KeyID).Msg("Removing old password credential")
		removePayload := map[string]string{"keyId": cred.KeyID}
		if _, _, err := graphPost(ctx, client, token, fmt.Sprintf("/applications/%s/removePassword", objectID), removePayload); err != nil {
			logging.Logger.Warn().Err(err).Str("keyId", cred.KeyID).Msg("failed to remove old credential (continuing)")
		}
	}

	// Add a new client secret. Graph's backend is eventually consistent after
	// an app registration is created, so addPassword may return 404 for tens
	// of seconds after POST /applications succeeds. Retry with backoff.
	logging.Logger.Debug().Str("objectId", objectID).Msg("Adding new password credential")
	addPayload := map[string]any{
		"passwordCredential": map[string]string{
			"displayName": "integration-test-secret",
			"endDateTime": "2099-12-31T00:00:00Z",
		},
	}

	secretBody, statusCode, err := withGraphRetry(ctx, "addPassword", nil, func() ([]byte, int, error) {
		return graphPost(ctx, client, token, fmt.Sprintf("/applications/%s/addPassword", objectID), addPayload)
	})
	if err != nil {
		return "", fmt.Errorf("add password: %w", err)
	}

	var secretResult struct {
		SecretText string `json:"secretText"`
	}
	if err := json.Unmarshal(secretBody, &secretResult); err != nil {
		return "", fmt.Errorf("parse addPassword response: %w (status=%d)", err, statusCode)
	}
	if secretResult.SecretText == "" {
		return "", fmt.Errorf("addPassword returned empty secret (status=%d body=%s)", statusCode, string(secretBody))
	}

	logging.Logger.Debug().Str("objectId", objectID).Msg("New password credential added successfully")
	return secretResult.SecretText, nil
}

// ensureServicePrincipal ensures a service principal exists for the given appId.
func ensureServicePrincipal(ctx context.Context, client *http.Client, token, appID string) error {
	path := fmt.Sprintf("/servicePrincipals(appId='%s')", url.PathEscape(appID))
	_, statusCode, err := withGraphRetry(ctx, "upsertServicePrincipal", servicePrincipalUpsertRetryable, func() ([]byte, int, error) {
		return graphPatch(ctx, client, token, path, map[string]string{}, http.Header{"Prefer": {"create-if-missing"}})
	})
	if err != nil {
		return fmt.Errorf("upsert service principal: %w", err)
	}
	if statusCode != http.StatusCreated && statusCode != http.StatusNoContent {
		return fmt.Errorf("upsert service principal: unexpected status %d", statusCode)
	}

	logging.Logger.Info().Str("appId", appID).Int("statusCode", statusCode).Msg("Ensured service principal")
	return nil
}

// grantAdminConsent consents to the login app's delegated scopes on behalf of
// every user in the tenant.
//
// The tenant disables user consent, so the first sign-in to a freshly created
// app stops on "Permissions requested" and then "Need admin approval" — a
// screen no test account can get past. Recording the grants here is what the
// portal's "Grant admin consent" button does, so the consent screen never
// appears for the deployment's own app.
// createVenomK8sSecretFunc is the function used to create the K8s secret.
// It is a variable so tests can override it to avoid requiring a real cluster.
var createVenomK8sSecretFunc = createVenomK8sSecret

// EnsureVenomApp provisions a venom Entra app registration for OIDC integration tests.
// It:
// 1. Authenticates to Graph API using the parent app credentials.
// 2. Finds or creates an app registration named "venom-test-<namespace>".
// 3. Rotates the client secret (removes old ones, creates a fresh one).
// 4. Ensures a service principal exists for the app.
// 5. Creates/updates the venom-entra-credentials K8s secret in the namespace.
// 6. Returns the provisioned VenomApp — callers should use ExtraEnv for per-entry env vars.
//
// Returns the provisioned VenomApp or an error.
func EnsureVenomApp(ctx context.Context, opts Options) (*VenomApp, error) {
	if err := resolveOpts(&opts); err != nil {
		return nil, fmt.Errorf("entra: %w", err)
	}

	displayName := appDisplayName(opts.Namespace)
	client := httpClient(&opts)

	logging.Logger.Info().
		Str("namespace", opts.Namespace).
		Str("displayName", displayName).
		Str("directoryId", opts.DirectoryID).
		Str("parentClientId", opts.ClientID).
		Msg("Ensuring venom Entra app registration")

	// Step 1: Acquire bearer token for Graph API.
	token, err := acquireBearerToken(ctx, &opts)
	if err != nil {
		return nil, fmt.Errorf("entra: %w", err)
	}

	// Step 2: Find or create the app registration.
	appID, objectID, err := findApp(ctx, client, token, displayName)
	if err != nil {
		return nil, fmt.Errorf("entra: %w", err)
	}

	if appID != "" {
		portalURL := entraPortalAppURL(appID)
		logging.Logger.Info().Str("appId", appID).Str("objectId", objectID).Msg("Found existing venom app")
		logging.Logger.Debug().Str("portalURL", portalURL).Msg("View app in Azure portal")
	} else {
		logging.Logger.Info().Str("displayName", displayName).Msg("Creating new venom app registration")
		appID, objectID, err = createApp(ctx, client, token, displayName, opts.ClientID)
		if err != nil {
			return nil, fmt.Errorf("entra: %w", err)
		}
		portalURL := entraPortalAppURL(appID)
		logging.Logger.Info().
			Str("appId", appID).
			Str("objectId", objectID).
			Str("portalURL", portalURL).
			Msg("Created venom app in Entra")

		propogated := false
		attempts := 0
		successes := 0

		for !propogated && attempts < 45 && successes < 5 {
			foundAppID, foundObjID, findErr := findApp(ctx, client, token, displayName)
			if findErr != nil {
				logging.Logger.Warn().Err(findErr).Msg("Error while checking for app registration propagation")
			}
			if foundAppID != "" && foundObjID != "" {
				appID = foundAppID
				objectID = foundObjID
				propogated = true
				// success counter is used to ensure that the app is consistently found across multiple attempts, indicating that it has fully propagated through the system.
				successes++
			} else {
				successes = 0
			}
			attempts++
			time.Sleep(propagationSleepDuration)
		}

	}

	// Step 3: Rotate credentials to get a fresh client secret.
	secret, err := rotateCredentials(ctx, client, token, objectID)
	if err != nil {
		return nil, fmt.Errorf("entra: %w", err)
	}
	logging.Logger.Info().Msg("Venom app client secret created")

	// Step 4: Ensure service principal exists (required for token acquisition).
	if err := ensureServicePrincipal(ctx, client, token, appID); err != nil {
		return nil, fmt.Errorf("entra: %w", err)
	}

	// Step 5: Create/update the K8s secret (unless deferred).
	if opts.SkipK8sSecret {
		logging.Logger.Info().
			Str("namespace", opts.Namespace).
			Msg("Skipping K8s secret creation (SkipK8sSecret=true); caller must create it later via CreateVenomK8sSecret")
	} else {
		logging.Logger.Debug().
			Str("namespace", opts.Namespace).
			Str("kubeContext", opts.KubeContext).
			Str("secretName", "venom-entra-credentials").
			Msg("Creating/updating K8s secret with venom credentials")
		if err := createVenomK8sSecretFunc(ctx, opts.KubeContext, opts.Namespace, appID, secret, opts.ClientID); err != nil {
			return nil, fmt.Errorf("entra: create K8s secret: %w", err)
		}
		logging.Logger.Info().
			Str("namespace", opts.Namespace).
			Msg("Venom Entra credentials stored in K8s secret 'venom-entra-credentials'")
	}

	// Step 6: Return the provisioned app. Callers that need env vars for values
	// substitution should use RuntimeFlags.ExtraEnv (per-entry, merged into the
	// isolated env map by buildScenarioEnv) rather than process-global os.Setenv
	// to avoid races in parallel execution.
	logging.Logger.Info().
		Str("VENOM_CLIENT_ID", appID).
		Str("CONNECTORS_CLIENT_ID", opts.ClientID).
		Msg("Venom app provisioned — callers should set VENOM_CLIENT_ID and CONNECTORS_CLIENT_ID via ExtraEnv")

	return &VenomApp{
		AppID:        appID,
		ObjectID:     objectID,
		ClientSecret: secret,
	}, nil
}

// awaitAppPropagation polls Graph until a freshly created app registration is
// returned by a lookup on its display name. Graph creates the app on one
// replica and a PATCH against the other replicas 404s until it catches up.
func awaitAppPropagation(ctx context.Context, client *http.Client, token, displayName string, appID, objectID *string) error {
	for range 45 {
		foundAppID, foundObjID, err := findApp(ctx, client, token, displayName)
		if err != nil {
			logging.Logger.Warn().Err(err).Msg("Error while checking for app registration propagation")
		}
		if foundAppID != "" && foundObjID != "" {
			*appID = foundAppID
			*objectID = foundObjID
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(propagationSleepDuration):
		}
	}
	return fmt.Errorf("app registration %q did not propagate", displayName)
}

// loginAppDisplayName returns the deterministic Entra app display name of a
// deployment's own OIDC login app.
func loginAppDisplayName(namespace string) string {
	return "camunda-oidc-" + namespace
}

// LoginApp holds the result of provisioning a deployment's own Entra app
// registration for user sign-in and component M2M tokens.
type LoginApp struct {
	// AppID is the Entra application (client) ID of the login app.
	AppID string
	// ObjectID is the Entra object ID of the login app registration.
	ObjectID string
	// ClientSecret is the generated client secret for the login app.
	ClientSecret string
}

// configureLoginApp writes everything the deployment needs onto its own app
// registration.
//
// The redirect URIs are set to exactly the ones host needs. The app belongs to
// a single deployment, so the write owns both arrays outright: nothing is read
// first, nothing is merged, and no other writer can lose an update against it.
//
// The app is also its own token resource: every component requests
// "<clientId>/.default" and the orchestration cluster then validates the token
// against the issuer in the values, https://login.microsoftonline.com/<tid>/v2.0.
// A bare app registration has no identifierUris and no requestedAccessTokenVersion,
// so Entra issues a v1 token whose iss is https://sts.windows.net/<tid>/ and the
// cluster rejects it with 401. Setting the API block makes the app a v2 resource
// so ".default" resolves against it.
//
// Being the resource is not enough on its own. The authorize endpoint reads the
// list of valid resources off the CLIENT's requiredResourceAccess, so an app
// that points ".default" at itself and lists nothing is refused with
// AADSTS650057 ("List of valid resources from app registration:" and then
// nothing) before the browser is ever redirected -- the sign-in page simply
// comes back with no error on it. Hence the self scope and the matching self
// entry.
//
// That pairing alone used to stop the pool accounts on "Need admin approval":
// the tenant reserves consent for admins, so any permission in there raises a
// prompt they cannot clear. preAuthorizedApplications is what settles it. An
// app listed as a pre-authorized client of its own scope holds that delegated
// permission implicitly, so no consent is asked of the signing-in user and no
// grant has to be written -- which matters because the parent app cannot write
// one (it has no DelegatedPermissionGrant.ReadWrite.All).
//
// It takes two PATCHes: preAuthorizedApplications can only reference a scope id
// that already exists on the app, so the scope has to land first.
//
// The permission entry createApp seeds is dropped on the way past: its all-zero
// id is rejected by the authorize endpoint with AADSTS65006. openid, email and
// profile need no entry at all.
func configureLoginApp(ctx context.Context, client *http.Client, token, objectID, appID, host string) error {
	web, spa := buildRedirectURIs(host)
	scopeID := uuid.NewString()
	path := fmt.Sprintf("/applications/%s", objectID)

	resource := map[string]any{
		"web":            map[string]any{"redirectUris": web},
		"spa":            map[string]any{"redirectUris": spa},
		"identifierUris": []string{"api://" + appID},
		"api": map[string]any{
			"requestedAccessTokenVersion": 2,
			"oauth2PermissionScopes": []map[string]any{{
				"id":                      scopeID,
				"isEnabled":               true,
				"type":                    "User",
				"value":                   "user_impersonation",
				"adminConsentDisplayName": "Access Camunda",
				"adminConsentDescription": "Allow the application to access Camunda on behalf of the signed-in user.",
				"userConsentDisplayName":  "Access Camunda",
				"userConsentDescription":  "Allow the application to access Camunda on your behalf.",
			}},
		},
		"requiredResourceAccess": []map[string]any{{
			"resourceAppId":  appID,
			"resourceAccess": []map[string]any{{"id": scopeID, "type": "Scope"}},
		}},
	}
	if err := patchLoginApp(ctx, client, token, path, "configureLoginApp", resource); err != nil {
		return err
	}

	preAuthorized := map[string]any{
		"api": map[string]any{
			"requestedAccessTokenVersion": 2,
			"preAuthorizedApplications": []map[string]any{{
				"appId":                  appID,
				"delegatedPermissionIds": []string{scopeID},
			}},
		},
	}
	if err := patchLoginApp(ctx, client, token, path, "preAuthorizeLoginApp", preAuthorized); err != nil {
		return err
	}

	logging.Logger.Info().
		Str("host", host).
		Str("identifierUri", "api://"+appID).
		Int("webURIs", len(web)).
		Int("spaURIs", len(spa)).
		Msg("Configured the deployment's own login app")
	return nil
}

// patchLoginApp applies one PATCH to the login app registration, failing on any
// status withGraphRetry does not treat as transient.
func patchLoginApp(ctx context.Context, client *http.Client, token, path, step string, payload map[string]any) error {
	body, statusCode, err := withGraphRetry(ctx, step, nil, func() ([]byte, int, error) {
		return graphPatch(ctx, client, token, path, payload, nil)
	})
	if err != nil {
		return fmt.Errorf("%s: %w", step, err)
	}
	if statusCode >= 300 {
		return fmt.Errorf("%s failed (status=%d): %s", step, statusCode, string(body))
	}
	return nil
}

// awaitLoginAppResource blocks until Entra's token service resolves the app as
// its own token resource. The Graph PATCH that adds identifierUris, the
// user_impersonation scope and the self requiredResourceAccess entry lands on
// Graph immediately, but login.microsoftonline.com serves it from a separate
// replica. Until that replica catches up every sign-in redirect fails with
// AADSTS650057 ("Invalid resource ... List of valid resources from app
// registration: <parent>"), which the browser sees as a bounce back to the
// component's own login page. Probing the same token service the browser hits
// is the only check that proves the resource is live.
func awaitLoginAppResource(ctx context.Context, opts *Options, appID, secret string) error {
	tokenURL := fmt.Sprintf("%s/%s/oauth2/v2.0/token", loginBaseURL, opts.DirectoryID)
	data := url.Values{
		"client_id":     {appID},
		"scope":         {"api://" + appID + "/.default"},
		"client_secret": {secret},
		"grant_type":    {"client_credentials"},
	}
	client := httpClient(opts)
	deadline := time.Now().Add(loginAppResourceTimeout)
	var lastErr string
	for attempt := 1; time.Now().Before(deadline); attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(data.Encode()))
		if err != nil {
			return fmt.Errorf("create login app probe request: %w", err)
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err.Error()
		} else {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode < 300 {
				logging.Logger.Info().
					Str("appId", appID).
					Int("attempts", attempt).
					Msg("Entra resolves the login app as its own token resource")
				return nil
			}
			lastErr = string(body)
			logging.Logger.Debug().
				Int("attempt", attempt).
				Int("statusCode", resp.StatusCode).
				Msg("Login app is not resolvable as a token resource yet")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(loginAppResourceInterval):
		}
	}
	return fmt.Errorf("login app %s did not become resolvable as a token resource: %s", appID, lastErr)
}

// EnsureLoginApp provisions a deployment's own Entra app registration for
// OIDC sign-in, named "camunda-oidc-<namespace>". The caller substitutes its
// appId and secret for ENTRA_APP_CLIENT_ID / ENTRA_APP_CLIENT_SECRET so every
// component of this deployment authenticates against it.
//
// It exists because the shared QA app registration holds one redirectUris
// array for every CI deployment at once. Graph offers no conditional write on
// /applications, so two deployments registering concurrently both read the
// array, both write their own version, and the later write silently drops the
// earlier deployment's URIs — which then fails every sign-in with AADSTS50011.
// A per-namespace app has an array only this deployment writes.
func EnsureLoginApp(ctx context.Context, opts Options, ingressHost string) (*LoginApp, error) {
	if err := resolveOpts(&opts); err != nil {
		return nil, fmt.Errorf("entra: %w", err)
	}
	if ingressHost == "" {
		return nil, fmt.Errorf("entra: ingress host is required to provision the login app")
	}

	displayName := loginAppDisplayName(opts.Namespace)
	client := httpClient(&opts)

	logging.Logger.Info().
		Str("namespace", opts.Namespace).
		Str("displayName", displayName).
		Str("ingressHost", ingressHost).
		Msg("Ensuring the deployment's own Entra login app")

	token, err := acquireBearerToken(ctx, &opts)
	if err != nil {
		return nil, fmt.Errorf("entra: %w", err)
	}

	appID, objectID, err := findApp(ctx, client, token, displayName)
	if err != nil {
		return nil, fmt.Errorf("entra: %w", err)
	}
	if appID != "" {
		logging.Logger.Info().
			Str("appId", appID).
			Str("objectId", objectID).
			Str("portalURL", entraPortalAppURL(appID)).
			Msg("Reusing the existing login app for this namespace")
	} else {
		appID, objectID, err = createApp(ctx, client, token, displayName, opts.ClientID)
		if err != nil {
			return nil, fmt.Errorf("entra: %w", err)
		}
		logging.Logger.Info().
			Str("appId", appID).
			Str("objectId", objectID).
			Str("portalURL", entraPortalAppURL(appID)).
			Msg("Created the deployment's login app in Entra")
		if err := awaitAppPropagation(ctx, client, token, displayName, &appID, &objectID); err != nil {
			return nil, fmt.Errorf("entra: %w", err)
		}
	}

	if err := configureLoginApp(ctx, client, token, objectID, appID, ingressHost); err != nil {
		return nil, fmt.Errorf("entra: %w", err)
	}

	secret, err := rotateCredentials(ctx, client, token, objectID)
	if err != nil {
		return nil, fmt.Errorf("entra: %w", err)
	}
	if err := ensureServicePrincipal(ctx, client, token, appID); err != nil {
		return nil, fmt.Errorf("entra: %w", err)
	}
	if err := awaitLoginAppResource(ctx, &opts, appID, secret); err != nil {
		return nil, fmt.Errorf("entra: %w", err)
	}

	logging.Logger.Info().
		Str("ENTRA_APP_CLIENT_ID", appID).
		Msg("Login app provisioned — callers should override ENTRA_APP_CLIENT_ID/SECRET via ExtraEnv")

	return &LoginApp{AppID: appID, ObjectID: objectID, ClientSecret: secret}, nil
}

// CleanupLoginApp deletes a deployment's own Entra login app registration.
// Errors are logged but not returned — cleanup is best-effort.
func CleanupLoginApp(ctx context.Context, opts Options) {
	if err := resolveOpts(&opts); err != nil {
		logging.Logger.Warn().Err(err).Msg("entra login cleanup: invalid options, skipping")
		return
	}

	displayName := loginAppDisplayName(opts.Namespace)
	client := httpClient(&opts)

	token, err := acquireBearerToken(ctx, &opts)
	if err != nil {
		logging.Logger.Warn().Err(err).Msg("entra login cleanup: failed to obtain bearer token")
		return
	}

	_, objectID, err := findApp(ctx, client, token, displayName)
	if err != nil {
		logging.Logger.Warn().Err(err).Msg("entra login cleanup: failed to search for app")
		return
	}
	if objectID == "" {
		logging.Logger.Info().Str("displayName", displayName).Msg("No login app found, nothing to clean up")
		return
	}

	statusCode, err := graphDelete(ctx, client, token, fmt.Sprintf("/applications/%s", objectID))
	if err != nil {
		logging.Logger.Warn().Err(err).Str("objectId", objectID).Msg("entra login cleanup: delete request failed")
		return
	}
	if statusCode == 204 || statusCode == 404 {
		logging.Logger.Info().Str("displayName", displayName).Int("status", statusCode).Msg("Successfully deleted the login app")
	} else {
		logging.Logger.Warn().Str("displayName", displayName).Int("status", statusCode).Msg("Unexpected status when deleting the login app")
	}
}

// CleanupVenomApp deletes the venom Entra app registration for a namespace.
// Errors are logged but not returned — cleanup is best-effort.
func CleanupVenomApp(ctx context.Context, opts Options) {
	if err := resolveOpts(&opts); err != nil {
		logging.Logger.Warn().Err(err).Msg("entra cleanup: invalid options, skipping")
		return
	}

	displayName := appDisplayName(opts.Namespace)
	client := httpClient(&opts)

	logging.Logger.Info().
		Str("namespace", opts.Namespace).
		Str("displayName", displayName).
		Msg("Cleaning up venom Entra app registration")

	// Acquire bearer token.
	token, err := acquireBearerToken(ctx, &opts)
	if err != nil {
		logging.Logger.Warn().Err(err).Msg("entra cleanup: failed to obtain bearer token")
		return
	}

	// Find the app.
	_, objectID, err := findApp(ctx, client, token, displayName)
	if err != nil {
		logging.Logger.Warn().Err(err).Msg("entra cleanup: failed to search for app")
		return
	}
	if objectID == "" {
		logging.Logger.Info().Str("displayName", displayName).Msg("No Entra app found, nothing to clean up")
		return
	}

	// Delete the app.
	logging.Logger.Debug().Str("objectId", objectID).Str("displayName", displayName).Msg("Sending DELETE request for app registration")
	statusCode, err := graphDelete(ctx, client, token, fmt.Sprintf("/applications/%s", objectID))
	if err != nil {
		logging.Logger.Warn().Err(err).Str("objectId", objectID).Msg("entra cleanup: delete request failed")
		return
	}

	if statusCode == 204 || statusCode == 404 {
		logging.Logger.Info().Str("displayName", displayName).Int("status", statusCode).Msg("Successfully deleted Entra app")
	} else {
		logging.Logger.Warn().Str("displayName", displayName).Int("status", statusCode).Msg("Unexpected status when deleting Entra app")
	}
}

// createVenomK8sSecret creates or updates the venom-entra-credentials Opaque
// secret in the given namespace using server-side apply.
func createVenomK8sSecret(ctx context.Context, kubeContext, namespace, venomClientID, venomClientSecret, audience string) error {
	k8sClient, err := kube.NewClient("", kubeContext)
	if err != nil {
		return fmt.Errorf("create K8s client: %w", err)
	}

	secretData := map[string]string{
		"client-id":     venomClientID,
		"client-secret": venomClientSecret,
		"audience":      audience,
	}

	if err := k8sClient.EnsureOpaqueSecret(ctx, namespace, "venom-entra-credentials", secretData); err != nil {
		return fmt.Errorf("apply secret: %w", err)
	}
	return nil
}

// CreateVenomK8sSecret creates or updates the venom-entra-credentials K8s
// secret using the data from a previously provisioned VenomApp. This is
// intended for callers that used SkipK8sSecret=true in EnsureVenomApp and
// need to create the secret later (e.g., after the namespace exists).
func CreateVenomK8sSecret(ctx context.Context, kubeContext, namespace string, app *VenomApp, audience string) error {
	logging.Logger.Debug().
		Str("namespace", namespace).
		Str("kubeContext", kubeContext).
		Str("secretName", "venom-entra-credentials").
		Msg("Creating/updating deferred K8s secret with venom credentials")
	if err := createVenomK8sSecretFunc(ctx, kubeContext, namespace, app.AppID, app.ClientSecret, audience); err != nil {
		return fmt.Errorf("entra: create K8s secret: %w", err)
	}
	logging.Logger.Info().
		Str("namespace", namespace).
		Msg("Venom Entra credentials stored in K8s secret 'venom-entra-credentials'")
	return nil
}

// IsOIDCEntry returns true if the matrix entry uses Entra OIDC authentication
// (either via the Auth field or the Identity field).
//
// auth=="oidc" is the legacy signal for "external OIDC provider"; the only
// implementation that path covered historically was Entra. Now that Auth0 is a
// distinct provider with its own provisioning hook (auth0.IsAuth0Identity),
// identity=="auth0" must short-circuit so a single matrix entry doesn't
// double-provision (Entra venom app AND Auth0 clients).
func IsOIDCEntry(auth, identity string) bool {
	if identity == "auth0" {
		return false
	}
	return auth == "oidc" || identity == "oidc"
}

// ciDomainSuffix is the CI ingress domain suffix used to identify redirect
// URIs that belong to CI deployments and can be pruned.
const ciDomainSuffix = ".ci.distro.ultrawombat.com"

// RedirectURIOptions configures an UpdateRedirectURIs operation.
type RedirectURIOptions struct {
	// ObjectID is the Entra app registration object ID whose redirect URIs
	// should be updated. Falls back to ENTRA_APP_OBJECT_ID env var when empty.
	ObjectID string

	// IngressHost is the ingress hostname for the current deployment
	// (e.g., "my-ns.ci.distro.ultrawombat.com").
	IngressHost string

	// DirectoryID (tenant ID) for the Entra directory.
	// Falls back to ENTRA_APP_DIRECTORY_ID env var when empty.
	DirectoryID string

	// ClientID of the parent Entra app used to authenticate to Graph API.
	// Falls back to ENTRA_APP_CLIENT_ID env var when empty.
	ClientID string

	// ClientSecret of the parent Entra app for Graph API authentication.
	// Falls back to ENTRA_APP_CLIENT_SECRET env var when empty.
	ClientSecret string

	// HTTPClient is an optional HTTP client for making requests.
	// When nil, http.DefaultClient is used.
	HTTPClient *http.Client
}

// resolveRedirectOpts fills in empty RedirectURIOptions fields from environment
// variables.
func resolveRedirectOpts(opts *RedirectURIOptions) error {
	if opts.ObjectID == "" {
		opts.ObjectID = os.Getenv("ENTRA_APP_OBJECT_ID")
	}
	if opts.DirectoryID == "" {
		opts.DirectoryID = os.Getenv("ENTRA_APP_DIRECTORY_ID")
	}
	if opts.ClientID == "" {
		opts.ClientID = os.Getenv("ENTRA_APP_CLIENT_ID")
	}
	if opts.ClientSecret == "" {
		opts.ClientSecret = os.Getenv("ENTRA_APP_CLIENT_SECRET")
	}

	if opts.DirectoryID == "" {
		return fmt.Errorf("ENTRA_APP_DIRECTORY_ID is required (set RedirectURIOptions.DirectoryID or ENTRA_APP_DIRECTORY_ID env var)")
	}
	if opts.ClientID == "" {
		return fmt.Errorf("ENTRA_APP_CLIENT_ID is required (set RedirectURIOptions.ClientID or ENTRA_APP_CLIENT_ID env var)")
	}
	if opts.ClientSecret == "" {
		return fmt.Errorf("ENTRA_APP_CLIENT_SECRET is required (set RedirectURIOptions.ClientSecret or ENTRA_APP_CLIENT_SECRET env var)")
	}
	if opts.ObjectID == "" {
		return fmt.Errorf("ENTRA_APP_OBJECT_ID is required (set RedirectURIOptions.ObjectID or ENTRA_APP_OBJECT_ID env var)")
	}
	if opts.IngressHost == "" {
		return fmt.Errorf("ingress host is required")
	}
	return nil
}

// webRedirectPaths are the path suffixes appended to the ingress host for
// web (server-side) redirect URIs.
var webRedirectPaths = []string{
	"/identity/auth/login-callback",
	"/operate/identity-callback",
	"/optimize/api/authentication/callback",
	"/tasklist/identity-callback",
	"/orchestration/sso-callback",
}

// spaRedirectPaths are the path suffixes appended to the ingress host for
// SPA (single-page application) redirect URIs.
var spaRedirectPaths = []string{
	"/modeler/login-callback",
	"/",
}

// buildRedirectURIs builds the web and SPA redirect URI lists for a given host.
func buildRedirectURIs(host string) (web []string, spa []string) {
	for _, p := range webRedirectPaths {
		web = append(web, "https://"+host+p)
	}
	for _, p := range spaRedirectPaths {
		spa = append(spa, "https://"+host+p)
	}
	return web, spa
}

// isValidURI returns true if the URI is a well-formed redirect URI.
// It rejects URIs with trailing commas (data corruption) and empty strings.
func isValidURI(uri string) bool {
	if uri == "" {
		return false
	}
	if strings.HasSuffix(uri, ",") {
		return false
	}
	return true
}

// uriHost returns the lowercased hostname of uri, or "" if it has none.
func uriHost(uri string) string {
	u, err := url.Parse(uri)
	if err != nil {
		return ""
	}
	return strings.ToLower(u.Hostname())
}

// isHostURI returns true if the URI's hostname is exactly the given host.
func isHostURI(uri, host string) bool {
	h := uriHost(uri)
	return h != "" && h == strings.ToLower(host)
}

// maxRedirectURIs is Entra's limit on redirect URIs per app registration,
// counted across all platform types.
// https://learn.microsoft.com/en-us/entra/identity-platform/reply-url#maximum-number-of-redirect-uris
const maxRedirectURIs = 256

// isPrunableURI reports whether uri's hostname is a CI host other than host.
func isPrunableURI(uri, host string) bool {
	return strings.HasSuffix(uriHost(uri), ciDomainSuffix) && !isHostURI(uri, host)
}

// oldestPrunableHost returns the host of the first prunable URI in web, then
// in spa. Registrations append, so list order is registration order.
func oldestPrunableHost(web, spa []string, host string) string {
	for _, list := range [][]string{web, spa} {
		for _, uri := range list {
			if isPrunableURI(uri, host) {
				return uriHost(uri)
			}
		}
	}
	return ""
}

// capRedirectURIs evicts whole CI deployments, oldest first and from both
// lists at once, until web, spa and reserved together fit maxRedirectURIs.
// Non-CI URIs and the current host's URIs are never dropped.
func capRedirectURIs(web, spa []string, reserved int, host string) ([]string, []string) {
	for len(web)+len(spa)+reserved > maxRedirectURIs {
		evict := oldestPrunableHost(web, spa, host)
		if evict == "" {
			return web, spa
		}
		logging.Logger.Debug().Str("host", evict).Msg("Pruning oldest CI deployment's redirect URIs")
		drop := func(uri string) bool { return isHostURI(uri, evict) }
		web = slices.DeleteFunc(web, drop)
		spa = slices.DeleteFunc(spa, drop)
	}
	return web, spa
}

// filterRedirectURIs filters a list of redirect URIs:
// - Removes invalid URIs (empty, trailing commas).
// - Removes the current host's URIs, which newURIs re-adds.
// - Preserves every other host's URIs, other CI deployments included.
// - Appends the new URIs for the current deployment.
// - Deduplicates the result. capRedirectURIs enforces the Entra limit.
func filterRedirectURIs(existing []string, newURIs []string, host string) []string {
	seen := make(map[string]bool)
	var result []string

	for _, uri := range existing {
		if !isValidURI(uri) {
			logging.Logger.Debug().Str("uri", uri).Msg("Removing invalid redirect URI")
			continue
		}
		if isHostURI(uri, host) {
			logging.Logger.Debug().Str("uri", uri).Msg("Replacing redirect URI of the current host")
			continue
		}
		if !seen[uri] {
			seen[uri] = true
			result = append(result, uri)
		}
	}

	for _, uri := range newURIs {
		if !seen[uri] {
			seen[uri] = true
			result = append(result, uri)
		}
	}

	return result
}

type redirectURIApp struct {
	Web struct {
		RedirectURIs []string `json:"redirectUris"`
	} `json:"web"`
	Spa struct {
		RedirectURIs []string `json:"redirectUris"`
	} `json:"spa"`
	PublicClient struct {
		RedirectURIs []string `json:"redirectUris"`
	} `json:"publicClient"`
}

// graphTarget addresses one Graph resource with an authenticated client.
type graphTarget struct {
	client *http.Client
	token  string
	path   string
}

func fetchRedirectURIApp(ctx context.Context, g graphTarget) (redirectURIApp, error) {
	var app redirectURIApp
	body, err := graphGet(ctx, g.client, g.token, g.path)
	if err != nil {
		return app, fmt.Errorf("entra: fetch app registration: %w", err)
	}
	if err := json.Unmarshal(body, &app); err != nil {
		return app, fmt.Errorf("entra: parse app registration: %w", err)
	}
	return app, nil
}

func patchRedirectURIs(ctx context.Context, g graphTarget, app redirectURIApp, host string) error {
	newWebURIs, newSpaURIs := buildRedirectURIs(host)
	web := filterRedirectURIs(app.Web.RedirectURIs, newWebURIs, host)
	spa := filterRedirectURIs(app.Spa.RedirectURIs, newSpaURIs, host)
	web, spa = capRedirectURIs(web, spa, len(app.PublicClient.RedirectURIs), host)

	logging.Logger.Info().
		Int("existingWebURIs", len(app.Web.RedirectURIs)).
		Int("existingSpaURIs", len(app.Spa.RedirectURIs)).
		Int("finalWebURIs", len(web)).
		Int("finalSpaURIs", len(spa)).
		Strs("newWebURIs", newWebURIs).
		Strs("newSpaURIs", newSpaURIs).
		Msg("Computed final redirect URI lists")

	payload := map[string]any{
		"web": map[string]any{"redirectUris": web},
		"spa": map[string]any{"redirectUris": spa},
	}
	body, statusCode, err := graphPatch(ctx, g.client, g.token, g.path, payload, nil)
	if err != nil {
		return fmt.Errorf("entra: PATCH redirect URIs: %w", err)
	}
	if statusCode >= 300 {
		return fmt.Errorf("entra: PATCH redirect URIs failed (status=%d): %s", statusCode, string(body))
	}
	return nil
}

// hasHostRedirectURIs reports whether app holds every redirect URI of host.
func hasHostRedirectURIs(app redirectURIApp, host string) bool {
	newWebURIs, newSpaURIs := buildRedirectURIs(host)
	for _, uri := range newWebURIs {
		if !slices.Contains(app.Web.RedirectURIs, uri) {
			return false
		}
	}
	for _, uri := range newSpaURIs {
		if !slices.Contains(app.Spa.RedirectURIs, uri) {
			return false
		}
	}
	return true
}

// awaitHostRedirectURIs reads the app back until it contains host's redirect
// URIs or redirectVerifyReads reads have passed, and returns the last read.
func awaitHostRedirectURIs(ctx context.Context, g graphTarget, host string) (redirectURIApp, bool, error) {
	var app redirectURIApp
	for range redirectVerifyReads {
		if err := waitRedirectVerify(ctx); err != nil {
			return app, false, fmt.Errorf("entra: verify redirect URIs: %w", err)
		}
		var err error
		if app, err = fetchRedirectURIApp(ctx, g); err != nil {
			return app, false, err
		}
		if hasHostRedirectURIs(app, host) {
			return app, true, nil
		}
	}
	return app, false, nil
}

func waitRedirectVerify(ctx context.Context) error {
	wait := redirectVerifyDelay
	if redirectVerifyJitter > 0 {
		wait += time.Duration(rand.Int63n(int64(redirectVerifyJitter)))
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(wait):
		return nil
	}
}

// UpdateRedirectURIs updates the redirect URIs on the parent Entra app
// registration. It:
//  1. Authenticates to Graph API using the parent app credentials.
//  2. Fetches the current web, SPA and public-client redirect URIs.
//  3. Replaces the current host's URIs and removes invalid/malformed URIs.
//  4. Adds the redirect URIs for the current deployment's ingress host.
//  5. PATCHes the updated URI lists back to the app registration.
//  6. Reads the app back up to redirectVerifyReads times, a jittered wait
//     apart, and repeats from step 3 from the last read when none contains this
//     host's URIs, failing after redirectVerifyAttempts writes.
func UpdateRedirectURIs(ctx context.Context, opts RedirectURIOptions) error {
	if err := resolveRedirectOpts(&opts); err != nil {
		return fmt.Errorf("entra: %w", err)
	}

	client := opts.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}

	logging.Logger.Info().
		Str("objectId", opts.ObjectID).
		Str("ingressHost", opts.IngressHost).
		Msg("Updating Entra app redirect URIs")

	authOpts := &Options{
		DirectoryID:  opts.DirectoryID,
		ClientID:     opts.ClientID,
		ClientSecret: opts.ClientSecret,
		Namespace:    "redirect-uri-update", // placeholder; not used for K8s
		HTTPClient:   opts.HTTPClient,
	}
	token, err := acquireBearerToken(ctx, authOpts)
	if err != nil {
		return fmt.Errorf("entra: %w", err)
	}

	g := graphTarget{client: client, token: token, path: fmt.Sprintf("/applications/%s", opts.ObjectID)}
	app, err := fetchRedirectURIApp(ctx, g)
	if err != nil {
		return err
	}

	for attempt := 1; ; attempt++ {
		if err := patchRedirectURIs(ctx, g, app, opts.IngressHost); err != nil {
			return err
		}
		var found bool
		if app, found, err = awaitHostRedirectURIs(ctx, g, opts.IngressHost); err != nil {
			return err
		}
		if found {
			logging.Logger.Info().
				Str("objectId", opts.ObjectID).
				Int("attempt", attempt).
				Msg("Successfully updated Entra app redirect URIs")
			return nil
		}
		if attempt == redirectVerifyAttempts {
			return fmt.Errorf("entra: redirect URIs of %s were removed by a concurrent update %d times in a row",
				opts.IngressHost, attempt)
		}
		logging.Logger.Warn().
			Str("ingressHost", opts.IngressHost).
			Int("attempt", attempt).
			Msg("Redirect URIs removed by a concurrent update, rewriting")
	}
}
