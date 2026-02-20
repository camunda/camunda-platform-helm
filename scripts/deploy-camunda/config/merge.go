package config

import (
	"fmt"
	"strconv"
	"strings"
)

// DebugConfig holds debug configuration for a component.
type DebugConfig struct {
	Port int
}

// RuntimeFlags holds all CLI flag values that can be merged with config.
type RuntimeFlags struct {
	ChartPath                string
	Chart                    string
	ChartVersion             string
	Namespace                string
	NamespacePrefix          string // Prefix to prepend to namespace (e.g., "distribution" for EKS)
	Release                  string
	Scenario                 string   // Single scenario or comma-separated list
	Scenarios                []string // Parsed list of scenarios (populated by Validate)
	ScenarioPath             string
	Auth                     string
	Platform                 string
	LogLevel                 string
	SkipDependencyUpdate     bool
	ExternalSecrets          bool
	ExternalSecretsStore     string
	KeycloakHost             string
	KeycloakProtocol         string
	KeycloakRealm            string
	OptimizeIndexPrefix      string
	OrchestrationIndexPrefix string
	TasklistIndexPrefix      string
	OperateIndexPrefix       string
	IngressSubdomain         string
	IngressBaseDomain        string
	IngressHostname          string
	RepoRoot                 string
	Flow                     string
	EnvFile                  string
	Interactive              bool
	VaultSecretMapping       string
	AutoGenerateSecrets      bool
	DeleteNamespaceFirst     bool
	DockerUsername           string
	DockerPassword           string
	EnsureDockerRegistry     bool
	RenderTemplates          bool
	RenderOutputDir          string
	ExtraValues              []string
	ValuesPreset             string
	Timeout                  int                    // Timeout in minutes for Helm deployment
	DebugComponents          map[string]DebugConfig // Components to enable JVM debugging for, with their ports
	DebugPort                int                    // Default JVM debug port (used when no port specified)
	DebugSuspend             bool                   // Suspend JVM on startup until debugger attaches
	OutputTestEnv            bool                   // Generate .env file for E2E tests after deployment
	OutputTestEnvPath        string                 // Path for the test .env file output
	// Selection + composition flags (new model)
	Identity     string   // Identity selection: keycloak, keycloak-external, oidc, basic, hybrid
	Persistence  string   // Persistence selection: elasticsearch, opensearch, rdbms, rdbms-oracle
	TestPlatform string   // Test platform selection: gke, eks, openshift
	Features     []string // Feature selections: multitenancy, rba, documentstore
	QA           bool     // Enable QA configuration (test users, etc.)
	ImageTags    bool     // Enable image tag overrides from env vars
	UpgradeFlow  bool     // Enable upgrade flow configuration

	// Deprecated layered values flags (backward compat)
	ValuesAuth     string   // DEPRECATED: use Identity instead
	ValuesBackend  string   // DEPRECATED: use Persistence instead
	ValuesFeatures []string // DEPRECATED: use Features instead
	ValuesQA       bool     // DEPRECATED: use QA instead
	ValuesInfra    string   // DEPRECATED: use TestPlatform instead

	// Test execution flags
	RunIntegrationTests   bool // Run integration tests after deployment
	RunE2ETests           bool // Run e2e tests after deployment
	RunAllTests           bool // Run both integration and e2e tests after deployment
	KubeContext           string
	UseVaultBackedSecrets bool
	TestExclude           string // Comma-separated test suites to exclude (passed as --test-exclude to test scripts)

	// ChangedFlags tracks which CLI flags were explicitly set by the user.
	// When populated, merge functions will not overwrite these flags with
	// config-file values. This is essential for boolean flags whose zero
	// value (false) is a valid explicit choice (e.g., --skip-dependency-update=false).
	ChangedFlags map[string]bool
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
	MergeStringField(&flags.ChartPath, dep.ChartPath, rc.ChartPath, changed, "chart-path")
	MergeStringField(&flags.Chart, dep.Chart, rc.Chart, changed, "chart")
	MergeStringField(&flags.ChartVersion, dep.Version, rc.Version, changed, "version")
	MergeStringField(&flags.Namespace, dep.Namespace, rc.Namespace, changed, "namespace")
	MergeStringField(&flags.NamespacePrefix, dep.NamespacePrefix, rc.NamespacePrefix, changed, "namespace-prefix")
	MergeStringField(&flags.Release, dep.Release, rc.Release, changed, "release")
	MergeStringField(&flags.Scenario, dep.Scenario, rc.Scenario, changed, "scenario")
	MergeStringField(&flags.Auth, dep.Auth, rc.Auth, changed, "auth")
	MergeStringField(&flags.Platform, dep.Platform, rc.Platform, changed, "platform")
	MergeStringField(&flags.LogLevel, dep.LogLevel, rc.LogLevel, changed, "log-level")
	MergeStringField(&flags.Flow, dep.Flow, rc.Flow, changed, "flow")
	MergeStringField(&flags.EnvFile, dep.EnvFile, rc.EnvFile, changed, "env-file")
	MergeStringField(&flags.VaultSecretMapping, dep.VaultSecretMapping, rc.VaultSecretMapping, changed, "vault-secret-mapping")
	MergeStringField(&flags.DockerUsername, dep.DockerUsername, rc.DockerUsername, changed, "docker-username")
	MergeStringField(&flags.DockerPassword, dep.DockerPassword, rc.DockerPassword, changed, "docker-password")
	MergeStringField(&flags.RenderOutputDir, dep.RenderOutputDir, rc.RenderOutputDir, changed, "render-output-dir")
	MergeStringField(&flags.RepoRoot, dep.RepoRoot, rc.RepoRoot, changed, "repo-root")
	MergeStringField(&flags.ValuesPreset, dep.ValuesPreset, rc.ValuesPreset, changed, "values-preset")
	MergeStringField(&flags.KeycloakRealm, dep.KeycloakRealm, rc.KeycloakRealm, changed, "keycloak-realm")
	MergeStringField(&flags.OptimizeIndexPrefix, dep.OptimizeIndexPrefix, rc.OptimizeIndexPrefix, changed, "optimize-index-prefix")
	MergeStringField(&flags.OrchestrationIndexPrefix, dep.OrchestrationIndexPrefix, rc.OrchestrationIndexPrefix, changed, "orchestration-index-prefix")
	MergeStringField(&flags.TasklistIndexPrefix, dep.TasklistIndexPrefix, rc.TasklistIndexPrefix, changed, "tasklist-index-prefix")
	MergeStringField(&flags.OperateIndexPrefix, dep.OperateIndexPrefix, rc.OperateIndexPrefix, changed, "operate-index-prefix")
	MergeStringField(&flags.KubeContext, dep.KubeContext, rc.KubeContext, changed, "kube-context")
	MergeStringField(&flags.IngressHostname, dep.IngressHost, rc.IngressHost, changed, "ingress-hostname")
	MergeStringField(&flags.IngressSubdomain, dep.IngressSubdomain, rc.IngressSubdomain, changed, "ingress-subdomain")
	MergeStringField(&flags.IngressBaseDomain, dep.IngressBaseDomain, rc.IngressBaseDomain, changed, "ingress-base-domain")
	MergeStringField(&flags.ExternalSecretsStore, "", "", changed, "external-secrets-store") // No config file support yet

	// ScenarioPath special handling — has multiple config sources
	if !(changed != nil && changed["scenario-path"]) && strings.TrimSpace(flags.ScenarioPath) == "" {
		flags.ScenarioPath = FirstNonEmpty(dep.ScenarioPath, dep.ScenarioRoot, rc.ScenarioPath, rc.ScenarioRoot)
	}

	// Boolean fields - skip if the user explicitly set the flag on the CLI
	MergeBoolField(&flags.ExternalSecrets, dep.ExternalSecrets, boolPtr(rc.ExternalSecrets), changed, "external-secrets")
	MergeBoolField(&flags.SkipDependencyUpdate, dep.SkipDependencyUpdate, boolPtr(rc.SkipDependencyUpdate), changed, "skip-dependency-update")
	MergeBoolField(&flags.Interactive, dep.Interactive, rc.Interactive, changed, "interactive")
	MergeBoolField(&flags.AutoGenerateSecrets, dep.AutoGenerateSecrets, rc.AutoGenerateSecrets, changed, "auto-generate-secrets")
	MergeBoolField(&flags.DeleteNamespaceFirst, dep.DeleteNamespace, rc.DeleteNamespaceFirst, changed, "delete-namespace")
	MergeBoolField(&flags.EnsureDockerRegistry, dep.EnsureDockerRegistry, rc.EnsureDockerRegistry, changed, "ensure-docker-registry")
	MergeBoolField(&flags.RenderTemplates, dep.RenderTemplates, rc.RenderTemplates, changed, "render-templates")

	// Test execution flags
	MergeBoolField(&flags.RunIntegrationTests, dep.RunIntegrationTests, rc.RunIntegrationTests, changed, "test-it")
	MergeBoolField(&flags.RunE2ETests, dep.RunE2ETests, rc.RunE2ETests, changed, "test-e2e")

	// Slice fields
	MergeStringSliceField(&flags.ExtraValues, dep.ExtraValues, rc.ExtraValues)

	// Keycloak
	MergeStringField(&flags.KeycloakHost, "", rc.Keycloak.Host, changed, "keycloak-host")
	MergeStringField(&flags.KeycloakProtocol, "", rc.Keycloak.Protocol, changed, "keycloak-protocol")

	return nil
}

// applyRootDefaults applies only root-level defaults when no deployment is active.
func applyRootDefaults(rc *RootConfig, flags *RuntimeFlags) error {
	if rc == nil {
		return nil
	}

	changed := flags.ChangedFlags
	MergeStringField(&flags.ChartPath, "", rc.ChartPath, changed, "chart-path")
	MergeStringField(&flags.Chart, "", rc.Chart, changed, "chart")
	MergeStringField(&flags.ChartVersion, "", rc.Version, changed, "version")
	MergeStringField(&flags.Namespace, "", rc.Namespace, changed, "namespace")
	MergeStringField(&flags.NamespacePrefix, "", rc.NamespacePrefix, changed, "namespace-prefix")
	MergeStringField(&flags.Release, "", rc.Release, changed, "release")
	MergeStringField(&flags.Scenario, "", rc.Scenario, changed, "scenario")
	MergeStringField(&flags.ScenarioPath, "", FirstNonEmpty(rc.ScenarioPath, rc.ScenarioRoot), changed, "scenario-path")
	MergeStringField(&flags.Auth, "", rc.Auth, changed, "auth")
	MergeStringField(&flags.Platform, "", rc.Platform, changed, "platform")
	MergeStringField(&flags.LogLevel, "", rc.LogLevel, changed, "log-level")
	MergeStringField(&flags.Flow, "", rc.Flow, changed, "flow")
	MergeStringField(&flags.EnvFile, "", rc.EnvFile, changed, "env-file")
	MergeStringField(&flags.VaultSecretMapping, "", rc.VaultSecretMapping, changed, "vault-secret-mapping")
	MergeStringField(&flags.DockerUsername, "", rc.DockerUsername, changed, "docker-username")
	MergeStringField(&flags.DockerPassword, "", rc.DockerPassword, changed, "docker-password")
	MergeStringField(&flags.RenderOutputDir, "", rc.RenderOutputDir, changed, "render-output-dir")
	MergeStringField(&flags.RepoRoot, "", rc.RepoRoot, changed, "repo-root")
	MergeStringField(&flags.ValuesPreset, "", rc.ValuesPreset, changed, "values-preset")
	MergeStringField(&flags.KeycloakRealm, "", rc.KeycloakRealm, changed, "keycloak-realm")
	MergeStringField(&flags.OptimizeIndexPrefix, "", rc.OptimizeIndexPrefix, changed, "optimize-index-prefix")
	MergeStringField(&flags.OrchestrationIndexPrefix, "", rc.OrchestrationIndexPrefix, changed, "orchestration-index-prefix")
	MergeStringField(&flags.TasklistIndexPrefix, "", rc.TasklistIndexPrefix, changed, "tasklist-index-prefix")
	MergeStringField(&flags.OperateIndexPrefix, "", rc.OperateIndexPrefix, changed, "operate-index-prefix")
	MergeStringField(&flags.KubeContext, "", rc.KubeContext, changed, "kube-context")
	MergeStringField(&flags.IngressHostname, "", rc.IngressHost, changed, "ingress-hostname")
	MergeStringField(&flags.IngressSubdomain, "", rc.IngressSubdomain, changed, "ingress-subdomain")
	MergeStringField(&flags.IngressBaseDomain, "", rc.IngressBaseDomain, changed, "ingress-base-domain")
	MergeStringField(&flags.ExternalSecretsStore, "", "", changed, "external-secrets-store") // No config file support yet

	MergeBoolField(&flags.ExternalSecrets, nil, boolPtr(rc.ExternalSecrets), changed, "external-secrets")
	MergeBoolField(&flags.SkipDependencyUpdate, nil, boolPtr(rc.SkipDependencyUpdate), changed, "skip-dependency-update")

	MergeBoolField(&flags.Interactive, nil, rc.Interactive, changed, "interactive")
	MergeBoolField(&flags.AutoGenerateSecrets, nil, rc.AutoGenerateSecrets, changed, "auto-generate-secrets")
	MergeBoolField(&flags.DeleteNamespaceFirst, nil, rc.DeleteNamespaceFirst, changed, "delete-namespace")
	MergeBoolField(&flags.EnsureDockerRegistry, nil, rc.EnsureDockerRegistry, changed, "ensure-docker-registry")
	MergeBoolField(&flags.RenderTemplates, nil, rc.RenderTemplates, changed, "render-templates")

	// Test execution flags
	MergeBoolField(&flags.RunIntegrationTests, nil, rc.RunIntegrationTests, changed, "test-it")
	MergeBoolField(&flags.RunE2ETests, nil, rc.RunE2ETests, changed, "test-e2e")

	MergeStringSliceField(&flags.ExtraValues, nil, rc.ExtraValues)

	MergeStringField(&flags.KeycloakHost, "", rc.Keycloak.Host, changed, "keycloak-host")
	MergeStringField(&flags.KeycloakProtocol, "", rc.Keycloak.Protocol, changed, "keycloak-protocol")

	return nil
}

// boolPtr returns a pointer to a bool value.
func boolPtr(b bool) *bool {
	return &b
}

// Validate performs validation on the merged runtime flags.
func Validate(flags *RuntimeFlags) error {
	// Ensure at least one of chart-path or chart is provided
	if flags.ChartPath == "" && flags.Chart == "" {
		return fmt.Errorf("either --chart-path or --chart must be provided")
	}

	// Validate --version compatibility
	if strings.TrimSpace(flags.ChartVersion) != "" && strings.TrimSpace(flags.Chart) == "" && strings.TrimSpace(flags.ChartPath) != "" {
		return fmt.Errorf("--version requires --chart to be set; do not combine --version with only --chart-path")
	}
	if strings.TrimSpace(flags.ChartVersion) != "" && strings.TrimSpace(flags.Chart) == "" && strings.TrimSpace(flags.ChartPath) == "" {
		return fmt.Errorf("--version requires --chart to be set")
	}

	// Validate required runtime identifiers
	if strings.TrimSpace(flags.Namespace) == "" {
		return fmt.Errorf("namespace not set; provide -n/--namespace or set 'namespace' in the active deployment/root config")
	}
	if strings.TrimSpace(flags.Release) == "" {
		return fmt.Errorf("release not set; provide -r/--release or set 'release' in the active deployment/root config")
	}
	if strings.TrimSpace(flags.Scenario) == "" {
		return fmt.Errorf("scenario not set; provide -s/--scenario or set 'scenario' in the active deployment/root config")
	}

	// Parse scenarios from comma-separated string
	flags.Scenarios = parseScenarios(flags.Scenario)
	if len(flags.Scenarios) == 0 {
		return fmt.Errorf("no valid scenarios found in %q", flags.Scenario)
	}

	// Validate ingress configuration
	// --ingress-hostname is mutually exclusive with --ingress-subdomain and --ingress-base-domain
	if flags.IngressHostname != "" && (flags.IngressSubdomain != "" || flags.IngressBaseDomain != "") {
		return fmt.Errorf("--ingress-hostname cannot be used with --ingress-subdomain or --ingress-base-domain; use either --ingress-hostname OR --ingress-subdomain with --ingress-base-domain")
	}
	if flags.IngressSubdomain != "" && flags.IngressBaseDomain == "" {
		return fmt.Errorf("--ingress-base-domain is required when using --ingress-subdomain; valid values: %s", strings.Join(ValidIngressBaseDomains, ", "))
	}
	if flags.IngressBaseDomain != "" {
		if !IsValidIngressBaseDomain(flags.IngressBaseDomain) {
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
func (f *RuntimeFlags) HasExplicitSelectionConfig() bool {
	return f.Identity != "" || f.Persistence != "" || f.TestPlatform != "" || len(f.Features) > 0 || f.QA || f.ImageTags || f.UpgradeFlow
}

// HasExplicitLayeredConfig returns true if any deprecated layered values flags were explicitly set.
// Deprecated: Use HasExplicitSelectionConfig instead.
func (f *RuntimeFlags) HasExplicitLayeredConfig() bool {
	return f.ValuesAuth != "" || f.ValuesBackend != "" || len(f.ValuesFeatures) > 0 || f.ValuesQA || f.ValuesInfra != ""
}

// MigrateDeprecatedFlags copies deprecated layered values flags to the new selection fields.
// This is called during validation to ensure backward compatibility.
func (f *RuntimeFlags) MigrateDeprecatedFlags() {
	// Only migrate if new fields are not already set
	if f.Identity == "" && f.ValuesAuth != "" {
		f.Identity = f.ValuesAuth
	}
	if f.Persistence == "" && f.ValuesBackend != "" {
		f.Persistence = f.ValuesBackend
	}
	if f.TestPlatform == "" && f.ValuesInfra != "" {
		f.TestPlatform = f.ValuesInfra
	}
	if len(f.Features) == 0 && len(f.ValuesFeatures) > 0 {
		// Filter out features that are now in other categories
		for _, feature := range f.ValuesFeatures {
			switch feature {
			case "rdbms", "rdbms-oracle":
				// These moved to persistence - only set if persistence not already set
				if f.Persistence == "" {
					f.Persistence = feature
				}
			case "upgrade":
				// This is now a separate flag
				f.UpgradeFlow = true
			default:
				f.Features = append(f.Features, feature)
			}
		}
	}
	if !f.QA && f.ValuesQA {
		f.QA = true
	}
}

// ResolveIngressHostname returns the resolved ingress hostname.
// If IngressHostname is set, it takes precedence (full override).
// Otherwise, IngressSubdomain is appended to IngressBaseDomain.
func (f *RuntimeFlags) ResolveIngressHostname() string {
	if f.IngressHostname != "" {
		return f.IngressHostname
	}
	if f.IngressSubdomain != "" && f.IngressBaseDomain != "" {
		return f.IngressSubdomain + "." + f.IngressBaseDomain
	}
	return ""
}

// EffectiveNamespace returns the namespace with the prefix applied if set.
// If NamespacePrefix is set, returns "prefix-namespace", otherwise just "namespace".
func (f *RuntimeFlags) EffectiveNamespace() string {
	if f.NamespacePrefix != "" && f.Namespace != "" {
		return f.NamespacePrefix + "-" + f.Namespace
	}
	return f.Namespace
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
