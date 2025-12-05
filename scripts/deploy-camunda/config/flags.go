package config

// ChartFlags holds chart-related configuration.
type ChartFlags struct {
	Path    string // Local path to chart directory
	Name    string // Remote chart name
	Version string // Chart version for remote charts
}

// DeploymentFlags holds deployment identifier configuration.
type DeploymentFlags struct {
	Namespace string   // Kubernetes namespace
	Release   string   // Helm release name
	Scenario  string   // Scenario name(s), comma-separated
	Scenarios []string // Parsed list of scenarios
	Auth      string   // Authentication scenario
}

// PlatformFlags holds platform and environment configuration.
type PlatformFlags struct {
	Platform string // Target platform (gke, rosa, eks)
	LogLevel string // Logging verbosity
	Flow     string // Deployment flow (install, upgrade)
	EnvFile  string // Path to .env file
}

// KeycloakFlags holds Keycloak-related configuration.
type KeycloakFlags struct {
	Host     string // Keycloak hostname
	Protocol string // Keycloak protocol (http, https)
	Realm    string // Keycloak realm name
}

// IndexPrefixFlags holds Elasticsearch index prefix configuration.
type IndexPrefixFlags struct {
	Optimize      string // Optimize index prefix
	Orchestration string // Orchestration index prefix
	Tasklist      string // Tasklist index prefix
	Operate       string // Operate index prefix
}

// SecretFlags holds secrets and authentication configuration.
type SecretFlags struct {
	ExternalSecrets    bool   // Enable External Secrets Operator
	VaultMapping       string // Vault secret mapping
	AutoGenerate       bool   // Generate random test secrets
	DockerUsername     string // Docker registry username
	DockerPassword     string // Docker registry password
	EnsureDockerSecret bool   // Create Docker registry secret
}

// OutputFlags holds output and rendering configuration.
type OutputFlags struct {
	RenderTemplates bool         // Render templates instead of deploying
	RenderOutputDir string       // Output directory for rendered templates
	DryRun          bool         // Preview without executing
	Format          OutputFormat // Output format (text, json)
}

// PathFlags holds repository and file path configuration.
type PathFlags struct {
	RepoRoot     string   // Repository root path
	ScenarioPath string   // Custom scenario path
	ValuesPreset string   // Values preset (latest, enterprise, local)
	ExtraValues  []string // Additional values files
	IngressSubdomain string // Ingress subdomain (combined with base domain)
	IngressHostname  string // Full ingress hostname override
}

// BehaviorFlags holds deployment behavior configuration.
type BehaviorFlags struct {
	Timeout              int  // Helm timeout in minutes
	SkipDependencyUpdate bool // Skip helm dependency update
	DeleteNamespaceFirst bool // Delete namespace before deploy
	Interactive          bool // Enable interactive prompts
}

// RuntimeFlagsGrouped is the refactored version of RuntimeFlags with grouped fields.
// This provides better organization and makes it easier to understand related settings.
type RuntimeFlagsGrouped struct {
	Chart         ChartFlags
	Deployment    DeploymentFlags
	Platform      PlatformFlags
	Keycloak      KeycloakFlags
	IndexPrefixes IndexPrefixFlags
	Secrets       SecretFlags
	Output        OutputFlags
	Paths         PathFlags
	Behavior      BehaviorFlags
}

// ToGrouped converts RuntimeFlags to RuntimeFlagsGrouped.
func (f *RuntimeFlags) ToGrouped() *RuntimeFlagsGrouped {
	return &RuntimeFlagsGrouped{
		Chart: ChartFlags{
			Path:    f.ChartPath,
			Name:    f.Chart,
			Version: f.ChartVersion,
		},
		Deployment: DeploymentFlags{
			Namespace: f.Namespace,
			Release:   f.Release,
			Scenario:  f.Scenario,
			Scenarios: f.Scenarios,
			Auth:      f.Auth,
		},
		Platform: PlatformFlags{
			Platform: f.Platform,
			LogLevel: f.LogLevel,
			Flow:     f.Flow,
			EnvFile:  f.EnvFile,
		},
		Keycloak: KeycloakFlags{
			Host:     f.KeycloakHost,
			Protocol: f.KeycloakProtocol,
			Realm:    f.KeycloakRealm,
		},
		IndexPrefixes: IndexPrefixFlags{
			Optimize:      f.OptimizeIndexPrefix,
			Orchestration: f.OrchestrationIndexPrefix,
			Tasklist:      f.TasklistIndexPrefix,
			Operate:       f.OperateIndexPrefix,
		},
		Secrets: SecretFlags{
			ExternalSecrets:    f.ExternalSecrets,
			VaultMapping:       f.VaultSecretMapping,
			AutoGenerate:       f.AutoGenerateSecrets,
			DockerUsername:     f.DockerUsername,
			DockerPassword:     f.DockerPassword,
			EnsureDockerSecret: f.EnsureDockerRegistry,
		},
		Output: OutputFlags{
			RenderTemplates: f.RenderTemplates,
			RenderOutputDir: f.RenderOutputDir,
			DryRun:          f.DryRun,
			Format:          f.OutputFormat,
		},
		Paths: PathFlags{
			RepoRoot:         f.RepoRoot,
			ScenarioPath:     f.ScenarioPath,
			ValuesPreset:     f.ValuesPreset,
			ExtraValues:      f.ExtraValues,
			IngressSubdomain: f.IngressSubdomain,
			IngressHostname:  f.IngressHostname,
		},
		Behavior: BehaviorFlags{
			Timeout:              f.Timeout,
			SkipDependencyUpdate: f.SkipDependencyUpdate,
			DeleteNamespaceFirst: f.DeleteNamespaceFirst,
			Interactive:          f.Interactive,
		},
	}
}

// FromGrouped converts RuntimeFlagsGrouped back to RuntimeFlags.
func FromGrouped(g *RuntimeFlagsGrouped) *RuntimeFlags {
	return &RuntimeFlags{
		ChartPath:                g.Chart.Path,
		Chart:                    g.Chart.Name,
		ChartVersion:             g.Chart.Version,
		Namespace:                g.Deployment.Namespace,
		Release:                  g.Deployment.Release,
		Scenario:                 g.Deployment.Scenario,
		Scenarios:                g.Deployment.Scenarios,
		Auth:                     g.Deployment.Auth,
		Platform:                 g.Platform.Platform,
		LogLevel:                 g.Platform.LogLevel,
		Flow:                     g.Platform.Flow,
		EnvFile:                  g.Platform.EnvFile,
		KeycloakHost:             g.Keycloak.Host,
		KeycloakProtocol:         g.Keycloak.Protocol,
		KeycloakRealm:            g.Keycloak.Realm,
		OptimizeIndexPrefix:      g.IndexPrefixes.Optimize,
		OrchestrationIndexPrefix: g.IndexPrefixes.Orchestration,
		TasklistIndexPrefix:      g.IndexPrefixes.Tasklist,
		OperateIndexPrefix:       g.IndexPrefixes.Operate,
		ExternalSecrets:          g.Secrets.ExternalSecrets,
		VaultSecretMapping:       g.Secrets.VaultMapping,
		AutoGenerateSecrets:      g.Secrets.AutoGenerate,
		DockerUsername:           g.Secrets.DockerUsername,
		DockerPassword:           g.Secrets.DockerPassword,
		EnsureDockerRegistry:     g.Secrets.EnsureDockerSecret,
		RenderTemplates:          g.Output.RenderTemplates,
		RenderOutputDir:          g.Output.RenderOutputDir,
		DryRun:                   g.Output.DryRun,
		OutputFormat:             g.Output.Format,
		RepoRoot:                 g.Paths.RepoRoot,
		ScenarioPath:             g.Paths.ScenarioPath,
		ValuesPreset:             g.Paths.ValuesPreset,
		ExtraValues:              g.Paths.ExtraValues,
		IngressSubdomain:         g.Paths.IngressSubdomain,
		IngressHostname:          g.Paths.IngressHostname,
		Timeout:                  g.Behavior.Timeout,
		SkipDependencyUpdate:     g.Behavior.SkipDependencyUpdate,
		DeleteNamespaceFirst:     g.Behavior.DeleteNamespaceFirst,
		Interactive:              g.Behavior.Interactive,
	}
}

// FlagGroup represents a group of related flags for documentation purposes.
type FlagGroup struct {
	Name        string
	Description string
	Flags       []FlagInfo
}

// FlagInfo holds metadata about a single flag.
type FlagInfo struct {
	Name        string
	Short       string
	Description string
	Default     string
	Required    bool
	EnvVar      string
}

// GetFlagGroups returns the flag groups for documentation/help purposes.
func GetFlagGroups() []FlagGroup {
	return []FlagGroup{
		{
			Name:        "Chart Source",
			Description: "Specify the Helm chart to deploy (choose one approach)",
			Flags: []FlagInfo{
				{Name: "chart-path", Description: "Local path to chart directory", EnvVar: ""},
				{Name: "chart", Short: "c", Description: "Remote chart name", EnvVar: ""},
				{Name: "version", Short: "v", Description: "Chart version for remote charts", EnvVar: ""},
			},
		},
		{
			Name:        "Deployment",
			Description: "Core deployment identifiers",
			Flags: []FlagInfo{
				{Name: "namespace", Short: "n", Description: "Kubernetes namespace", Required: true},
				{Name: "release", Short: "r", Description: "Helm release name", Required: true},
				{Name: "scenario", Short: "s", Description: "Scenario name(s)", Required: true},
				{Name: "auth", Description: "Authentication scenario", Default: "keycloak"},
			},
		},
		{
			Name:        "Platform",
			Description: "Platform and environment settings",
			Flags: []FlagInfo{
				{Name: "platform", Description: "Target platform", Default: "gke", EnvVar: "CAMUNDA_PLATFORM"},
				{Name: "log-level", Short: "l", Description: "Log verbosity", Default: "info", EnvVar: "CAMUNDA_LOG_LEVEL"},
				{Name: "flow", Description: "Deployment flow", Default: "install"},
			},
		},
		{
			Name:        "Keycloak",
			Description: "Keycloak integration settings",
			Flags: []FlagInfo{
				{Name: "keycloak-host", Description: "Keycloak hostname", EnvVar: "CAMUNDA_KEYCLOAK_HOST"},
				{Name: "keycloak-protocol", Description: "Keycloak protocol", Default: "https", EnvVar: "CAMUNDA_KEYCLOAK_PROTOCOL"},
				{Name: "keycloak-realm", Description: "Realm name (auto-generated if empty)", EnvVar: "CAMUNDA_KEYCLOAK_REALM"},
			},
		},
		{
			Name:        "Secrets",
			Description: "Secret management options",
			Flags: []FlagInfo{
				{Name: "external-secrets", Description: "Enable External Secrets", Default: "true"},
				{Name: "auto-generate-secrets", Description: "Generate random test secrets", Default: "false"},
				{Name: "vault-secret-mapping", Description: "Vault secret mapping spec"},
			},
		},
		{
			Name:        "Output",
			Description: "Output and rendering options",
			Flags: []FlagInfo{
				{Name: "render-templates", Description: "Render templates without deploying", Default: "false"},
				{Name: "render-output-dir", Description: "Output directory for templates"},
				{Name: "dry-run", Description: "Preview without executing", Default: "false"},
				{Name: "output", Short: "o", Description: "Output format (text, json)", Default: "text"},
				{Name: "explain", Description: "Show config value sources", Default: "false"},
			},
		},
		{
			Name:        "Behavior",
			Description: "Deployment behavior options",
			Flags: []FlagInfo{
				{Name: "timeout", Description: "Helm timeout in minutes", Default: "5"},
				{Name: "skip-dependency-update", Description: "Skip helm dep update", Default: "true"},
				{Name: "delete-namespace", Description: "Delete namespace first", Default: "false"},
				{Name: "interactive", Description: "Enable prompts", Default: "true"},
			},
		},
	}
}

