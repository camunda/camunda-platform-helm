package matrix

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// ChartVersions holds the parsed content of charts/chart-versions.yaml.
type ChartVersions struct {
	CamundaVersions struct {
		Alpha           []string `yaml:"alpha"`
		SupportStandard []string `yaml:"supportStandard"`
		SupportExtended []string `yaml:"supportExtended"`
		EndOfLife       []string `yaml:"endOfLife"`
	} `yaml:"camundaVersions"`
}

// ActiveVersions returns the list of active versions (alpha + supportStandard).
func (cv *ChartVersions) ActiveVersions() []string {
	var versions []string
	versions = append(versions, cv.CamundaVersions.Alpha...)
	versions = append(versions, cv.CamundaVersions.SupportStandard...)
	return versions
}

// LoadChartVersions reads and parses charts/chart-versions.yaml from the repo root.
func LoadChartVersions(repoRoot string) (*ChartVersions, error) {
	path := filepath.Join(repoRoot, "charts", "chart-versions.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read chart-versions.yaml: %w", err)
	}
	var cv ChartVersions
	if err := yaml.Unmarshal(data, &cv); err != nil {
		return nil, fmt.Errorf("failed to parse chart-versions.yaml: %w", err)
	}
	return &cv, nil
}

// CITestConfig holds the parsed content of a ci-test-config.yaml file.
type CITestConfig struct {
	Integration struct {
		Vars struct {
			TasksBaseDir  string `yaml:"tasksBaseDir"`
			ValuesBaseDir string `yaml:"valuesBaseDir"`
			ChartsBaseDir string `yaml:"chartsBaseDir"`
		} `yaml:"vars"`
		Case struct {
			PR struct {
				Scenarios []CIScenario `yaml:"scenario"`
			} `yaml:"pr"`
			Nightly struct {
				Scenarios []CIScenario `yaml:"scenario"`
			} `yaml:"nightly"`
		} `yaml:"case"`
		// Flows declares lifecycle hooks scoped to a flow rather than a scenario,
		// e.g. pre-upgrade scripts shared by all scenarios using a given flow.
		// Keys are flow strings such as "upgrade-patch", "upgrade-minor".
		Flows map[string]*FlowHooks `yaml:"flows,omitempty"`
		// DependencyProfiles are reusable companion-chart bundles keyed by name.
		// Scenarios reference them via Profiles; ResolveProfiles expands the
		// references into each scenario's Dependencies and PreInstall at load
		// time, so common companion setup is declared once instead of repeated
		// in every scenario.
		DependencyProfiles map[string]DependencyProfile `yaml:"dependency-profiles,omitempty"`
	} `yaml:"integration"`
}

// DependencyProfile is a reusable bundle of companion-chart dependencies and an
// optional pre-install hook, referenced by name from a scenario's Profiles list.
// PreInstall, when set, must use fixtures (not a script) so profiles compose
// cleanly across scenarios.
type DependencyProfile struct {
	Dependencies []ChartDependency `yaml:"dependencies,omitempty"`
	PreInstall   *LifecycleHook    `yaml:"pre-install,omitempty"`
}

// LifecycleHook declares a fixture or shell script that runs at a defined
// point in a scenario or flow lifecycle. Exactly one of Fixtures or Script
// must be set; Description is required so reviewers can understand the
// effect from a ci-test-config.yaml diff alone.
type LifecycleHook struct {
	// Fixtures lists manifest filenames under
	// charts/<version>/test/integration/scenarios/common/resources/ that are
	// applied via Go server-side apply. Use for trivial kubectl-apply cases.
	Fixtures []string `yaml:"fixtures,omitempty"`

	// Script names a shell script under
	// charts/<version>/test/integration/scenarios/pre-setup-scripts/ executed
	// via bash. Use only when fixtures cannot express the logic
	// (cert generation, JKS keystores, conditional kubectl ops).
	Script string `yaml:"script,omitempty"`

	// Description is human-readable and required.
	Description string `yaml:"description"`
}

// Validate enforces the cross-field invariants documented on LifecycleHook:
// non-empty description, exactly one of fixtures or script, and each
// referenced filename is plain (no path separators or "..") so filepath.Join
// downstream cannot escape pre-setup-scripts/ or common/resources/.
// ctx is prepended to error messages so callers see e.g.
// `scenario "rdbms": pre-install: ...`. A nil receiver is a no-op so callers
// can pass optional fields directly.
func (h *LifecycleHook) Validate(ctx string) error {
	if h == nil {
		return nil
	}
	if strings.TrimSpace(h.Description) == "" {
		return fmt.Errorf("%s: description: empty or whitespace-only (required)", ctx)
	}
	hasFixtures := len(h.Fixtures) > 0
	hasScript := h.Script != ""
	if hasFixtures == hasScript {
		return fmt.Errorf("%s: must specify exactly one of fixtures or script (fixtures=%v script=%q)",
			ctx, h.Fixtures, h.Script)
	}
	if hasScript && !isPlainFilename(h.Script) {
		return fmt.Errorf("%s: script %q must be a plain filename (no path separators or \"..\")", ctx, h.Script)
	}
	for _, f := range h.Fixtures {
		if !isPlainFilename(f) {
			return fmt.Errorf("%s: fixture %q must be a plain filename (no path separators or \"..\")", ctx, f)
		}
	}
	return nil
}

// isPlainFilename returns true if name has no path separators and is not "."
// or "..". Used by LifecycleHook.Validate to reject inputs that would let
// filepath.Join downstream escape the configured directory.
func isPlainFilename(name string) bool {
	if name == "" || name == "." || name == ".." {
		return false
	}
	if strings.ContainsAny(name, `/\`) {
		return false
	}
	return filepath.Base(name) == name
}

// FlowHooks groups lifecycle hooks attached to a flow rather than a scenario.
type FlowHooks struct {
	// PreUpgrade runs between Step 1 and Step 2 of a two-step upgrade flow.
	PreUpgrade *LifecycleHook `yaml:"pre-upgrade,omitempty"`
}

// CIScenario represents a single scenario entry in ci-test-config.yaml.
type CIScenario struct {
	Name      string   `yaml:"name"`
	Enabled   bool     `yaml:"enabled"`
	Shortname string   `yaml:"shortname"`
	Auth      string   `yaml:"auth"`
	Flow      string   `yaml:"flow"`
	Platforms []string `yaml:"platforms"`
	Exclude   []string `yaml:"exclude"`
	Tier      int      `yaml:"tier,omitempty"`

	// InfraType maps platform names to infrastructure pool types, e.g.,
	// {"gke": "distroci", "eks": "preemptible"}.
	// The resolved value selects the values-infra-<suffix>.yaml file at deployment time.
	InfraType map[string]string `yaml:"infra-type,omitempty"`

	// Selection + Composition fields (explicit layer overrides).
	// When set, these take precedence over name-based derivation in MapScenarioToConfig.
	Identity    string   `yaml:"identity,omitempty"`
	Persistence string   `yaml:"persistence,omitempty"`
	Features    []string `yaml:"features,omitempty"`

	// ExtraValues lists scenario-specific values files (paths relative to the
	// scenario's chart-full-setup dir) appended to the helm values chain after
	// any global --extra-values. Lets a scenario specialize without losing the
	// global override.
	ExtraValues []string `yaml:"extra-values,omitempty"`

	// Base modifier flags.
	QA         bool `yaml:"qa,omitempty"`
	ImageTags  bool `yaml:"image-tags,omitempty"`
	Upgrade    bool `yaml:"upgrade,omitempty"`
	Enterprise bool `yaml:"enterprise,omitempty"`

	// HelmVersion, when set, overrides the pre-baked Helm binary in CI with the
	// given version via azure/setup-helm. Free-form version string (e.g. "3.20.2",
	// "v4.0.0"). Empty means use whatever Helm ships in the CI runner image.
	HelmVersion string `yaml:"helmVersion,omitempty"`

	// Test skip flags — declarative controls read from ci-test-config.yaml.
	// When set, these prevent the corresponding test types from running for this scenario,
	// replacing hardcoded shortname-based skip logic in both the Go CLI and GHA workflows.
	SkipE2E bool `yaml:"skip-e2e,omitempty"`

	// Profiles names reusable dependency profiles (see
	// integration.dependency-profiles) to expand into this scenario's
	// Dependencies and PreInstall. Profiles are applied in list order, before
	// any scenario-inline Dependencies. Special scenarios may omit Profiles and
	// declare Dependencies/PreInstall directly.
	Profiles []string `yaml:"profiles,omitempty"`

	// Dependencies specifies companion charts to deploy before the main Camunda chart.
	// Each dependency is deployed as a separate Helm release in the same namespace.
	// After ResolveProfiles, this holds the fully-expanded list (profile
	// dependencies first, then any inline entries).
	Dependencies []ChartDependency `yaml:"dependencies,omitempty"`

	// PrefixKey, when set, overrides the scenario name for index prefix
	// derivation. This ensures that two scenarios with different names but
	// representing the same logical deployment (e.g., across chart versions)
	// produce identical index prefixes. Without this, an install on version A
	// (scenario name X) and an upgrade on version B (scenario name Y) would
	// generate different prefixes, breaking the upgrade.
	PrefixKey string `yaml:"prefix-key,omitempty"`

	// PreInstall declares a fixture or script to run before helm install for
	// this scenario. Replaces the legacy filename-derived discovery
	// (pre-install-<scenario>.sh) with an explicit, reviewable reference.
	PreInstall *LifecycleHook `yaml:"pre-install,omitempty"`

	// PostInfra declares a fixture or script to run after the scenario's
	// companion charts (external infrastructure) are deployed and ready, but
	// before the main Camunda chart is installed/upgraded. Used to act on
	// freshly-provisioned infrastructure — e.g. migrating data from a prior
	// release's bundled backends onto the companion services.
	PostInfra *LifecycleHook `yaml:"post-infra,omitempty"`

	// PostDeploy declares a fixture or script to run after helm install
	// completes successfully. Used for resources whose CRDs are only
	// installed by the chart itself (e.g., the Gateway API
	// ProxySettingsPolicy applied for gateway-keycloak).
	PostDeploy *LifecycleHook `yaml:"post-deploy,omitempty"`

	// Topology, when set, describes a multi-namespace deployment shape for
	// this scenario (management + orchestration releases sharing a central
	// Identity and secondary storage). Scenarios without it deploy as a
	// single namespace/release exactly as before.
	Topology *Topology `yaml:"topology,omitempty"`
}

// ChartDependency represents a companion chart that must be deployed
// before the main Camunda chart. The chart is deployed as a separate
// Helm release in the same namespace.
type ChartDependency struct {
	// Chart is the Helm chart reference. This can be a repo/chart name
	// (e.g., "opensearch/opensearch") or a local path relative to the repo root.
	Chart string `yaml:"chart" json:"chart"`
	// Version is the chart version to install (e.g., "3.6.0").
	// Required for remote charts; ignored for local paths.
	Version string `yaml:"version,omitempty" json:"version,omitempty"`
	// ReleaseName is the Helm release name for the companion chart.
	// Example: "opensearch"
	ReleaseName string `yaml:"release-name" json:"release-name"`
	// ValuesFile is the path to a values file for the companion chart,
	// relative to the repo root. Optional — omit to use chart defaults.
	ValuesFile string `yaml:"values-file,omitempty" json:"values-file,omitempty"`
	// EnvVars is the explicit allowlist of environment variable names to
	// substitute in ValuesFile ($VAR / ${VAR}). Only these names are expanded;
	// all other $-tokens (e.g. shell vars $n, $max in init scripts) are left
	// intact. Empty means the values file is used verbatim.
	EnvVars []string `yaml:"env-vars,omitempty" json:"env-vars,omitempty"`
	// RepoName is the Helm repository name to register before installing
	// the chart (e.g., "opensearch"). Required for repo-style chart refs;
	// not needed for OCI or local paths.
	RepoName string `yaml:"repo-name,omitempty" json:"repo-name,omitempty"`
	// RepoURL is the Helm repository URL (e.g.,
	// "https://opensearch-project.github.io/helm-charts/").
	// Required when RepoName is set.
	RepoURL string `yaml:"repo-url,omitempty" json:"repo-url,omitempty"`
}

// ResolveProfiles expands every scenario's Profiles into its Dependencies and
// PreInstall, in place. Profile dependencies are placed first, in profile-list
// order, followed by any scenario-inline Dependencies. A profile pre-install
// fixture is merged into the scenario PreInstall (fixtures are unioned). The
// resolution is deterministic so that matrix list/run output is unchanged from
// the equivalent fully-inlined config. Returns an error for an unknown profile
// reference or a pre-install mode conflict.
func ResolveProfiles(cfg *CITestConfig) error {
	if cfg == nil {
		return nil
	}
	profiles := cfg.Integration.DependencyProfiles
	// Iterate by index over each backing slice directly so the in-place writes
	// in resolveScenarioProfiles are obviously reaching the real structs.
	for i := range cfg.Integration.Case.PR.Scenarios {
		if err := resolveScenarioProfiles(&cfg.Integration.Case.PR.Scenarios[i], profiles); err != nil {
			return err
		}
	}
	for i := range cfg.Integration.Case.Nightly.Scenarios {
		if err := resolveScenarioProfiles(&cfg.Integration.Case.Nightly.Scenarios[i], profiles); err != nil {
			return err
		}
	}
	return nil
}

// resolveScenarioProfiles expands a single scenario's Profiles in place.
func resolveScenarioProfiles(s *CIScenario, profiles map[string]DependencyProfile) error {
	if len(s.Profiles) == 0 {
		return nil
	}
	var deps []ChartDependency
	for _, name := range s.Profiles {
		p, ok := profiles[name]
		if !ok {
			return fmt.Errorf("scenario %q references unknown dependency profile %q", s.Name, name)
		}
		for _, d := range p.Dependencies {
			// Deep-copy EnvVars so two scenarios expanding the same profile do
			// not alias one slice (mirrors the fixtures copy in
			// mergeProfilePreInstall).
			if len(d.EnvVars) > 0 {
				d.EnvVars = append([]string(nil), d.EnvVars...)
			}
			deps = append(deps, d)
		}
		merged, err := mergeProfilePreInstall(s.Name, name, s.PreInstall, p.PreInstall)
		if err != nil {
			return err
		}
		s.PreInstall = merged
	}
	// Scenario-inline dependencies follow the profile-derived ones.
	deps = append(deps, s.Dependencies...)
	s.Dependencies = deps
	// Clear Profiles so ResolveProfiles is idempotent: a second call must not
	// re-expand and double the already-resolved dependencies.
	s.Profiles = nil
	return nil
}

// mergeProfilePreInstall folds a profile's pre-install hook into the scenario's
// current pre-install. Profiles may only contribute fixtures; the result unions
// fixtures. Merging into a scenario script pre-install (or a profile declaring a
// script) is a conflict.
func mergeProfilePreInstall(scenario, profile string, existing, add *LifecycleHook) (*LifecycleHook, error) {
	if add == nil {
		return existing, nil
	}
	if add.Script != "" {
		return nil, fmt.Errorf("scenario %q: dependency profile %q pre-install must use fixtures, not script", scenario, profile)
	}
	if existing == nil {
		clone := *add
		clone.Fixtures = append([]string(nil), add.Fixtures...)
		return &clone, nil
	}
	if existing.Script != "" {
		return nil, fmt.Errorf("scenario %q: cannot merge fixtures from dependency profile %q into a script pre-install", scenario, profile)
	}
	// Concatenate descriptions so no profile's rationale is silently dropped
	// when two fixture-contributing hooks merge.
	desc := existing.Description
	if add.Description != "" {
		if desc != "" {
			desc += "\n" + add.Description
		} else {
			desc = add.Description
		}
	}
	// Ordering: existing fixtures (the scenario inline hook, or the accumulator
	// from earlier profiles in list order) come first, then this profile's, so
	// multiple fixture-contributing profiles yield profile-list order. Duplicate
	// fixture names are dropped (first occurrence wins) so e.g. an inline
	// postgresql-cluster.yaml plus a profile that also provides it does not
	// apply the same manifest twice.
	merged := &LifecycleHook{
		Fixtures:    dedupeStrings(append(append([]string(nil), existing.Fixtures...), add.Fixtures...)),
		Description: desc,
	}
	return merged, nil
}

// dedupeStrings returns s with duplicate values removed, preserving first-seen
// order.
func dedupeStrings(s []string) []string {
	seen := make(map[string]bool, len(s))
	out := s[:0]
	for _, v := range s {
		if seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

// PermittedFlows holds the parsed content of .github/config/permitted-flows.yaml.
type PermittedFlows struct {
	Defaults struct {
		Flows []string `yaml:"flows"`
	} `yaml:"defaults"`
	Rules []PermittedFlowRule `yaml:"rules"`
}

// PermittedFlowRule represents a single deny rule.
type PermittedFlowRule struct {
	Match string   `yaml:"match"`
	Deny  []string `yaml:"deny"`
}

// LoadPermittedFlows reads and parses .github/config/permitted-flows.yaml.
func LoadPermittedFlows(repoRoot string) (*PermittedFlows, error) {
	path := filepath.Join(repoRoot, ".github", "config", "permitted-flows.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read permitted-flows.yaml: %w", err)
	}
	var pf PermittedFlows
	if err := yaml.Unmarshal(data, &pf); err != nil {
		return nil, fmt.Errorf("failed to parse permitted-flows.yaml: %w", err)
	}
	return &pf, nil
}

// FilterFlows removes denied flows for a given version based on the permitted-flows rules.
// It returns the filtered list of flows that are permitted for the version.
func FilterFlows(pf *PermittedFlows, version string, flows []string) []string {
	// Build the set of denied flows for this version
	denied := make(map[string]bool)
	for _, rule := range pf.Rules {
		if matchesVersion(rule.Match, version) {
			for _, flow := range rule.Deny {
				denied[flow] = true
			}
		}
	}

	// Filter out denied flows
	var permitted []string
	for _, flow := range flows {
		if !denied[flow] {
			permitted = append(permitted, flow)
		}
	}
	return permitted
}

// matchesVersion checks if a version matches a semver-like constraint.
// Supports: "==X.Y", "<=X.Y", ">=X.Y", "<X.Y", ">X.Y".
func matchesVersion(constraint, version string) bool {
	constraint = strings.TrimSpace(constraint)
	if constraint == "" {
		return false
	}

	// Extract operator and target version
	var op, target string
	if strings.HasPrefix(constraint, "<=") {
		op = "<="
		target = strings.TrimSpace(constraint[2:])
	} else if strings.HasPrefix(constraint, ">=") {
		op = ">="
		target = strings.TrimSpace(constraint[2:])
	} else if strings.HasPrefix(constraint, "==") {
		op = "=="
		target = strings.TrimSpace(constraint[2:])
	} else if strings.HasPrefix(constraint, "<") {
		op = "<"
		target = strings.TrimSpace(constraint[1:])
	} else if strings.HasPrefix(constraint, ">") {
		op = ">"
		target = strings.TrimSpace(constraint[1:])
	} else {
		// No operator means exact match
		op = "=="
		target = constraint
	}

	cmp := compareVersions(version, target)
	switch op {
	case "==":
		return cmp == 0
	case "<=":
		return cmp <= 0
	case ">=":
		return cmp >= 0
	case "<":
		return cmp < 0
	case ">":
		return cmp > 0
	}
	return false
}

// compareVersions compares two "major.minor" version strings.
// Returns -1 if a < b, 0 if a == b, 1 if a > b.
func compareVersions(a, b string) int {
	aMajor, aMinor := parseVersion(a)
	bMajor, bMinor := parseVersion(b)

	if aMajor != bMajor {
		if aMajor < bMajor {
			return -1
		}
		return 1
	}
	if aMinor != bMinor {
		if aMinor < bMinor {
			return -1
		}
		return 1
	}
	return 0
}

// parseVersion extracts major and minor from a "X.Y" string.
func parseVersion(v string) (int, int) {
	parts := strings.SplitN(v, ".", 2)
	major := 0
	minor := 0
	if len(parts) >= 1 {
		major, _ = strconv.Atoi(parts[0])
	}
	if len(parts) >= 2 {
		minor, _ = strconv.Atoi(parts[1])
	}
	return major, minor
}
