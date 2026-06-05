package matrix

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jwalton/gchalk"

	"scripts/camunda-core/pkg/docker"
	"scripts/camunda-core/pkg/kube"
	"scripts/camunda-core/pkg/logging"
	"scripts/camunda-core/pkg/scenarios"
	"scripts/camunda-core/pkg/versionmatrix"
	"scripts/camunda-deployer/pkg/deployer"
	"scripts/deploy-camunda/auth0"
	"scripts/deploy-camunda/config"
	"scripts/deploy-camunda/deploy"
	"scripts/deploy-camunda/entra"
	"scripts/prepare-helm-values/pkg/env"
)

// applyChartRefOverride mutates chart to point at opts.ChartRef (an OCI reference
// or a local .tgz path) when set, and forces SkipDependencyUpdate so helm does
// not try to run `dependency update` against an external/packaged chart.
// ChartPath is left untouched so values-file resolution (scenario directory,
// chart-root overlays) still uses the local repo. It returns true when the
// override was applied. Centralizing this logic here lets executeEntry and
// tests share a single code path.
func applyChartRefOverride(chart *config.ChartFlags, opts RunOptions) bool {
	if opts.ChartRef == "" {
		return false
	}
	chart.Chart = opts.ChartRef
	chart.ChartVersion = opts.ChartRefVersion
	chart.SkipDependencyUpdate = true
	logging.Logger.Info().
		Str("chartRef", opts.ChartRef).
		Str("chartVersion", opts.ChartRefVersion).
		Msg("Using external chart reference (OCI/tgz) instead of local chart directory")
	return true
}

func ociImmutabilityMode(opts RunOptions) bool {
	return opts.ChartRef != "" && !opts.ForceImageOverrides
}

func effectiveImageTags(entry Entry, opts RunOptions) bool {
	if ociImmutabilityMode(opts) {
		return false
	}
	return entry.ImageTags
}

func resolveChartRootOverlays(entry Entry, opts RunOptions) []string {
	if ociImmutabilityMode(opts) {
		// OCI artifacts bake all image versions; skip overlays that would
		// override them (includes enterprise sub-chart image pins).
		return nil
	}

	var overlays []string
	if entry.Enterprise {
		overlays = append(overlays, "enterprise")
	}
	if !effectiveImageTags(entry, opts) {
		if opts.UseLatest {
			overlays = append(overlays, "latest")
		} else {
			overlays = append(overlays, "digest")
		}
	}
	return overlays
}

func sanitizeEnvFileForOCIImmutability(envFile string, opts RunOptions) (string, func(), error) {
	cleanup := func() {}
	if !ociImmutabilityMode(opts) || envFile == "" {
		return envFile, cleanup, nil
	}

	values, err := env.ReadFile(envFile)
	if err != nil {
		return "", cleanup, fmt.Errorf("read env file for OCI immutability guard: %w", err)
	}

	filtered := make(map[string]string, len(values))
	removed := make([]string, 0)
	for key, value := range values {
		if strings.HasSuffix(key, "_IMAGE_TAG") {
			removed = append(removed, key)
			continue
		}
		filtered[key] = value
	}
	if len(removed) == 0 {
		return envFile, cleanup, nil
	}

	sort.Strings(removed)
	logging.Logger.Warn().
		Str("chartRef", opts.ChartRef).
		Strs("removedKeys", removed).
		Msg("OCI immutability mode: removing image tag env overrides")

	if len(filtered) == 0 {
		return "", cleanup, nil
	}

	file, err := os.CreateTemp("", "deploy-camunda-oci-env-*.env")
	if err != nil {
		return "", cleanup, fmt.Errorf("create sanitized env file for OCI immutability guard: %w", err)
	}
	defer file.Close()

	keys := make([]string, 0, len(filtered))
	for key := range filtered {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if _, err := fmt.Fprintf(file, "%s=%s\n", key, strconv.Quote(filtered[key])); err != nil {
			_ = os.Remove(file.Name())
			return "", cleanup, fmt.Errorf("write sanitized env file for OCI immutability guard: %w", err)
		}
	}

	return file.Name(), func() { _ = os.Remove(file.Name()) }, nil
}

// RunResult holds the result of a single matrix entry execution.
type RunResult struct {
	Entry       Entry
	Namespace   string
	KubeContext string
	Error       error
	Duration    time.Duration // Wall-clock time for this entry's execution.
	Diagnostics string        // Post-failure diagnostics run directory path

	// venomOpts stores the Entra options used to provision a venom app for OIDC entries.
	// Populated only when the entry uses OIDC authentication. Used during cleanup to
	// delete the corresponding Entra app registration.
	venomOpts *entra.Options

	// auth0Opts stores the Auth0 options used to provision per-component clients
	// for Auth0 entries. Populated only when entry.Identity == "auth0". Used
	// during cleanup to delete the corresponding Auth0 clients.
	auth0Opts *auth0.Options
}

// Run executes the matrix entries, building RuntimeFlags for each and calling deploy.Execute().
// When MaxParallel <= 1, entries are processed sequentially. When MaxParallel > 1, up to
// MaxParallel entries run concurrently. If Cleanup is enabled, each entry's namespace is
// deleted immediately after that entry's deployment and tests complete (regardless of
// success or failure). This frees cluster resources as early as possible rather than
// waiting for the entire matrix run to finish.
func Run(ctx context.Context, entries []Entry, opts RunOptions) ([]RunResult, error) {
	if len(entries) == 0 {
		return nil, fmt.Errorf("no matrix entries to run")
	}

	// Dry-run is always sequential
	if opts.DryRun {
		return dryRun(entries, opts), nil
	}

	// Coverage mode: resolve and display layer breakdown, no deployment
	if opts.Coverage {
		return coverageReport(entries, opts), nil
	}

	// Perform docker login ONCE before dispatching entries. Running `docker login`
	// concurrently causes keychain conflicts on macOS ("item already exists" -25299).
	// After this, each entry's deployer runs with SkipDockerLogin=true so it only
	// creates the per-namespace K8s pull secrets without touching `docker login`.
	if opts.EnsureDockerHub {
		if err := docker.EnsureDockerHubLogin(ctx, opts.DockerHubUsername, opts.DockerHubPassword); err != nil {
			return nil, fmt.Errorf("failed to ensure Docker Hub login: %w", err)
		}
	}
	if opts.EnsureDockerRegistry {
		if err := docker.EnsureHarborLogin(ctx, opts.DockerUsername, opts.DockerPassword); err != nil {
			return nil, fmt.Errorf("failed to ensure Harbor login: %w", err)
		}
	}

	// Warm up each unique kube context ONCE before dispatching entries.
	// For Teleport-managed clusters (EKS), the first API call may trigger
	// an interactive browser login. Doing this sequentially ensures only one
	// login prompt per context, rather than N parallel goroutines racing.
	if err := warmUpKubeContexts(ctx, entries, opts); err != nil {
		return nil, err
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

	// If no early-termination error was returned (StopOnFailure) but entries
	// still failed, synthesize a summary error so callers (and CI steps) get a
	// non-zero exit code. Also catch the edge case where the context was
	// cancelled before any entry was dispatched (runParallel returns nil, nil).
	if retErr == nil {
		retErr = synthesizeRunError(ctx, results, len(entries))
	}

	return results, retErr
}

// synthesizeRunError checks completed results for failures and returns a
// summary error when any entries failed. It also detects context cancellation
// that prevented entries from being dispatched (e.g., parent ctx already done
// when runParallel starts). This is an unexported helper so tests can exercise
// the exact production logic.
func synthesizeRunError(ctx context.Context, results []RunResult, totalEntries int) error {
	// If the context was cancelled and fewer results were produced than entries
	// expected, report the cancellation — this catches the edge case where
	// runParallel breaks out of its dispatch loop before any entry starts.
	if ctx.Err() != nil && len(results) < totalEntries {
		return fmt.Errorf("run cancelled: %d of %d entries never started: %w",
			totalEntries-len(results), totalEntries, ctx.Err())
	}

	var failCount int
	for _, r := range results {
		if r.Error != nil {
			failCount++
		}
	}
	if failCount > 0 {
		return fmt.Errorf("%d of %d matrix entries failed", failCount, len(results))
	}
	return nil
}

// dryRunEntry holds resolved details for one matrix entry in dry-run mode.
type dryRunEntry struct {
	entry                Entry
	namespace            string
	kubeCtx              string
	platform             string
	infraType            string
	ingressHost          string
	envFile              string
	useVault             bool
	deleteNS             bool
	ensureDockerRegistry bool
	ensureDockerHub      bool
	// Resolved layer config (derived from scenario name + explicit overrides).
	identity    string
	persistence string
	features    []string
	layerFiles  []string // short relative paths, e.g., "values/identity/keycloak.yaml"
	// Upgrade flow fields (populated only for upgrade flows).
	upgradeFromVersion string   // The "from" chart version for upgrade flows (e.g., "13.5.0").
	preUpgradeScript   string   // Path to the pre-upgrade script (e.g., "charts/.../pre-upgrade-patch.sh"), or empty.
	upgradeOnly        bool     // True for modular-upgrade-minor (single-step upgrade, no install).
	step1ValuesFrom    string   // For upgrade-minor Step 1: the previous version whose values are used (e.g., "8.7"), or empty.
	chartRootOverlays  []string // Chart-root overlay files that will be applied (e.g., ["enterprise", "digest"]).
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
			namespace := resolveNamespace(opts, entry)
			platform := resolvePlatform(opts, entry)
			kubeCtx := resolveKubeContext(opts, platform)
			envFile := resolveEnvFile(opts, entry.Version)
			useVault := resolveUseVaultBackedSecrets(opts, platform)
			baseDomain := resolveIngressBaseDomain(opts, platform)
			ingressHost := ""
			if baseDomain != "" {
				ingressHost = namespace + "." + baseDomain
			}

			// Resolve deployment layers via the canonical builder (same logic as deploy.go prepareScenarioValues).
			scenarioDir := filepath.Join(entry.ChartPath, "test/integration/scenarios/chart-full-setup")
			deployConfig, buildErr := scenarios.BuildDeploymentConfig(entry.Scenario, scenarioDir, scenarios.BuilderOverrides{
				Identity:    entry.Identity,
				Persistence: entry.Persistence,
				Platform:    platform,
				Features:    entry.Features,
				InfraType:   entry.InfraType,
				Flow:        entry.Flow,
				QA:          entry.QA || opts.UseQA,
				ImageTags:   effectiveImageTags(entry, opts),
				Upgrade:     entry.Upgrade,
			})
			if buildErr != nil {
				results = append(results, RunResult{
					Entry: entry,
					Error: fmt.Errorf("deployment config validation failed: %w", buildErr),
				})
				continue
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
				entry:                entry,
				namespace:            namespace,
				kubeCtx:              kubeCtx,
				platform:             platform,
				infraType:            entry.InfraType,
				ingressHost:          ingressHost,
				envFile:              envFile,
				useVault:             useVault,
				deleteNS:             opts.DeleteNamespaceFirst,
				ensureDockerRegistry: opts.EnsureDockerRegistry,
				ensureDockerHub:      opts.EnsureDockerHub,
				identity:             deployConfig.Identity,
				persistence:          deployConfig.Persistence,
				features:             deployConfig.Features,
				layerFiles:           layerFiles,
				upgradeFromVersion:   resolveUpgradeFromVersionQuiet(opts.RepoRoot, entry, opts.UpgradeFromVersion),
				preUpgradeScript:     resolvePreUpgradeScriptQuiet(opts.RepoRoot, entry),
				upgradeOnly:          versionmatrix.IsUpgradeOnlyFlow(entry.Flow),
				step1ValuesFrom:      resolveStep1ValuesFromQuiet(entry),
				chartRootOverlays:    resolveChartRootOverlaysQuiet(entry.ChartPath, entry, opts),
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
	dryFail = func(s string) string { return logging.Emphasize(s, gchalk.Red) }
	dryDim  = func(s string) string { return logging.Emphasize(s, gchalk.WithBrightBlack().Italic) }
)

// resolveUpgradeFromVersionQuiet resolves the "from" chart version for upgrade flows.
// Returns empty string for non-upgrade flows or on error (dry-run is best-effort).
// If overrideVersion is non-empty, it is returned directly for upgrade flows.
func resolveUpgradeFromVersionQuiet(repoRoot string, entry Entry, overrideVersion string) string {
	if !versionmatrix.IsUpgradeFlow(entry.Flow) {
		return ""
	}
	if overrideVersion != "" {
		return overrideVersion
	}
	version, err := versionmatrix.ResolveUpgradeFromVersion(repoRoot, entry.Version, entry.Flow)
	if err != nil {
		logging.Logger.Warn().Err(err).Str("version", entry.Version).Str("flow", entry.Flow).
			Msg("dry-run: could not resolve upgrade-from version")
		return "???"
	}
	return version
}

// resolvePreUpgradeScriptQuiet returns the pre-upgrade script path declared
// on the entry's PreUpgrade hook (if any). Used by the dry-run summary;
// returns empty string for non-upgrade flows, fixture-mode hooks, or scripts
// that do not exist on disk for the entry's version.
func resolvePreUpgradeScriptQuiet(repoRoot string, entry Entry) string {
	if !versionmatrix.IsUpgradeFlow(entry.Flow) {
		return ""
	}
	if entry.PreUpgrade == nil || entry.PreUpgrade.Script == "" {
		return ""
	}
	if !versionmatrix.HasPreSetupScript(repoRoot, entry.Version, entry.PreUpgrade.Script) {
		return ""
	}
	return versionmatrix.PreSetupScriptPath(repoRoot, entry.Version, entry.PreUpgrade.Script)
}

// resolveStep1ValuesFromQuiet returns the previous app version whose values files are used
// for Step 1 of upgrade-minor flows. For upgrade-minor, Step 1 uses the previous app
// version's chart directory (e.g., "8.7" values for an "8.8" entry). For all other flows
// (including upgrade-patch, which uses the current version's values), returns empty string.
// Errors are silently logged — dry-run is best-effort.
func resolveStep1ValuesFromQuiet(entry Entry) string {
	if entry.Flow != "upgrade-minor" {
		return ""
	}
	prev, err := versionmatrix.PreviousAppVersion(entry.Version)
	if err != nil {
		logging.Logger.Warn().Err(err).Str("version", entry.Version).
			Msg("dry-run: could not resolve previous app version for Step 1 values")
		return "???"
	}
	return prev
}

// resolveChartRootOverlaysQuiet returns the list of chart-root overlays that exist on disk.
// This is a dry-run helper — best-effort, silently filters to existing files only.
// enterprise adds registry/repo config and sub-chart image pins (Keycloak, PostgreSQL).
// digest, latest, and image-tags are mutually exclusive for image version resolution:
//   - image-tags (SNAPSHOT tags from env) takes priority over digest/latest
//   - useLatest selects values-latest.yaml instead of values-digest.yaml
//   - OCI immutability mode selects no chart-root overlays
//   - digest is the CI default when neither image-tags nor useLatest is active
func resolveChartRootOverlaysQuiet(chartPath string, entry Entry, opts RunOptions) []string {
	if chartPath == "" {
		return nil
	}
	if ociImmutabilityMode(opts) {
		logging.Logger.Warn().
			Str("chartRef", opts.ChartRef).
			Msg("OCI immutability mode: skipping chart-root image overlays (dry-run)")
	}
	overlays := resolveChartRootOverlays(entry, opts)
	// Filter to only overlays whose files exist on disk.
	var existing []string
	for _, name := range overlays {
		path := filepath.Join(chartPath, "values-"+name+".yaml")
		if _, err := os.Stat(path); err == nil {
			existing = append(existing, name)
		}
	}
	return existing
}

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

			// Upgrade plan — show two-step upgrade details for two-step upgrade flows,
			// or upgrade-only details for modular-upgrade-minor.
			step2Target := "local chart"
			if opts.ChartRef != "" {
				step2Target = opts.ChartRef
				if opts.ChartRefVersion != "" {
					step2Target += "@" + opts.ChartRefVersion
				}
			}
			if e.upgradeOnly && e.upgradeFromVersion != "" {
				fmt.Fprintf(&b, "      %s %s %s → %s %s\n",
					dryKey("upgrade:"),
					dryDim("upgrade-only (no install step), expects"),
					dryWarn(versionmatrix.DefaultHelmChartRef+"@"+e.upgradeFromVersion),
					dryDim("already running, upgrading to"),
					dryWarn(step2Target))
			} else if !e.upgradeOnly && e.upgradeFromVersion != "" {
				fmt.Fprintf(&b, "      %s %s %s → %s %s\n",
					dryKey("upgrade:"),
					dryDim("Step 1: install"),
					dryWarn(versionmatrix.DefaultHelmChartRef+"@"+e.upgradeFromVersion),
					dryDim("Step 2: upgrade to"),
					dryWarn(step2Target))
				// For upgrade-minor, Step 1 uses the previous version's values files.
				// Show this explicitly so operators know values come from a different chart dir.
				if e.step1ValuesFrom != "" {
					fmt.Fprintf(&b, "      %s %s %s\n",
						dryKey("step1-values:"),
						dryDim("from"),
						dryWarn("camunda-platform-"+e.step1ValuesFrom))
				}
			}

			// Pre-upgrade script — show the script that runs between Step 1 and Step 2.
			if e.preUpgradeScript != "" {
				// Show a relative path from the repo root for readability.
				scriptDisplay := e.preUpgradeScript
				if opts.RepoRoot != "" {
					if rel, err := filepath.Rel(opts.RepoRoot, e.preUpgradeScript); err == nil {
						scriptDisplay = rel
					}
				}
				fmt.Fprintf(&b, "      %s %s\n",
					dryKey("pre-upgrade:"),
					dryWarn(scriptDisplay))
			}

			// Chart-root overlays — show when overlay files will be applied.
			if len(e.chartRootOverlays) > 0 {
				fmt.Fprintf(&b, "      %s %s\n",
					dryKey("overlays:"),
					dryWarn(strings.Join(e.chartRootOverlays, ", ")))
			}

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
			if e.deleteNS {
				fmt.Fprintf(&b, "      %s %s\n", dryKey("delete-ns:"), dryWarn("true"))
			}
			if e.ensureDockerRegistry {
				fmt.Fprintf(&b, "      %s    %s\n", dryKey("docker:"), dryWarn("true"))
			}
			if e.ensureDockerHub {
				fmt.Fprintf(&b, "      %s %s\n", dryKey("dockerhub:"), dryWarn("true"))
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

// coverageEntry holds resolved layer information for one matrix entry in coverage mode.
type coverageEntry struct {
	entry       Entry
	platform    string
	identity    string
	persistence string
	infraType   string
	features    []string
	flow        string
}

// coverageReport resolves all entries and prints a layer-breakdown table to stdout.
// Like dryRun it performs no deployment — it shows what IS tested in the matrix.
func coverageReport(entries []Entry, opts RunOptions) []RunResult {
	var results []RunResult
	versions := VersionOrder(entries)
	groups := GroupByVersion(entries)

	var resolved []coverageEntry
	for _, version := range versions {
		for _, entry := range groups[version] {
			platform := resolvePlatform(opts, entry)

			// Resolve deployment layers via the canonical builder.
			scenarioDir := filepath.Join(entry.ChartPath, "test/integration/scenarios/chart-full-setup")
			deployConfig, buildErr := scenarios.BuildDeploymentConfig(entry.Scenario, scenarioDir, scenarios.BuilderOverrides{
				Identity:    entry.Identity,
				Persistence: entry.Persistence,
				Platform:    platform,
				Features:    entry.Features,
				InfraType:   entry.InfraType,
				Flow:        entry.Flow,
				QA:          entry.QA || opts.UseQA,
				ImageTags:   effectiveImageTags(entry, opts),
				Upgrade:     entry.Upgrade,
			})
			if buildErr != nil {
				results = append(results, RunResult{
					Entry: entry,
					Error: fmt.Errorf("deployment config validation failed: %w", buildErr),
				})
				continue
			}

			resolved = append(resolved, coverageEntry{
				entry:       entry,
				platform:    platform,
				identity:    deployConfig.Identity,
				persistence: deployConfig.Persistence,
				infraType:   entry.InfraType,
				features:    deployConfig.Features,
				flow:        entry.Flow,
			})

			namespace := resolveNamespace(opts, entry)
			results = append(results, RunResult{Entry: entry, Namespace: namespace})
		}
	}

	fmt.Fprintln(os.Stdout, formatCoverageOutput(resolved, versions))
	return results
}

// formatCoverageOutput produces a compact table showing what each scenario tests.
// Columns: VER | SCENARIO | ENABLED | FLOW | PLATFORM | INFRA-TYPE | IDENTITY | PERSISTENCE | FEATURES
func formatCoverageOutput(entries []coverageEntry, versions []string) string {
	var b strings.Builder

	// Group by version.
	groups := make(map[string][]coverageEntry)
	for _, e := range entries {
		groups[e.entry.Version] = append(groups[e.entry.Version], e)
	}

	// Table header — pad text first, then apply style (ANSI codes break %-Ns padding).
	fmt.Fprintf(&b, "%s\n\n", dryHead("=== Coverage: Layer Breakdown ==="))
	fmt.Fprintf(&b, "%s %s %s %s %s %s %s %s %s\n",
		dryHead(fmt.Sprintf("%-6s", "VER")),
		dryHead(fmt.Sprintf("%-25s", "SCENARIO")),
		dryHead(fmt.Sprintf("%-8s", "ENABLED")),
		dryHead(fmt.Sprintf("%-16s", "FLOW")),
		dryHead(fmt.Sprintf("%-10s", "PLATFORM")),
		dryHead(fmt.Sprintf("%-14s", "INFRA-TYPE")),
		dryHead(fmt.Sprintf("%-20s", "IDENTITY")),
		dryHead(fmt.Sprintf("%-22s", "PERSISTENCE")),
		dryHead("FEATURES"))
	fmt.Fprintf(&b, "%-6s %-25s %-8s %-16s %-10s %-14s %-20s %-22s %s\n",
		"---", "--------", "-------", "----", "--------", "----------", "--------", "-----------", "--------")

	for _, version := range versions {
		versionEntries := groups[version]
		for _, e := range versionEntries {
			// Pad enabled text before applying color so column width is consistent.
			enabled := fmt.Sprintf("%-8s", "yes")
			if e.entry.Enabled {
				enabled = dryOk(enabled)
			} else {
				enabled = dryWarn(fmt.Sprintf("%-8s", "no"))
			}

			platform := e.platform
			if platform == "" {
				platform = "-"
			}
			infraType := e.infraType
			if infraType == "" {
				infraType = "-"
			}
			identity := e.identity
			if identity == "" {
				identity = "(derived)"
			}
			persistence := e.persistence
			if persistence == "" {
				persistence = "(derived)"
			}
			features := strings.Join(e.features, ",")
			if features == "" {
				features = "-"
			}
			flow := e.flow
			if flow == "" {
				flow = "install"
			}

			fmt.Fprintf(&b, "%-6s %-25s %s %-16s %-10s %-14s %-20s %-22s %s\n",
				e.entry.Version,
				e.entry.Scenario,
				enabled,
				flow,
				platform,
				infraType,
				identity,
				persistence,
				features)
		}
	}

	// Summary.
	total := len(entries)
	enabledCount := 0
	for _, e := range entries {
		if e.entry.Enabled {
			enabledCount++
		}
	}
	fmt.Fprintf(&b, "\n%s\n",
		dryHead(fmt.Sprintf("--- %d entries (%d enabled, %d disabled) across %d versions ---",
			total, enabledCount, total-enabledCount, len(versions))))

	// Layer summary: unique values per dimension.
	identities := uniqueStrings(entries, func(e coverageEntry) string { return e.identity })
	persistences := uniqueStrings(entries, func(e coverageEntry) string { return e.persistence })
	platforms := uniqueStrings(entries, func(e coverageEntry) string { return e.platform })
	infraTypes := uniqueStrings(entries, func(e coverageEntry) string { return e.infraType })
	features := uniqueFeatures(entries)
	flows := uniqueStrings(entries, func(e coverageEntry) string { return e.flow })

	fmt.Fprintf(&b, "\n%s\n", dryHead("Layer Coverage:"))
	fmt.Fprintf(&b, "  %s  %s\n", dryKey("identities: "), strings.Join(identities, ", "))
	fmt.Fprintf(&b, "  %s  %s\n", dryKey("persistence:"), strings.Join(persistences, ", "))
	fmt.Fprintf(&b, "  %s   %s\n", dryKey("platforms:  "), strings.Join(platforms, ", "))
	fmt.Fprintf(&b, "  %s  %s\n", dryKey("infra-types:"), strings.Join(infraTypes, ", "))
	fmt.Fprintf(&b, "  %s    %s\n", dryKey("features:  "), strings.Join(features, ", "))
	fmt.Fprintf(&b, "  %s       %s\n", dryKey("flows:  "), strings.Join(flows, ", "))

	return b.String()
}

// uniqueStrings returns unique non-empty values from a field extractor, preserving first-seen order.
func uniqueStrings(entries []coverageEntry, extract func(coverageEntry) string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, e := range entries {
		v := extract(e)
		if v != "" && !seen[v] {
			seen[v] = true
			result = append(result, v)
		}
	}
	return result
}

// uniqueFeatures returns all unique feature names across entries, preserving first-seen order.
func uniqueFeatures(entries []coverageEntry) []string {
	seen := make(map[string]bool)
	var result []string
	for _, e := range entries {
		for _, f := range e.features {
			if !seen[f] {
				seen[f] = true
				result = append(result, f)
			}
		}
	}
	if len(result) == 0 {
		return []string{"-"}
	}
	return result
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
				logEvent := logging.Logger.Error().
					Err(result.Error).
					Str("version", entry.Version).
					Str("scenario", entry.Scenario).
					Str("flow", entry.Flow)
				var helmErr *deployer.HelmError
				if errors.As(result.Error, &helmErr) {
					logEvent = logEvent.Str("command", helmErr.ShortCommand())
				}
				logEvent.Msg("Matrix entry failed")

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
					Namespace: resolveNamespace(opts, e),
					Error:     fmt.Errorf("skipped: run cancelled"),
				}
				if opts.OnEntryComplete != nil {
					opts.OnEntryComplete(e, results[idx])
				}
				return
			}

			result := executeEntry(runCtx, e, opts)
			results[idx] = result

			if result.Error != nil {
				logEvent := logging.Logger.Error().
					Err(result.Error).
					Str("version", e.Version).
					Str("scenario", e.Scenario).
					Str("flow", e.Flow)
				var helmErr *deployer.HelmError
				if errors.As(result.Error, &helmErr) {
					logEvent = logEvent.Str("command", helmErr.ShortCommand())
				}
				logEvent.Msg("Matrix entry failed")

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

// resolveNamespace returns opts.NamespaceOverride when set, otherwise the
// matrix-formula namespace. Used by per-scenario CI workflows that pre-create
// the namespace and need matrix run to deploy into that exact namespace.
func resolveNamespace(opts RunOptions, entry Entry) string {
	if opts.NamespaceOverride != "" {
		return opts.NamespaceOverride
	}
	return buildNamespace(opts.NamespacePrefix, entry)
}

// buildBaseNamespace constructs the namespace suffix for a matrix entry without the prefix.
// Pattern: <version-compact>-<shortname>-<flow>[-<platform>]
// e.g., 88-eske-inst, 87-es-upgp-gke.
//
// Special case: modular-upgrade-minor targets the install flow's namespace (uses "inst"
// suffix instead of "mugm"). In CI, modular-upgrade-minor reuses the install flow's
// namespace — it does not create its own. This ensures the upgrade-only step deploys
// into the same namespace where the prior install run created the deployment.
func buildBaseNamespace(entry Entry) string {
	versionCompact := strings.ReplaceAll(entry.Version, ".", "")
	shortname := entry.Shortname
	if shortname == "" {
		shortname = entry.Scenario
	}
	flow := flowAbbrev(entry.Flow)
	// modular-upgrade-minor targets the install namespace.
	if versionmatrix.IsUpgradeOnlyFlow(entry.Flow) {
		flow = flowAbbrev("install")
	}
	if entry.Platform != "" {
		return fmt.Sprintf("%s-%s-%s-%s", versionCompact, shortname, flow, entry.Platform)
	}
	return fmt.Sprintf("%s-%s-%s", versionCompact, shortname, flow)
}

// flowAbbrev returns a short abbreviation for a flow name, used in namespace construction.
var flowAbbrevMap = map[string]string{
	"install":               "inst",
	"upgrade-patch":         "upgp",
	"upgrade-minor":         "upgm",
	"modular-upgrade-minor": "mugm",
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

// warmUpKubeContexts makes a lightweight API call to each unique kube context
// used by the matrix entries. This triggers any pending interactive login
// (e.g., Teleport browser SSO) sequentially, before parallel dispatch begins.
func warmUpKubeContexts(ctx context.Context, entries []Entry, opts RunOptions) error {
	seen := make(map[string]bool)
	for _, entry := range entries {
		platform := resolvePlatform(opts, entry)
		kubeCtx := resolveKubeContext(opts, platform)
		if kubeCtx == "" || seen[kubeCtx] {
			continue
		}
		seen[kubeCtx] = true

		logging.Logger.Info().
			Str("kubeContext", kubeCtx).
			Msg("Verifying cluster connectivity")

		if err := kube.CheckConnectivity(ctx, kubeCtx); err != nil {
			return fmt.Errorf("cluster connectivity check failed: %w", err)
		}
	}
	return nil
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

// diagnosticsDir is the directory where per-namespace diagnostic files are written.
const diagnosticsDir = "diagnostics"

const diagnosticsPodTailLines = 500

var diagnosticsFileSanitizer = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

type diagnosticsPodLog struct {
	Pod   string `json:"pod"`
	File  string `json:"file,omitempty"`
	Error string `json:"error,omitempty"`
}

type diagnosticsSummary struct {
	Namespace         string              `json:"namespace"`
	KubeContext       string              `json:"kubeContext,omitempty"`
	CollectedAt       string              `json:"collectedAt"`
	PodLogTailLines   int                 `json:"podLogTailLines"`
	Pods              string              `json:"pods,omitempty"`
	Events            string              `json:"events,omitempty"`
	PodLogs           []diagnosticsPodLog `json:"podLogs,omitempty"`
	TestOutputLast200 string              `json:"testOutputLast200,omitempty"`
	Errors            []string            `json:"errors,omitempty"`
}

func diagnosticsTimestamp(now time.Time) string {
	return now.UTC().Format("20060102T150405Z")
}

func diagnosticsRunDir(namespace string, now time.Time) string {
	return filepath.Join(diagnosticsDir, namespace, diagnosticsTimestamp(now))
}

func sanitizeDiagnosticsFilename(name string) string {
	clean := strings.TrimSpace(name)
	clean = diagnosticsFileSanitizer.ReplaceAllString(clean, "_")
	if clean == "" {
		return "unknown"
	}
	return clean
}

func writeDiagnosticsSummary(runDir string, summary diagnosticsSummary) error {
	summaryPath := filepath.Join(runDir, "summary.json")
	b, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal diagnostics summary: %w", err)
	}
	if err := os.WriteFile(summaryPath, append(b, '\n'), 0o644); err != nil {
		return fmt.Errorf("write diagnostics summary: %w", err)
	}
	return nil
}

func writeDiagnosticsReadme(runDir string, summary diagnosticsSummary) error {
	var b strings.Builder
	fmt.Fprintf(&b, "Diagnostics for namespace %s\n", summary.Namespace)
	fmt.Fprintf(&b, "Collected at: %s\n", summary.CollectedAt)
	if summary.KubeContext != "" {
		fmt.Fprintf(&b, "Kube context: %s\n", summary.KubeContext)
	}
	fmt.Fprintf(&b, "\nFiles:\n")
	fmt.Fprintf(&b, "- summary.json\n")
	fmt.Fprintf(&b, "- logs/<pod>.log (last %d lines per pod)\n", summary.PodLogTailLines)
	if summary.TestOutputLast200 != "" {
		fmt.Fprintf(&b, "- test-output.txt (last 200 lines)\n")
	}

	readmePath := filepath.Join(runDir, "README.txt")
	if err := os.WriteFile(readmePath, []byte(b.String()), 0o644); err != nil {
		return fmt.Errorf("write diagnostics readme: %w", err)
	}
	return nil
}

// collectDiagnostics gathers best-effort namespace diagnostics after a deployment failure.
// It writes run-scoped artifacts under diagnostics/<namespace>/<timestamp>/:
// - summary.json (machine-friendly metadata)
// - README.txt (quick human index)
// - logs/<pod>.log (last diagnosticsPodTailLines lines for every pod)
// Uses a fresh background context so that diagnostics work even when the parent context
// is cancelled (e.g., StopOnFailure).
// Returns the run directory path on success, or empty string if collection/write fails.
// Never returns an error — all failures are silently swallowed.
func collectDiagnostics(namespace, kubeContext string) string {
	// Use a fresh context with a generous timeout for the full collection.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	runDir := diagnosticsRunDir(namespace, time.Now())
	logsDir := filepath.Join(runDir, "logs")
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		logging.Logger.Warn().Err(err).Str("path", logsDir).Msg("failed to create diagnostics logs directory")
		return ""
	}

	summary := diagnosticsSummary{
		Namespace:       namespace,
		KubeContext:     kubeContext,
		CollectedAt:     time.Now().UTC().Format(time.RFC3339),
		PodLogTailLines: diagnosticsPodTailLines,
	}

	// Pods
	if pods, err := kube.GetPods(ctx, kubeContext, namespace); err == nil && pods != "" {
		summary.Pods = pods
	} else if err != nil {
		summary.Errors = append(summary.Errors, fmt.Sprintf("get pods: %v", err))
	}

	// Events (truncate to last 20 lines)
	if events, err := kube.GetEvents(ctx, kubeContext, namespace); err == nil && events != "" {
		lines := strings.Split(events, "\n")
		if len(lines) > 21 { // 1 header + 20 events
			lines = append(lines[:1], lines[len(lines)-20:]...)
		}
		summary.Events = strings.Join(lines, "\n")
	} else if err != nil {
		summary.Errors = append(summary.Errors, fmt.Sprintf("get events: %v", err))
	}

	// Logs from all pods: one file per pod under logs/.
	if podNames, err := kube.GetPodNames(ctx, kubeContext, namespace); err == nil {
		sort.Strings(podNames)
		for _, pod := range podNames {
			entry := diagnosticsPodLog{Pod: pod}

			logs, logErr := kube.GetPodLogs(ctx, kubeContext, namespace, pod, diagnosticsPodTailLines)
			if logErr != nil {
				entry.Error = logErr.Error()
				summary.PodLogs = append(summary.PodLogs, entry)
				summary.Errors = append(summary.Errors, fmt.Sprintf("get pod logs (%s): %v", pod, logErr))
				continue
			}

			if logs != "" {
				relPath := filepath.Join("logs", sanitizeDiagnosticsFilename(pod)+".log")
				absPath := filepath.Join(runDir, relPath)
				if err := os.WriteFile(absPath, []byte(logs), 0o644); err != nil {
					entry.Error = fmt.Sprintf("write log file: %v", err)
					summary.Errors = append(summary.Errors, fmt.Sprintf("write pod logs (%s): %v", pod, err))
				} else {
					entry.File = relPath
				}
			}

			summary.PodLogs = append(summary.PodLogs, entry)
		}
	} else {
		summary.Errors = append(summary.Errors, fmt.Sprintf("list pod names: %v", err))
	}

	if err := writeDiagnosticsSummary(runDir, summary); err != nil {
		logging.Logger.Warn().Err(err).Msg("failed to write diagnostics summary")
		return ""
	}
	if err := writeDiagnosticsReadme(runDir, summary); err != nil {
		logging.Logger.Warn().Err(err).Msg("failed to write diagnostics readme")
		return ""
	}

	return runDir
}

// appendTestOutputToDiagnostics checks whether err wraps a *deploy.TestError and, if so,
// persists the captured test script output in diagnostics. When the deployment
// itself succeeded (all pods healthy) but the post-deployment tests failed, the standard
// collectDiagnostics may produce an empty or minimal file. This function ensures the
// test output is always persisted so developers can debug test failures from the
// diagnostics directory alone.
//
// If diagPath is empty (no k8s diagnostics were collected), a new run directory is created.
// Returns the diagnostics run directory path, or the original diagPath if no test output was found.
func appendTestOutputToDiagnostics(err error, namespace, diagPath string) string {
	var testErr *deploy.TestError
	if !errors.As(err, &testErr) || testErr.Output == "" {
		return diagPath
	}

	// Truncate test output to the last 200 lines to keep diagnostics files manageable.
	output := lastNLines(testErr.Output, 200)
	runDir := diagPath
	if runDir == "" {
		runDir = diagnosticsRunDir(namespace, time.Now())
		if err := os.MkdirAll(filepath.Join(runDir, "logs"), 0o755); err != nil {
			logging.Logger.Warn().Err(err).Str("path", runDir).Msg("failed to create diagnostics directory for test output")
			return ""
		}
	}

	summaryPath := filepath.Join(runDir, "summary.json")
	summary := diagnosticsSummary{
		Namespace:       namespace,
		CollectedAt:     time.Now().UTC().Format(time.RFC3339),
		PodLogTailLines: diagnosticsPodTailLines,
	}
	if existing, readErr := os.ReadFile(summaryPath); readErr == nil {
		if err := json.Unmarshal(existing, &summary); err != nil {
			logging.Logger.Warn().Err(err).Str("path", summaryPath).Msg("failed to parse existing diagnostics summary")
		}
	}
	if summary.Namespace == "" {
		summary.Namespace = namespace
	}
	if summary.CollectedAt == "" {
		summary.CollectedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if summary.PodLogTailLines == 0 {
		summary.PodLogTailLines = diagnosticsPodTailLines
	}
	summary.TestOutputLast200 = output

	testOutputPath := filepath.Join(runDir, "test-output.txt")
	if err := os.WriteFile(testOutputPath, []byte(output), 0o644); err != nil {
		logging.Logger.Warn().Err(err).Str("path", testOutputPath).Msg("failed to write test output file")
		summary.Errors = append(summary.Errors, fmt.Sprintf("write test output: %v", err))
	}

	if err := writeDiagnosticsSummary(runDir, summary); err != nil {
		logging.Logger.Warn().Err(err).Str("path", summaryPath).Msg("failed to write diagnostics summary with test output")
		return runDir
	}
	if err := writeDiagnosticsReadme(runDir, summary); err != nil {
		logging.Logger.Warn().Err(err).Str("path", runDir).Msg("failed to update diagnostics readme with test output")
	}

	return runDir
}

// lastNLines returns the last n lines of s. If s has n or fewer lines, it is
// returned unchanged (without a trailing newline added).
func lastNLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}

// cleanupEntry performs per-entry cleanup after deployment and tests have completed.
// It cleans up the Entra app registration (for OIDC entries) and deletes the namespace.
// This runs regardless of whether the entry succeeded or failed — cleanup should always
// happen after diagnostics have been collected. Errors are logged but do not affect the
// entry's result.
func cleanupEntry(ctx context.Context, result RunResult, opts RunOptions) {
	// Clean up Entra app registration for OIDC entries (best-effort, before namespace deletion).
	if result.venomOpts != nil {
		logging.Logger.Info().
			Str("namespace", result.Namespace).
			Msg("Cleaning up venom Entra app registration")
		entra.CleanupVenomApp(ctx, *result.venomOpts)
	}

	// Clean up Auth0 clients for Auth0 entries (best-effort, before namespace deletion).
	if result.auth0Opts != nil {
		logging.Logger.Info().
			Str("namespace", result.Namespace).
			Msg("Cleaning up Auth0 clients")
		auth0.CleanupClients(ctx, *result.auth0Opts)
	}

	// Delete the namespace.
	if result.Namespace != "" {
		logging.Logger.Info().
			Str("namespace", result.Namespace).
			Str("kubeContext", result.KubeContext).
			Msg("Deleting namespace (per-entry cleanup)")

		// Do not let cleanup block matrix completion for too long.
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := kube.DeleteNamespace(cleanupCtx, "", result.KubeContext, result.Namespace); err != nil {
			logging.Logger.Error().
				Err(err).
				Str("namespace", result.Namespace).
				Msg("Failed to delete namespace during per-entry cleanup")
		} else {
			logging.Logger.Info().
				Str("namespace", result.Namespace).
				Msg("Namespace deleted successfully")
		}
	}
}

// mergeHelmSets returns a new map containing all entries from base, with entries
// from override applied on top. nil maps are tolerated. Returns nil when both
// maps are empty so downstream code stays nil-clean.
func mergeHelmSets(base, override map[string]string) map[string]string {
	if len(base) == 0 && len(override) == 0 {
		return nil
	}
	out := make(map[string]string, len(base)+len(override))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range override {
		out[k] = v
	}
	return out
}

// parseHelmSetPairs converts a slice of "key=value" strings into a map suitable
// for config.DeploymentFlags.ExtraHelmSets. Entries without "=" are skipped.
func parseHelmSetPairs(pairs []string) map[string]string {
	if len(pairs) == 0 {
		return nil
	}
	out := make(map[string]string, len(pairs))
	for _, p := range pairs {
		idx := strings.Index(p, "=")
		if idx <= 0 {
			continue
		}
		out[p[:idx]] = p[idx+1:]
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// PrintRunSummary outputs a summary of all run results including per-entry timings.
// wallClock is the actual elapsed wall-clock duration for the entire matrix run.
// When entries run in parallel, this will be less than the sum of individual entry durations.
// logDir, when non-empty, shows per-entry log file paths for failed entries.
func PrintRunSummary(results []RunResult, wallClock time.Duration, logDir string) string {
	if len(results) == 0 {
		return "No entries executed."
	}

	var b strings.Builder
	successCount := 0
	failCount := 0
	skipCount := 0
	var sumDuration time.Duration

	for _, r := range results {
		sumDuration += r.Duration
		if r.Error == nil {
			successCount++
		} else if r.Duration == 0 && strings.Contains(r.Error.Error(), "skipped") {
			skipCount++
		} else {
			failCount++
		}
	}

	fmt.Fprintf(&b, "\n%s\n", dryHead("=== Matrix Run Summary ==="))
	fmt.Fprintf(&b, "%s   %d\n", dryKey("Total:"), len(results))
	fmt.Fprintf(&b, "%s %s\n", dryKey("Success:"), dryOk(fmt.Sprintf("%d", successCount)))
	if failCount > 0 {
		fmt.Fprintf(&b, "%s  %s\n", dryKey("Failed:"), dryFail(fmt.Sprintf("%d", failCount)))
	} else {
		fmt.Fprintf(&b, "%s  %d\n", dryKey("Failed:"), failCount)
	}
	if skipCount > 0 {
		fmt.Fprintf(&b, "%s %s\n", dryKey("Skipped:"), dryWarn(fmt.Sprintf("%d", skipCount)))
	}

	// Per-entry timings table.
	fmt.Fprintf(&b, "\n%s\n", dryHead("Entry timings:"))
	for _, r := range results {
		var status string
		if r.Error != nil {
			if r.Duration == 0 && strings.Contains(r.Error.Error(), "skipped") {
				status = dryWarn("[SKIP]")
			} else {
				status = dryFail("[FAIL]")
			}
		} else {
			status = dryOk("[PASS]")
		}

		shortname := r.Entry.Shortname
		if shortname == "" {
			shortname = r.Entry.Scenario
		}
		label := fmt.Sprintf("%s/%s (%s, flow=%s)",
			r.Entry.Version, r.Entry.Scenario, shortname, r.Entry.Flow)

		fmt.Fprintf(&b, "  %-6s %-60s %s\n", status, label, dryDim(formatDuration(r.Duration)))
	}
	fmt.Fprintf(&b, "\n  %s %s\n", dryKey("Total time:"), dryVal(formatDuration(wallClock)))
	// Show the sum of entry durations when it differs from wall-clock (parallel execution).
	if sumDuration > wallClock+(1*time.Second) {
		fmt.Fprintf(&b, "  %s %s\n", dryKey("Sum of entries:"), dryVal(formatDuration(sumDuration)))
	}

	if failCount > 0 {
		fmt.Fprintf(&b, "\n%s\n", dryHead("Failed entries:"))
		for _, r := range results {
			if r.Error != nil {
				if r.Duration == 0 && strings.Contains(r.Error.Error(), "skipped") {
					continue // Skip cancelled entries in the failure details.
				}
				fmt.Fprintf(&b, "\n  - %s/%s (%s, flow=%s)\n",
					r.Entry.Version, r.Entry.Scenario, r.Entry.Shortname, r.Entry.Flow)

				var helmErr *deployer.HelmError
				if errors.As(r.Error, &helmErr) {
					// Show step context if error is wrapped (e.g. "step 1: install ... failed")
					fullMsg := r.Error.Error()
					helmMsg := helmErr.Error()
					if prefix := strings.TrimSuffix(fullMsg, helmMsg); prefix != "" {
						prefix = strings.TrimRight(prefix, ": ")
						fmt.Fprintf(&b, "    %s    %s\n", dryKey("Step:"), dryWarn(prefix))
					}
					fmt.Fprintf(&b, "    %s  %s\n", dryKey("Reason:"), dryFail(fmt.Sprintf("%s: %v", helmErr.Reason, helmErr.Cause)))
					fmt.Fprintf(&b, "    %s %s\n", dryKey("Command:"), helmErr.ShortCommand())
				} else {
					fmt.Fprintf(&b, "    %s %s\n", dryKey("Error:"), dryFail(fmt.Sprintf("%v", r.Error)))
				}

				if r.Diagnostics != "" {
					fmt.Fprintf(&b, "    %s %s\n", dryKey("Diagnostics:"), dryWarn(r.Diagnostics))
				}

				if logDir != "" {
					baseName := entryLogFileName(r.Entry)
					fmt.Fprintf(&b, "    %s\n", dryKey("Logs:"))
					fmt.Fprintf(&b, "      deploy:  %s\n", filepath.Join(logDir, baseName+".deploy.log"))
					fmt.Fprintf(&b, "      it:      %s\n", filepath.Join(logDir, baseName+".it.log"))
					fmt.Fprintf(&b, "      e2e:     %s\n", filepath.Join(logDir, baseName+".e2e.log"))
				}
			}
		}
	}

	if logDir != "" {
		fmt.Fprintf(&b, "\n  %s %s\n", dryKey("Log directory:"), logDir)
	}

	return b.String()
}

// formatDuration formats a duration into a human-friendly string.
// Uses the compact "1m30s" style for durations >= 1 minute, and "45.2s" for shorter durations.
func formatDuration(d time.Duration) string {
	if d == 0 {
		return "0s"
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60
	if minutes >= 60 {
		hours := minutes / 60
		minutes = minutes % 60
		return fmt.Sprintf("%dh%dm%ds", hours, minutes, seconds)
	}
	return fmt.Sprintf("%dm%02ds", minutes, seconds)
}
