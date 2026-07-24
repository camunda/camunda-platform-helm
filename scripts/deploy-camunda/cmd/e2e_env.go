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
// to the management namespace where Identity/Keycloak run.
func newE2EEnvMergeCommand() *cobra.Command {
	var (
		orchestrationNamespace string
		managementNamespace    string
		chartPath              string
		output                 string
		renderScript           string
		kubeContext            string
		ci                     bool
		runSmokeTests          bool
	)

	cmd := &cobra.Command{
		Use:   "merge",
		Short: "Merge orchestration endpoints with management-plane auth into one .env",
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

			mgmtHost, err := resolveIngressHost(kubeContext, managementNamespace)
			if err != nil {
				return err
			}

			firstUserPw, err := resolveSecretKey(kubeContext, managementNamespace, "identity-user-password")
			if err != nil {
				return err
			}
			kcAdminPw, err := resolveSecretKey(kubeContext, managementNamespace, "identity-keycloak-admin-password")
			if err != nil {
				return err
			}
			clientSecret, err := resolveSecretKey(kubeContext, managementNamespace, "client-secret")
			if err != nil {
				return err
			}

			tokenURL := "https://" + mgmtHost + "/auth/realms/camunda-platform/protocol/openid-connect/token"
			overrides := map[string]string{
				"MANAGEMENT_BASE_URL": "https://" + mgmtHost,
				"KEYCLOAK_URL":        "https://" + mgmtHost,
				"OAUTH_URL":           tokenURL,
				"AUTH_URL":            tokenURL,
				"DISTRO_QA_E2E_TESTS_IDENTITY_FIRSTUSER_PASSWORD": firstUserPw,
				"DISTRO_QA_E2E_TESTS_KEYCLOAK_PASSWORD":           kcAdminPw,
				"DISTRO_QA_E2E_TESTS_KEYCLOAK_CLIENTS_SECRET":     clientSecret,
			}

			content, err := os.ReadFile(output)
			if err != nil {
				return err
			}
			merged := mergeEnvOverrides(string(content), overrides)
			if err := os.WriteFile(output, []byte(merged), 0o600); err != nil {
				return err
			}
			if err := os.Chmod(output, 0o600); err != nil {
				return err
			}

			fmt.Fprintf(os.Stderr, "merged e2e env: mgmtHost=%s overrode %d keys -> %s\n", mgmtHost, len(overrides), output)
			return nil
		},
	}

	cmd.Flags().StringVar(&orchestrationNamespace, "orchestration-namespace", "", "orchestration release namespace")
	cmd.Flags().StringVar(&managementNamespace, "management-namespace", "", "management release namespace")
	cmd.Flags().StringVar(&chartPath, "absolute-chart-path", "", "absolute chart path")
	cmd.Flags().StringVar(&output, "output", ".env", "output .env path")
	cmd.Flags().StringVar(&renderScript, "render-script", "scripts/render-e2e-env.sh", "path to render-e2e-env.sh")
	cmd.Flags().StringVar(&kubeContext, "kube-context", "", "kube context (optional)")
	cmd.Flags().BoolVar(&ci, "ci", false, "set CI=true in the merged .env (matches render-e2e-env.sh's default; pass when running in an actual CI job)")
	cmd.Flags().BoolVar(&runSmokeTests, "run-smoke-tests", true, "pass --run-smoke-tests to render-e2e-env.sh (sets IS_SMOKE=true)")
	_ = cmd.MarkFlagRequired("orchestration-namespace")
	_ = cmd.MarkFlagRequired("management-namespace")
	_ = cmd.MarkFlagRequired("absolute-chart-path")

	return cmd
}

// resolveSecretKey reads and base64-decodes one key from the management
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
// tokens (as emitted by a kubectl jsonpath query), dropping any host that
// looks like the Zeebe gRPC gateway, de-duplicates repeats (a namespace's
// Ingress can list the same host across multiple rules, e.g. in a
// shared-host multi-namespace topology), and joins what remains with a
// comma, preserving first-seen order. Returns "" if no host survives the
// filter.
func selectIngressHost(raw string) string {
	tokens := strings.Fields(raw)
	kept := make([]string, 0, len(tokens))
	seen := make(map[string]bool, len(tokens))
	for _, t := range tokens {
		if strings.Contains(t, "zeebe") || strings.Contains(t, "grpc") {
			continue
		}
		if seen[t] {
			continue
		}
		seen[t] = true
		kept = append(kept, t)
	}
	return strings.Join(kept, ",")
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
	if host := selectIngressHost(string(out)); host != "" {
		return host, nil
	}

	gatewayArgs := []string{}
	if kubeContext != "" {
		gatewayArgs = append(gatewayArgs, "--context", kubeContext)
	}
	gatewayArgs = append(gatewayArgs, "-n", namespace, "get", "gateway",
		"-o", "jsonpath={.items[*].spec.listeners[*].hostname}")
	if gwOut, err := exec.Command("kubectl", gatewayArgs...).Output(); err == nil {
		if host := selectIngressHost(string(gwOut)); host != "" {
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
