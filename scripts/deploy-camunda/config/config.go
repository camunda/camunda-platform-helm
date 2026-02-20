package config

import (
	"fmt"
	"io/fs"
	"os"
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

// DeploymentConfig represents a single deployment profile.
type DeploymentConfig struct {
	Name                     string   `mapstructure:"-" yaml:"-"` // filled at runtime from map key
	Chart                    string   `mapstructure:"chart" yaml:"chart,omitempty"`
	Version                  string   `mapstructure:"version" yaml:"version,omitempty"`
	Scenario                 string   `mapstructure:"scenario" yaml:"scenario,omitempty"`
	ChartPath                string   `mapstructure:"chartPath" yaml:"chartPath,omitempty"`
	Namespace                string   `mapstructure:"namespace" yaml:"namespace,omitempty"`
	NamespacePrefix          string   `mapstructure:"namespacePrefix" yaml:"namespacePrefix,omitempty"`
	Release                  string   `mapstructure:"release" yaml:"release,omitempty"`
	ScenarioPath             string   `mapstructure:"scenarioPath" yaml:"scenarioPath,omitempty"`
	Auth                     string   `mapstructure:"auth" yaml:"auth,omitempty"`
	Platform                 string   `mapstructure:"platform" yaml:"platform,omitempty"`
	LogLevel                 string   `mapstructure:"logLevel" yaml:"logLevel,omitempty"`
	ExternalSecrets          *bool    `mapstructure:"externalSecrets" yaml:"externalSecrets,omitempty"`
	SkipDependencyUpdate     *bool    `mapstructure:"skipDependencyUpdate" yaml:"skipDependencyUpdate,omitempty"`
	KeycloakRealm            string   `mapstructure:"keycloakRealm" yaml:"keycloakRealm,omitempty"`
	OptimizeIndexPrefix      string   `mapstructure:"optimizeIndexPrefix" yaml:"optimizeIndexPrefix,omitempty"`
	OrchestrationIndexPrefix string   `mapstructure:"orchestrationIndexPrefix" yaml:"orchestrationIndexPrefix,omitempty"`
	TasklistIndexPrefix      string   `mapstructure:"tasklistIndexPrefix" yaml:"tasklistIndexPrefix,omitempty"`
	OperateIndexPrefix       string   `mapstructure:"operateIndexPrefix" yaml:"operateIndexPrefix,omitempty"`
	IngressHost              string   `mapstructure:"ingressHost" yaml:"ingressHost,omitempty"`
	IngressBaseDomain        string   `mapstructure:"ingressBaseDomain" yaml:"ingressBaseDomain,omitempty"`
	Flow                     string   `mapstructure:"flow" yaml:"flow,omitempty"`
	EnvFile                  string   `mapstructure:"envFile" yaml:"envFile,omitempty"`
	Interactive              *bool    `mapstructure:"interactive" yaml:"interactive,omitempty"`
	VaultSecretMapping       string   `mapstructure:"vaultSecretMapping" yaml:"vaultSecretMapping,omitempty"`
	AutoGenerateSecrets      *bool    `mapstructure:"autoGenerateSecrets" yaml:"autoGenerateSecrets,omitempty"`
	DeleteNamespace          *bool    `mapstructure:"deleteNamespace" yaml:"deleteNamespace,omitempty"`
	DockerUsername           string   `mapstructure:"dockerUsername" yaml:"dockerUsername,omitempty"`
	DockerPassword           string   `mapstructure:"dockerPassword" yaml:"dockerPassword,omitempty"`
	EnsureDockerRegistry     *bool    `mapstructure:"ensureDockerRegistry" yaml:"ensureDockerRegistry,omitempty"`
	RenderTemplates          *bool    `mapstructure:"renderTemplates" yaml:"renderTemplates,omitempty"`
	RenderOutputDir          string   `mapstructure:"renderOutputDir" yaml:"renderOutputDir,omitempty"`
	ExtraValues              []string `mapstructure:"extraValues" yaml:"extraValues,omitempty"`
	RepoRoot                 string   `mapstructure:"repoRoot" yaml:"repoRoot,omitempty"`
	ScenarioRoot             string   `mapstructure:"scenarioRoot" yaml:"scenarioRoot,omitempty"`
	ValuesPreset             string   `mapstructure:"valuesPreset" yaml:"valuesPreset,omitempty"`
	RunIntegrationTests      *bool    `mapstructure:"runIntegrationTests" yaml:"runIntegrationTests,omitempty"`
	RunE2ETests              *bool    `mapstructure:"runE2ETests" yaml:"runE2ETests,omitempty"`
	KubeContext              string   `mapstructure:"kubeContext" yaml:"kubeContext,omitempty"`
}

// RootConfig represents the entire configuration file.
type RootConfig struct {
	Current                  string                      `mapstructure:"current" yaml:"current,omitempty"`
	RepoRoot                 string                      `mapstructure:"repoRoot" yaml:"repoRoot,omitempty"`
	ScenarioRoot             string                      `mapstructure:"scenarioRoot" yaml:"scenarioRoot,omitempty"`
	ValuesPreset             string                      `mapstructure:"valuesPreset" yaml:"valuesPreset,omitempty"`
	ChartPath                string                      `mapstructure:"chartPath" yaml:"chartPath,omitempty"`
	Chart                    string                      `mapstructure:"chart" yaml:"chart,omitempty"`
	Version                  string                      `mapstructure:"version" yaml:"version,omitempty"`
	Namespace                string                      `mapstructure:"namespace" yaml:"namespace,omitempty"`
	NamespacePrefix          string                      `mapstructure:"namespacePrefix" yaml:"namespacePrefix,omitempty"`
	Release                  string                      `mapstructure:"release" yaml:"release,omitempty"`
	Scenario                 string                      `mapstructure:"scenario" yaml:"scenario,omitempty"`
	ScenarioPath             string                      `mapstructure:"scenarioPath" yaml:"scenarioPath,omitempty"`
	Auth                     string                      `mapstructure:"auth" yaml:"auth,omitempty"`
	Platform                 string                      `mapstructure:"platform" yaml:"platform,omitempty"`
	LogLevel                 string                      `mapstructure:"logLevel" yaml:"logLevel,omitempty"`
	ExternalSecrets          bool                        `mapstructure:"externalSecrets" yaml:"externalSecrets,omitempty"`
	SkipDependencyUpdate     bool                        `mapstructure:"skipDependencyUpdate" yaml:"skipDependencyUpdate,omitempty"`
	KeycloakRealm            string                      `mapstructure:"keycloakRealm" yaml:"keycloakRealm,omitempty"`
	OptimizeIndexPrefix      string                      `mapstructure:"optimizeIndexPrefix" yaml:"optimizeIndexPrefix,omitempty"`
	OrchestrationIndexPrefix string                      `mapstructure:"orchestrationIndexPrefix" yaml:"orchestrationIndexPrefix,omitempty"`
	TasklistIndexPrefix      string                      `mapstructure:"tasklistIndexPrefix" yaml:"tasklistIndexPrefix,omitempty"`
	OperateIndexPrefix       string                      `mapstructure:"operateIndexPrefix" yaml:"operateIndexPrefix,omitempty"`
	IngressHost              string                      `mapstructure:"ingressHost" yaml:"ingressHost,omitempty"`
	IngressBaseDomain        string                      `mapstructure:"ingressBaseDomain" yaml:"ingressBaseDomain,omitempty"`
	Flow                     string                      `mapstructure:"flow" yaml:"flow,omitempty"`
	EnvFile                  string                      `mapstructure:"envFile" yaml:"envFile,omitempty"`
	Interactive              *bool                       `mapstructure:"interactive" yaml:"interactive,omitempty"`
	VaultSecretMapping       string                      `mapstructure:"vaultSecretMapping" yaml:"vaultSecretMapping,omitempty"`
	AutoGenerateSecrets      *bool                       `mapstructure:"autoGenerateSecrets" yaml:"autoGenerateSecrets,omitempty"`
	DeleteNamespaceFirst     *bool                       `mapstructure:"deleteNamespace" yaml:"deleteNamespace,omitempty"`
	DockerUsername           string                      `mapstructure:"dockerUsername" yaml:"dockerUsername,omitempty"`
	DockerPassword           string                      `mapstructure:"dockerPassword" yaml:"dockerPassword,omitempty"`
	EnsureDockerRegistry     *bool                       `mapstructure:"ensureDockerRegistry" yaml:"ensureDockerRegistry,omitempty"`
	RenderTemplates          *bool                       `mapstructure:"renderTemplates" yaml:"renderTemplates,omitempty"`
	RenderOutputDir          string                      `mapstructure:"renderOutputDir" yaml:"renderOutputDir,omitempty"`
	ExtraValues              []string                    `mapstructure:"extraValues" yaml:"extraValues,omitempty"`
	RunIntegrationTests      *bool                       `mapstructure:"runIntegrationTests" yaml:"runIntegrationTests,omitempty"`
	RunE2ETests              *bool                       `mapstructure:"runE2ETests" yaml:"runE2ETests,omitempty"`
	Keycloak                 KeycloakConfig              `mapstructure:"keycloak" yaml:"keycloak,omitempty"`
	Deployments              map[string]DeploymentConfig `mapstructure:"deployments" yaml:"deployments,omitempty"`
	FilePath                 string                      `mapstructure:"-" yaml:"-"`
	KubeContext              string                      `mapstructure:"kubeContext" yaml:"kubeContext,omitempty"`
}

// ResolvePath determines the config file path to use.
func ResolvePath(explicit string) (string, error) {
	if strings.TrimSpace(explicit) != "" {
		return explicit, nil
	}
	// prefer local project file if present
	local := ".camunda-deploy.yaml"
	if _, err := os.Stat(local); err == nil {
		return local, nil
	}
	// fallback to user config directory
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "camunda", "deploy.yaml"), nil
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
		rc.ExternalSecrets = strings.EqualFold(v, "true") || v == "1"
	}
	if v := get("CAMUNDA_SKIP_DEPENDENCY_UPDATE"); v != "" {
		rc.SkipDependencyUpdate = strings.EqualFold(v, "true") || v == "1"
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
