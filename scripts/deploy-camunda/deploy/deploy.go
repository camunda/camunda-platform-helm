package deploy

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"scripts/camunda-core/pkg/kube"
	"scripts/camunda-core/pkg/logging"
	"scripts/camunda-deployer/pkg/deployer"
	"scripts/camunda-deployer/pkg/types"
	"scripts/deploy-camunda/config"
	"scripts/prepare-helm-values/pkg/env"
	"scripts/prepare-helm-values/pkg/values"
	"strings"
	"time"
	"vault-secret-mapper/pkg/mapper"

	"github.com/jwalton/gchalk"
)

// generateRandomSuffix creates an 8-character random string.
func generateRandomSuffix() string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	result := make([]byte, 8)
	for i := range result {
		num, _ := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		result[i] = chars[num.Int64()]
	}
	return string(result)
}

// maskIfSet returns a masked placeholder when a sensitive value is set.
func maskIfSet(val string) string {
	if val == "" {
		return ""
	}
	return "***"
}

// Execute performs the actual Camunda deployment based on the provided flags.
func Execute(ctx context.Context, flags *config.RuntimeFlags) error {
	// Generate identifiers
	suffix := generateRandomSuffix()
	realmName := fmt.Sprintf("%s-%s", flags.Namespace, suffix)
	optimizePrefix := fmt.Sprintf("opt-%s", suffix)
	orchestrationPrefix := fmt.Sprintf("orch-%s", suffix)

	logging.Logger.Info().
		Str("realm", realmName).
		Str("optimize", optimizePrefix).
		Str("orchestration", orchestrationPrefix).
		Msg("Generated identifiers")

	// Create temp directory for values
	tempDir, err := os.MkdirTemp("", "camunda-values-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)
	logging.Logger.Info().Str("dir", tempDir).Msg("Created temporary values directory")

	// Set environment variables for prepare-helm-values
	if os.Getenv("KEYCLOAK_REALM") == "" {
		os.Setenv("KEYCLOAK_REALM", realmName)
	}
	if os.Getenv("OPTIMIZE_INDEX_PREFIX") == "" {
		os.Setenv("OPTIMIZE_INDEX_PREFIX", optimizePrefix)
	}
	if os.Getenv("ORCHESTRATION_INDEX_PREFIX") == "" {
		os.Setenv("ORCHESTRATION_INDEX_PREFIX", orchestrationPrefix)
	}

	os.Setenv("FLOW", flags.Flow)

	// Set Keycloak environment variables
	if flags.KeycloakHost != "" {
		kcVersionSafe := "24_9_0"
		kcHostVar := fmt.Sprintf("KEYCLOAK_EXT_HOST_%s", kcVersionSafe)
		kcProtoVar := fmt.Sprintf("KEYCLOAK_EXT_PROTOCOL_%s", kcVersionSafe)
		os.Setenv(kcHostVar, flags.KeycloakHost)
		os.Setenv(kcProtoVar, flags.KeycloakProtocol)
	}

	// Process values files
	processValues := func(scen string) error {
		opts := values.Options{
			ChartPath:   flags.ChartPath,
			Scenario:    scen,
			ScenarioDir: flags.ScenarioPath,
			OutputDir:   tempDir,
			Interactive: flags.Interactive,
			EnvFile:     flags.EnvFile,
		}
		if opts.EnvFile == "" {
			opts.EnvFile = ".env"
		}

		file, err := values.ResolveValuesFile(opts)
		if err != nil {
			return err
		}
		_, _, err = values.Process(file, opts)
		return err
	}

	// Process auth scenario if different from main scenario
	if flags.Auth != "" && flags.Auth != flags.Scenario {
		logging.Logger.Info().Str("scenario", flags.Auth).Msg("Preparing auth scenario")
		if err := processValues(flags.Auth); err != nil {
			return err
		}
	}

	// Process main scenario
	logging.Logger.Info().Str("scenario", flags.Scenario).Msg("Preparing main scenario")
	if err := processValues(flags.Scenario); err != nil {
		return err
	}

	// Auto-generate secrets if requested
	if flags.AutoGenerateSecrets {
		if err := generateTestSecrets(flags.EnvFile); err != nil {
			return err
		}
	}

	// Generate vault secrets if mapping is provided
	var vaultSecretPath string
	if flags.VaultSecretMapping != "" {
		vaultSecretPath = filepath.Join(tempDir, "vault-mapped-secrets.yaml")
		logging.Logger.Info().Msg("Generating vault secrets")

		if err := mapper.Generate(flags.VaultSecretMapping, "vault-mapped-secrets", vaultSecretPath); err != nil {
			return fmt.Errorf("failed to generate vault secrets: %w", err)
		}
	}

	// Build values files list
	vals, err := deployer.BuildValuesList(tempDir, []string{flags.Scenario}, flags.Auth, false, false, flags.ExtraValues)
	if err != nil {
		return err
	}

	// Perform deployment
	deployOpts := types.Options{
		ChartPath:              flags.ChartPath,
		Chart:                  flags.Chart,
		Version:                flags.ChartVersion,
		ReleaseName:            flags.Release,
		Namespace:              flags.Namespace,
		Wait:                   true,
		Atomic:                 true,
		Timeout:                15 * time.Minute,
		ValuesFiles:            vals,
		EnsureDockerRegistry:   flags.EnsureDockerRegistry,
		SkipDependencyUpdate:   flags.SkipDependencyUpdate,
		ExternalSecretsEnabled: flags.ExternalSecrets,
		DockerRegistryUsername: flags.DockerUsername,
		DockerRegistryPassword: flags.DockerPassword,
		Platform:               flags.Platform,
		RepoRoot:               flags.RepoRoot,
		Identifier:             fmt.Sprintf("%s-%s", flags.Release, time.Now().Format("20060102150405")),
		TTL:                    "30m",
		LoadKeycloakRealm:      true,
		KeycloakRealmName:      realmName,
		RenderTemplates:        flags.RenderTemplates,
		RenderOutputDir:        flags.RenderOutputDir,
		IncludeCRDs:            true,
		CIMetadata: types.CIMetadata{
			Flow: flags.Flow,
		},
		ApplyIntegrationCreds: true,
		VaultSecretPath:       vaultSecretPath,
	}

	// Delete namespace first if requested
	if flags.DeleteNamespaceFirst {
		logging.Logger.Info().Str("namespace", flags.Namespace).Msg("Deleting namespace prior to deployment as requested")
		if err := deleteNamespace(ctx, flags.Namespace); err != nil {
			return fmt.Errorf("failed to delete namespace %q: %w", flags.Namespace, err)
		}
	}

	// Execute deployment
	if err := deployer.Deploy(ctx, deployOpts); err != nil {
		return err
	}

	// Print deployment summary
	printDeploymentSummary(realmName, optimizePrefix, orchestrationPrefix)

	return nil
}

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

	os.Setenv("DISTRO_QA_E2E_TESTS_IDENTITY_FIRSTUSER_PASSWORD", firstUserPwd)
	os.Setenv("DISTRO_QA_E2E_TESTS_IDENTITY_SECONDUSER_PASSWORD", secondUserPwd)
	os.Setenv("DISTRO_QA_E2E_TESTS_IDENTITY_THIRDUSER_PASSWORD", thirdUserPwd)
	os.Setenv("DISTRO_QA_E2E_TESTS_KEYCLOAK_CLIENTS_SECRET", keycloakClientsSecret)

	// Persist to .env file
	targetEnvFile := envFile
	if targetEnvFile == "" {
		targetEnvFile = ".env"
	}

	type pair struct{ key, val string }
	toPersist := []pair{
		{"DISTRO_QA_E2E_TESTS_IDENTITY_FIRSTUSER_PASSWORD", firstUserPwd},
		{"DISTRO_QA_E2E_TESTS_IDENTITY_SECONDUSER_PASSWORD", secondUserPwd},
		{"DISTRO_QA_E2E_TESTS_IDENTITY_THIRDUSER_PASSWORD", thirdUserPwd},
		{"DISTRO_QA_E2E_TESTS_KEYCLOAK_CLIENTS_SECRET", keycloakClientsSecret},
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

// deleteNamespace deletes a Kubernetes namespace.
func deleteNamespace(ctx context.Context, namespace string) error {
	return kube.DeleteNamespace(ctx, "", "", namespace)
}

// printDeploymentSummary outputs the deployment results.
func printDeploymentSummary(realm, optimizePrefix, orchestrationPrefix string) {
	firstPwd := os.Getenv("DISTRO_QA_E2E_TESTS_IDENTITY_FIRSTUSER_PASSWORD")
	secondPwd := os.Getenv("DISTRO_QA_E2E_TESTS_IDENTITY_SECONDUSER_PASSWORD")
	thirdPwd := os.Getenv("DISTRO_QA_E2E_TESTS_IDENTITY_THIRDUSER_PASSWORD")
	clientSecret := os.Getenv("DISTRO_QA_E2E_TESTS_KEYCLOAK_CLIENTS_SECRET")

	if !logging.IsTerminal(os.Stdout.Fd()) {
		// Plain, machine-friendly output
		var out strings.Builder
		fmt.Fprintf(&out, "deployment: success\n")
		fmt.Fprintf(&out, "realm: %s\n", realm)
		fmt.Fprintf(&out, "optimizeIndexPrefix: %s\n", optimizePrefix)
		fmt.Fprintf(&out, "orchestrationIndexPrefix: %s\n", orchestrationPrefix)
		fmt.Fprintf(&out, "credentials:\n")
		fmt.Fprintf(&out, "  firstUserPassword: %s\n", firstPwd)
		fmt.Fprintf(&out, "  secondUserPassword: %s\n", secondPwd)
		fmt.Fprintf(&out, "  thirdUserPassword: %s\n", thirdPwd)
		fmt.Fprintf(&out, "  keycloakClientsSecret: %s\n", clientSecret)
		logging.Logger.Info().Msg(out.String())
		return
	}

	// Pretty, human-friendly output
	styleKey := func(s string) string { return logging.Emphasize(s, gchalk.Cyan) }
	styleVal := func(s string) string { return logging.Emphasize(s, gchalk.Magenta) }
	styleOk := func(s string) string { return logging.Emphasize(s, gchalk.Green) }
	styleHead := func(s string) string { return logging.Emphasize(s, gchalk.Bold) }

	var out strings.Builder
	out.WriteString(styleOk("🎉 Deployment completed successfully"))
	out.WriteString("\n\n")

	// Identifiers
	out.WriteString(styleHead("Identifiers"))
	out.WriteString("\n")
	maxKey := 25
	fmt.Fprintf(&out, "  - %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "Realm")), styleVal(realm))
	fmt.Fprintf(&out, "  - %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "Optimize index prefix")), styleVal(optimizePrefix))
	fmt.Fprintf(&out, "  - %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "Orchestration index prefix")), styleVal(orchestrationPrefix))

	out.WriteString("\n")
	out.WriteString(styleHead("Test credentials"))
	out.WriteString("\n")
	fmt.Fprintf(&out, "  - %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "First user password")), styleVal(firstPwd))
	fmt.Fprintf(&out, "  - %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "Second user password")), styleVal(secondPwd))
	fmt.Fprintf(&out, "  - %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "Third user password")), styleVal(thirdPwd))
	fmt.Fprintf(&out, "  - %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "Keycloak clients secret")), styleVal(clientSecret))

	out.WriteString("\n")
	out.WriteString("Please keep these credentials safe. If you have any questions, refer to the documentation or reach out for support. 🚀")

	logging.Logger.Info().Msg(out.String())
}
