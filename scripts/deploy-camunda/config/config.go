package config

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// ValidIngressBaseDomains lists the allowed base domains for ingress hosts.
var ValidIngressBaseDomains = []string{
	"ci.distro.ultrawombat.com",
	"distribution.aws.camunda.cloud",
}

const (
	// DefaultKeycloakHost is the default external Keycloak hostname used in CI.
	DefaultKeycloakHost = "keycloak-24-9-0.ci.distro.ultrawombat.com"
	// DefaultKeycloakProtocol is the default protocol for the external Keycloak.
	DefaultKeycloakProtocol = "https"
)

// DeployPlatforms lists the valid deployment infrastructure platforms.
var DeployPlatforms = []string{"gke", "eks", "rosa"}

// TestPlatforms lists the valid test platform identifiers.
var TestPlatforms = []string{"gke", "eks", "openshift"}

// KeycloakConfig holds Keycloak connection settings.
type KeycloakConfig struct {
	Host     string `mapstructure:"host" yaml:"host,omitempty"`
	Protocol string `mapstructure:"protocol" yaml:"protocol,omitempty"`
}

// InfraConfig holds infrastructure fields shared across RootConfig,
// DeploymentConfig, and MatrixConfig. Embedding this struct eliminates
// field duplication and ensures new fields only need to be added once.
type InfraConfig struct {
	Platform             string `mapstructure:"platform" yaml:"platform,omitempty"`
	RepoRoot             string `mapstructure:"repoRoot" yaml:"repoRoot,omitempty"`
	NamespacePrefix      string `mapstructure:"namespacePrefix" yaml:"namespacePrefix,omitempty"`
	LogLevel             string `mapstructure:"logLevel" yaml:"logLevel,omitempty"`
	SkipDependencyUpdate *bool  `mapstructure:"skipDependencyUpdate" yaml:"skipDependencyUpdate,omitempty"`
	EnvFile              string `mapstructure:"envFile" yaml:"envFile,omitempty"`
	IngressBaseDomain    string `mapstructure:"ingressBaseDomain" yaml:"ingressBaseDomain,omitempty"`
	KubeContext          string `mapstructure:"kubeContext" yaml:"kubeContext,omitempty"`
	DeleteNamespace      *bool  `mapstructure:"deleteNamespace" yaml:"deleteNamespace,omitempty"`

	// Docker registries
	DockerUsername       string `mapstructure:"dockerUsername" yaml:"dockerUsername,omitempty"`
	DockerPassword       string `mapstructure:"dockerPassword" yaml:"dockerPassword,omitempty"`
	EnsureDockerRegistry *bool  `mapstructure:"ensureDockerRegistry" yaml:"ensureDockerRegistry,omitempty"`
	DockerHubUsername    string `mapstructure:"dockerHubUsername" yaml:"dockerHubUsername,omitempty"`
	DockerHubPassword    string `mapstructure:"dockerHubPassword" yaml:"dockerHubPassword,omitempty"`
	EnsureDockerHub      *bool  `mapstructure:"ensureDockerHub" yaml:"ensureDockerHub,omitempty"`
}

// DeploySpecConfig holds deployment-specification fields shared between
// RootConfig and DeploymentConfig (but not MatrixConfig). These are the
// fields that describe what to deploy and how.
type DeploySpecConfig struct {
	Chart                    string   `mapstructure:"chart" yaml:"chart,omitempty"`
	Version                  string   `mapstructure:"version" yaml:"version,omitempty"`
	ChartPath                string   `mapstructure:"chartPath" yaml:"chartPath,omitempty"`
	Namespace                string   `mapstructure:"namespace" yaml:"namespace,omitempty"`
	Release                  string   `mapstructure:"release" yaml:"release,omitempty"`
	Scenario                 string   `mapstructure:"scenario" yaml:"scenario,omitempty"`
	ScenarioPath             string   `mapstructure:"scenarioPath" yaml:"scenarioPath,omitempty"`
	Auth                     string   `mapstructure:"auth" yaml:"auth,omitempty"`
	Flow                     string   `mapstructure:"flow" yaml:"flow,omitempty"`
	ExternalSecrets          *bool    `mapstructure:"externalSecrets" yaml:"externalSecrets,omitempty"`
	KeycloakRealm            string   `mapstructure:"keycloakRealm" yaml:"keycloakRealm,omitempty"`
	OptimizeIndexPrefix      string   `mapstructure:"optimizeIndexPrefix" yaml:"optimizeIndexPrefix,omitempty"`
	OrchestrationIndexPrefix string   `mapstructure:"orchestrationIndexPrefix" yaml:"orchestrationIndexPrefix,omitempty"`
	TasklistIndexPrefix      string   `mapstructure:"tasklistIndexPrefix" yaml:"tasklistIndexPrefix,omitempty"`
	OperateIndexPrefix       string   `mapstructure:"operateIndexPrefix" yaml:"operateIndexPrefix,omitempty"`
	IngressHost              string   `mapstructure:"ingressHost" yaml:"ingressHost,omitempty"`
	IngressSubdomain         string   `mapstructure:"ingressSubdomain" yaml:"ingressSubdomain,omitempty"`
	Interactive              *bool    `mapstructure:"interactive" yaml:"interactive,omitempty"`
	VaultSecretMapping       string   `mapstructure:"vaultSecretMapping" yaml:"vaultSecretMapping,omitempty"`
	AutoGenerateSecrets      *bool    `mapstructure:"autoGenerateSecrets" yaml:"autoGenerateSecrets,omitempty"`
	RenderTemplates          *bool    `mapstructure:"renderTemplates" yaml:"renderTemplates,omitempty"`
	RenderOutputDir          string   `mapstructure:"renderOutputDir" yaml:"renderOutputDir,omitempty"`
	ExtraValues              []string `mapstructure:"extraValues" yaml:"extraValues,omitempty"`
	ScenarioRoot             string   `mapstructure:"scenarioRoot" yaml:"scenarioRoot,omitempty"`
	ValuesPreset             string   `mapstructure:"valuesPreset" yaml:"valuesPreset,omitempty"`
	RunIntegrationTests      *bool    `mapstructure:"runIntegrationTests" yaml:"runIntegrationTests,omitempty"`
	RunE2ETests              *bool    `mapstructure:"runE2ETests" yaml:"runE2ETests,omitempty"`

	// Selection + composition model fields (alternative to Scenario)
	Identity     string   `mapstructure:"identity" yaml:"identity,omitempty"`
	Persistence  string   `mapstructure:"persistence" yaml:"persistence,omitempty"`
	TestPlatform string   `mapstructure:"testPlatform" yaml:"testPlatform,omitempty"`
	Features     []string `mapstructure:"features" yaml:"features,omitempty"`
	QA           *bool    `mapstructure:"qa" yaml:"qa,omitempty"`
	ImageTags    *bool    `mapstructure:"imageTags" yaml:"imageTags,omitempty"`
	UpgradeFlow  *bool    `mapstructure:"upgradeFlow" yaml:"upgradeFlow,omitempty"`
}

// MatrixConfig holds configuration specific to the "matrix" subcommand.
// Fields here can be set in the deploy.yaml config file under a top-level
// "matrix:" key. Shared fields (repoRoot, platform, logLevel, keycloak, etc.)
// fall back to root-level config when not set in the matrix section.
type MatrixConfig struct {
	InfraConfig `mapstructure:",squash" yaml:",inline"`

	// Filtering & generation
	Versions        []string `mapstructure:"versions" yaml:"versions,omitempty"`
	IncludeDisabled *bool    `mapstructure:"includeDisabled" yaml:"includeDisabled,omitempty"`
	ScenarioFilter  string   `mapstructure:"scenarioFilter" yaml:"scenarioFilter,omitempty"`
	ShortnameFilter string   `mapstructure:"shortnameFilter" yaml:"shortnameFilter,omitempty"`
	FlowFilter      string   `mapstructure:"flowFilter" yaml:"flowFilter,omitempty"`
	OutputFormat    string   `mapstructure:"outputFormat" yaml:"outputFormat,omitempty"`

	// Execution
	MaxParallel   *int  `mapstructure:"maxParallel" yaml:"maxParallel,omitempty"`
	StopOnFailure *bool `mapstructure:"stopOnFailure" yaml:"stopOnFailure,omitempty"`
	Cleanup       *bool `mapstructure:"cleanup" yaml:"cleanup,omitempty"`
	DryRun        *bool `mapstructure:"dryRun" yaml:"dryRun,omitempty"`
	Coverage      *bool `mapstructure:"coverage" yaml:"coverage,omitempty"`
	HelmTimeout   *int  `mapstructure:"helmTimeout" yaml:"helmTimeout,omitempty"`

	// Tests
	TestIT  *bool `mapstructure:"testIT" yaml:"testIT,omitempty"`
	TestE2E *bool `mapstructure:"testE2E" yaml:"testE2E,omitempty"`
	TestAll *bool `mapstructure:"testAll" yaml:"testAll,omitempty"`

	// Per-platform kube contexts
	KubeContexts map[string]string `mapstructure:"kubeContexts" yaml:"kubeContexts,omitempty"`

	// Per-platform ingress domains
	IngressBaseDomains map[string]string `mapstructure:"ingressBaseDomains" yaml:"ingressBaseDomains,omitempty"`

	// Per-platform vault-backed secrets
	UseVaultBackedSecrets *bool           `mapstructure:"useVaultBackedSecrets" yaml:"useVaultBackedSecrets,omitempty"`
	VaultBackedSecrets    map[string]bool `mapstructure:"vaultBackedSecrets" yaml:"vaultBackedSecrets,omitempty"`

	// Per-version env files (keys are version strings like "8.6", "8.7")
	EnvFiles map[string]string `mapstructure:"envFiles" yaml:"envFiles,omitempty"`

	// Keycloak overrides (if different from root)
	KeycloakHost     string `mapstructure:"keycloakHost" yaml:"keycloakHost,omitempty"`
	KeycloakProtocol string `mapstructure:"keycloakProtocol" yaml:"keycloakProtocol,omitempty"`

	// Upgrade
	UpgradeFromVersion string `mapstructure:"upgradeFromVersion" yaml:"upgradeFromVersion,omitempty"`
}

// DeploymentConfig represents a single deployment profile.
type DeploymentConfig struct {
	InfraConfig      `mapstructure:",squash" yaml:",inline"`
	DeploySpecConfig `mapstructure:",squash" yaml:",inline"`

	Name string `mapstructure:"-" yaml:"-"` // filled at runtime from map key
}

// RootConfig represents the entire configuration file.
type RootConfig struct {
	InfraConfig      `mapstructure:",squash" yaml:",inline"`
	DeploySpecConfig `mapstructure:",squash" yaml:",inline"`

	Current     string                      `mapstructure:"current" yaml:"current,omitempty"`
	Keycloak    KeycloakConfig              `mapstructure:"keycloak" yaml:"keycloak,omitempty"`
	Matrix      MatrixConfig                `mapstructure:"matrix" yaml:"matrix,omitempty"`
	Deployments map[string]DeploymentConfig `mapstructure:"deployments" yaml:"deployments,omitempty"`
	FilePath    string                      `mapstructure:"-" yaml:"-"`
}

// ConfigResolution captures how the config file was resolved — which paths
// were searched, whether a file was actually found, and which file is in use.
// This enables downstream code (especially Validate) to produce actionable
// error messages instead of generic "flag X not set" errors.
type ConfigResolution struct {
	Path     string   // resolved config file path (may not exist on disk)
	Found    bool     // true when the file actually exists
	Searched []string // all candidate paths that were checked, in order
}

// DetectRepoRoot uses git to auto-detect the repository root from the current
// working directory. This correctly returns the worktree root when running
// inside a git worktree. Returns ("", nil) when git is not available or the
// CWD is not inside a git repository. Returns an error when inside a git repo
// that is not the Camunda Helm repo (missing sentinel file).
func DetectRepoRoot() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", nil // git not available or not in a repo — not an error
	}
	root := strings.TrimSpace(string(out))
	if root == "" {
		return "", nil
	}
	// Validate it's the expected repo
	if _, err := os.Stat(filepath.Join(root, "charts", "chart-versions.yaml")); err != nil {
		return "", fmt.Errorf("auto-detected git root %q is not the Camunda Helm repo (missing charts/chart-versions.yaml); set --repo-root explicitly", root)
	}
	return root, nil
}

// ResolvePath determines the config file path to use.
// It returns a ConfigResolution that records which paths were searched and
// whether the resolved file actually exists on disk.
func ResolvePath(explicit string) (*ConfigResolution, error) {
	if strings.TrimSpace(explicit) != "" {
		_, statErr := os.Stat(explicit)
		return &ConfigResolution{
			Path:     explicit,
			Found:    statErr == nil,
			Searched: []string{explicit},
		}, nil
	}

	var searched []string
	local := ".camunda-deploy.yaml"

	// prefer local project file if present
	cwd, _ := os.Getwd()
	cwdLocal := filepath.Join(cwd, local)
	searched = append(searched, cwdLocal)
	if _, err := os.Stat(local); err == nil {
		return &ConfigResolution{Path: local, Found: true, Searched: searched}, nil
	}

	// check repo root (handles running from subdirectories)
	if root, err := DetectRepoRoot(); err == nil && root != "" {
		repoLocal := filepath.Join(root, local)
		// avoid duplicate if CWD is the repo root
		if repoLocal != cwdLocal {
			searched = append(searched, repoLocal)
		}
		if _, err := os.Stat(repoLocal); err == nil {
			return &ConfigResolution{Path: repoLocal, Found: true, Searched: searched}, nil
		}
	}

	// fallback to user config directory
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	fallback := filepath.Join(home, ".config", "camunda", "deploy.yaml")
	searched = append(searched, fallback)
	_, statErr := os.Stat(fallback)
	return &ConfigResolution{Path: fallback, Found: statErr == nil, Searched: searched}, nil
}

// Read loads configuration from the specified path.
func Read(path string, includeEnv bool) (*RootConfig, error) {
	rc := &RootConfig{}
	// Missing file is not an error; we will create it on write
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to read config %q: %w", path, err)
	}
	if err == nil {
		if err := yaml.Unmarshal(data, rc); err != nil {
			return nil, fmt.Errorf("failed to parse config %q: %w", path, err)
		}
	}
	// Apply environment overrides (CAMUNDA_*) only when requested
	if includeEnv {
		applyEnvOverrides(rc)
	}
	rc.FilePath = path
	return rc, nil
}

// Write saves the configuration to disk.
func Write(rc *RootConfig) error {
	if strings.TrimSpace(rc.FilePath) == "" {
		return fmt.Errorf("no config file path resolved for writing")
	}
	dir := filepath.Dir(rc.FilePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	out, err := yaml.Marshal(rc)
	if err != nil {
		return err
	}
	return os.WriteFile(rc.FilePath, out, fs.FileMode(0o644))
}

// WriteCurrentOnly updates only the top-level "current" key in the YAML file.
func WriteCurrentOnly(path string, current string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		// If file doesn't exist, create a minimal one
		minimal := map[string]any{"current": current}
		out, mErr := yaml.Marshal(minimal)
		if mErr != nil {
			return mErr
		}
		return os.WriteFile(path, out, fs.FileMode(0o644))
	}
	var data map[string]any
	if err := yaml.Unmarshal(content, &data); err != nil || data == nil {
		data = map[string]any{}
	}
	data["current"] = current
	out, err := yaml.Marshal(data)
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, fs.FileMode(0o644))
}

// applyEnvOverrides applies environment variables to the config.
func applyEnvOverrides(rc *RootConfig) {
	if rc == nil {
		return
	}
	get := func(key string) string { return strings.TrimSpace(os.Getenv(key)) }
	if v := get("CAMUNDA_CURRENT"); v != "" {
		rc.Current = v
	}
	if v := get("CAMUNDA_REPO_ROOT"); v != "" {
		rc.RepoRoot = v
	}
	if v := get("CAMUNDA_SCENARIO_ROOT"); v != "" {
		rc.ScenarioRoot = v
	}
	if v := get("CAMUNDA_VALUES_PRESET"); v != "" {
		rc.ValuesPreset = v
	}
	if v := get("CAMUNDA_PLATFORM"); v != "" {
		rc.Platform = v
	}
	if v := get("CAMUNDA_LOG_LEVEL"); v != "" {
		rc.LogLevel = v
	}
	if v := get("CAMUNDA_EXTERNAL_SECRETS"); v != "" {
		b := strings.EqualFold(v, "true") || v == "1"
		rc.ExternalSecrets = &b
	}
	if v := get("CAMUNDA_SKIP_DEPENDENCY_UPDATE"); v != "" {
		b := strings.EqualFold(v, "true") || v == "1"
		rc.SkipDependencyUpdate = &b
	}
	if v := get("CAMUNDA_KEYCLOAK_HOST"); v != "" {
		rc.Keycloak.Host = v
	}
	if v := get("CAMUNDA_KEYCLOAK_PROTOCOL"); v != "" {
		rc.Keycloak.Protocol = v
	}
	if v := get("CAMUNDA_KEYCLOAK_REALM"); v != "" {
		rc.KeycloakRealm = v
	}
	if v := get("CAMUNDA_OPTIMIZE_INDEX_PREFIX"); v != "" {
		rc.OptimizeIndexPrefix = v
	}
	if v := get("CAMUNDA_ORCHESTRATION_INDEX_PREFIX"); v != "" {
		rc.OrchestrationIndexPrefix = v
	}
	if v := get("CAMUNDA_TASKLIST_INDEX_PREFIX"); v != "" {
		rc.TasklistIndexPrefix = v
	}
	if v := get("CAMUNDA_OPERATE_INDEX_PREFIX"); v != "" {
		rc.OperateIndexPrefix = v
	}
	if v := get("CAMUNDA_HOSTNAME"); v != "" {
		rc.IngressHost = v
	}
	if v := get("CAMUNDA_KUBE_CONTEXT"); v != "" {
		rc.KubeContext = v
	}

	// Matrix-specific env overrides (CAMUNDA_MATRIX_*)
	applyMatrixEnvOverrides(&rc.Matrix, get)
}

// applyMatrixEnvOverrides applies CAMUNDA_MATRIX_* environment variables to the matrix config.
func applyMatrixEnvOverrides(m *MatrixConfig, get func(string) string) {
	if m == nil {
		return
	}
	parseBool := func(v string) bool {
		return strings.EqualFold(v, "true") || v == "1"
	}
	if v := get("CAMUNDA_MATRIX_REPO_ROOT"); v != "" {
		m.RepoRoot = v
	}
	if v := get("CAMUNDA_MATRIX_PLATFORM"); v != "" {
		m.Platform = v
	}
	if v := get("CAMUNDA_MATRIX_LOG_LEVEL"); v != "" {
		m.LogLevel = v
	}
	if v := get("CAMUNDA_MATRIX_NAMESPACE_PREFIX"); v != "" {
		m.NamespacePrefix = v
	}
	if v := get("CAMUNDA_MATRIX_MAX_PARALLEL"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			m.MaxParallel = &n
		}
	}
	if v := get("CAMUNDA_MATRIX_HELM_TIMEOUT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			m.HelmTimeout = &n
		}
	}
	if v := get("CAMUNDA_MATRIX_CLEANUP"); v != "" {
		b := parseBool(v)
		m.Cleanup = &b
	}
	if v := get("CAMUNDA_MATRIX_STOP_ON_FAILURE"); v != "" {
		b := parseBool(v)
		m.StopOnFailure = &b
	}
	if v := get("CAMUNDA_MATRIX_SKIP_DEPENDENCY_UPDATE"); v != "" {
		b := parseBool(v)
		m.SkipDependencyUpdate = &b
	}
	if v := get("CAMUNDA_MATRIX_DELETE_NAMESPACE"); v != "" {
		b := parseBool(v)
		m.DeleteNamespace = &b
	}
	if v := get("CAMUNDA_MATRIX_DRY_RUN"); v != "" {
		b := parseBool(v)
		m.DryRun = &b
	}
	if v := get("CAMUNDA_MATRIX_KUBE_CONTEXT"); v != "" {
		m.KubeContext = v
	}
	if v := get("CAMUNDA_MATRIX_INGRESS_BASE_DOMAIN"); v != "" {
		m.IngressBaseDomain = v
	}
	if v := get("CAMUNDA_MATRIX_ENV_FILE"); v != "" {
		m.EnvFile = v
	}
	if v := get("CAMUNDA_MATRIX_DOCKER_USERNAME"); v != "" {
		m.DockerUsername = v
	}
	if v := get("CAMUNDA_MATRIX_DOCKER_PASSWORD"); v != "" {
		m.DockerPassword = v
	}
	if v := get("CAMUNDA_MATRIX_ENSURE_DOCKER_REGISTRY"); v != "" {
		b := parseBool(v)
		m.EnsureDockerRegistry = &b
	}
	if v := get("CAMUNDA_MATRIX_DOCKERHUB_USERNAME"); v != "" {
		m.DockerHubUsername = v
	}
	if v := get("CAMUNDA_MATRIX_DOCKERHUB_PASSWORD"); v != "" {
		m.DockerHubPassword = v
	}
	if v := get("CAMUNDA_MATRIX_ENSURE_DOCKER_HUB"); v != "" {
		b := parseBool(v)
		m.EnsureDockerHub = &b
	}
	if v := get("CAMUNDA_MATRIX_KEYCLOAK_HOST"); v != "" {
		m.KeycloakHost = v
	}
	if v := get("CAMUNDA_MATRIX_KEYCLOAK_PROTOCOL"); v != "" {
		m.KeycloakProtocol = v
	}
	if v := get("CAMUNDA_MATRIX_USE_VAULT_BACKED_SECRETS"); v != "" {
		b := parseBool(v)
		m.UseVaultBackedSecrets = &b
	}
	if v := get("CAMUNDA_MATRIX_UPGRADE_FROM_VERSION"); v != "" {
		m.UpgradeFromVersion = v
	}
	if v := get("CAMUNDA_MATRIX_TEST_IT"); v != "" {
		b := parseBool(v)
		m.TestIT = &b
	}
	if v := get("CAMUNDA_MATRIX_TEST_E2E"); v != "" {
		b := parseBool(v)
		m.TestE2E = &b
	}
	if v := get("CAMUNDA_MATRIX_TEST_ALL"); v != "" {
		b := parseBool(v)
		m.TestAll = &b
	}
}

// FirstNonEmpty returns the first non-empty string.
func FirstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// MergeStringField applies deployment/root value to target when the CLI flag
// was not explicitly set by the user. When changedFlags is non-nil and contains
// flagName, the target is left untouched because the CLI value takes precedence.
// For flags whose default is empty (""), the legacy behaviour (skip when
// non-empty) is equivalent, but for flags with non-empty defaults (e.g.
// --platform "gke") this ensures config-file values can still take effect.
func MergeStringField(target *string, depVal, rootVal string, changedFlags map[string]bool, flagName string) {
	if changedFlags != nil && changedFlags[flagName] {
		return // CLI flag was explicitly set; do not override
	}
	merged := FirstNonEmpty(depVal, rootVal)
	if merged != "" {
		*target = merged
	}
}

// MergeBoolField applies deployment/root value to target if the flag was not
// explicitly set by the user. When changedFlags is non-nil and contains flagName,
// the target is left untouched because the CLI value takes precedence.
func MergeBoolField(target *bool, depVal, rootVal *bool, changedFlags map[string]bool, flagName string) {
	if changedFlags != nil && changedFlags[flagName] {
		return // CLI flag was explicitly set; do not override
	}
	if depVal != nil {
		*target = *depVal
	} else if rootVal != nil {
		*target = *rootVal
	}
}

// MergeStringSliceField applies deployment/root value to target if target is empty.
func MergeStringSliceField(target *[]string, depVal, rootVal []string) {
	if len(*target) == 0 {
		if len(depVal) > 0 {
			*target = append(*target, depVal...)
		} else if len(rootVal) > 0 {
			*target = append(*target, rootVal...)
		}
	}
}

// SetValue sets a configuration value in the config file.
// Key format: "key" for root-level or "deployment.key" for deployment-specific.
func SetValue(cfgPath, key, value string) error {
	// Read current config as raw map to preserve unknown fields
	content, err := os.ReadFile(cfgPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read config: %w", err)
	}

	var data map[string]any
	if len(content) > 0 {
		if err := yaml.Unmarshal(content, &data); err != nil {
			return fmt.Errorf("failed to parse config: %w", err)
		}
	}
	if data == nil {
		data = make(map[string]any)
	}

	// Check if this is a deployment-specific key (contains a dot)
	if idx := strings.Index(key, "."); idx > 0 {
		depName := key[:idx]
		fieldKey := key[idx+1:]

		// Ensure deployments map exists
		deployments, ok := data["deployments"].(map[string]any)
		if !ok {
			deployments = make(map[string]any)
			data["deployments"] = deployments
		}

		// Ensure specific deployment exists
		dep, ok := deployments[depName].(map[string]any)
		if !ok {
			dep = make(map[string]any)
			deployments[depName] = dep
		}

		// Set the value (with type conversion for booleans)
		dep[fieldKey] = parseValue(value)
	} else {
		// Root-level key
		data[key] = parseValue(value)
	}

	// Write back
	out, err := yaml.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	dir := filepath.Dir(cfgPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	return os.WriteFile(cfgPath, out, 0o644)
}

// GetValue gets a configuration value from the config file.
// Key format: "key" for root-level or "deployment.key" for deployment-specific.
func GetValue(cfgPath, key string) (string, error) {
	content, err := os.ReadFile(cfgPath)
	if err != nil {
		return "", fmt.Errorf("failed to read config: %w", err)
	}

	var data map[string]any
	if err := yaml.Unmarshal(content, &data); err != nil {
		return "", fmt.Errorf("failed to parse config: %w", err)
	}

	// Check if this is a deployment-specific key
	if idx := strings.Index(key, "."); idx > 0 {
		depName := key[:idx]
		fieldKey := key[idx+1:]

		deployments, ok := data["deployments"].(map[string]any)
		if !ok {
			return "", fmt.Errorf("no deployments configured")
		}

		dep, ok := deployments[depName].(map[string]any)
		if !ok {
			return "", fmt.Errorf("deployment %q not found", depName)
		}

		val, ok := dep[fieldKey]
		if !ok {
			return "", fmt.Errorf("key %q not found in deployment %q", fieldKey, depName)
		}

		return formatValue(val), nil
	}

	// Root-level key
	val, ok := data[key]
	if !ok {
		return "", fmt.Errorf("key %q not found", key)
	}

	return formatValue(val), nil
}

// CreateDeployment creates a new empty deployment configuration.
func CreateDeployment(cfgPath, name string) error {
	content, err := os.ReadFile(cfgPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read config: %w", err)
	}

	var data map[string]any
	if len(content) > 0 {
		if err := yaml.Unmarshal(content, &data); err != nil {
			return fmt.Errorf("failed to parse config: %w", err)
		}
	}
	if data == nil {
		data = make(map[string]any)
	}

	// Ensure deployments map exists
	deployments, ok := data["deployments"].(map[string]any)
	if !ok {
		deployments = make(map[string]any)
		data["deployments"] = deployments
	}

	// Check if deployment already exists
	if _, exists := deployments[name]; exists {
		return fmt.Errorf("deployment %q already exists", name)
	}

	// Create empty deployment
	deployments[name] = make(map[string]any)

	// Write back
	out, err := yaml.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	dir := filepath.Dir(cfgPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	return os.WriteFile(cfgPath, out, 0o644)
}

// parseValue converts a string value to an appropriate type.
func parseValue(value string) any {
	// Try boolean
	if strings.EqualFold(value, "true") {
		return true
	}
	if strings.EqualFold(value, "false") {
		return false
	}

	// Try integer
	if i, err := strconv.Atoi(value); err == nil {
		return i
	}

	// Return as string
	return value
}

// formatValue converts a value to a string for display.
func formatValue(val any) string {
	switch v := val.(type) {
	case string:
		return v
	case bool:
		if v {
			return "true"
		}
		return "false"
	case int:
		return fmt.Sprintf("%d", v)
	case float64:
		return fmt.Sprintf("%v", v)
	case []any:
		var parts []string
		for _, item := range v {
			parts = append(parts, formatValue(item))
		}
		return strings.Join(parts, ",")
	default:
		return fmt.Sprintf("%v", v)
	}
}
