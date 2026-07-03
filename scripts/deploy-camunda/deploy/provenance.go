package deploy

import (
	"os"
	"sort"
	"strings"

	"scripts/deploy-camunda/config"
	"scripts/prepare-helm-values/pkg/env"
)

// EnvVar is one effective environment variable and the layer it came from.
type EnvVar struct {
	Name   string
	Value  string
	Origin string // "process-env", ".env (<path>)", or "extra-env"
}

// EnvProvenance returns the effective environment a deploy would see, in the
// same layering buildScenarioEnv uses, annotated with the winning source per
// key. Later layers override earlier ones: process env → .env file → per-entry
// ExtraEnv. The result is sorted by name. Values are returned unmasked; callers
// are responsible for masking secrets in human-facing output (see
// values.IsSecretName).
func EnvProvenance(flags *config.RuntimeFlags) []EnvVar {
	value := map[string]string{}
	origin := map[string]string{}

	for _, e := range os.Environ() {
		if k, v, ok := strings.Cut(e, "="); ok {
			value[k] = v
			origin[k] = "process-env"
		}
	}

	envFile := flags.EnvFile
	if envFile == "" {
		envFile = ".env"
	}
	if dotenv, err := env.ReadFile(envFile); err == nil {
		src := ".env (" + envFile + ")"
		for k, v := range dotenv {
			value[k] = v
			origin[k] = src
		}
	}

	for k, v := range flags.ExtraEnv {
		value[k] = v
		origin[k] = "extra-env"
	}

	names := make([]string, 0, len(value))
	for k := range value {
		names = append(names, k)
	}
	sort.Strings(names)

	out := make([]EnvVar, 0, len(names))
	for _, n := range names {
		out = append(out, EnvVar{Name: n, Value: value[n], Origin: origin[n]})
	}
	return out
}
