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

package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	shortAttempts = 24
	longAttempts  = 60
	shortDelay    = 5 * time.Second
	longDelay     = 10 * time.Second
)

type config struct {
	namespace string
	release   string
	context   string
	bpmnFile  string
}

type commandRunner func(context.Context, string, ...string) ([]byte, error)
type requestFactory func() (*http.Request, error)

type verifier struct {
	cfg        config
	kubectl    commandRunner
	httpClient *http.Client
	sleep      func(context.Context, time.Duration) error
	hostname   string
	ingressIP  string
	token      string
}

type ingressList struct {
	Items []struct {
		Spec struct {
			Rules []struct {
				Host string `json:"host"`
			} `json:"rules"`
		} `json:"spec"`
		Status struct {
			LoadBalancer struct {
				Ingress []struct {
					IP string `json:"ip"`
				} `json:"ingress"`
			} `json:"loadBalancer"`
		} `json:"status"`
	} `json:"items"`
}

type processDefinition struct {
	Key      string          `json:"key"`
	Version  json.RawMessage `json:"version"`
	TenantID json.RawMessage `json:"tenantId"`
}

func main() {
	cfg := config{}
	flag.StringVar(&cfg.namespace, "namespace", "", "Kubernetes namespace")
	flag.StringVar(&cfg.release, "release", "integration", "Helm release name")
	flag.StringVar(&cfg.context, "context", "", "kubectl context")
	flag.StringVar(&cfg.bpmnFile, "bpmn-file", "", "BPMN file to deploy")
	flag.Parse()

	if cfg.namespace == "" || cfg.bpmnFile == "" {
		fmt.Fprintln(os.Stderr, "usage: orchestration-entra-lifecycle --namespace <namespace> [--release <release>] [--context <context>] --bpmn-file <path>")
		os.Exit(2)
	}

	v := &verifier{
		cfg:        cfg,
		kubectl:    runCommand,
		httpClient: newHTTPClient(directTransport()),
		sleep:      sleepContext,
	}
	if err := v.run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		os.Exit(1)
	}
	fmt.Println("Orchestration, Connectors, and an Optimize report worked with Entra and no Management Identity.")
}

func newHTTPClient(transport http.RoundTripper) *http.Client {
	return &http.Client{
		Timeout:   20 * time.Second,
		Transport: transport,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func (v *verifier) run(ctx context.Context) error {
	if err := v.waitForIngress(ctx); err != nil {
		return err
	}
	v.httpClient.Transport = resolvedTransport(v.hostname, v.ingressIP)

	for _, args := range [][]string{
		{"rollout", "status", "statefulset/" + v.cfg.release + "-zeebe", "--timeout=2m"},
		{"rollout", "status", "deployment", "-l", "app.kubernetes.io/component=connectors", "--timeout=2m"},
		{"rollout", "status", "deployment", "-l", "app.kubernetes.io/component=optimize", "--timeout=2m"},
	} {
		if _, err := v.kube(ctx, args...); err != nil {
			return err
		}
	}

	if err := v.assertIdentityAbsent(ctx); err != nil {
		return err
	}
	if err := v.acquireToken(ctx); err != nil {
		return err
	}
	return v.verifyLifecycle(ctx)
}

func (v *verifier) waitForIngress(ctx context.Context) error {
	return retry(ctx, shortAttempts, shortDelay, v.sleep, func() error {
		raw, err := v.kube(ctx, "get", "ingress", "-l", "app.kubernetes.io/instance="+v.cfg.release, "-o", "json")
		if err != nil {
			return err
		}
		host, ip, err := selectIngress(raw)
		if err != nil {
			return err
		}
		v.hostname, v.ingressIP = host, ip
		return nil
	}, "the Camunda ingress did not receive a load balancer address")
}

func (v *verifier) assertIdentityAbsent(ctx context.Context) error {
	raw, err := v.kube(ctx, "get", "deployment,service", "-l", "app.kubernetes.io/component=identity", "-o", "json")
	if err != nil {
		return err
	}
	if err := assertNoItems(raw); err != nil {
		return fmt.Errorf("Management Identity resources must not exist: %w", err)
	}

	shared := v.cfg.release + "-camunda-platform-identity-env-vars"
	for _, name := range []string{shared, v.cfg.release + "-camunda-platform-optimize-identity-env-vars"} {
		raw, err = v.kube(ctx, "get", "configmap", name, "-o", "json")
		if err != nil {
			return err
		}
		if err := assertDataKeyAbsent(raw, "CAMUNDA_IDENTITY_BASEURL"); err != nil {
			return fmt.Errorf("Management Identity URL must not be configured in %s: %w", name, err)
		}
	}
	return nil
}

func (v *verifier) acquireToken(ctx context.Context) error {
	shared := v.cfg.release + "-camunda-platform-identity-env-vars"
	tokenURLRaw, err := v.kube(ctx, "get", "configmap", shared, "-o", "jsonpath={.metadata.annotations.keycloak-token-url}")
	if err != nil {
		return err
	}
	tokenURL := strings.TrimSpace(string(tokenURLRaw))
	if tokenURL == "" {
		return errors.New("Entra token URL annotation is empty")
	}

	secretRaw, err := v.kube(ctx, "get", "secret", "venom-entra-credentials", "-o", "json")
	if err != nil {
		return err
	}
	clientID, clientSecret, audience, err := parseCredentials(secretRaw)
	if err != nil {
		return err
	}
	form := url.Values{
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"scope":         {audience + "/.default"},
		"grant_type":    {"client_credentials"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	body, err := do(v.httpClient, req)
	if err != nil {
		return fmt.Errorf("acquire Entra token: %w", err)
	}
	var response struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &response); err != nil || response.AccessToken == "" {
		return errors.New("Entra token response did not contain an access token")
	}
	v.token = response.AccessToken
	return nil
}

func (v *verifier) verifyLifecycle(ctx context.Context) error {
	orchestrationURL := "https://" + v.hostname + "/orchestration/v2"
	optimizeURL := "https://" + v.hostname + "/optimize/api"

	if err := v.retryRequest(ctx, shortAttempts+1, shortDelay, http.MethodGet, orchestrationURL+"/topology", "", nil, func(body []byte) error {
		var topology struct {
			Brokers []json.RawMessage `json:"brokers"`
		}
		if err := json.Unmarshal(body, &topology); err != nil || len(topology.Brokers) == 0 {
			return errors.New("topology contains no brokers")
		}
		return nil
	}, "authenticated topology request failed"); err != nil {
		return err
	}
	if err := v.retryRequest(ctx, shortAttempts+1, shortDelay, http.MethodGet, optimizeURL+"/dashboard/management", "", nil, nil, "authenticated Optimize request failed"); err != nil {
		return err
	}

	if err := retry(ctx, shortAttempts, shortDelay, v.sleep, func() error {
		body, contentType, err := multipartFile("resources", v.cfg.bpmnFile)
		if err != nil {
			return err
		}
		return v.requestWithHeaders(ctx, http.MethodPost, orchestrationURL+"/deployments", contentType, body, map[string]string{"Accept": "application/json"}, nil)
	}, "the Entra test client did not receive BPMN deployment permission"); err != nil {
		return err
	}

	webhookBody := []byte(`{"webhookDataKey":"webhookDataValue"}`)
	if err := retry(ctx, shortAttempts, shortDelay, v.sleep, func() error {
		return v.request(ctx, http.MethodPost, "https://"+v.hostname+"/connectors/inbound/test-mywebhook", "application/json", webhookBody, nil)
	}, "connectors did not activate the inbound webhook"); err != nil {
		return err
	}

	searchBody := []byte(`{"filter":{"processDefinitionId":"test-inbound-process","state":"COMPLETED"}}`)
	if err := retry(ctx, shortAttempts, shortDelay, v.sleep, func() error {
		return v.request(ctx, http.MethodPost, orchestrationURL+"/process-instances/search", "application/json", searchBody, assertItemsPresent)
	}, "the inbound connector did not complete a process instance"); err != nil {
		return err
	}

	var definition processDefinition
	if err := retry(ctx, longAttempts, longDelay, v.sleep, func() error {
		return v.request(ctx, http.MethodGet, optimizeURL+"/definition/process", "", nil, func(body []byte) error {
			selected, err := selectLatestDefinition(body, "test-inbound-process")
			if err == nil {
				definition = selected
			}
			return err
		})
	}, "Optimize did not import the connector process definition"); err != nil {
		return err
	}

	reportBody, err := reportDefinition(definition)
	if err != nil {
		return err
	}
	var reportID string
	if err := v.request(ctx, http.MethodPost, optimizeURL+"/report/process/single", "application/json", reportBody, func(body []byte) error {
		var report struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(body, &report); err != nil || report.ID == "" {
			return errors.New("Optimize report response did not contain an id")
		}
		reportID = report.ID
		return nil
	}); err != nil {
		return err
	}

	return retry(ctx, longAttempts, longDelay, v.sleep, func() error {
		return v.request(ctx, http.MethodGet, optimizeURL+"/public/export/report/"+url.PathEscape(reportID)+"/result/json", "", nil, assertPositiveReport)
	}, "the Optimize report did not include the connector process instance")
}

func (v *verifier) retryRequest(ctx context.Context, attempts int, delay time.Duration, method, endpoint, contentType string, body []byte, validate func([]byte) error, message string) error {
	return retry(ctx, attempts, delay, v.sleep, func() error {
		return v.request(ctx, method, endpoint, contentType, body, validate)
	}, message)
}

func (v *verifier) request(ctx context.Context, method, endpoint, contentType string, body []byte, validate func([]byte) error) error {
	return v.requestWithHeaders(ctx, method, endpoint, contentType, body, nil, validate)
}

func (v *verifier) requestWithHeaders(ctx context.Context, method, endpoint, contentType string, body []byte, headers map[string]string, validate func([]byte) error) error {
	req, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+v.token)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	response, err := do(v.httpClient, req)
	if err != nil {
		return err
	}
	if validate != nil {
		return validate(response)
	}
	return nil
}

func (v *verifier) kube(ctx context.Context, args ...string) ([]byte, error) {
	base := make([]string, 0, len(args)+4)
	if v.cfg.context != "" {
		base = append(base, "--context", v.cfg.context)
	}
	base = append(base, "-n", v.cfg.namespace)
	base = append(base, args...)
	out, err := v.kubectl(ctx, "kubectl", base...)
	if err != nil {
		return nil, fmt.Errorf("kubectl %s: %w", strings.Join(args, " "), err)
	}
	return out, nil
}

func runCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message != "" {
			return nil, fmt.Errorf("%s: %w", message, err)
		}
		return nil, err
	}
	return stdout.Bytes(), nil
}

func resolvedTransport(hostname, ip string) *http.Transport {
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	return &http.Transport{
		TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
		Proxy:           http.ProxyFromEnvironment,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err == nil && host == hostname {
				address = net.JoinHostPort(ip, port)
			}
			return dialer.DialContext(ctx, network, address)
		},
	}
}

func directTransport() *http.Transport {
	return &http.Transport{
		TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
		Proxy:           http.ProxyFromEnvironment,
		DialContext:     (&net.Dialer{Timeout: 5 * time.Second}).DialContext,
	}
}

func do(client *http.Client, req *http.Request) ([]byte, error) {
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil, fmt.Errorf("%s returned HTTP %d", req.URL.Redacted(), resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read %s response: %w", req.URL.Redacted(), err)
	}
	return body, nil
}

func retry(ctx context.Context, attempts int, delay time.Duration, sleep func(context.Context, time.Duration) error, action func() error, message string) error {
	if attempts < 1 {
		return errors.New("retry attempts must be positive")
	}
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		if err := action(); err == nil {
			return nil
		} else {
			lastErr = err
		}
		if attempt < attempts {
			if err := sleep(ctx, delay); err != nil {
				return fmt.Errorf("%s: %w", message, err)
			}
		}
	}
	return fmt.Errorf("%s after %d attempts: %w", message, attempts, lastErr)
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func selectIngress(raw []byte) (string, string, error) {
	var ingresses ingressList
	if err := json.Unmarshal(raw, &ingresses); err != nil {
		return "", "", fmt.Errorf("parse ingress list: %w", err)
	}
	var hostname string
	for _, ingress := range ingresses.Items {
		for _, rule := range ingress.Spec.Rules {
			if rule.Host != "" && !strings.HasPrefix(rule.Host, "grpc-") {
				hostname = rule.Host
				break
			}
		}
		if hostname != "" {
			break
		}
	}
	for _, ingress := range ingresses.Items {
		for _, rule := range ingress.Spec.Rules {
			if rule.Host == hostname && len(ingress.Status.LoadBalancer.Ingress) > 0 {
				if ip := ingress.Status.LoadBalancer.Ingress[0].IP; ip != "" {
					return hostname, ip, nil
				}
				return "", "", errors.New("ingress load balancer address is empty")
			}
		}
	}
	return "", "", errors.New("ingress hostname or load balancer address is empty")
}

func assertNoItems(raw []byte) error {
	var list struct {
		Items []json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(raw, &list); err != nil {
		return fmt.Errorf("parse resource list: %w", err)
	}
	if len(list.Items) != 0 {
		return fmt.Errorf("found %d resource(s)", len(list.Items))
	}
	return nil
}

func assertDataKeyAbsent(raw []byte, key string) error {
	var configMap struct {
		Data map[string]string `json:"data"`
	}
	if err := json.Unmarshal(raw, &configMap); err != nil {
		return fmt.Errorf("parse ConfigMap: %w", err)
	}
	if _, exists := configMap.Data[key]; exists {
		return fmt.Errorf("found forbidden key %s", key)
	}
	return nil
}

func parseCredentials(raw []byte) (string, string, string, error) {
	var secret struct {
		Data map[string]string `json:"data"`
	}
	if err := json.Unmarshal(raw, &secret); err != nil {
		return "", "", "", errors.New("parse Entra credentials secret")
	}
	values := make([]string, 0, 3)
	for _, key := range []string{"client-id", "client-secret", "audience"} {
		encoded := secret.Data[key]
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil || len(decoded) == 0 {
			return "", "", "", fmt.Errorf("Entra credentials secret has invalid or empty %s", key)
		}
		values = append(values, string(decoded))
	}
	return values[0], values[1], values[2], nil
}

func assertItemsPresent(raw []byte) error {
	var result struct {
		Items []json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return err
	}
	if len(result.Items) == 0 {
		return errors.New("response contains no items")
	}
	return nil
}

func selectLatestDefinition(raw []byte, key string) (processDefinition, error) {
	var definitions []processDefinition
	if err := json.Unmarshal(raw, &definitions); err != nil {
		return processDefinition{}, fmt.Errorf("parse Optimize definitions: %w", err)
	}
	var selected processDefinition
	selectedVersion := -1
	for _, definition := range definitions {
		if definition.Key != key {
			continue
		}
		versionText := strings.Trim(string(definition.Version), `"`)
		version, err := strconv.Atoi(versionText)
		if err != nil {
			return processDefinition{}, fmt.Errorf("definition %s has invalid version", key)
		}
		if version >= selectedVersion {
			selected, selectedVersion = definition, version
		}
	}
	if selectedVersion < 0 {
		return processDefinition{}, fmt.Errorf("definition %s was not found", key)
	}
	return selected, nil
}

func reportDefinition(definition processDefinition) ([]byte, error) {
	version := strings.Trim(string(definition.Version), `"`)
	tenantID := strings.Trim(string(definition.TenantID), `"`)
	payload := map[string]any{"data": map[string]any{
		"definitions":   []map[string]any{{"key": definition.Key, "versions": []string{version}, "tenantIds": []string{tenantID}}},
		"view":          map[string]any{"entity": "processInstance", "properties": []string{"frequency"}},
		"groupBy":       map[string]any{"type": "none", "value": nil},
		"distributedBy": map[string]any{"type": "none", "value": nil},
		"visualization": "number",
	}}
	return json.Marshal(payload)
}

func assertPositiveReport(raw []byte) error {
	var report struct {
		Data float64 `json:"data"`
	}
	if err := json.Unmarshal(raw, &report); err != nil {
		return err
	}
	if report.Data <= 0 {
		return errors.New("report data is not positive")
	}
	return nil
}

func multipartFile(field, path string) ([]byte, string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, "", fmt.Errorf("read BPMN file: %w", err)
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile(field, filepath.Base(path))
	if err != nil {
		return nil, "", err
	}
	if _, err := part.Write(content); err != nil {
		return nil, "", err
	}
	if err := writer.Close(); err != nil {
		return nil, "", err
	}
	return body.Bytes(), writer.FormDataContentType(), nil
}
