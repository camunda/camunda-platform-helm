package deploy

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"

	"scripts/camunda-core/pkg/logging"
	"scripts/deploy-camunda/config"
	"scripts/prepare-helm-values/pkg/env"
)

// generateTestSecrets creates random secrets for testing and returns them as a
// map[string]string. The caller is responsible for merging these into its env
// map; this function does NOT call os.Setenv.
//
// If a key already exists in existingEnv with a non-empty value, the existing
// value is preserved (idempotent across multiple calls / scenarios).
//
// The generated secrets are also persisted to envFile (or ".env" by default)
// so that subsequent processes (e.g., test runners) can pick them up.
//
// The returned map also contains `vault_secret_mapping` so the caller can feed
// it into mapper.Generate without touching the process environment.
func generateTestSecrets(envFile string, existingEnv map[string]string) (map[string]string, error) {
	text := func() (string, error) {
		const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
		result := make([]byte, 32)
		for i := range result {
			num, err := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
			if err != nil {
				return "", fmt.Errorf("crypto/rand failed: %w", err)
			}
			result[i] = chars[num.Int64()]
		}
		return string(result), nil
	}

	firstUserPwd, err := text()
	if err != nil {
		return nil, err
	}
	secondUserPwd, err := text()
	if err != nil {
		return nil, err
	}
	thirdUserPwd, err := text()
	if err != nil {
		return nil, err
	}
	keycloakClientsSecret, err := text()
	if err != nil {
		return nil, err
	}

	secrets := map[string]string{
		"DISTRO_QA_E2E_TESTS_IDENTITY_FIRSTUSER_PASSWORD":  firstUserPwd,
		"DISTRO_QA_E2E_TESTS_IDENTITY_SECONDUSER_PASSWORD": secondUserPwd,
		"DISTRO_QA_E2E_TESTS_IDENTITY_THIRDUSER_PASSWORD":  thirdUserPwd,
		"DISTRO_QA_E2E_TESTS_KEYCLOAK_CLIENTS_SECRET":      keycloakClientsSecret,
	}

	// Preserve existing non-empty values (idempotent).
	for k := range secrets {
		if existing, ok := existingEnv[k]; ok && existing != "" {
			secrets[k] = existing
		}
	}

	// Add vault secret mapping.
	secrets["vault_secret_mapping"] = "ci/path DISTRO_QA_E2E_TESTS_IDENTITY_FIRSTUSER_PASSWORD;ci/path DISTRO_QA_E2E_TESTS_IDENTITY_SECONDUSER_PASSWORD;ci/path DISTRO_QA_E2E_TESTS_IDENTITY_THIRDUSER_PASSWORD;ci/path DISTRO_QA_E2E_TESTS_KEYCLOAK_CLIENTS_SECRET;"

	// Persist to .env file
	targetEnvFile := envFile
	if targetEnvFile == "" {
		targetEnvFile = ".env"
	}

	toPersist := map[string]string{
		"DISTRO_QA_E2E_TESTS_IDENTITY_FIRSTUSER_PASSWORD":  secrets["DISTRO_QA_E2E_TESTS_IDENTITY_FIRSTUSER_PASSWORD"],
		"DISTRO_QA_E2E_TESTS_IDENTITY_SECONDUSER_PASSWORD": secrets["DISTRO_QA_E2E_TESTS_IDENTITY_SECONDUSER_PASSWORD"],
		"DISTRO_QA_E2E_TESTS_IDENTITY_THIRDUSER_PASSWORD":  secrets["DISTRO_QA_E2E_TESTS_IDENTITY_THIRDUSER_PASSWORD"],
		"DISTRO_QA_E2E_TESTS_KEYCLOAK_CLIENTS_SECRET":      secrets["DISTRO_QA_E2E_TESTS_KEYCLOAK_CLIENTS_SECRET"],
	}

	if err := env.AppendMultiple(targetEnvFile, toPersist); err != nil {
		logging.Logger.Warn().Err(err).Str("path", targetEnvFile).Msg("Failed to persist generated secrets to .env")
	} else {
		for k := range toPersist {
			logging.Logger.Info().Str("key", k).Str("path", targetEnvFile).Msg("Persisted generated secret to .env")
		}
	}

	return secrets, nil
}

// renderTestEnvFile generates the E2E test .env file by calling render-e2e-env.sh.
// For multi-scenario deployments, the scenario name is appended to the output path.
// This function logs warnings but does not fail the deployment if env file generation fails.
func renderTestEnvFile(ctx context.Context, flags *config.RuntimeFlags, namespace, scenario string) (string, error) {
	if !flags.Test.OutputTestEnv {
		return "", nil
	}

	// Determine output path - for multi-scenario, append scenario name
	outputPath := flags.Test.OutputTestEnvPath
	if len(flags.Deployment.Scenarios) > 1 && scenario != "" {
		outputPath = fmt.Sprintf("%s.%s", flags.Test.OutputTestEnvPath, scenario)
	}

	// Locate the render-e2e-env.sh script
	var scriptPath string
	candidates := []string{
		filepath.Join(flags.Chart.RepoRoot, "scripts", "render-e2e-env.sh"),
		filepath.Join(filepath.Dir(flags.Chart.ChartPath), "..", "scripts", "render-e2e-env.sh"),
		filepath.Join(flags.Chart.ChartPath, "..", "..", "scripts", "render-e2e-env.sh"),
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			scriptPath = candidate
			break
		}
	}

	if scriptPath == "" {
		return "", fmt.Errorf("render-e2e-env.sh script not found; searched: %v", candidates)
	}

	// Build args for render-e2e-env.sh
	args := []string{
		"--absolute-chart-path", flags.Chart.ChartPath,
		"--namespace", namespace,
		"--output", outputPath,
	}

	if flags.Test.KubeContext != "" {
		args = append(args, "--kube-context", flags.Test.KubeContext)
	}

	logging.Logger.Info().
		Str("output", outputPath).
		Str("namespace", namespace).
		Str("script", scriptPath).
		Msg("Generating E2E test environment file")

	cmd := exec.CommandContext(ctx, scriptPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("render-e2e-env.sh failed: %w", err)
	}

	logging.Logger.Info().
		Str("path", outputPath).
		Msg("E2E test environment file generated successfully")

	return outputPath, nil
}
