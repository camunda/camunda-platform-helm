// Package deploy preflight.go implements a self-diagnosing check of the
// secrets/environment setup a deployment needs, mirroring tools like
// `flyctl doctor` and `gh auth status`. The same checks back both the
// standalone `deploy-camunda doctor` command and the fail-fast guard that runs
// before any cluster mutation, so a missing credential surfaces up front with a
// remediation hint instead of mid-deploy as an ImagePullBackOff or a
// `kubectl apply` parse error.
package deploy

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"scripts/camunda-core/pkg/scenarios"
	"scripts/camunda-core/pkg/utils"
	"scripts/deploy-camunda/config"
	"scripts/prepare-helm-values/pkg/env"
	"scripts/prepare-helm-values/pkg/placeholders"
	"scripts/vault-secret-mapper/pkg/mapper"
)

// CheckStatus is the outcome of a single preflight check.
type CheckStatus string

const (
	StatusOK   CheckStatus = "ok"   // requirement satisfied
	StatusWarn CheckStatus = "warn" // not satisfied, but not strictly required for this run
	StatusFail CheckStatus = "fail" // required and missing — a deploy would fail
)

// Check is the result of one preflight probe.
type Check struct {
	Name        string
	Status      CheckStatus
	Detail      string   // human-readable summary of what was found
	Remediation string   // how to fix it, shown when Status != StatusOK
	Missing     []string // env var names this check found unset (for aggregation)
}

// Report is the collected result of all preflight checks.
type Report struct {
	Checks []Check
}

// OK reports whether no check failed (warnings are tolerated).
func (r *Report) OK() bool {
	for _, c := range r.Checks {
		if c.Status == StatusFail {
			return false
		}
	}
	return true
}

// MissingEnv returns the sorted union of env var names reported missing by any
// check, so callers can prompt for or report them all at once.
func (r *Report) MissingEnv() []string {
	seen := map[string]struct{}{}
	for _, c := range r.Checks {
		for _, m := range c.Missing {
			seen[m] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for m := range seen {
		out = append(out, m)
	}
	sort.Strings(out)
	return out
}

// PreflightOptions tunes the preflight run.
type PreflightOptions struct {
	// ConfigPath is the config file path resolved by the caller (config.ResolvePath).
	ConfigPath string
	// ConfigFound reports whether ConfigPath exists on disk.
	ConfigFound bool
	// SkipKubeReachability skips the network round-trip that pings the cluster
	// (used in tests and when only static checks are wanted).
	SkipKubeReachability bool
}

// Preflight runs all secrets/env checks against the merged flags and returns a
// Report. It has no side effects: it never writes files, mutates the process
// environment, or contacts the cluster beyond an optional read-only readiness
// ping. Sources are resolved exactly as a deploy would see them — process
// environment overlaid with the configured .env file.
func Preflight(ctx context.Context, flags *config.RuntimeFlags, opts PreflightOptions) *Report {
	r := &Report{}
	// baseEnv is process env + .env + ExtraEnv. deployEnv additionally includes the
	// variables the deploy machinery computes per scenario (CAMUNDA_HOSTNAME,
	// KEYCLOAK_REALM, the *_INDEX_PREFIX vars, FLOW, KEYCLOAK_EXT_*), so scenario
	// and companion placeholder checks don't false-positive on values the deploy
	// supplies itself. Vault-mapping and docker checks use baseEnv — those vars are
	// never deploy-computed.
	baseEnv := effectiveEnv(flags)
	deployEnv := scenarioDeployEnv(flags, baseEnv)

	r.Checks = append(r.Checks, checkConfigFile(opts))
	r.Checks = append(r.Checks, checkKubeContext(ctx, flags, opts))
	r.Checks = append(r.Checks, checkDockerCredentials(flags, baseEnv)...)
	r.Checks = append(r.Checks, checkVaultMapping(flags, baseEnv))
	r.Checks = append(r.Checks, checkScenarioEnv(flags, deployEnv))
	r.Checks = append(r.Checks, checkCompanionEnv(flags, deployEnv))

	return r
}

// effectiveEnv reproduces the env layering buildScenarioEnv uses for presence
// checks, sharing one source of truth with EnvProvenance.
func effectiveEnv(flags *config.RuntimeFlags) map[string]string {
	m := map[string]string{}
	for _, e := range EnvProvenance(flags) {
		m[e.Name] = e.Value
	}
	return m
}

// scenarioDeployEnv returns the environment a deploy would actually resolve for
// the first configured scenario — baseEnv plus the variables buildScenarioEnv
// computes (ingress hostname, Keycloak realm, index prefixes, flow). It reuses
// the same pure functions the deploy uses (generateScenarioContext +
// buildScenarioEnv), so the preflight cannot disagree with reality about which
// variables the user must supply. Falls back to baseEnv when no scenario/chart
// is configured or the computation fails. Computed variable names are identical
// across scenarios, so one representative scenario suffices.
func scenarioDeployEnv(flags *config.RuntimeFlags, baseEnv map[string]string) map[string]string {
	scenario := firstScenario(flags)
	if scenario == "" || flags.Chart.ChartPath == "" {
		return baseEnv
	}
	scenarioCtx, err := generateScenarioContext(scenario, flags)
	if err != nil || scenarioCtx == nil {
		return baseEnv
	}
	env, err := buildScenarioEnv(scenarioCtx, flags)
	if err != nil || env == nil {
		return baseEnv
	}
	return env
}

// firstScenario returns the first scenario name from flags, honoring both the
// parsed Scenarios slice and a comma-separated Scenario string.
func firstScenario(flags *config.RuntimeFlags) string {
	if len(flags.Deployment.Scenarios) > 0 {
		return flags.Deployment.Scenarios[0]
	}
	for _, s := range strings.Split(flags.Deployment.Scenario, ",") {
		if t := strings.TrimSpace(s); t != "" {
			return t
		}
	}
	return ""
}

// presence partitions names into those set to a non-empty value and those unset.
func presence(envMap map[string]string, names []string) (present, missing []string) {
	for _, n := range names {
		if v, ok := envMap[n]; ok && v != "" {
			present = append(present, n)
		} else {
			missing = append(missing, n)
		}
	}
	return present, missing
}

// firstNonEmptyEnv returns the explicit flag value if set, else the first
// non-empty value among the named env vars (resolved through envMap so .env
// entries count).
func firstNonEmptyEnv(envMap map[string]string, flagVal string, names ...string) string {
	if flagVal != "" {
		return flagVal
	}
	vals := make([]string, 0, len(names))
	for _, n := range names {
		vals = append(vals, envMap[n])
	}
	return utils.FirstNonEmpty(vals...)
}

func checkConfigFile(opts PreflightOptions) Check {
	if opts.ConfigFound {
		return Check{Name: "config file", Status: StatusOK, Detail: opts.ConfigPath}
	}
	detail := "no config file found"
	if opts.ConfigPath != "" {
		detail = fmt.Sprintf("not found at %s", opts.ConfigPath)
	}
	return Check{
		Name:        "config file",
		Status:      StatusWarn,
		Detail:      detail + " (relying on flags/env)",
		Remediation: "run `deploy-camunda config init` to scaffold one",
	}
}

func checkKubeContext(ctx context.Context, flags *config.RuntimeFlags, opts PreflightOptions) Check {
	kubeCtx := flags.Test.KubeContext
	if kubeCtx == "" {
		if out, err := exec.CommandContext(ctx, "kubectl", "config", "current-context").Output(); err == nil {
			kubeCtx = strings.TrimSpace(string(out))
		}
	}
	if kubeCtx == "" {
		return Check{
			Name:        "kube context",
			Status:      StatusFail,
			Detail:      "no context set and no current-context configured",
			Remediation: "pass --kube-context or run `kubectl config use-context <ctx>`",
		}
	}
	if opts.SkipKubeReachability {
		return Check{Name: "kube context", Status: StatusOK, Detail: kubeCtx + " (reachability not checked)"}
	}
	// Read-only readiness ping; short timeout so an unreachable cluster doesn't hang.
	probe := exec.CommandContext(ctx, "kubectl", "--context", kubeCtx, "get", "--raw=/readyz", "--request-timeout=5s")
	if err := probe.Run(); err != nil {
		return Check{
			Name:        "kube context",
			Status:      StatusWarn,
			Detail:      fmt.Sprintf("%s set but not reachable", kubeCtx),
			Remediation: "check VPN/Teleport login and that the context points at a live cluster",
		}
	}
	return Check{Name: "kube context", Status: StatusOK, Detail: kubeCtx + " (reachable)"}
}

// checkDockerCredentials validates Harbor (and optionally Docker Hub) creds
// using the same flag/env fallback chain as camunda-core/pkg/docker. The checks
// only fail when the corresponding --ensure-docker-* flag makes the pull secret
// mandatory; otherwise an absence is a warning.
func checkDockerCredentials(flags *config.RuntimeFlags, envMap map[string]string) []Check {
	var checks []Check

	harborUser := firstNonEmptyEnv(envMap, flags.Docker.DockerUsername, "HARBOR_USERNAME", "TEST_DOCKER_USERNAME_CAMUNDA_CLOUD", "NEXUS_USERNAME")
	harborPass := firstNonEmptyEnv(envMap, flags.Docker.DockerPassword, "HARBOR_PASSWORD", "TEST_DOCKER_PASSWORD_CAMUNDA_CLOUD", "NEXUS_PASSWORD")
	checks = append(checks, dockerCredCheck(
		"docker creds (Harbor)", harborUser, harborPass, flags.Docker.EnsureDockerRegistry,
		"set --docker-username/--docker-password or HARBOR_USERNAME/HARBOR_PASSWORD (or TEST_DOCKER_*_CAMUNDA_CLOUD)"))

	// Only probe Docker Hub when its pull secret is requested.
	if flags.Docker.EnsureDockerHub {
		hubUser := firstNonEmptyEnv(envMap, flags.Docker.DockerHubUsername, "DOCKERHUB_USERNAME", "TEST_DOCKER_USERNAME")
		hubPass := firstNonEmptyEnv(envMap, flags.Docker.DockerHubPassword, "DOCKERHUB_PASSWORD", "TEST_DOCKER_PASSWORD")
		checks = append(checks, dockerCredCheck(
			"docker creds (Docker Hub)", hubUser, hubPass, true,
			"set --dockerhub-username/--dockerhub-password or DOCKERHUB_USERNAME/DOCKERHUB_PASSWORD"))
	}

	return checks
}

func dockerCredCheck(name, user, pass string, required bool, remediation string) Check {
	if user != "" && pass != "" {
		return Check{Name: name, Status: StatusOK, Detail: "present"}
	}
	status := StatusWarn
	detail := "not set (pull secret not requested)"
	if required {
		status = StatusFail
		detail = "missing — required by --ensure-docker-* (pods would ImagePullBackOff)"
	}
	return Check{Name: name, Status: status, Detail: detail, Remediation: remediation}
}

func checkVaultMapping(flags *config.RuntimeFlags, envMap map[string]string) Check {
	mapping := flags.Secrets.VaultSecretMapping
	if strings.TrimSpace(mapping) == "" {
		return Check{Name: "vault secret mapping", Status: StatusOK, Detail: "no mapping configured"}
	}
	required := mapper.RequiredEnvVars(mapping)
	present, missing := presence(envMap, required)
	if len(missing) == 0 {
		return Check{Name: "vault secret mapping", Status: StatusOK, Detail: fmt.Sprintf("all %d mapped vars set", len(present))}
	}
	return Check{
		Name:        "vault secret mapping",
		Status:      StatusFail,
		Detail:      fmt.Sprintf("%d/%d vars unset: %s", len(missing), len(required), strings.Join(missing, ", ")),
		Remediation: "set the listed vars in your .env or environment (the generated Secret silently drops unset keys)",
		Missing:     missing,
	}
}

// checkScenarioEnv scans the values file(s) for the selected scenario(s) and
// reports any $PLACEHOLDER env vars that are unset. Resolution failures are a
// soft warning rather than a hard fail so `doctor` is still useful when no
// scenario/chart is configured yet.
func checkScenarioEnv(flags *config.RuntimeFlags, envMap map[string]string) Check {
	scenarioList := flags.Deployment.Scenarios
	if len(scenarioList) == 0 && flags.Deployment.Scenario != "" {
		for _, s := range strings.Split(flags.Deployment.Scenario, ",") {
			if t := strings.TrimSpace(s); t != "" {
				scenarioList = append(scenarioList, t)
			}
		}
	}
	if len(scenarioList) == 0 || flags.Chart.ChartPath == "" {
		return Check{Name: "scenario env vars", Status: StatusOK, Detail: "no scenario/chart configured to scan"}
	}

	scenarioDir := flags.Deployment.ScenarioPath
	if scenarioDir == "" {
		scenarioDir = filepath.Join(flags.Chart.ChartPath, "test/integration/scenarios/chart-full-setup")
	}

	required := map[string]struct{}{}
	var unresolved []string
	for _, scenario := range scenarioList {
		files, err := scenarioLayerFiles(flags, scenarioDir, scenario)
		if err != nil || len(files) == 0 {
			unresolved = append(unresolved, scenario)
			continue
		}
		for _, valuesFile := range files {
			content, err := os.ReadFile(valuesFile)
			if err != nil {
				continue
			}
			for _, p := range placeholders.Find(string(content)) {
				required[p] = struct{}{}
			}
		}
	}

	if len(required) == 0 {
		if len(unresolved) > 0 {
			return Check{
				Name:        "scenario env vars",
				Status:      StatusWarn,
				Detail:      "could not resolve values for: " + strings.Join(unresolved, ", "),
				Remediation: "check --scenario/--scenario-path and --chart-path",
			}
		}
		return Check{Name: "scenario env vars", Status: StatusOK, Detail: "no placeholders in scenario values"}
	}

	names := make([]string, 0, len(required))
	for n := range required {
		names = append(names, n)
	}
	sort.Strings(names)
	_, missing := presence(envMap, names)
	if len(missing) == 0 {
		return Check{Name: "scenario env vars", Status: StatusOK, Detail: fmt.Sprintf("all %d scenario vars set", len(names))}
	}
	return Check{
		Name:        "scenario env vars",
		Status:      StatusFail,
		Detail:      fmt.Sprintf("%d unset: %s", len(missing), strings.Join(missing, ", ")),
		Remediation: "set the listed vars (run with --interactive to be prompted, or `deploy-camunda config init`)",
		Missing:     missing,
	}
}

// scenarioLayerFiles returns the full set of layered values files a deploy would
// compose for a scenario — base + identity + persistence + platform + infra +
// features + … — mirroring the resolution in prepareScenarioValues
// (values.go ≈ L708). Placeholders ($VAR) live across these layers, not only in
// the top-level scenario file: e.g. $VENOM_CLIENT_ID / $CONNECTORS_CLIENT_ID live
// in values/persistence/elasticsearch.yaml. Scanning only the single resolved
// file made the preflight false-negative on those, letting a deploy pass
// preflight and then fail in prepareScenarioValues. Selection overrides come from
// flags exactly as the deploy applies them; with none set, BuildDeploymentConfig
// derives identity/persistence from the scenario name. Falls back to the single
// resolved scenario file when the directory is not layered.
func scenarioLayerFiles(flags *config.RuntimeFlags, scenarioDir, scenario string) ([]string, error) {
	if !scenarios.HasLayeredValues(scenarioDir) {
		f, err := scenarios.ResolvePath(scenarioDir, scenario)
		if err != nil {
			return nil, err
		}
		return []string{f}, nil
	}
	platform := flags.Selection.TestPlatform
	if platform == "" {
		platform = flags.Deployment.Platform
	}
	deployConfig, err := scenarios.BuildDeploymentConfig(scenario, scenarios.BuilderOverrides{
		Identity:     flags.Selection.Identity,
		Persistence:  flags.Selection.Persistence,
		Platform:     platform,
		Features:     flags.Selection.Features,
		InfraType:    flags.Selection.InfraType,
		Flow:         flags.Deployment.Flow,
		QA:           flags.Selection.QA,
		ImageTags:    flags.Selection.ImageTags,
		Upgrade:      flags.Selection.UpgradeFlow,
		ChartVersion: flags.Chart.ChartVersion,
	})
	if err != nil {
		return nil, err
	}
	return deployConfig.ResolvePaths(scenarioDir)
}

// checkCompanionEnv validates the env vars that companion chart values files
// need for substitution. It mirrors substituteCompanionEnvVars exactly: a chart
// contributes requirements only when it has both a values file and a non-empty
// EnvVars allowlist, and a variable counts as present when its key exists
// (empty is allowed). The matrix runner populates flags.CompanionCharts from a
// scenario's dependencies; without this check the substitution would otherwise
// fail mid-prepare with the same missing-var error.
func checkCompanionEnv(flags *config.RuntimeFlags, envMap map[string]string) Check {
	required := map[string]struct{}{}
	for _, c := range flags.CompanionCharts {
		if c.ValuesFile == "" {
			continue
		}
		for _, n := range c.EnvVars {
			if n != "" {
				required[n] = struct{}{}
			}
		}
	}
	if len(required) == 0 {
		return Check{Name: "companion env vars", Status: StatusOK, Detail: "no companion charts requiring env vars"}
	}

	names := make([]string, 0, len(required))
	for n := range required {
		names = append(names, n)
	}
	sort.Strings(names)

	var missing []string
	for _, n := range names {
		if _, ok := envMap[n]; !ok {
			missing = append(missing, n)
		}
	}
	if len(missing) == 0 {
		return Check{Name: "companion env vars", Status: StatusOK, Detail: fmt.Sprintf("all %d companion vars set", len(names))}
	}
	return Check{
		Name:        "companion env vars",
		Status:      StatusFail,
		Detail:      fmt.Sprintf("%d unset: %s", len(missing), strings.Join(missing, ", ")),
		Remediation: "set the listed vars in your .env (RDBMS_POSTGRESQL_* are scaffolded by `deploy-camunda config init`, or use `doctor --fix`)",
		Missing:     missing,
	}
}

// ResolveMissingInteractively prompts for each variable the report flagged as
// missing, persisting answers to the configured .env file (default ".env") and
// into the process environment so an in-process deploy picks them up
// immediately. Returns the number of variables resolved. Intended for
// human-driven runs (interactive deploys, `doctor --fix`); the matrix runs
// non-interactively and never calls this. Reuses env.Prompt (context-aware) and
// env.AppendMultiple (format-preserving, concurrency-safe).
func ResolveMissingInteractively(ctx context.Context, report *Report, flags *config.RuntimeFlags) (int, error) {
	missing := report.MissingEnv()
	if len(missing) == 0 {
		return 0, nil
	}
	envFile := flags.EnvFile
	if envFile == "" {
		envFile = ".env"
	}
	updates := map[string]string{}
	for _, name := range missing {
		val, err := env.Prompt(ctx, name, "")
		if err != nil {
			// No more input available (EOF, e.g. non-interactive stdin): stop
			// prompting and persist whatever was entered so far. Context
			// cancellation (Ctrl+C) and other errors propagate.
			if errors.Is(err, io.EOF) {
				break
			}
			return len(updates), err
		}
		if strings.TrimSpace(val) == "" {
			continue
		}
		os.Setenv(name, val)
		updates[name] = val
	}
	if len(updates) > 0 {
		if err := env.AppendMultiple(envFile, updates); err != nil {
			return len(updates), fmt.Errorf("failed to persist resolved vars to %s: %w", envFile, err)
		}
	}
	return len(updates), nil
}

// Render writes the report as an aligned ✓/✗ checklist to w.
func (r *Report) Render(w *bytes.Buffer) {
	symbol := map[CheckStatus]string{StatusOK: "✓", StatusWarn: "!", StatusFail: "✗"}
	width := 0
	for _, c := range r.Checks {
		if len(c.Name) > width {
			width = len(c.Name)
		}
	}
	for _, c := range r.Checks {
		fmt.Fprintf(w, "%s %-*s  %s\n", symbol[c.Status], width, c.Name, c.Detail)
		if c.Status != StatusOK && c.Remediation != "" {
			fmt.Fprintf(w, "  %-*s  → %s\n", width, "", c.Remediation)
		}
	}
}
