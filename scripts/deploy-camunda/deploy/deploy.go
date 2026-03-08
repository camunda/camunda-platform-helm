package deploy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"scripts/camunda-core/pkg/kube"
	"scripts/camunda-core/pkg/logging"
	"scripts/camunda-core/pkg/scenarios"
	"scripts/camunda-deployer/pkg/deployer"
	"scripts/camunda-deployer/pkg/types"
	"scripts/deploy-camunda/config"
)

// Execute performs the actual Camunda deployment based on the provided flags.
func Execute(ctx context.Context, flags *config.RuntimeFlags) error {
	// Check if we're deploying multiple scenarios in parallel
	if len(flags.Deployment.Scenarios) > 1 {
		return executeParallelDeployments(ctx, flags)
	}

	// Single scenario deployment (original behavior)
	return executeSingleDeployment(ctx, flags)
}

// executeParallelDeployments deploys multiple scenarios concurrently.
func executeParallelDeployments(ctx context.Context, flags *config.RuntimeFlags) error {
	logging.Logger.Info().
		Int("count", len(flags.Deployment.Scenarios)).
		Strs("scenarios", flags.Deployment.Scenarios).
		Msg("Starting parallel deployment of multiple scenarios")

	// Validate all scenarios exist before starting any deployments
	// This provides better error messages and fails fast
	scenarioDir := flags.Deployment.ScenarioPath
	if scenarioDir == "" {
		scenarioDir = filepath.Join(flags.Chart.ChartPath, "test/integration/scenarios/chart-full-setup")
	}

	for _, scenario := range flags.Deployment.Scenarios {
		// Use the scenarios package to resolve paths - this supports both layered and legacy values
		_, err := scenarios.ResolvePath(scenarioDir, scenario)
		if err != nil {
			// Enhance error with helpful context
			return enhanceScenarioError(err, scenario, flags.Deployment.ScenarioPath, flags.Chart.ChartPath)
		}
	}

	logging.Logger.Info().Msg("All scenarios validated successfully")

	// ============================================================
	// PHASE 1: Prepare all scenarios SEQUENTIALLY
	// This handles interactive prompts and environment variable substitution
	// safely before any parallel execution begins.
	// ============================================================
	logging.Logger.Info().
		Int("count", len(flags.Deployment.Scenarios)).
		Msg("Phase 1: Preparing values for all scenarios sequentially")

	prepared := make([]*PreparedScenario, 0, len(flags.Deployment.Scenarios))
	for _, scenario := range flags.Deployment.Scenarios {
		scenarioCtx, err := generateScenarioContext(scenario, flags)
		if err != nil {
			return fmt.Errorf("failed to generate scenario context for %s: %w", scenario, err)
		}

		logging.Logger.Info().
			Str("scenario", scenario).
			Str("namespace", scenarioCtx.Namespace).
			Msg("Preparing scenario")

		p, err := prepareScenarioValues(ctx, scenarioCtx, flags)
		if err != nil {
			// Cleanup any already-prepared temp directories
			for _, prep := range prepared {
				logging.Logger.Debug().
					Str("dir", prep.TempDir).
					Str("scenario", prep.ScenarioCtx.ScenarioName).
					Msg("🧹 Cleaning up prepared scenario temp dir due to preparation failure")
				os.RemoveAll(prep.TempDir)
			}
			return fmt.Errorf("scenario %q failed during preparation: %w", scenario, err)
		}
		prepared = append(prepared, p)
	}

	logging.Logger.Info().
		Int("count", len(prepared)).
		Msg("Phase 1 complete: All scenarios prepared successfully")

	// ============================================================
	// PHASE 2: Deploy all scenarios IN PARALLEL
	// All interactive prompts and env var substitution is complete,
	// so deployments can safely run concurrently.
	// ============================================================
	logging.Logger.Info().
		Int("count", len(prepared)).
		Msg("Phase 2: Deploying all scenarios in parallel")

	var wg sync.WaitGroup
	resultCh := make(chan *ScenarioResult, len(prepared))

	for _, p := range prepared {
		p := p // capture for closure
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Use original context (not a cancellable one) so failures don't cancel others
			result := executeDeployment(ctx, p, flags)
			resultCh <- result
		}()
	}

	// Wait for all deployments to complete
	wg.Wait()
	close(resultCh)

	// Collect results
	results := make([]*ScenarioResult, 0, len(flags.Deployment.Scenarios))
	for result := range resultCh {
		results = append(results, result)
	}

	// Print summary
	printMultiScenarioSummary(results, flags)

	// Return error if any scenario failed deployment
	var hasDeploymentErrors bool
	for _, r := range results {
		if r.Error != nil {
			hasDeploymentErrors = true
			break
		}
	}

	if hasDeploymentErrors {
		return fmt.Errorf("one or more scenarios failed deployment")
	}

	// Run tests for each successful deployment (in parallel)
	// For multi-scenario deployments, we run tests against the first successful namespace
	// since all scenarios should be equivalent for testing purposes
	for _, r := range results {
		if r.Error == nil {
			if err := RunTests(ctx, flags, r.Namespace); err != nil {
				return fmt.Errorf("post-deployment tests failed for namespace %s: %w", r.Namespace, err)
			}
			// Only run tests once - against the first successful deployment
			break
		}
	}

	return nil
}

// executeSingleDeployment deploys a single scenario (original behavior).
func executeSingleDeployment(ctx context.Context, flags *config.RuntimeFlags) error {
	scenario := flags.Deployment.Scenarios[0]
	scenarioCtx, err := generateScenarioContext(scenario, flags)
	if err != nil {
		return fmt.Errorf("failed to generate scenario context: %w", err)
	}

	// Phase 1: Prepare values
	prepared, err := prepareScenarioValues(ctx, scenarioCtx, flags)
	if err != nil {
		return fmt.Errorf("failed to prepare scenario: %w", err)
	}

	// Phase 2: Deploy
	result := executeDeployment(ctx, prepared, flags)

	if result.Error != nil {
		return result.Error
	}

	// Generate E2E test env file if requested
	if flags.Test.OutputTestEnv {
		envPath, err := renderTestEnvFile(ctx, flags, result.Namespace, scenario)
		if err != nil {
			logging.Logger.Warn().Err(err).Msg("Failed to generate E2E test env file")
		} else if envPath != "" {
			result.TestEnvFile = envPath
		}
	}

	// Print single deployment summary
	printDeploymentSummary(result, flags)

	// Phase 3: Run tests if requested
	if err := RunTests(ctx, flags, result.Namespace); err != nil {
		return fmt.Errorf("post-deployment tests failed: %w", err)
	}

	return nil
}

// executeDeployment runs the helm deployment for a prepared scenario.
// This function is safe to run in parallel as it doesn't do any interactive prompts
// or environment variable manipulation. Credentials are carried in PreparedScenario.Secrets.
func executeDeployment(ctx context.Context, prepared *PreparedScenario, flags *config.RuntimeFlags) *ScenarioResult {
	startTime := time.Now()
	scenarioCtx := prepared.ScenarioCtx

	logging.Logger.Debug().
		Str("scenario", scenarioCtx.ScenarioName).
		Str("namespace", scenarioCtx.Namespace).
		Str("release", scenarioCtx.Release).
		Str("ingressHost", scenarioCtx.IngressHost).
		Strs("valuesFiles", prepared.ValuesFiles).
		Msg("🚀 [executeDeployment] ENTRY - starting deployment")

	result := &ScenarioResult{
		Scenario:                 scenarioCtx.ScenarioName,
		Namespace:                scenarioCtx.Namespace,
		Release:                  scenarioCtx.Release,
		IngressHost:              scenarioCtx.IngressHost,
		KeycloakRealm:            prepared.RealmName,
		OptimizeIndexPrefix:      prepared.OptimizePrefix,
		OrchestrationIndexPrefix: prepared.OrchestrationPrefix,
		LayeredFiles:             prepared.LayeredFiles,
	}

	// Ensure temp directory is cleaned up when deployment completes
	defer func() {
		logging.Logger.Debug().
			Str("dir", prepared.TempDir).
			Str("scenario", scenarioCtx.ScenarioName).
			Msg("🧹 [executeDeployment] cleaning up temporary directory")
		os.RemoveAll(prepared.TempDir)
	}()

	logging.Logger.Info().
		Str("scenario", scenarioCtx.ScenarioName).
		Str("namespace", scenarioCtx.Namespace).
		Str("realm", prepared.RealmName).
		Msg("Starting scenario deployment")

	// Determine timeout duration from flags (default to 10 minutes if not set)
	timeoutMinutes := flags.Deployment.Timeout
	if timeoutMinutes <= 0 {
		timeoutMinutes = 10
	}

	identifier := fmt.Sprintf("%s-%s-%s", scenarioCtx.Release, scenarioCtx.ScenarioName, time.Now().Format("20060102150405"))
	logging.Logger.Debug().
		Str("scenario", scenarioCtx.ScenarioName).
		Str("identifier", identifier).
		Msg("🏷️ [executeDeployment] generated deployment identifier")

	// Determine external secrets store - vault-backend if using vault-backed secrets
	externalSecretsStore := flags.Secrets.ExternalSecretsStore
	if flags.Secrets.UseVaultBackedSecrets {
		externalSecretsStore = "vault-backend"
		logging.Logger.Debug().
			Str("scenario", scenarioCtx.ScenarioName).
			Msg("🔐 [executeDeployment] using vault-backed external secrets")
	}

	// Log kubeContext if set
	if flags.Test.KubeContext != "" {
		logging.Logger.Debug().
			Str("scenario", scenarioCtx.ScenarioName).
			Str("kubeContext", flags.Test.KubeContext).
			Msg("🔧 [executeDeployment] using specified kubeContext")
	}

	// Build deployment options
	deployOpts := types.Options{
		ChartPath:              flags.Chart.ChartPath,
		Chart:                  flags.Chart.Chart,
		Version:                flags.Chart.ChartVersion,
		ReleaseName:            scenarioCtx.Release,
		Namespace:              scenarioCtx.Namespace,
		KubeContext:            flags.Test.KubeContext,
		Wait:                   true,
		Atomic:                 false, // Intentionally not atomic: failed pods must stay alive for post-failure diagnostics. Namespace cleanup handles teardown.
		Timeout:                time.Duration(timeoutMinutes) * time.Minute,
		ValuesFiles:            prepared.ValuesFiles,
		EnsureDockerRegistry:   flags.Docker.EnsureDockerRegistry,
		SkipDependencyUpdate:   flags.Chart.SkipDependencyUpdate,
		ExternalSecretsEnabled: flags.Secrets.ExternalSecrets,
		ExternalSecretsStore:   externalSecretsStore,
		DockerRegistryUsername: flags.Docker.DockerUsername,
		DockerRegistryPassword: flags.Docker.DockerPassword,
		EnsureDockerHub:        flags.Docker.EnsureDockerHub,
		DockerHubUsername:      flags.Docker.DockerHubUsername,
		DockerHubPassword:      flags.Docker.DockerHubPassword,
		SkipDockerLogin:        flags.Docker.SkipDockerLogin,
		Platform:               flags.Deployment.Platform,
		NamespacePrefix:        flags.Deployment.NamespacePrefix,
		RepoRoot:               flags.Chart.RepoRoot,
		Identifier:             identifier,
		TTL:                    "60m",
		LoadKeycloakRealm:      true,
		KeycloakRealmName:      prepared.RealmName,
		RenderTemplates:        flags.Deployment.RenderTemplates,
		RenderOutputDir:        flags.Deployment.RenderOutputDir,
		IncludeCRDs:            true,
		CIMetadata: types.CIMetadata{
			Flow: flags.Deployment.Flow,
		},
		ApplyIntegrationCreds: false,
		VaultSecretPath:       prepared.VaultSecretPath,
		ExtraArgs:             flags.Deployment.ExtraHelmArgs,
		SetPairs:              flags.Deployment.ExtraHelmSets,
		PreInstallHooks:       flags.PreInstallHooks,
	}

	// Log deployment options (redact sensitive fields)
	logging.Logger.Debug().
		Str("scenario", scenarioCtx.ScenarioName).
		Interface("deployOpts", redactDeployOpts(deployOpts)).
		Msg("🚀 [executeDeployment] deployment options configured")

	// Delete namespace first if requested
	if flags.Deployment.DeleteNamespaceFirst {
		logging.Logger.Info().Str("namespace", scenarioCtx.Namespace).Str("scenario", scenarioCtx.ScenarioName).Msg("Deleting namespace prior to deployment as requested")
		if err := deleteNamespace(ctx, flags.Test.KubeContext, scenarioCtx.Namespace); err != nil {
			logging.Logger.Debug().
				Err(err).
				Str("namespace", scenarioCtx.Namespace).
				Str("scenario", scenarioCtx.ScenarioName).
				Msg("❌ [executeDeployment] FAILED to delete namespace")
			result.Error = fmt.Errorf("failed to delete namespace %q: %w", scenarioCtx.Namespace, err)
			return result
		}
		logging.Logger.Debug().
			Str("namespace", scenarioCtx.Namespace).
			Str("scenario", scenarioCtx.ScenarioName).
			Msg("✅ [executeDeployment] namespace deleted successfully")
	}

	// Execute deployment
	deployStartTime := time.Now()
	logging.Logger.Debug().
		Str("scenario", scenarioCtx.ScenarioName).
		Str("namespace", scenarioCtx.Namespace).
		Str("release", scenarioCtx.Release).
		Time("startTime", deployStartTime).
		Msg("🚀 [executeDeployment] initiating helm deployment")

	if err := deployer.Deploy(ctx, deployOpts); err != nil {
		deployDuration := time.Since(deployStartTime)
		logging.Logger.Debug().
			Err(err).
			Str("scenario", scenarioCtx.ScenarioName).
			Str("namespace", scenarioCtx.Namespace).
			Dur("deployDuration", deployDuration).
			Msg("❌ [executeDeployment] DEPLOYMENT FAILED")
		result.Error = err
		return result
	}

	deployDuration := time.Since(deployStartTime)
	logging.Logger.Debug().
		Str("scenario", scenarioCtx.ScenarioName).
		Str("namespace", scenarioCtx.Namespace).
		Dur("deployDuration", deployDuration).
		Msg("✅ [executeDeployment] helm deployment completed successfully")

	// Apply post-deploy resources (e.g., Gateway API ProxySettingsPolicy)
	if err := applyPostDeployResources(ctx, scenarioCtx, flags.Chart.ChartPath, flags.Test.KubeContext); err != nil {
		logging.Logger.Error().
			Err(err).
			Str("scenario", scenarioCtx.ScenarioName).
			Str("namespace", scenarioCtx.Namespace).
			Msg("❌ [executeDeployment] failed to apply post-deploy resources")
		result.Error = fmt.Errorf("post-deploy resources failed: %w", err)
		return result
	}

	// Capture credentials from the secrets map prepared in prepareScenarioValues.
	if prepared.Secrets != nil {
		result.FirstUserPassword = prepared.Secrets["DISTRO_QA_E2E_TESTS_IDENTITY_FIRSTUSER_PASSWORD"]
		result.SecondUserPassword = prepared.Secrets["DISTRO_QA_E2E_TESTS_IDENTITY_SECONDUSER_PASSWORD"]
		result.ThirdUserPassword = prepared.Secrets["DISTRO_QA_E2E_TESTS_IDENTITY_THIRDUSER_PASSWORD"]
		result.KeycloakClientsSecret = prepared.Secrets["DISTRO_QA_E2E_TESTS_KEYCLOAK_CLIENTS_SECRET"]
	}

	// Generate E2E test env file if requested
	if flags.Test.OutputTestEnv {
		envPath, err := renderTestEnvFile(ctx, flags, scenarioCtx.Namespace, scenarioCtx.ScenarioName)
		if err != nil {
			logging.Logger.Warn().Err(err).Str("scenario", scenarioCtx.ScenarioName).Msg("Failed to generate E2E test env file")
		} else if envPath != "" {
			result.TestEnvFile = envPath
		}
	}

	totalDuration := time.Since(startTime)
	logging.Logger.Debug().
		Str("scenario", scenarioCtx.ScenarioName).
		Str("namespace", scenarioCtx.Namespace).
		Str("release", scenarioCtx.Release).
		Str("ingressHost", scenarioCtx.IngressHost).
		Str("keycloakRealm", result.KeycloakRealm).
		Dur("totalDuration", totalDuration).
		Dur("deployDuration", deployDuration).
		Msg("🎉 [executeDeployment] EXIT - scenario deployment completed successfully")

	logging.Logger.Info().
		Str("scenario", scenarioCtx.ScenarioName).
		Str("namespace", scenarioCtx.Namespace).
		Msg("Scenario deployment completed successfully")

	return result
}

// deleteNamespace deletes a Kubernetes namespace.
func deleteNamespace(ctx context.Context, kubeContext, namespace string) error {
	return kube.DeleteNamespace(ctx, "", kubeContext, namespace)
}
