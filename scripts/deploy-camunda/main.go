package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"io/fs"
	"math/big"
	"os"
	"path/filepath"
	"scripts/camunda-core/pkg/completion"
	"scripts/camunda-core/pkg/kube"
	"scripts/camunda-core/pkg/logging"
	"scripts/camunda-deployer/pkg/deployer"
	"scripts/camunda-deployer/pkg/types"
	"scripts/prepare-helm-values/pkg/env"
	"scripts/prepare-helm-values/pkg/values"
	"strings"
	"time"
	"vault-secret-mapper/pkg/mapper"

	"github.com/jwalton/gchalk"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"gopkg.in/yaml.v3"
)

var (
	chartPath            string
	chart                string
	chartVersion         string
	namespace            string
	release              string
	scenario             string
	scenarioPath         string
	auth                 string
	platform             string
	logLevel             string
	skipDependencyUpdate bool
	externalSecrets      bool
	keycloakHost         string
	keycloakProtocol     string
	repoRoot             string
	flow                 string
	envFile              string
	interactive          bool
	vaultSecretMapping   string
	autoGenerateSecrets  bool
	deleteNamespaceFirst bool
	dockerUsername       string
	dockerPassword       string
	ensureDockerRegistry bool
	renderTemplates      bool
	renderOutputDir      string
	extraValues          []string
	valuesPreset         string
	configFile           string
)

// Config types for multi-deployment configuration
type KeycloakConfig struct {
	Host     string `mapstructure:"host" yaml:"host,omitempty"`
	Protocol string `mapstructure:"protocol" yaml:"protocol,omitempty"`
}

type DeploymentConfig struct {
	Name     string `mapstructure:"-" yaml:"-"` // filled at runtime from map key
	Chart    string `mapstructure:"chart" yaml:"chart,omitempty"`
	Version  string `mapstructure:"version" yaml:"version,omitempty"`
	Scenario string `mapstructure:"scenario" yaml:"scenario,omitempty"`
	// Per-deployment overrides mirroring CLI flags
	ChartPath            string   `mapstructure:"chartPath" yaml:"chartPath,omitempty"`
	Namespace            string   `mapstructure:"namespace" yaml:"namespace,omitempty"`
	Release              string   `mapstructure:"release" yaml:"release,omitempty"`
	ScenarioPath         string   `mapstructure:"scenarioPath" yaml:"scenarioPath,omitempty"`
	Auth                 string   `mapstructure:"auth" yaml:"auth,omitempty"`
	Platform             string   `mapstructure:"platform" yaml:"platform,omitempty"`
	LogLevel             string   `mapstructure:"logLevel" yaml:"logLevel,omitempty"`
	ExternalSecrets      *bool    `mapstructure:"externalSecrets" yaml:"externalSecrets,omitempty"`
	SkipDependencyUpdate *bool    `mapstructure:"skipDependencyUpdate" yaml:"skipDependencyUpdate,omitempty"`
	Flow                 string   `mapstructure:"flow" yaml:"flow,omitempty"`
	EnvFile              string   `mapstructure:"envFile" yaml:"envFile,omitempty"`
	Interactive          *bool    `mapstructure:"interactive" yaml:"interactive,omitempty"`
	VaultSecretMapping   string   `mapstructure:"vaultSecretMapping" yaml:"vaultSecretMapping,omitempty"`
	AutoGenerateSecrets  *bool    `mapstructure:"autoGenerateSecrets" yaml:"autoGenerateSecrets,omitempty"`
	DeleteNamespace      *bool    `mapstructure:"deleteNamespace" yaml:"deleteNamespace,omitempty"`
	DockerUsername       string   `mapstructure:"dockerUsername" yaml:"dockerUsername,omitempty"`
	DockerPassword       string   `mapstructure:"dockerPassword" yaml:"dockerPassword,omitempty"`
	EnsureDockerRegistry *bool    `mapstructure:"ensureDockerRegistry" yaml:"ensureDockerRegistry,omitempty"`
	RenderTemplates      *bool    `mapstructure:"renderTemplates" yaml:"renderTemplates,omitempty"`
	RenderOutputDir      string   `mapstructure:"renderOutputDir" yaml:"renderOutputDir,omitempty"`
	ExtraValues          []string `mapstructure:"extraValues" yaml:"extraValues,omitempty"`
	RepoRoot             string   `mapstructure:"repoRoot" yaml:"repoRoot,omitempty"`
	ScenarioRoot         string   `mapstructure:"scenarioRoot" yaml:"scenarioRoot,omitempty"`
	ValuesPreset         string   `mapstructure:"valuesPreset" yaml:"valuesPreset,omitempty"`
}

type RootConfig struct {
	Current      string `mapstructure:"current" yaml:"current,omitempty"`
	RepoRoot     string `mapstructure:"repoRoot" yaml:"repoRoot,omitempty"`
	ScenarioRoot string `mapstructure:"scenarioRoot" yaml:"scenarioRoot,omitempty"`
	ValuesPreset string `mapstructure:"valuesPreset" yaml:"valuesPreset,omitempty"`
	// Global defaults mirroring CLI flags
	ChartPath            string                      `mapstructure:"chartPath" yaml:"chartPath,omitempty"`
	Chart                string                      `mapstructure:"chart" yaml:"chart,omitempty"`
	Version              string                      `mapstructure:"version" yaml:"version,omitempty"`
	Namespace            string                      `mapstructure:"namespace" yaml:"namespace,omitempty"`
	Release              string                      `mapstructure:"release" yaml:"release,omitempty"`
	Scenario             string                      `mapstructure:"scenario" yaml:"scenario,omitempty"`
	ScenarioPath         string                      `mapstructure:"scenarioPath" yaml:"scenarioPath,omitempty"`
	Auth                 string                      `mapstructure:"auth" yaml:"auth,omitempty"`
	Platform             string                      `mapstructure:"platform" yaml:"platform,omitempty"`
	LogLevel             string                      `mapstructure:"logLevel" yaml:"logLevel,omitempty"`
	ExternalSecrets      bool                        `mapstructure:"externalSecrets" yaml:"externalSecrets,omitempty"`
	SkipDependencyUpdate bool                        `mapstructure:"skipDependencyUpdate" yaml:"skipDependencyUpdate,omitempty"`
	Flow                 string                      `mapstructure:"flow" yaml:"flow,omitempty"`
	EnvFile              string                      `mapstructure:"envFile" yaml:"envFile,omitempty"`
	Interactive          *bool                       `mapstructure:"interactive" yaml:"interactive,omitempty"`
	VaultSecretMapping   string                      `mapstructure:"vaultSecretMapping" yaml:"vaultSecretMapping,omitempty"`
	AutoGenerateSecrets  *bool                       `mapstructure:"autoGenerateSecrets" yaml:"autoGenerateSecrets,omitempty"`
	DeleteNamespaceFirst *bool                       `mapstructure:"deleteNamespace" yaml:"deleteNamespace,omitempty"`
	DockerUsername       string                      `mapstructure:"dockerUsername" yaml:"dockerUsername,omitempty"`
	DockerPassword       string                      `mapstructure:"dockerPassword" yaml:"dockerPassword,omitempty"`
	EnsureDockerRegistry *bool                       `mapstructure:"ensureDockerRegistry" yaml:"ensureDockerRegistry,omitempty"`
	RenderTemplates      *bool                       `mapstructure:"renderTemplates" yaml:"renderTemplates,omitempty"`
	RenderOutputDir      string                      `mapstructure:"renderOutputDir" yaml:"renderOutputDir,omitempty"`
	ExtraValues          []string                    `mapstructure:"extraValues" yaml:"extraValues,omitempty"`
	Keycloak             KeycloakConfig              `mapstructure:"keycloak" yaml:"keycloak,omitempty"`
	Deployments          map[string]DeploymentConfig `mapstructure:"deployments" yaml:"deployments,omitempty"`
	// FilePath is not serialized; used for write-back
	FilePath string `mapstructure:"-" yaml:"-"`
}

func defaultScenarioRoot() string {
	return filepath.Join("test", "integration", "scenarios", "chart-full-setup")
}

func resolveConfigPath(explicit string) (string, error) {
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

func readConfig(path string, includeEnv bool) (*RootConfig, error) {
	rc := &RootConfig{}
	// Missing file is not an error; we will create it on write
	if data, err := os.ReadFile(path); err == nil {
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
}

func writeConfig(rc *RootConfig) error {
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

// writeConfigCurrentOnly updates only the top-level "current" key in the YAML file,
// preserving all other fields and their original spelling/style.
func writeConfigCurrentOnly(path string, current string) error {
	// Ensure directory exists
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
		// On parse error or empty content, fall back to minimal
		data = map[string]any{}
	}
	data["current"] = current
	out, err := yaml.Marshal(data)
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, fs.FileMode(0o644))
}

// applyActiveDeployment merges active deployment and top-level defaults into CLI vars when flags are not set.
func applyActiveDeployment(rc *RootConfig, active string) error {
	if rc == nil || rc.Deployments == nil {
		return nil
	}
	if strings.TrimSpace(active) == "" {
		// If exactly one deployment exists, use it implicitly
		if len(rc.Deployments) == 1 {
			for name := range rc.Deployments {
				active = name
			}
		}
	}
	if strings.TrimSpace(active) == "" {
		return nil
	}
	dep, ok := rc.Deployments[active]
	if !ok {
		return fmt.Errorf("active deployment %q not found in config", active)
	}
	// fill CLI vars only when not provided via flags
	if strings.TrimSpace(chartPath) == "" && strings.TrimSpace(dep.ChartPath) != "" {
		chartPath = dep.ChartPath
	}
	if strings.TrimSpace(chart) == "" && strings.TrimSpace(chartPath) == "" && strings.TrimSpace(dep.Chart) != "" {
		chart = dep.Chart
	}
	if strings.TrimSpace(chartVersion) == "" && strings.TrimSpace(chartPath) == "" && strings.TrimSpace(dep.Version) != "" {
		chartVersion = dep.Version
	}
	if strings.TrimSpace(namespace) == "" && strings.TrimSpace(dep.Namespace) != "" {
		namespace = dep.Namespace
	}
	if strings.TrimSpace(release) == "" && strings.TrimSpace(dep.Release) != "" {
		release = dep.Release
	}
	if strings.TrimSpace(scenario) == "" && strings.TrimSpace(dep.Scenario) != "" {
		scenario = dep.Scenario
	}
	if strings.TrimSpace(auth) == "" && strings.TrimSpace(dep.Auth) != "" {
		auth = dep.Auth
	}
	if strings.TrimSpace(platform) == "" && strings.TrimSpace(dep.Platform) != "" {
		platform = dep.Platform
	}
	if strings.TrimSpace(logLevel) == "" && strings.TrimSpace(dep.LogLevel) != "" {
		logLevel = dep.LogLevel
	}
	if dep.ExternalSecrets != nil {
		externalSecrets = *dep.ExternalSecrets
	}
	if dep.SkipDependencyUpdate != nil {
		skipDependencyUpdate = *dep.SkipDependencyUpdate
	}
	if strings.TrimSpace(flow) == "" && strings.TrimSpace(dep.Flow) != "" {
		flow = dep.Flow
	}
	if strings.TrimSpace(envFile) == "" && strings.TrimSpace(dep.EnvFile) != "" {
		envFile = dep.EnvFile
	}
	if dep.Interactive != nil {
		interactive = *dep.Interactive
	}
	if strings.TrimSpace(vaultSecretMapping) == "" && strings.TrimSpace(dep.VaultSecretMapping) != "" {
		vaultSecretMapping = dep.VaultSecretMapping
	}
	if dep.AutoGenerateSecrets != nil {
		autoGenerateSecrets = *dep.AutoGenerateSecrets
	}
	if dep.DeleteNamespace != nil {
		deleteNamespaceFirst = *dep.DeleteNamespace
	}
	if strings.TrimSpace(dockerUsername) == "" && strings.TrimSpace(dep.DockerUsername) != "" {
		dockerUsername = dep.DockerUsername
	}
	if strings.TrimSpace(dockerPassword) == "" && strings.TrimSpace(dep.DockerPassword) != "" {
		dockerPassword = dep.DockerPassword
	}
	if dep.EnsureDockerRegistry != nil {
		ensureDockerRegistry = *dep.EnsureDockerRegistry
	}
	if dep.RenderTemplates != nil {
		renderTemplates = *dep.RenderTemplates
	}
	if strings.TrimSpace(renderOutputDir) == "" && strings.TrimSpace(dep.RenderOutputDir) != "" {
		renderOutputDir = dep.RenderOutputDir
	}
	if len(extraValues) == 0 && len(dep.ExtraValues) > 0 {
		extraValues = append(extraValues, dep.ExtraValues...)
	}
	if strings.TrimSpace(repoRoot) == "" && strings.TrimSpace(dep.RepoRoot) != "" {
		repoRoot = dep.RepoRoot
	} else if strings.TrimSpace(repoRoot) == "" && strings.TrimSpace(rc.RepoRoot) != "" {
		repoRoot = rc.RepoRoot
	}
	if strings.TrimSpace(scenarioPath) == "" {
		if strings.TrimSpace(dep.ScenarioPath) != "" {
			scenarioPath = dep.ScenarioPath
		} else if strings.TrimSpace(dep.ScenarioRoot) != "" {
			scenarioPath = dep.ScenarioRoot
		} else if strings.TrimSpace(rc.ScenarioPath) != "" {
			scenarioPath = rc.ScenarioPath
		} else if strings.TrimSpace(rc.ScenarioRoot) != "" {
			scenarioPath = rc.ScenarioRoot
		}
	}
	if strings.TrimSpace(valuesPreset) == "" {
		if strings.TrimSpace(dep.ValuesPreset) != "" {
			valuesPreset = dep.ValuesPreset
		} else if strings.TrimSpace(rc.ValuesPreset) != "" {
			valuesPreset = rc.ValuesPreset
		}
	}
	if strings.TrimSpace(platform) == "" && strings.TrimSpace(rc.Platform) != "" {
		platform = rc.Platform
	}
	// booleans: only if flags were not explicitly set; we approximate by using defaults
	if rc.ExternalSecrets {
		externalSecrets = true
	}
	if rc.SkipDependencyUpdate {
		skipDependencyUpdate = true
	}
	// global defaults mirroring flags (root-level)
	if strings.TrimSpace(chartPath) == "" && strings.TrimSpace(rc.ChartPath) != "" {
		chartPath = rc.ChartPath
	}
	if strings.TrimSpace(chart) == "" && strings.TrimSpace(rc.Chart) != "" {
		chart = rc.Chart
	}
	if strings.TrimSpace(chartVersion) == "" && strings.TrimSpace(rc.Version) != "" {
		chartVersion = rc.Version
	}
	if strings.TrimSpace(namespace) == "" && strings.TrimSpace(rc.Namespace) != "" {
		namespace = rc.Namespace
	}
	if strings.TrimSpace(release) == "" && strings.TrimSpace(rc.Release) != "" {
		release = rc.Release
	}
	if strings.TrimSpace(scenario) == "" && strings.TrimSpace(rc.Scenario) != "" {
		scenario = rc.Scenario
	}
	if strings.TrimSpace(scenarioPath) == "" && strings.TrimSpace(rc.ScenarioPath) != "" {
		scenarioPath = rc.ScenarioPath
	}
	if strings.TrimSpace(auth) == "" && strings.TrimSpace(rc.Auth) != "" {
		auth = rc.Auth
	}
	if strings.TrimSpace(flow) == "" && strings.TrimSpace(rc.Flow) != "" {
		flow = rc.Flow
	}
	if strings.TrimSpace(envFile) == "" && strings.TrimSpace(rc.EnvFile) != "" {
		envFile = rc.EnvFile
	}
	if rc.Interactive != nil {
		interactive = *rc.Interactive
	}
	if strings.TrimSpace(vaultSecretMapping) == "" && strings.TrimSpace(rc.VaultSecretMapping) != "" {
		vaultSecretMapping = rc.VaultSecretMapping
	}
	if rc.AutoGenerateSecrets != nil {
		autoGenerateSecrets = *rc.AutoGenerateSecrets
	}
	if rc.DeleteNamespaceFirst != nil {
		deleteNamespaceFirst = *rc.DeleteNamespaceFirst
	}
	if strings.TrimSpace(dockerUsername) == "" && strings.TrimSpace(rc.DockerUsername) != "" {
		dockerUsername = rc.DockerUsername
	}
	if strings.TrimSpace(dockerPassword) == "" && strings.TrimSpace(rc.DockerPassword) != "" {
		dockerPassword = rc.DockerPassword
	}
	if rc.EnsureDockerRegistry != nil {
		ensureDockerRegistry = *rc.EnsureDockerRegistry
	}
	if rc.RenderTemplates != nil {
		renderTemplates = *rc.RenderTemplates
	}
	if strings.TrimSpace(renderOutputDir) == "" && strings.TrimSpace(rc.RenderOutputDir) != "" {
		renderOutputDir = rc.RenderOutputDir
	}
	if len(extraValues) == 0 && len(rc.ExtraValues) > 0 {
		extraValues = append(extraValues, rc.ExtraValues...)
	}
	// keycloak
	if strings.TrimSpace(keycloakHost) == "" && strings.TrimSpace(rc.Keycloak.Host) != "" {
		keycloakHost = rc.Keycloak.Host
	}
	if strings.TrimSpace(keycloakProtocol) == "" && strings.TrimSpace(rc.Keycloak.Protocol) != "" {
		keycloakProtocol = rc.Keycloak.Protocol
	}
	// log level
	if strings.TrimSpace(logLevel) == "" && strings.TrimSpace(rc.LogLevel) != "" {
		logLevel = rc.LogLevel
	}
	return nil
}

func main() {
	rootCmd := &cobra.Command{
		Use:   "deploy-camunda",
		Short: "Deploy Camunda Platform with prepared Helm values",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Skip for config subcommands
			if cmd != nil {
				if cmd.Name() == "config" || (cmd.Parent() != nil && cmd.Parent().Name() == "config") {
					return nil
				}
			}
			// Try to load .env file
			if envFile != "" {
				_ = env.Load(envFile)
			} else {
				// Try default locations if not specified
				_ = env.Load(".env")
			}
			// Skip validations for shell completion invocations
			if cmd != nil {
				if cmd.Name() == "completion" ||
					cmd.Name() == cobra.ShellCompRequestCmd ||
					cmd.Name() == cobra.ShellCompNoDescRequestCmd {
					return nil
				}
			}
			// Load config and apply active deployment defaults before validations
			cfgPath, err := resolveConfigPath(configFile)
			if err != nil {
				return err
			}
			rc, err := readConfig(cfgPath, true)
			if err != nil {
				return err
			}

			// Apply active deployment if present
			if err := applyActiveDeployment(rc, rc.Current); err != nil {
				return err
			}
			// Ensure at least one of chart-path or chart is provided
			if chartPath == "" && chart == "" {
				return fmt.Errorf("either --chart-path or --chart must be provided")
			}
			// Validate --version compatibility
			if strings.TrimSpace(chartVersion) != "" && strings.TrimSpace(chart) == "" && strings.TrimSpace(chartPath) != "" {
				// version provided but chart is not; user likely provided chart-path explicitly
				return fmt.Errorf("--version requires --chart to be set; do not combine --version with only --chart-path")
			}
			if strings.TrimSpace(chartVersion) != "" && strings.TrimSpace(chart) == "" && strings.TrimSpace(chartPath) == "" {
				return fmt.Errorf("--version requires --chart to be set")
			}
			// Helpful validation of derived chartPath
			if strings.TrimSpace(chartPath) != "" {
				if fi, err := os.Stat(chartPath); err != nil || !fi.IsDir() {
					return fmt.Errorf("resolved chart path %q does not exist or is not a directory; set --repo-root/--chart/--version or --chart-path explicitly", chartPath)
				}
			}
			// Validate required runtime identifiers (allowing config-derived values)
			if strings.TrimSpace(namespace) == "" {
				return fmt.Errorf("namespace not set; provide -n/--namespace or set 'namespace' in the active deployment/root config")
			}
			if strings.TrimSpace(release) == "" {
				return fmt.Errorf("release not set; provide -r/--release or set 'release' in the active deployment/root config")
			}
			if strings.TrimSpace(scenario) == "" {
				return fmt.Errorf("scenario not set; provide -s/--scenario or set 'scenario' in the active deployment/root config")
			}
			return nil
		},
		RunE: run,
	}

	// completion subcommand (bash|zsh|fish|powershell)
	completionCmd := &cobra.Command{
		Use:       "completion [bash|zsh|fish|powershell]",
		Short:     "Generate shell completion scripts",
		Args:      cobra.ExactValidArgs(1),
		ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return rootCmd.GenBashCompletion(os.Stdout)
			case "zsh":
				return rootCmd.GenZshCompletion(os.Stdout)
			case "fish":
				return rootCmd.GenFishCompletion(os.Stdout, true)
			case "powershell":
				return rootCmd.GenPowerShellCompletionWithDesc(os.Stdout)
			default:
				return nil
			}
		},
	}
	rootCmd.AddCommand(completionCmd)

	// config subcommand (list/show/use)
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Manage deploy-camunda configuration and active deployment",
	}
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List configured deployments",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath, err := resolveConfigPath(configFile)
			if err != nil {
				return err
			}
			rc, err := readConfig(cfgPath, false)
			if err != nil {
				return err
			}
			active := rc.Current
			for name := range rc.Deployments {
				marker := " "
				if name == active {
					marker = "*"
				}
				fmt.Fprintf(os.Stdout, "%s %s\n", marker, name)
			}
			if len(rc.Deployments) == 0 {
				fmt.Fprintln(os.Stdout, "(no deployments configured)")
			}
			return nil
		},
	}
	showCmd := &cobra.Command{
		Use:               "show [name|current]",
		Short:             "Show a deployment (merged with defaults)",
		Args:              cobra.RangeArgs(0, 1),
		ValidArgsFunction: completeDeploymentNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath, err := resolveConfigPath(configFile)
			if err != nil {
				return err
			}
			rc, err := readConfig(cfgPath, false)
			if err != nil {
				return err
			}
			name := ""
			if len(args) == 0 || args[0] == "current" {
				name = rc.Current
			} else {
				name = args[0]
			}
			if strings.TrimSpace(name) == "" {
				return fmt.Errorf("no deployment selected; set a current one with: deploy-camunda config use <name>")
			}
			dep, ok := rc.Deployments[name]
			if !ok {
				return fmt.Errorf("deployment %q not found", name)
			}
			// merge view
			chartStr := firstNonEmpty(dep.Chart, "")
			versionStr := firstNonEmpty(dep.Version, "")
			scenarioStr := firstNonEmpty(dep.Scenario, "")
			repoRootStr := firstNonEmpty(dep.RepoRoot, rc.RepoRoot)
			scenarioRootStr := firstNonEmpty(dep.ScenarioRoot, rc.ScenarioRoot)
			valuesPresetStr := firstNonEmpty(dep.ValuesPreset, rc.ValuesPreset)
			platformStr := rc.Platform
			logLevelStr := rc.LogLevel

			// If stdout is not a terminal, output YAML for scripting
			if !logging.IsTerminal(os.Stdout.Fd()) {
				view := map[string]any{
					"name":         name,
					"chart":        chartStr,
					"version":      versionStr,
					"scenario":     scenarioStr,
					"repoRoot":     repoRootStr,
					"scenarioRoot": scenarioRootStr,
					"valuesPreset": valuesPresetStr,
					"platform":     platformStr,
					"logLevel":     logLevelStr,
				}
				out, err := yaml.Marshal(view)
				if err != nil {
					return err
				}
				fmt.Fprint(os.Stdout, string(out))
				return nil
			}

			// Pretty, colored terminal output
			styleKey := func(s string) string { return logging.Emphasize(s, gchalk.Cyan) }
			styleVal := func(s string) string { return logging.Emphasize(s, gchalk.Magenta) }
			styleHead := func(s string) string { return logging.Emphasize(s, gchalk.Bold) }

			type kv struct{ k, v string }
			rows := []kv{
				{"name", name},
				{"chart", chartStr},
				{"version", versionStr},
				{"scenario", scenarioStr},
				{"repoRoot", repoRootStr},
				{"scenarioRoot", scenarioRootStr},
				{"valuesPreset", valuesPresetStr},
				{"platform", platformStr},
				{"logLevel", logLevelStr},
			}
			maxKey := 0
			for _, r := range rows {
				if len(r.k) > maxKey {
					maxKey = len(r.k)
				}
			}
			var b strings.Builder
			b.WriteString(styleHead(fmt.Sprintf("Deployment %s", name)))
			b.WriteString("\n")
			for _, r := range rows {
				keyPadded := fmt.Sprintf("%-*s", maxKey, r.k)
				fmt.Fprintf(&b, "  - %s: %s\n", styleKey(keyPadded), styleVal(r.v))
			}
			fmt.Fprint(os.Stdout, b.String())
			return nil
		},
	}
	useCmd := &cobra.Command{
		Use:               "use <name>",
		Short:             "Set the active deployment",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeDeploymentNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			cfgPath, err := resolveConfigPath(configFile)
			if err != nil {
				return err
			}
			rc, err := readConfig(cfgPath, false)
			if err != nil {
				return err
			}
			if rc.Deployments == nil {
				return fmt.Errorf("no deployments configured in %q", cfgPath)
			}
			if _, ok := rc.Deployments[name]; !ok {
				return fmt.Errorf("deployment %q not found in %q", name, cfgPath)
			}
			// Update only the "current" field in the YAML to avoid overwriting other keys or styles
			if err := writeConfigCurrentOnly(cfgPath, name); err != nil {
				return err
			}
			fmt.Fprintf(os.Stdout, "Now using deployment %q in %s\n", name, cfgPath)
			return nil
		},
	}
	configCmd.AddCommand(listCmd, showCmd, useCmd)
	rootCmd.AddCommand(configCmd)

	flags := rootCmd.Flags()
	// Make --config persistent so it applies to subcommands like `config`
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "F", "", "Path to config file (.camunda-deploy.yaml or ~/.config/camunda/deploy.yaml)")
	flags.StringVar(&chartPath, "chart-path", "", "Path to the Camunda chart directory")
	flags.StringVarP(&chart, "chart", "c", "", "Chart name")
	flags.StringVarP(&chartVersion, "version", "v", "", "Chart version (only valid with --chart; not allowed with --chart-path)")
	flags.StringVarP(&namespace, "namespace", "n", "", "Kubernetes namespace")
	flags.StringVarP(&release, "release", "r", "", "Helm release name")
	flags.StringVarP(&scenario, "scenario", "s", "", "The name of the scneario to deploy")
	flags.StringVar(&scenarioPath, "scenario-path", "", "Path to scenario files")
	flags.StringVar(&auth, "auth", "keycloak", "Auth scenario")
	flags.StringVar(&platform, "platform", "gke", "Target platform: gke, rosa, eks")
	flags.StringVarP(&logLevel, "log-level", "l", "info", "Log level")
	flags.BoolVar(&skipDependencyUpdate, "skip-dependency-update", true, "Skip Helm dependency update")
	flags.BoolVar(&externalSecrets, "external-secrets", true, "Enable external secrets")
	flags.StringVar(&keycloakHost, "keycloak-host", "keycloak-24-9-0.ci.distro.ultrawombat.com", "Keycloak external host")
	flags.StringVar(&keycloakProtocol, "keycloak-protocol", "https", "Keycloak protocol")
	flags.StringVar(&repoRoot, "repo-root", "", "Repository root path")
	flags.StringVar(&flow, "flow", "install", "Flow type")
	flags.StringVar(&envFile, "env-file", "", "Path to .env file (defaults to .env in current dir)")
	flags.BoolVar(&interactive, "interactive", true, "Enable interactive prompts for missing variables")
	flags.StringVar(&vaultSecretMapping, "vault-secret-mapping", "", "Vault secret mapping content")
	flags.BoolVar(&autoGenerateSecrets, "auto-generate-secrets", false, "Auto-generate certain secrets for testing purposes")
	flags.BoolVar(&deleteNamespaceFirst, "delete-namespace", false, "Delete the namespace first, then deploy")
	flags.StringVar(&dockerUsername, "docker-username", "", "Docker registry username")
	flags.StringVar(&dockerPassword, "docker-password", "", "Docker registry password")
	flags.BoolVar(&ensureDockerRegistry, "ensure-docker-registry", false, "Ensure Docker registry secret is created")
	flags.BoolVar(&renderTemplates, "render-templates", false, "Render manifests to a directory instead of installing")
	flags.StringVar(&renderOutputDir, "render-output-dir", "", "Output directory for rendered manifests (defaults to ./rendered/<release>)")
	flags.StringSliceVar(&extraValues, "extra-values", nil, "Additional Helm values files to apply last (comma-separated or repeatable)")
	flags.StringVar(&valuesPreset, "values-preset", "", "Shortcut to append values-<preset>.yaml from chartPath if present (e.g. latest, enterprise)")

	// Do not mark flags required; we validate after loading config so config values can satisfy them

	completion.RegisterScenarioCompletion(rootCmd, "scenario", "chart-path")
	completion.RegisterScenarioCompletion(rootCmd, "auth", "chart-path")
	registerDeploymentNameCompletion(configCmd, "show")
	registerDeploymentNameCompletion(configCmd, "use")

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func completeDeploymentNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	cfgPath, err := resolveConfigPath(configFile)
	if err != nil {
		return nil, cobra.ShellCompDirectiveDefault
	}
	rc, err := readConfig(cfgPath, false)
	if err != nil {
		return nil, cobra.ShellCompDirectiveDefault
	}
	var names []string
	for name := range rc.Deployments {
		if toComplete == "" || strings.HasPrefix(name, toComplete) {
			names = append(names, name)
		}
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}

func registerDeploymentNameCompletion(parent *cobra.Command, name string) {
	if c := parent.Commands(); len(c) > 0 {
		for _, sub := range c {
			if sub.Name() == name {
				sub.ValidArgsFunction = completeDeploymentNames
			}
		}
	}
}

func generateRandomSuffix() string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	result := make([]byte, 8)
	for i := range result {
		num, _ := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		result[i] = chars[num.Int64()]
	}
	return string(result)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// maskIfSet returns a masked placeholder when a sensitive value is set.
// This prevents leaking secrets while still indicating that a value exists.
func maskIfSet(val string) string {
	if val == "" {
		return ""
	}
	return "***"
}

func run(cmd *cobra.Command, args []string) error {
	// Setup logging
	if err := logging.Setup(logging.Options{
		LevelString:  logLevel,
		ColorEnabled: logging.IsTerminal(os.Stdout.Fd()),
	}); err != nil {
		return err
	}

	// Log flags as a colored, multi-line list (sensitive values are masked).
	// Iterate over the Cobra flag set to avoid duplication and keep logs in sync with flags.
	styleKey := func(s string) string { return logging.Emphasize(s, gchalk.Cyan) }
	styleVal := func(s string) string { return logging.Emphasize(s, gchalk.Magenta) }
	stylePwd := func(s string) string { return logging.Emphasize(s, gchalk.Yellow) }
	styleBool := func(s string) string {
		if strings.EqualFold(s, "true") || s == "1" {
			return logging.Emphasize("true", gchalk.Green)
		}
		return logging.Emphasize("false", gchalk.Red)
	}
	styleHead := func(s string) string { return logging.Emphasize(s, gchalk.Bold) }

	var b strings.Builder
	b.WriteString(styleHead("Starting deployment with flags:"))
	b.WriteString("\n")

	printFlag := func(f *pflag.Flag) {
		name := f.Name // actual CLI flag name
		val := f.Value.String()
		typ := f.Value.Type()

		// Sensitive handling
		switch name {
		case "docker-password":
			val = stylePwd(maskIfSet(val))
		case "vault-secret-mapping":
			if strings.TrimSpace(val) != "" {
				val = styleVal("provided")
			} else {
				val = styleVal("not-provided")
			}
		default:
			if typ == "bool" {
				val = styleBool(val)
			} else {
				val = styleVal(val)
			}
		}
		fmt.Fprintf(&b, "  - %s: %s\n", styleKey(name), val)
	}

	// Visit all flags on the current command (sorted and without duplication)
	if cmd != nil && cmd.Flags() != nil {
		cmd.Flags().VisitAll(printFlag)
	}
	logging.Logger.Info().Msg(b.String())

	// Identifiers
	suffix := generateRandomSuffix()
	realmName := fmt.Sprintf("%s-%s", namespace, suffix)
	optimizePrefix := fmt.Sprintf("opt-%s", suffix)
	orchestrationPrefix := fmt.Sprintf("orch-%s", suffix)

	logging.Logger.Info().Str("realm", realmName).Str("optimize", optimizePrefix).Str("orchestration", orchestrationPrefix).Msg("Generated identifiers")

	// Temp directory
	tempDir, err := os.MkdirTemp("", "camunda-values-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)
	logging.Logger.Info().Str("dir", tempDir).Msg("Created temporary values directory")

	// Set Env Vars for prepare-helm-values
	os.Setenv("KEYCLOAK_REALM", realmName)
	os.Setenv("OPTIMIZE_INDEX_PREFIX", optimizePrefix)
	os.Setenv("ORCHESTRATION_INDEX_PREFIX", orchestrationPrefix)
	os.Setenv("FLOW", flow)

	// Keycloak Env Vars
	if keycloakHost != "" {
		// Hardcoded version from script
		kcVersionSafe := "24_9_0"
		kcHostVar := fmt.Sprintf("KEYCLOAK_EXT_HOST_%s", kcVersionSafe)
		kcProtoVar := fmt.Sprintf("KEYCLOAK_EXT_PROTOCOL_%s", kcVersionSafe)

		os.Setenv(kcHostVar, keycloakHost)
		os.Setenv(kcProtoVar, keycloakProtocol)
	}

	// Prepare Values
	processValues := func(scen string) error {
		opts := values.Options{
			ChartPath:   chartPath,
			Scenario:    scen,
			ScenarioDir: scenarioPath,
			OutputDir:   tempDir,
			Interactive: interactive,
			EnvFile:     envFile,
		}
		if opts.EnvFile == "" {
			opts.EnvFile = ".env"
		}

		file, err := values.ResolveValuesFile(opts)
		if err != nil {
			return err
		}
		_, _, err = values.Process(file, opts)
		return err
	}

	if auth != "" && auth != scenario {
		logging.Logger.Info().Str("scenario", auth).Msg("Preparing auth scenario")
		if err := processValues(auth); err != nil {
			return err
		}
	}

	logging.Logger.Info().Str("scenario", scenario).Msg("Preparing main scenario")
	if err := processValues(scenario); err != nil {
		return err
	}

	if autoGenerateSecrets {
		vaultSecretMapping = "ci/path DISTRO_QA_E2E_TESTS_IDENTITY_FIRSTUSER_PASSWORD;ci/path DISTRO_QA_E2E_TESTS_IDENTITY_SECONDUSER_PASSWORD;ci/path DISTRO_QA_E2E_TESTS_IDENTITY_THIRDUSER_PASSWORD;ci/path DISTRO_QA_E2E_TESTS_KEYCLOAK_CLIENTS_SECRET;"
		// This is overriding the .env values for testing purposes
		firstUserPwd := rand.Text()
		secondUserPwd := rand.Text()
		thirdUserPwd := rand.Text()
		keycloakClientsSecret := rand.Text()

		os.Setenv("DISTRO_QA_E2E_TESTS_IDENTITY_FIRSTUSER_PASSWORD", firstUserPwd)
		os.Setenv("DISTRO_QA_E2E_TESTS_IDENTITY_SECONDUSER_PASSWORD", secondUserPwd)
		os.Setenv("DISTRO_QA_E2E_TESTS_IDENTITY_THIRDUSER_PASSWORD", thirdUserPwd)
		os.Setenv("DISTRO_QA_E2E_TESTS_KEYCLOAK_CLIENTS_SECRET", keycloakClientsSecret)

		// Persist the generated secrets to the .env file
		targetEnvFile := envFile
		if targetEnvFile == "" {
			targetEnvFile = ".env"
		}
		type pair struct{ key, val string }
		toPersist := []pair{
			{"DISTRO_QA_E2E_TESTS_IDENTITY_FIRSTUSER_PASSWORD", firstUserPwd},
			{"DISTRO_QA_E2E_TESTS_IDENTITY_SECONDUSER_PASSWORD", secondUserPwd},
			{"DISTRO_QA_E2E_TESTS_IDENTITY_THIRDUSER_PASSWORD", thirdUserPwd},
			{"DISTRO_QA_E2E_TESTS_KEYCLOAK_CLIENTS_SECRET", keycloakClientsSecret},
		}
		for _, p := range toPersist {
			if err := env.Append(targetEnvFile, p.key, p.val); err != nil {
				logging.Logger.Warn().Err(err).Str("key", p.key).Str("path", targetEnvFile).Msg("Failed to persist generated secret to .env")
			} else {
				logging.Logger.Info().Str("key", p.key).Str("path", targetEnvFile).Msg("Persisted generated secret to .env")
			}
		}
	}
	// Generate Vault Secrets
	var vaultSecretPath string
	if vaultSecretMapping != "" {
		vaultSecretPath = filepath.Join(tempDir, "vault-mapped-secrets.yaml")
		logging.Logger.Info().Msg("Generating vault secrets")

		if err := mapper.Generate(vaultSecretMapping, "vault-mapped-secrets", vaultSecretPath); err != nil {
			return fmt.Errorf("failed to generate vault secrets: %w", err)
		}
	}

	// Deploy
	if deleteNamespaceFirst {
		logging.Logger.Info().Str("namespace", namespace).Msg("Deleting namespace prior to deployment as requested")
		if err := kube.DeleteNamespace(context.Background(), "", "", namespace); err != nil {
			return fmt.Errorf("failed to delete namespace %q: %w", namespace, err)
		}
	}
	vals, err := deployer.BuildValuesList(tempDir, []string{scenario}, auth, false, false, extraValues)
	if err != nil {
		return err
	}

	deployOpts := types.Options{
		ChartPath:              chartPath,
		Chart:                  chart,
		Version:                chartVersion,
		ReleaseName:            release,
		Namespace:              namespace,
		Wait:                   true,
		Atomic:                 true,
		Timeout:                15 * time.Minute,
		ValuesFiles:            vals,
		EnsureDockerRegistry:   ensureDockerRegistry,
		SkipDependencyUpdate:   skipDependencyUpdate,
		ExternalSecretsEnabled: externalSecrets,
		DockerRegistryUsername: dockerUsername,
		DockerRegistryPassword: dockerPassword,
		Platform:               platform,
		RepoRoot:               repoRoot,
		Identifier:             fmt.Sprintf("%s-%s", release, time.Now().Format("20060102150405")),
		TTL:                    "30m",
		LoadKeycloakRealm:      true,
		KeycloakRealmName:      realmName,
		RenderTemplates:        renderTemplates,
		RenderOutputDir:        renderOutputDir,
		IncludeCRDs:            true,
		CIMetadata: types.CIMetadata{
			Flow: flow,
		},
		ApplyIntegrationCreds: true,
		VaultSecretPath:       vaultSecretPath,
	}

	err = deployer.Deploy(context.Background(), deployOpts)
	if err != nil {
		return err
	}

	// Print out the details of the deployment including the auto generated secrets
	firstPwd := os.Getenv("DISTRO_QA_E2E_TESTS_IDENTITY_FIRSTUSER_PASSWORD")
	secondPwd := os.Getenv("DISTRO_QA_E2E_TESTS_IDENTITY_SECONDUSER_PASSWORD")
	thirdPwd := os.Getenv("DISTRO_QA_E2E_TESTS_IDENTITY_THIRDUSER_PASSWORD")
	clientSecret := os.Getenv("DISTRO_QA_E2E_TESTS_KEYCLOAK_CLIENTS_SECRET")

	if !logging.IsTerminal(os.Stdout.Fd()) {
		// Plain, machine-friendly output
		var out strings.Builder
		fmt.Fprintf(&out, "deployment: success\n")
		fmt.Fprintf(&out, "realm: %s\n", realmName)
		fmt.Fprintf(&out, "optimizeIndexPrefix: %s\n", optimizePrefix)
		fmt.Fprintf(&out, "orchestrationIndexPrefix: %s\n", orchestrationPrefix)
		fmt.Fprintf(&out, "credentials:\n")
		fmt.Fprintf(&out, "  firstUserPassword: %s\n", firstPwd)
		fmt.Fprintf(&out, "  secondUserPassword: %s\n", secondPwd)
		fmt.Fprintf(&out, "  thirdUserPassword: %s\n", thirdPwd)
		fmt.Fprintf(&out, "  keycloakClientsSecret: %s\n", clientSecret)
		logging.Logger.Info().Msg(out.String())
		return nil
	}

	// Pretty, human-friendly output
	styleOk := func(s string) string { return logging.Emphasize(s, gchalk.Green) }

	var out strings.Builder
	out.WriteString(styleOk("🎉 Deployment completed successfully"))
	out.WriteString("\n\n")

	// Identifiers
	out.WriteString(styleHead("Identifiers"))
	out.WriteString("\n")
	maxKey := 25
	fmt.Fprintf(&out, "  - %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "Realm")), styleVal(realmName))
	fmt.Fprintf(&out, "  - %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "Optimize index prefix")), styleVal(optimizePrefix))
	fmt.Fprintf(&out, "  - %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "Orchestration index prefix")), styleVal(orchestrationPrefix))

	out.WriteString("\n")
	out.WriteString(styleHead("Test credentials"))
	out.WriteString("\n")
	fmt.Fprintf(&out, "  - %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "First user password")), styleVal(firstPwd))
	fmt.Fprintf(&out, "  - %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "Second user password")), styleVal(secondPwd))
	fmt.Fprintf(&out, "  - %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "Third user password")), styleVal(thirdPwd))
	fmt.Fprintf(&out, "  - %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "Keycloak clients secret")), styleVal(clientSecret))

	out.WriteString("\n")
	out.WriteString("Please keep these credentials safe. If you have any questions, refer to the documentation or reach out for support. 🚀")

	logging.Logger.Info().Msg(out.String())
	return nil
}
