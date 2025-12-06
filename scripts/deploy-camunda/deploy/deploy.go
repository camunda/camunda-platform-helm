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
	"scripts/vault-secret-mapper/pkg/mapper"
	"strings"
	"sync"
	"time"

	"github.com/jwalton/gchalk"
)

// ScenarioContext holds scenario-specific deployment configuration.
type ScenarioContext struct {
	ScenarioName             string
	Namespace                string
	Release                  string
	IngressHost              string
	KeycloakRealm            string
	OptimizeIndexPrefix      string
	OrchestrationIndexPrefix string
	TasklistIndexPrefix      string
	OperateIndexPrefix       string
	TempDir                  string
}

// ScenarioResult holds the result of a scenario deployment.
type ScenarioResult struct {
	Scenario                 string
	Namespace                string
	Release                  string
	IngressHost              string
	KeycloakRealm            string
	OptimizeIndexPrefix      string
	OrchestrationIndexPrefix string
	FirstUserPassword        string
	SecondUserPassword       string
	ThirdUserPassword        string
	KeycloakClientsSecret    string
	Error                    error
}

// envMutex protects environment variable access during parallel deployments.
var envMutex sync.Mutex

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

// generateCompactRealmName creates a realm name that fits within Keycloak's 36 character limit.
// Format: {prefix}-{hash} where hash is derived from namespace+scenario+suffix.
func generateCompactRealmName(namespace, scenario, suffix string) string {
	const maxLength = 36

	// Try simple format first: scenario-suffix (e.g., "keycloak-mt-a8x9z3k1")
	simple := fmt.Sprintf("%s-%s", scenario, suffix)
	if len(simple) <= maxLength {
		return simple
	}

	// If scenario name is too long, truncate it and add a short hash for uniqueness
	// Format: {truncated-scenario}-{short-hash}
	// Reserve 9 characters for "-" + 8 char hash
	maxScenarioLen := maxLength - 9

	if len(scenario) > maxScenarioLen {
		scenario = scenario[:maxScenarioLen]
	}

	// Create a short hash from the full identifier for uniqueness
	fullId := fmt.Sprintf("%s-%s-%s", namespace, scenario, suffix)
	hash := fmt.Sprintf("%x", big.NewInt(0).SetBytes([]byte(fullId)).Int64())
	if len(hash) > 8 {
		hash = hash[:8]
	}

	result := fmt.Sprintf("%s-%s", scenario, hash)

	// Final safety check - truncate if still too long
	if len(result) > maxLength {
		result = result[:maxLength]
	}

	return result
}

// captureEnv saves current values of specified environment variables.
func captureEnv(keys []string) map[string]string {
	envVars := make(map[string]string, len(keys))
	for _, key := range keys {
		envVars[key] = os.Getenv(key)
	}
	return envVars
}

// restoreEnv restores environment variables to captured values.
func restoreEnv(envVars map[string]string) {
	for key, val := range envVars {
		if val == "" {
			_ = os.Unsetenv(key)
		} else {
			_ = os.Setenv(key, val)
		}
	}
}

// enhanceScenarioError wraps scenario resolution errors with helpful context.
func enhanceScenarioError(err error, scenario, scenarioPath, chartPath string) error {
	if err == nil {
		return nil
	}

	// Check if it's a "not found" type error
	errStr := err.Error()
	if !strings.Contains(errStr, "not found") && !strings.Contains(errStr, "no such file") {
		return err
	}

	// Try to list available scenarios
	scenarioDir := scenarioPath
	if scenarioDir == "" {
		// Default scenario location
		scenarioDir = filepath.Join(chartPath, "test/integration/scenarios/chart-full-setup")
	}

	var helpMsg strings.Builder
	fmt.Fprintf(&helpMsg, "❌ Scenario %q not found\n\n", scenario)
	fmt.Fprintf(&helpMsg, "Searched in: %s\n", scenarioDir)
	fmt.Fprintf(&helpMsg, "Expected file: values-integration-test-ingress-%s.yaml\n\n", scenario)

	// Try to list available scenarios
	entries, readErr := os.ReadDir(scenarioDir)
	if readErr != nil {
		fmt.Fprintf(&helpMsg, "⚠️  Could not list available scenarios: %v\n\n", readErr)
		fmt.Fprintf(&helpMsg, "Please check:\n")
		fmt.Fprintf(&helpMsg, "  1. The scenario directory exists: %s\n", scenarioDir)
		fmt.Fprintf(&helpMsg, "  2. You have permission to read it\n")
		fmt.Fprintf(&helpMsg, "  3. The --chart-path or --scenario-path flags are set correctly\n")
	} else {
		var availableScenarios []string
		for _, e := range entries {
			name := e.Name()
			if !e.IsDir() && strings.HasPrefix(name, "values-integration-test-ingress-") && strings.HasSuffix(name, ".yaml") {
				// Extract scenario name
				scenarioName := strings.TrimPrefix(name, "values-integration-test-ingress-")
				scenarioName = strings.TrimSuffix(scenarioName, ".yaml")
				availableScenarios = append(availableScenarios, scenarioName)
			}
		}

		if len(availableScenarios) == 0 {
			fmt.Fprintf(&helpMsg, "⚠️  No scenario files found in: %s\n\n", scenarioDir)
			fmt.Fprintf(&helpMsg, "Expected files matching pattern: values-integration-test-ingress-*.yaml\n")
		} else {
			fmt.Fprintf(&helpMsg, "✅ Available scenarios (%d found):\n", len(availableScenarios))
			for _, s := range availableScenarios {
				fmt.Fprintf(&helpMsg, "  • %s\n", s)
			}
			fmt.Fprintf(&helpMsg, "\n💡 Hint: Use --scenario <name> or --scenario <name1>,<name2> for multiple scenarios\n")
		}
	}

	fmt.Fprintf(&helpMsg, "\n📚 Documentation: Check the chart's test/integration/scenarios/ directory\n")
	fmt.Fprintf(&helpMsg, "   for available scenario configurations.\n")

	return fmt.Errorf("%s\n%s", helpMsg.String(), err)
}

// Execute performs the actual Camunda deployment based on the provided flags.
func Execute(ctx context.Context, flags *config.RuntimeFlags) error {
	// Check if we're deploying multiple scenarios in parallel
	if len(flags.Scenarios) > 1 {
		return executeParallelDeployments(ctx, flags)
	}

	// Single scenario deployment (original behavior)
	return executeSingleDeployment(ctx, flags)
}

// executeParallelDeployments deploys multiple scenarios concurrently.
func executeParallelDeployments(ctx context.Context, flags *config.RuntimeFlags) error {
	logging.Logger.Info().
		Int("count", len(flags.Scenarios)).
		Strs("scenarios", flags.Scenarios).
		Msg("Starting parallel deployment of multiple scenarios")

	// Validate all scenarios exist before starting any deployments
	// This provides better error messages and fails fast
	scenarioDir := flags.ScenarioPath
	if scenarioDir == "" {
		scenarioDir = filepath.Join(flags.ChartPath, "test/integration/scenarios/chart-full-setup")
	}

	for _, scenario := range flags.Scenarios {
		// Try to resolve the scenario file
		var filename string
		if strings.HasPrefix(scenario, "values-integration-test-ingress-") && strings.HasSuffix(scenario, ".yaml") {
			filename = scenario
		} else {
			filename = fmt.Sprintf("values-integration-test-ingress-%s.yaml", scenario)
		}

		sourceValuesFile := filepath.Join(scenarioDir, filename)
		if _, err := os.Stat(sourceValuesFile); err != nil {
			// Enhance error with helpful context
			return enhanceScenarioError(err, scenario, flags.ScenarioPath, flags.ChartPath)
		}
	}

	logging.Logger.Info().Msg("All scenarios validated successfully")

	// Create scenario contexts
	contexts := make([]*ScenarioContext, len(flags.Scenarios))
	for i, scenario := range flags.Scenarios {
		contexts[i] = generateScenarioContext(scenario, flags)
	}

	// Deploy scenarios in parallel independently (don't cancel on first failure)
	// Use WaitGroup instead of errgroup to allow all scenarios to complete
	var wg sync.WaitGroup
	resultCh := make(chan *ScenarioResult, len(flags.Scenarios))

	for _, scenarioCtx := range contexts {
		scenarioCtx := scenarioCtx // capture for closure
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Use original context (not a cancellable one) so failures don't cancel others
			result := deployScenario(ctx, scenarioCtx, flags)
			resultCh <- result
		}()
	}

	// Wait for all deployments to complete
	wg.Wait()
	close(resultCh)

	// Collect results
	results := make([]*ScenarioResult, 0, len(flags.Scenarios))
	for result := range resultCh {
		results = append(results, result)
	}

	// Print summary
	printMultiScenarioSummary(results)

	// Return error if any scenario failed
	var hasErrors bool
	for _, r := range results {
		if r.Error != nil {
			hasErrors = true
			break
		}
	}

	if hasErrors {
		return fmt.Errorf("one or more scenarios failed deployment")
	}
	return nil
}

// executeSingleDeployment deploys a single scenario (original behavior).
func executeSingleDeployment(ctx context.Context, flags *config.RuntimeFlags) error {
	scenario := flags.Scenarios[0]
	scenarioCtx := generateScenarioContext(scenario, flags)
	result := deployScenario(ctx, scenarioCtx, flags)

	if result.Error != nil {
		return result.Error
	}

	// Print single deployment summary
	printDeploymentSummary(result.KeycloakRealm, result.OptimizeIndexPrefix, result.OrchestrationIndexPrefix)
	return nil
}

// generateScenarioContext creates a scenario-specific deployment context.
func generateScenarioContext(scenario string, flags *config.RuntimeFlags) *ScenarioContext {
	suffix := generateRandomSuffix()

	// Generate unique identifiers for this scenario
	var realmName, optimizePrefix, orchestrationPrefix, tasklistPrefix, operatePrefix string
	var namespace, release, ingressHost string

	// Use provided values or generate unique ones
	if flags.KeycloakRealm != "" && len(flags.Scenarios) == 1 {
		realmName = flags.KeycloakRealm
	} else {
		// Keycloak realm name has a maximum length of 36 characters
		// Generate a compact name that fits within this limit
		realmName = generateCompactRealmName(flags.Namespace, scenario, suffix)
	}

	if flags.OptimizeIndexPrefix != "" && len(flags.Scenarios) == 1 {
		optimizePrefix = flags.OptimizeIndexPrefix
	} else {
		optimizePrefix = fmt.Sprintf("opt-%s-%s", scenario, suffix)
	}

	if flags.OrchestrationIndexPrefix != "" && len(flags.Scenarios) == 1 {
		orchestrationPrefix = flags.OrchestrationIndexPrefix
	} else {
		orchestrationPrefix = fmt.Sprintf("orch-%s-%s", scenario, suffix)
	}

	if flags.TasklistIndexPrefix != "" && len(flags.Scenarios) == 1 {
		tasklistPrefix = flags.TasklistIndexPrefix
	} else {
		tasklistPrefix = fmt.Sprintf("task-%s-%s", scenario, suffix)
	}

	if flags.OperateIndexPrefix != "" && len(flags.Scenarios) == 1 {
		operatePrefix = flags.OperateIndexPrefix
	} else {
		operatePrefix = fmt.Sprintf("op-%s-%s", scenario, suffix)
	}

	// Generate unique namespace for multi-scenario, but always use "integration" as release name
	// since we never have multiple deployments in the same namespace
	resolvedHost := flags.ResolveIngressHostname()
	if len(flags.Scenarios) > 1 {
		namespace = fmt.Sprintf("%s-%s", flags.Namespace, scenario)
		if resolvedHost != "" {
			ingressHost = fmt.Sprintf("%s-%s", scenario, resolvedHost)
		}
	} else {
		namespace = flags.Namespace
		ingressHost = resolvedHost
	}

	// Always use "integration" as the release name
	release = "integration"

	return &ScenarioContext{
		ScenarioName:             scenario,
		Namespace:                namespace,
		Release:                  release,
		IngressHost:              ingressHost,
		KeycloakRealm:            realmName,
		OptimizeIndexPrefix:      optimizePrefix,
		OrchestrationIndexPrefix: orchestrationPrefix,
		TasklistIndexPrefix:      tasklistPrefix,
		OperateIndexPrefix:       operatePrefix,
	}
}

// deployScenario performs deployment for a single scenario.
func deployScenario(ctx context.Context, scenarioCtx *ScenarioContext, flags *config.RuntimeFlags) *ScenarioResult {
	result := &ScenarioResult{
		Scenario:                 scenarioCtx.ScenarioName,
		Namespace:                scenarioCtx.Namespace,
		Release:                  scenarioCtx.Release,
		IngressHost:              scenarioCtx.IngressHost,
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
	var realmName, optimizePrefix, orchestrationPrefix string

	realmName = scenarioCtx.KeycloakRealm
	logging.Logger.Info().Str("realm", realmName).Str("scenario", scenarioCtx.ScenarioName).Msg("Using Keycloak realm")

	optimizePrefix = scenarioCtx.OptimizeIndexPrefix
	logging.Logger.Info().Str("optimize", optimizePrefix).Str("scenario", scenarioCtx.ScenarioName).Msg("Using Optimize index prefix")

	orchestrationPrefix = scenarioCtx.OrchestrationIndexPrefix
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

	// Thread-safe environment variable manipulation
	// Lock for values processing phase only
	envMutex.Lock()

	// Set environment variables for prepare-helm-values
	originalEnv := captureEnv([]string{
		"KEYCLOAK_REALM",
		"OPTIMIZE_INDEX_PREFIX",
		"ORCHESTRATION_INDEX_PREFIX",
		"TASKLIST_INDEX_PREFIX",
		"OPERATE_INDEX_PREFIX",
		"CAMUNDA_HOSTNAME",
		"FLOW",
	})

	// Ensure environment is restored and mutex is unlocked even on error
	unlocked := false
	defer func() {
		if !unlocked {
			restoreEnv(originalEnv)
			envMutex.Unlock()
		}
	}()

	os.Setenv("KEYCLOAK_REALM", realmName)
	os.Setenv("OPTIMIZE_INDEX_PREFIX", optimizePrefix)
	os.Setenv("ORCHESTRATION_INDEX_PREFIX", orchestrationPrefix)
	if scenarioCtx.TasklistIndexPrefix != "" {
		os.Setenv("TASKLIST_INDEX_PREFIX", scenarioCtx.TasklistIndexPrefix)
	}
	if scenarioCtx.OperateIndexPrefix != "" {
		os.Setenv("OPERATE_INDEX_PREFIX", scenarioCtx.OperateIndexPrefix)
	}
	if scenarioCtx.IngressHost != "" {
		os.Setenv("CAMUNDA_HOSTNAME", scenarioCtx.IngressHost)
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
			// Enhance error with helpful context about available scenarios
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
			result.Error = fmt.Errorf("failed to prepare auth scenario: %w", err)
			return result
		}
	}

	// Process main scenario
	logging.Logger.Info().Str("scenario", scenarioCtx.ScenarioName).Msg("Preparing main scenario")
	if err := processValues(scenarioCtx.ScenarioName); err != nil {
		result.Error = fmt.Errorf("failed to prepare main scenario: %w", err)
		return result
	}

	// Auto-generate secrets if requested
	if flags.AutoGenerateSecrets {
		if err := generateTestSecrets(flags.EnvFile); err != nil {
			result.Error = err
			return result
		}
	}

	// Generate vault secrets if mapping is provided
	var vaultSecretPath string
	if flags.VaultSecretMapping != "" || flags.AutoGenerateSecrets {
		vaultSecretPath = filepath.Join(tempDir, "vault-mapped-secrets.yaml")
		logging.Logger.Info().Str("scenario", scenarioCtx.ScenarioName).Msg("Generating vault secrets")
		mapping := flags.VaultSecretMapping
		// If auto-generating secrets, use a default mapping if none provided
		if mapping == "" {
			mapping = os.Getenv("vault_secret_mapping")
		}
		if err := mapper.Generate(mapping, "vault-mapped-secrets", vaultSecretPath); err != nil {
			result.Error = fmt.Errorf("failed to generate vault secrets: %w", err)
			return result
		}
	}

	// Build values files list
	vals, err := deployer.BuildValuesList(tempDir, []string{scenarioCtx.ScenarioName}, flags.Auth, false, false, flags.ExtraValues)
	if err != nil {
		result.Error = err
		return result
	}

	// Restore environment and unlock mutex before deployment
	// This allows other scenarios to proceed with their env setup
	restoreEnv(originalEnv)
	envMutex.Unlock()
	unlocked = true // Mark as unlocked to prevent defer from unlocking again

	// Determine timeout duration from flags (default to 5 minutes if not set)
	timeoutMinutes := flags.Timeout
	if timeoutMinutes <= 0 {
		timeoutMinutes = 5
	}

	// Perform deployment
	deployOpts := types.Options{
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

// printMultiScenarioSummary outputs the deployment results for multiple scenarios.
func printMultiScenarioSummary(results []*ScenarioResult) {
	successCount := 0
	failureCount := 0
	for _, r := range results {
		if r.Error == nil {
			successCount++
		} else {
			failureCount++
		}
	}

	if !logging.IsTerminal(os.Stdout.Fd()) {
		// Plain, machine-friendly output
		var out strings.Builder
		fmt.Fprintf(&out, "parallel deployment: completed\n")
		fmt.Fprintf(&out, "total scenarios: %d\n", len(results))
		fmt.Fprintf(&out, "successful: %d\n", successCount)
		fmt.Fprintf(&out, "failed: %d\n", failureCount)
		fmt.Fprintf(&out, "\nscenarios:\n")
		for _, r := range results {
			fmt.Fprintf(&out, "- scenario: %s\n", r.Scenario)
			fmt.Fprintf(&out, "  namespace: %s\n", r.Namespace)
			fmt.Fprintf(&out, "  release: %s\n", r.Release)
			if r.Error != nil {
				fmt.Fprintf(&out, "  status: failed\n")
				fmt.Fprintf(&out, "  error: %v\n", r.Error)
			} else {
				fmt.Fprintf(&out, "  status: success\n")
				fmt.Fprintf(&out, "  realm: %s\n", r.KeycloakRealm)
				fmt.Fprintf(&out, "  optimizeIndexPrefix: %s\n", r.OptimizeIndexPrefix)
				fmt.Fprintf(&out, "  orchestrationIndexPrefix: %s\n", r.OrchestrationIndexPrefix)
				if r.IngressHost != "" {
					fmt.Fprintf(&out, "  ingressHost: %s\n", r.IngressHost)
				}
				fmt.Fprintf(&out, "  credentials:\n")
				fmt.Fprintf(&out, "    firstUserPassword: %s\n", r.FirstUserPassword)
				fmt.Fprintf(&out, "    secondUserPassword: %s\n", r.SecondUserPassword)
				fmt.Fprintf(&out, "    thirdUserPassword: %s\n", r.ThirdUserPassword)
				fmt.Fprintf(&out, "    keycloakClientsSecret: %s\n", r.KeycloakClientsSecret)
			}
		}
		logging.Logger.Info().Msg(out.String())
		return
	}

	// Pretty, human-friendly output
	styleKey := func(s string) string { return logging.Emphasize(s, gchalk.Cyan) }
	styleVal := func(s string) string { return logging.Emphasize(s, gchalk.Magenta) }
	styleOk := func(s string) string { return logging.Emphasize(s, gchalk.Green) }
	styleErr := func(s string) string { return logging.Emphasize(s, gchalk.Red) }
	styleHead := func(s string) string { return logging.Emphasize(s, gchalk.Bold) }
	styleWarn := func(s string) string { return logging.Emphasize(s, gchalk.Yellow) }

	var out strings.Builder
	if failureCount == 0 {
		out.WriteString(styleOk("🎉 All scenarios deployed successfully!"))
	} else if successCount == 0 {
		out.WriteString(styleErr("❌ All scenarios failed to deploy"))
	} else {
		out.WriteString(styleWarn(fmt.Sprintf("⚠️  Partial success: %d/%d scenarios deployed", successCount, len(results))))
	}
	out.WriteString("\n\n")

	// Summary
	out.WriteString(styleHead("Deployment Summary"))
	out.WriteString("\n")
	fmt.Fprintf(&out, "  Total scenarios: %s\n", styleVal(fmt.Sprintf("%d", len(results))))
	fmt.Fprintf(&out, "  Successful: %s\n", styleOk(fmt.Sprintf("%d", successCount)))
	if failureCount > 0 {
		fmt.Fprintf(&out, "  Failed: %s\n", styleErr(fmt.Sprintf("%d", failureCount)))
	}
	out.WriteString("\n")

	// Details per scenario
	maxKey := 30
	for i, r := range results {
		if i > 0 {
			out.WriteString("\n")
		}

		if r.Error != nil {
			out.WriteString(styleErr(fmt.Sprintf("Scenario %d: %s [FAILED]", i+1, r.Scenario)))
			out.WriteString("\n")
			fmt.Fprintf(&out, "  %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "Namespace")), styleVal(r.Namespace))
			fmt.Fprintf(&out, "  %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "Error")), styleErr(r.Error.Error()))
		} else {
			out.WriteString(styleOk(fmt.Sprintf("Scenario %d: %s [SUCCESS]", i+1, r.Scenario)))
			out.WriteString("\n")
			fmt.Fprintf(&out, "  %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "Namespace")), styleVal(r.Namespace))
			fmt.Fprintf(&out, "  %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "Release")), styleVal(r.Release))
			if r.IngressHost != "" {
				fmt.Fprintf(&out, "  %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "Ingress Host")), styleVal(r.IngressHost))
			}
			fmt.Fprintf(&out, "  %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "Keycloak Realm")), styleVal(r.KeycloakRealm))
			fmt.Fprintf(&out, "  %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "Optimize Index Prefix")), styleVal(r.OptimizeIndexPrefix))
			fmt.Fprintf(&out, "  %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "Orchestration Index Prefix")), styleVal(r.OrchestrationIndexPrefix))
			out.WriteString(styleHead("  Credentials:"))
			out.WriteString("\n")
			fmt.Fprintf(&out, "    %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey-2, "First user password")), styleVal(r.FirstUserPassword))
			fmt.Fprintf(&out, "    %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey-2, "Second user password")), styleVal(r.SecondUserPassword))
			fmt.Fprintf(&out, "    %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey-2, "Third user password")), styleVal(r.ThirdUserPassword))
			fmt.Fprintf(&out, "    %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey-2, "Keycloak clients secret")), styleVal(r.KeycloakClientsSecret))
		}
	}

	out.WriteString("\n")
	if failureCount == 0 {
		out.WriteString("Please keep these credentials safe. All deployments are ready to use! 🚀")
	}

	logging.Logger.Info().Msg(out.String())
}
