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

// executeEntry deploys a single matrix entry by constructing RuntimeFlags and calling deploy.Execute().
// The flow determines the execution strategy:
//   - Two-step upgrade (upgrade-patch, upgrade-minor): Step 1 installs old version, Step 2 upgrades.
//   - Upgrade-only (modular-upgrade-minor): Upgrades an already-running deployment (no install step).
//   - Install (default): Single-step fresh install.
func executeEntry(ctx context.Context, entry Entry, opts RunOptions) RunResult {
	start := time.Now()
	namespace := resolveNamespace(opts, entry)
	baseNamespace := buildBaseNamespace(entry)
	// When the caller overrides the namespace (per-scenario CI workflow), feed
	// the override directly into RuntimeFlags and clear the prefix so
	// EffectiveNamespace() resolves to the override verbatim.
	flagsNamespace := baseNamespace
	flagsNamespacePrefix := opts.NamespacePrefix
	if opts.NamespaceOverride != "" {
		flagsNamespace = opts.NamespaceOverride
		flagsNamespacePrefix = ""
	}

	if opts.OnEntryStart != nil {
		opts.OnEntryStart(entry, namespace)
	}

	// Fire "preparing" phase and wire flags.OnPhase so deploy/test callbacks
	// propagate back to the status display.
	if opts.OnPhaseChange != nil {
		opts.OnPhaseChange(entry, "preparing")
	}

	// Determine platform and kube context
	platform := resolvePlatform(opts, entry)
	kubeCtx := resolveKubeContext(opts, platform)
	envFile := resolveEnvFile(opts, entry.Version)
	envFile, cleanupEnvFile, err := sanitizeEnvFileForOCIImmutability(envFile, opts)
	defer cleanupEnvFile() // safe: cleanup is always a valid no-op func even on error
	if err != nil {
		return RunResult{Entry: entry, Namespace: namespace, KubeContext: kubeCtx, Error: err}
	}
	useVault := resolveUseVaultBackedSecrets(opts, platform)

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

	// Build the base flags (used for single-step install or as Step 2 for upgrades).
	flags := &config.RuntimeFlags{
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
			// Build chart-root overlays.
			// enterprise is composable (changes registry/repo, not tags).
			// digest, latest, and image-tags are mutually exclusive for image version resolution:
			//   - image-tags (SNAPSHOT tags from env) takes priority over digest/latest
			//   - useLatest selects values-latest.yaml instead of values-digest.yaml
			//   - digest is the CI default when neither image-tags nor useLatest is active
			ChartRootOverlays: func() []string {
				if ociImmutabilityMode(opts) {
					logging.Logger.Warn().
						Str("chartRef", opts.ChartRef).
						Msg("OCI immutability mode: skipping chart-root image overlays")
				}
				return resolveChartRootOverlays(entry, opts)
			}(),
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
			ExtraValues: appendScenarioExtraValues(append([]string(nil), opts.ExtraValues...), entry, scenarioDir),
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
			IngressSubdomain:  ingressSubdomain(resolveIngressBaseDomain(opts, platform), namespace),
			IngressBaseDomain: resolveIngressBaseDomain(opts, platform),
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
			// the runner's ExternalSecrets path in that case — re-running it on
			// EKS would try to read aws-camunda-cloud-tls from the global "certs"
			// namespace, which CI service accounts don't have RBAC for.
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
			ImageTags:   effectiveImageTags(entry, opts),
			UpgradeFlow: entry.Upgrade,
		},
	}

	// Wire companion chart dependencies from ci-test-config.yaml.
	flags.CompanionCharts = append(flags.CompanionCharts, companionChartsForEntry(entry, opts.RepoRoot)...)

	// Populate the vault secret mapping up front so the fail-fast preflight sees
	// it; prepareScenarioValues otherwise resolves it only after preflight runs.
	if flags.Secrets.AutoGenerateSecrets {
		if mapping, err := deploy.TestSecretMapping(); err == nil {
			flags.Secrets.VaultSecretMapping = mapping
		} else {
			logging.Logger.Warn().Err(err).Msg("Could not load embedded vault secret mapping for preflight validation")
		}
	}

	// Wire phase reporting: deploy.Execute and RunTests call flags.OnPhase,
	// which we forward to the matrix-level OnPhaseChange callback.
	if opts.OnPhaseChange != nil {
		flags.OnPhase = func(phase string) {
			opts.OnPhaseChange(entry, phase)
		}
	}

	// Redirect test script output and deploy logs to per-entry files when logDir is set.
	// This keeps output out of the terminal so the status table stays clean.
	if opts.LogDir != "" {
		baseName := entryLogFileName(entry)
		if e2eFile, err := os.Create(filepath.Join(opts.LogDir, baseName+".e2e.log")); err != nil {
			logging.Logger.Warn().Err(err).Msg("Failed to create e2e log file, output will go to terminal")
		} else {
			defer e2eFile.Close()
			flags.E2EOutputWriter = e2eFile
		}

		// Per-entry deploy log: captures all subprocess output (helm, kubectl, etc.)
		// plus lifecycle events, giving a complete timeline for this entry.
		deployLogPath := filepath.Join(opts.LogDir, baseName+".deploy.log")
		if deployLog, err := os.Create(deployLogPath); err != nil {
			logging.Logger.Warn().Err(err).Msg("Failed to create deploy log file")
		} else {
			defer deployLog.Close()
			writeEntryLog := func(level, msg string) {
				if !logging.ShouldLog(logLevel, level) {
					return
				}
				ts := time.Now().Format("15:04:05")
				fmt.Fprintf(deployLog, "[%s] %s: %s\n", ts, strings.ToUpper(level), msg)
			}
			writeEntryLog("info", fmt.Sprintf("entry=%s namespace=%s platform=%s flow=%s", entryID(entry), namespace, platform, entry.Flow))

			// Intercept all subprocess output (helm, kubectl) via executil buffer callback.
			// The callback tees lines to both the per-entry file and the normal logger.
			ctx = executil.ContextWithBuffer(ctx, func(level, line string) {
				writeEntryLog(level, line)
				prefix := logging.BuildPrefix(logging.FieldsFromContext(ctx), "")
				switch strings.ToLower(level) {
				case "trace":
					logging.Logger.Trace().Msg(prefix + line)
				case "debug":
					logging.Logger.Debug().Msg(prefix + line)
				case "warn", "warning":
					logging.Logger.Warn().Msg(prefix + line)
				case "error":
					logging.Logger.Error().Msg(prefix + line)
				default:
					logging.Logger.Info().Msg(prefix + line)
				}
			})
		}
	}

	// OIDC hook: provision a venom Entra app registration before deployment.
	// The entra package is the canonical implementation for OIDC app provisioning,
	// used by both this matrix runner and the "deploy-camunda entra" CLI subcommand.
	//
	// Two-phase approach: Phase 1 (Entra API provisioning + env vars) runs now,
	// before deploy.Execute(). Phase 2 (K8s secret creation) is deferred to a
	// PreInstallHook because deploy.Execute() may delete and recreate the
	// namespace (via DeleteNamespaceFirst), which would wipe any secret created
	// before namespace setup.
	var venomOpts *entra.Options
	if entra.IsOIDCEntry(entry.Auth, entry.Identity) {
		entraOpts := entra.Options{
			Namespace:     namespace,
			KubeContext:   kubeCtx,
			SkipK8sSecret: true, // Phase 2 is deferred to PreInstallHook.
		}

		// Populate Entra credentials from the version-specific env file.
		// resolveOpts in entra.go falls back to os.Getenv, but the version-specific
		// env file (e.g., --env-file-89) is only stored in flags.EnvFile for later
		// use by buildScenarioEnv — it is NOT loaded into the process environment.
		// Read it explicitly and inject the values into Options to avoid the lookup
		// miss (and to stay safe for parallel execution without os.Setenv races).
		if envFile != "" {
			envMap, err := env.ReadFile(envFile)
			if err != nil {
				logging.Logger.Warn().Err(err).Str("envFile", envFile).Msg("Could not read env file for Entra credentials")
			} else {
				if v, ok := envMap["ENTRA_APP_DIRECTORY_ID"]; ok && v != "" {
					entraOpts.DirectoryID = v
				}
				if v, ok := envMap["ENTRA_APP_CLIENT_ID"]; ok && v != "" {
					entraOpts.ClientID = v
				}
				if v, ok := envMap["ENTRA_APP_CLIENT_SECRET"]; ok && v != "" {
					entraOpts.ClientSecret = v
				}
			}
		}

		logging.Logger.Info().
			Str("namespace", namespace).
			Msg("OIDC entry detected — provisioning venom Entra app (Phase 1: API + env vars)")

		venomApp, err := entra.EnsureVenomApp(ctx, entraOpts)
		if err != nil {
			return RunResult{Entry: entry, Namespace: namespace, KubeContext: kubeCtx, Error: fmt.Errorf("entra: provision venom app: %w", err)}
		}
		venomOpts = &entraOpts

		// Inject VENOM_CLIENT_ID and CONNECTORS_CLIENT_ID via per-entry ExtraEnv
		// so that buildScenarioEnv merges them into the isolated env map for
		// values.Process(), avoiding process-global os.Setenv races when multiple
		// OIDC entries execute concurrently (each has a distinct venom app registration).
		audience := entraOpts.ClientID
		if audience == "" {
			audience = os.Getenv("ENTRA_APP_CLIENT_ID")
		}
		if flags.ExtraEnv == nil {
			flags.ExtraEnv = make(map[string]string)
		}
		flags.ExtraEnv["VENOM_CLIENT_ID"] = venomApp.AppID
		flags.ExtraEnv["CONNECTORS_CLIENT_ID"] = audience

		// Phase 2: register a PreInstallHook that creates the K8s secret after
		// the namespace exists and before helm install.
		flags.PreInstallHooks = append(flags.PreInstallHooks, func(hookCtx context.Context) error {
			logging.Logger.Info().
				Str("namespace", namespace).
				Msg("OIDC Phase 2 — creating venom-entra-credentials K8s secret (PreInstallHook)")
			return entra.CreateVenomK8sSecret(hookCtx, kubeCtx, namespace, venomApp, audience)
		})
	}

	// Auth0 hook: provision per-component Auth0 clients before deployment.
	// Mirrors the Entra two-phase pattern: client creation + env var injection
	// runs synchronously now; the K8s secret is created via a PreInstallHook
	// so it survives namespace recreation by deploy.Execute().
	var auth0Opts *auth0.Options
	if auth0.IsAuth0Identity(entry.Identity) {
		// Per-entry ingress host. flags.ResolveIngressHostname() is empty in CI
		// because test-integration-runner.yaml passes the host via
		// `--extra-helm-set global.host=...` + the CAMUNDA_HOSTNAME /
		// TEST_INGRESS_HOST env vars rather than --ingress-hostname. Auth0
		// clients need the host at creation time (it's baked into redirect
		// URIs), so fall back to those env vars if the flag is empty.
		ingressHost := flags.ResolveIngressHostname()
		if ingressHost == "" {
			ingressHost = os.Getenv("CAMUNDA_HOSTNAME")
		}
		if ingressHost == "" {
			ingressHost = os.Getenv("TEST_INGRESS_HOST")
		}
		auth0Options := auth0.Options{
			Namespace:     namespace,
			KubeContext:   kubeCtx,
			IngressHost:   ingressHost,
			SkipK8sSecret: true, // Phase 2 deferred to PreInstallHook.
		}

		// Read AUTH0_* credentials from the version-specific env file (same
		// pattern as Entra above). The env file is loaded explicitly because
		// flags.EnvFile isn't injected into os.Environ.
		if envFile != "" {
			envMap, err := env.ReadFile(envFile)
			if err != nil {
				logging.Logger.Warn().Err(err).Str("envFile", envFile).Msg("Could not read env file for Auth0 credentials")
			} else {
				if v, ok := envMap["AUTH0_DOMAIN"]; ok && v != "" {
					auth0Options.Domain = v
				}
				if v, ok := envMap["AUTH0_AUDIENCE"]; ok && v != "" {
					auth0Options.Audience = v
				}
				if v, ok := envMap["AUTH0_MGMT_TOKEN"]; ok && v != "" {
					auth0Options.MgmtToken = v
				}
				if v, ok := envMap["AUTH0_MGMT_CLIENT_ID"]; ok && v != "" {
					auth0Options.MgmtClientID = v
				}
				if v, ok := envMap["AUTH0_MGMT_CLIENT_SECRET"]; ok && v != "" {
					auth0Options.MgmtClientSecret = v
				}
			}
		}
		// Fall back to process env (vault-action's exportEnv: true puts the
		// AUTH0_* secrets here in CI). Without this, auth0Options.Domain stays ""
		// and the AUTH0_ISSUER_URL set into flags.ExtraEnv below ends up empty —
		// Spring's OIDC client then can't resolve the .well-known discovery and
		// Zeebe CrashLoopBackOffs with "Unable to connect to the Identity
		// Provider endpoint '/'".
		// auth0.resolveOpts already does this fallback, but it operates on the
		// EnsureClients-internal Options copy, not this one.
		if auth0Options.Domain == "" {
			auth0Options.Domain = os.Getenv("AUTH0_DOMAIN")
		}
		if auth0Options.Audience == "" {
			auth0Options.Audience = os.Getenv("AUTH0_AUDIENCE")
		}
		if auth0Options.MgmtToken == "" {
			auth0Options.MgmtToken = os.Getenv("AUTH0_MGMT_TOKEN")
		}
		if auth0Options.MgmtClientID == "" {
			auth0Options.MgmtClientID = os.Getenv("AUTH0_MGMT_CLIENT_ID")
		}
		if auth0Options.MgmtClientSecret == "" {
			auth0Options.MgmtClientSecret = os.Getenv("AUTH0_MGMT_CLIENT_SECRET")
		}

		logging.Logger.Info().
			Str("namespace", namespace).
			Msg("Auth0 entry detected — provisioning per-component clients (Phase 1: API + env vars)")

		// Capture credentials for cleanup BEFORE provisioning so a partial
		// failure inside EnsureClients (e.g. clients 1-3 of 6 created, then
		// network drops) still leaves cleanupEntry able to delete whatever
		// was created — preventing orphaned clients in the Auth0 tenant.
		auth0Opts = &auth0Options

		prov, err := auth0.EnsureClients(ctx, auth0Options)
		if err != nil {
			// Build a result that carries auth0Opts so cleanupEntry tears down
			// the partial provisioning, then invoke the same cleanup/callback
			// path the success branch uses at the bottom of executeEntry.
			result := RunResult{
				Entry:       entry,
				Namespace:   namespace,
				KubeContext: kubeCtx,
				Error:       fmt.Errorf("auth0: provision clients: %w", err),
				Duration:    time.Since(start),
				auth0Opts:   auth0Opts,
			}
			if opts.Cleanup {
				if opts.OnPhaseChange != nil {
					opts.OnPhaseChange(entry, "cleanup")
				}
				cleanupEntry(ctx, result, opts)
			}
			if opts.OnEntryComplete != nil {
				opts.OnEntryComplete(entry, result)
			}
			return result
		}

		// Inject AUTH0_* env vars per-entry so buildScenarioEnv merges them
		// into the isolated env map for values.Process(). Avoids os.Setenv
		// races across parallel entries that each have distinct client_ids.
		if flags.ExtraEnv == nil {
			flags.ExtraEnv = make(map[string]string)
		}
		audience := auth0Options.Audience
		if audience == "" {
			audience = auth0.DefaultAudience
		}
		flags.ExtraEnv["AUTH0_AUDIENCE"] = audience
		// Issuer URL has NO trailing slash. The values file appends explicit
		// `/` for the canonical iss-claim form on issuer-only fields and
		// `/<path>` for derived URLs, avoiding `//` in rendered output.
		flags.ExtraEnv["AUTH0_ISSUER_URL"] = strings.TrimSuffix(auth0Options.Domain, "/")
		// Initial admin email defaults to the test user. Override via AUTH0_INITIAL_ADMIN_EMAIL.
		if v := os.Getenv("AUTH0_INITIAL_ADMIN_EMAIL"); v != "" {
			flags.ExtraEnv["AUTH0_INITIAL_ADMIN_EMAIL"] = v
		} else {
			flags.ExtraEnv["AUTH0_INITIAL_ADMIN_EMAIL"] = "demo@camunda.com"
		}

		// Per-component client_ids. Component names are upper-cased and spaces
		// replaced with underscores: "Web Modeler" → "AUTH0_WEB_MODELER_CLIENT_ID".
		envify := func(component string) string {
			s := strings.ToUpper(component)
			s = strings.ReplaceAll(s, " ", "_")
			s = strings.ReplaceAll(s, "-", "_")
			return s
		}
		for _, c := range prov.All() {
			flags.ExtraEnv["AUTH0_"+envify(c.Component)+"_CLIENT_ID"] = c.ClientID
		}

		// Phase 2: write the K8s secret in a PreInstallHook so it lands after
		// namespace creation/reset. The secret carries auth0-info-* keys
		// (issuer URL, audience, per-component client_ids) so the test job
		// (separate GH Actions job that doesn't inherit per-entry env vars)
		// can resolve them via a single kubectl-get-secret.
		issuerForSecret := strings.TrimSuffix(auth0Options.Domain, "/") + "/"
		audienceForSecret := audience
		provForSecret := prov
		secretNameForHook := auth0Options.SecretName
		flags.PreInstallHooks = append(flags.PreInstallHooks, func(hookCtx context.Context) error {
			logging.Logger.Info().
				Str("namespace", namespace).
				Msg("Auth0 Phase 2 — creating client-secret-for-components K8s secret (PreInstallHook)")
			return auth0.CreateK8sSecret(
				hookCtx, kubeCtx, namespace, secretNameForHook,
				provForSecret, nil, issuerForSecret, audienceForSecret,
			)
		})
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
		Bool("vaultBackedSecrets", useVault).
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

	// Per-entry cleanup: delete namespace and Entra app after deployment + tests complete.
	// This runs regardless of success/failure, after diagnostics have been collected.
	if opts.Cleanup {
		if opts.OnPhaseChange != nil {
			opts.OnPhaseChange(entry, "cleanup")
		}
		cleanupEntry(ctx, result, opts)
	}

	if opts.OnEntryComplete != nil {
		opts.OnEntryComplete(entry, result)
	}

	return result
}
