package deploy

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"scripts/camunda-core/pkg/logging"
	"scripts/camunda-core/pkg/scenarios"
	"scripts/camunda-deployer/pkg/deployer"
	"scripts/deploy-camunda/config"
	"scripts/prepare-helm-values/pkg/env"
	"scripts/prepare-helm-values/pkg/values"
	"scripts/vault-secret-mapper/pkg/mapper"
)

// processCommonValues finds and processes common values files from the common/ sibling directory.
// It processes each file through values.Process() to apply env var substitution and writes to outputDir.
// If platform is specified, it also processes files from the platform-specific subdirectory (e.g., common/eks/).
// envOverrides, when non-nil, is passed through to values.Options.EnvOverrides so that
// placeholder substitution uses the caller-supplied env map instead of the process environment.
// Returns the list of processed file paths in the output directory.
func processCommonValues(scenarioPath, outputDir, envFile, platform string, envOverrides map[string]string) ([]string, error) {
	// Common directory is a sibling to the scenario directory
	commonDir := filepath.Join(filepath.Dir(scenarioPath), "..", "common")

	logging.Logger.Debug().
		Str("scenarioPath", scenarioPath).
		Str("commonDir", commonDir).
		Str("outputDir", outputDir).
		Str("platform", platform).
		Msg("🔍 [processCommonValues] looking for common values directory")

	info, err := os.Stat(commonDir)
	if err != nil || !info.IsDir() {
		logging.Logger.Debug().
			Str("commonDir", commonDir).
			Msg("🔍 [processCommonValues] common directory not found - skipping")
		return nil, nil
	}

	// Collect common values files in order
	var sourceFiles []string

	// First, add predefined common files in order (if they exist)
	for _, fileName := range deployer.CommonValuesFiles {
		p := filepath.Join(commonDir, fileName)
		if _, err := os.Stat(p); err == nil {
			logging.Logger.Debug().
				Str("file", p).
				Msg("🔍 [processCommonValues] found predefined common values file")
			sourceFiles = append(sourceFiles, p)
		}
	}

	// Then, discover any additional values-*.yaml files not in the predefined list
	entries, err := os.ReadDir(commonDir)
	if err != nil {
		logging.Logger.Debug().
			Err(err).
			Str("commonDir", commonDir).
			Msg("⚠️ [processCommonValues] failed to read common directory")
		return sourceFiles, nil
	}

	predefinedSet := make(map[string]bool)
	for _, f := range deployer.CommonValuesFiles {
		predefinedSet[f] = true
	}

	var additionalFiles []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if predefinedSet[name] {
			continue
		}
		if strings.HasPrefix(name, "values-") && strings.HasSuffix(name, ".yaml") {
			p := filepath.Join(commonDir, name)
			logging.Logger.Debug().
				Str("file", p).
				Msg("🔍 [processCommonValues] found additional common values file")
			additionalFiles = append(additionalFiles, p)
		}
	}

	// Sort additional files for deterministic ordering
	sort.Strings(additionalFiles)
	sourceFiles = append(sourceFiles, additionalFiles...)

	// Discover platform-specific files from common/<platform>/ subdirectory
	if strings.TrimSpace(platform) != "" {
		platformDir := filepath.Join(commonDir, platform)
		logging.Logger.Debug().
			Str("platformDir", platformDir).
			Str("platform", platform).
			Msg("🔍 [processCommonValues] looking for platform-specific values directory")

		if pInfo, pErr := os.Stat(platformDir); pErr == nil && pInfo.IsDir() {
			platformEntries, pReadErr := os.ReadDir(platformDir)
			if pReadErr != nil {
				logging.Logger.Debug().
					Err(pReadErr).
					Str("platformDir", platformDir).
					Msg("⚠️ [processCommonValues] failed to read platform directory")
			} else {
				var platformFiles []string
				for _, entry := range platformEntries {
					if entry.IsDir() {
						continue
					}
					name := entry.Name()
					if strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml") {
						p := filepath.Join(platformDir, name)
						logging.Logger.Debug().
							Str("file", p).
							Str("platform", platform).
							Msg("🔍 [processCommonValues] found platform-specific values file")
						platformFiles = append(platformFiles, p)
					}
				}
				// Sort platform files for deterministic ordering
				sort.Strings(platformFiles)
				sourceFiles = append(sourceFiles, platformFiles...)
			}
		} else {
			logging.Logger.Debug().
				Str("platformDir", platformDir).
				Str("platform", platform).
				Msg("🔍 [processCommonValues] platform-specific directory not found - skipping")
		}
	}

	if len(sourceFiles) == 0 {
		logging.Logger.Debug().
			Str("commonDir", commonDir).
			Msg("🔍 [processCommonValues] no common values files found")
		return nil, nil
	}

	// Process each common file
	var processedFiles []string
	for _, srcFile := range sourceFiles {
		logging.Logger.Debug().
			Str("source", srcFile).
			Str("outputDir", outputDir).
			Str("envFile", envFile).
			Msg("⚙️ [processCommonValues] processing common values file")

		opts := values.Options{
			OutputDir:    outputDir,
			EnvFile:      envFile,
			EnvOverrides: envOverrides,
		}

		outputPath, _, err := values.Process(srcFile, opts)
		if err != nil {
			logging.Logger.Debug().
				Err(err).
				Str("source", srcFile).
				Msg("❌ [processCommonValues] failed to process common values file")
			return nil, fmt.Errorf("failed to process common values file %q: %w", srcFile, err)
		}

		logging.Logger.Debug().
			Str("source", srcFile).
			Str("output", outputPath).
			Msg("✅ [processCommonValues] processed common values file")
		processedFiles = append(processedFiles, outputPath)
	}

	logging.Logger.Debug().
		Strs("processedFiles", processedFiles).
		Int("count", len(processedFiles)).
		Msg("✅ [processCommonValues] all common values files processed")

	return processedFiles, nil
}

// generateDebugValuesFile creates a temporary values file with debug configuration
// for the specified components. Returns the path to the generated file, or empty string
// if no debug components are enabled.
func generateDebugValuesFile(outputDir string, flags *config.RuntimeFlags) (string, error) {
	if len(flags.Debug.DebugComponents) == 0 {
		return "", nil
	}

	// Determine suspend mode
	suspendMode := "n"
	if flags.Debug.DebugSuspend {
		suspendMode = "y"
	}

	// Build debug values YAML
	var yamlContent strings.Builder

	for component, debugCfg := range flags.Debug.DebugComponents {
		switch component {
		case "orchestration":
			// Include default JVM options + debug agent
			// The default javaOpts from values.yaml are:
			// -XX:+HeapDumpOnOutOfMemoryError -XX:HeapDumpPath=/usr/local/camunda/data
			// -XX:ErrorFile=/usr/local/camunda/data/camunda_error%p.log -XX:+ExitOnOutOfMemoryError
			debugJavaOpts := fmt.Sprintf(
				"-XX:+HeapDumpOnOutOfMemoryError -XX:HeapDumpPath=/usr/local/camunda/data -XX:ErrorFile=/usr/local/camunda/data/camunda_error%%p.log -XX:+ExitOnOutOfMemoryError -agentlib:jdwp=transport=dt_socket,server=y,suspend=%s,address=*:%d",
				suspendMode,
				debugCfg.Port,
			)

			// For debugging, set clusterSize, partitionCount, and replicationFactor to 1
			// to avoid complexity with multiple replicas during debug sessions
			yamlContent.WriteString(fmt.Sprintf(`orchestration:
  clusterSize: "1"
  partitionCount: "1"
  replicationFactor: "1"
  env:
    - name: JAVA_TOOL_OPTIONS
      value: "%s"
  service:
    extraPorts:
      - name: debug
        protocol: TCP
        port: %d
        targetPort: %d
`, debugJavaOpts, debugCfg.Port, debugCfg.Port))

			logging.Logger.Info().
				Str("component", "orchestration").
				Int("port", debugCfg.Port).
				Bool("suspend", flags.Debug.DebugSuspend).
				Msg("Debug mode enabled for component (clusterSize, partitionCount, replicationFactor set to 1)")

		case "connectors":
			// Connectors uses JAVA_TOOL_OPTIONS via connectors.env
			debugJavaOpts := fmt.Sprintf(
				"-agentlib:jdwp=transport=dt_socket,server=y,suspend=%s,address=*:%d",
				suspendMode,
				debugCfg.Port,
			)

			// Set replicas to 1 for easier debugging
			yamlContent.WriteString(fmt.Sprintf(`connectors:
  replicas: 1
  env:
    - name: JAVA_TOOL_OPTIONS
      value: "%s"
`, debugJavaOpts))

			logging.Logger.Info().
				Str("component", "connectors").
				Int("port", debugCfg.Port).
				Bool("suspend", flags.Debug.DebugSuspend).
				Msg("Debug mode enabled for component (replicas set to 1)")

		default:
			logging.Logger.Warn().
				Str("component", component).
				Msg("Unknown debug component (supported: orchestration, connectors)")
		}
	}

	if yamlContent.Len() == 0 {
		return "", nil
	}

	// Write to temp file
	debugValuesPath := filepath.Join(outputDir, "values-debug.yaml")
	if err := os.WriteFile(debugValuesPath, []byte(yamlContent.String()), 0644); err != nil {
		return "", fmt.Errorf("failed to write debug values file: %w", err)
	}

	logging.Logger.Debug().
		Str("path", debugValuesPath).
		Msg("Generated debug values file")

	return debugValuesPath, nil
}

// buildScenarioEnv creates an isolated environment map for a scenario by snapshotting
// the process environment, loading the .env file, and applying scenario-specific
// overrides. The returned map is used as values.Options.EnvOverrides so that
// parallel calls to values.Process never touch (or race on) the real process env.
func buildScenarioEnv(scenarioCtx *ScenarioContext, flags *config.RuntimeFlags) (map[string]string, error) {
	// 1. Snapshot the current process environment as the baseline.
	envMap := make(map[string]string)
	for _, entry := range os.Environ() {
		if k, v, ok := strings.Cut(entry, "="); ok {
			envMap[k] = v
		}
	}

	// 2. Overlay .env file values (without modifying the process environment).
	if flags.EnvFile != "" {
		dotenvMap, err := env.ReadFile(flags.EnvFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read env file %q: %w", flags.EnvFile, err)
		}
		for k, v := range dotenvMap {
			envMap[k] = v
		}
	}

	// 3. Apply scenario-specific overrides.
	envMap["KEYCLOAK_REALM"] = scenarioCtx.KeycloakRealm
	envMap["OPTIMIZE_INDEX_PREFIX"] = scenarioCtx.OptimizeIndexPrefix
	envMap["ORCHESTRATION_INDEX_PREFIX"] = scenarioCtx.OrchestrationIndexPrefix
	if scenarioCtx.TasklistIndexPrefix != "" {
		envMap["TASKLIST_INDEX_PREFIX"] = scenarioCtx.TasklistIndexPrefix
	}
	if scenarioCtx.OperateIndexPrefix != "" {
		envMap["OPERATE_INDEX_PREFIX"] = scenarioCtx.OperateIndexPrefix
	}
	if scenarioCtx.IngressHost != "" {
		envMap["CAMUNDA_HOSTNAME"] = scenarioCtx.IngressHost
	}
	envMap["FLOW"] = flags.Deployment.Flow

	// Default ES pool index to 0 if not set (for local dev / manual runs).
	esPoolIndex := flags.ESPoolIndex
	if esPoolIndex == "" {
		esPoolIndex = envMap["ES_POOL_INDEX"]
	}
	if esPoolIndex == "" {
		esPoolIndex = "0"
	}
	envMap["ES_POOL_INDEX"] = esPoolIndex

	// 4. Apply per-entry extra environment variables (e.g., VENOM_CLIENT_ID, CONNECTORS_CLIENT_ID).
	for k, v := range flags.ExtraEnv {
		envMap[k] = v
	}

	// 5. Set Keycloak versioned environment variables.
	if flags.Auth.KeycloakHost != "" {
		kcVersionSafe := keycloakVersionSuffix(flags.Auth.KeycloakHost)
		envMap[fmt.Sprintf("KEYCLOAK_EXT_HOST_%s", kcVersionSafe)] = flags.Auth.KeycloakHost
		envMap[fmt.Sprintf("KEYCLOAK_EXT_PROTOCOL_%s", kcVersionSafe)] = flags.Auth.KeycloakProtocol
	}

	return envMap, nil
}

// buildDeploymentConfigFromFlags creates a DeploymentConfig from RuntimeFlags.
// It prefers the new selection flags (--identity, --persistence, etc.) over deprecated flags.
// Returns nil if no explicit flags are set (use auto-detection from scenario name).
func buildDeploymentConfigFromFlags(flags *config.RuntimeFlags, scenarioName string) *scenarios.DeploymentConfig {
	// Migrate deprecated flags to new fields first
	flags.MigrateDeprecatedFlags()

	// Check if we have any explicit configuration
	if !flags.HasExplicitSelectionConfig() && !flags.HasExplicitLayeredConfig() {
		return nil
	}

	return scenarios.BuildDeploymentConfig(scenarioName, scenarios.BuilderOverrides{
		Identity:     flags.Selection.Identity,
		Persistence:  flags.Selection.Persistence,
		Platform:     flags.Selection.TestPlatform,
		Features:     flags.Selection.Features,
		QA:           flags.Selection.QA,
		ImageTags:    flags.Selection.ImageTags,
		Upgrade:      flags.Selection.UpgradeFlow,
		ChartVersion: flags.Chart.ChartVersion,
		Flow:         flags.Deployment.Flow,
	})
}

// enhanceScenarioError wraps scenario resolution errors with helpful context.
// Supports both layered values (values/ directory) and legacy single-file approach.
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
	fmt.Fprintf(&helpMsg, "Scenario %q not found\n\n", scenario)
	fmt.Fprintf(&helpMsg, "Searched in: %s\n\n", scenarioDir)

	// Check if this directory uses layered values (new structure)
	valuesDir := filepath.Join(scenarioDir, "values")
	if _, statErr := os.Stat(valuesDir); statErr == nil {
		// Layered values structure
		fmt.Fprintf(&helpMsg, "This scenario directory uses SELECTION + COMPOSITION model.\n")
		fmt.Fprintf(&helpMsg, "Scenario names are derived from layer combinations.\n\n")

		// List available identity types
		identityDir := filepath.Join(valuesDir, "identity")
		if identityEntries, err := os.ReadDir(identityDir); err == nil && len(identityEntries) > 0 {
			fmt.Fprintf(&helpMsg, "Available identity types (--identity):\n")
			for _, e := range identityEntries {
				if !e.IsDir() && strings.HasSuffix(e.Name(), ".yaml") {
					name := strings.TrimSuffix(e.Name(), ".yaml")
					fmt.Fprintf(&helpMsg, "  - %s\n", name)
				}
			}
			fmt.Fprintf(&helpMsg, "\n")
		}

		// List available persistence types
		persistenceDir := filepath.Join(valuesDir, "persistence")
		if persistenceEntries, err := os.ReadDir(persistenceDir); err == nil && len(persistenceEntries) > 0 {
			fmt.Fprintf(&helpMsg, "Available persistence types (--persistence):\n")
			for _, e := range persistenceEntries {
				if !e.IsDir() && strings.HasSuffix(e.Name(), ".yaml") {
					name := strings.TrimSuffix(e.Name(), ".yaml")
					fmt.Fprintf(&helpMsg, "  - %s\n", name)
				}
			}
			fmt.Fprintf(&helpMsg, "\n")
		}

		// List available platform types
		platformDir := filepath.Join(valuesDir, "platform")
		if platformEntries, err := os.ReadDir(platformDir); err == nil && len(platformEntries) > 0 {
			fmt.Fprintf(&helpMsg, "Available platforms (--test-platform):\n")
			for _, e := range platformEntries {
				if !e.IsDir() && strings.HasSuffix(e.Name(), ".yaml") {
					name := strings.TrimSuffix(e.Name(), ".yaml")
					fmt.Fprintf(&helpMsg, "  - %s\n", name)
				}
			}
			fmt.Fprintf(&helpMsg, "\n")
		}

		// List available features
		featuresDir := filepath.Join(valuesDir, "features")
		if featureEntries, err := os.ReadDir(featuresDir); err == nil && len(featureEntries) > 0 {
			fmt.Fprintf(&helpMsg, "Available features (--features):\n")
			for _, e := range featureEntries {
				if !e.IsDir() && strings.HasSuffix(e.Name(), ".yaml") {
					name := strings.TrimSuffix(e.Name(), ".yaml")
					fmt.Fprintf(&helpMsg, "  - %s\n", name)
				}
			}
			fmt.Fprintf(&helpMsg, "\n")
		}

		fmt.Fprintf(&helpMsg, "Hint: Use the new flags directly. Examples:\n")
		fmt.Fprintf(&helpMsg, "  --identity keycloak --persistence elasticsearch --test-platform gke\n")
		fmt.Fprintf(&helpMsg, "  --identity keycloak-external --persistence opensearch --test-platform gke --features multitenancy\n")
		fmt.Fprintf(&helpMsg, "  --identity keycloak --persistence elasticsearch --test-platform gke --qa --image-tags\n")
	} else {
		// Legacy single-file structure
		fmt.Fprintf(&helpMsg, "Expected file: values-integration-test-ingress-%s.yaml\n\n", scenario)

		// Try to list available scenarios
		entries, readErr := os.ReadDir(scenarioDir)
		if readErr != nil {
			fmt.Fprintf(&helpMsg, "Could not list available scenarios: %v\n\n", readErr)
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
				fmt.Fprintf(&helpMsg, "No scenario files found in: %s\n\n", scenarioDir)
				fmt.Fprintf(&helpMsg, "Expected files matching pattern: values-integration-test-ingress-*.yaml\n")
			} else {
				fmt.Fprintf(&helpMsg, "Available scenarios (%d found):\n", len(availableScenarios))
				for _, s := range availableScenarios {
					fmt.Fprintf(&helpMsg, "  - %s\n", s)
				}
				fmt.Fprintf(&helpMsg, "\nHint: Use --scenario <name> or --scenario <name1>,<name2> for multiple scenarios\n")
			}
		}
	}

	fmt.Fprintf(&helpMsg, "\nDocumentation: Check the chart's test/integration/scenarios/ directory\n")
	fmt.Fprintf(&helpMsg, "for available scenario configurations.\n")

	return fmt.Errorf("%s\n%s", helpMsg.String(), err)
}

// prepareScenarioValues processes values files for a scenario and returns a PreparedScenario.
// Each invocation builds an isolated env map via buildScenarioEnv so that parallel
// calls never race on the process environment. The resulting map is passed to
// values.Process via EnvOverrides.
func prepareScenarioValues(scenarioCtx *ScenarioContext, flags *config.RuntimeFlags) (*PreparedScenario, error) {
	logging.Logger.Debug().
		Str("scenario", scenarioCtx.ScenarioName).
		Str("namespace", scenarioCtx.Namespace).
		Str("keycloakRealm", scenarioCtx.KeycloakRealm).
		Msg("📋 [prepareScenarioValues] ENTRY - starting values preparation")

	logging.Logger.Info().
		Str("scenario", scenarioCtx.ScenarioName).
		Str("namespace", scenarioCtx.Namespace).
		Msg("Preparing scenario values")

	// Generate identifiers
	realmName := scenarioCtx.KeycloakRealm
	optimizePrefix := scenarioCtx.OptimizeIndexPrefix
	orchestrationPrefix := scenarioCtx.OrchestrationIndexPrefix

	// Create temp directory for values
	logging.Logger.Debug().
		Str("scenario", scenarioCtx.ScenarioName).
		Str("pattern", fmt.Sprintf("camunda-values-%s-*", scenarioCtx.ScenarioName)).
		Msg("📁 [prepareScenarioValues] creating temporary directory for values files")

	tempDir, err := os.MkdirTemp("", fmt.Sprintf("camunda-values-%s-*", scenarioCtx.ScenarioName))
	if err != nil {
		logging.Logger.Debug().
			Err(err).
			Str("scenario", scenarioCtx.ScenarioName).
			Msg("❌ [prepareScenarioValues] FAILED to create temp directory")
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	scenarioCtx.TempDir = tempDir
	logging.Logger.Debug().Str("dir", tempDir).Str("scenario", scenarioCtx.ScenarioName).Msg("✅ [prepareScenarioValues] temp directory created successfully")

	// Build an isolated environment map for this scenario. This replaces the
	// old envMutex + os.Setenv/os.Getenv approach: each scenario gets its own
	// map so parallel calls never race on the process environment.
	envMap, err := buildScenarioEnv(scenarioCtx, flags)
	if err != nil {
		os.RemoveAll(tempDir)
		return nil, fmt.Errorf("failed to build scenario env: %w", err)
	}

	// Helper function to process values files
	processValues := func(scen string) error {
		logging.Logger.Debug().
			Str("scenario", scen).
			Str("chartPath", flags.Chart.ChartPath).
			Str("scenarioDir", flags.Deployment.ScenarioPath).
			Str("outputDir", tempDir).
			Bool("interactive", flags.Interactive).
			Str("envFile", flags.EnvFile).
			Msg("📝 [prepareScenarioValues.processValues] building values options")

		opts := values.Options{
			ChartPath:    flags.Chart.ChartPath,
			Scenario:     scen,
			ScenarioDir:  flags.Deployment.ScenarioPath,
			OutputDir:    tempDir,
			Interactive:  flags.Interactive,
			EnvFile:      flags.EnvFile,
			EnvOverrides: envMap,
		}

		file, err := values.ResolveValuesFile(opts)
		if err != nil {
			return enhanceScenarioError(err, scen, flags.Deployment.ScenarioPath, flags.Chart.ChartPath)
		}

		_, _, err = values.Process(file, opts)
		if err != nil {
			return fmt.Errorf("failed to process scenario %q: %w", scen, err)
		}
		logging.Logger.Debug().
			Str("scenario", scen).
			Str("file", file).
			Msg("✅ [prepareScenarioValues.processValues] values file processed successfully")
		return nil
	}

	// Process common values files first (base layer)
	logging.Logger.Debug().
		Str("scenarioPath", flags.Deployment.ScenarioPath).
		Str("tempDir", tempDir).
		Str("platform", flags.Deployment.Platform).
		Msg("📋 [prepareScenarioValues] processing common values files")
	processedCommonFiles, err := processCommonValues(flags.Deployment.ScenarioPath, tempDir, flags.EnvFile, flags.Deployment.Platform, envMap)
	if err != nil {
		os.RemoveAll(tempDir) // Cleanup on error
		return nil, fmt.Errorf("failed to process common values: %w", err)
	}

	// Determine the effective scenario directory for resolution.
	effectiveScenarioDir := flags.Deployment.ScenarioPath
	if effectiveScenarioDir == "" {
		effectiveScenarioDir = filepath.Join(flags.Chart.ChartPath, "test/integration/scenarios/chart-full-setup")
	}
	isLayered := scenarios.HasLayeredValues(effectiveScenarioDir)

	// Process and resolve scenario values.
	// For layered values, we resolve all layer files from the original scenario directory,
	// process each one into tempDir, and build the values list directly.
	// For legacy values, we use the existing processValues + BuildValuesList flow.
	var scenarioValueFiles []string
	var resolvedLayerFiles []string // source layer files before env var processing (for display)
	if isLayered {
		logging.Logger.Debug().
			Str("scenarioDir", effectiveScenarioDir).
			Str("scenario", scenarioCtx.ScenarioName).
			Msg("📋 [prepareScenarioValues] layered values detected; resolving all layer files")

		// Build deployment config via the canonical builder, applying any explicit
		// flag overrides so that --platform / --test-platform propagates to the
		// layered values resolution (e.g. selecting eks.yaml instead of gke.yaml).
		effectivePlatform := flags.Selection.TestPlatform
		if effectivePlatform == "" {
			effectivePlatform = flags.Deployment.Platform
		}
		deployConfig := scenarios.BuildDeploymentConfig(scenarioCtx.ScenarioName, scenarios.BuilderOverrides{
			Identity:     flags.Selection.Identity,
			Persistence:  flags.Selection.Persistence,
			Platform:     effectivePlatform,
			Features:     flags.Selection.Features,
			InfraType:    flags.Selection.InfraType,
			Flow:         flags.Deployment.Flow,
			QA:           flags.Selection.QA,
			ImageTags:    flags.Selection.ImageTags,
			Upgrade:      flags.Selection.UpgradeFlow,
			ChartVersion: flags.Chart.ChartVersion,
		})

		layeredFiles, err := deployConfig.ResolvePaths(effectiveScenarioDir)
		if err != nil {
			os.RemoveAll(tempDir)
			return nil, fmt.Errorf("failed to resolve layered values for scenario %q: %w", scenarioCtx.ScenarioName, err)
		}

		logging.Logger.Debug().
			Strs("layeredFiles", layeredFiles).
			Int("count", len(layeredFiles)).
			Msg("📋 [prepareScenarioValues] processing layered values files")

		// Log resolved deployment config and layer files at INFO level for visibility.
		// Show short relative paths (from scenario dir) instead of absolute paths.
		shortFiles := make([]string, len(layeredFiles))
		for i, f := range layeredFiles {
			if rel, err := filepath.Rel(effectiveScenarioDir, f); err == nil {
				shortFiles[i] = rel
			} else {
				shortFiles[i] = filepath.Base(f)
			}
		}
		logging.Logger.Info().
			Str("scenario", scenarioCtx.ScenarioName).
			Str("identity", deployConfig.Identity).
			Str("persistence", deployConfig.Persistence).
			Str("platform", deployConfig.Platform).
			Str("infraType", deployConfig.InfraType).
			Strs("features", deployConfig.Features).
			Strs("layerFiles", shortFiles).
			Msg("Resolved deployment layers")
		resolvedLayerFiles = shortFiles

		// Process each layered file (env var substitution) into tempDir
		for _, srcFile := range layeredFiles {
			opts := values.Options{
				ChartPath:    flags.Chart.ChartPath,
				ScenarioDir:  effectiveScenarioDir,
				OutputDir:    tempDir,
				Interactive:  flags.Interactive,
				EnvFile:      flags.EnvFile,
				EnvOverrides: envMap,
			}
			outputPath, _, procErr := values.Process(srcFile, opts)
			if procErr != nil {
				os.RemoveAll(tempDir)
				return nil, fmt.Errorf("failed to process layered values file %q: %w", srcFile, procErr)
			}
			scenarioValueFiles = append(scenarioValueFiles, outputPath)
		}

		logging.Logger.Debug().
			Strs("processedFiles", scenarioValueFiles).
			Msg("📋 [prepareScenarioValues] layered values files processed")

		// Deep-merge all layered files into a single YAML file to prevent
		// Helm's shallow array replacement from silently dropping entries.
		// Without this, a later layer's array (e.g., rba.yaml's operate.env)
		// completely replaces an earlier layer's array (e.g., elasticsearch-external.yaml's
		// operate.env), losing critical env vars like index prefix overrides.
		scenarioValueFiles, err = MergeLayeredValues(scenarioValueFiles, tempDir)
		if err != nil {
			os.RemoveAll(tempDir)
			return nil, fmt.Errorf("failed to merge layered values: %w", err)
		}
		logging.Logger.Debug().
			Strs("mergedFiles", scenarioValueFiles).
			Msg("📋 [prepareScenarioValues] layered values merged")
	} else {
		// Legacy path: process auth and main scenario, then let BuildValuesList resolve
		effectiveAuth := flags.Auth.Auth
		if effectiveAuth != "" && effectiveAuth != scenarioCtx.ScenarioName {
			logging.Logger.Info().Str("auth", effectiveAuth).Str("scenario", scenarioCtx.ScenarioName).Msg("Preparing auth scenario")
			if err := processValues(effectiveAuth); err != nil {
				os.RemoveAll(tempDir)
				return nil, fmt.Errorf("failed to prepare auth scenario: %w", err)
			}
		}

		// Process main scenario
		logging.Logger.Info().Str("scenario", scenarioCtx.ScenarioName).Msg("Preparing main scenario")
		if err := processValues(scenarioCtx.ScenarioName); err != nil {
			os.RemoveAll(tempDir)
			return nil, fmt.Errorf("failed to prepare main scenario: %w", err)
		}
	}

	// Auto-generate secrets if requested
	var secrets map[string]string
	if flags.Secrets.AutoGenerateSecrets {
		logging.Logger.Debug().
			Str("scenario", scenarioCtx.ScenarioName).
			Msg("🔑 [prepareScenarioValues] auto-generating test secrets")
		secrets, err = generateTestSecrets(flags.EnvFile, envMap)
		if err != nil {
			os.RemoveAll(tempDir) // Cleanup on error
			return nil, fmt.Errorf("failed to generate test secrets: %w", err)
		}
		// Merge secrets into envMap so that vault_secret_mapping is available below.
		for k, v := range secrets {
			envMap[k] = v
		}
	}

	// Generate vault secrets if auto-generating
	var vaultSecretPath string
	if flags.Secrets.AutoGenerateSecrets {
		vaultSecretPath = filepath.Join(tempDir, "vault-mapped-secrets.yaml")
		logging.Logger.Info().Str("scenario", scenarioCtx.ScenarioName).Msg("Generating vault secrets")
		mapping := flags.Secrets.VaultSecretMapping
		if mapping == "" {
			mapping = envMap["vault_secret_mapping"]
		}
		if err := mapper.Generate(mapping, "vault-mapped-secrets", vaultSecretPath); err != nil {
			os.RemoveAll(tempDir) // Cleanup on error
			return nil, fmt.Errorf("failed to generate vault secrets: %w", err)
		}
	}

	// Build values files list
	logging.Logger.Debug().
		Str("scenario", scenarioCtx.ScenarioName).
		Str("tempDir", tempDir).
		Msg("📋 [prepareScenarioValues] building values files list")

	// Generate debug values file if debug components are specified
	debugValuesFile, err := generateDebugValuesFile(tempDir, flags)
	if err != nil {
		os.RemoveAll(tempDir)
		return nil, fmt.Errorf("failed to generate debug values: %w", err)
	}

	var vals []string
	if isLayered {
		// For layered values, we already have the processed scenario files.
		// Build the values list directly: common + scenario layers + overlays + user values.
		vals = append(vals, processedCommonFiles...)
		vals = append(vals, scenarioValueFiles...)
		vals = append(vals, flags.Deployment.ExtraValues...)
	} else {
		// Legacy path: let BuildValuesList resolve scenario files from tempDir.
		vals, err = deployer.BuildValuesList(tempDir, []string{scenarioCtx.ScenarioName}, flags.Auth.Auth, false, false, flags.Deployment.ExtraValues, processedCommonFiles)
		if err != nil {
			os.RemoveAll(tempDir)
			return nil, fmt.Errorf("failed to build values list: %w", err)
		}
	}

	// Append chart-root overlay files (e.g., values-enterprise.yaml, values-digest.yaml).
	// Each name in ChartRootOverlays resolves to <chartPath>/values-<name>.yaml.
	// Applied after scenario layers + extra values, before debug values.
	// Not passed through values.Process() — these files contain literal values, no env placeholders.
	for _, overlay := range flags.Chart.ChartRootOverlays {
		if flags.Chart.ChartPath == "" {
			continue // repo-based installs (upgrade Step 1) have no local chart path
		}
		overlayPath := filepath.Join(flags.Chart.ChartPath, "values-"+overlay+".yaml")
		if _, statErr := os.Stat(overlayPath); statErr == nil {
			vals = append(vals, overlayPath)
			logging.Logger.Info().
				Str("overlay", overlay).
				Str("path", overlayPath).
				Msg("Including chart-root overlay")
		} else {
			logging.Logger.Debug().
				Str("overlay", overlay).
				Str("path", overlayPath).
				Msg("Chart-root overlay not found, skipping")
		}
	}

	// Append debug values file last to ensure it overrides other values
	if debugValuesFile != "" {
		vals = append(vals, debugValuesFile)
	}

	logging.Logger.Debug().
		Str("scenario", scenarioCtx.ScenarioName).
		Strs("valuesFiles", vals).
		Int("count", len(vals)).
		Msg("✅ [prepareScenarioValues] EXIT - values preparation completed successfully")

	logging.Logger.Info().
		Str("scenario", scenarioCtx.ScenarioName).
		Int("valuesFilesCount", len(vals)).
		Msg("Scenario values preparation completed")

	return &PreparedScenario{
		ScenarioCtx:         scenarioCtx,
		ValuesFiles:         vals,
		LayeredFiles:        resolvedLayerFiles,
		VaultSecretPath:     vaultSecretPath,
		TempDir:             tempDir,
		RealmName:           realmName,
		OptimizePrefix:      optimizePrefix,
		OrchestrationPrefix: orchestrationPrefix,
		Secrets:             secrets,
	}, nil
}
