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
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// apiSuiteKeys are the variables `e2e-env api-suite` may set. A value already
// in the caller's environment wins, so a run can override any of them.
var apiSuiteKeys = []string{
	"API_CONNECTORS_URL", "API_CONSOLE_URL", "API_IDENTITY_URL", "API_WEB_MODELER_URL",
	"AUTH_METHOD", "BASE_URL", "BASIC_AUTH_PASSWORD", "BASIC_AUTH_USER",
	"CLIENT_ID", "CLIENT_SECRET", "GRPC_ADDRESS", "MT", "REQUIRE_API_TEST_SUITE",
	"SECONDARY_CLIENT_ID", "SECONDARY_CLIENT_SECRET", "TOKEN_URL", "ZEEBE_VERSION",
}

var minorVersionPattern = regexp.MustCompile(`[0-9]+\.[0-9]+`)

type apiSuiteInputs struct {
	// Env is the rendered e2e .env of the deployment.
	Env map[string]string
	// Auth is the scenario's auth type (TEST_AUTH_TYPE); "basic" selects basic auth.
	Auth         string
	MultiTenancy bool
	// Preset holds values the caller already set, which are kept as they are.
	Preset map[string]string
}

// clientSecretLookup returns the Keycloak secret of a client declared in the
// chart's identity.clients, or "" when the deployment does not declare it.
type clientSecretLookup func(clientID string) (string, error)

// apiSuiteEnv computes the variables of the REST v2 API suite in
// @camunda/e2e-test-suite (tests/api/README.md there) for one deployment. It
// returns the variables and the values among them that are secrets.
func apiSuiteEnv(in apiSuiteInputs, lookup clientSecretLookup) (map[string]string, []string, error) {
	baseURL := strings.TrimRight(in.Env["PLAYWRIGHT_BASE_URL"], "/")
	if baseURL == "" {
		return nil, nil, fmt.Errorf("the e2e env has no PLAYWRIGHT_BASE_URL")
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Hostname() == "" {
		return nil, nil, fmt.Errorf("PLAYWRIGHT_BASE_URL %q has no host", baseURL)
	}
	zeebeVersion := minorVersionPattern.FindString(in.Env["MINOR_VERSION"])
	if zeebeVersion == "" {
		return nil, nil, fmt.Errorf("MINOR_VERSION %q names no minor version", in.Env["MINOR_VERSION"])
	}

	vars := map[string]string{
		"BASE_URL":               baseURL + "/orchestration",
		"ZEEBE_VERSION":          zeebeVersion,
		"MT":                     fmt.Sprintf("%t", in.MultiTenancy),
		"REQUIRE_API_TEST_SUITE": "true",
		// The gRPC ingress host the CI values configure (orchestration.ingress.grpc).
		"GRPC_ADDRESS": "grpc-" + parsed.Hostname() + ":443",
	}
	if connectors := in.Env["CONNECTORS_BASE_URL"]; connectors != "" {
		vars["API_CONNECTORS_URL"] = strings.TrimSuffix(strings.TrimRight(connectors, "/"), "/inbound")
	}

	var secrets []string
	if in.Auth == "basic" {
		// The chart's default initial admin (orchestration.security.initialization.users).
		vars["AUTH_METHOD"] = "basic"
		vars["BASIC_AUTH_USER"] = "demo"
		vars["BASIC_AUTH_PASSWORD"] = "demo"
	} else {
		vars["AUTH_METHOD"] = "oauth2"
		vars["TOKEN_URL"] = in.Env["OAUTH_URL"]
		for key, envKey := range map[string]string{
			"API_IDENTITY_URL":    "IDENTITY_BASE_URL",
			"API_CONSOLE_URL":     "CONSOLE_BASE_URL",
			"API_WEB_MODELER_URL": "WEBMODELER_BASE_URL",
		} {
			if v := in.Env[envKey]; v != "" {
				vars[key] = v
			}
		}

		secret, err := lookup("venom")
		if err != nil {
			return nil, nil, err
		}
		if secret == "" {
			return nil, nil, fmt.Errorf("the identity deployment declares no secret for the venom client")
		}
		vars["CLIENT_ID"] = "venom"
		vars["CLIENT_SECRET"] = secret
		secrets = append(secrets, secret)

		// The CI values' role-less client; without it the enforcement tests skip.
		secondary, err := lookup("unprivileged")
		if err != nil {
			return nil, nil, err
		}
		if secondary != "" {
			vars["SECONDARY_CLIENT_ID"] = "unprivileged"
			vars["SECONDARY_CLIENT_SECRET"] = secondary
			secrets = append(secrets, secondary)
		}
	}

	for key, value := range in.Preset {
		if value != "" {
			if _, managed := vars[key]; managed {
				vars[key] = value
			}
		}
	}
	return vars, secrets, nil
}

// parseEnvFile reads KEY=VALUE lines, ignoring blanks, comments and lines
// without "=".
func parseEnvFile(content string) map[string]string {
	env := map[string]string{}
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok || key == "" {
			continue
		}
		env[key] = value
	}
	return env
}

// shellEnv renders vars as sorted KEY='value' lines that bash can source
// without expanding anything inside a value.
func shellEnv(vars map[string]string) string {
	keys := make([]string, 0, len(vars))
	for k := range vars {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&b, "%s='%s'\n", k, strings.ReplaceAll(vars[k], "'", `'\''`))
	}
	return b.String()
}

// identityClientSecret resolves VALUES_<CLIENT>_CLIENT_SECRET on the identity
// deployment, from its value or the secret it references, as
// render-e2e-env.sh's resolve_env_password does.
func identityClientSecret(kubeContext, namespace, clientID string) (string, error) {
	envName := "VALUES_" + strings.ToUpper(clientID) + "_CLIENT_SECRET"
	field := func(path string) (string, error) {
		args := []string{}
		if kubeContext != "" {
			args = append(args, "--context", kubeContext)
		}
		args = append(args, "-n", namespace, "get", "deployment",
			"-l", "app.kubernetes.io/component=identity",
			"-o", fmt.Sprintf(`jsonpath={.items[0].spec.template.spec.containers[0].env[?(@.name=="%s")]%s}`, envName, path))
		out, err := exec.Command("kubectl", args...).Output()
		if err != nil {
			return "", fmt.Errorf("read %s from the identity deployment in %s: %w", envName, namespace, err)
		}
		return strings.TrimSpace(string(out)), nil
	}

	if value, err := field(".value"); err != nil || value != "" {
		return value, err
	}
	secretName, err := field(".valueFrom.secretKeyRef.name")
	if err != nil || secretName == "" {
		return "", err
	}
	secretKey, err := field(".valueFrom.secretKeyRef.key")
	if err != nil {
		return "", err
	}
	args := []string{}
	if kubeContext != "" {
		args = append(args, "--context", kubeContext)
	}
	args = append(args, "-n", namespace, "get", "secret", secretName,
		"-o", fmt.Sprintf("jsonpath={.data['%s']}", secretKey))
	out, err := exec.Command("kubectl", args...).Output()
	if err != nil {
		return "", fmt.Errorf("read %s/%s in %s: %w", secretName, secretKey, namespace, err)
	}
	return decodeSecretValue(string(out))
}

func newE2EEnvAPISuiteCommand() *cobra.Command {
	var (
		envFile      string
		namespace    string
		kubeContext  string
		auth         string
		multiTenancy bool
		output       string
		ci           bool
	)

	cmd := &cobra.Command{
		Use:   "api-suite",
		Short: "Write the REST v2 API suite's variables for a deployment, for bash to source",
		RunE: func(cmd *cobra.Command, args []string) error {
			content, err := os.ReadFile(envFile)
			if err != nil {
				return err
			}
			preset := map[string]string{}
			for _, key := range apiSuiteKeys {
				preset[key] = os.Getenv(key)
			}
			vars, secrets, err := apiSuiteEnv(apiSuiteInputs{
				Env:          parseEnvFile(string(content)),
				Auth:         auth,
				MultiTenancy: multiTenancy,
				Preset:       preset,
			}, func(clientID string) (string, error) {
				return identityClientSecret(kubeContext, namespace, clientID)
			})
			if err != nil {
				return err
			}
			if ci {
				for _, s := range secrets {
					fmt.Fprintf(cmd.OutOrStdout(), "::add-mask::%s\n", s)
				}
			}
			if err := os.WriteFile(output, []byte(shellEnv(vars)), 0o600); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "REST v2 API suite: %s, gRPC %s (version %s, auth %s, multi-tenancy %s)\n",
				vars["BASE_URL"], vars["GRPC_ADDRESS"], vars["ZEEBE_VERSION"], vars["AUTH_METHOD"], vars["MT"])
			return nil
		},
	}

	cmd.Flags().StringVar(&envFile, "env-file", "", "the deployment's rendered e2e .env")
	cmd.Flags().StringVar(&namespace, "namespace", "", "namespace of the deployment")
	cmd.Flags().StringVar(&kubeContext, "kube-context", "", "kube context (optional)")
	cmd.Flags().StringVar(&auth, "auth", "", `the scenario's auth type; "basic" runs the suite with basic auth`)
	cmd.Flags().BoolVar(&multiTenancy, "mt", false, "the deployment has multi-tenancy enabled")
	cmd.Flags().StringVar(&output, "output", "", "file to write the variables to")
	cmd.Flags().BoolVar(&ci, "ci", false, "print ::add-mask:: lines for the secrets")
	_ = cmd.MarkFlagRequired("env-file")
	_ = cmd.MarkFlagRequired("namespace")
	_ = cmd.MarkFlagRequired("output")
	return cmd
}
