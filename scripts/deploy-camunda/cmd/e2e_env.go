package cmd

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// newE2EEnvCommand groups e2e .env generation subcommands.
func newE2EEnvCommand() *cobra.Command {
	c := &cobra.Command{
		Use:   "e2e-env",
		Short: "Generate e2e .env files",
	}
	c.AddCommand(newE2EEnvMergeCommand())
	return c
}

// newE2EEnvMergeCommand produces a single merged .env for a multi-namespace
// topology deploy: endpoints come from the orchestration namespace (via
// render-e2e-env.sh), while the auth/host vars and credentials are overridden
// to the Hub namespace where Identity/Keycloak run.
func newE2EEnvMergeCommand() *cobra.Command {
	var (
		orchestrationNamespace string
		hubNamespace           string
		optimizeNamespace      string
		optimizeContextPath    string
		chartPath              string
		output                 string
		renderScript           string
		kubeContext            string
		ci                     bool
		runSmokeTests          bool
	)

	cmd := &cobra.Command{
		Use:   "merge",
		Short: "Merge orchestration endpoints with Hub auth into one .env",
		RunE: func(cmd *cobra.Command, args []string) error {
			renderArgs := []string{
				renderScript,
				"--absolute-chart-path", chartPath,
				"--namespace", orchestrationNamespace,
				"--output", output,
			}
			if runSmokeTests {
				renderArgs = append(renderArgs, "--run-smoke-tests")
			}
			if !ci {
				renderArgs = append(renderArgs, "--not-ci")
			}
			if kubeContext != "" {
				renderArgs = append(renderArgs, "--kube-context", kubeContext)
			}
			rc := exec.Command("bash", renderArgs...)
			rc.Stdout = os.Stderr
			rc.Stderr = os.Stderr
			if err := rc.Run(); err != nil {
				return fmt.Errorf("render script failed: %w", err)
			}

			hubHost, err := resolveIngressHost(kubeContext, hubNamespace)
			if err != nil {
				return err
			}

			firstUserPw, err := resolveSecretKey(kubeContext, hubNamespace, "identity-user-password")
			if err != nil {
				return err
			}
			kcAdminPw, err := resolveSecretKey(kubeContext, hubNamespace, "identity-keycloak-admin-password")
			if err != nil {
				return err
			}
			clientSecret, err := resolveSecretKey(kubeContext, hubNamespace, "client-secret")
			if err != nil {
				return err
			}

			tokenURL := "https://" + hubHost + "/auth/realms/camunda-platform/protocol/openid-connect/token"
			overrides := map[string]string{
				"MANAGEMENT_BASE_URL":              "https://" + hubHost,
				"MANAGEMENT_IDENTITY_CONTEXT_PATH": "https://" + hubHost + "/identity",
				"MODELER_CONTEXT_PATH":             "https://" + hubHost + "/modeler",
				"CONSOLE_CONTEXT_PATH":             "https://" + hubHost + "/modeler",
				"CONSOLE_BASE_URL":                 "https://" + hubHost,
				"IDENTITY_BASE_URL":                "https://" + hubHost + "/identity/",
				"KEYCLOAK_BASE_URL":                "https://" + hubHost + "/auth",
				"KEYCLOAK_URL":                     "https://" + hubHost,
				"WEBMODELER_BASE_URL":              "https://" + hubHost + "/modeler",
				"OAUTH_URL":                        tokenURL,
				"AUTH_URL":                         tokenURL,
				"DISTRO_QA_E2E_TESTS_IDENTITY_FIRSTUSER_PASSWORD": firstUserPw,
				"DISTRO_QA_E2E_TESTS_KEYCLOAK_PASSWORD":           kcAdminPw,
				"DISTRO_QA_E2E_TESTS_KEYCLOAK_CLIENTS_SECRET":     clientSecret,
			}

			if optimizeNamespace != "" {
				if err := assertOptimizeDeployed(kubeContext, optimizeNamespace); err != nil {
					return err
				}
				optimizeHost, err := resolveIngressHost(kubeContext, optimizeNamespace)
				if err != nil {
					return fmt.Errorf("optimize namespace %q: %w", optimizeNamespace, err)
				}
				overrides["CAMUNDA_OPTIMIZE_BASE_URL"] = "https://" + optimizeHost + optimizeContextPath
				overrides["IS_OPTIMIZE"] = "true"
			}

			content, err := os.ReadFile(output)
			if err != nil {
				return err
			}
			merged := mergeEnvOverrides(string(content), overrides)
			if optimizeNamespace != "" {
				if err := assertOptimizeEnabled(merged, optimizeNamespace); err != nil {
					return err
				}
			}
			if err := os.WriteFile(output, []byte(merged), 0o600); err != nil {
				return err
			}
			if err := os.Chmod(output, 0o600); err != nil {
				return err
			}

			fmt.Fprintf(os.Stderr, "merged e2e env: hubHost=%s overrode %d keys -> %s\n", hubHost, len(overrides), output)
			return nil
		},
	}

	cmd.Flags().StringVar(&orchestrationNamespace, "orchestration-namespace", "", "orchestration release namespace")
	cmd.Flags().StringVar(&hubNamespace, "hub-namespace", "", "Hub release namespace")
	cmd.Flags().StringVar(&optimizeNamespace, "optimize-namespace", "", "namespace of the Optimize-only release serving this leg; render-e2e-env.sh derives Optimize from the orchestration namespace, which is not where Optimize runs in a topology using the optimize release role")
	cmd.Flags().StringVar(&optimizeContextPath, "optimize-context-path", "", "ingress path the Optimize-only release is served on, e.g. /optimize-orcha")
	cmd.Flags().StringVar(&chartPath, "absolute-chart-path", "", "absolute chart path")
	cmd.Flags().StringVar(&output, "output", ".env", "output .env path")
	cmd.Flags().StringVar(&renderScript, "render-script", "scripts/render-e2e-env.sh", "path to render-e2e-env.sh")
	cmd.Flags().StringVar(&kubeContext, "kube-context", "", "kube context (optional)")
	cmd.Flags().BoolVar(&ci, "ci", false, "set CI=true in the merged .env (matches render-e2e-env.sh's default; pass when running in an actual CI job)")
	cmd.Flags().BoolVar(&runSmokeTests, "run-smoke-tests", true, "pass --run-smoke-tests to render-e2e-env.sh (sets IS_SMOKE=true)")
	_ = cmd.MarkFlagRequired("orchestration-namespace")
	_ = cmd.MarkFlagRequired("hub-namespace")
	_ = cmd.MarkFlagRequired("absolute-chart-path")

	return cmd
}

// assertOptimizeDeployed fails when a leg declares an Optimize namespace that
// runs no Optimize workload. Without it the merged env would point at nothing
// and, because render-e2e-env.sh only sets IS_OPTIMIZE from the orchestration
// namespace and every Optimize spec is guarded on that variable, the suite
// would report success having silently skipped Optimize entirely.
func assertOptimizeDeployed(kubeContext, namespace string) error {
	args := []string{}
	if kubeContext != "" {
		args = append(args, "--context", kubeContext)
	}
	args = append(args, "-n", namespace, "get", "deployment",
		"-l", "app.kubernetes.io/component=optimize",
		"-o", "jsonpath={.items[*].metadata.name}")
	out, err := exec.Command("kubectl", args...).Output()
	if err != nil {
		return fmt.Errorf("look for an Optimize deployment in namespace %q: %w", namespace, err)
	}
	if len(strings.Fields(string(out))) == 0 {
		return fmt.Errorf("no Optimize deployment found in namespace %q; every Optimize spec is guarded on IS_OPTIMIZE, so continuing would skip Optimize coverage and still report success", namespace)
	}
	return nil
}

// assertOptimizeEnabled guards the merge itself: the rendered env carries
// IS_OPTIMIZE=false for a topology whose Optimize runs outside the
// orchestration namespace, and that value must have been overridden.
func assertOptimizeEnabled(merged, namespace string) error {
	for _, line := range strings.Split(merged, "\n") {
		if strings.TrimSpace(line) == "IS_OPTIMIZE=false" {
			return fmt.Errorf("merged env still sets IS_OPTIMIZE=false while Optimize runs in namespace %q; Optimize specs would be skipped", namespace)
		}
	}
	return nil
}

// resolveSecretKey reads and base64-decodes one key from the Hub
// namespace's shared integration-test-credentials secret. This is the secret
// that actually backs the users under ExternalSecrets, so it is authoritative
// (render-e2e-env.sh's DISTRO_QA_* values diverge from it in the topology path).
func resolveSecretKey(kubeContext, namespace, key string) (string, error) {
	a := []string{}
	if kubeContext != "" {
		a = append(a, "--context", kubeContext)
	}
	a = append(a, "-n", namespace, "get", "secret", "integration-test-credentials",
		"-o", fmt.Sprintf("jsonpath={.data['%s']}", key))
	out, err := exec.Command("kubectl", a...).Output()
	if err != nil {
		return "", fmt.Errorf("resolve secret key %q in %s: %w", key, namespace, err)
	}
	dec, err := decodeSecretValue(string(out))
	if err != nil {
		return "", fmt.Errorf("decode secret key %q: %w", key, err)
	}
	if dec == "" {
		return "", fmt.Errorf("secret key %q in %s resolved to an empty value", key, namespace)
	}
	return dec, nil
}

// selectIngressHost filters raw whitespace-separated ingress/gateway host
// tokens (as emitted by a kubectl jsonpath query), dropping hosts that look
// like the Zeebe gRPC gateway, and returning the first remaining host.
// Returns "" if no host survives the filter.
func selectIngressHost(raw string) (string, error) {
	tokens := strings.Fields(raw)
	host := ""
	for _, t := range tokens {
		if strings.Contains(t, "zeebe") || strings.Contains(t, "grpc") {
			continue
		}
		if host == "" {
			host = t
			continue
		}
		if t != host {
			return "", fmt.Errorf("multiple distinct HTTP hosts found: %q and %q", host, t)
		}
	}
	return host, nil
}

// resolveIngressHost discovers the live ingress hostname for a namespace by
// querying the cluster directly, since CI ingress hosts are assigned by a
// hash-based scheme at deploy time and cannot be reconstructed from the
// namespace name. Falls back to the Gateway API when no Ingress host is
// found (e.g. on a mesh-based cluster without classic Ingress objects).
func resolveIngressHost(kubeContext, namespace string) (string, error) {
	ingressArgs := []string{}
	if kubeContext != "" {
		ingressArgs = append(ingressArgs, "--context", kubeContext)
	}
	ingressArgs = append(ingressArgs, "-n", namespace, "get", "ingress",
		"-o", "jsonpath={.items[*].spec.rules[*].host}")
	out, err := exec.Command("kubectl", ingressArgs...).Output()
	if err != nil {
		return "", fmt.Errorf("resolve ingress host for namespace %q: %w", namespace, err)
	}
	host, err := selectIngressHost(string(out))
	if err != nil {
		return "", fmt.Errorf("resolve ingress host for namespace %q: %w", namespace, err)
	}
	if host != "" {
		return host, nil
	}

	gatewayArgs := []string{}
	if kubeContext != "" {
		gatewayArgs = append(gatewayArgs, "--context", kubeContext)
	}
	gatewayArgs = append(gatewayArgs, "-n", namespace, "get", "gateway",
		"-o", "jsonpath={.items[*].spec.listeners[*].hostname}")
	if gwOut, err := exec.Command("kubectl", gatewayArgs...).Output(); err == nil {
		host, selectErr := selectIngressHost(string(gwOut))
		if selectErr != nil {
			return "", fmt.Errorf("resolve gateway host for namespace %q: %w", namespace, selectErr)
		}
		if host != "" {
			return host, nil
		}
	}

	return "", fmt.Errorf("resolve ingress host for namespace %q: no non-zeebe/grpc ingress or gateway host found", namespace)
}

// decodeSecretValue base64-decodes a (possibly whitespace-padded) Kubernetes
// secret data value. Extracted so the decode-failure path is unit-testable
// without shelling out to kubectl.
func decodeSecretValue(raw string) (string, error) {
	dec, err := base64.StdEncoding.DecodeString(strings.TrimSpace(raw))
	if err != nil {
		return "", err
	}
	return string(dec), nil
}

// mergeEnvOverrides replaces matching KEY= lines in content with the override
// values (preserving order and all other lines), then appends any override
// keys not already present in sorted order. Trailing newline is preserved.
func mergeEnvOverrides(content string, overrides map[string]string) string {
	hadTrailingNewline := strings.HasSuffix(content, "\n")
	lines := strings.Split(content, "\n")
	if hadTrailingNewline && len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	applied := map[string]bool{}
	for i, line := range lines {
		idx := strings.Index(line, "=")
		if idx <= 0 {
			continue
		}
		key := line[:idx]
		if v, ok := overrides[key]; ok {
			lines[i] = key + "=" + v
			applied[key] = true
		}
	}
	remaining := make([]string, 0, len(overrides))
	for k := range overrides {
		if !applied[k] {
			remaining = append(remaining, k)
		}
	}
	sort.Strings(remaining)
	for _, k := range remaining {
		lines = append(lines, k+"="+overrides[k])
	}
	result := strings.Join(lines, "\n")
	if hadTrailingNewline && !strings.HasSuffix(result, "\n") {
		result += "\n"
	}
	return result
}
