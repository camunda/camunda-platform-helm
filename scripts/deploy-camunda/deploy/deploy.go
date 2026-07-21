package deploy

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"scripts/camunda-core/pkg/kube"
	"scripts/camunda-core/pkg/logging"
	"scripts/camunda-core/pkg/scenarios"
	"scripts/deploy-camunda/config"
	"scripts/deploy-camunda/pkg/deployer"
	"scripts/deploy-camunda/pkg/types"
)

// runFailFastPreflight runs the secrets/env preflight before a deploy and
// returns an error when a required input is missing. It skips the cluster
// reachability probe (the deploy will contact the cluster regardless) to avoid
// adding a network round-trip to every run. In interactive mode it downgrades a
// failure to a warning, because the downstream values.Process step prompts for
// missing scenario placeholders; non-interactive runs fail fast.
func runFailFastPreflight(ctx context.Context, flags *config.RuntimeFlags) error {
	if flags.SkipPreflight {
		return nil
	}
	// Prefer the path resolved by the root command; fall back to auto-discovery
	// for callers that don't populate it (e.g. tests).
	configPath, found := flags.ConfigPath, flags.ConfigFound
	if configPath == "" {
		if cfgRes, err := config.ResolvePath(""); err == nil && cfgRes != nil {
			configPath, found = cfgRes.Path, cfgRes.Found
		}
	}
	report := Preflight(ctx, flags, PreflightOptions{
		ConfigPath:           configPath,
		ConfigFound:          found,
		SkipKubeReachability: true,
	})
	if report.OK() {
		return nil
	}

	// Interactive runs: prompt for the missing vars, persist them, and re-check
	// before deciding. Non-interactive runs (the matrix) fail fast with the list.
	if flags.Interactive {
		var buf bytes.Buffer
		report.Render(&buf)
		logging.Logger.Info().Msgf("preflight found missing inputs:\n%s", buf.String())
		if n, err := ResolveMissingInteractively(ctx, report, flags); err != nil {
			return err
		} else if n > 0 {
			report = Preflight(ctx, flags, PreflightOptions{
				ConfigPath:           configPath,
				ConfigFound:          found,
				SkipKubeReachability: true,
			})
			if report.OK() {
				return nil
			}
		}
		// Still not satisfied (or nothing entered). Some missing vars may still be
		// resolvable by the downstream interactive values.Process prompt, so warn
		// and continue rather than blocking.
		var after bytes.Buffer
		report.Render(&after)
		logging.Logger.Warn().Msgf("preflight still has issues (continuing — downstream prompts may resolve scenario vars):\n%s", after.String())
		return nil
	}

	var buf bytes.Buffer
	report.Render(&buf)
	return fmt.Errorf("preflight failed before deploy:\n%s\nfix the above, or re-run with --interactive (to be prompted) or --skip-preflight", buf.String())
}

// Execute performs the actual Camunda deployment based on the provided flags.
func Execute(ctx context.Context, flags *config.RuntimeFlags) error {
	// Fail-fast: validate secrets/env before any cluster mutation so a missing
	// credential surfaces here rather than as an ImagePullBackOff minutes later.
	if err := runFailFastPreflight(ctx, flags); err != nil {
		return err
	}

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
	if flags.OnPhase != nil {
		flags.OnPhase("deploying")
	}
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
		TTL:                    resolveDeployTTL(flags.Deployment.TTL, os.Getenv("DEPLOY_CAMUNDA_TTL")),
		LoadKeycloakRealm:      true,
		KeycloakRealmName:      prepared.RealmName,
		RenderTemplates:        flags.Deployment.RenderTemplates,
		RenderOutputDir:        flags.Deployment.RenderOutputDir,
		IncludeCRDs:            true,
		CIMetadata: types.CIMetadata{
			Flow:        flags.Deployment.Flow,
			GithubRunID: os.Getenv("GITHUB_RUN_ID"),
		},
		ApplyIntegrationCreds: false,
		VaultSecretPath:       prepared.VaultSecretPath,
		ExtraArgs:             flags.Deployment.ExtraHelmArgs,
		SetPairs:              flags.Deployment.ExtraHelmSets,
		PreInstallHooks:       flags.PreInstallHooks,
		CompanionCharts:       toDeployerCompanionCharts(prepared.CompanionCharts),
		PostInfraHooks:        flags.PostInfraHooks,
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

	// Apply post-deploy hooks (e.g., the Gateway API ProxySettingsPolicy
	// applied for gateway-keycloak via a declarative post-deploy: block in
	// ci-test-config.yaml). Registered by the matrix runner before deploy.
	for _, hook := range flags.PostDeployHooks {
		if err := hook(ctx); err != nil {
			logging.Logger.Error().
				Err(err).
				Str("scenario", scenarioCtx.ScenarioName).
				Str("namespace", scenarioCtx.Namespace).
				Msg("❌ [executeDeployment] post-deploy hook failed")
			result.Error = fmt.Errorf("post-deploy hook failed: %w", err)
			return result
		}
	}

	// Gate on the ingress URL becoming publicly DNS-resolvable and HTTP-reachable
	// before reporting success, so a nightly failure surfaces at the deploy step
	// instead of downstream in the E2E suite. The host falls back to the same
	// CAMUNDA_HOSTNAME / TEST_INGRESS_HOST env vars the auth0 flow uses, since CI
	// passes the hostname that way rather than via --ingress-hostname.
	ingressHost := scenarioCtx.IngressHost
	if ingressHost == "" {
		ingressHost = os.Getenv("CAMUNDA_HOSTNAME")
	}
	if ingressHost == "" {
		ingressHost = os.Getenv("TEST_INGRESS_HOST")
	}
	if flags.Deployment.WaitIngressReady {
		if ingressHost == "" {
			result.Error = fmt.Errorf("--wait-ingress-ready is set but no ingress host could be determined: set --ingress-hostname, --ingress-base-domain, CAMUNDA_HOSTNAME, or TEST_INGRESS_HOST")
			return result
		}
		timeoutMinutes := flags.Deployment.IngressReadyTimeoutMinutes
		if timeoutMinutes <= 0 {
			timeoutMinutes = config.DefaultIngressReadyTimeoutMinutes
		}
		logging.Logger.Debug().
			Str("scenario", scenarioCtx.ScenarioName).
			Str("ingressHost", ingressHost).
			Int("timeoutMinutes", timeoutMinutes).
			Msg("⏳ [executeDeployment] waiting for ingress to become reachable")
		if err := waitIngressReady(ctx, ingressHost, time.Duration(timeoutMinutes)*time.Minute, ingressReadyPollInterval); err != nil {
			logging.Logger.Error().
				Err(err).
				Str("scenario", scenarioCtx.ScenarioName).
				Str("ingressHost", ingressHost).
				Msg("❌ [executeDeployment] ingress not reachable")
			result.Error = fmt.Errorf("ingress not reachable: %w", err)
			return result
		}
		logging.Logger.Debug().
			Str("scenario", scenarioCtx.ScenarioName).
			Str("ingressHost", ingressHost).
			Msg("✅ [executeDeployment] ingress reachable")
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

// BuildTopologyCrossRefEnv derives the cross-namespace env vars every release
// in a multi-namespace topology needs to reach back into the management
// release: its namespace (for the Identity/Keycloak FQDNs baked into the
// external-Keycloak identity layer), its Keycloak realm, and the shared
// secondary-storage backend's FQDN (Service lives in the management
// namespace, reachable cluster-wide via <service>.<namespace>.svc.cluster.local).
//
// Exported so the topology deploy driver (cmd's runTopologyEntry) can compute
// this UP FRONT — every release's namespace/realm is known before any
// release is deployed (see GenerateTopologyContexts) — and inject it into
// EVERY release's ExtraEnv (including the management release itself, whose
// inherited placeholders then resolve harmlessly) before render/preflight,
// rather than only after the management release finishes deploying.
func BuildTopologyCrossRefEnv(managementCtx *ScenarioContext, sharedStorageServiceName, sharedStoragePort, sharedStorageScheme string) map[string]string {
	env := map[string]string{
		"MGMT_NAMESPACE": managementCtx.Namespace,
		"KEYCLOAK_REALM": managementCtx.KeycloakRealm,
	}
	if sharedStorageServiceName != "" {
		env["EXTERNAL_ELASTICSEARCH_HOST"] = fmt.Sprintf("%s.%s.svc.cluster.local", sharedStorageServiceName, managementCtx.Namespace)
	}
	if sharedStoragePort != "" {
		env["EXTERNAL_ELASTICSEARCH_PORT"] = sharedStoragePort
	}
	if sharedStorageScheme != "" {
		env["EXTERNAL_ELASTICSEARCH_SCHEME"] = sharedStorageScheme
	}
	return env
}

func resolveDeployTTL(flagTTL, envTTL string) string {
	if strings.TrimSpace(flagTTL) != "" {
		return flagTTL
	}
	if strings.TrimSpace(envTTL) != "" {
		return envTTL
	}
	return "60m"
}

// deleteNamespace deletes a Kubernetes namespace.
func deleteNamespace(ctx context.Context, kubeContext, namespace string) error {
	return kube.DeleteNamespace(ctx, "", kubeContext, namespace)
}

// toDeployerCompanionCharts converts config.CompanionChart to types.CompanionChart.
func toDeployerCompanionCharts(charts []config.CompanionChart) []types.CompanionChart {
	if len(charts) == 0 {
		return nil
	}
	result := make([]types.CompanionChart, len(charts))
	for i, c := range charts {
		result[i] = types.CompanionChart{
			ChartRef:    c.ChartRef,
			Version:     c.Version,
			ReleaseName: c.ReleaseName,
			ValuesFile:  c.ValuesFile,
			RepoName:    c.RepoName,
			RepoURL:     c.RepoURL,
		}
	}
	return result
}
