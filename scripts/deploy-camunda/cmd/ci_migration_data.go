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

package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newCIMigrationDataCommand() *cobra.Command {
	var chartPath, namespace, kubeContext string
	cmd := &cobra.Command{
		Use:   "migration-data",
		Short: "Load the connector fixture required by SM migration tests",
		RunE: func(cmd *cobra.Command, args []string) error {
			return loadMigrationData(cmd.Context(), chartPath, namespace, kubeContext)
		},
	}
	cmd.Flags().StringVar(&chartPath, "chart-path", "", "absolute path to the installed-version chart")
	cmd.Flags().StringVar(&namespace, "namespace", "", "namespace containing the installed version")
	cmd.Flags().StringVar(&kubeContext, "kube-context", "", "optional Kubernetes context")
	_ = cmd.MarkFlagRequired("chart-path")
	_ = cmd.MarkFlagRequired("namespace")
	return cmd
}

func loadMigrationData(ctx context.Context, chartPath, namespace, kubeContext string) error {
	envPath := filepath.Join(chartPath, "test", "e2e", ".env."+namespace+".migration-data")
	defer os.Remove(envPath)
	if err := renderMigrationEnv(ctx, chartPath, namespace, kubeContext, envPath); err != nil {
		return err
	}
	envFile, err := os.Open(envPath)
	if err != nil {
		return fmt.Errorf("read migration environment: %w", err)
	}
	defer envFile.Close()
	env, err := parseMigrationEnv(envFile)
	if err != nil {
		return err
	}
	baseURL := strings.TrimSuffix(env["BASE_URL"], "/")
	tokenURL := env["OAUTH_URL"]
	secret := env["DISTRO_QA_E2E_TESTS_KEYCLOAK_CLIENTS_SECRET"]
	if baseURL == "" || tokenURL == "" || secret == "" {
		return fmt.Errorf("rendered migration environment is missing BASE_URL, OAUTH_URL, or test client secret")
	}
	token, err := waitForMigrationAdminRole(ctx, baseURL+"/orchestration", tokenURL, secret, 10, 5*time.Second, 30*time.Second)
	if err != nil {
		return err
	}
	resource := filepath.Join(chartPath, "test", "e2e", "node_modules", "@camunda", "e2e-test-suite", "resources", "ConnectorsBasicTest.bpmn")
	if err := deployMigrationResource(ctx, baseURL, token, resource); err != nil {
		return err
	}
	payload, err := json.Marshal(map[string]string{
		"baseurl": baseURL, "version": "8.8", "tokenEndpoint": tokenURL,
		"zeebeSecret": secret, "zeebeClientId": "test",
	})
	if err != nil {
		return err
	}
	_, err = retryMigrationRequest(ctx, baseURL+"/connectors/inbound/basetestsconnectors", token, "application/json", payload, 12, 10*time.Second, true)
	return err
}

func renderMigrationEnv(ctx context.Context, chartPath, namespace, kubeContext, output string) error {
	repoRoot := filepath.Dir(filepath.Dir(chartPath))
	args := []string{filepath.Join(repoRoot, "scripts", "render-e2e-env.sh"), "--absolute-chart-path", chartPath, "--namespace", namespace, "--output", output}
	if kubeContext != "" {
		args = append(args, "--kube-context", kubeContext)
	}
	command := exec.CommandContext(ctx, "bash", args...)
	command.Stdout, command.Stderr = os.Stdout, os.Stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("render migration environment: %w", err)
	}
	return nil
}

func parseMigrationEnv(reader io.Reader) (map[string]string, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("parse migration environment: %w", err)
	}
	values := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if ok {
			values[key] = value
		}
	}
	return values, nil
}

func requestMigrationToken(ctx context.Context, endpoint, secret string) (string, error) {
	form := url.Values{"grant_type": {"client_credentials"}, "client_id": {"test"}, "client_secret": {secret}}
	client := &http.Client{Timeout: time.Minute}
	var lastError string
	for attempt := 1; attempt <= 5; attempt++ {
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
		if err != nil {
			return "", err
		}
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		response, err := client.Do(request)
		retryable := err != nil
		if err == nil {
			retryable = isTransientMigrationStatus(response.StatusCode)
			var body struct {
				AccessToken string `json:"access_token"`
			}
			decodeErr := json.NewDecoder(response.Body).Decode(&body)
			response.Body.Close()
			if response.StatusCode >= 200 && response.StatusCode < 300 && decodeErr == nil && body.AccessToken != "" {
				return body.AccessToken, nil
			}
			lastError = fmt.Sprintf("HTTP %s", response.Status)
		} else {
			lastError = err.Error()
		}
		if !retryable {
			return "", fmt.Errorf("request migration token failed: %s", lastError)
		}
		if attempt < 5 {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(10 * time.Second):
			}
		}
	}
	return "", fmt.Errorf("request migration token failed after 5 attempts: %s", lastError)
}

func waitForMigrationAdminRole(ctx context.Context, orchestrationURL, tokenURL, secret string, attempts int, delay, propagationDelay time.Duration) (string, error) {
	client := &http.Client{Timeout: time.Minute}
	endpoint := strings.TrimSuffix(orchestrationURL, "/") + "/v2/roles/admin/clients/test"
	var lastError string
	for attempt := 1; attempt <= attempts; attempt++ {
		token, err := requestMigrationToken(ctx, tokenURL, secret)
		if err != nil {
			lastError = err.Error()
		} else {
			request, requestErr := http.NewRequestWithContext(ctx, http.MethodPut, endpoint, nil)
			if requestErr != nil {
				return "", requestErr
			}
			request.Header.Set("Authorization", "Bearer "+token)
			response, requestErr := client.Do(request)
			if requestErr != nil {
				lastError = requestErr.Error()
			} else {
				responseBody, readErr := io.ReadAll(response.Body)
				response.Body.Close()
				if readErr != nil {
					lastError = readErr.Error()
				} else if response.StatusCode == http.StatusConflict || (response.StatusCode >= 200 && response.StatusCode < 300) {
					if err := waitForMigrationRetry(ctx, propagationDelay); err != nil {
						return "", err
					}
					return requestMigrationToken(ctx, tokenURL, secret)
				} else {
					lastError = fmt.Sprintf("HTTP %s: %s", response.Status, strings.TrimSpace(string(responseBody)))
				}
			}
		}
		if attempt < attempts {
			if err := waitForMigrationRetry(ctx, delay); err != nil {
				return "", err
			}
		}
	}
	return "", fmt.Errorf("test client never obtained the admin role after %d attempts: %s", attempts, lastError)
}

func deployMigrationResource(ctx context.Context, baseURL, token, path string) error {
	resource, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read migration BPMN resource: %w", err)
	}
	body, contentType, err := migrationMultipartBody(filepath.Base(path), resource)
	if err != nil {
		return err
	}
	response, err := retryMigrationRequest(ctx, baseURL+"/orchestration/v2/deployments", token, contentType, body, 5, 10*time.Second, false)
	if err != nil {
		return err
	}
	var deployment struct {
		DeploymentKey string `json:"deploymentKey"`
	}
	if err := json.Unmarshal(response, &deployment); err != nil {
		return fmt.Errorf("decode migration deployment response: %w", err)
	}
	if deployment.DeploymentKey == "" {
		return fmt.Errorf("migration deployment response contains no deploymentKey")
	}
	return nil
}

func migrationMultipartBody(filename string, resource []byte) ([]byte, string, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("resources", filename)
	if err != nil {
		return nil, "", err
	}
	if _, err := part.Write(resource); err != nil {
		return nil, "", err
	}
	if err := writer.Close(); err != nil {
		return nil, "", err
	}
	return body.Bytes(), writer.FormDataContentType(), nil
}

func retryMigrationRequest(ctx context.Context, endpoint, token, contentType string, body []byte, attempts int, delay time.Duration, retryAllStatuses bool) ([]byte, error) {
	client := &http.Client{Timeout: 2 * time.Minute}
	var lastError string
	for attempt := 1; attempt <= attempts; attempt++ {
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		request.Header.Set("Authorization", "Bearer "+token)
		request.Header.Set("Content-Type", contentType)
		response, err := client.Do(request)
		if err == nil {
			responseBody, readErr := io.ReadAll(response.Body)
			response.Body.Close()
			if readErr == nil && response.StatusCode >= 200 && response.StatusCode < 300 {
				return responseBody, nil
			}
			lastError = fmt.Sprintf("HTTP %s: %s", response.Status, strings.TrimSpace(string(responseBody)))
			if !retryAllStatuses && !isTransientMigrationStatus(response.StatusCode) {
				return nil, fmt.Errorf("migration data request %s failed: %s", endpoint, lastError)
			}
		} else {
			lastError = err.Error()
		}
		if attempt < attempts {
			if err := waitForMigrationRetry(ctx, delay); err != nil {
				return nil, err
			}
		}
	}
	return nil, fmt.Errorf("migration data request %s failed after %d attempts: %s", endpoint, attempts, lastError)
}

func isTransientMigrationStatus(status int) bool {
	return status == http.StatusRequestTimeout || status == http.StatusTooManyRequests || status >= 500
}

func waitForMigrationRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
