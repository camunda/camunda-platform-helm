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
	"regexp"
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
	if !namespacePattern.MatchString(opts.Namespace) {
		return fmt.Errorf("namespace %q is not a valid DNS-1123 label", opts.Namespace)
	}
	return nil
}

var namespacePattern = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]{0,61}[a-z0-9])?$`)

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

	path := fmt.Sprintf("/applications?$filter=displayName%%20eq%%20'%s'", url.PathEscape(strings.ReplaceAll(displayName, "'", "''")))
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

// graphAppID is the well-known application id of Microsoft Graph.
const graphAppID = "00000003-0000-0000-c000-000000000000"

// loginAppDelegatedScopes are the delegated permissions a sign-in needs: the
// OIDC trio from Microsoft Graph, and the app's own scope, which every
// component asks for as "<clientId>/.default".
const (
	graphSignInScopes = "openid profile email"
	loginAppSelfScope = "user_impersonation"
)

// loginAppRoleValue is the app role each login app exposes to the parent app.
const loginAppRoleValue = "camunda.api.access"

// loginAppRoleID derives a stable role id from the app id.
func loginAppRoleID(appID string) string {
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte("camunda-login-app-role:"+appID)).String()
}

// loginAppScopeID derives a stable user_impersonation scope id from the app id.
func loginAppScopeID(appID string) string {
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte("camunda-login-app-scope:"+appID)).String()
}

// appRoleAssignRetryable treats a duplicate assignment as terminal and retries
// the 400 Graph returns while the login app's new role is not yet replicated.
func appRoleAssignRetryable(statusCode int, body []byte, err error) bool {
	if isDuplicateAppRoleAssignment(statusCode, body) {
		return false
	}
	if statusCode == http.StatusBadRequest &&
		bytes.Contains(body, []byte("Permission being assigned was not found")) {
		return true
	}
	return defaultRetryable(statusCode, body, err)
}

func isDuplicateAppRoleAssignment(statusCode int, body []byte) bool {
	if statusCode != http.StatusBadRequest && statusCode != http.StatusConflict {
		return false
	}
	return bytes.Contains(body, []byte("Permission being assigned already exists"))
}

// assignAPIAppRole assigns the login app's API role to the parent app's service
// principal.
func assignAPIAppRole(ctx context.Context, client *http.Client, token, parentClientID string, app loginAppRef) error {
	appID, roleID := app.appID, app.roleID
	resourceSPID, err := servicePrincipalID(ctx, client, token, appID)
	if err != nil {
		return fmt.Errorf("assign api app role: %w", err)
	}
	principalSPID, err := servicePrincipalID(ctx, client, token, parentClientID)
	if err != nil {
		return fmt.Errorf("assign api app role: %w", err)
	}

	path := fmt.Sprintf("/servicePrincipals/%s/appRoleAssignedTo", resourceSPID)
	payload := map[string]any{
		"principalId": principalSPID,
		"resourceId":  resourceSPID,
		"appRoleId":   roleID,
	}
	body, statusCode, err := withGraphRetry(ctx, "assignAPIAppRole", appRoleAssignRetryable, func() ([]byte, int, error) {
		return graphPost(ctx, client, token, path, payload)
	})
	if isDuplicateAppRoleAssignment(statusCode, body) {
		logging.Logger.Debug().Str("appId", appID).Msg("API app role already assigned to the parent app")
		return nil
	}
	if err != nil {
		return fmt.Errorf("assign api app role: %w", err)
	}
	if statusCode >= 300 {
		return fmt.Errorf("assign api app role failed (status=%d): %s", statusCode, string(body))
	}

	logging.Logger.Info().
		Str("appId", appID).
		Str("role", loginAppRoleValue).
		Msg("Assigned the login app's API role to the parent app")
	return nil
}

// permissionGrantRetryable treats a duplicate grant as terminal and retries the
// 400 Graph returns for a not-yet-replicated service principal.
func permissionGrantRetryable(statusCode int, body []byte, err error) bool {
	if isDuplicatePermissionGrant(statusCode, body) {
		return false
	}
	if statusCode == http.StatusBadRequest &&
		bytes.Contains(body, []byte("Specified resourceId was not found")) {
		return true
	}
	return defaultRetryable(statusCode, body, err)
}

func isDuplicatePermissionGrant(statusCode int, body []byte) bool {
	if statusCode != http.StatusBadRequest && statusCode != http.StatusConflict {
		return false
	}
	return bytes.Contains(body, []byte("Permission entry already exists"))
}

// grantAdminConsent records AllPrincipals consent for the login app's delegated
// scopes. Requires DelegatedPermissionGrant.ReadWrite.All on the parent app.
func grantAdminConsent(ctx context.Context, client *http.Client, token, appID string) error {
	clientSPID, err := servicePrincipalID(ctx, client, token, appID)
	if err != nil {
		return fmt.Errorf("grant admin consent: %w", err)
	}
	graphSPID, err := servicePrincipalID(ctx, client, token, graphAppID)
	if err != nil {
		return fmt.Errorf("grant admin consent: %w", err)
	}

	for _, grant := range []struct {
		resourceID string
		scope      string
	}{
		{graphSPID, graphSignInScopes},
		{clientSPID, loginAppSelfScope},
	} {
		payload := map[string]any{
			"clientId":    clientSPID,
			"consentType": "AllPrincipals",
			"resourceId":  grant.resourceID,
			"scope":       grant.scope,
		}
		body, statusCode, err := withGraphRetry(ctx, "grantAdminConsent", permissionGrantRetryable, func() ([]byte, int, error) {
			return graphPost(ctx, client, token, "/oauth2PermissionGrants", payload)
		})
		if isDuplicatePermissionGrant(statusCode, body) {
			if err := ensureGrantScopes(ctx, client, token, permissionGrant{clientID: clientSPID, resourceID: grant.resourceID, scope: grant.scope}); err != nil {
				return fmt.Errorf("grant admin consent for %q: %w", grant.scope, err)
			}
			continue
		}
		if err != nil {
			return fmt.Errorf("grant admin consent for %q: %w", grant.scope, err)
		}
		if statusCode >= 300 {
			return fmt.Errorf("grant admin consent for %q failed (status=%d): %s", grant.scope, statusCode, string(body))
		}
	}

	logging.Logger.Info().
		Str("appId", appID).
		Str("scopes", graphSignInScopes+" "+loginAppSelfScope).
		Msg("Consented the login app for all principals")
	return nil
}

// permissionGrant names an AllPrincipals delegated grant by client and resource
// service principal ids and its space-separated scopes.
type permissionGrant struct {
	clientID   string
	resourceID string
	scope      string
}

// ensureGrantScopes adds any of grant.scope's entries missing from the existing
// AllPrincipals grant of grant.clientID on grant.resourceID.
func ensureGrantScopes(ctx context.Context, client *http.Client, token string, grant permissionGrant) error {
	clientSPID, resourceID, scope := grant.clientID, grant.resourceID, grant.scope
	filter := fmt.Sprintf("clientId eq '%s' and resourceId eq '%s' and consentType eq 'AllPrincipals'", clientSPID, resourceID)
	body, err := graphGet(ctx, client, token, "/oauth2PermissionGrants?$filter="+strings.ReplaceAll(url.QueryEscape(filter), "+", "%20"))
	if err != nil {
		return fmt.Errorf("read existing grant: %w", err)
	}
	var result struct {
		Value []struct {
			ID    string `json:"id"`
			Scope string `json:"scope"`
		} `json:"value"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("decode existing grant: %w", err)
	}
	if len(result.Value) == 0 {
		return fmt.Errorf("graph reported a duplicate grant but returned none for client %s on %s", clientSPID, resourceID)
	}
	existing := result.Value[0]
	have := strings.Fields(existing.Scope)
	merged := have
	for want := range strings.FieldsSeq(scope) {
		if !slices.Contains(merged, want) {
			merged = append(merged, want)
		}
	}
	if len(merged) == len(have) {
		logging.Logger.Debug().Str("grantId", existing.ID).Str("scope", existing.Scope).Msg("Admin consent already granted")
		return nil
	}
	newScope := strings.Join(merged, " ")
	body, statusCode, err := withGraphRetry(ctx, "updateGrantScopes", nil, func() ([]byte, int, error) {
		return graphPatch(ctx, client, token, "/oauth2PermissionGrants/"+url.PathEscape(existing.ID), map[string]string{"scope": newScope}, nil)
	})
	if err != nil {
		return fmt.Errorf("update grant scopes: %w", err)
	}
	if statusCode >= 300 {
		return fmt.Errorf("update grant scopes failed (status=%d): %s", statusCode, string(body))
	}
	logging.Logger.Info().Str("grantId", existing.ID).Str("scope", newScope).Msg("Extended the existing admin consent grant")
	return nil
}

// servicePrincipalID resolves an application id to its service principal object
// id, retrying 404s.
func servicePrincipalID(ctx context.Context, client *http.Client, token, appID string) (string, error) {
	path := fmt.Sprintf("/servicePrincipals(appId='%s')?$select=id", url.PathEscape(appID))
	body, statusCode, err := withGraphRetry(ctx, "servicePrincipalID", servicePrincipalUpsertRetryable, func() ([]byte, int, error) {
		return graphGetStatus(ctx, client, token, path)
	})
	if err != nil {
		return "", fmt.Errorf("look up service principal for %s: %w", appID, err)
	}
	if statusCode >= 300 {
		return "", fmt.Errorf("look up service principal for %s failed (status=%d): %s", appID, statusCode, string(body))
	}
	var sp struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &sp); err != nil {
		return "", fmt.Errorf("decode service principal for %s: %w", appID, err)
	}
	if sp.ID == "" {
		return "", fmt.Errorf("service principal for %s has no id", appID)
	}
	return sp.ID, nil
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

		if foundAppID, foundObjID, waitErr := awaitAppPropagation(ctx, client, token, displayName); waitErr != nil {
			logging.Logger.Warn().Err(waitErr).Msg("Continuing before the venom app registration propagated")
		} else {
			appID, objectID = foundAppID, foundObjID
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

// awaitAppPropagation polls findApp until displayName resolves, then writes the
// found ids to appID and objectID.
func awaitAppPropagation(ctx context.Context, client *http.Client, token, displayName string) (string, string, error) {
	for range 45 {
		foundAppID, foundObjID, err := findApp(ctx, client, token, displayName)
		if err != nil {
			logging.Logger.Warn().Err(err).Msg("Error while checking for app registration propagation")
		}
		if foundAppID != "" && foundObjID != "" {
			return foundAppID, foundObjID, nil
		}
		select {
		case <-ctx.Done():
			return "", "", ctx.Err()
		case <-time.After(propagationSleepDuration):
		}
	}
	return "", "", fmt.Errorf("app registration %q did not propagate", displayName)
}

// CleanupVenomApp deletes the venom Entra app registration for a namespace.
// Errors are logged but not returned — cleanup is best-effort.
func CleanupVenomApp(ctx context.Context, opts Options) {
	cleanupApp(ctx, opts, appDisplayName, "venom")
}

// CleanupLoginApp deletes a deployment's own Entra login app registration.
// Errors are logged but not returned — cleanup is best-effort.
func CleanupLoginApp(ctx context.Context, opts Options) {
	cleanupApp(ctx, opts, loginAppDisplayName, "login")
}

// cleanupApp deletes the app registration displayName(opts.Namespace) names.
// Errors are logged, not returned.
func cleanupApp(ctx context.Context, opts Options, displayName func(string) string, kind string) {
	log := logging.Logger.With().Str("app", kind).Logger()
	if err := resolveOpts(&opts); err != nil {
		log.Warn().Err(err).Msg("entra cleanup: invalid options, skipping")
		return
	}

	name := displayName(opts.Namespace)
	client := httpClient(&opts)
	log.Info().Str("namespace", opts.Namespace).Str("displayName", name).Msg("Cleaning up Entra app registration")

	token, err := acquireBearerToken(ctx, &opts)
	if err != nil {
		log.Warn().Err(err).Msg("entra cleanup: failed to obtain bearer token")
		return
	}

	_, objectID, err := findApp(ctx, client, token, name)
	if err != nil {
		log.Warn().Err(err).Msg("entra cleanup: failed to search for app")
		return
	}
	if objectID == "" {
		log.Info().Str("displayName", name).Msg("No Entra app found, nothing to clean up")
		return
	}

	log.Debug().Str("objectId", objectID).Str("displayName", name).Msg("Sending DELETE request for app registration")
	statusCode, err := graphDelete(ctx, client, token, fmt.Sprintf("/applications/%s", objectID))
	if err != nil {
		log.Warn().Err(err).Str("objectId", objectID).Msg("entra cleanup: delete request failed")
		return
	}
	if statusCode == 204 || statusCode == 404 {
		log.Info().Str("displayName", name).Int("status", statusCode).Msg("Successfully deleted Entra app")
	} else {
		log.Warn().Str("displayName", name).Int("status", statusCode).Msg("Unexpected status when deleting Entra app")
	}
}

// loginAppRef identifies a login app registration and, once configured, its API role.
type loginAppRef struct {
	appID    string
	objectID string
	roleID   string
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

// configureLoginApp sets the login app's redirect URIs to exactly those host
// needs, exposes the app as a v2 token resource with one delegated scope and one
// application role, and drops the all-zero permission entry createApp seeds.
// Returns the role id.
func configureLoginApp(ctx context.Context, client *http.Client, token string, app loginAppRef, host string) (string, error) {
	objectID, appID := app.objectID, app.appID
	web, spa := buildRedirectURIs(host)
	scopeID := loginAppScopeID(appID)
	roleID := loginAppRoleID(appID)
	path := fmt.Sprintf("/applications/%s", objectID)

	resource := map[string]any{
		"web":            map[string]any{"redirectUris": web},
		"spa":            map[string]any{"redirectUris": spa},
		"identifierUris": []string{"api://" + appID},
		"appRoles": []map[string]any{{
			"id":                 roleID,
			"isEnabled":          true,
			"allowedMemberTypes": []string{"Application"},
			"value":              loginAppRoleValue,
			"displayName":        "Call Camunda APIs",
			"description":        "Allow an application to call this deployment's Camunda APIs.",
		}},
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
	if err := patchLoginApp(ctx, client, token, path, resource); err != nil {
		return "", err
	}

	logging.Logger.Info().
		Str("host", host).
		Str("identifierUri", "api://"+appID).
		Int("webURIs", len(web)).
		Int("spaURIs", len(spa)).
		Msg("Configured the deployment's own login app")
	return roleID, nil
}

// patchLoginApp applies one PATCH to the login app registration, failing on any
// status withGraphRetry does not treat as transient.
func patchLoginApp(ctx context.Context, client *http.Client, token, path string, payload map[string]any) error {
	const step = "configureLoginApp"
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

// awaitLoginAppResource polls the token endpoint with a client-credentials
// request for "<appID>/.default" until Entra issues a token for it.
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
		if appID, objectID, err = awaitAppPropagation(ctx, client, token, displayName); err != nil {
			return nil, fmt.Errorf("entra: %w", err)
		}
	}

	roleID, err := configureLoginApp(ctx, client, token, loginAppRef{appID: appID, objectID: objectID}, ingressHost)
	if err != nil {
		return nil, fmt.Errorf("entra: %w", err)
	}

	secret, err := rotateCredentials(ctx, client, token, objectID)
	if err != nil {
		return nil, fmt.Errorf("entra: %w", err)
	}
	if err := ensureServicePrincipal(ctx, client, token, appID); err != nil {
		return nil, fmt.Errorf("entra: %w", err)
	}
	if err := grantAdminConsent(ctx, client, token, appID); err != nil {
		return nil, fmt.Errorf("entra: %w", err)
	}
	if err := assignAPIAppRole(ctx, client, token, opts.ClientID, loginAppRef{appID: appID, roleID: roleID}); err != nil {
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
