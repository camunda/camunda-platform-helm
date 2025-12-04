package deploy

import (
	"crypto/rand"
	"math/big"
	"os"
	"scripts/camunda-core/pkg/logging"
	"scripts/prepare-helm-values/pkg/env"
)

// generateTestSecrets creates random secrets for testing.
func generateTestSecrets(envFile string) error {
	text := func() string {
		const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
		result := make([]byte, 32)
		for i := range result {
			num, _ := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
			result[i] = chars[num.Int64()]
		}
		return string(result)
	}

	firstUserPwd := text()
	secondUserPwd := text()
	thirdUserPwd := text()
	keycloakClientsSecret := text()

	if os.Getenv("DISTRO_QA_E2E_TESTS_IDENTITY_FIRSTUSER_PASSWORD") == "" {
		os.Setenv("DISTRO_QA_E2E_TESTS_IDENTITY_FIRSTUSER_PASSWORD", firstUserPwd)
	}
	if os.Getenv("DISTRO_QA_E2E_TESTS_IDENTITY_SECONDUSER_PASSWORD") == "" {
		os.Setenv("DISTRO_QA_E2E_TESTS_IDENTITY_SECONDUSER_PASSWORD", secondUserPwd)
	}
	if os.Getenv("DISTRO_QA_E2E_TESTS_IDENTITY_THIRDUSER_PASSWORD") == "" {
		os.Setenv("DISTRO_QA_E2E_TESTS_IDENTITY_THIRDUSER_PASSWORD", thirdUserPwd)
	}
	if os.Getenv("DISTRO_QA_E2E_TESTS_KEYCLOAK_CLIENTS_SECRET") == "" {
		os.Setenv("DISTRO_QA_E2E_TESTS_KEYCLOAK_CLIENTS_SECRET", keycloakClientsSecret)
	}

	// Persist to .env file
	targetEnvFile := envFile
	if targetEnvFile == "" {
		targetEnvFile = ".env"
	}

	type pair struct{ key, val string }
	toPersist := []pair{
		{"DISTRO_QA_E2E_TESTS_IDENTITY_FIRSTUSER_PASSWORD", os.Getenv("DISTRO_QA_E2E_TESTS_IDENTITY_FIRSTUSER_PASSWORD")},
		{"DISTRO_QA_E2E_TESTS_IDENTITY_SECONDUSER_PASSWORD", os.Getenv("DISTRO_QA_E2E_TESTS_IDENTITY_SECONDUSER_PASSWORD")},
		{"DISTRO_QA_E2E_TESTS_IDENTITY_THIRDUSER_PASSWORD", os.Getenv("DISTRO_QA_E2E_TESTS_IDENTITY_THIRDUSER_PASSWORD")},
		{"DISTRO_QA_E2E_TESTS_KEYCLOAK_CLIENTS_SECRET", os.Getenv("DISTRO_QA_E2E_TESTS_KEYCLOAK_CLIENTS_SECRET")},
	}

	for _, p := range toPersist {
		if err := env.Append(targetEnvFile, p.key, p.val); err != nil {
			logging.Logger.Warn().Err(err).Str("key", p.key).Str("path", targetEnvFile).Msg("Failed to persist generated secret to .env")
		} else {
			logging.Logger.Info().Str("key", p.key).Str("path", targetEnvFile).Msg("Persisted generated secret to .env")
		}
	}

	// Build vault secret mapping
	os.Setenv("vault_secret_mapping", "ci/path DISTRO_QA_E2E_TESTS_IDENTITY_FIRSTUSER_PASSWORD;ci/path DISTRO_QA_E2E_TESTS_IDENTITY_SECONDUSER_PASSWORD;ci/path DISTRO_QA_E2E_TESTS_IDENTITY_THIRDUSER_PASSWORD;ci/path DISTRO_QA_E2E_TESTS_KEYCLOAK_CLIENTS_SECRET;")

	return nil
}

