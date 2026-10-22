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
	"bufio"
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
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	attempts = 24
	delay    = 5 * time.Second

	credentialsSecret    = "integration-test-credentials"
	venomClientID        = "venom"
	venomClientSecretKey = "identity-admin-client-password"
	tokenAudience        = "orchestration-api"
	gatewayComponent     = "zeebe-standalone-gateway"
)

type config struct {
	namespace string
	release   string
	context   string
	bpmnFile  string
	processID string
	zbctl     string
	scaleBy   int
}

type commandRunner func(context.Context, string, ...string) ([]byte, error)

type checker struct {
	cfg        config
	kubectl    commandRunner
	exec       commandRunner
	httpClient *http.Client
	sleep      func(context.Context, time.Duration) error
	logf       func(string, ...any)

	clientSecret string
	token        string
	tokenURL     string
	restBaseURL  string
}

type ingressList struct {
	Items []ingress `json:"items"`
}

type ingress struct {
	Metadata struct {
		Name string `json:"name"`
	} `json:"metadata"`
	Spec struct {
		Rules []struct {
			Host string `json:"host"`
			HTTP struct {
				Paths []struct {
					Path    string `json:"path"`
					Backend struct {
						Service struct {
							Name string `json:"name"`
						} `json:"service"`
					} `json:"backend"`
				} `json:"paths"`
			} `json:"http"`
		} `json:"rules"`
	} `json:"spec"`
	Status struct {
		LoadBalancer struct {
			Ingress []struct {
				IP string `json:"ip"`
			} `json:"ingress"`
		} `json:"loadBalancer"`
	} `json:"status"`
}

type endpoint struct {
	host        string
	ip          string
	contextPath string
}

func main() {
	cfg := config{}
	flag.StringVar(&cfg.namespace, "namespace", "", "Kubernetes namespace")
	flag.StringVar(&cfg.release, "release", "integration", "Helm release name")
	flag.StringVar(&cfg.context, "context", "", "kubectl context")
	flag.StringVar(&cfg.bpmnFile, "bpmn-file", "", "BPMN file to deploy")
	flag.StringVar(&cfg.processID, "process-id", "", "BPMN process id of the deployed file")
	flag.StringVar(&cfg.zbctl, "zbctl", "zbctl", "zbctl binary used for the gRPC checks")
	flag.IntVar(&cfg.scaleBy, "scale-by", 1, "number of standalone gateway replicas added by the scaling check")
	flag.Parse()

	if cfg.namespace == "" || cfg.bpmnFile == "" || cfg.processID == "" || cfg.scaleBy < 1 {
		fmt.Fprintln(os.Stderr, "usage: standalone-gateway-check --namespace <namespace> [--release <release>] [--context <context>] --bpmn-file <path> --process-id <id> [--zbctl <path>] [--scale-by <n>]")
		os.Exit(2)
	}

	c := &checker{
		cfg:        cfg,
		kubectl:    runCommand,
		exec:       runCommand,
		httpClient: newHTTPClient(directTransport()),
		sleep:      sleepContext,
		logf:       func(format string, args ...any) { fmt.Printf(format+"\n", args...) },
	}
	if err := c.run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		os.Exit(1)
	}
	fmt.Println("The standalone gateway served gRPC and REST before and after scaling, independently of the brokers.")
}

func (c *checker) gatewayName() string { return c.cfg.release + "-zeebe-standalone-gateway" }
func (c *checker) brokerName() string  { return c.cfg.release + "-zeebe" }
func (c *checker) mainService() string { return c.cfg.release + "-zeebe-gateway" }
func (c *checker) ingressPrefix() string {
	return c.cfg.release + "-camunda-platform"
}

func (c *checker) run(ctx context.Context) error {
	if err := c.waitForRollout(ctx); err != nil {
		return err
	}
	brokers, gateways, err := c.readyReplicas(ctx)
	if err != nil {
		return err
	}
	if gateways <= brokers {
		return fmt.Errorf("expected more Ready standalone gateways than brokers, got %d gateways and %d brokers", gateways, brokers)
	}
	c.logf("Ready brokers: %d, Ready standalone gateways: %d", brokers, gateways)

	web, rest, err := c.verifyIngresses(ctx)
	if err != nil {
		return err
	}
	c.httpClient.Transport = resolvedTransport(map[string]string{web.host: web.ip, rest.host: rest.ip})
	c.tokenURL = "https://" + web.host + "/auth/realms/camunda-platform/protocol/openid-connect/token"
	c.restBaseURL = "https://" + rest.host + strings.TrimSuffix(web.contextPath, "/") + "/v2"
	c.logf("REST checks use the standalone gateway REST Ingress: %s", c.restBaseURL)

	if err := c.loadClientSecret(ctx); err != nil {
		return err
	}
	if err := c.acquireToken(ctx); err != nil {
		return err
	}

	if err := c.verifyRound(ctx, brokers, "before scaling"); err != nil {
		return err
	}

	target := gateways + c.cfg.scaleBy
	if _, err := c.kube(ctx, "scale", "deployment/"+c.gatewayName(), "--replicas="+strconv.Itoa(target)); err != nil {
		return err
	}
	if _, err := c.kube(ctx, "rollout", "status", "deployment/"+c.gatewayName(), "--timeout=10m"); err != nil {
		return err
	}
	if err := retry(ctx, attempts, delay, c.sleep, func() error {
		scaledBrokers, scaledGateways, err := c.readyReplicas(ctx)
		if err != nil {
			return err
		}
		if scaledGateways != target {
			return fmt.Errorf("expected %d Ready standalone gateways, got %d", target, scaledGateways)
		}
		if scaledBrokers != brokers {
			return fmt.Errorf("broker count changed from %d to %d while scaling the gateway", brokers, scaledBrokers)
		}
		return nil
	}, "the standalone gateway did not scale independently of the brokers"); err != nil {
		return err
	}
	c.logf("Scaled standalone gateways to %d, brokers stayed at %d", target, brokers)

	if err := c.acquireToken(ctx); err != nil {
		return err
	}
	return c.verifyRound(ctx, brokers, "after scaling")
}

func (c *checker) waitForRollout(ctx context.Context) error {
	for _, args := range [][]string{
		{"rollout", "status", "statefulset/" + c.brokerName(), "--timeout=10m"},
		{"rollout", "status", "deployment/" + c.gatewayName(), "--timeout=10m"},
	} {
		if _, err := c.kube(ctx, args...); err != nil {
			return err
		}
	}
	return nil
}

func (c *checker) readyReplicas(ctx context.Context) (int, int, error) {
	brokers, err := c.readyCount(ctx, "statefulset/"+c.brokerName())
	if err != nil {
		return 0, 0, err
	}
	gateways, err := c.readyCount(ctx, "deployment/"+c.gatewayName())
	if err != nil {
		return 0, 0, err
	}
	return brokers, gateways, nil
}

func (c *checker) readyCount(ctx context.Context, resource string) (int, error) {
	raw, err := c.kube(ctx, "get", resource, "-o", "jsonpath={.status.readyReplicas}")
	if err != nil {
		return 0, err
	}
	return parseCount(raw)
}

func parseCount(raw []byte) (int, error) {
	value := strings.TrimSpace(string(raw))
	if value == "" {
		return 0, nil
	}
	count, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("parse replica count %q: %w", value, err)
	}
	return count, nil
}

func (c *checker) verifyIngresses(ctx context.Context) (endpoint, endpoint, error) {
	var web, rest endpoint
	err := retry(ctx, attempts, delay, c.sleep, func() error {
		raw, err := c.kube(ctx, "get", "ingress", "-o", "json")
		if err != nil {
			return err
		}
		var list ingressList
		if err := json.Unmarshal(raw, &list); err != nil {
			return fmt.Errorf("parse ingress list: %w", err)
		}
		web, rest, err = selectEndpoints(list, c.ingressPrefix(), c.mainService(), c.gatewayName())
		return err
	}, "the Ingress resources did not route as expected")
	return web, rest, err
}

func selectEndpoints(list ingressList, prefix, mainService, gatewayService string) (endpoint, endpoint, error) {
	byName := map[string]ingress{}
	for _, item := range list.Items {
		byName[item.Metadata.Name] = item
	}

	grpc, ok := byName[prefix+"-grpc"]
	if !ok {
		return endpoint{}, endpoint{}, fmt.Errorf("ingress %s-grpc not found", prefix)
	}
	if backends := backendsOf(grpc); len(backends) != 1 || backends[0] != gatewayService {
		return endpoint{}, endpoint{}, fmt.Errorf("gRPC ingress must target %s, targets %v", gatewayService, backends)
	}

	restIngress, ok := byName[prefix+"-standalone-gateway-rest"]
	if !ok {
		return endpoint{}, endpoint{}, fmt.Errorf("ingress %s-standalone-gateway-rest not found", prefix)
	}
	if backends := backendsOf(restIngress); len(backends) != 1 || backends[0] != gatewayService {
		return endpoint{}, endpoint{}, fmt.Errorf("REST ingress must target %s, targets %v", gatewayService, backends)
	}

	var web endpoint
	for _, item := range list.Items {
		for _, rule := range item.Spec.Rules {
			for _, path := range rule.HTTP.Paths {
				switch path.Backend.Service.Name {
				case gatewayService:
					if item.Metadata.Name != prefix+"-grpc" && item.Metadata.Name != prefix+"-standalone-gateway-rest" {
						return endpoint{}, endpoint{}, fmt.Errorf("ingress %s must not target the standalone gateway", item.Metadata.Name)
					}
				case mainService:
					web = endpoint{host: rule.Host, ip: loadBalancerIP(item), contextPath: path.Path}
				}
			}
		}
	}
	if web.host == "" {
		return endpoint{}, endpoint{}, fmt.Errorf("no web ingress targets %s", mainService)
	}
	if web.ip == "" {
		return endpoint{}, endpoint{}, errors.New("the web ingress has no load balancer address")
	}
	if len(restIngress.Spec.Rules) == 0 || restIngress.Spec.Rules[0].Host == "" {
		return endpoint{}, endpoint{}, errors.New("the REST ingress has no host")
	}
	rest := endpoint{host: restIngress.Spec.Rules[0].Host, ip: loadBalancerIP(restIngress)}
	if rest.ip == "" {
		return endpoint{}, endpoint{}, errors.New("the REST ingress has no load balancer address")
	}
	return web, rest, nil
}

func backendsOf(item ingress) []string {
	var names []string
	for _, rule := range item.Spec.Rules {
		for _, path := range rule.HTTP.Paths {
			names = append(names, path.Backend.Service.Name)
		}
	}
	return names
}

func loadBalancerIP(item ingress) string {
	if len(item.Status.LoadBalancer.Ingress) == 0 {
		return ""
	}
	return item.Status.LoadBalancer.Ingress[0].IP
}

func (c *checker) loadClientSecret(ctx context.Context) error {
	raw, err := c.kube(ctx, "get", "secret", credentialsSecret, "-o", "jsonpath={.data."+venomClientSecretKey+"}")
	if err != nil {
		return err
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(raw)))
	if err != nil || len(decoded) == 0 {
		return fmt.Errorf("secret %s has an invalid or empty %s", credentialsSecret, venomClientSecretKey)
	}
	c.clientSecret = string(decoded)
	return nil
}

func (c *checker) acquireToken(ctx context.Context) error {
	return retry(ctx, attempts, delay, c.sleep, func() error {
		form := url.Values{
			"client_id":     {venomClientID},
			"client_secret": {c.clientSecret},
			"grant_type":    {"client_credentials"},
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.tokenURL, strings.NewReader(form.Encode()))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		body, err := do(c.httpClient, req)
		if err != nil {
			return err
		}
		var response struct {
			AccessToken string `json:"access_token"`
		}
		if err := json.Unmarshal(body, &response); err != nil || response.AccessToken == "" {
			return errors.New("token response did not contain an access token")
		}
		c.token = response.AccessToken
		return nil
	}, "could not acquire a token for the venom client")
}

func (c *checker) verifyRound(ctx context.Context, brokers int, phase string) error {
	if err := c.verifyREST(ctx, brokers); err != nil {
		return fmt.Errorf("REST %s: %w", phase, err)
	}
	c.logf("REST %s: topology, deployment, process instance creation and search passed", phase)
	pods, err := c.gatewayPods(ctx)
	if err != nil {
		return err
	}
	for _, pod := range pods {
		if err := c.verifyGRPC(ctx, pod); err != nil {
			return fmt.Errorf("gRPC %s on %s: %w", phase, pod, err)
		}
		c.logf("gRPC %s on %s: topology, deployment and process instance creation passed", phase, pod)
	}
	return nil
}

func (c *checker) verifyREST(ctx context.Context, brokers int) error {
	if err := retry(ctx, attempts, delay, c.sleep, func() error {
		body, err := c.restRequest(ctx, http.MethodGet, "/topology", "", nil)
		if err != nil {
			return err
		}
		return assertBrokerCount(body, brokers)
	}, "topology"); err != nil {
		return err
	}

	if err := retry(ctx, attempts, delay, c.sleep, func() error {
		body, contentType, err := multipartFile("resources", c.cfg.bpmnFile)
		if err != nil {
			return err
		}
		_, err = c.restRequest(ctx, http.MethodPost, "/deployments", contentType, body)
		return err
	}, "deployment"); err != nil {
		return err
	}

	var instanceKey string
	if err := retry(ctx, attempts, delay, c.sleep, func() error {
		payload, _ := json.Marshal(map[string]string{"processDefinitionId": c.cfg.processID})
		body, err := c.restRequest(ctx, http.MethodPost, "/process-instances", "application/json", payload)
		if err != nil {
			return err
		}
		instanceKey, err = processInstanceKey(body)
		return err
	}, "process instance creation"); err != nil {
		return err
	}

	return retry(ctx, attempts, delay, c.sleep, func() error {
		payload := []byte(`{"filter":{"processInstanceKey":"` + instanceKey + `"}}`)
		body, err := c.restRequest(ctx, http.MethodPost, "/process-instances/search", "application/json", payload)
		if err != nil {
			return err
		}
		return assertItemsPresent(body)
	}, "process instance search through secondary storage")
}

func (c *checker) restRequest(ctx context.Context, method, path, contentType string, body []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.restBaseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return do(c.httpClient, req)
}

func (c *checker) gatewayPods(ctx context.Context) ([]string, error) {
	raw, err := c.kube(ctx, "get", "pods",
		"-l", "app.kubernetes.io/instance="+c.cfg.release+",app.kubernetes.io/component="+gatewayComponent,
		"--field-selector=status.phase=Running",
		"-o", "jsonpath={.items[*].metadata.name}")
	if err != nil {
		return nil, err
	}
	pods := strings.Fields(string(raw))
	if len(pods) == 0 {
		return nil, errors.New("no running standalone gateway pods")
	}
	return pods, nil
}

func (c *checker) verifyGRPC(ctx context.Context, pod string) error {
	forwardCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	port, err := c.portForward(forwardCtx, "pod/"+pod, 26500)
	if err != nil {
		return err
	}

	cacheDir, err := os.MkdirTemp("", "zbctl-cache-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(cacheDir)

	base := []string{
		"--address", "127.0.0.1:" + strconv.Itoa(port),
		"--insecure",
		"--clientId", venomClientID,
		"--clientSecret", c.clientSecret,
		"--authzUrl", c.tokenURL,
		"--audience", tokenAudience,
		"--clientCache", filepath.Join(cacheDir, "credentials"),
		"--requestTimeout", "30s",
	}
	zbctl := func(args ...string) ([]byte, error) {
		return c.exec(ctx, c.cfg.zbctl, append(append([]string{}, args...), base...)...)
	}

	if err := retry(ctx, attempts, delay, c.sleep, func() error {
		_, err := zbctl("status")
		return err
	}, "topology"); err != nil {
		return err
	}
	if err := retry(ctx, attempts, delay, c.sleep, func() error {
		_, err := zbctl("deploy", "resource", c.cfg.bpmnFile)
		return err
	}, "deployment"); err != nil {
		return err
	}
	return retry(ctx, attempts, delay, c.sleep, func() error {
		out, err := zbctl("create", "instance", c.cfg.processID)
		if err != nil {
			return err
		}
		_, err = processInstanceKey(out)
		return err
	}, "process instance creation")
}

var forwardingPattern = regexp.MustCompile(`Forwarding from 127\.0\.0\.1:(\d+) ->`)

func (c *checker) portForward(ctx context.Context, target string, remotePort int) (int, error) {
	args := []string{}
	if c.cfg.context != "" {
		args = append(args, "--context", c.cfg.context)
	}
	args = append(args, "-n", c.cfg.namespace, "port-forward", target, fmt.Sprintf(":%d", remotePort))
	cmd := exec.CommandContext(ctx, "kubectl", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return 0, err
	}
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	go func() {
		<-ctx.Done()
		_ = cmd.Wait()
	}()

	ports := make(chan int, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			if port, ok := parseForwardedPort(scanner.Text()); ok {
				ports <- port
				break
			}
		}
		_, _ = io.Copy(io.Discard, stdout)
	}()

	select {
	case port := <-ports:
		return port, nil
	case <-time.After(30 * time.Second):
		return 0, fmt.Errorf("kubectl port-forward %s did not become ready", target)
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}

func parseForwardedPort(line string) (int, bool) {
	match := forwardingPattern.FindStringSubmatch(line)
	if match == nil {
		return 0, false
	}
	port, err := strconv.Atoi(match[1])
	if err != nil {
		return 0, false
	}
	return port, true
}

func assertBrokerCount(raw []byte, expected int) error {
	var topology struct {
		Brokers []json.RawMessage `json:"brokers"`
	}
	if err := json.Unmarshal(raw, &topology); err != nil {
		return fmt.Errorf("parse topology: %w", err)
	}
	if len(topology.Brokers) != expected {
		return fmt.Errorf("topology lists %d brokers, expected %d", len(topology.Brokers), expected)
	}
	return nil
}

func processInstanceKey(raw []byte) (string, error) {
	var response struct {
		ProcessInstanceKey json.RawMessage `json:"processInstanceKey"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return "", fmt.Errorf("parse process instance response: %w", err)
	}
	key := strings.Trim(string(response.ProcessInstanceKey), `"`)
	if key == "" || key == "null" {
		return "", errors.New("response contains no processInstanceKey")
	}
	return key, nil
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

func (c *checker) kube(ctx context.Context, args ...string) ([]byte, error) {
	base := make([]string, 0, len(args)+4)
	if c.cfg.context != "" {
		base = append(base, "--context", c.cfg.context)
	}
	base = append(base, "-n", c.cfg.namespace)
	base = append(base, args...)
	out, err := c.kubectl(ctx, "kubectl", base...)
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

func newHTTPClient(transport http.RoundTripper) *http.Client {
	return &http.Client{
		Timeout:   30 * time.Second,
		Transport: transport,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func resolvedTransport(hosts map[string]string) *http.Transport {
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	return &http.Transport{
		TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
		Proxy:           http.ProxyFromEnvironment,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if ip, ok := hosts[host]; err == nil && ok && ip != "" {
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
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read %s response: %w", req.URL.Redacted(), err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s %s returned HTTP %d: %s", req.Method, req.URL.Redacted(), resp.StatusCode, truncate(body, 300))
	}
	return body, nil
}

func truncate(body []byte, limit int) string {
	text := strings.TrimSpace(string(body))
	if len(text) > limit {
		return text[:limit] + "..."
	}
	return text
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
	return fmt.Errorf("%s failed after %d attempts: %w", message, attempts, lastErr)
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
