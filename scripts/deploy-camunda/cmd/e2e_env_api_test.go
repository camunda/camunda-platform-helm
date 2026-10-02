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
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func renderedEnv() map[string]string {
	return map[string]string{
		"PLAYWRIGHT_BASE_URL": "https://ci.example.com",
		"BASE_URL":            "https://ci.example.com",
		"OAUTH_URL":           "https://ci.example.com/auth/realms/camunda-platform/protocol/openid-connect/token",
		"MINOR_VERSION":       "SM-8.10",
		"CONNECTORS_BASE_URL": "https://ci.example.com/connectors/inbound",
		"IDENTITY_BASE_URL":   "https://ci.example.com/identity/",
		"CONSOLE_BASE_URL":    "https://ci.example.com",
		"WEBMODELER_BASE_URL": "https://ci.example.com/modeler",
	}
}

func secretsOf(clients map[string]string) clientSecretLookup {
	return func(clientID string) (string, error) { return clients[clientID], nil }
}

func TestAPISuiteEnvOAuth2(t *testing.T) {
	vars, secrets, err := apiSuiteEnv(apiSuiteInputs{Env: renderedEnv()},
		secretsOf(map[string]string{"venom": "v-secret", "unprivileged": "u-secret"}))
	if err != nil {
		t.Fatalf("apiSuiteEnv: %v", err)
	}
	want := map[string]string{
		"BASE_URL":                "https://ci.example.com/orchestration",
		"ZEEBE_VERSION":           "8.10",
		"MT":                      "false",
		"REQUIRE_API_TEST_SUITE":  "true",
		"GRPC_ADDRESS":            "grpc-ci.example.com:443",
		"API_CONNECTORS_URL":      "https://ci.example.com/connectors",
		"AUTH_METHOD":             "oauth2",
		"TOKEN_URL":               "https://ci.example.com/auth/realms/camunda-platform/protocol/openid-connect/token",
		"API_IDENTITY_URL":        "https://ci.example.com/identity/",
		"API_CONSOLE_URL":         "https://ci.example.com",
		"API_WEB_MODELER_URL":     "https://ci.example.com/modeler",
		"CLIENT_ID":               "venom",
		"CLIENT_SECRET":           "v-secret",
		"SECONDARY_CLIENT_ID":     "unprivileged",
		"SECONDARY_CLIENT_SECRET": "u-secret",
	}
	if !reflect.DeepEqual(vars, want) {
		t.Errorf("vars =\n%v\nwant\n%v", vars, want)
	}
	if !reflect.DeepEqual(secrets, []string{"v-secret", "u-secret"}) {
		t.Errorf("secrets = %v, want both client secrets", secrets)
	}
}

func TestAPISuiteEnvWithoutUnprivilegedClientLeavesEnforcementOff(t *testing.T) {
	vars, _, err := apiSuiteEnv(apiSuiteInputs{Env: renderedEnv()},
		secretsOf(map[string]string{"venom": "v-secret"}))
	if err != nil {
		t.Fatalf("apiSuiteEnv: %v", err)
	}
	for _, key := range []string{"SECONDARY_CLIENT_ID", "SECONDARY_CLIENT_SECRET"} {
		if _, ok := vars[key]; ok {
			t.Errorf("%s is set without an unprivileged client", key)
		}
	}
}

func TestAPISuiteEnvRequiresVenom(t *testing.T) {
	_, _, err := apiSuiteEnv(apiSuiteInputs{Env: renderedEnv()}, secretsOf(nil))
	if err == nil || !strings.Contains(err.Error(), "venom") {
		t.Fatalf("err = %v, want a missing venom secret error", err)
	}
}

func TestAPISuiteEnvPropagatesLookupErrors(t *testing.T) {
	boom := errors.New("kubectl failed")
	_, _, err := apiSuiteEnv(apiSuiteInputs{Env: renderedEnv()},
		func(string) (string, error) { return "", boom })
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want the lookup error", err)
	}
}

func TestAPISuiteEnvBasicAuth(t *testing.T) {
	called := false
	vars, secrets, err := apiSuiteEnv(apiSuiteInputs{Env: renderedEnv(), Auth: "basic", MultiTenancy: true},
		func(string) (string, error) { called = true; return "", nil })
	if err != nil {
		t.Fatalf("apiSuiteEnv: %v", err)
	}
	if called {
		t.Error("basic auth looked up a Keycloak client secret")
	}
	if vars["AUTH_METHOD"] != "basic" || vars["BASIC_AUTH_USER"] != "demo" || vars["BASIC_AUTH_PASSWORD"] != "demo" {
		t.Errorf("basic auth vars = %v", vars)
	}
	if vars["MT"] != "true" {
		t.Errorf("MT = %q, want true", vars["MT"])
	}
	for _, key := range []string{"CLIENT_ID", "TOKEN_URL", "API_IDENTITY_URL", "API_CONSOLE_URL", "API_WEB_MODELER_URL"} {
		if _, ok := vars[key]; ok {
			t.Errorf("%s is set on a basic-auth run", key)
		}
	}
	if vars["API_CONNECTORS_URL"] == "" {
		t.Error("API_CONNECTORS_URL is missing; Connectors needs no token")
	}
	if len(secrets) != 0 {
		t.Errorf("secrets = %v, want none", secrets)
	}
}

func TestAPISuiteEnvKeepsCallerValues(t *testing.T) {
	vars, _, err := apiSuiteEnv(apiSuiteInputs{
		Env:    renderedEnv(),
		Preset: map[string]string{"GRPC_ADDRESS": "localhost:26500", "BASE_URL": "", "UNRELATED": "x"},
	}, secretsOf(map[string]string{"venom": "v"}))
	if err != nil {
		t.Fatalf("apiSuiteEnv: %v", err)
	}
	if vars["GRPC_ADDRESS"] != "localhost:26500" {
		t.Errorf("GRPC_ADDRESS = %q, want the caller's value", vars["GRPC_ADDRESS"])
	}
	if vars["BASE_URL"] != "https://ci.example.com/orchestration" {
		t.Errorf("BASE_URL = %q; an empty caller value must not win", vars["BASE_URL"])
	}
	if _, ok := vars["UNRELATED"]; ok {
		t.Error("a caller value outside the managed keys was added")
	}
}

func TestAPISuiteEnvIgnoresValuesSourcedFromTheRenderedEnv(t *testing.T) {
	// given run-e2e-tests.sh has sourced the rendered .env, so BASE_URL in the
	// caller's environment is the browser suite's ingress root
	env := renderedEnv()

	// when
	vars, _, err := apiSuiteEnv(apiSuiteInputs{
		Env:    env,
		Preset: map[string]string{"BASE_URL": env["BASE_URL"]},
	}, secretsOf(map[string]string{"venom": "v"}))

	// then
	if err != nil {
		t.Fatalf("apiSuiteEnv: %v", err)
	}
	if vars["BASE_URL"] != "https://ci.example.com/orchestration" {
		t.Errorf("BASE_URL = %q, want the API suite's /orchestration URL", vars["BASE_URL"])
	}
}

func TestE2EEnvAPISuiteCommandAfterSourcingTheRenderedEnv(t *testing.T) {
	// given the environment run-e2e-tests.sh has after sourcing the rendered .env
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	var rendered strings.Builder
	for key, value := range renderedEnv() {
		rendered.WriteString(key + "=" + value + "\n")
		t.Setenv(key, value)
	}
	writeFile(t, envFile, rendered.String())
	fakeKubectl(t, map[string]string{"value": "v-secret"}, "")
	output := filepath.Join(dir, ".env.api")

	// when
	root := NewRootCommand()
	root.AddCommand(newE2EEnvCommand())
	root.SetOut(&strings.Builder{})
	root.SetArgs([]string{"e2e-env", "api-suite", "--env-file", envFile, "--namespace", "ns", "--output", output})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	// then
	written, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if want := "BASE_URL='https://ci.example.com/orchestration'\n"; !strings.Contains(string(written), want) {
		t.Errorf("output lacks %q:\n%s", want, written)
	}
}

func TestAPISuiteEnvRejectsIncompleteEnv(t *testing.T) {
	for name, mutate := range map[string]func(map[string]string){
		"no base url":      func(e map[string]string) { delete(e, "PLAYWRIGHT_BASE_URL") },
		"no minor version": func(e map[string]string) { e["MINOR_VERSION"] = "SNAPSHOT" },
	} {
		t.Run(name, func(t *testing.T) {
			env := renderedEnv()
			mutate(env)
			if _, _, err := apiSuiteEnv(apiSuiteInputs{Env: env}, secretsOf(map[string]string{"venom": "v"})); err == nil {
				t.Fatal("want an error")
			}
		})
	}
}

func TestParseEnvFile(t *testing.T) {
	got := parseEnvFile("# comment\nA=1\n\nB=x=y\nnot a pair\n=empty\n")
	want := map[string]string{"A": "1", "B": "x=y"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseEnvFile = %v, want %v", got, want)
	}
}

// The output is sourced by bash, so a value must come back byte for byte.
func TestShellEnvRoundTripsThroughBash(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}
	value := `it's $HOME "quoted" ` + "`cmd`"
	file := filepath.Join(t.TempDir(), "api.env")
	if err := os.WriteFile(file, []byte(shellEnv(map[string]string{"SECRET": value, "A": "1"})), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("bash", "-c", `set -a; source "$0"; set +a; printf '%s' "$SECRET"`, file).Output()
	if err != nil {
		t.Fatalf("bash: %v", err)
	}
	if string(out) != value {
		t.Fatalf("sourced value = %q, want %q", out, value)
	}
}

// fakeKubectl puts a kubectl on PATH that logs its arguments and answers the
// identity deployment and Secret reads with the given responses.
func fakeKubectl(t *testing.T, responses map[string]string, failOn string) (logPath string) {
	t.Helper()
	dir := t.TempDir()
	logPath = filepath.Join(dir, "calls.log")
	for name, body := range responses {
		writeFile(t, filepath.Join(dir, name), body)
	}
	script := `#!/bin/sh
printf '%s\n' "$*" >> "` + logPath + `"
if [ -n "$KUBECTL_FAIL_ON" ]; then
  case "$*" in *"$KUBECTL_FAIL_ON"*) echo "boom" >&2; exit 1 ;; esac
fi
case "$*" in
  *" get secret "*) cat "` + dir + `/secret" 2>/dev/null ;;
  *'.valueFrom.secretKeyRef.name}') cat "` + dir + `/name" 2>/dev/null ;;
  *'.valueFrom.secretKeyRef.key}') cat "` + dir + `/key" 2>/dev/null ;;
  *'.value}') cat "` + dir + `/value" 2>/dev/null ;;
esac
exit 0
`
	writeFile(t, filepath.Join(dir, "kubectl"), script)
	if err := os.Chmod(filepath.Join(dir, "kubectl"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("KUBECTL_FAIL_ON", failOn)
	return logPath
}

func kubectlCalls(t *testing.T, logPath string) []string {
	t.Helper()
	raw, err := os.ReadFile(logPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
}

func TestIdentityClientSecret(t *testing.T) {
	const deploymentRead = `-n ns get deployment -l app.kubernetes.io/component=identity -o jsonpath={.items[0].spec.template.spec.containers[0].env[?(@.name=="VALUES_VENOM_CLIENT_SECRET")]`
	tests := []struct {
		name        string
		kubeContext string
		responses   map[string]string
		failOn      string
		want        string
		wantErr     string
		wantCalls   []string
	}{
		{
			name:      "inline value",
			responses: map[string]string{"value": "inline-secret\n"},
			want:      "inline-secret",
			wantCalls: []string{deploymentRead + ".value}"},
		},
		{
			name:        "Secret reference with kube context",
			kubeContext: "gke-ctx",
			responses: map[string]string{
				"name":   "integration-test-credentials",
				"key":    "identity-admin-client-password",
				"secret": "cEBzcyd3b3Jk", // p@ss'word
			},
			want: "p@ss'word",
			wantCalls: []string{
				"--context gke-ctx " + deploymentRead + ".value}",
				"--context gke-ctx " + deploymentRead + ".valueFrom.secretKeyRef.name}",
				"--context gke-ctx " + deploymentRead + ".valueFrom.secretKeyRef.key}",
				"--context gke-ctx -n ns get secret integration-test-credentials -o jsonpath={.data['identity-admin-client-password']}",
			},
		},
		{
			name: "client not configured",
			want: "",
			wantCalls: []string{
				deploymentRead + ".value}",
				deploymentRead + ".valueFrom.secretKeyRef.name}",
			},
		},
		{
			name:      "deployment read fails",
			failOn:    "get deployment",
			wantErr:   "read VALUES_VENOM_CLIENT_SECRET from the identity deployment in ns",
			wantCalls: []string{deploymentRead + ".value}"},
		},
		{
			name:      "Secret read fails",
			responses: map[string]string{"name": "creds", "key": "pw"},
			failOn:    "get secret",
			wantErr:   "read creds/pw in ns",
		},
		{
			name:      "Secret value is not base64",
			responses: map[string]string{"name": "creds", "key": "pw", "secret": "not base64!"},
			wantErr:   "illegal base64",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logPath := fakeKubectl(t, tt.responses, tt.failOn)

			got, err := identityClientSecret(tt.kubeContext, "ns", "venom")

			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want it to contain %q", err, tt.wantErr)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("secret = %q, want %q", got, tt.want)
			}
			if tt.wantCalls != nil {
				if calls := kubectlCalls(t, logPath); !reflect.DeepEqual(calls, tt.wantCalls) {
					t.Errorf("kubectl calls:\n%s\nwant:\n%s", strings.Join(calls, "\n"), strings.Join(tt.wantCalls, "\n"))
				}
			}
		})
	}
}
