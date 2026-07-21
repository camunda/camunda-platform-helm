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
		baseDomain             string
		renderScript           string
		kubeContext            string
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
				"--not-ci", "--run-smoke-tests",
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

			mgmtHost := managementNamespace + "." + baseDomain

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
			if err := os.WriteFile(output, []byte(merged), 0o644); err != nil {
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
	cmd.Flags().StringVar(&baseDomain, "base-domain", "ci.distro.ultrawombat.com", "ingress base domain")
	cmd.Flags().StringVar(&renderScript, "render-script", "scripts/render-e2e-env.sh", "path to render-e2e-env.sh")
	cmd.Flags().StringVar(&kubeContext, "kube-context", "", "kube context (optional)")
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
		"-o", fmt.Sprintf("jsonpath={.data.%s}", key))
	out, err := exec.Command("kubectl", a...).Output()
	if err != nil {
		return "", fmt.Errorf("resolve secret key %q in %s: %w", key, namespace, err)
	}
	dec, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(out)))
	if err != nil {
		return "", fmt.Errorf("decode secret key %q: %w", key, err)
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
