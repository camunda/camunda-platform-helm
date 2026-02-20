package matrix

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"scripts/camunda-core/pkg/kube"
	"scripts/camunda-core/pkg/logging"
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
	// IngressBaseDomain is the base domain for ingress hosts.
	// When set, each entry gets <namespace>.<base-domain> as its hostname.
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

// dryRun logs what would be deployed without executing anything.
func dryRun(entries []Entry, opts RunOptions) []RunResult {
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
			namespace := buildNamespace(opts.NamespacePrefix, entry)
			ingressHost := ""
			if opts.IngressBaseDomain != "" {
				ingressHost = namespace + "." + opts.IngressBaseDomain
			}
			platform := resolvePlatform(opts, entry)
			kubeCtx := resolveKubeContext(opts, platform)
			envFile := resolveEnvFile(opts, entry.Version)
			useVault := resolveUseVaultBackedSecrets(opts, platform)
			logging.Logger.Info().
				Str("version", entry.Version).
				Str("scenario", entry.Scenario).
				Str("shortname", entry.Shortname).
				Str("auth", entry.Auth).
				Str("flow", entry.Flow).
				Str("namespace", namespace).
				Str("platform", platform).
				Str("kubeContext", kubeCtx).
				Str("envFile", envFile).
				Str("ingressHost", ingressHost).
				Str("entryPlatform", entry.Platform).
				Strs("exclude", entry.Exclude).
				Bool("enabled", entry.Enabled).
				Bool("cleanup", opts.Cleanup).
				Bool("vaultBackedSecrets", useVault).
				Int("maxParallel", opts.MaxParallel).
				Msg("[DRY-RUN] Would deploy")
			results = append(results, RunResult{Entry: entry, Namespace: namespace, KubeContext: kubeCtx})
		}
	}
	return results
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
// Pattern: <prefix>-<version-compact>-<shortname> when no platform is set (e.g., matrix-88-eske),
// or <prefix>-<version-compact>-<shortname>-<platform> when a platform is set (e.g., matrix-88-eske-eks).
// The platform suffix prevents namespace collisions for scenarios that deploy to multiple platforms.
func buildNamespace(prefix string, entry Entry) string {
	base := buildBaseNamespace(entry)
	return prefix + "-" + base
}

// buildBaseNamespace constructs the namespace suffix for a matrix entry without the prefix.
// Pattern: <version-compact>-<shortname> when no platform is set (e.g., 88-eske),
// or <version-compact>-<shortname>-<platform> when a platform is set (e.g., 88-eske-eks).
func buildBaseNamespace(entry Entry) string {
	versionCompact := strings.ReplaceAll(entry.Version, ".", "")
	shortname := entry.Shortname
	if shortname == "" {
		shortname = entry.Scenario
	}
	if entry.Platform != "" {
		return fmt.Sprintf("%s-%s-%s", versionCompact, shortname, entry.Platform)
	}
	return fmt.Sprintf("%s-%s", versionCompact, shortname)
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
		// e.g., namespace "matrix-89-eske" + base "ci.distro.ultrawombat.com"
		//     → hostname "matrix-89-eske.ci.distro.ultrawombat.com"
		IngressSubdomain:  ingressSubdomain(opts.IngressBaseDomain, namespace),
		IngressBaseDomain: opts.IngressBaseDomain,
	}

	logging.Logger.Info().
		Str("version", entry.Version).
		Str("scenario", entry.Scenario).
		Str("shortname", entry.Shortname).
		Str("auth", entry.Auth).
		Str("flow", entry.Flow).
		Str("namespace", namespace).
		Str("platform", platform).
		Str("kubeContext", kubeCtx).
		Str("envFile", envFile).
		Str("chartPath", entry.ChartPath).
		Str("ingressHost", flags.ResolveIngressHostname()).
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
