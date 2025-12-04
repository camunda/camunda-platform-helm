package config

import (
	"fmt"
	"scripts/deploy-camunda/internal/util"
	"strings"
)

// RuntimeFlags holds all CLI flag values that can be merged with config.
type RuntimeFlags struct {
	ChartPath                string
	Chart                    string
	ChartVersion             string
	Namespace                string
	Release                  string
	Scenario                 string   // Single scenario or comma-separated list
	Scenarios                []string // Parsed list of scenarios (populated by Validate)
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
	Timeout                  int  // Timeout in minutes for Helm deployment
	DryRun                   bool // Preview deployment without executing
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

	// Use fluent merger for cleaner code
	NewFieldMerger(flags).
		MergeStrings(
			// Chart identification
			S(&flags.ChartPath, dep.ChartPath, rc.ChartPath),
			S(&flags.Chart, dep.Chart, rc.Chart),
			S(&flags.ChartVersion, dep.Version, rc.Version),
			// Deployment identifiers
			S(&flags.Namespace, dep.Namespace, rc.Namespace),
			S(&flags.Release, dep.Release, rc.Release),
			S(&flags.Scenario, dep.Scenario, rc.Scenario),
			S(&flags.Auth, dep.Auth, rc.Auth),
			// Environment settings
			S(&flags.Platform, dep.Platform, rc.Platform),
			S(&flags.LogLevel, dep.LogLevel, rc.LogLevel),
			S(&flags.Flow, dep.Flow, rc.Flow),
			S(&flags.EnvFile, dep.EnvFile, rc.EnvFile),
			// Secrets and Docker
			S(&flags.VaultSecretMapping, dep.VaultSecretMapping, rc.VaultSecretMapping),
			S(&flags.DockerUsername, dep.DockerUsername, rc.DockerUsername),
			S(&flags.DockerPassword, dep.DockerPassword, rc.DockerPassword),
			// Output and paths
			S(&flags.RenderOutputDir, dep.RenderOutputDir, rc.RenderOutputDir),
			S(&flags.RepoRoot, dep.RepoRoot, rc.RepoRoot),
			S(&flags.ValuesPreset, dep.ValuesPreset, rc.ValuesPreset),
			// Elasticsearch index prefixes
			S(&flags.KeycloakRealm, dep.KeycloakRealm, rc.KeycloakRealm),
			S(&flags.OptimizeIndexPrefix, dep.OptimizeIndexPrefix, rc.OptimizeIndexPrefix),
			S(&flags.OrchestrationIndexPrefix, dep.OrchestrationIndexPrefix, rc.OrchestrationIndexPrefix),
			S(&flags.TasklistIndexPrefix, dep.TasklistIndexPrefix, rc.TasklistIndexPrefix),
			S(&flags.OperateIndexPrefix, dep.OperateIndexPrefix, rc.OperateIndexPrefix),
			// Keycloak
			S(&flags.KeycloakHost, "", rc.Keycloak.Host),
			S(&flags.KeycloakProtocol, "", rc.Keycloak.Protocol),
		).
		MergeBools(
			B(&flags.ExternalSecrets, dep.ExternalSecrets, boolPtr(rc.ExternalSecrets)),
			B(&flags.SkipDependencyUpdate, dep.SkipDependencyUpdate, boolPtr(rc.SkipDependencyUpdate)),
			B(&flags.Interactive, dep.Interactive, rc.Interactive),
			B(&flags.AutoGenerateSecrets, dep.AutoGenerateSecrets, rc.AutoGenerateSecrets),
			B(&flags.DeleteNamespaceFirst, dep.DeleteNamespace, rc.DeleteNamespaceFirst),
			B(&flags.EnsureDockerRegistry, dep.EnsureDockerRegistry, rc.EnsureDockerRegistry),
			B(&flags.RenderTemplates, dep.RenderTemplates, rc.RenderTemplates),
		).
		MergeSlices(
			Sl(&flags.ExtraValues, dep.ExtraValues, rc.ExtraValues),
		)

	// ScenarioPath special handling (4-way merge)
	if util.IsEmpty(flags.ScenarioPath) {
		flags.ScenarioPath = util.FirstNonEmpty(dep.ScenarioPath, dep.ScenarioRoot, rc.ScenarioPath, rc.ScenarioRoot)
	}

	return nil
}

// applyRootDefaults applies only root-level defaults when no deployment is active.
func applyRootDefaults(rc *RootConfig, flags *RuntimeFlags) error {
	if rc == nil {
		return nil
	}

	// Use fluent merger for cleaner code
	NewFieldMerger(flags).
		MergeStrings(
			// Chart identification
			S(&flags.ChartPath, "", rc.ChartPath),
			S(&flags.Chart, "", rc.Chart),
			S(&flags.ChartVersion, "", rc.Version),
			// Deployment identifiers
			S(&flags.Namespace, "", rc.Namespace),
			S(&flags.Release, "", rc.Release),
			S(&flags.Scenario, "", rc.Scenario),
			S(&flags.ScenarioPath, "", util.FirstNonEmpty(rc.ScenarioPath, rc.ScenarioRoot)),
			S(&flags.Auth, "", rc.Auth),
			// Environment settings
			S(&flags.Platform, "", rc.Platform),
			S(&flags.LogLevel, "", rc.LogLevel),
			S(&flags.Flow, "", rc.Flow),
			S(&flags.EnvFile, "", rc.EnvFile),
			// Secrets and Docker
			S(&flags.VaultSecretMapping, "", rc.VaultSecretMapping),
			S(&flags.DockerUsername, "", rc.DockerUsername),
			S(&flags.DockerPassword, "", rc.DockerPassword),
			// Output and paths
			S(&flags.RenderOutputDir, "", rc.RenderOutputDir),
			S(&flags.RepoRoot, "", rc.RepoRoot),
			S(&flags.ValuesPreset, "", rc.ValuesPreset),
			// Elasticsearch index prefixes
			S(&flags.KeycloakRealm, "", rc.KeycloakRealm),
			S(&flags.OptimizeIndexPrefix, "", rc.OptimizeIndexPrefix),
			S(&flags.OrchestrationIndexPrefix, "", rc.OrchestrationIndexPrefix),
			S(&flags.TasklistIndexPrefix, "", rc.TasklistIndexPrefix),
			S(&flags.OperateIndexPrefix, "", rc.OperateIndexPrefix),
			// Keycloak
			S(&flags.KeycloakHost, "", rc.Keycloak.Host),
			S(&flags.KeycloakProtocol, "", rc.Keycloak.Protocol),
		).
		MergeBools(
			B(&flags.Interactive, nil, rc.Interactive),
			B(&flags.AutoGenerateSecrets, nil, rc.AutoGenerateSecrets),
			B(&flags.DeleteNamespaceFirst, nil, rc.DeleteNamespaceFirst),
			B(&flags.EnsureDockerRegistry, nil, rc.EnsureDockerRegistry),
			B(&flags.RenderTemplates, nil, rc.RenderTemplates),
		).
		MergeSlices(
			Sl(&flags.ExtraValues, nil, rc.ExtraValues),
		)

	// Direct boolean assignments from root config
	if rc.ExternalSecrets {
		flags.ExternalSecrets = true
	}
	if rc.SkipDependencyUpdate {
		flags.SkipDependencyUpdate = true
	}

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
