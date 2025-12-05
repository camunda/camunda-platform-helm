package deploy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"scripts/camunda-core/pkg/kube"
	"scripts/camunda-core/pkg/logging"
	"scripts/camunda-deployer/pkg/deployer"
	"scripts/camunda-deployer/pkg/types"
	"scripts/deploy-camunda/config"
	"scripts/prepare-helm-values/pkg/values"
	"scripts/vault-secret-mapper/pkg/mapper"
	"time"
)

// deployScenario performs deployment for a single scenario.
func deployScenario(ctx context.Context, scenarioCtx *ScenarioContext, flags *config.RuntimeFlags) *ScenarioResult {
	result := &ScenarioResult{
		Scenario:                 scenarioCtx.ScenarioName,
		Namespace:                scenarioCtx.Namespace,
		Release:                  scenarioCtx.Release,
		IngressHostname:          scenarioCtx.IngressHostname,
		KeycloakRealm:            scenarioCtx.KeycloakRealm,
		OptimizeIndexPrefix:      scenarioCtx.OptimizeIndexPrefix,
		OrchestrationIndexPrefix: scenarioCtx.OrchestrationIndexPrefix,
	}

	logging.Logger.Info().
		Str("scenario", scenarioCtx.ScenarioName).
		Str("namespace", scenarioCtx.Namespace).
		Str("realm", scenarioCtx.KeycloakRealm).
		Msg("Starting scenario deployment")

	// Generate identifiers only if not provided via flags
	realmName := scenarioCtx.KeycloakRealm
	logging.Logger.Info().Str("realm", realmName).Str("scenario", scenarioCtx.ScenarioName).Msg("Using Keycloak realm")

	optimizePrefix := scenarioCtx.OptimizeIndexPrefix
	logging.Logger.Info().Str("optimize", optimizePrefix).Str("scenario", scenarioCtx.ScenarioName).Msg("Using Optimize index prefix")

	orchestrationPrefix := scenarioCtx.OrchestrationIndexPrefix
	logging.Logger.Info().Str("orchestration", orchestrationPrefix).Str("scenario", scenarioCtx.ScenarioName).Msg("Using Orchestration index prefix")

	// Create temp directory for values
	tempDir, err := os.MkdirTemp("", fmt.Sprintf("camunda-values-%s-*", scenarioCtx.ScenarioName))
	if err != nil {
		result.Error = err
		return result
	}
	scenarioCtx.TempDir = tempDir
	defer os.RemoveAll(tempDir)
	logging.Logger.Info().Str("dir", tempDir).Str("scenario", scenarioCtx.ScenarioName).Msg("Created temporary values directory")

	// Prepare values files with environment setup
	vals, vaultSecretPath, err := prepareValuesFiles(scenarioCtx, flags, tempDir)
	if err != nil {
		result.Error = err
		return result
	}

	// Build deployment options
	deployOpts := buildDeployOptions(scenarioCtx, flags, vals, vaultSecretPath, realmName)

	// Handle dry-run mode
	if flags.DryRun {
		printDryRunSummary(scenarioCtx, deployOpts, vals)
		logging.Logger.Info().
			Str("scenario", scenarioCtx.ScenarioName).
			Msg("Dry-run completed - no changes made")
		return result
	}

	// Delete namespace first if requested
	if flags.DeleteNamespaceFirst {
		logging.Logger.Info().Str("namespace", scenarioCtx.Namespace).Str("scenario", scenarioCtx.ScenarioName).Msg("Deleting namespace prior to deployment as requested")
		if err := deleteNamespace(ctx, scenarioCtx.Namespace); err != nil {
			result.Error = fmt.Errorf("failed to delete namespace %q: %w", scenarioCtx.Namespace, err)
			return result
		}
	}

	// Execute deployment
	if err := deployer.Deploy(ctx, deployOpts); err != nil {
		result.Error = err
		return result
	}

	// Capture credentials from environment
	result.FirstUserPassword = os.Getenv("DISTRO_QA_E2E_TESTS_IDENTITY_FIRSTUSER_PASSWORD")
	result.SecondUserPassword = os.Getenv("DISTRO_QA_E2E_TESTS_IDENTITY_SECONDUSER_PASSWORD")
	result.ThirdUserPassword = os.Getenv("DISTRO_QA_E2E_TESTS_IDENTITY_THIRDUSER_PASSWORD")
	result.KeycloakClientsSecret = os.Getenv("DISTRO_QA_E2E_TESTS_KEYCLOAK_CLIENTS_SECRET")

	logging.Logger.Info().
		Str("scenario", scenarioCtx.ScenarioName).
		Str("namespace", scenarioCtx.Namespace).
		Msg("Scenario deployment completed successfully")

	return result
}

// prepareValuesFiles sets up environment and processes values files.
func prepareValuesFiles(scenarioCtx *ScenarioContext, flags *config.RuntimeFlags, tempDir string) ([]string, string, error) {
	// Use EnvScope for thread-safe environment variable manipulation
	envScope := NewEnvScope(DeploymentEnvKeys())

	// Define the environment setter for this scenario
	envSetter := func() {
		os.Setenv("KEYCLOAK_REALM", scenarioCtx.KeycloakRealm)
		os.Setenv("OPTIMIZE_INDEX_PREFIX", scenarioCtx.OptimizeIndexPrefix)
		os.Setenv("ORCHESTRATION_INDEX_PREFIX", scenarioCtx.OrchestrationIndexPrefix)
		if scenarioCtx.TasklistIndexPrefix != "" {
			os.Setenv("TASKLIST_INDEX_PREFIX", scenarioCtx.TasklistIndexPrefix)
		}
		if scenarioCtx.OperateIndexPrefix != "" {
			os.Setenv("OPERATE_INDEX_PREFIX", scenarioCtx.OperateIndexPrefix)
		}
		if scenarioCtx.IngressHostname != "" {
			os.Setenv("CAMUNDA_HOSTNAME", scenarioCtx.IngressHostname)
		}
		os.Setenv("FLOW", flags.Flow)

		// Set Keycloak environment variables
		if flags.KeycloakHost != "" {
			kcVersionSafe := KeycloakVersionSafe
			kcHostVar := fmt.Sprintf("KEYCLOAK_EXT_HOST_%s", kcVersionSafe)
			kcProtoVar := fmt.Sprintf("KEYCLOAK_EXT_PROTOCOL_%s", kcVersionSafe)
			os.Setenv(kcHostVar, flags.KeycloakHost)
			os.Setenv(kcProtoVar, flags.KeycloakProtocol)
		}
	}

	// Apply environment and get cleanup function
	cleanup := envScope.Apply(envSetter)
	defer cleanup()

	// Process values files helper
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
			return enhanceScenarioError(err, scen, flags.ScenarioPath, flags.ChartPath)
		}
		_, _, err = values.Process(file, opts)
		if err != nil {
			return fmt.Errorf("failed to process scenario %q: %w", scen, err)
		}
		return nil
	}

	// Process auth scenario if different from main scenario
	if flags.Auth != "" && flags.Auth != scenarioCtx.ScenarioName {
		logging.Logger.Info().Str("auth", flags.Auth).Str("scenario", scenarioCtx.ScenarioName).Msg("Preparing auth scenario")
		if err := processValues(flags.Auth); err != nil {
			return nil, "", fmt.Errorf("failed to prepare auth scenario: %w", err)
		}
	}

	// Process main scenario
	logging.Logger.Info().Str("scenario", scenarioCtx.ScenarioName).Msg("Preparing main scenario")
	if err := processValues(scenarioCtx.ScenarioName); err != nil {
		return nil, "", fmt.Errorf("failed to prepare main scenario: %w", err)
	}

	// Auto-generate secrets if requested
	if flags.AutoGenerateSecrets {
		if err := generateTestSecrets(flags.EnvFile); err != nil {
			return nil, "", err
		}
	}

	// Generate vault secrets if mapping is provided
	var vaultSecretPath string
	if flags.VaultSecretMapping != "" || flags.AutoGenerateSecrets {
		vaultSecretPath = filepath.Join(tempDir, "vault-mapped-secrets.yaml")
		logging.Logger.Info().Str("scenario", scenarioCtx.ScenarioName).Msg("Generating vault secrets")
		mapping := flags.VaultSecretMapping
		if mapping == "" {
			mapping = os.Getenv("vault_secret_mapping")
		}
		if err := mapper.Generate(mapping, "vault-mapped-secrets", vaultSecretPath); err != nil {
			return nil, "", fmt.Errorf("failed to generate vault secrets: %w", err)
		}
	}

	// Build values files list
	vals, err := deployer.BuildValuesList(tempDir, []string{scenarioCtx.ScenarioName}, flags.Auth, false, false, flags.ExtraValues)
	if err != nil {
		return nil, "", err
	}

	return vals, vaultSecretPath, nil
}

// buildDeployOptions constructs the deployment options struct.
func buildDeployOptions(scenarioCtx *ScenarioContext, flags *config.RuntimeFlags, vals []string, vaultSecretPath, realmName string) types.Options {
	// Determine timeout duration from flags (default to 5 minutes if not set)
	timeoutMinutes := flags.Timeout
	if timeoutMinutes <= 0 {
		timeoutMinutes = 5
	}

	return types.Options{
		ChartPath:              flags.ChartPath,
		Chart:                  flags.Chart,
		Version:                flags.ChartVersion,
		ReleaseName:            scenarioCtx.Release,
		Namespace:              scenarioCtx.Namespace,
		Wait:                   true,
		Atomic:                 true,
		Timeout:                time.Duration(timeoutMinutes) * time.Minute,
		ValuesFiles:            vals,
		EnsureDockerRegistry:   flags.EnsureDockerRegistry,
		SkipDependencyUpdate:   flags.SkipDependencyUpdate,
		ExternalSecretsEnabled: flags.ExternalSecrets,
		DockerRegistryUsername: flags.DockerUsername,
		DockerRegistryPassword: flags.DockerPassword,
		Platform:               flags.Platform,
		RepoRoot:               flags.RepoRoot,
		Identifier:             fmt.Sprintf("%s-%s-%s", scenarioCtx.Release, scenarioCtx.ScenarioName, time.Now().Format("20060102150405")),
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
}

// deleteNamespace deletes a Kubernetes namespace.
func deleteNamespace(ctx context.Context, namespace string) error {
	return kube.DeleteNamespace(ctx, "", "", namespace)
}

