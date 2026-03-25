// Package entra provisions and cleans up Microsoft Entra ID (Azure AD)
// app registrations for OIDC integration tests. This is the canonical
// implementation, called by both the "deploy-camunda entra" CLI subcommand
// and the matrix runner to execute OIDC scenarios end-to-end.
package entra

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"scripts/camunda-core/pkg/kube"
	"scripts/camunda-core/pkg/logging"
	"strings"
	"time"
)

// graphBaseURL is the Microsoft Graph API base URL. It is a variable so tests
// can override it with a local httptest server URL.
var graphBaseURL = "https://graph.microsoft.com/v1.0"

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
func entraPortalAppURL(directoryID, appID string) string {
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

const (
	graphMaxAttempts = 10
)

// graphBackoff waits an exponentially increasing duration before a retry.
// Attempt 1 → 2s, attempt 2 → 4s. Respects context cancellation.
func graphBackoff(ctx context.Context, attempt int) error {
	d := time.Duration(1<<uint(attempt)) * time.Second
	logging.Logger.Warn().
		Int("attempt", attempt).
		Int("maxAttempts", graphMaxAttempts).
		Dur("backoff", d).
		Msg("Graph API got 409 ConcurrencyViolation, retrying")
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}

// graphGet performs a GET request to the Graph API with retry on 409.
func graphGet(ctx context.Context, client *http.Client, token, path string) ([]byte, error) {
	reqURL := graphBaseURL + path
	logging.Logger.Debug().Str("method", "GET").Str("url", reqURL).Msg("Graph API request")

	var (
		body       []byte
		statusCode int
	)
	for attempt := 1; attempt <= graphMaxAttempts; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		body, err = io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		statusCode = resp.StatusCode
		logging.Logger.Debug().Int("statusCode", statusCode).Int("bodyLen", len(body)).Msg("Graph API GET response")

		if statusCode != 409 || attempt == graphMaxAttempts {
			break
		}
		if err := graphBackoff(ctx, attempt); err != nil {
			return nil, err
		}
	}
	return body, nil
}

// graphPost performs a POST request to the Graph API with retry on 409.
func graphPost(ctx context.Context, client *http.Client, token, path string, payload interface{}) ([]byte, int, error) {
	var rawPayload []byte
	if payload != nil {
		var err error
		rawPayload, err = json.Marshal(payload)
		if err != nil {
			return nil, 0, fmt.Errorf("marshal payload: %w", err)
		}
	}

	reqURL := graphBaseURL + path
	logging.Logger.Debug().Str("method", "POST").Str("url", reqURL).Msg("Graph API request")

	var (
		body       []byte
		statusCode int
	)
	for attempt := 1; attempt <= graphMaxAttempts; attempt++ {
		var bodyReader io.Reader
		if rawPayload != nil {
			bodyReader = strings.NewReader(string(rawPayload))
		}
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
		body, err = io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, 0, err
		}
		statusCode = resp.StatusCode
		logging.Logger.Debug().Int("statusCode", statusCode).Int("bodyLen", len(body)).Msg("Graph API POST response")

		if statusCode != 409 || attempt == graphMaxAttempts {
			break
		}
		if err := graphBackoff(ctx, attempt); err != nil {
			return nil, 0, err
		}
	}
	return body, statusCode, nil
}

// graphPatch performs a PATCH request to the Graph API with retry on 409.
func graphPatch(ctx context.Context, client *http.Client, token, path string, payload interface{}) ([]byte, int, error) {
	var rawPayload []byte
	if payload != nil {
		var err error
		rawPayload, err = json.Marshal(payload)
		if err != nil {
			return nil, 0, fmt.Errorf("marshal payload: %w", err)
		}
	}

	reqURL := graphBaseURL + path
	logging.Logger.Debug().Str("method", "PATCH").Str("url", reqURL).Msg("Graph API request")

	var (
		body       []byte
		statusCode int
	)
	for attempt := 1; attempt <= graphMaxAttempts; attempt++ {
		var bodyReader io.Reader
		if rawPayload != nil {
			bodyReader = strings.NewReader(string(rawPayload))
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPatch, reqURL, bodyReader)
		if err != nil {
			return nil, 0, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return nil, 0, err
		}
		body, err = io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, 0, err
		}
		statusCode = resp.StatusCode
		logging.Logger.Debug().Int("statusCode", statusCode).Int("bodyLen", len(body)).Msg("Graph API PATCH response")

		if statusCode != 409 || attempt == graphMaxAttempts {
			break
		}
		if err := graphBackoff(ctx, attempt); err != nil {
			return nil, 0, err
		}
	}
	return body, statusCode, nil
}

// graphDelete performs a DELETE request to the Graph API with retry on 409.
func graphDelete(ctx context.Context, client *http.Client, token, path string) (int, error) {
	reqURL := graphBaseURL + path
	logging.Logger.Debug().Str("method", "DELETE").Str("url", reqURL).Msg("Graph API request")

	var statusCode int
	for attempt := 1; attempt <= graphMaxAttempts; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodDelete, reqURL, nil)
		if err != nil {
			return 0, err
		}
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := client.Do(req)
		if err != nil {
			return 0, err
		}
		io.ReadAll(resp.Body) //nolint:errcheck // drain body
		resp.Body.Close()
		statusCode = resp.StatusCode
		logging.Logger.Debug().Int("statusCode", statusCode).Msg("Graph API DELETE response")

		if statusCode != 409 || attempt == graphMaxAttempts {
			break
		}
		if err := graphBackoff(ctx, attempt); err != nil {
			return 0, err
		}
	}
	return statusCode, nil
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

	payload := map[string]interface{}{
		"displayName":    displayName,
		"signInAudience": "AzureADMyOrg",
		"requiredResourceAccess": []map[string]interface{}{
			{
				"resourceAppId": parentClientID,
				"resourceAccess": []map[string]interface{}{
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
	body, err := graphGet(ctx, client, token, fmt.Sprintf("/applications/%s", objectID))
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

	// Add a new client secret.
	logging.Logger.Debug().Str("objectId", objectID).Msg("Adding new password credential")
	addPayload := map[string]interface{}{
		"passwordCredential": map[string]string{
			"displayName": "integration-test-secret",
			"endDateTime": "2099-12-31T00:00:00Z",
		},
	}

	secretBody, statusCode, err := graphPost(ctx, client, token, fmt.Sprintf("/applications/%s/addPassword", objectID), addPayload)
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
	logging.Logger.Debug().Str("appId", appID).Msg("Checking for existing service principal")

	// Check if service principal already exists.
	path := fmt.Sprintf("/servicePrincipals?$filter=appId%%20eq%%20'%s'", url.PathEscape(appID))
	body, err := graphGet(ctx, client, token, path)
	if err != nil {
		return fmt.Errorf("search service principal: %w", err)
	}

	var spResult struct {
		Value []struct {
			ID string `json:"id"`
		} `json:"value"`
	}
	if err := json.Unmarshal(body, &spResult); err != nil {
		return fmt.Errorf("parse service principal search: %w", err)
	}

	if len(spResult.Value) > 0 {
		logging.Logger.Debug().Str("appId", appID).Str("spId", spResult.Value[0].ID).Msg("Service principal already exists")
		return nil
	}

	// Create service principal.
	logging.Logger.Debug().Str("appId", appID).Msg("Creating new service principal")
	spPayload := map[string]string{"appId": appID}
	spBody, statusCode, err := graphPost(ctx, client, token, "/servicePrincipals", spPayload)
	if err != nil {
		return fmt.Errorf("create service principal: %w", err)
	}

	var spCreateResult struct {
		ID    string `json:"id"`
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(spBody, &spCreateResult); err != nil {
		return fmt.Errorf("parse create service principal response: %w (status=%d)", err, statusCode)
	}
	if spCreateResult.ID == "" && spCreateResult.Error.Code != "" {
		return fmt.Errorf("create service principal failed: %s: %s", spCreateResult.Error.Code, spCreateResult.Error.Message)
	}

	logging.Logger.Info().Str("appId", appID).Str("spId", spCreateResult.ID).Msg("Created service principal")
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
		portalURL := entraPortalAppURL(opts.DirectoryID, appID)
		logging.Logger.Info().Str("appId", appID).Str("objectId", objectID).Msg("Found existing venom app")
		logging.Logger.Debug().Str("portalURL", portalURL).Msg("View app in Azure portal")
	} else {
		logging.Logger.Info().Str("displayName", displayName).Msg("Creating new venom app registration")
		appID, objectID, err = createApp(ctx, client, token, displayName, opts.ClientID)
		if err != nil {
			return nil, fmt.Errorf("entra: %w", err)
		}
		portalURL := entraPortalAppURL(opts.DirectoryID, appID)
		logging.Logger.Info().
			Str("appId", appID).
			Str("objectId", objectID).
			Str("portalURL", portalURL).
			Msg("Created venom app in Entra")
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

// IsOIDCEntry returns true if the matrix entry uses OIDC authentication
// (either via the Auth field or the Identity field).
func IsOIDCEntry(auth, identity string) bool {
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

// isCIDomainURI returns true if the URI belongs to a CI deployment
// (contains the CI ingress domain suffix).
func isCIDomainURI(uri string) bool {
	return strings.Contains(uri, ciDomainSuffix)
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

// filterRedirectURIs filters a list of redirect URIs:
// - Removes invalid URIs (empty, trailing commas).
// - Removes stale CI URIs (those matching the CI domain suffix).
// - Preserves all non-CI URIs (e.g., production, staging).
// - Appends the new URIs for the current deployment.
// - Deduplicates the result.
func filterRedirectURIs(existing []string, newURIs []string) []string {
	seen := make(map[string]bool)
	var result []string

	// Keep existing non-CI URIs that are valid.
	for _, uri := range existing {
		if !isValidURI(uri) {
			logging.Logger.Debug().Str("uri", uri).Msg("Removing invalid redirect URI")
			continue
		}
		if isCIDomainURI(uri) {
			logging.Logger.Debug().Str("uri", uri).Msg("Removing stale CI redirect URI")
			continue
		}
		if !seen[uri] {
			seen[uri] = true
			result = append(result, uri)
		}
	}

	// Add new URIs for the current deployment.
	for _, uri := range newURIs {
		if !seen[uri] {
			seen[uri] = true
			result = append(result, uri)
		}
	}

	return result
}

// UpdateRedirectURIs updates the redirect URIs on the parent Entra app
// registration. It:
// 1. Authenticates to Graph API using the parent app credentials.
// 2. Fetches the current web and SPA redirect URIs.
// 3. Removes stale CI URIs and invalid/malformed URIs.
// 4. Adds the redirect URIs for the current deployment's ingress host.
// 5. PATCHes the updated URI lists back to the app registration.
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

	// Step 1: Acquire bearer token.
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

	// Step 2: Fetch current app registration.
	appPath := fmt.Sprintf("/applications/%s", opts.ObjectID)
	body, err := graphGet(ctx, client, token, appPath)
	if err != nil {
		return fmt.Errorf("entra: fetch app registration: %w", err)
	}

	var appData struct {
		Web struct {
			RedirectURIs []string `json:"redirectUris"`
		} `json:"web"`
		Spa struct {
			RedirectURIs []string `json:"redirectUris"`
		} `json:"spa"`
	}
	if err := json.Unmarshal(body, &appData); err != nil {
		return fmt.Errorf("entra: parse app registration: %w", err)
	}

	logging.Logger.Info().
		Int("existingWebURIs", len(appData.Web.RedirectURIs)).
		Int("existingSpaURIs", len(appData.Spa.RedirectURIs)).
		Msg("Fetched current redirect URIs")

	// Step 3: Build new URIs for the current deployment.
	newWebURIs, newSpaURIs := buildRedirectURIs(opts.IngressHost)

	// Step 4: Filter and merge.
	finalWebURIs := filterRedirectURIs(appData.Web.RedirectURIs, newWebURIs)
	finalSpaURIs := filterRedirectURIs(appData.Spa.RedirectURIs, newSpaURIs)

	logging.Logger.Info().
		Int("finalWebURIs", len(finalWebURIs)).
		Int("finalSpaURIs", len(finalSpaURIs)).
		Strs("newWebURIs", newWebURIs).
		Strs("newSpaURIs", newSpaURIs).
		Msg("Computed final redirect URI lists")

	// Step 5: PATCH the app registration.
	patchPayload := map[string]interface{}{
		"web": map[string]interface{}{
			"redirectUris": finalWebURIs,
		},
		"spa": map[string]interface{}{
			"redirectUris": finalSpaURIs,
		},
	}

	patchBody, statusCode, err := graphPatch(ctx, client, token, appPath, patchPayload)
	if err != nil {
		return fmt.Errorf("entra: PATCH redirect URIs: %w", err)
	}

	if statusCode >= 300 {
		return fmt.Errorf("entra: PATCH redirect URIs failed (status=%d): %s", statusCode, string(patchBody))
	}

	logging.Logger.Info().
		Int("statusCode", statusCode).
		Str("objectId", opts.ObjectID).
		Msg("Successfully updated Entra app redirect URIs")

	return nil
}
