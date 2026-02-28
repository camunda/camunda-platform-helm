package format

import (
	"fmt"
	"os"
	"scripts/camunda-core/pkg/logging"
	"scripts/deploy-camunda/config"
	"strings"

	"github.com/jwalton/gchalk"
	"github.com/spf13/pflag"
	"gopkg.in/yaml.v3"
)

// Style helpers for terminal output.
var (
	styleKey  = func(s string) string { return logging.Emphasize(s, gchalk.Cyan) }
	styleVal  = func(s string) string { return logging.Emphasize(s, gchalk.Magenta) }
	styleHead = func(s string) string { return logging.Emphasize(s, gchalk.Bold) }
)

// PrintFlags logs all CLI flags in a formatted, colored style.
func PrintFlags(flags *pflag.FlagSet) {
	stylePwd := func(s string) string { return logging.Emphasize(s, gchalk.Yellow) }
	styleBool := func(s string) string {
		if strings.EqualFold(s, "true") || s == "1" {
			return logging.Emphasize("true", gchalk.Green)
		}
		return logging.Emphasize("false", gchalk.Red)
	}
	var b strings.Builder
	b.WriteString(styleHead("Starting deployment with flags:"))
	b.WriteString("\n")

	printFlag := func(f *pflag.Flag) {
		name := f.Name
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

	if flags != nil {
		flags.VisitAll(printFlag)
	}
	logging.Logger.Info().Msg(b.String())
}

// maskIfSet returns a masked placeholder when a sensitive value is set.
func maskIfSet(val string) string {
	if val == "" {
		return ""
	}
	return "***"
}

// PrintDeploymentConfig displays a deployment configuration.
func PrintDeploymentConfig(name string, dep config.DeploymentConfig, root config.RootConfig) error {
	chartStr := config.FirstNonEmpty(dep.Chart, "")
	versionStr := config.FirstNonEmpty(dep.Version, "")
	scenarioStr := config.FirstNonEmpty(dep.Scenario, "")
	repoRootStr := config.FirstNonEmpty(dep.RepoRoot, root.RepoRoot)
	scenarioRootStr := config.FirstNonEmpty(dep.ScenarioRoot, root.ScenarioRoot)
	valuesPresetStr := config.FirstNonEmpty(dep.ValuesPreset, root.ValuesPreset)
	platformStr := root.Platform
	logLevelStr := root.LogLevel

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
}
