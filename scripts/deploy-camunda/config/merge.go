package config

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// DebugConfig holds debug configuration for a component.
type DebugConfig struct {
	Port int
}

// ChartFlags holds chart-related configuration.
type ChartFlags struct {
	ChartPath            string
	Chart                string
	ChartVersion         string
	SkipDependencyUpdate bool
	RepoRoot             string
	// ChartRootOverlays lists chart-root overlay files to apply, in order.
	// Each name resolves to <chartPath>/values-<name>.yaml if the file exists.
	// Common values: "enterprise" (CVE-patched images), "digest" (SHA256 pins),
	// "latest", "local", "bitnami-legacy".
	// In the matrix runner, "digest" is always included; "enterprise" is added
	// when the ci-test-config entry has enterprise: true.
	ChartRootOverlays []string
}

// DeploymentFlags holds deployment-related configuration.
type DeploymentFlags struct {
	Namespace            string
	NamespacePrefix      string // Prefix to prepend to namespace (e.g., "distribution" for EKS)
	Release              string
	Scenario             string   // Single scenario or comma-separated list
	Scenarios            []string // Parsed list of scenarios (populated by Validate)
	ScenarioPath         string
	Platform             string
	Flow                 string
	Timeout              int // Timeout in minutes for Helm deployment
	DeleteNamespaceFirst bool
	RenderTemplates      bool
	RenderOutputDir      string
	ExtraValues          []string
	// Extra helm arguments for advanced use cases (e.g., upgrade flows).
	// These are appended to the helm command after all other arguments.
	ExtraHelmArgs []string
	// Extra --set pairs for helm (e.g., {"orchestration.upgrade.allowPreReleaseImages": "true"}).
	ExtraHelmSets map[string]string
}

// IngressFlags holds ingress-related configuration.
type IngressFlags struct {
	IngressSubdomain  string
	IngressBaseDomain string
	IngressHostname   string
}

// ResolveIngressHostname returns the resolved ingress hostname.
// If IngressHostname is set, it takes precedence (full override).
// Otherwise, IngressSubdomain is appended to IngressBaseDomain.
func (f *IngressFlags) ResolveIngressHostname() string {
	if f.IngressHostname != "" {
		return f.IngressHostname
	}
	if f.IngressSubdomain != "" && f.IngressBaseDomain != "" {
		return f.IngressSubdomain + "." + f.IngressBaseDomain
	}
	return ""
}

// AuthFlags holds authentication/Keycloak configuration.
type AuthFlags struct {
	Auth             string
	KeycloakHost     string
	KeycloakProtocol string
	KeycloakRealm    string
}

// DockerFlags holds Docker registry configuration.
type DockerFlags struct {
	DockerUsername       string
	DockerPassword       string
	EnsureDockerRegistry bool
	DockerHubUsername    string
	DockerHubPassword    string
	EnsureDockerHub      bool
	// SkipDockerLogin skips the `docker login` step inside the deployer.
	// The matrix runner sets this to true after performing docker login once
	// before parallel dispatch — preventing concurrent keychain writes.
	SkipDockerLogin bool
}

// SecretsFlags holds secrets-related configuration.
type SecretsFlags struct {
	ExternalSecrets       bool
	ExternalSecretsStore  string
	VaultSecretMapping    string
	AutoGenerateSecrets   bool
	UseVaultBackedSecrets bool
}

// DebugFlags holds JVM debug configuration.
type DebugFlags struct {
	DebugComponents map[string]DebugConfig // Components to enable JVM debugging for, with their ports
	DebugPort       int                    // Default JVM debug port (used when no port specified)
	DebugSuspend    bool                   // Suspend JVM on startup until debugger attaches
}

// TestFlags holds test execution configuration.
type TestFlags struct {
	RunIntegrationTests bool   // Run integration tests after deployment
	RunE2ETests         bool   // Run e2e tests after deployment
	RunAllTests         bool   // Run both integration and e2e tests after deployment
	TestExclude         string // Pipe-separated regex for test suites to exclude (passed as --grep-invert to Playwright)
	OutputTestEnv       bool   // Generate .env file for E2E tests after deployment
	OutputTestEnvPath   string // Path for the test .env file output
	KubeContext         string
}

// SelectionFlags holds selection + composition model flags.
type SelectionFlags struct {
	Identity     string   // Identity selection: keycloak, keycloak-external, oidc, basic, hybrid
	Persistence  string   // Persistence selection: elasticsearch, opensearch, rdbms, rdbms-oracle
	TestPlatform string   // Test platform selection: gke, eks, openshift
	Features     []string // Feature selections: multitenancy, rba, documentstore
	QA           bool     // Enable QA configuration (test users, etc.)
	ImageTags    bool     // Enable image tag overrides from env vars
	UpgradeFlow  bool     // Enable upgrade flow configuration
	InfraType    string   // Infrastructure pool type (e.g., preemptible, distroci, standard, arm)
}

// HasExplicitSelectionConfig returns true if any selection + composition flags were explicitly set.
func (f *SelectionFlags) HasExplicitSelectionConfig() bool {
	return f.Identity != "" || f.Persistence != "" || f.TestPlatform != "" || len(f.Features) > 0 || f.QA || f.ImageTags || f.UpgradeFlow
}

// DeprecatedFlags holds deprecated layered values flags (backward compat).
type DeprecatedFlags struct {
	ValuesAuth     string   // DEPRECATED: use Identity instead
	ValuesBackend  string   // DEPRECATED: use Persistence instead
	ValuesFeatures []string // DEPRECATED: use Features instead
	ValuesQA       bool     // DEPRECATED: use QA instead
	ValuesInfra    string   // DEPRECATED: use TestPlatform instead
}

// HasExplicitLayeredConfig returns true if any deprecated layered values flags were explicitly set.
// Deprecated: Use SelectionFlags.HasExplicitSelectionConfig instead.
func (f *DeprecatedFlags) HasExplicitLayeredConfig() bool {
	return f.ValuesAuth != "" || f.ValuesBackend != "" || len(f.ValuesFeatures) > 0 || f.ValuesQA || f.ValuesInfra != ""
}

// IndexPrefixFlags holds Elasticsearch/OpenSearch index prefix configuration.
type IndexPrefixFlags struct {
	OptimizeIndexPrefix      string
	OrchestrationIndexPrefix string
	TasklistIndexPrefix      string
	OperateIndexPrefix       string
}

// RuntimeFlags holds all CLI flag values that can be merged with config.
// Fields are grouped into composed sub-structs by domain concern.
type RuntimeFlags struct {
	Chart      ChartFlags
	Deployment DeploymentFlags
	Ingress    IngressFlags
	Auth       AuthFlags
	Docker     DockerFlags
	Secrets    SecretsFlags
	Debug      DebugFlags
	Test       TestFlags
	Selection  SelectionFlags
	Deprecated DeprecatedFlags
	Index      IndexPrefixFlags

	// Cross-cutting / runtime fields that don't belong to a single domain.
	LogLevel    string
	EnvFile     string
	Interactive bool

	// ESPoolIndex specifies which Elasticsearch pool to target (e.g., "0", "1", "2", or "3").
	// When set, this value is used directly for $ES_POOL_INDEX substitution in values files.
	// When empty, prepareScenarioValues falls back to the ES_POOL_INDEX env var, then "0".
	ESPoolIndex string

	// ChangedFlags tracks which CLI flags were explicitly set by the user.
	// When populated, merge functions will not overwrite these flags with
	// config-file values. This is essential for boolean flags whose zero
	// value (false) is a valid explicit choice (e.g., --skip-dependency-update=false).
	ChangedFlags map[string]bool

	// PreInstallHooks are functions called by the deployer after namespace and
	// registry secrets are set up but before helm upgrade/install. This allows
	// callers (e.g., the matrix runner) to create K8s resources that must
	// exist at install time but cannot be created earlier because the namespace
	// may not yet exist or may be recreated by DeleteNamespaceFirst.
	PreInstallHooks []func(ctx context.Context) error

	// ExtraEnv holds per-entry environment variables that are merged into the
	// isolated env map by buildScenarioEnv before values.Process() runs.
	// This avoids process-global os.Setenv races when multiple OIDC entries
	// run concurrently — each entry carries its own VENOM_CLIENT_ID and
	// CONNECTORS_CLIENT_ID in an isolated map instead of relying on os.Setenv.
	ExtraEnv map[string]string
}

// ParseDebugFlag parses a debug flag value in the format "component" or "component:port".
// Returns the component name and port (using defaultPort if not specified).
func ParseDebugFlag(value string, defaultPort int) (string, int, error) {
	parts := strings.SplitN(value, ":", 2)
	component := strings.ToLower(strings.TrimSpace(parts[0]))
	if component == "" {
		return "", 0, fmt.Errorf("empty component name")
	}

	port := defaultPort
	if len(parts) == 2 {
		var err error
		port, err = strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil {
			return "", 0, fmt.Errorf("invalid port %q: %w", parts[1], err)
		}
		if port < 1 || port > 65535 {
			return "", 0, fmt.Errorf("port %d out of range (1-65535)", port)
		}
	}

	return component, port, nil
}

// ApplyActiveDeployment merges active deployment and root config into runtime flags.
func ApplyActiveDeployment(rc *RootConfig, active string, flags *RuntimeFlags) error {
	if rc == nil || rc.Deployments == nil {
		return applyRootDefaults(rc, flags)
	}

	// Auto-select if exactly one deployment exists
	if strings.TrimSpace(active) == "" && len(rc.Deployments) == 1 {
		for name := range rc.Deployments {
			active = name
		}
	}

	if strings.TrimSpace(active) == "" {
		return applyRootDefaults(rc, flags)
	}

	dep, ok := rc.Deployments[active]
	if !ok {
		return fmt.Errorf("active deployment %q not found in config", active)
	}

	// Apply deployment-specific values — string fields are skipped if the
	// user explicitly set the corresponding flag on the CLI.
	changed := flags.ChangedFlags
	MergeStringField(&flags.Chart.ChartPath, dep.ChartPath, rc.ChartPath, changed, "chart-path")
	MergeStringField(&flags.Chart.Chart, dep.Chart, rc.Chart, changed, "chart")
	MergeStringField(&flags.Chart.ChartVersion, dep.Version, rc.Version, changed, "version")
	MergeStringField(&flags.Deployment.Namespace, dep.Namespace, rc.Namespace, changed, "namespace")
	MergeStringField(&flags.Deployment.NamespacePrefix, dep.NamespacePrefix, rc.NamespacePrefix, changed, "namespace-prefix")
	MergeStringField(&flags.Deployment.Release, dep.Release, rc.Release, changed, "release")
	MergeStringField(&flags.Deployment.Scenario, dep.Scenario, rc.Scenario, changed, "scenario")
	MergeStringField(&flags.Auth.Auth, dep.Auth, rc.Auth, changed, "auth")
	MergeStringField(&flags.Deployment.Platform, dep.Platform, rc.Platform, changed, "platform")
	MergeStringField(&flags.LogLevel, dep.LogLevel, rc.LogLevel, changed, "log-level")
	MergeStringField(&flags.Deployment.Flow, dep.Flow, rc.Flow, changed, "flow")
	MergeStringField(&flags.EnvFile, dep.EnvFile, rc.EnvFile, changed, "env-file")
	MergeStringField(&flags.Secrets.VaultSecretMapping, dep.VaultSecretMapping, rc.VaultSecretMapping, changed, "vault-secret-mapping")
	MergeStringField(&flags.Docker.DockerUsername, dep.DockerUsername, rc.DockerUsername, changed, "docker-username")
	MergeStringField(&flags.Docker.DockerPassword, dep.DockerPassword, rc.DockerPassword, changed, "docker-password")
	MergeStringField(&flags.Docker.DockerHubUsername, dep.DockerHubUsername, rc.DockerHubUsername, changed, "dockerhub-username")
	MergeStringField(&flags.Docker.DockerHubPassword, dep.DockerHubPassword, rc.DockerHubPassword, changed, "dockerhub-password")
	MergeStringField(&flags.Deployment.RenderOutputDir, dep.RenderOutputDir, rc.RenderOutputDir, changed, "render-output-dir")
	MergeStringField(&flags.Chart.RepoRoot, dep.RepoRoot, rc.RepoRoot, changed, "repo-root")
	// ChartRootOverlays: merge from config's ValuesPreset (comma-separated string → []string).
	// CLI --values-preset sets ChartRootOverlays directly; config files provide ValuesPreset as a string.
	if !(changed != nil && changed["values-preset"]) && len(flags.Chart.ChartRootOverlays) == 0 {
		if presetStr := FirstNonEmpty(dep.ValuesPreset, rc.ValuesPreset); presetStr != "" {
			for _, p := range strings.Split(presetStr, ",") {
				if t := strings.TrimSpace(p); t != "" {
					flags.Chart.ChartRootOverlays = append(flags.Chart.ChartRootOverlays, t)
				}
			}
		}
	}
	MergeStringField(&flags.Auth.KeycloakRealm, dep.KeycloakRealm, rc.KeycloakRealm, changed, "keycloak-realm")
	MergeStringField(&flags.Index.OptimizeIndexPrefix, dep.OptimizeIndexPrefix, rc.OptimizeIndexPrefix, changed, "optimize-index-prefix")
	MergeStringField(&flags.Index.OrchestrationIndexPrefix, dep.OrchestrationIndexPrefix, rc.OrchestrationIndexPrefix, changed, "orchestration-index-prefix")
	MergeStringField(&flags.Index.TasklistIndexPrefix, dep.TasklistIndexPrefix, rc.TasklistIndexPrefix, changed, "tasklist-index-prefix")
	MergeStringField(&flags.Index.OperateIndexPrefix, dep.OperateIndexPrefix, rc.OperateIndexPrefix, changed, "operate-index-prefix")
	MergeStringField(&flags.Test.KubeContext, dep.KubeContext, rc.KubeContext, changed, "kube-context")
	MergeStringField(&flags.Ingress.IngressHostname, dep.IngressHost, rc.IngressHost, changed, "ingress-hostname")
	MergeStringField(&flags.Ingress.IngressSubdomain, dep.IngressSubdomain, rc.IngressSubdomain, changed, "ingress-subdomain")
	MergeStringField(&flags.Ingress.IngressBaseDomain, dep.IngressBaseDomain, rc.IngressBaseDomain, changed, "ingress-base-domain")
	MergeStringField(&flags.Secrets.ExternalSecretsStore, "", "", changed, "external-secrets-store") // No config file support yet

	// ScenarioPath special handling — has multiple config sources
	if !(changed != nil && changed["scenario-path"]) && strings.TrimSpace(flags.Deployment.ScenarioPath) == "" {
		flags.Deployment.ScenarioPath = FirstNonEmpty(dep.ScenarioPath, dep.ScenarioRoot, rc.ScenarioPath, rc.ScenarioRoot)
	}

	// Boolean fields - skip if the user explicitly set the flag on the CLI
	MergeBoolField(&flags.Secrets.ExternalSecrets, dep.ExternalSecrets, rc.ExternalSecrets, changed, "external-secrets")
	MergeBoolField(&flags.Chart.SkipDependencyUpdate, dep.SkipDependencyUpdate, rc.SkipDependencyUpdate, changed, "skip-dependency-update")
	MergeBoolField(&flags.Interactive, dep.Interactive, rc.Interactive, changed, "interactive")
	MergeBoolField(&flags.Secrets.AutoGenerateSecrets, dep.AutoGenerateSecrets, rc.AutoGenerateSecrets, changed, "auto-generate-secrets")
	MergeBoolField(&flags.Deployment.DeleteNamespaceFirst, dep.DeleteNamespace, rc.DeleteNamespace, changed, "delete-namespace")
	MergeBoolField(&flags.Docker.EnsureDockerRegistry, dep.EnsureDockerRegistry, rc.EnsureDockerRegistry, changed, "ensure-docker-registry")
	MergeBoolField(&flags.Docker.EnsureDockerHub, dep.EnsureDockerHub, rc.EnsureDockerHub, changed, "ensure-docker-hub")
	MergeBoolField(&flags.Deployment.RenderTemplates, dep.RenderTemplates, rc.RenderTemplates, changed, "render-templates")

	// Test execution flags
	MergeBoolField(&flags.Test.RunIntegrationTests, dep.RunIntegrationTests, rc.RunIntegrationTests, changed, "test-it")
	MergeBoolField(&flags.Test.RunE2ETests, dep.RunE2ETests, rc.RunE2ETests, changed, "test-e2e")

	// Slice fields
	MergeStringSliceField(&flags.Deployment.ExtraValues, dep.ExtraValues, rc.ExtraValues)

	// Keycloak
	MergeStringField(&flags.Auth.KeycloakHost, "", rc.Keycloak.Host, changed, "keycloak-host")
	MergeStringField(&flags.Auth.KeycloakProtocol, "", rc.Keycloak.Protocol, changed, "keycloak-protocol")

	return nil
}

// applyRootDefaults applies only root-level defaults when no deployment is active.
func applyRootDefaults(rc *RootConfig, flags *RuntimeFlags) error {
	if rc == nil {
		return nil
	}

	changed := flags.ChangedFlags
	MergeStringField(&flags.Chart.ChartPath, "", rc.ChartPath, changed, "chart-path")
	MergeStringField(&flags.Chart.Chart, "", rc.Chart, changed, "chart")
	MergeStringField(&flags.Chart.ChartVersion, "", rc.Version, changed, "version")
	MergeStringField(&flags.Deployment.Namespace, "", rc.Namespace, changed, "namespace")
	MergeStringField(&flags.Deployment.NamespacePrefix, "", rc.NamespacePrefix, changed, "namespace-prefix")
	MergeStringField(&flags.Deployment.Release, "", rc.Release, changed, "release")
	MergeStringField(&flags.Deployment.Scenario, "", rc.Scenario, changed, "scenario")
	MergeStringField(&flags.Deployment.ScenarioPath, "", FirstNonEmpty(rc.ScenarioPath, rc.ScenarioRoot), changed, "scenario-path")
	MergeStringField(&flags.Auth.Auth, "", rc.Auth, changed, "auth")
	MergeStringField(&flags.Deployment.Platform, "", rc.Platform, changed, "platform")
	MergeStringField(&flags.LogLevel, "", rc.LogLevel, changed, "log-level")
	MergeStringField(&flags.Deployment.Flow, "", rc.Flow, changed, "flow")
	MergeStringField(&flags.EnvFile, "", rc.EnvFile, changed, "env-file")
	MergeStringField(&flags.Secrets.VaultSecretMapping, "", rc.VaultSecretMapping, changed, "vault-secret-mapping")
	MergeStringField(&flags.Docker.DockerUsername, "", rc.DockerUsername, changed, "docker-username")
	MergeStringField(&flags.Docker.DockerPassword, "", rc.DockerPassword, changed, "docker-password")
	MergeStringField(&flags.Docker.DockerHubUsername, "", rc.DockerHubUsername, changed, "dockerhub-username")
	MergeStringField(&flags.Docker.DockerHubPassword, "", rc.DockerHubPassword, changed, "dockerhub-password")
	MergeStringField(&flags.Deployment.RenderOutputDir, "", rc.RenderOutputDir, changed, "render-output-dir")
	MergeStringField(&flags.Chart.RepoRoot, "", rc.RepoRoot, changed, "repo-root")
	// ChartRootOverlays: merge from root config's ValuesPreset (comma-separated string → []string).
	if !(changed != nil && changed["values-preset"]) && len(flags.Chart.ChartRootOverlays) == 0 {
		if presetStr := rc.ValuesPreset; presetStr != "" {
			for _, p := range strings.Split(presetStr, ",") {
				if t := strings.TrimSpace(p); t != "" {
					flags.Chart.ChartRootOverlays = append(flags.Chart.ChartRootOverlays, t)
				}
			}
		}
	}
	MergeStringField(&flags.Auth.KeycloakRealm, "", rc.KeycloakRealm, changed, "keycloak-realm")
	MergeStringField(&flags.Index.OptimizeIndexPrefix, "", rc.OptimizeIndexPrefix, changed, "optimize-index-prefix")
	MergeStringField(&flags.Index.OrchestrationIndexPrefix, "", rc.OrchestrationIndexPrefix, changed, "orchestration-index-prefix")
	MergeStringField(&flags.Index.TasklistIndexPrefix, "", rc.TasklistIndexPrefix, changed, "tasklist-index-prefix")
	MergeStringField(&flags.Index.OperateIndexPrefix, "", rc.OperateIndexPrefix, changed, "operate-index-prefix")
	MergeStringField(&flags.Test.KubeContext, "", rc.KubeContext, changed, "kube-context")
	MergeStringField(&flags.Ingress.IngressHostname, "", rc.IngressHost, changed, "ingress-hostname")
	MergeStringField(&flags.Ingress.IngressSubdomain, "", rc.IngressSubdomain, changed, "ingress-subdomain")
	MergeStringField(&flags.Ingress.IngressBaseDomain, "", rc.IngressBaseDomain, changed, "ingress-base-domain")
	MergeStringField(&flags.Secrets.ExternalSecretsStore, "", "", changed, "external-secrets-store") // No config file support yet

	MergeBoolField(&flags.Secrets.ExternalSecrets, nil, rc.ExternalSecrets, changed, "external-secrets")
	MergeBoolField(&flags.Chart.SkipDependencyUpdate, nil, rc.SkipDependencyUpdate, changed, "skip-dependency-update")

	MergeBoolField(&flags.Interactive, nil, rc.Interactive, changed, "interactive")
	MergeBoolField(&flags.Secrets.AutoGenerateSecrets, nil, rc.AutoGenerateSecrets, changed, "auto-generate-secrets")
	MergeBoolField(&flags.Deployment.DeleteNamespaceFirst, nil, rc.DeleteNamespace, changed, "delete-namespace")
	MergeBoolField(&flags.Docker.EnsureDockerRegistry, nil, rc.EnsureDockerRegistry, changed, "ensure-docker-registry")
	MergeBoolField(&flags.Docker.EnsureDockerHub, nil, rc.EnsureDockerHub, changed, "ensure-docker-hub")
	MergeBoolField(&flags.Deployment.RenderTemplates, nil, rc.RenderTemplates, changed, "render-templates")

	// Test execution flags
	MergeBoolField(&flags.Test.RunIntegrationTests, nil, rc.RunIntegrationTests, changed, "test-it")
	MergeBoolField(&flags.Test.RunE2ETests, nil, rc.RunE2ETests, changed, "test-e2e")

	MergeStringSliceField(&flags.Deployment.ExtraValues, nil, rc.ExtraValues)

	MergeStringField(&flags.Auth.KeycloakHost, "", rc.Keycloak.Host, changed, "keycloak-host")
	MergeStringField(&flags.Auth.KeycloakProtocol, "", rc.Keycloak.Protocol, changed, "keycloak-protocol")

	return nil
}

// Validate performs validation on the merged runtime flags.
func Validate(flags *RuntimeFlags) error {
	// Ensure at least one of chart-path or chart is provided
	if flags.Chart.ChartPath == "" && flags.Chart.Chart == "" {
		return fmt.Errorf("either --chart-path or --chart must be provided")
	}

	// Validate --version compatibility
	if strings.TrimSpace(flags.Chart.ChartVersion) != "" && strings.TrimSpace(flags.Chart.Chart) == "" && strings.TrimSpace(flags.Chart.ChartPath) != "" {
		return fmt.Errorf("--version requires --chart to be set; do not combine --version with only --chart-path")
	}
	if strings.TrimSpace(flags.Chart.ChartVersion) != "" && strings.TrimSpace(flags.Chart.Chart) == "" && strings.TrimSpace(flags.Chart.ChartPath) == "" {
		return fmt.Errorf("--version requires --chart to be set")
	}

	// Validate required runtime identifiers
	if strings.TrimSpace(flags.Deployment.Namespace) == "" {
		return fmt.Errorf("namespace not set; provide -n/--namespace or set 'namespace' in the active deployment/root config")
	}
	if strings.TrimSpace(flags.Deployment.Release) == "" {
		return fmt.Errorf("release not set; provide -r/--release or set 'release' in the active deployment/root config")
	}
	if strings.TrimSpace(flags.Deployment.Scenario) == "" {
		return fmt.Errorf("scenario not set; provide -s/--scenario or set 'scenario' in the active deployment/root config")
	}

	// Parse scenarios from comma-separated string
	flags.Deployment.Scenarios = parseScenarios(flags.Deployment.Scenario)
	if len(flags.Deployment.Scenarios) == 0 {
		return fmt.Errorf("no valid scenarios found in %q", flags.Deployment.Scenario)
	}

	// Validate ingress configuration
	// --ingress-hostname is mutually exclusive with --ingress-subdomain and --ingress-base-domain
	if flags.Ingress.IngressHostname != "" && (flags.Ingress.IngressSubdomain != "" || flags.Ingress.IngressBaseDomain != "") {
		return fmt.Errorf("--ingress-hostname cannot be used with --ingress-subdomain or --ingress-base-domain; use either --ingress-hostname OR --ingress-subdomain with --ingress-base-domain")
	}
	if flags.Ingress.IngressSubdomain != "" && flags.Ingress.IngressBaseDomain == "" {
		return fmt.Errorf("--ingress-base-domain is required when using --ingress-subdomain; valid values: %s", strings.Join(ValidIngressBaseDomains, ", "))
	}
	if flags.Ingress.IngressBaseDomain != "" {
		if !IsValidIngressBaseDomain(flags.Ingress.IngressBaseDomain) {
			return fmt.Errorf("--ingress-base-domain must be one of: %s", strings.Join(ValidIngressBaseDomains, ", "))
		}
	}

	return nil
}

// parseScenarios splits a comma-separated scenario string into a slice.
func parseScenarios(scenario string) []string {
	var scenarios []string
	for _, s := range strings.Split(scenario, ",") {
		s = strings.TrimSpace(s)
		if s != "" {
			scenarios = append(scenarios, s)
		}
	}
	return scenarios
}

// IsValidIngressBaseDomain checks if the given domain is in the allowed list.
func IsValidIngressBaseDomain(domain string) bool {
	for _, valid := range ValidIngressBaseDomains {
		if domain == valid {
			return true
		}
	}
	return false
}

// HasExplicitSelectionConfig returns true if any selection + composition flags were explicitly set.
// Delegates to SelectionFlags.
func (f *RuntimeFlags) HasExplicitSelectionConfig() bool {
	return f.Selection.HasExplicitSelectionConfig()
}

// HasExplicitLayeredConfig returns true if any deprecated layered values flags were explicitly set.
// Deprecated: Use HasExplicitSelectionConfig instead.
func (f *RuntimeFlags) HasExplicitLayeredConfig() bool {
	return f.Deprecated.HasExplicitLayeredConfig()
}

// MigrateDeprecatedFlags copies deprecated layered values flags to the new selection fields.
// This is called during validation to ensure backward compatibility.
func (f *RuntimeFlags) MigrateDeprecatedFlags() {
	// Only migrate if new fields are not already set
	if f.Selection.Identity == "" && f.Deprecated.ValuesAuth != "" {
		f.Selection.Identity = f.Deprecated.ValuesAuth
	}
	if f.Selection.Persistence == "" && f.Deprecated.ValuesBackend != "" {
		f.Selection.Persistence = f.Deprecated.ValuesBackend
	}
	if f.Selection.TestPlatform == "" && f.Deprecated.ValuesInfra != "" {
		f.Selection.TestPlatform = f.Deprecated.ValuesInfra
	}
	if len(f.Selection.Features) == 0 && len(f.Deprecated.ValuesFeatures) > 0 {
		// Filter out features that are now in other categories
		for _, feature := range f.Deprecated.ValuesFeatures {
			switch feature {
			case "rdbms", "rdbms-oracle":
				// These moved to persistence - only set if persistence not already set
				if f.Selection.Persistence == "" {
					f.Selection.Persistence = feature
				}
			case "upgrade":
				// This is now a separate flag
				f.Selection.UpgradeFlow = true
			default:
				f.Selection.Features = append(f.Selection.Features, feature)
			}
		}
	}
	if !f.Selection.QA && f.Deprecated.ValuesQA {
		f.Selection.QA = true
	}
}

// ResolveIngressHostname returns the resolved ingress hostname.
// Delegates to IngressFlags.
func (f *RuntimeFlags) ResolveIngressHostname() string {
	return f.Ingress.ResolveIngressHostname()
}

// EffectiveNamespace returns the namespace with the prefix applied if set.
// If NamespacePrefix is set, returns "prefix-namespace", otherwise just "namespace".
func (f *RuntimeFlags) EffectiveNamespace() string {
	if f.Deployment.NamespacePrefix != "" && f.Deployment.Namespace != "" {
		return f.Deployment.NamespacePrefix + "-" + f.Deployment.Namespace
	}
	return f.Deployment.Namespace
}

// LoadAndMerge loads config from the given path and merges the active deployment into flags.
// If configPath is empty, it resolves the default config location.
// The includeEnv parameter controls whether environment variable overrides are applied.
func LoadAndMerge(configPath string, includeEnv bool, flags *RuntimeFlags) (*RootConfig, error) {
	cfgPath, err := ResolvePath(configPath)
	if err != nil {
		return nil, err
	}
	rc, err := Read(cfgPath, includeEnv)
	if err != nil {
		return nil, err
	}
	if err := ApplyActiveDeployment(rc, rc.Current, flags); err != nil {
		return nil, err
	}
	return rc, nil
}
