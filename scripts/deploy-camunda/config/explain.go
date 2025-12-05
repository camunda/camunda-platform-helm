package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"scripts/camunda-core/pkg/logging"

	"github.com/jwalton/gchalk"
)

// ConfigSource represents where a configuration value originated.
type ConfigSource string

const (
	SourceFlag       ConfigSource = "flag"
	SourceEnvVar     ConfigSource = "env"
	SourceDeployment ConfigSource = "deployment"
	SourceRootConfig ConfigSource = "root-config"
	SourceDefault    ConfigSource = "default"
	SourceComputed   ConfigSource = "computed"
)

// TrackedValue holds a configuration value along with its source.
type TrackedValue struct {
	Value  string       `json:"value"`
	Source ConfigSource `json:"source"`
	Detail string       `json:"detail,omitempty"` // e.g., env var name, deployment name
}

// ConfigExplanation holds the traced sources of all configuration values.
type ConfigExplanation struct {
	ConfigFile string                  `json:"configFile,omitempty"`
	Active     string                  `json:"activeDeployment,omitempty"`
	Fields     map[string]TrackedValue `json:"fields"`
}

// NewConfigExplanation creates a new ConfigExplanation.
func NewConfigExplanation() *ConfigExplanation {
	return &ConfigExplanation{
		Fields: make(map[string]TrackedValue),
	}
}

// Track records the source of a configuration field.
func (e *ConfigExplanation) Track(field, value string, source ConfigSource, detail string) {
	e.Fields[field] = TrackedValue{
		Value:  value,
		Source: source,
		Detail: detail,
	}
}

// TrackBool records the source of a boolean configuration field.
func (e *ConfigExplanation) TrackBool(field string, value bool, source ConfigSource, detail string) {
	e.Track(field, fmt.Sprintf("%t", value), source, detail)
}

// TrackSlice records the source of a slice configuration field.
func (e *ConfigExplanation) TrackSlice(field string, values []string, source ConfigSource, detail string) {
	e.Track(field, strings.Join(values, ", "), source, detail)
}

// JSON returns the explanation as JSON.
func (e *ConfigExplanation) JSON() (string, error) {
	data, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// Format returns a human-readable formatted string with colors.
func (e *ConfigExplanation) Format() string {
	styleHead := func(s string) string { return logging.Emphasize(s, gchalk.Bold) }
	styleKey := func(s string) string { return logging.Emphasize(s, gchalk.Cyan) }
	styleVal := func(s string) string { return logging.Emphasize(s, gchalk.Magenta) }
	styleSource := func(s ConfigSource) string {
		switch s {
		case SourceFlag:
			return logging.Emphasize(string(s), gchalk.Green)
		case SourceEnvVar:
			return logging.Emphasize(string(s), gchalk.Yellow)
		case SourceDeployment:
			return logging.Emphasize(string(s), gchalk.Blue)
		case SourceRootConfig:
			return logging.Emphasize(string(s), gchalk.Cyan)
		case SourceDefault:
			return logging.Emphasize(string(s), gchalk.Dim)
		case SourceComputed:
			return logging.Emphasize(string(s), gchalk.Magenta)
		default:
			return string(s)
		}
	}

	var b strings.Builder
	b.WriteString(styleHead("Configuration Resolution"))
	b.WriteString("\n")

	if e.ConfigFile != "" {
		fmt.Fprintf(&b, "  Config file: %s\n", styleVal(e.ConfigFile))
	}
	if e.Active != "" {
		fmt.Fprintf(&b, "  Active deployment: %s\n", styleVal(e.Active))
	}
	b.WriteString("\n")

	// Group fields by category for better readability
	categories := []struct {
		name   string
		fields []string
	}{
		{"Chart", []string{"chartPath", "chart", "chartVersion"}},
		{"Deployment", []string{"namespace", "release", "scenario", "scenarios", "auth"}},
		{"Platform", []string{"platform", "logLevel", "flow", "timeout"}},
		{"Keycloak", []string{"keycloakHost", "keycloakProtocol", "keycloakRealm"}},
		{"Index Prefixes", []string{"optimizeIndexPrefix", "orchestrationIndexPrefix", "tasklistIndexPrefix", "operateIndexPrefix"}},
		{"Secrets", []string{"externalSecrets", "autoGenerateSecrets", "vaultSecretMapping"}},
		{"Docker", []string{"dockerUsername", "ensureDockerRegistry"}},
		{"Output", []string{"renderTemplates", "renderOutputDir", "dryRun"}},
		{"Paths", []string{"repoRoot", "scenarioPath", "valuesPreset", "extraValues", "ingressSubdomain", "ingressHostname"}},
		{"Behavior", []string{"interactive", "skipDependencyUpdate", "deleteNamespaceFirst"}},
	}

	// Find the longest field name for alignment
	maxFieldLen := 0
	for field := range e.Fields {
		if len(field) > maxFieldLen {
			maxFieldLen = len(field)
		}
	}

	printed := make(map[string]bool)

	for _, cat := range categories {
		hasFields := false
		for _, f := range cat.fields {
			if _, ok := e.Fields[f]; ok {
				hasFields = true
				break
			}
		}
		if !hasFields {
			continue
		}

		b.WriteString(styleHead(cat.name))
		b.WriteString("\n")

		for _, field := range cat.fields {
			tv, ok := e.Fields[field]
			if !ok {
				continue
			}
			printed[field] = true

			displayVal := tv.Value
			if displayVal == "" {
				displayVal = "(empty)"
			}
			// Truncate long values
			if len(displayVal) > 50 {
				displayVal = displayVal[:47] + "..."
			}

			sourceStr := styleSource(tv.Source)
			if tv.Detail != "" {
				sourceStr = fmt.Sprintf("%s: %s", sourceStr, tv.Detail)
			}

			fmt.Fprintf(&b, "  %s: %s  (%s)\n",
				styleKey(fmt.Sprintf("%-*s", maxFieldLen, field)),
				styleVal(displayVal),
				sourceStr)
		}
		b.WriteString("\n")
	}

	// Print any remaining fields not in categories
	var remaining []string
	for field := range e.Fields {
		if !printed[field] {
			remaining = append(remaining, field)
		}
	}

	if len(remaining) > 0 {
		b.WriteString(styleHead("Other"))
		b.WriteString("\n")
		for _, field := range remaining {
			tv := e.Fields[field]
			displayVal := tv.Value
			if displayVal == "" {
				displayVal = "(empty)"
			}
			sourceStr := styleSource(tv.Source)
			if tv.Detail != "" {
				sourceStr = fmt.Sprintf("%s: %s", sourceStr, tv.Detail)
			}
			fmt.Fprintf(&b, "  %s: %s  (%s)\n",
				styleKey(fmt.Sprintf("%-*s", maxFieldLen, field)),
				styleVal(displayVal),
				sourceStr)
		}
	}

	return b.String()
}

// ExplainConfig builds a ConfigExplanation by tracing each field's source.
func ExplainConfig(rc *RootConfig, flags *RuntimeFlags, flagsSet map[string]bool) *ConfigExplanation {
	exp := NewConfigExplanation()

	if rc != nil {
		exp.ConfigFile = rc.FilePath
		exp.Active = rc.Current
	}

	// Helper to determine source for a string field
	trackString := func(name, flagVal, envVar, depVal, rootVal, defaultVal string) {
		// Check if flag was explicitly set
		if flagsSet != nil && flagsSet[name] && flagVal != "" {
			exp.Track(name, flagVal, SourceFlag, "--"+name)
			return
		}

		// Check environment variable
		if envVar != "" {
			if v := os.Getenv(envVar); v != "" {
				exp.Track(name, v, SourceEnvVar, envVar)
				return
			}
		}

		// Check deployment config
		if depVal != "" {
			exp.Track(name, depVal, SourceDeployment, rc.Current)
			return
		}

		// Check root config
		if rootVal != "" {
			exp.Track(name, rootVal, SourceRootConfig, "")
			return
		}

		// Default value
		if defaultVal != "" || flags != nil {
			exp.Track(name, defaultVal, SourceDefault, "")
		}
	}

	// Get deployment config if available
	var dep DeploymentConfig
	if rc != nil && rc.Current != "" {
		if d, ok := rc.Deployments[rc.Current]; ok {
			dep = d
		}
	}

	// Track each field - these map to the most commonly used config fields
	trackString("chartPath", flags.ChartPath, "", dep.ChartPath, maybeString(rc, func(r *RootConfig) string { return r.ChartPath }), "")
	trackString("chart", flags.Chart, "", dep.Chart, maybeString(rc, func(r *RootConfig) string { return r.Chart }), "")
	trackString("chartVersion", flags.ChartVersion, "", dep.Version, maybeString(rc, func(r *RootConfig) string { return r.Version }), "")
	trackString("namespace", flags.Namespace, "", dep.Namespace, maybeString(rc, func(r *RootConfig) string { return r.Namespace }), "")
	trackString("release", flags.Release, "", dep.Release, maybeString(rc, func(r *RootConfig) string { return r.Release }), "")
	trackString("scenario", flags.Scenario, "", dep.Scenario, maybeString(rc, func(r *RootConfig) string { return r.Scenario }), "")
	trackString("auth", flags.Auth, "", dep.Auth, maybeString(rc, func(r *RootConfig) string { return r.Auth }), "keycloak")
	trackString("platform", flags.Platform, "CAMUNDA_PLATFORM", dep.Platform, maybeString(rc, func(r *RootConfig) string { return r.Platform }), "gke")
	trackString("logLevel", flags.LogLevel, "CAMUNDA_LOG_LEVEL", dep.LogLevel, maybeString(rc, func(r *RootConfig) string { return r.LogLevel }), "info")
	trackString("flow", flags.Flow, "", dep.Flow, maybeString(rc, func(r *RootConfig) string { return r.Flow }), "install")
	trackString("keycloakHost", flags.KeycloakHost, "CAMUNDA_KEYCLOAK_HOST", "", maybeString(rc, func(r *RootConfig) string { return r.Keycloak.Host }), "")
	trackString("keycloakProtocol", flags.KeycloakProtocol, "CAMUNDA_KEYCLOAK_PROTOCOL", "", maybeString(rc, func(r *RootConfig) string { return r.Keycloak.Protocol }), "https")
	trackString("keycloakRealm", flags.KeycloakRealm, "CAMUNDA_KEYCLOAK_REALM", dep.KeycloakRealm, maybeString(rc, func(r *RootConfig) string { return r.KeycloakRealm }), "")
	trackString("repoRoot", flags.RepoRoot, "CAMUNDA_REPO_ROOT", dep.RepoRoot, maybeString(rc, func(r *RootConfig) string { return r.RepoRoot }), "")
	trackString("valuesPreset", flags.ValuesPreset, "CAMUNDA_VALUES_PRESET", dep.ValuesPreset, maybeString(rc, func(r *RootConfig) string { return r.ValuesPreset }), "")
	trackString("ingressSubdomain", flags.IngressSubdomain, "CAMUNDA_INGRESS_SUBDOMAIN", dep.IngressSubdomain, maybeString(rc, func(r *RootConfig) string { return r.IngressSubdomain }), "")
	trackString("ingressHostname", flags.IngressHostname, "CAMUNDA_INGRESS_HOSTNAME", dep.IngressHostname, maybeString(rc, func(r *RootConfig) string { return r.IngressHostname }), "")

	// Index prefixes
	trackString("optimizeIndexPrefix", flags.OptimizeIndexPrefix, "CAMUNDA_OPTIMIZE_INDEX_PREFIX", dep.OptimizeIndexPrefix, maybeString(rc, func(r *RootConfig) string { return r.OptimizeIndexPrefix }), "")
	trackString("orchestrationIndexPrefix", flags.OrchestrationIndexPrefix, "CAMUNDA_ORCHESTRATION_INDEX_PREFIX", dep.OrchestrationIndexPrefix, maybeString(rc, func(r *RootConfig) string { return r.OrchestrationIndexPrefix }), "")
	trackString("tasklistIndexPrefix", flags.TasklistIndexPrefix, "CAMUNDA_TASKLIST_INDEX_PREFIX", dep.TasklistIndexPrefix, maybeString(rc, func(r *RootConfig) string { return r.TasklistIndexPrefix }), "")
	trackString("operateIndexPrefix", flags.OperateIndexPrefix, "CAMUNDA_OPERATE_INDEX_PREFIX", dep.OperateIndexPrefix, maybeString(rc, func(r *RootConfig) string { return r.OperateIndexPrefix }), "")

	// Boolean fields
	exp.TrackBool("externalSecrets", flags.ExternalSecrets, determineSourceBool(flagsSet, "external-secrets", dep.ExternalSecrets, boolPtrValue(rc, func(r *RootConfig) bool { return r.ExternalSecrets })), "")
	exp.TrackBool("skipDependencyUpdate", flags.SkipDependencyUpdate, determineSourceBool(flagsSet, "skip-dependency-update", dep.SkipDependencyUpdate, boolPtrValue(rc, func(r *RootConfig) bool { return r.SkipDependencyUpdate })), "")
	exp.TrackBool("interactive", flags.Interactive, determineSourceBool(flagsSet, "interactive", dep.Interactive, rc.Interactive), "")
	exp.TrackBool("autoGenerateSecrets", flags.AutoGenerateSecrets, determineSourceBool(flagsSet, "auto-generate-secrets", dep.AutoGenerateSecrets, rc.AutoGenerateSecrets), "")
	exp.TrackBool("deleteNamespaceFirst", flags.DeleteNamespaceFirst, determineSourceBool(flagsSet, "delete-namespace", dep.DeleteNamespace, rc.DeleteNamespaceFirst), "")
	exp.TrackBool("renderTemplates", flags.RenderTemplates, determineSourceBool(flagsSet, "render-templates", dep.RenderTemplates, rc.RenderTemplates), "")
	exp.TrackBool("dryRun", flags.DryRun, SourceFlag, "--dry-run")

	// Timeout
	exp.Track("timeout", fmt.Sprintf("%dm", flags.Timeout), SourceDefault, "")

	// Scenarios (computed from scenario string)
	if len(flags.Scenarios) > 0 {
		exp.TrackSlice("scenarios", flags.Scenarios, SourceComputed, "parsed from scenario")
	}

	return exp
}

// Helper functions

func maybeString(rc *RootConfig, getter func(*RootConfig) string) string {
	if rc == nil {
		return ""
	}
	return getter(rc)
}

func boolPtrValue(rc *RootConfig, getter func(*RootConfig) bool) *bool {
	if rc == nil {
		return nil
	}
	v := getter(rc)
	return &v
}

func determineSourceBool(flagsSet map[string]bool, flagName string, depVal *bool, rootVal *bool) ConfigSource {
	if flagsSet != nil && flagsSet[flagName] {
		return SourceFlag
	}
	if depVal != nil {
		return SourceDeployment
	}
	if rootVal != nil {
		return SourceRootConfig
	}
	return SourceDefault
}

