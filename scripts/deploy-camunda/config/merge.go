package config

import (
	"fmt"
	"strings"
)

// RuntimeFlags holds all CLI flag values that can be merged with config.
type RuntimeFlags struct {
	ChartPath                string
	Chart                    string
	ChartVersion             string
	Namespace                string
	Release                  string
	Scenario                 string
	ScenarioPath             string
	Auth                     string
	Platform                 string
	LogLevel                 string
	SkipDependencyUpdate     bool
	ExternalSecrets          bool
	KeycloakHost             string
	KeycloakProtocol         string
	KeycloakRealm            string
	OptimizeIndexPrefix      string
	OrchestrationIndexPrefix string
	TasklistIndexPrefix      string
	OperateIndexPrefix       string
	IngressHost              string
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

	// Apply deployment-specific values
	MergeStringField(&flags.ChartPath, dep.ChartPath, rc.ChartPath)
	MergeStringField(&flags.Chart, dep.Chart, rc.Chart)
	MergeStringField(&flags.ChartVersion, dep.Version, rc.Version)
	MergeStringField(&flags.Namespace, dep.Namespace, rc.Namespace)
	MergeStringField(&flags.Release, dep.Release, rc.Release)
	MergeStringField(&flags.Scenario, dep.Scenario, rc.Scenario)
	MergeStringField(&flags.Auth, dep.Auth, rc.Auth)
	MergeStringField(&flags.Platform, dep.Platform, rc.Platform)
	MergeStringField(&flags.LogLevel, dep.LogLevel, rc.LogLevel)
	MergeStringField(&flags.Flow, dep.Flow, rc.Flow)
	MergeStringField(&flags.EnvFile, dep.EnvFile, rc.EnvFile)
	MergeStringField(&flags.VaultSecretMapping, dep.VaultSecretMapping, rc.VaultSecretMapping)
	MergeStringField(&flags.DockerUsername, dep.DockerUsername, rc.DockerUsername)
	MergeStringField(&flags.DockerPassword, dep.DockerPassword, rc.DockerPassword)
	MergeStringField(&flags.RenderOutputDir, dep.RenderOutputDir, rc.RenderOutputDir)
	MergeStringField(&flags.RepoRoot, dep.RepoRoot, rc.RepoRoot)
	MergeStringField(&flags.ValuesPreset, dep.ValuesPreset, rc.ValuesPreset)
	MergeStringField(&flags.KeycloakRealm, dep.KeycloakRealm, rc.KeycloakRealm)
	MergeStringField(&flags.OptimizeIndexPrefix, dep.OptimizeIndexPrefix, rc.OptimizeIndexPrefix)
	MergeStringField(&flags.OrchestrationIndexPrefix, dep.OrchestrationIndexPrefix, rc.OrchestrationIndexPrefix)
	MergeStringField(&flags.TasklistIndexPrefix, dep.TasklistIndexPrefix, rc.TasklistIndexPrefix)
	MergeStringField(&flags.OperateIndexPrefix, dep.OperateIndexPrefix, rc.OperateIndexPrefix)

	// ScenarioPath special handling
	if strings.TrimSpace(flags.ScenarioPath) == "" {
		flags.ScenarioPath = firstNonEmpty(dep.ScenarioPath, dep.ScenarioRoot, rc.ScenarioPath, rc.ScenarioRoot)
	}

	// Boolean fields - apply if flag wasn't explicitly set
	MergeBoolField(&flags.ExternalSecrets, dep.ExternalSecrets, boolPtr(rc.ExternalSecrets))
	MergeBoolField(&flags.SkipDependencyUpdate, dep.SkipDependencyUpdate, boolPtr(rc.SkipDependencyUpdate))
	MergeBoolField(&flags.Interactive, dep.Interactive, rc.Interactive)
	MergeBoolField(&flags.AutoGenerateSecrets, dep.AutoGenerateSecrets, rc.AutoGenerateSecrets)
	MergeBoolField(&flags.DeleteNamespaceFirst, dep.DeleteNamespace, rc.DeleteNamespaceFirst)
	MergeBoolField(&flags.EnsureDockerRegistry, dep.EnsureDockerRegistry, rc.EnsureDockerRegistry)
	MergeBoolField(&flags.RenderTemplates, dep.RenderTemplates, rc.RenderTemplates)

	// Slice fields
	MergeStringSliceField(&flags.ExtraValues, dep.ExtraValues, rc.ExtraValues)

	// Keycloak
	MergeStringField(&flags.KeycloakHost, "", rc.Keycloak.Host)
	MergeStringField(&flags.KeycloakProtocol, "", rc.Keycloak.Protocol)

	return nil
}

// applyRootDefaults applies only root-level defaults when no deployment is active.
func applyRootDefaults(rc *RootConfig, flags *RuntimeFlags) error {
	if rc == nil {
		return nil
	}

	MergeStringField(&flags.ChartPath, "", rc.ChartPath)
	MergeStringField(&flags.Chart, "", rc.Chart)
	MergeStringField(&flags.ChartVersion, "", rc.Version)
	MergeStringField(&flags.Namespace, "", rc.Namespace)
	MergeStringField(&flags.Release, "", rc.Release)
	MergeStringField(&flags.Scenario, "", rc.Scenario)
	MergeStringField(&flags.ScenarioPath, "", firstNonEmpty(rc.ScenarioPath, rc.ScenarioRoot))
	MergeStringField(&flags.Auth, "", rc.Auth)
	MergeStringField(&flags.Platform, "", rc.Platform)
	MergeStringField(&flags.LogLevel, "", rc.LogLevel)
	MergeStringField(&flags.Flow, "", rc.Flow)
	MergeStringField(&flags.EnvFile, "", rc.EnvFile)
	MergeStringField(&flags.VaultSecretMapping, "", rc.VaultSecretMapping)
	MergeStringField(&flags.DockerUsername, "", rc.DockerUsername)
	MergeStringField(&flags.DockerPassword, "", rc.DockerPassword)
	MergeStringField(&flags.RenderOutputDir, "", rc.RenderOutputDir)
	MergeStringField(&flags.RepoRoot, "", rc.RepoRoot)
	MergeStringField(&flags.ValuesPreset, "", rc.ValuesPreset)
	MergeStringField(&flags.KeycloakRealm, "", rc.KeycloakRealm)
	MergeStringField(&flags.OptimizeIndexPrefix, "", rc.OptimizeIndexPrefix)
	MergeStringField(&flags.OrchestrationIndexPrefix, "", rc.OrchestrationIndexPrefix)
	MergeStringField(&flags.TasklistIndexPrefix, "", rc.TasklistIndexPrefix)
	MergeStringField(&flags.OperateIndexPrefix, "", rc.OperateIndexPrefix)

	if rc.ExternalSecrets {
		flags.ExternalSecrets = true
	}
	if rc.SkipDependencyUpdate {
		flags.SkipDependencyUpdate = true
	}

	MergeBoolField(&flags.Interactive, nil, rc.Interactive)
	MergeBoolField(&flags.AutoGenerateSecrets, nil, rc.AutoGenerateSecrets)
	MergeBoolField(&flags.DeleteNamespaceFirst, nil, rc.DeleteNamespaceFirst)
	MergeBoolField(&flags.EnsureDockerRegistry, nil, rc.EnsureDockerRegistry)
	MergeBoolField(&flags.RenderTemplates, nil, rc.RenderTemplates)

	MergeStringSliceField(&flags.ExtraValues, nil, rc.ExtraValues)

	MergeStringField(&flags.KeycloakHost, "", rc.Keycloak.Host)
	MergeStringField(&flags.KeycloakProtocol, "", rc.Keycloak.Protocol)

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

	return nil
}
