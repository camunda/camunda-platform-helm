package matrix

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/jwalton/gchalk"

	"scripts/camunda-core/pkg/kube"
	"scripts/camunda-core/pkg/logging"
	"scripts/camunda-core/pkg/scenarios"
	"scripts/deploy-camunda/config"
	"scripts/deploy-camunda/deploy"
)

// RunOptions controls matrix execution.
type RunOptions struct {
	// DryRun logs what would be done without executing.
	DryRun bool
	// StopOnFailure stops the run on the first failure.
	// In parallel mode, this cancels in-flight entries and prevents new ones from starting.
	StopOnFailure bool
	// Cleanup deletes all created namespaces after the run completes.
	Cleanup bool
	// KubeContexts maps platform names to Kubernetes contexts, e.g.,
	// {"gke": "gke_my-project_us-east1_cluster", "eks": "arn:aws:eks:..."}
	// When an entry's platform matches a key, that context is used for deployment and cleanup.
	KubeContexts map[string]string
	// KubeContext is a fallback Kubernetes context used when no platform-specific
	// context is configured. If both KubeContexts and KubeContext are set, the
	// platform-specific context takes priority.
	KubeContext string
	// NamespacePrefix is prepended to generated namespaces.
	NamespacePrefix string
	// Platform overrides the platform for all entries.
	Platform string
	// MaxParallel controls how many entries run concurrently.
	// 0 or 1 means sequential execution (default). Values > 1 enable parallel execution
	// with at most MaxParallel entries running simultaneously.
	MaxParallel int
	// TestIT runs integration tests after each deployment.
	TestIT bool
	// TestE2E runs e2e tests after each deployment.
	TestE2E bool
	// TestAll runs both integration and e2e tests after each deployment.
	TestAll bool
	// RepoRoot is the repository root path.
	RepoRoot string
	// EnvFiles maps chart versions to .env file paths, e.g.,
	// {"8.9": ".env.89", "8.8": ".env.88"}
	// When an entry's version matches a key, that .env file is loaded before deployment.
	EnvFiles map[string]string
	// EnvFile is a fallback .env file used when no version-specific file is configured.
	// If both EnvFiles and EnvFile are set, the version-specific file takes priority.
	EnvFile string
	// KeycloakHost is the external Keycloak hostname.
	// Defaults to config.DefaultKeycloakHost when empty.
	KeycloakHost string
	// KeycloakProtocol is the protocol for the external Keycloak (e.g., "https").
	// Defaults to config.DefaultKeycloakProtocol when empty.
	KeycloakProtocol string
	// IngressBaseDomains maps platform names to ingress base domains, e.g.,
	// {"gke": "ci.distro.ultrawombat.com", "eks": "distribution.aws.camunda.cloud"}
	// When an entry's platform matches a key, that domain is used for ingress hostname construction.
	IngressBaseDomains map[string]string
	// IngressBaseDomain is a fallback base domain for ingress hosts used when no
	// platform-specific domain is configured. If both IngressBaseDomains and
	// IngressBaseDomain are set, the platform-specific domain takes priority.
	// Valid values: ci.distro.ultrawombat.com, distribution.aws.camunda.cloud
	IngressBaseDomain string
	// LogLevel controls the log verbosity for each entry's deployment.
	// Valid values: debug, info, warn, error. Defaults to "info" if empty.
	LogLevel string
	// SkipDependencyUpdate skips running "helm dependency update" before deploying.
	// Default is false, meaning dependency update runs for every entry.
	SkipDependencyUpdate bool
	// VaultBackedSecrets maps platform names to whether vault-backed secrets should be used, e.g.,
	// {"eks": true, "gke": false}
	// When an entry's platform matches a key, the corresponding value controls whether
	// the vault-backend ClusterSecretStore and -vault.yaml manifest variants are selected.
	VaultBackedSecrets map[string]bool
	// UseVaultBackedSecrets is a fallback for platforms not in VaultBackedSecrets.
	// If both VaultBackedSecrets and UseVaultBackedSecrets are set, the platform-specific
	// value takes priority.
	UseVaultBackedSecrets bool
}

// RunResult holds the result of a single matrix entry execution.
type RunResult struct {
	Entry       Entry
	Namespace   string
	KubeContext string
	Error       error
}

// Run executes the matrix entries, building RuntimeFlags for each and calling deploy.Execute().
// When MaxParallel <= 1, entries are processed sequentially. When MaxParallel > 1, up to
// MaxParallel entries run concurrently. If Cleanup is enabled, all created namespaces are
// deleted after the run completes (regardless of success or failure).
func Run(ctx context.Context, entries []Entry, opts RunOptions) ([]RunResult, error) {
	if len(entries) == 0 {
		return nil, fmt.Errorf("no matrix entries to run")
	}

	// Dry-run is always sequential
	if opts.DryRun {
		return dryRun(entries, opts), nil
	}

	parallel := opts.MaxParallel > 1
	if parallel {
		logging.Logger.Info().
			Int("maxParallel", opts.MaxParallel).
			Int("totalEntries", len(entries)).
			Msg("Starting parallel matrix run")
	}

	var results []RunResult
	var retErr error

	if parallel {
		results, retErr = runParallel(ctx, entries, opts)
	} else {
		results, retErr = runSequential(ctx, entries, opts)
	}

	// Cleanup phase: delete all namespaces that were created
	if opts.Cleanup && !opts.DryRun {
		cleanupNamespaces(ctx, results, opts)
	}

	return results, retErr
}

// dryRunEntry holds resolved details for one matrix entry in dry-run mode.
type dryRunEntry struct {
	entry       Entry
	namespace   string
	kubeCtx     string
	platform    string
	infraType   string
	ingressHost string
	envFile     string
	useVault    bool
	// Resolved layer config (derived from scenario name + explicit overrides).
	identity    string
	persistence string
	features    []string
	layerFiles  []string // short relative paths, e.g., "values/identity/keycloak.yaml"
}

// dryRun resolves what would be deployed and prints a clean summary to stdout.
func dryRun(entries []Entry, opts RunOptions) []RunResult {
	var results []RunResult
	versions := VersionOrder(entries)
	groups := GroupByVersion(entries)

	// Resolve all entries first.
	var resolved []dryRunEntry
	for _, version := range versions {
		for _, entry := range groups[version] {
			namespace := buildNamespace(opts.NamespacePrefix, entry)
			platform := resolvePlatform(opts, entry)
			kubeCtx := resolveKubeContext(opts, platform)
			envFile := resolveEnvFile(opts, entry.Version)
			useVault := resolveUseVaultBackedSecrets(opts, platform)
			baseDomain := resolveIngressBaseDomain(opts, platform)
			ingressHost := ""
			if baseDomain != "" {
				ingressHost = namespace + "." + baseDomain
			}

			// Resolve deployment layers (same logic as deploy.go prepareScenarioValues).
			scenarioDir := filepath.Join(entry.ChartPath, "test/integration/scenarios/chart-full-setup")
			deployConfig := scenarios.MapScenarioToConfig(entry.Scenario)
			deployConfig.Platform = platform
			deployConfig.Flow = entry.Flow
			if entry.Identity != "" {
				deployConfig.Identity = entry.Identity
			}
			if entry.Persistence != "" {
				deployConfig.Persistence = entry.Persistence
			}
			if len(entry.Features) > 0 {
				deployConfig.Features = entry.Features
			}

			var layerFiles []string
			if paths, err := deployConfig.ResolvePaths(scenarioDir); err == nil {
				for _, p := range paths {
					if rel, relErr := filepath.Rel(scenarioDir, p); relErr == nil {
						layerFiles = append(layerFiles, rel)
					} else {
						layerFiles = append(layerFiles, filepath.Base(p))
					}
				}
			}

			resolved = append(resolved, dryRunEntry{
				entry:       entry,
				namespace:   namespace,
				kubeCtx:     kubeCtx,
				platform:    platform,
				infraType:   entry.InfraType,
				ingressHost: ingressHost,
				envFile:     envFile,
				useVault:    useVault,
				identity:    deployConfig.Identity,
				persistence: deployConfig.Persistence,
				features:    deployConfig.Features,
				layerFiles:  layerFiles,
			})
			results = append(results, RunResult{Entry: entry, Namespace: namespace, KubeContext: kubeCtx})
		}
	}

	// Print clean dry-run output.
	fmt.Fprintln(os.Stdout, formatDryRunOutput(resolved, versions, opts))
	return results
}

// Style helpers for dry-run output. These wrap logging.Emphasize so colors
// are automatically disabled in CI/non-TTY environments.
var (
	dryHead = func(s string) string { return logging.Emphasize(s, gchalk.Bold) }
	dryKey  = func(s string) string { return logging.Emphasize(s, gchalk.Cyan) }
	dryVal  = func(s string) string { return logging.Emphasize(s, gchalk.Magenta) }
	dryOk   = func(s string) string { return logging.Emphasize(s, gchalk.Green) }
	dryWarn = func(s string) string { return logging.Emphasize(s, gchalk.Yellow) }
	dryDim  = func(s string) string { return logging.Emphasize(s, gchalk.WithBrightBlack().Italic) }
)

// formatDryRunOutput produces a human-readable dry-run summary grouped by version.
func formatDryRunOutput(entries []dryRunEntry, versions []string, opts RunOptions) string {
	var b strings.Builder

	// Group by version.
	groups := make(map[string][]dryRunEntry)
	for _, e := range entries {
		groups[e.entry.Version] = append(groups[e.entry.Version], e)
	}

	for i, version := range versions {
		versionEntries := groups[version]
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "%s\n",
			dryHead(fmt.Sprintf("=== Version %s (%d entries) ===", version, len(versionEntries))))

		for j, e := range versionEntries {
			b.WriteString("\n")

			// Header line: number, scenario, shortname, flow, platform, infra-type, auth.
			scenarioLabel := dryKey(e.entry.Scenario)
			if e.entry.Shortname != "" {
				scenarioLabel += " " + dryDim("("+e.entry.Shortname+")")
			}
			fmt.Fprintf(&b, "  %s %s | %s | %s (%s) | %s\n",
				dryHead(fmt.Sprintf("[%d]", j+1)),
				scenarioLabel,
				dryOk(e.entry.Flow),
				dryOk(e.platform),
				dryOk(e.infraType),
				dryOk(e.entry.Auth))

			// Layers — the most important info.
			features := dryDim("-")
			if len(e.features) > 0 {
				features = dryWarn(strings.Join(e.features, ", "))
			}
			fmt.Fprintf(&b, "      %s %s + %s + %s  %s %s\n",
				dryKey("layers:"),
				dryVal(e.identity), dryVal(e.persistence), dryVal(e.platform),
				dryKey("features:"), features)

			// Namespace.
			fmt.Fprintf(&b, "      %s %s\n", dryKey("namespace:"), e.namespace)

			// Optional fields — only shown when set.
			if e.kubeCtx != "" {
				fmt.Fprintf(&b, "      %s   %s\n", dryKey("context:"), e.kubeCtx)
			}
			if e.ingressHost != "" {
				fmt.Fprintf(&b, "      %s   %s\n", dryKey("ingress:"), e.ingressHost)
			}
			if e.envFile != "" {
				fmt.Fprintf(&b, "      %s   %s\n", dryKey("envFile:"), e.envFile)
			}
			if e.useVault {
				fmt.Fprintf(&b, "      %s     %s\n", dryKey("vault:"), dryWarn("true"))
			}
			if len(e.entry.Exclude) > 0 {
				fmt.Fprintf(&b, "      %s   %s\n", dryKey("exclude:"), dryWarn(strings.Join(e.entry.Exclude, ", ")))
			}

			// Resolved values files.
			if len(e.layerFiles) > 0 {
				fmt.Fprintf(&b, "      %s\n", dryKey("files:"))
				for _, f := range e.layerFiles {
					fmt.Fprintf(&b, "        %s %s\n", dryDim("-"), f)
				}
			}
		}
	}

	// Footer.
	fmt.Fprintf(&b, "\n%s\n",
		dryHead(fmt.Sprintf("--- %d entries across %d versions (dry-run, nothing deployed) ---", len(entries), len(versions))))

	return b.String()
}

// runSequential processes all entries one at a time.
func runSequential(ctx context.Context, entries []Entry, opts RunOptions) ([]RunResult, error) {
	var results []RunResult
	versions := VersionOrder(entries)
	groups := GroupByVersion(entries)

	for _, version := range versions {
		versionEntries := groups[version]

		logging.Logger.Info().
			Str("version", version).
			Int("entries", len(versionEntries)).
			Msg("Processing version")

		for _, entry := range versionEntries {
			result := executeEntry(ctx, entry, opts)
			results = append(results, result)

			if result.Error != nil {
				logging.Logger.Error().
					Err(result.Error).
					Str("version", entry.Version).
					Str("scenario", entry.Scenario).
					Str("flow", entry.Flow).
					Msg("Matrix entry failed")

				if opts.StopOnFailure {
					return results, fmt.Errorf("stopping on failure: %w", result.Error)
				}
			} else {
				logging.Logger.Info().
					Str("version", entry.Version).
					Str("scenario", entry.Scenario).
					Str("flow", entry.Flow).
					Msg("Matrix entry completed successfully")
			}
		}
	}

	return results, nil
}

// runParallel processes entries concurrently with a bounded semaphore.
// Results are collected in entry order. If StopOnFailure is set, the context
// is cancelled on the first failure, which prevents new entries from starting
// and signals in-flight deploy.Execute() calls to abort.
func runParallel(ctx context.Context, entries []Entry, opts RunOptions) ([]RunResult, error) {
	// Pre-allocate results slice so each goroutine writes to its own index (no mutex needed for slots).
	results := make([]RunResult, len(entries))

	// Use a cancellable context for stop-on-failure.
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Semaphore to limit concurrency.
	sem := make(chan struct{}, opts.MaxParallel)

	var wg sync.WaitGroup

	// Track first failure for stop-on-failure.
	var (
		firstErr error
		errOnce  sync.Once
	)

	for i, entry := range entries {
		// Check if context is already cancelled (stop-on-failure triggered).
		if runCtx.Err() != nil {
			break
		}

		wg.Add(1)
		sem <- struct{}{} // Acquire semaphore slot.

		go func(idx int, e Entry) {
			defer wg.Done()
			defer func() { <-sem }() // Release semaphore slot.

			// Check again after acquiring semaphore slot.
			if runCtx.Err() != nil {
				results[idx] = RunResult{
					Entry:     e,
					Namespace: buildNamespace(opts.NamespacePrefix, e),
					Error:     fmt.Errorf("skipped: run cancelled"),
				}
				return
			}

			result := executeEntry(runCtx, e, opts)
			results[idx] = result

			if result.Error != nil {
				logging.Logger.Error().
					Err(result.Error).
					Str("version", e.Version).
					Str("scenario", e.Scenario).
					Str("flow", e.Flow).
					Msg("Matrix entry failed")

				if opts.StopOnFailure {
					errOnce.Do(func() {
						firstErr = result.Error
						cancel()
					})
				}
			} else {
				logging.Logger.Info().
					Str("version", e.Version).
					Str("scenario", e.Scenario).
					Str("flow", e.Flow).
					Msg("Matrix entry completed successfully")
			}
		}(i, entry)
	}

	wg.Wait()

	// Trim any trailing zero-value results from entries that were never dispatched
	// (can happen if stop-on-failure breaks the loop before all entries are enqueued).
	var trimmed []RunResult
	for _, r := range results {
		if r.Namespace != "" || r.Error != nil {
			trimmed = append(trimmed, r)
		}
	}

	if firstErr != nil {
		return trimmed, fmt.Errorf("stopping on failure: %w", firstErr)
	}
	return trimmed, nil
}

// buildNamespace constructs the namespace for a matrix entry.
// Pattern: <prefix>-<version-compact>-<shortname>-<flow>[-<platform>]
// e.g., matrix-88-eske-inst-gke, matrix-87-es-upgp-eks.
// The flow suffix prevents namespace collisions when a scenario has multiple flows
// (e.g., install + upgrade-patch).
func buildNamespace(prefix string, entry Entry) string {
	base := buildBaseNamespace(entry)
	return prefix + "-" + base
}

// buildBaseNamespace constructs the namespace suffix for a matrix entry without the prefix.
// Pattern: <version-compact>-<shortname>-<flow>[-<platform>]
// e.g., 88-eske-inst, 87-es-upgp-gke.
func buildBaseNamespace(entry Entry) string {
	versionCompact := strings.ReplaceAll(entry.Version, ".", "")
	shortname := entry.Shortname
	if shortname == "" {
		shortname = entry.Scenario
	}
	flow := flowAbbrev(entry.Flow)
	if entry.Platform != "" {
		return fmt.Sprintf("%s-%s-%s-%s", versionCompact, shortname, flow, entry.Platform)
	}
	return fmt.Sprintf("%s-%s-%s", versionCompact, shortname, flow)
}

// flowAbbrev returns a short abbreviation for a flow name, used in namespace construction.
var flowAbbrevMap = map[string]string{
	"install":       "inst",
	"upgrade-patch": "upgp",
	"upgrade-minor": "upgm",
}

func flowAbbrev(flow string) string {
	if abbrev, ok := flowAbbrevMap[flow]; ok {
		return abbrev
	}
	if flow == "" {
		return "inst"
	}
	// Unknown flow: use first 4 chars as fallback.
	if len(flow) > 4 {
		return flow[:4]
	}
	return flow
}

// ingressSubdomain returns the namespace as the ingress subdomain when a base
// domain is configured, or empty string when ingress is not configured.
func ingressSubdomain(baseDomain, namespace string) string {
	if baseDomain == "" {
		return ""
	}
	return namespace
}

// resolveKubeContext returns the Kubernetes context for a given platform.
// It checks KubeContexts (platform-specific map) first, then falls back to KubeContext.
func resolveKubeContext(opts RunOptions, platform string) string {
	if ctx, ok := opts.KubeContexts[platform]; ok && ctx != "" {
		return ctx
	}
	return opts.KubeContext
}

// resolveEnvFile returns the .env file path for a matrix entry's version.
// It checks EnvFiles (version-specific map) first, then falls back to EnvFile.
func resolveEnvFile(opts RunOptions, version string) string {
	if f, ok := opts.EnvFiles[version]; ok && f != "" {
		return f
	}
	return opts.EnvFile
}

// resolvePlatform determines the effective platform for a matrix entry.
func resolvePlatform(opts RunOptions, entry Entry) string {
	if opts.Platform != "" {
		return opts.Platform
	}
	if entry.Platform != "" {
		return entry.Platform
	}
	return "gke"
}

// resolveUseVaultBackedSecrets returns whether vault-backed secrets should be used for a given platform.
// It checks VaultBackedSecrets (platform-specific map) first, then falls back to UseVaultBackedSecrets.
func resolveUseVaultBackedSecrets(opts RunOptions, platform string) bool {
	if v, ok := opts.VaultBackedSecrets[platform]; ok {
		return v
	}
	return opts.UseVaultBackedSecrets
}

// resolveIngressBaseDomain returns the ingress base domain for a given platform.
// It checks IngressBaseDomains (platform-specific map) first, then falls back to IngressBaseDomain.
func resolveIngressBaseDomain(opts RunOptions, platform string) string {
	if d, ok := opts.IngressBaseDomains[platform]; ok && d != "" {
		return d
	}
	return opts.IngressBaseDomain
}

// executeEntry deploys a single matrix entry by constructing RuntimeFlags and calling deploy.Execute().
func executeEntry(ctx context.Context, entry Entry, opts RunOptions) RunResult {
	namespace := buildNamespace(opts.NamespacePrefix, entry)
	baseNamespace := buildBaseNamespace(entry)

	// Determine platform and kube context
	platform := resolvePlatform(opts, entry)
	kubeCtx := resolveKubeContext(opts, platform)
	envFile := resolveEnvFile(opts, entry.Version)
	useVault := resolveUseVaultBackedSecrets(opts, platform)

	// Compute the scenario directory. deploy.Execute uses this to resolve
	// values files — both layered and legacy formats are handled there.
	scenarioDir := filepath.Join(entry.ChartPath, "test/integration/scenarios/chart-full-setup")

	// Build the test exclude string from entry excludes (goroutine-safe via RuntimeFlags,
	// avoids using os.Setenv which is process-global and unsafe for concurrent execution).
	testExclude := ""
	if len(entry.Exclude) > 0 {
		testExclude = strings.Join(entry.Exclude, ",")
	}

	// Default log level to "info" if not set.
	logLevel := opts.LogLevel
	if logLevel == "" {
		logLevel = "info"
	}

	// Default Keycloak host/protocol if not set.
	keycloakHost := config.FirstNonEmpty(opts.KeycloakHost, config.DefaultKeycloakHost)
	keycloakProtocol := config.FirstNonEmpty(opts.KeycloakProtocol, config.DefaultKeycloakProtocol)

	flags := &config.RuntimeFlags{
		ChartPath:             entry.ChartPath,
		ScenarioPath:          scenarioDir,
		Namespace:             baseNamespace,
		NamespacePrefix:       opts.NamespacePrefix,
		Release:               "integration",
		Scenario:              entry.Scenario,
		Scenarios:             []string{entry.Scenario},
		Auth:                  entry.Auth,
		Platform:              platform,
		Flow:                  entry.Flow,
		LogLevel:              logLevel,
		SkipDependencyUpdate:  opts.SkipDependencyUpdate,
		ExternalSecrets:       true,
		Interactive:           false,
		AutoGenerateSecrets:   true,
		UseVaultBackedSecrets: useVault,
		KeycloakHost:          keycloakHost,
		KeycloakProtocol:      keycloakProtocol,
		RepoRoot:              opts.RepoRoot,
		KubeContext:           kubeCtx,
		EnvFile:               envFile,
		TestExclude:           testExclude,
		RunIntegrationTests:   opts.TestIT || opts.TestAll,
		RunE2ETests:           opts.TestE2E || opts.TestAll,
		RunAllTests:           opts.TestAll,
		// Ingress: use the namespace as subdomain so each entry gets a unique hostname.
		// e.g., namespace "matrix-89-eske-inst" + base "ci.distro.ultrawombat.com"
		//     → hostname "matrix-89-eske-inst.ci.distro.ultrawombat.com"
		// The base domain is resolved per-platform (GKE/EKS may have different domains).
		IngressSubdomain:  ingressSubdomain(resolveIngressBaseDomain(opts, platform), namespace),
		IngressBaseDomain: resolveIngressBaseDomain(opts, platform),
		// Selection + Composition: pass explicit layer overrides from ci-test-config.yaml.
		// When set, these override MapScenarioToConfig name-based derivation in deploy.go.
		Identity:    entry.Identity,
		Persistence: entry.Persistence,
		Features:    entry.Features,
		InfraType:   entry.InfraType,
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

	if err := deploy.Execute(ctx, flags); err != nil {
		return RunResult{Entry: entry, Namespace: namespace, KubeContext: kubeCtx, Error: err}
	}

	return RunResult{Entry: entry, Namespace: namespace, KubeContext: kubeCtx}
}

// cleanupNamespaces deletes all namespaces that were created during the matrix run.
// It deduplicates namespaces (in case multiple flows share the same namespace)
// and logs each deletion. Each namespace is deleted using the kube context that
// was resolved for it during execution. Errors are logged but do not halt the cleanup.
func cleanupNamespaces(ctx context.Context, results []RunResult, opts RunOptions) {
	// Deduplicate namespaces while preserving order and tracking kube context
	type nsInfo struct {
		namespace   string
		kubeContext string
	}
	seen := make(map[string]bool)
	var namespaces []nsInfo
	for _, r := range results {
		if r.Namespace != "" && !seen[r.Namespace] {
			seen[r.Namespace] = true
			namespaces = append(namespaces, nsInfo{namespace: r.Namespace, kubeContext: r.KubeContext})
		}
	}

	if len(namespaces) == 0 {
		return
	}

	logging.Logger.Info().
		Int("count", len(namespaces)).
		Msg("Cleaning up namespaces")

	for _, ns := range namespaces {
		logging.Logger.Info().
			Str("namespace", ns.namespace).
			Str("kubeContext", ns.kubeContext).
			Msg("Deleting namespace")

		if err := kube.DeleteNamespace(ctx, "", ns.kubeContext, ns.namespace); err != nil {
			logging.Logger.Error().
				Err(err).
				Str("namespace", ns.namespace).
				Msg("Failed to delete namespace during cleanup")
		} else {
			logging.Logger.Info().
				Str("namespace", ns.namespace).
				Msg("Namespace deleted successfully")
		}
	}
}

// PrintRunSummary outputs a summary of all run results.
func PrintRunSummary(results []RunResult) string {
	if len(results) == 0 {
		return "No entries executed."
	}

	var b strings.Builder
	successCount := 0
	failCount := 0

	for _, r := range results {
		if r.Error == nil {
			successCount++
		} else {
			failCount++
		}
	}

	fmt.Fprintf(&b, "\n=== Matrix Run Summary ===\n")
	fmt.Fprintf(&b, "Total:   %d\n", len(results))
	fmt.Fprintf(&b, "Success: %d\n", successCount)
	fmt.Fprintf(&b, "Failed:  %d\n", failCount)

	if failCount > 0 {
		fmt.Fprintf(&b, "\nFailed entries:\n")
		for _, r := range results {
			if r.Error != nil {
				fmt.Fprintf(&b, "  - %s/%s (%s, flow=%s): %v\n",
					r.Entry.Version, r.Entry.Scenario, r.Entry.Shortname, r.Entry.Flow, r.Error)
			}
		}
	}

	return b.String()
}
