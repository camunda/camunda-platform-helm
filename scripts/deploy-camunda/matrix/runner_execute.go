// Copyright 2026 Camunda Services GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package matrix

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"scripts/camunda-core/pkg/executil"
	"scripts/camunda-core/pkg/logging"
	"scripts/camunda-core/pkg/versionmatrix"
	"scripts/deploy-camunda/auth0"
	"scripts/deploy-camunda/config"
	"scripts/deploy-camunda/deploy"
	"scripts/deploy-camunda/entra"
	"scripts/prepare-helm-values/pkg/env"
)

// ResolveNamespace is the exported form of resolveNamespace, for callers
// outside the matrix package that need the same "what namespace would a
// normal (single-namespace) matrix entry get" computation — e.g. the
// topology deploy driver derives each release's namespace as
// "<this>-<namespace-suffix>" rather than deploying into it directly.
func ResolveNamespace(opts RunOptions, entry Entry) string {
	return resolveNamespace(opts, entry)
}

// companionChartsForEntry builds the companion chart list for a matrix entry
// from its ci-test-config dependencies. Values file paths are resolved relative
// to the repo root; chart references are resolved to absolute paths when they
// point at an existing local directory, otherwise passed through as remote
// repo/chart names. Shared by the deploy path (executeEntry) and the dry-run
// preflight so both see the same companion env-var requirements.
func companionChartsForEntry(entry Entry, repoRoot string) []config.CompanionChart {
	if len(entry.Dependencies) == 0 {
		return nil
	}
	charts := make([]config.CompanionChart, 0, len(entry.Dependencies))
	for _, dep := range entry.Dependencies {
		chartRef := dep.Chart
		version := dep.Version
		localChartPath := filepath.Join(repoRoot, dep.Chart)
		if info, err := os.Stat(localChartPath); err == nil && info.IsDir() {
			chartRef = localChartPath
			version = "" // --version is only meaningful for remote charts
		}
		cc := config.CompanionChart{
			ChartRef:    chartRef,
			Version:     version,
			ReleaseName: dep.ReleaseName,
			EnvVars:     dep.EnvVars,
			RepoName:    dep.RepoName,
			RepoURL:     dep.RepoURL,
		}
		if dep.ValuesFile != "" {
			cc.ValuesFile = filepath.Join(repoRoot, dep.ValuesFile)
		}
		charts = append(charts, cc)
	}
	return charts
}

// appendScenarioExtraValues resolves a scenario's declared extra-values files
// against scenarioDir (absolute paths pass through) and appends them to base.
// Resolving to absolute paths here sidesteps helm's CWD-relative -f resolution.
func appendScenarioExtraValues(base []string, entry Entry, scenarioDir string) []string {
	for _, p := range entry.ExtraValues {
		if filepath.IsAbs(p) {
			base = append(base, p)
			continue
		}
		base = append(base, filepath.Join(scenarioDir, p))
	}
	return base
}

// BuildEntryFlags is the exported, reusable form of the per-entry RuntimeFlags
// assembly executeEntry uses: namespace/platform/kubeContext resolution,
// companion charts (from entry.Dependencies), and the vault secret mapping
// used by the fail-fast preflight. It does NOT cover OIDC/Auth0 provisioning,
// two-step-upgrade lifecycle hooks, or per-entry log-file redirection — those
// remain specific to executeEntry's own orchestration.
//
// Exported so callers outside the matrix package (the topology deploy driver
// in deploy.ExecuteTopology / cmd's runTopologyEntry) can synthesize a
// per-release matrix.Entry and get the exact same flags/companion-chart/
// vault-mapping assembly a normal matrix entry gets, instead of hand-rolling
// a partial RuntimeFlags that silently omits companion charts or the
// per-entry ingress hostname.
func BuildEntryFlags(entry Entry, opts RunOptions) (flags *config.RuntimeFlags, namespace string, kubeCtx string, envFile string, err error) {
	namespace = resolveNamespace(opts, entry)
	baseNamespace := buildBaseNamespace(entry)
	// When the caller overrides the namespace (per-scenario CI workflow, or a
	// topology release's own namespace), feed the override directly into
	// RuntimeFlags and clear the prefix so EffectiveNamespace() resolves to
	// the override verbatim.
	flagsNamespace := baseNamespace
	flagsNamespacePrefix := opts.NamespacePrefix
	if opts.NamespaceOverride != "" {
		flagsNamespace = opts.NamespaceOverride
		flagsNamespacePrefix = ""
	}

	platform := resolvePlatform(opts, entry)
	infra := EntryInfra(opts, platform, entry.Version)
	kubeCtx, envFile = infra.KubeContext, infra.EnvFile
	strategy, err := ResolveImageStrategy(entry, opts)
	if err != nil {
		return nil, namespace, kubeCtx, envFile, err
	}
	logging.Logger.Info().Str("version", entry.Version).Str("scenario", entry.Scenario).Str("images", string(strategy)).Msg("Image strategy")
	useVault := infra.UseVaultBackedSecrets != nil && *infra.UseVaultBackedSecrets
	explicitIngressHost := explicitIngressHost(opts)
	baseDomain := infra.IngressBaseDomain

	// Compute the scenario directory. deploy.Execute uses this to resolve
	// values files — both layered and legacy formats are handled there.
	scenarioDir := filepath.Join(entry.ChartPath, "test/integration/scenarios/chart-full-setup")

	// Build the test exclude string from entry excludes (goroutine-safe via RuntimeFlags,
	// avoids using os.Setenv which is process-global and unsafe for concurrent execution).
	testExclude := ""
	if len(entry.Exclude) > 0 {
		testExclude = strings.Join(entry.Exclude, "|")
	}

	// Default log level to "info" if not set.
	logLevel := opts.LogLevel
	if logLevel == "" {
		logLevel = "info"
	}

	// Default Keycloak host/protocol if not set.
	keycloakHost := config.FirstNonEmpty(opts.KeycloakHost, config.DefaultKeycloakHost)
	keycloakProtocol := config.FirstNonEmpty(opts.KeycloakProtocol, config.DefaultKeycloakProtocol)

	flags = &config.RuntimeFlags{
		LogLevel:    logLevel,
		Interactive: false,
		EnvFile:     envFile,
		// Leave SkipPreflight false so deploy.Execute's fail-fast preflight runs
		// per entry. By this point flags.CompanionCharts is populated from the
		// entry's dependencies, so the preflight catches missing companion vars
		// (e.g. RDBMS_POSTGRESQL_*) and any unset scenario placeholders up front —
		// with a clear remediation — instead of failing deep in value prep. The
		// kube reachability probe is skipped in that path, so this adds no network
		// round-trip on top of the runner's existing docker login + context warmup.
		Chart: config.ChartFlags{
			ChartPath:            entry.ChartPath,
			SkipDependencyUpdate: opts.SkipDependencyUpdate,
			RepoRoot:             opts.RepoRoot,
			ChartRootOverlays:    chartRootOverlays(entry, strategy),
		},
		Deployment: config.DeploymentFlags{
			Namespace:                  flagsNamespace,
			NamespacePrefix:            flagsNamespacePrefix,
			Release:                    "integration",
			Scenario:                   entry.Scenario,
			Scenarios:                  []string{entry.Scenario},
			ScenarioPath:               scenarioDir,
			Platform:                   platform,
			Flow:                       entry.Flow,
			Timeout:                    opts.HelmTimeout,
			DeleteNamespaceFirst:       opts.DeleteNamespaceFirst,
			WaitIngressReady:           opts.WaitIngressReady,
			IngressReadyTimeoutMinutes: opts.IngressReadyTimeoutMinutes,
			ExtraHelmArgs:              append([]string(nil), opts.ExtraHelmArgs...),
			// Global --extra-values first, then scenario-declared extra-values
			// (resolved against the scenario dir) so the per-scenario files win
			// within the chain's `extra` slot.
			ExtraValues:       appendScenarioExtraValues(append([]string(nil), opts.ExtraValues...), entry, scenarioDir),
			ImageOverrides:    append([]string(nil), opts.ImageOverrides...),
			AllowDigestShadow: opts.AllowDigestShadow,
			// Always include allowPreReleaseImages=true for CI deployments —
			// matches the legacy Taskfile install/upgrade behaviour. User-supplied
			// --extra-helm-set values are merged on top and take precedence.
			ExtraHelmSets: mergeHelmSets(
				map[string]string{"orchestration.upgrade.allowPreReleaseImages": "true"},
				parseHelmSetPairs(opts.ExtraHelmSets),
			),
		},
		Ingress: config.IngressFlags{
			// Ingress: use the namespace as subdomain so each entry gets a unique hostname.
			// e.g., namespace "matrix-89-eske-inst" + base "ci.distro.ultrawombat.com"
			//     → hostname "matrix-89-eske-inst.ci.distro.ultrawombat.com"
			// The base domain is resolved per-platform (GKE/EKS may have different domains).
			IngressHostname:   explicitIngressHost,
			IngressSubdomain:  ingressSubdomain(baseDomain, namespace, explicitIngressHost),
			IngressBaseDomain: ingressBaseDomain(baseDomain, explicitIngressHost),
		},
		Auth: config.AuthFlags{
			Auth:             entry.Auth,
			KeycloakHost:     keycloakHost,
			KeycloakProtocol: keycloakProtocol,
		},
		Docker: config.DockerFlags{
			DockerUsername:       opts.DockerUsername,
			DockerPassword:       opts.DockerPassword,
			EnsureDockerRegistry: opts.EnsureDockerRegistry,
			DockerHubUsername:    opts.DockerHubUsername,
			DockerHubPassword:    opts.DockerHubPassword,
			EnsureDockerHub:      opts.EnsureDockerHub,
			// Docker login is performed once in Run() before parallel dispatch.
			// Each entry only creates per-namespace K8s pull secrets.
			SkipDockerLogin: true,
		},
		Secrets: config.SecretsFlags{
			// When the caller pre-creates the namespace (--namespace-override) it
			// also pre-applies platform secrets/TLS via cluster-setup-secrets. Skip
			// the runner's ExternalSecrets path in that case — on EKS the runner
			// would otherwise wait a second time for aws-camunda-cloud-tls, which
			// the replicator has already written into the test namespace.
			ExternalSecrets:       opts.NamespaceOverride == "",
			AutoGenerateSecrets:   true,
			UseVaultBackedSecrets: useVault,
		},
		Test: config.TestFlags{
			KubeContext: kubeCtx,
			TestExclude: testExclude,
			RunE2ETests: (opts.TestE2E || opts.TestAll) && !entry.SkipE2E,
			// Do NOT propagate RunAllTests here — RunE2ETests already encodes
			// the full decision (including skip-e2e from ci-test-config.yaml).
			// Setting RunAllTests would bypass the skip logic in deploy/test.go
			// which ORs RunAllTests with RunE2ETests.
			RunAllTests: false,
		},
		// Selection + Composition: pass explicit layer overrides from ci-test-config.yaml.
		// When set, these override MapScenarioToConfig name-based derivation in deploy.go.
		Selection: config.SelectionFlags{
			Identity:    entry.Identity,
			Persistence: entry.Persistence,
			Features:    entry.Features,
			InfraType:   entry.InfraType,
			QA:          entry.QA || opts.UseQA,
			ImageTags:   strategy == ImageTags,
			UpgradeFlow: entry.Upgrade,
		},
	}

	// Wire companion chart dependencies from ci-test-config.yaml.
	flags.CompanionCharts = append(flags.CompanionCharts, companionChartsForEntry(entry, opts.RepoRoot)...)

	// Populate the vault secret mapping up front so the fail-fast preflight sees
	// it; prepareScenarioValues otherwise resolves it only after preflight runs.
	if flags.Secrets.AutoGenerateSecrets {
		if mapping, mErr := deploy.TestSecretMapping(); mErr == nil {
			flags.Secrets.VaultSecretMapping = mapping
		} else {
			logging.Logger.Warn().Err(mErr).Msg("Could not load embedded vault secret mapping for preflight validation")
		}
	}

	return flags, namespace, kubeCtx, envFile, nil
}

func explicitIngressHost(opts RunOptions) string {
	return parseHelmSetPairs(opts.ExtraHelmSets)["global.host"]
}

// executeEntry deploys a single matrix entry by constructing RuntimeFlags and calling deploy.Execute().
// The flow determines the execution strategy:
//   - Two-step upgrade (upgrade-patch, upgrade-minor): Step 1 installs old version, Step 2 upgrades.
//   - Upgrade-only (modular-upgrade-minor): Upgrades an already-running deployment (no install step).
//   - Install (default): Single-step fresh install.
func executeEntry(ctx context.Context, entry Entry, opts RunOptions) RunResult {
	start := time.Now()
	namespace := resolveNamespace(opts, entry)

	if opts.OnEntryStart != nil {
		opts.OnEntryStart(entry, namespace)
	}

	// Fire "preparing" phase and wire flags.OnPhase so deploy/test callbacks
	// propagate back to the status display.
	if opts.OnPhaseChange != nil {
		opts.OnPhaseChange(entry, "preparing")
	}

	flags, namespace, kubeCtx, envFile, err := BuildEntryFlags(entry, opts)
	if err != nil {
		return RunResult{Entry: entry, Namespace: namespace, KubeContext: kubeCtx, Error: err}
	}
	platform := resolvePlatform(opts, entry)

	// Wire phase reporting: deploy.Execute and RunTests call flags.OnPhase,
	// which we forward to the matrix-level OnPhaseChange callback.
	if opts.OnPhaseChange != nil {
		flags.OnPhase = func(phase string) {
			opts.OnPhaseChange(entry, phase)
		}
	}

	ctx, closeLogs := openEntryLogs(ctx, entry, opts, flags)
	defer closeLogs()

	var venomOpts *entra.Options
	if entra.IsOIDCEntry(entry.Auth, entry.Identity) {
		venomOpts, err = prepareEntraEntry(ctx, flags, namespace, kubeCtx)
		if err != nil {
			return RunResult{Entry: entry, Namespace: namespace, KubeContext: kubeCtx, Error: err}
		}
	}

	var auth0Opts *auth0.Options
	if auth0.IsAuth0Identity(entry.Identity) {
		auth0Opts, err = prepareAuth0Entry(ctx, flags, namespace, kubeCtx)
		if err != nil {
			return finishEntry(ctx, RunResult{
				Entry:       entry,
				Namespace:   namespace,
				KubeContext: kubeCtx,
				Error:       err,
				Duration:    time.Since(start),
				auth0Opts:   auth0Opts,
			}, opts)
		}
	}

	logging.Logger.Info().
		Str("version", entry.Version).
		Str("scenario", entry.Scenario).
		Str("shortname", entry.Shortname).
		Str("auth", entry.Auth).
		Str("flow", entry.Flow).
		Str("namespace", namespace).
		Str("platform", platform).
		Str("infraType", entry.InfraType).
		Str("kubeContext", kubeCtx).
		Str("envFile", envFile).
		Str("chartPath", entry.ChartPath).
		Str("ingressHost", flags.ResolveIngressHostname()).
		Str("identity", entry.Identity).
		Str("persistence", entry.Persistence).
		Strs("features", entry.Features).
		Bool("vaultBackedSecrets", flags.Secrets.UseVaultBackedSecrets).
		Msg("Deploying matrix entry")

	// --- Lifecycle hook registration (single-step flows) ---
	// Two-step upgrade flows register pre-install against step1Flags and
	// post-deploy against step2Flags inside executeTwoStepUpgrade.
	// Upgrade-only flows skip pre-install entirely (no install step). We
	// append to flags.{PreInstall,PostDeploy}Hooks rather than overwriting
	// because earlier code (e.g. the OIDC venom-secret PreInstallHook
	// registered above) may have populated those slots already.
	isTwoStepUpgrade := versionmatrix.IsTwoStepUpgradeFlow(entry.Flow)
	isUpgradeOnly := versionmatrix.IsUpgradeOnlyFlow(entry.Flow)
	if !isTwoStepUpgrade && !isUpgradeOnly {
		if err := registerDeclarativePreInstallHook(flags, entry.PreInstall, opts.RepoRoot, entry.Version, entry.Scenario); err != nil {
			return RunResult{Entry: entry, Namespace: namespace, Error: err}
		}
		if err := registerDeclarativePostInfraHook(flags, entry.PostInfra, opts.RepoRoot, entry.Version, entry.Scenario); err != nil {
			return RunResult{Entry: entry, Namespace: namespace, Error: err}
		}
	}
	if !isTwoStepUpgrade {
		if err := registerDeclarativePostDeployHook(flags, entry.PostDeploy, opts.RepoRoot, entry.Version, entry.Scenario); err != nil {
			return RunResult{Entry: entry, Namespace: namespace, Error: err}
		}
	}

	// Execute the deployment (deploy + tests run inside deploy.Execute).
	// All code paths converge into a single result so cleanup runs exactly once.
	var deployErr error
	var diag string

	// When prefix-key is set, pin index prefixes using the prefix-key instead of
	// the scenario name. This ensures cross-version consistency: an install on 8.8
	// (scenario name "qa-opensearch-tasklist-v1") and an upgrade on 8.9
	// (scenario name "qa-opensearch-upg") produce identical prefixes when both
	// declare the same prefix-key.
	if entry.PrefixKey != "" {
		if err := deploy.PinScenarioPrefixes(entry.PrefixKey, flags); err != nil {
			return RunResult{Entry: entry, Namespace: namespace, KubeContext: kubeCtx, Error: fmt.Errorf("pin scenario prefixes (prefix-key=%s): %w", entry.PrefixKey, err)}
		}
	}

	// Override chart source when --chart-ref is set (OCI install path).
	// See applyChartRefOverride for details.
	applyChartRefOverride(&flags.Chart, opts)

	// Two-step upgrade flow: install old version first, then upgrade to current.
	if versionmatrix.IsTwoStepUpgradeFlow(entry.Flow) {
		deployErr = executeTwoStepUpgrade(ctx, entry, flags, opts)
	} else if versionmatrix.IsUpgradeOnlyFlow(entry.Flow) {
		// Upgrade-only flow (modular-upgrade-minor): upgrade an already-running deployment.
		// No Step 1 install — the prior "install" flow must have already deployed the old version.
		deployErr = executeUpgradeOnly(ctx, entry, flags, opts)
	} else {
		// Single-step install (default flow).
		deployErr = deploy.Execute(ctx, flags)
	}

	// Collect diagnostics on failure (before cleanup deletes the namespace).
	if deployErr != nil {
		diag = collectDiagnostics(namespace, kubeCtx)
		diag = appendTestOutputToDiagnostics(deployErr, namespace, diag)
	}

	result := RunResult{Entry: entry, Namespace: namespace, KubeContext: kubeCtx, Error: deployErr, Duration: time.Since(start), Diagnostics: diag, venomOpts: venomOpts, auth0Opts: auth0Opts}
	return finishEntry(ctx, result, opts)
}

func finishEntry(ctx context.Context, result RunResult, opts RunOptions) RunResult {
	if opts.Cleanup {
		if opts.OnPhaseChange != nil {
			opts.OnPhaseChange(result.Entry, "cleanup")
		}
		cleanupEntry(ctx, result)
	}
	if opts.OnEntryComplete != nil {
		opts.OnEntryComplete(result.Entry, result)
	}
	return result
}

func readIdentityEnv(provider, envFile string) map[string]string {
	if envFile == "" {
		return nil
	}
	values, err := env.ReadFile(envFile)
	if err != nil {
		logging.Logger.Warn().Err(err).Str("envFile", envFile).Msg("Could not read env file for " + provider + " credentials")
		return nil
	}
	return values
}

func prepareEntraEntry(ctx context.Context, flags *config.RuntimeFlags, namespace, kubeCtx string) (*entra.Options, error) {
	envMap := readIdentityEnv("Entra", flags.EnvFile)
	opts := entra.Options{
		Namespace: namespace, KubeContext: kubeCtx, SkipK8sSecret: true,
		DirectoryID:  envMap["ENTRA_APP_DIRECTORY_ID"],
		ClientID:     envMap["ENTRA_APP_CLIENT_ID"],
		ClientSecret: envMap["ENTRA_APP_CLIENT_SECRET"],
	}
	logging.Logger.Info().Str("namespace", namespace).
		Msg("OIDC entry detected — provisioning venom Entra app (Phase 1: API + env vars)")
	app, err := entra.EnsureVenomApp(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("entra: provision venom app: %w", err)
	}
	audience := opts.ClientID
	if audience == "" {
		audience = os.Getenv("ENTRA_APP_CLIENT_ID")
	}
	if flags.ExtraEnv == nil {
		flags.ExtraEnv = make(map[string]string)
	}
	flags.ExtraEnv["VENOM_CLIENT_ID"] = app.AppID
	flags.ExtraEnv["CONNECTORS_CLIENT_ID"] = audience
	flags.PreInstallHooks = append(flags.PreInstallHooks, func(hookCtx context.Context) error {
		logging.Logger.Info().Str("namespace", namespace).
			Msg("OIDC Phase 2 — creating venom-entra-credentials K8s secret (PreInstallHook)")
		return entra.CreateVenomK8sSecret(hookCtx, kubeCtx, namespace, app, audience)
	})
	return &opts, nil
}

func prepareAuth0Entry(ctx context.Context, flags *config.RuntimeFlags, namespace, kubeCtx string) (*auth0.Options, error) {
	ingressHost := flags.ResolveIngressHostname()
	if ingressHost == "" {
		ingressHost = os.Getenv("CAMUNDA_HOSTNAME")
	}
	if ingressHost == "" {
		ingressHost = os.Getenv("TEST_INGRESS_HOST")
	}
	envMap := readIdentityEnv("Auth0", flags.EnvFile)
	opts := auth0.Options{
		Namespace: namespace, KubeContext: kubeCtx, IngressHost: ingressHost, SkipK8sSecret: true,
		Domain:           envMap["AUTH0_DOMAIN"],
		Audience:         envMap["AUTH0_AUDIENCE"],
		MgmtToken:        envMap["AUTH0_MGMT_TOKEN"],
		MgmtClientID:     envMap["AUTH0_MGMT_CLIENT_ID"],
		MgmtClientSecret: envMap["AUTH0_MGMT_CLIENT_SECRET"],
	}
	for target, name := range map[*string]string{
		&opts.Domain: "AUTH0_DOMAIN", &opts.Audience: "AUTH0_AUDIENCE",
		&opts.MgmtToken: "AUTH0_MGMT_TOKEN", &opts.MgmtClientID: "AUTH0_MGMT_CLIENT_ID", &opts.MgmtClientSecret: "AUTH0_MGMT_CLIENT_SECRET",
	} {
		if *target == "" {
			*target = os.Getenv(name)
		}
	}
	logging.Logger.Info().Str("namespace", namespace).
		Msg("Auth0 entry detected — provisioning per-component clients (Phase 1: API + env vars)")
	prov, err := auth0.EnsureClients(ctx, opts)
	if err != nil {
		return &opts, fmt.Errorf("auth0: provision clients: %w", err)
	}
	if flags.ExtraEnv == nil {
		flags.ExtraEnv = make(map[string]string)
	}
	audience := opts.Audience
	if audience == "" {
		audience = auth0.DefaultAudience
	}
	flags.ExtraEnv["AUTH0_AUDIENCE"] = audience
	flags.ExtraEnv["AUTH0_ISSUER_URL"] = strings.TrimSuffix(opts.Domain, "/")
	if v := os.Getenv("AUTH0_INITIAL_ADMIN_EMAIL"); v != "" {
		flags.ExtraEnv["AUTH0_INITIAL_ADMIN_EMAIL"] = v
	} else {
		flags.ExtraEnv["AUTH0_INITIAL_ADMIN_EMAIL"] = "demo@camunda.com"
	}
	envName := strings.NewReplacer(" ", "_", "-", "_")
	for _, client := range prov.All() {
		flags.ExtraEnv["AUTH0_"+envName.Replace(strings.ToUpper(client.Component))+"_CLIENT_ID"] = client.ClientID
	}
	issuerForSecret := strings.TrimSuffix(opts.Domain, "/") + "/"
	secretName := opts.SecretName
	flags.PreInstallHooks = append(flags.PreInstallHooks, func(hookCtx context.Context) error {
		logging.Logger.Info().Str("namespace", namespace).
			Msg("Auth0 Phase 2 — creating client-secret-for-components K8s secret (PreInstallHook)")
		return auth0.CreateK8sSecret(hookCtx, kubeCtx, namespace, secretName, prov, nil, issuerForSecret, audience)
	})
	return &opts, nil
}

func openEntryLogs(ctx context.Context, entry Entry, opts RunOptions, flags *config.RuntimeFlags) (context.Context, func()) {
	var files []*os.File
	closeLogs := func() {
		for i := len(files) - 1; i >= 0; i-- {
			files[i].Close()
		}
	}
	if opts.LogDir == "" {
		return ctx, closeLogs
	}
	baseName := entryLogFileName(entry)
	if e2eFile, err := os.Create(filepath.Join(opts.LogDir, baseName+".e2e.log")); err != nil {
		logging.Logger.Warn().Err(err).Msg("Failed to create e2e log file, output will go to terminal")
	} else {
		files = append(files, e2eFile)
		flags.E2EOutputWriter = e2eFile
	}
	deployLog, err := os.Create(filepath.Join(opts.LogDir, baseName+".deploy.log"))
	if err != nil {
		logging.Logger.Warn().Err(err).Msg("Failed to create deploy log file")
		return ctx, closeLogs
	}
	files = append(files, deployLog)
	logLevel := flags.LogLevel
	writeEntryLog := func(level, msg string) {
		if logging.ShouldLog(logLevel, level) {
			fmt.Fprintf(deployLog, "[%s] %s: %s\n", time.Now().Format("15:04:05"), strings.ToUpper(level), msg)
		}
	}
	writeEntryLog("info", fmt.Sprintf("entry=%s namespace=%s platform=%s flow=%s", entryID(entry), resolveNamespace(opts, entry), resolvePlatform(opts, entry), entry.Flow))
	ctx = executil.ContextWithBuffer(ctx, func(level, line string) {
		writeEntryLog(level, line)
		switch strings.ToLower(level) {
		case "trace":
			logging.Logger.Trace().Msg(line)
		case "debug":
			logging.Logger.Debug().Msg(line)
		case "warn", "warning":
			logging.Logger.Warn().Msg(line)
		case "error":
			logging.Logger.Error().Msg(line)
		default:
			logging.Logger.Info().Msg(line)
		}
	})
	return ctx, closeLogs
}
