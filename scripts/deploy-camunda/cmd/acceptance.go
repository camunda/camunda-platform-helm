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
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
)

type physicalTenantAcceptanceOptions struct {
	namespace, release, hubNamespace, hubHost, orchHost, kubeContext string
	optimizePaths                                                    map[string]string
}

type acceptanceProcess interface {
	Alive() bool
	Stop() error
	Output() string
}

type acceptanceDependencies struct {
	doHTTP       func(*http.Request) (*http.Response, error)
	runCommand   func(context.Context, string, ...string) ([]byte, error)
	startForward func(context.Context, []string) (acceptanceProcess, error)
	sleep        func(context.Context, time.Duration) error
	now          func() time.Time
	pid          func() int
	out, errOut  io.Writer
}

type synchronizedBuffer struct {
	sync.Mutex
	bytes.Buffer
}

func (b *synchronizedBuffer) Write(value []byte) (int, error) {
	b.Lock()
	defer b.Unlock()
	return b.Buffer.Write(value)
}

func (b *synchronizedBuffer) String() string {
	b.Lock()
	defer b.Unlock()
	return b.Buffer.String()
}

type osAcceptanceProcess struct {
	cmd    *exec.Cmd
	output *synchronizedBuffer
	done   chan struct{}
}

func (p *osAcceptanceProcess) Alive() bool {
	select {
	case <-p.done:
		return false
	default:
		return true
	}
}
func (p *osAcceptanceProcess) Output() string { return p.output.String() }
func (p *osAcceptanceProcess) Stop() error {
	if p.cmd.Process == nil || !p.Alive() {
		return nil
	}
	return p.cmd.Process.Kill()
}

func newAcceptanceCommand() *cobra.Command {
	parent := &cobra.Command{Use: "acceptance", Short: "Run repository acceptance checks"}
	var opts physicalTenantAcceptanceOptions
	cmd := &cobra.Command{
		Use:   "physical-tenant-exporters",
		Short: "Validate Physical Tenant exporters and Optimize isolation",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runPhysicalTenantAcceptance(cmd.Context(), opts, productionAcceptanceDependencies(opts))
		},
	}
	f := cmd.Flags()
	f.StringVar(&opts.namespace, "namespace", os.Getenv("TEST_NAMESPACE"), "Camunda namespace")
	f.StringVar(&opts.release, "release", envDefault("RELEASE_NAME", "integration"), "Camunda release")
	f.StringVar(&opts.hubNamespace, "hub-namespace", os.Getenv("HUB_NAMESPACE"), "Hub namespace")
	f.StringVar(&opts.hubHost, "hub-host", os.Getenv("HUB_HOST"), "Hub hostname")
	f.StringVar(&opts.orchHost, "orchestration-host", os.Getenv("ORCH_HOST"), "Orchestration hostname")
	f.StringVar(&opts.kubeContext, "kube-context", os.Getenv("KUBE_CONTEXT"), "kubectl context")
	paths := map[string]*string{}
	for _, tenant := range []string{"default", "tenanta", "tenantb"} {
		paths[tenant] = new(string)
	}
	f.StringVar(paths["default"], "default-optimize-path", os.Getenv("OPTDEFAULT_OPTIMIZE_CONTEXT_PATH"), "Default Optimize context path")
	f.StringVar(paths["tenanta"], "tenanta-optimize-path", os.Getenv("OPTTA_OPTIMIZE_CONTEXT_PATH"), "Tenant A Optimize context path")
	f.StringVar(paths["tenantb"], "tenantb-optimize-path", os.Getenv("OPTTB_OPTIMIZE_CONTEXT_PATH"), "Tenant B Optimize context path")
	cmd.PreRunE = func(*cobra.Command, []string) error {
		opts.optimizePaths = map[string]string{}
		for tenant, value := range paths {
			opts.optimizePaths[tenant] = *value
		}
		return validatePhysicalTenantOptions(opts)
	}
	parent.AddCommand(cmd)
	return parent
}

func envDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func validatePhysicalTenantOptions(opts physicalTenantAcceptanceOptions) error {
	required := map[string]string{
		"namespace": opts.namespace, "hub-namespace": opts.hubNamespace, "hub-host": opts.hubHost,
		"orchestration-host": opts.orchHost, "default-optimize-path": opts.optimizePaths["default"],
		"tenanta-optimize-path": opts.optimizePaths["tenanta"], "tenantb-optimize-path": opts.optimizePaths["tenantb"],
	}
	for name, value := range required {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("required flag --%s is empty", name)
		}
	}
	return nil
}

func productionAcceptanceDependencies(opts physicalTenantAcceptanceOptions) acceptanceDependencies {
	kubectlBase := func() []string {
		if opts.kubeContext != "" {
			return []string{"--context", opts.kubeContext}
		}
		return nil
	}
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{Timeout: 10 * time.Second}).DialContext,
		},
	}
	return acceptanceDependencies{
		doHTTP: client.Do,
		runCommand: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return exec.CommandContext(ctx, name, args...).CombinedOutput()
		},
		startForward: func(ctx context.Context, args []string) (acceptanceProcess, error) {
			allArgs := append(kubectlBase(), args...)
			cmd := exec.CommandContext(ctx, "kubectl", allArgs...)
			output := &synchronizedBuffer{}
			cmd.Stdout, cmd.Stderr = output, output
			if err := cmd.Start(); err != nil {
				return nil, err
			}
			process := &osAcceptanceProcess{cmd: cmd, output: output, done: make(chan struct{})}
			go func() {
				_ = cmd.Wait()
				close(process.done)
			}()
			return process, nil
		},
		sleep: func(ctx context.Context, duration time.Duration) error {
			timer := time.NewTimer(duration)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
				return nil
			}
		},
		now: time.Now, pid: os.Getpid, out: os.Stdout, errOut: os.Stderr,
	}
}

type partitionState struct {
	ExporterPhase    string `json:"exporterPhase"`
	ExportedPosition int64  `json:"exportedPosition"`
}

func runPhysicalTenantAcceptance(ctx context.Context, opts physicalTenantAcceptanceOptions, deps acceptanceDependencies) (err error) {
	var partitionsBody, brokerLog string
	fail := func(cause error) error {
		var exporters []string
		for _, line := range strings.Split(brokerLog, "\n") {
			if strings.Contains(line, "broker.exporter.elasticsearch") || strings.Contains(line, "broker.exporter.opensearch") || strings.Contains(line, "BlockingExporter") {
				exporters = append(exporters, line)
			}
		}
		return fmt.Errorf("%w\n--- partitions ---\n%s\n--- exporter log lines ---\n%s", cause, defaultText(partitionsBody), defaultText(strings.Join(exporters, "\n")))
	}

	process, err := deps.startForward(ctx, []string{"-n", opts.namespace, "port-forward", "statefulset/" + opts.release + "-zeebe", ":9600"})
	if err != nil {
		return fail(fmt.Errorf("start management port-forward: %w", err))
	}
	defer process.Stop()
	localPort, err := waitForForwardPort(ctx, process, deps.sleep)
	if err != nil {
		return fail(err)
	}
	managementURL := "http://127.0.0.1:" + localPort
	actuator := ""
	for _, candidate := range []string{"/orchestration/actuator", "/actuator"} {
		if _, status, requestErr := httpGet(ctx, deps.doHTTP, managementURL+candidate, ""); requestErr == nil && status >= 200 && status < 300 {
			actuator = candidate
			break
		}
	}
	if actuator == "" {
		return fail(errors.New("could not reach the broker actuator on the management port"))
	}
	for attempt := 1; attempt <= 60; attempt++ {
		if !process.Alive() {
			return fail(fmt.Errorf("port-forward to %s-zeebe died before every physical tenant reported exporting: %s", opts.release, strings.TrimSpace(process.Output())))
		}
		body, _, requestErr := httpGet(ctx, deps.doHTTP, managementURL+actuator+"/partitions", "")
		if requestErr == nil {
			partitionsBody = string(body)
		}
		if partitionsHealthy(body) {
			break
		}
		if attempt == 60 {
			return fail(errors.New("physical tenants did not all reach a healthy exporting state"))
		}
		fmt.Fprintf(deps.out, "Waiting for every physical tenant to export (attempt %d/60)...\n", attempt)
		if err := deps.sleep(ctx, 5*time.Second); err != nil {
			return fail(err)
		}
	}

	commandArgs := kubectlArgs(opts.kubeContext, "logs", "-n", opts.namespace, "statefulset/"+opts.release+"-zeebe", "--all-containers")
	logBytes, commandErr := deps.runCommand(ctx, "kubectl", commandArgs...)
	brokerLog = string(logBytes)
	if commandErr != nil {
		return fail(fmt.Errorf("read broker logs: %w: %s", commandErr, strings.TrimSpace(brokerLog)))
	}
	if strings.Contains(brokerLog, "BlockingExporter") {
		return fail(errors.New("a physical tenant fell back to a BlockingExporter: an exporter id is enabled in the partition state but its configuration is missing"))
	}
	for _, tenant := range []string{"default", "tenanta", "tenantb"} {
		matched, _ := regexp.MatchString(`broker\.exporter\.(elasticsearch|opensearch).*physicalTenant=`+regexp.QuoteMeta(tenant)+`[},]`, brokerLog)
		if !matched {
			return fail(fmt.Errorf("the legacy exporter never opened for physical tenant %q", tenant))
		}
	}
	fmt.Fprintln(deps.out, "Legacy exporter opened for all 3 physical tenants, with no blocked exporters.")

	secretNames := map[string]string{
		"venom": "identity-admin-client-password", "default": "identity-optimize-default-client-token",
		"tenanta": "identity-optimize-tenanta-client-token", "tenantb": "identity-optimize-tenantb-client-token",
	}
	tokens := map[string]string{}
	for tenant, secretName := range secretNames {
		secret, secretErr := readSecret(ctx, deps, opts, secretName)
		if secretErr != nil {
			return fail(secretErr)
		}
		clientID := "optimize-orcha-" + tenant
		if tenant == "venom" {
			clientID = "venom"
		}
		tokens[tenant], secretErr = clientToken(ctx, deps.doHTTP, "https://"+opts.hubHost, clientID, secret)
		if secretErr != nil {
			return fail(secretErr)
		}
	}
	for _, tenant := range []string{"default", "tenanta", "tenantb"} {
		forbidden := []string{}
		for _, other := range []string{"default", "tenanta", "tenantb"} {
			if other != tenant {
				forbidden = append(forbidden, "optimize-orcha-"+other+"-api")
			}
		}
		if audienceErr := validateTokenAudience(tokens[tenant], "optimize-orcha-"+tenant+"-api", forbidden); audienceErr != nil {
			return fail(fmt.Errorf("%s Optimize token has the wrong audience: %w", tenant, audienceErr))
		}
	}

	runID := strconv.FormatInt(deps.now().Unix(), 10) + "-" + strconv.Itoa(deps.pid())
	processes := map[string]string{}
	for _, tenant := range []string{"default", "tenanta", "tenantb"} {
		processID := "pt-accept-" + tenant + "-" + runID
		key, deployErr := deployAcceptanceProcess(ctx, deps.doHTTP, "https://"+opts.orchHost, tenant, processID, tokens["venom"])
		if deployErr != nil {
			return fail(deployErr)
		}
		if startErr := startAcceptanceProcess(ctx, deps.doHTTP, "https://"+opts.orchHost, tenant, key, processID, tokens["venom"]); startErr != nil {
			return fail(startErr)
		}
		processes[tenant] = processID
	}
	for _, tenant := range []string{"default", "tenanta", "tenantb"} {
		if importErr := waitForOptimizeImport(ctx, deps, "https://"+opts.hubHost+opts.optimizePaths[tenant], tenant, processes[tenant], tokens[tenant], 60); importErr != nil {
			return fail(importErr)
		}
	}
	for _, tenant := range []string{"default", "tenanta", "tenantb"} {
		var siblings []string
		for other, processID := range processes {
			if other != tenant {
				siblings = append(siblings, processID)
			}
		}
		if isolationErr := assertOptimizeExcludesSiblings(ctx, deps, "https://"+opts.hubHost+opts.optimizePaths[tenant], tenant, tokens[tenant], siblings, 6); isolationErr != nil {
			return fail(isolationErr)
		}
	}
	for source, token := range tokens {
		if source == "venom" {
			continue
		}
		for target, path := range opts.optimizePaths {
			if source == target {
				continue
			}
			_, status, requestErr := httpGet(ctx, deps.doHTTP, "https://"+opts.hubHost+path+"/api/definition/process/keys", token)
			if requestErr != nil {
				return fail(fmt.Errorf("check %s credentials against %s Optimize: %w", source, target, requestErr))
			}
			if status != http.StatusUnauthorized && status != http.StatusForbidden {
				return fail(fmt.Errorf("Optimize for %q accepted %q credentials (HTTP %d)", target, source, status))
			}
		}
	}
	fmt.Fprintln(deps.out, "Default, tenanta, and tenantb Optimize releases imported only their own process and rejected sibling credentials.")
	return nil
}

func defaultText(value string) string {
	if value == "" {
		return "<none>"
	}
	return value
}

func kubectlArgs(contextName string, args ...string) []string {
	if contextName == "" {
		return args
	}
	return append([]string{"--context", contextName}, args...)
}

func waitForForwardPort(ctx context.Context, process acceptanceProcess, sleep func(context.Context, time.Duration) error) (string, error) {
	pattern := regexp.MustCompile(`Forwarding from 127\.0\.0\.1:([0-9]+)`)
	for attempt := 0; attempt < 30; attempt++ {
		body := process.Output()
		if match := pattern.FindStringSubmatch(body); len(match) == 2 {
			return string(match[1]), nil
		}
		if !process.Alive() {
			return "", fmt.Errorf("management port-forward died: %s", strings.TrimSpace(body))
		}
		if err := sleep(ctx, time.Second); err != nil {
			return "", err
		}
	}
	return "", fmt.Errorf("management port-forward never reported a local port: %s", strings.TrimSpace(process.Output()))
}

func partitionsHealthy(body []byte) bool {
	var partitions map[string][]partitionState
	if json.Unmarshal(body, &partitions) != nil || len(partitions) != 3 {
		return false
	}
	for _, tenant := range []string{"default", "tenanta", "tenantb"} {
		states, ok := partitions[tenant]
		if !ok || len(states) == 0 {
			return false
		}
		for _, state := range states {
			if state.ExporterPhase != "EXPORTING" || state.ExportedPosition < 0 {
				return false
			}
		}
	}
	return true
}

func readSecret(ctx context.Context, deps acceptanceDependencies, opts physicalTenantAcceptanceOptions, key string) (string, error) {
	args := kubectlArgs(opts.kubeContext, "-n", opts.hubNamespace, "get", "secret", "integration-test-credentials", "-o", "jsonpath={.data."+key+"}")
	body, err := deps.runCommand(ctx, "kubectl", args...)
	if err != nil {
		return "", fmt.Errorf("read secret %s: %w: %s", key, err, strings.TrimSpace(string(body)))
	}
	decoded, err := base64.StdEncoding.DecodeString(string(body))
	if err != nil {
		return "", fmt.Errorf("decode secret %s: %w", key, err)
	}
	return string(decoded), nil
}

func clientToken(ctx context.Context, doHTTP func(*http.Request) (*http.Response, error), hubURL, clientID, secret string) (string, error) {
	form := url.Values{"grant_type": {"client_credentials"}, "client_id": {clientID}, "client_secret": {secret}}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, hubURL+"/auth/realms/camunda-platform/protocol/openid-connect/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	body, status, err := doRequest(doHTTP, req)
	if err != nil {
		return "", fmt.Errorf("request token for %s: %w", clientID, err)
	}
	if status < 200 || status >= 300 {
		return "", fmt.Errorf("request token for %s: HTTP %d: %s", clientID, status, strings.TrimSpace(string(body)))
	}
	var response struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &response); err != nil || response.AccessToken == "" {
		return "", fmt.Errorf("parse token response for %s: %s", clientID, strings.TrimSpace(string(body)))
	}
	return response.AccessToken, nil
}

func validateTokenAudience(token, expected string, forbidden []string) error {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return errors.New("token is not a JWT")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return fmt.Errorf("decode JWT payload: %w", err)
	}
	var claims struct {
		Audience any `json:"aud"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return fmt.Errorf("parse JWT payload: %w", err)
	}
	audiences := map[string]bool{}
	switch value := claims.Audience.(type) {
	case string:
		audiences[value] = true
	case []any:
		for _, audience := range value {
			if text, ok := audience.(string); ok {
				audiences[text] = true
			}
		}
	}
	if !audiences[expected] {
		return fmt.Errorf("missing %q", expected)
	}
	for _, audience := range forbidden {
		if audiences[audience] {
			return fmt.Errorf("contains forbidden %q", audience)
		}
	}
	return nil
}

func tenantAPIPath(tenant string) string {
	if tenant == "default" {
		return "/orchestration/v2"
	}
	return "/orchestration/physical-tenants/" + tenant + "/v2"
}

func deployAcceptanceProcess(ctx context.Context, doHTTP func(*http.Request) (*http.Response, error), orchURL, tenant, processID, token string) (string, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", `form-data; name="resources"; filename="acceptance.bpmn"`)
	header.Set("Content-Type", "application/vnd.bpmn+xml")
	part, _ := writer.CreatePart(header)
	fmt.Fprintf(part, `<?xml version="1.0" encoding="UTF-8"?><definitions xmlns="http://www.omg.org/spec/BPMN/20100524/MODEL" targetNamespace="http://camunda.io/schema/1.0/bpmn"><process id="%s" name="%s" isExecutable="true"><startEvent id="start"/><sequenceFlow id="flow" sourceRef="start" targetRef="end"/><endEvent id="end"/></process></definitions>`, processID, processID)
	_ = writer.Close()
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, orchURL+tenantAPIPath(tenant)+"/deployments", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	response, status, err := doRequest(doHTTP, req)
	if err != nil {
		return "", fmt.Errorf("deploy %s process: %w", tenant, err)
	}
	if status < 200 || status >= 300 {
		return "", fmt.Errorf("deploy %s process: HTTP %d: %s", tenant, status, strings.TrimSpace(string(response)))
	}
	var result struct {
		Deployments []struct {
			ProcessDefinition struct {
				Key string `json:"processDefinitionKey"`
			} `json:"processDefinition"`
		} `json:"deployments"`
	}
	if err := json.Unmarshal(response, &result); err != nil || len(result.Deployments) == 0 || result.Deployments[0].ProcessDefinition.Key == "" {
		return "", fmt.Errorf("parse %s deployment response: %s", tenant, strings.TrimSpace(string(response)))
	}
	return result.Deployments[0].ProcessDefinition.Key, nil
}

func startAcceptanceProcess(ctx context.Context, doHTTP func(*http.Request) (*http.Response, error), orchURL, tenant, key, marker, token string) error {
	body, _ := json.Marshal(map[string]any{"processDefinitionKey": key, "variables": map[string]string{"physicalTenantAcceptanceMarker": marker}})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, orchURL+tenantAPIPath(tenant)+"/process-instances", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	response, status, err := doRequest(doHTTP, req)
	if err != nil {
		return fmt.Errorf("start %s process: %w", tenant, err)
	}
	if status < 200 || status >= 300 {
		return fmt.Errorf("start %s process: HTTP %d: %s", tenant, status, strings.TrimSpace(string(response)))
	}
	return nil
}

func definitionsContain(body []byte, processID string) bool {
	var definitions []struct{ Key, Name string }
	if json.Unmarshal(body, &definitions) != nil {
		return false
	}
	for _, definition := range definitions {
		if definition.Key == processID || definition.Name == processID {
			return true
		}
	}
	return false
}

func waitForOptimizeImport(ctx context.Context, deps acceptanceDependencies, optimizeURL, tenant, processID, token string, attempts int) error {
	lastDiagnostic := "no response"
	for attempt := 1; attempt <= attempts; attempt++ {
		body, status, err := httpGet(ctx, deps.doHTTP, optimizeURL+"/api/definition/process/keys", token)
		if err != nil {
			lastDiagnostic = "transport error: " + err.Error()
		} else {
			lastDiagnostic = fmt.Sprintf("HTTP %d: %s", status, strings.TrimSpace(string(body)))
			if status == http.StatusOK && definitionsContain(body, processID) {
				return nil
			}
		}
		if attempt < attempts {
			if err := deps.sleep(ctx, 5*time.Second); err != nil {
				return err
			}
		}
	}
	return fmt.Errorf("Optimize for %q did not import %q within five minutes (last response: %s)", tenant, processID, lastDiagnostic)
}

func assertOptimizeExcludesSiblings(ctx context.Context, deps acceptanceDependencies, optimizeURL, tenant, token string, siblings []string, attempts int) error {
	lastDiagnostic := "no response"
	for attempt := 1; attempt <= attempts; attempt++ {
		body, status, err := httpGet(ctx, deps.doHTTP, optimizeURL+"/api/definition/process/keys", token)
		if err != nil {
			lastDiagnostic = "transport error: " + err.Error()
			if attempt < attempts {
				if sleepErr := deps.sleep(ctx, 5*time.Second); sleepErr != nil {
					return sleepErr
				}
				continue
			}
			return fmt.Errorf("Optimize for %q isolation check exhausted transport retries (last response: %s)", tenant, lastDiagnostic)
		}
		lastDiagnostic = fmt.Sprintf("HTTP %d: %s", status, strings.TrimSpace(string(body)))
		if status != http.StatusOK {
			return fmt.Errorf("Optimize for %q returned HTTP %d during isolation check: %s", tenant, status, strings.TrimSpace(string(body)))
		}
		for _, sibling := range siblings {
			if definitionsContain(body, sibling) {
				return fmt.Errorf("Optimize for %q imported sibling process %q", tenant, sibling)
			}
		}
		if attempt < attempts {
			if err := deps.sleep(ctx, 5*time.Second); err != nil {
				return err
			}
		}
	}
	return nil
}

func httpGet(ctx context.Context, doHTTP func(*http.Request) (*http.Response, error), target, token string) ([]byte, int, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return doRequest(doHTTP, req)
}

func doRequest(doHTTP func(*http.Request) (*http.Response, error), req *http.Request) ([]byte, int, error) {
	response, err := doHTTP(req)
	if err != nil {
		return nil, 0, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, response.StatusCode, err
	}
	return body, response.StatusCode, nil
}
