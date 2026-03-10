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
	"scripts/camunda-core/pkg/executil"
	"scripts/camunda-core/pkg/helm"
	"scripts/camunda-core/pkg/kube"
	"scripts/camunda-core/pkg/logging"
	"scripts/camunda-core/pkg/scenarios"
	"scripts/camunda-core/pkg/versionmatrix"
	"scripts/camunda-deployer/pkg/deployer"
	"scripts/deploy-camunda/config"
	"scripts/deploy-camunda/deploy"
	"scripts/deploy-camunda/entra"
	"scripts/prepare-helm-values/pkg/env"
)

// numESPools is the number of Elasticsearch pools across which matrix entries
// are distributed via round-robin. This matches the 4-cluster pool infra.
const numESPools = 4

// RunOptions controls matrix execution.
type RunOptions struct {
	// DryRun logs what would be done without executing.
	DryRun bool
	// StopOnFailure stops the run on the first failure.
	// In parallel mode, this cancels in-flight entries and prevents new ones from starting.
	StopOnFailure bool
	// Cleanup deletes each entry's namespace immediately after its deployment
	// and tests complete (regardless of success or failure). This frees cluster
	// resources as early as possible rather than waiting for the entire run to finish.
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
	// DeleteNamespaceFirst deletes the namespace before deploying each matrix entry.
	// This ensures a clean-slate deployment by removing any existing resources in the namespace.
	DeleteNamespaceFirst bool
	// Coverage produces a layer-breakdown report showing what IS tested in the matrix.
	// Behaves like DryRun (no deployment), but outputs a focused table showing each
	// scenario's resolved layers (identity, persistence, platform, infra-type, features, flow).
	Coverage bool
	// UpgradeFromVersion overrides the auto-resolved "from" chart version for upgrade flows.
	// When set, this version is used instead of resolving from version-matrix JSON files.
	// Only applies to entries with upgrade flows (upgrade-patch, upgrade-minor, modular-upgrade-minor).
	UpgradeFromVersion string
	// HelmTimeout is the timeout in minutes for each Helm deployment.
	// Applies uniformly to all matrix entries (install, upgrade Step 1, upgrade Step 2).
	// When <= 0, deploy.Execute defaults to 5 minutes.
	HelmTimeout int
	// DockerUsername is the Harbor registry username for pulling images.
	// When empty, the deployer falls back to HARBOR_USERNAME, TEST_DOCKER_USERNAME_CAMUNDA_CLOUD, or NEXUS_USERNAME env vars.
	DockerUsername string
	// DockerPassword is the Harbor registry password for pulling images.
	// When empty, the deployer falls back to HARBOR_PASSWORD, TEST_DOCKER_PASSWORD_CAMUNDA_CLOUD, or NEXUS_PASSWORD env vars.
	DockerPassword string
	// EnsureDockerRegistry creates a Harbor registry secret in each entry's namespace.
	// When true, the deployer performs docker login and creates a registry-camunda-cloud
	// Kubernetes secret of type kubernetes.io/dockerconfigjson.
	EnsureDockerRegistry bool
	// DockerHubUsername is the Docker Hub registry username.
	// When empty, the deployer falls back to DOCKERHUB_USERNAME or TEST_DOCKER_USERNAME env vars.
	DockerHubUsername string
	// DockerHubPassword is the Docker Hub registry password.
	// When empty, the deployer falls back to DOCKERHUB_PASSWORD or TEST_DOCKER_PASSWORD env vars.
	DockerHubPassword string
	// EnsureDockerHub creates a Docker Hub pull secret (index-docker-io) in each entry's namespace.
	// When true, the deployer performs docker login and creates an index-docker-io
	// Kubernetes secret of type kubernetes.io/dockerconfigjson.
	EnsureDockerHub bool
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

	return results, retErr
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

			// Resolve deployment layers via the canonical builder (same logic as deploy.go prepareScenarioValues).
			scenarioDir := filepath.Join(entry.ChartPath, "test/integration/scenarios/chart-full-setup")
			deployConfig, buildErr := scenarios.BuildDeploymentConfig(entry.Scenario, scenarios.BuilderOverrides{
				Identity:    entry.Identity,
				Persistence: entry.Persistence,
				Platform:    platform,
				Features:    entry.Features,
				InfraType:   entry.InfraType,
				Flow:        entry.Flow,
				QA:          entry.QA,
				ImageTags:   entry.ImageTags,
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
				chartRootOverlays:    resolveChartRootOverlaysQuiet(entry.ChartPath, entry),
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

// resolvePreUpgradeScriptQuiet returns the pre-upgrade script path if one exists on disk.
// Returns empty string for non-upgrade flows or when no script is found (dry-run is best-effort).
func resolvePreUpgradeScriptQuiet(repoRoot string, entry Entry) string {
	if !versionmatrix.IsUpgradeFlow(entry.Flow) {
		return ""
	}
	if versionmatrix.HasPreUpgradeScript(repoRoot, entry.Version, entry.Flow) {
		return versionmatrix.PreUpgradeScriptPath(repoRoot, entry.Version, entry.Flow)
	}
	return ""
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
func resolveChartRootOverlaysQuiet(chartPath string, entry Entry) []string {
	if chartPath == "" {
		return nil
	}
	var overlays []string
	if entry.Enterprise {
		overlays = append(overlays, "enterprise")
	}
	overlays = append(overlays, "digest")
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
			if e.upgradeOnly && e.upgradeFromVersion != "" {
				fmt.Fprintf(&b, "      %s %s %s → %s %s\n",
					dryKey("upgrade:"),
					dryDim("upgrade-only (no install step), expects"),
					dryWarn(versionmatrix.DefaultHelmChartRef+"@"+e.upgradeFromVersion),
					dryDim("already running, upgrading to"),
					dryWarn("local chart"))
			} else if !e.upgradeOnly && e.upgradeFromVersion != "" {
				fmt.Fprintf(&b, "      %s %s %s → %s %s\n",
					dryKey("upgrade:"),
					dryDim("Step 1: install"),
					dryWarn(versionmatrix.DefaultHelmChartRef+"@"+e.upgradeFromVersion),
					dryDim("Step 2: upgrade to"),
					dryWarn("local chart"))
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
			deployConfig, buildErr := scenarios.BuildDeploymentConfig(entry.Scenario, scenarios.BuilderOverrides{
				Identity:    entry.Identity,
				Persistence: entry.Persistence,
				Platform:    platform,
				Features:    entry.Features,
				InfraType:   entry.InfraType,
				Flow:        entry.Flow,
				QA:          entry.QA,
				ImageTags:   entry.ImageTags,
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

			namespace := buildNamespace(opts.NamespacePrefix, entry)
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

	globalIndex := 0
	for _, version := range versions {
		versionEntries := groups[version]

		logging.Logger.Info().
			Str("version", version).
			Int("entries", len(versionEntries)).
			Msg("Processing version")

		for _, entry := range versionEntries {
			result := executeEntry(ctx, entry, opts, globalIndex)
			globalIndex++
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
					Namespace: buildNamespace(opts.NamespacePrefix, e),
					Error:     fmt.Errorf("skipped: run cancelled"),
				}
				return
			}

			result := executeEntry(runCtx, e, opts, idx)
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

// executeEntry deploys a single matrix entry by constructing RuntimeFlags and calling deploy.Execute().
// The entryIndex is used for round-robin ES pool distribution across the 4-cluster pool infra.
// The flow determines the execution strategy:
//   - Two-step upgrade (upgrade-patch, upgrade-minor): Step 1 installs old version, Step 2 upgrades.
//   - Upgrade-only (modular-upgrade-minor): Upgrades an already-running deployment (no install step).
//   - Install (default): Single-step fresh install.
func executeEntry(ctx context.Context, entry Entry, opts RunOptions, entryIndex int) RunResult {
	start := time.Now()
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
		Chart: config.ChartFlags{
			ChartPath:            entry.ChartPath,
			SkipDependencyUpdate: opts.SkipDependencyUpdate,
			RepoRoot:             opts.RepoRoot,
			// Build chart-root overlays: enterprise (if flagged) + digest (always in CI).
			ChartRootOverlays: func() []string {
				var overlays []string
				if entry.Enterprise {
					overlays = append(overlays, "enterprise")
				}
				overlays = append(overlays, "digest") // CI default: always pin image digests.
				return overlays
			}(),
		},
		Deployment: config.DeploymentFlags{
			Namespace:            baseNamespace,
			NamespacePrefix:      opts.NamespacePrefix,
			Release:              "integration",
			Scenario:             entry.Scenario,
			Scenarios:            []string{entry.Scenario},
			ScenarioPath:         scenarioDir,
			Platform:             platform,
			Flow:                 entry.Flow,
			Timeout:              opts.HelmTimeout,
			DeleteNamespaceFirst: opts.DeleteNamespaceFirst,
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
			ExternalSecrets:       true,
			AutoGenerateSecrets:   true,
			UseVaultBackedSecrets: useVault,
		},
		Test: config.TestFlags{
			KubeContext:         kubeCtx,
			TestExclude:         testExclude,
			RunIntegrationTests: (opts.TestIT || opts.TestAll) && !entry.SkipIT,
			RunE2ETests:         (opts.TestE2E || opts.TestAll) && !entry.SkipE2E,
			// Do NOT propagate RunAllTests here — RunE2ETests/RunIntegrationTests already
			// encode the full decision (including skip-e2e/skip-it from ci-test-config.yaml).
			// Setting RunAllTests would bypass the skip logic in deploy/test.go which ORs
			// RunAllTests with each individual flag.
			RunAllTests: false,
		},
		// Selection + Composition: pass explicit layer overrides from ci-test-config.yaml.
		// When set, these override MapScenarioToConfig name-based derivation in deploy.go.
		Selection: config.SelectionFlags{
			Identity:    entry.Identity,
			Persistence: entry.Persistence,
			Features:    entry.Features,
			InfraType:   entry.InfraType,
			QA:          entry.QA,
			ImageTags:   entry.ImageTags,
			UpgradeFlow: entry.Upgrade,
		},
	}

	flags.ESPoolIndex = strconv.Itoa(entryIndex % numESPools)

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
		Str("esPoolIndex", flags.ESPoolIndex).
		Msg("Deploying matrix entry")

	// Execute the deployment (deploy + tests run inside deploy.Execute).
	// All code paths converge into a single result so cleanup runs exactly once.
	var deployErr error
	var diag string

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

	result := RunResult{Entry: entry, Namespace: namespace, KubeContext: kubeCtx, Error: deployErr, Duration: time.Since(start), Diagnostics: diag, venomOpts: venomOpts}

	// Per-entry cleanup: delete namespace and Entra app after deployment + tests complete.
	// This runs regardless of success/failure, after diagnostics have been collected.
	if opts.Cleanup {
		cleanupEntry(ctx, result, opts)
	}

	return result
}

// executeTwoStepUpgrade performs a two-step upgrade deployment:
//
//	Step 1: Install the previously released chart version from the Helm repository.
//	Step 2: Upgrade to the current on-disk chart (the branch version) with
//	        upgrade-specific flags (--force, allowPreReleaseImages).
//
// Values file resolution for Step 1 depends on the flow:
//   - upgrade-patch: uses the CURRENT chart's values files (same app version, older release).
//   - upgrade-minor: uses the PREVIOUS app version's chart values files (e.g., 8.7 for an 8.8 entry).
//     This matches CI behavior where test-type-vars sets CHART_PATH to the previous version's
//     chart directory for upgrade-minor.
//
// The "from" chart version is resolved from version-matrix JSON files:
//   - upgrade-patch: latest stable chart for the SAME app version
//   - upgrade-minor: latest stable chart for the PREVIOUS app version

// bitnamiPGPasswordMapping maps Kubernetes Secret keys (from the "integration-test-credentials"
// secret) to Helm value paths that satisfy Bitnami PostgreSQL's password validation during upgrades.
//
// During `helm upgrade --force`, Bitnami's common.secrets.passwords.manage function does a `lookup`
// of the existing Secret. When --force causes resource deletion/recreation, the lookup can return nil,
// triggering a fail() if no explicit password is provided via `providedValues`. By extracting
// passwords from the cluster secret and passing them as --set overrides, we satisfy the
// `honorProvidedValues` check and bypass the lookup/fail path entirely.
var bitnamiPGPasswordMapping = map[string][]string{
	"identity-keycloak-postgresql-user-password":  {"identityKeycloak.postgresql.auth.password", "identityPostgresql.auth.password"},
	"identity-keycloak-postgresql-admin-password": {"identityKeycloak.postgresql.auth.postgresPassword", "identityPostgresql.auth.postgresPassword"},
	"webmodeler-postgresql-user-password":         {"webModelerPostgresql.auth.password"},
	"webmodeler-postgresql-admin-password":        {"webModelerPostgresql.auth.postgresPassword"},
}

// extractBitnamiPGPasswords reads the "integration-test-credentials" Kubernetes Secret from the
// given namespace and returns a map of Helm --set key=value pairs that provide Bitnami PostgreSQL
// passwords explicitly. This prevents the PASSWORDS ERROR that occurs during `helm upgrade --force`
// when Bitnami's template lookup returns nil for a temporarily-absent Secret.
//
// The function is intentionally lenient: if the secret doesn't exist, or individual keys are missing,
// it logs warnings and returns whatever it could extract. Callers should merge the result into
// ExtraHelmSets.
func extractBitnamiPGPasswords(ctx context.Context, namespace, kubeContext string) map[string]string {
	const secretName = "integration-test-credentials"

	kubeClient, err := kube.NewClient("", kubeContext)
	if err != nil {
		logging.Logger.Warn().Err(err).
			Str("namespace", namespace).
			Msg("Failed to create kube client for Bitnami PG password extraction; upgrade may fail with PASSWORDS ERROR")
		return nil
	}

	secretData, err := kubeClient.GetSecretData(ctx, namespace, secretName)
	if err != nil {
		logging.Logger.Warn().Err(err).
			Str("namespace", namespace).
			Str("secret", secretName).
			Msg("Failed to read secret for Bitnami PG password extraction; upgrade may fail with PASSWORDS ERROR")
		return nil
	}
	if secretData == nil {
		logging.Logger.Warn().
			Str("namespace", namespace).
			Str("secret", secretName).
			Msg("Secret not found for Bitnami PG password extraction; upgrade may fail with PASSWORDS ERROR")
		return nil
	}

	helmSets := make(map[string]string)
	for secretKey, helmPaths := range bitnamiPGPasswordMapping {
		value, ok := secretData[secretKey]
		if !ok || value == "" {
			logging.Logger.Warn().
				Str("namespace", namespace).
				Str("secret", secretName).
				Str("key", secretKey).
				Msg("Secret key missing or empty; corresponding Bitnami PG password override skipped")
			continue
		}
		for _, helmPath := range helmPaths {
			helmSets[helmPath] = value
		}
	}

	if len(helmSets) > 0 {
		logging.Logger.Info().
			Str("namespace", namespace).
			Int("overrides", len(helmSets)).
			Msg("Extracted Bitnami PG passwords from cluster secret for upgrade --set overrides")
	}

	return helmSets
}

func executeTwoStepUpgrade(ctx context.Context, entry Entry, flags *config.RuntimeFlags, opts RunOptions) error {
	// Resolve the "from" chart version for the upgrade.
	// If UpgradeFromVersion is set via CLI flag, use it directly; otherwise auto-resolve.
	var fromVersion string
	if opts.UpgradeFromVersion != "" {
		fromVersion = opts.UpgradeFromVersion
		logging.Logger.Info().
			Str("flow", entry.Flow).
			Str("fromVersion", fromVersion).
			Str("source", "cli-override").
			Msg("Two-step upgrade: using CLI-provided from-version")
	} else {
		var err error
		fromVersion, err = versionmatrix.ResolveUpgradeFromVersion(opts.RepoRoot, entry.Version, entry.Flow)
		if err != nil {
			return fmt.Errorf("resolve upgrade-from version for %s/%s: %w", entry.Version, entry.Flow, err)
		}
	}

	logging.Logger.Info().
		Str("flow", entry.Flow).
		Str("fromVersion", fromVersion).
		Str("toChart", entry.ChartPath).
		Str("version", entry.Version).
		Msg("Two-step upgrade: resolved from-version")

	// Pin index prefixes and Keycloak realm so that Step 1 and Step 2 share the
	// same values. Without this, each call to deploy.Execute() generates a new
	// random suffix, causing the upgraded components to look for indices/realm
	// that don't match what Step 1 created.
	if err := deploy.PinScenarioPrefixes(entry.Scenario, flags); err != nil {
		return fmt.Errorf("pin scenario prefixes for upgrade: %w", err)
	}
	logging.Logger.Info().
		Str("realm", flags.Auth.KeycloakRealm).
		Str("orchPrefix", flags.Index.OrchestrationIndexPrefix).
		Str("optPrefix", flags.Index.OptimizeIndexPrefix).
		Msg("Two-step upgrade: pinned index prefixes and realm for both steps")

	// --- Step 1: Install old version from Helm repo ---
	logging.Logger.Info().
		Str("step", "1/2").
		Str("action", "install").
		Str("chart", versionmatrix.DefaultHelmChartRef).
		Str("version", fromVersion).
		Msg("Step 1: Installing previous chart version from Helm repo")

	// Ensure the Camunda Helm repo is registered and up-to-date.
	if err := helm.RepoAdd(ctx, versionmatrix.DefaultHelmRepoName, versionmatrix.DefaultHelmRepoURL); err != nil {
		return fmt.Errorf("step 1: helm repo add: %w", err)
	}
	if err := helm.RepoUpdate(ctx); err != nil {
		return fmt.Errorf("step 1: helm repo update: %w", err)
	}

	// Clone flags for Step 1: deploy from repo instead of local chart path.
	step1Flags := *flags
	step1Flags.Chart.Chart = versionmatrix.DefaultHelmChartRef
	step1Flags.Chart.ChartVersion = fromVersion
	step1Flags.Chart.ChartPath = "" // Use repo chart, not local path.
	step1Flags.Deployment.Flow = "install"
	step1Flags.Selection.UpgradeFlow = false     // Step 1 is a fresh install, no base-upgrade.yaml.
	step1Flags.Chart.ChartRootOverlays = nil     // Step 1 installs old version from repo — no chart-root overlays.
	step1Flags.Chart.SkipDependencyUpdate = true // Repo charts don't need local dep update.
	step1Flags.Test.RunIntegrationTests = false  // Don't run tests after Step 1.
	step1Flags.Test.RunE2ETests = false
	step1Flags.Test.RunAllTests = false
	step1Flags.Deployment.DeleteNamespaceFirst = flags.Deployment.DeleteNamespaceFirst // Only delete on Step 1.

	// For upgrade-minor, Step 1 uses the PREVIOUS app version's values files.
	// In CI, test-type-vars sets CHART_PATH to charts/camunda-platform-<previous> for
	// the install step of upgrade-minor, so values files come from the older chart.
	// For upgrade-patch, Step 1 uses the current chart's values (same app version).
	if entry.Flow == "upgrade-minor" {
		prevVersion, err := versionmatrix.PreviousAppVersion(entry.Version)
		if err != nil {
			return fmt.Errorf("step 1: resolve previous app version for %s: %w", entry.Version, err)
		}
		prevChartDir := filepath.Join(opts.RepoRoot, "charts", "camunda-platform-"+prevVersion)
		prevScenarioDir := filepath.Join(prevChartDir, "test/integration/scenarios/chart-full-setup")
		step1Flags.Deployment.ScenarioPath = prevScenarioDir

		logging.Logger.Info().
			Str("flow", entry.Flow).
			Str("previousVersion", prevVersion).
			Str("scenarioDir", prevScenarioDir).
			Msg("Step 1: using previous app version's values files (matching CI behavior)")
	}

	if err := deploy.Execute(ctx, &step1Flags); err != nil {
		return fmt.Errorf("step 1: install %s@%s failed: %w", versionmatrix.DefaultHelmChartRef, fromVersion, err)
	}

	logging.Logger.Info().
		Str("step", "1/2").
		Str("version", fromVersion).
		Msg("Step 1 complete: previous version installed successfully")

	// --- Pre-upgrade lifecycle script ---
	// Run the pre-upgrade script (if it exists) between Step 1 and Step 2.
	// These scripts perform version-specific cleanup (e.g., deleting StatefulSets/PVCs)
	// that must happen after the old version is installed but before the upgrade.
	if scriptPath := versionmatrix.PreUpgradeScriptPath(opts.RepoRoot, entry.Version, entry.Flow); scriptPath != "" {
		if versionmatrix.HasPreUpgradeScript(opts.RepoRoot, entry.Version, entry.Flow) {
			namespace := flags.EffectiveNamespace()
			logging.Logger.Info().
				Str("script", scriptPath).
				Str("namespace", namespace).
				Str("flow", entry.Flow).
				Msg("Running pre-upgrade script")

			scriptEnv := []string{"TEST_NAMESPACE=" + namespace}
			if flags.Test.KubeContext != "" {
				scriptEnv = append(scriptEnv, "KUBE_CONTEXT="+flags.Test.KubeContext)
			}

			if err := executil.RunCommand(ctx, "bash", []string{"-x", scriptPath}, scriptEnv, ""); err != nil {
				return fmt.Errorf("pre-upgrade script %s failed: %w", scriptPath, err)
			}

			logging.Logger.Info().
				Str("script", scriptPath).
				Msg("Pre-upgrade script completed successfully")
		} else {
			logging.Logger.Debug().
				Str("script", scriptPath).
				Msg("Pre-upgrade script not found on disk, skipping")
		}
	}

	// --- Step 2: Upgrade to current on-disk chart ---
	logging.Logger.Info().
		Str("step", "2/2").
		Str("action", "upgrade").
		Str("chartPath", entry.ChartPath).
		Msg("Step 2: Upgrading to current chart version")

	// Clone flags for Step 2: upgrade from installed state to local chart.
	step2Flags := *flags
	step2Flags.Selection.UpgradeFlow = true            // Ensure base-upgrade.yaml is included.
	step2Flags.Deployment.DeleteNamespaceFirst = false // Namespace already exists from Step 1.
	step2Flags.Deployment.Flow = "install"             // Must match Step 1's Flow so $FLOW in index prefixes resolves identically.
	step2Flags.Deployment.ExtraHelmArgs = []string{"--force"}
	step2Flags.Deployment.ExtraHelmSets = map[string]string{
		"orchestration.upgrade.allowPreReleaseImages": "true",
	}

	// Extract Bitnami PostgreSQL passwords from the cluster secret and merge into --set overrides.
	// This prevents the PASSWORDS ERROR that Bitnami's secrets.yaml triggers during `helm upgrade --force`
	// when the Secret lookup returns nil (due to --force deleting/recreating resources).
	if pgPasswords := extractBitnamiPGPasswords(ctx, flags.EffectiveNamespace(), flags.Test.KubeContext); len(pgPasswords) > 0 {
		for k, v := range pgPasswords {
			step2Flags.Deployment.ExtraHelmSets[k] = v
		}
	}

	if err := deploy.Execute(ctx, &step2Flags); err != nil {
		return fmt.Errorf("step 2: upgrade to local chart failed: %w", err)
	}

	logging.Logger.Info().
		Str("step", "2/2").
		Msg("Step 2 complete: upgrade to current version succeeded")

	return nil
}

// executeUpgradeOnly performs a single-step upgrade against an already-running deployment.
// This is used for "modular-upgrade-minor" which, in CI, skips the install job entirely
// and only runs the upgrade job against the namespace of a prior "install" flow.
//
// The sequence is:
//  1. Run pre-upgrade script (if it exists on disk).
//  2. Helm upgrade to the current on-disk chart with upgrade-specific flags.
//
// Unlike executeTwoStepUpgrade, there is NO Step 1 install from the Helm repo.
// The previous version must already be deployed (by a prior "install" flow entry).
func executeUpgradeOnly(ctx context.Context, entry Entry, flags *config.RuntimeFlags, opts RunOptions) error {
	logging.Logger.Info().
		Str("flow", entry.Flow).
		Str("namespace", flags.EffectiveNamespace()).
		Str("chartPath", entry.ChartPath).
		Msg("Upgrade-only flow: upgrading existing deployment (no install step)")

	// --- Pre-upgrade lifecycle script ---
	if scriptPath := versionmatrix.PreUpgradeScriptPath(opts.RepoRoot, entry.Version, entry.Flow); scriptPath != "" {
		if versionmatrix.HasPreUpgradeScript(opts.RepoRoot, entry.Version, entry.Flow) {
			namespace := flags.EffectiveNamespace()
			logging.Logger.Info().
				Str("script", scriptPath).
				Str("namespace", namespace).
				Str("flow", entry.Flow).
				Msg("Running pre-upgrade script")

			scriptEnv := []string{"TEST_NAMESPACE=" + namespace}
			if flags.Test.KubeContext != "" {
				scriptEnv = append(scriptEnv, "KUBE_CONTEXT="+flags.Test.KubeContext)
			}

			if err := executil.RunCommand(ctx, "bash", []string{"-x", scriptPath}, scriptEnv, ""); err != nil {
				return fmt.Errorf("pre-upgrade script %s failed: %w", scriptPath, err)
			}

			logging.Logger.Info().
				Str("script", scriptPath).
				Msg("Pre-upgrade script completed successfully")
		} else {
			logging.Logger.Debug().
				Str("script", scriptPath).
				Msg("Pre-upgrade script not found on disk, skipping")
		}
	}

	// --- Upgrade to current on-disk chart ---
	logging.Logger.Info().
		Str("action", "upgrade").
		Str("chartPath", entry.ChartPath).
		Msg("Upgrading to current chart version")

	upgradeFlags := *flags
	upgradeFlags.Selection.UpgradeFlow = true            // Ensure base-upgrade.yaml is included.
	upgradeFlags.Deployment.DeleteNamespaceFirst = false // Namespace must already exist from prior install.
	upgradeFlags.Deployment.ExtraHelmArgs = []string{"--force"}
	upgradeFlags.Deployment.ExtraHelmSets = map[string]string{
		"orchestration.upgrade.allowPreReleaseImages": "true",
	}

	// Extract Bitnami PostgreSQL passwords from the cluster secret and merge into --set overrides.
	// This prevents the PASSWORDS ERROR that Bitnami's secrets.yaml triggers during `helm upgrade --force`
	// when the Secret lookup returns nil (due to --force deleting/recreating resources).
	if pgPasswords := extractBitnamiPGPasswords(ctx, flags.EffectiveNamespace(), flags.Test.KubeContext); len(pgPasswords) > 0 {
		for k, v := range pgPasswords {
			upgradeFlags.Deployment.ExtraHelmSets[k] = v
		}
	}

	if err := deploy.Execute(ctx, &upgradeFlags); err != nil {
		return fmt.Errorf("upgrade-only: upgrade to local chart failed: %w", err)
	}

	logging.Logger.Info().
		Str("flow", entry.Flow).
		Msg("Upgrade-only flow completed successfully")

	return nil
}

// PrintRunSummary outputs a summary of all run results including per-entry timings.
// wallClock is the actual elapsed wall-clock duration for the entire matrix run.
// When entries run in parallel, this will be less than the sum of individual entry durations.
func PrintRunSummary(results []RunResult, wallClock time.Duration) string {
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
			}
		}
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
