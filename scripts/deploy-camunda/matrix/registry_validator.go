// Copyright 2026 Camunda Services GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package matrix

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"

	"scripts/camunda-core/pkg/versionmatrix"
)

const bitnamiMigrationHookScript = "post-infra-bitnami-migration.sh"

const bitnamiMigrationHookID = "post-infra-bitnami-migration"

// RegistryValidator enforces the registry's load-time invariants from
// ADR 0093 §3:
//
//   - every LifecycleHook passes LifecycleHook.Validate (description set,
//     exactly one of fixtures or script, plain basenames);
//   - referenced basenames (hook fixtures under common/resources/, hook
//     scripts under pre-setup-scripts/, feature values-files, persistence
//     values-files, dependency values-files) resolve to existing files;
//   - no upgrade-flow scenario selects a persistence layer that enables, in the
//     previous chart version, a subchart this chart no longer bundles, unless a
//     migration hook bridges the two backends;
//   - no two post-fan-out CIScenario entries collide on
//     (Shortname, Flow, Platform) after applying matrix.Generate's flow
//     defaulting (empty flow treated as "install") — the natural CI namespace key;
//   - every scenario's (Platform, Flow) is not denied by
//     .github/config/permitted-flows.yaml for this chart version;
//   - no scenario combines skip-e2e with any e2e leg declaration, and none sets
//     e2e-full-suite-blocking without e2e-full-suite, or e2e-api-suite-blocking
//     without e2e-api-suite — all are dead config;
//   - no orphan files exist in pre-setup-scripts/ or common/resources/ —
//     every .sh / .yaml must be referenced by at least one LifecycleHook
//     across PR/Nightly scenarios, dependency-profile pre-install hooks, and
//     flow-scoped pre-upgrade hooks, or be exempt via sibling-invocation
//     detection or an in-file "# orphan-ok: <reason>" header marker.
//
// The validator runs at the tail of LoadRegistry. Errors are aggregated and
// returned as a single error so the caller sees every problem at once.
type RegistryValidator struct {
	// ChartDir is the chart's directory (e.g. charts/camunda-platform-8.10).
	// Required.
	ChartDir string
}

type registryValidation struct {
	chartDir, repoRoot, version                 string
	resourcesDir, scriptsDir, setupDir, depsDir string
	problems                                    []string
	referencedScripts, referencedFixtures       map[string]bool
}

// Validate runs every check against the post-fan-out CITestConfig. Returns
// nil when the registry is well-formed; otherwise an error whose Error()
// lists every problem on its own line.
func (v *RegistryValidator) Validate(cfg *CITestConfig) error {
	if v.ChartDir == "" {
		return fmt.Errorf("RegistryValidator: ChartDir is required")
	}
	repoRoot, version, err := deriveRepoRootAndVersion(v.ChartDir)
	if err != nil {
		return err
	}

	scenariosDir := filepath.Join(v.ChartDir, "test", "integration", "scenarios")
	check := registryValidation{
		chartDir: v.ChartDir, repoRoot: repoRoot, version: version,
		resourcesDir:      filepath.Join(scenariosDir, "common", "resources"),
		scriptsDir:        filepath.Join(scenariosDir, "pre-setup-scripts"),
		setupDir:          filepath.Join(scenariosDir, "chart-full-setup"),
		depsDir:           filepath.Join(v.ChartDir, "test", RegistryDirName, "dependencies"),
		referencedScripts: map[string]bool{}, referencedFixtures: map[string]bool{},
	}
	check.checkUpgrades(cfg.Integration.Case.PR.Scenarios)
	check.checkScenarios(cfg)
	check.checkOrphanScripts()
	check.checkOrphanFixtures()
	if len(check.problems) == 0 {
		return nil
	}
	sort.Strings(check.problems)
	return fmt.Errorf("registry validation failed:\n  - %s", strings.Join(check.problems, "\n  - "))
}

func (v *registryValidation) checkHook(ctx string, hook *LifecycleHook) {
	if hook == nil {
		return
	}
	if err := hook.Validate(ctx); err != nil {
		v.problems = append(v.problems, err.Error())
		return
	}
	if hook.Script != "" {
		v.referencedScripts[hook.Script] = true
		path := filepath.Join(v.scriptsDir, hook.Script)
		if info, err := os.Stat(path); err != nil || info.IsDir() {
			v.problems = append(v.problems, fmt.Sprintf("%s: script %q: missing or not a file at %s", ctx, hook.Script, path))
		}
	}
	for _, fixture := range hook.Fixtures {
		v.referencedFixtures[fixture] = true
		path := filepath.Join(v.resourcesDir, fixture)
		if info, err := os.Stat(path); err != nil || info.IsDir() {
			v.problems = append(v.problems, fmt.Sprintf("%s: fixture %q: missing or not a file at %s", ctx, fixture, path))
		}
	}
}

func valuesLayerProblems(label, kind, dir string, ids ...string) []string {
	var problems []string
	for _, id := range ids {
		if !isPlainFilename(id) {
			problems = append(problems, fmt.Sprintf("%s: %s reference %q must be a plain filename (no path separators)", label, kind, id))
			continue
		}
		path := filepath.Join(dir, id+".yaml")
		if info, err := os.Stat(path); err != nil || info.IsDir() {
			problems = append(problems, fmt.Sprintf("%s: %s %q: missing values file at %s", label, kind, id, path))
		}
	}
	return problems
}

func (v *registryValidation) checkExtraValues(ctx, value string) {
	if filepath.IsAbs(value) {
		return
	}
	path := filepath.Join(v.setupDir, value)
	rel, err := filepath.Rel(v.setupDir, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		v.problems = append(v.problems, fmt.Sprintf("%s: extra-values %q: path escapes chart-full-setup dir", ctx, value))
		return
	}
	if info, err := os.Stat(path); err != nil || info.IsDir() {
		v.problems = append(v.problems, fmt.Sprintf("%s: extra-values %q: missing values file at %s", ctx, value, path))
	}
}

func (v *registryValidation) checkDependency(ctx string, dep ChartDependency) {
	if dep.ValuesFile == "" {
		return
	}
	path := filepath.Join(v.repoRoot, dep.ValuesFile)
	if info, err := os.Stat(path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			v.problems = append(v.problems, fmt.Sprintf("%s: dependency %s: values-file %q: missing at %s", ctx, dep.ReleaseName, dep.ValuesFile, path))
			return
		}
		v.problems = append(v.problems, fmt.Sprintf("%s: dependency %s: values-file %q: stat error at %s: %v", ctx, dep.ReleaseName, dep.ValuesFile, path, err))
		return
	} else if info.IsDir() {
		v.problems = append(v.problems, fmt.Sprintf("%s: dependency %s: values-file %q: is a directory at %s", ctx, dep.ReleaseName, dep.ValuesFile, path))
	}
}

func (v *registryValidation) checkUpgrades(scenarios []CIScenario) {
	currentDeps, currentDepsErr := readChartDependencies(v.chartDir)
	prevVersion, err := versionmatrix.PreviousAppVersion(v.version)
	if err != nil {
		return
	}
	prevChartDir := filepath.Join(v.repoRoot, "charts", "camunda-platform-"+prevVersion)
	prevDeps, prevDepsErr := readChartDependencies(prevChartDir)
	prevPersistenceDir := filepath.Join(prevChartDir, "test", "integration", "scenarios", "chart-full-setup", "values", "persistence")
	missingCredentialKeys, err := credentialKeysMissingFrom(v.chartDir, prevChartDir)
	if err != nil {
		v.problems = append(v.problems, err.Error())
	}
	for _, scn := range scenarios {
		label, flow := scenarioLabel(scn), scenarioFlow(scn)
		if len(missingCredentialKeys) > 0 && slices.Contains(strings.Split(flow, ","), "upgrade-minor") {
			v.problems = append(v.problems, fmt.Sprintf(
				"%s: integration-test credentials lack keys that chart version %s reads (%s)"+
					" — CI applies this chart's ExternalSecret before step 1 of the two-step upgrade installs %s;"+
					" add the keys to this chart's ExternalSecret",
				label, prevVersion, strings.Join(missingCredentialKeys, ", "), prevVersion))
		}
		if scn.Persistence == "" || !isPlainFilename(scn.Persistence) ||
			(flow != "upgrade-minor" && !versionmatrix.IsUpgradeOnlyFlow(flow)) ||
			currentDepsErr != nil || prevDepsErr != nil {
			continue
		}
		prevValues, err := readYAMLMap(filepath.Join(prevPersistenceDir, scn.Persistence+".yaml"))
		if err != nil {
			if flow == "upgrade-minor" {
				v.problems = append(v.problems, fmt.Sprintf(
					"%s: persistence %q does not resolve in chart version %s (%v)"+
						" — step 1 of the two-step upgrade installs that chart from its own scenario dir;"+
						" add the layer there or select one both versions provide",
					label, scn.Persistence, prevVersion, err))
			}
			continue
		}
		if scn.PostInfra != nil && scn.PostInfra.Script == bitnamiMigrationHookScript {
			continue
		}
		for _, dep := range prevDeps {
			if dep.Condition == "" || hasChartDependency(currentDeps, dep.Name) || !anyConditionKeyTrue(prevValues, dep.Condition) {
				continue
			}
			v.problems = append(v.problems, fmt.Sprintf(
				"%s: persistence %q resolves in chart version %s to a values file that enables subchart %q (%s: true), which this chart no longer bundles"+
					" — the upgrade would address two different backends and start against empty storage;"+
					" select a companion persistence layer both versions can reach, or declare \"post-infra: %s\" to migrate the data first",
				label, scn.Persistence, prevVersion, dep.Name, dep.Condition, bitnamiMigrationHookID))
		}
	}
}

func scenarioLabel(scn CIScenario) string {
	return fmt.Sprintf("scenario %q (shortname %q, flow %q)", scn.Name, scn.Shortname, scn.Flow)
}

func scenarioFlow(scn CIScenario) string {
	if flow := strings.TrimSpace(scn.Flow); flow != "" {
		return flow
	}
	return "install"
}

func (v *registryValidation) checkScenarios(cfg *CITestConfig) {
	pf, err := LoadPermittedFlows(v.repoRoot)
	if err != nil {
		v.problems = append(v.problems, fmt.Sprintf("permitted-flows: %v", err))
	}
	seen := map[[3]string]string{}
	for _, scn := range cfg.Integration.Case.PR.Scenarios {
		v.checkScenario(scn, pf, seen)
	}
	for _, scn := range cfg.Integration.Case.Nightly.Scenarios {
		label := "nightly " + scenarioLabel(scn)
		v.checkHook(label+" pre-install", scn.PreInstall)
		v.checkHook(label+" post-infra", scn.PostInfra)
		v.checkHook(label+" post-deploy", scn.PostDeploy)
		for _, ev := range scn.ExtraValues {
			v.checkExtraValues(label, ev)
		}
	}
	for name, hooks := range cfg.Integration.Flows {
		if hooks != nil {
			v.checkHook(fmt.Sprintf("flow %q pre-upgrade", name), hooks.PreUpgrade)
		}
	}
}

func (v *registryValidation) checkScenario(scn CIScenario, pf *PermittedFlows, seen map[[3]string]string) {
	label := scenarioLabel(scn)
	v.checkHook(label+" pre-install", scn.PreInstall)
	v.checkHook(label+" post-infra", scn.PostInfra)
	v.checkHook(label+" post-deploy", scn.PostDeploy)
	v.problems = append(v.problems, valuesLayerProblems(label, "feature", filepath.Join(v.setupDir, "values", "features"), scn.Features...)...)
	for _, ev := range scn.ExtraValues {
		v.checkExtraValues(label, ev)
	}
	for _, dep := range scn.Dependencies {
		v.checkDependency(label, dep)
	}
	if scn.Persistence != "" {
		v.problems = append(v.problems, valuesLayerProblems(label, "persistence", filepath.Join(v.setupDir, "values", "persistence"), scn.Persistence)...)
	}
	if err := scn.Topology.Validate(label, v.chartDir, v.depsDir); err != nil {
		v.problems = append(v.problems, err.Error())
	}
	v.problems = append(v.problems, scenarioE2EProblems(label, scn)...)
	flow := scenarioFlow(scn)
	if pf != nil && len(FilterFlows(pf, v.version, []string{flow})) == 0 {
		v.problems = append(v.problems, fmt.Sprintf("%s: flow %q denied by permitted-flows for version %s", label, flow, v.version))
	}
	platforms := scn.Platforms
	if len(platforms) == 0 {
		platforms = []string{""}
	}
	for _, platform := range platforms {
		key := [3]string{scn.Shortname, flow, platform}
		if previous, ok := seen[key]; ok {
			v.problems = append(v.problems, fmt.Sprintf("%s platform %q: duplicate of %s", label, platform, previous))
		} else {
			seen[key] = label + " platform " + platform
		}
	}
}

func scenarioE2EProblems(label string, scn CIScenario) []string {
	var problems []string
	if scn.SkipE2E {
		for _, field := range []struct {
			name string
			set  bool
		}{
			{"e2e-full-suite", scn.E2EFullSuite},
			{"e2e-smoke-blocking", scn.E2ESmokeBlocking != nil},
			{"e2e-full-suite-blocking", scn.E2EFullSuiteBlocking != nil},
			{"e2e-api-suite", scn.E2EAPISuite},
			{"e2e-api-suite-blocking", scn.E2EAPISuiteBlocking != nil},
		} {
			if field.set {
				problems = append(problems, fmt.Sprintf("%s: %s is set but skip-e2e disables e2e entirely", label, field.name))
			}
		}
	}
	if !scn.E2EFullSuite && scn.E2EFullSuiteBlocking != nil {
		problems = append(problems, fmt.Sprintf("%s: e2e-full-suite-blocking is set but e2e-full-suite is not enabled", label))
	}
	if !scn.E2EAPISuite && scn.E2EAPISuiteBlocking != nil {
		problems = append(problems, fmt.Sprintf("%s: e2e-api-suite-blocking is set but e2e-api-suite is not enabled", label))
	}
	return problems
}

func (v *registryValidation) checkOrphanScripts() {
	entries, err := os.ReadDir(v.scriptsDir)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			v.problems = append(v.problems, fmt.Sprintf("read pre-setup-scripts/: %v", err))
		}
		return
	}
	siblingContent := map[string]string{}
	siblingHeader := map[string]string{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sh") {
			continue
		}
		path := filepath.Join(v.scriptsDir, e.Name())
		if data, err := os.ReadFile(path); err == nil {
			siblingContent[e.Name()] = string(data)
		}
		if header, err := readFirstKB(path); err == nil {
			siblingHeader[e.Name()] = header
		}
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".sh") || v.referencedScripts[name] || isSiblingInvoked(name, siblingContent) {
			continue
		}
		ok, err := parseOrphanOk(siblingHeader[name])
		if err != nil {
			v.problems = append(v.problems, fmt.Sprintf("script %q: \"orphan-ok:\" header found but reason is empty — add a reason after the colon", name))
			continue
		}
		if ok {
			continue
		}
		v.problems = append(v.problems, fmt.Sprintf(
			"orphan script %q in pre-setup-scripts/: no LifecycleHook references it and no \"# orphan-ok: <reason>\" header found"+
				" — either add it to a LifecycleHook (script: %s) or add \"# orphan-ok: <reason>\" to the file header",
			name, name))
	}
}

func (v *registryValidation) checkOrphanFixtures() {
	entries, err := os.ReadDir(v.resourcesDir)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			v.problems = append(v.problems, fmt.Sprintf("read common/resources/: %v", err))
		}
		return
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || (!strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml")) || v.referencedFixtures[name] {
			continue
		}
		content, _ := readFirstKB(filepath.Join(v.resourcesDir, name))
		ok, err := parseOrphanOk(content)
		if err != nil {
			v.problems = append(v.problems, fmt.Sprintf("fixture %q: \"orphan-ok:\" header found but reason is empty — add a reason after the colon", name))
			continue
		}
		if ok {
			continue
		}
		v.problems = append(v.problems, fmt.Sprintf(
			"orphan fixture %q in common/resources/: no LifecycleHook references it and no \"# orphan-ok: <reason>\" header found"+
				" — either add it to a LifecycleHook (fixtures: [%s]) or add \"# orphan-ok: <reason>\" to the file header",
			name, name))
	}
}

func credentialKeysMissingFrom(chartDir, prevChartDir string) ([]string, error) {
	referenced, err := fileTokens(filepath.Join(prevChartDir, "test", "integration", "scenarios"))
	if err != nil {
		return nil, err
	}
	var missing []string
	for _, name := range []string{
		"external-secret-integration-test-credentials.yaml",
		"external-secret-integration-test-credentials-vault.yaml",
	} {
		prevKeys, err := externalSecretKeys(filepath.Join(prevChartDir, "test", "integration", "external-secrets", name))
		if err != nil {
			return nil, err
		}
		keys, err := externalSecretKeys(filepath.Join(chartDir, "test", "integration", "external-secrets", name))
		if err != nil {
			return nil, err
		}
		if prevKeys == nil || keys == nil {
			continue
		}
		for key := range prevKeys {
			if referenced[key] && !keys[key] {
				missing = append(missing, name+": "+key)
			}
		}
	}
	sort.Strings(missing)
	return missing, nil
}

func fileTokens(dir string) (map[string]bool, error) {
	tokens := map[string]bool{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if errors.Is(err, fs.ErrNotExist) && path == dir {
			return filepath.SkipDir
		}
		if err != nil {
			return err
		}
		if !d.Type().IsRegular() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, token := range strings.FieldsFunc(string(data), func(r rune) bool {
			return r != '-' && r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r)
		}) {
			tokens[token] = true
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan %s: %w", dir, err)
	}
	return tokens, nil
}

func externalSecretKeys(path string) (map[string]bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var es struct {
		Spec struct {
			Target struct {
				Template struct {
					Data map[string]string `yaml:"data"`
				} `yaml:"template"`
			} `yaml:"target"`
			Data []struct {
				SecretKey string `yaml:"secretKey"`
			} `yaml:"data"`
		} `yaml:"spec"`
	}
	if err := yaml.Unmarshal(data, &es); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	keys := map[string]bool{}
	for key := range es.Spec.Target.Template.Data {
		keys[key] = true
	}
	for _, d := range es.Spec.Data {
		keys[d.SecretKey] = true
	}
	return keys, nil
}

// readFirstKB reads at most 1024 bytes from a file and returns them as a
// string. Used to scan file headers without loading whole files.
func readFirstKB(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	buf := make([]byte, 1024)
	n, _ := f.Read(buf)
	return string(buf[:n]), nil
}

// isSiblingInvoked returns true when any sibling script contains an invocation
// of name — i.e. a line that both contains the filename and also contains a
// shell-invocation keyword (bash, source) or a dot-source prefix (". ").
// Requiring an invocation keyword prevents a comment that merely mentions the
// filename from suppressing the orphan check.
func isSiblingInvoked(name string, siblingContent map[string]string) bool {
	for sib, content := range siblingContent {
		if sib == name {
			continue
		}
		for line := range strings.SplitSeq(content, "\n") {
			if !strings.Contains(line, name) {
				continue
			}
			if strings.Contains(line, "bash") || strings.Contains(line, "source") ||
				strings.HasPrefix(strings.TrimSpace(line), ". ") {
				return true
			}
		}
	}
	return false
}

// parseOrphanOk scans content for a line matching `# orphan-ok: <reason>`.
// Returns (true, nil) when found with a non-empty reason,
// (false, fmt.Errorf) when the marker is present but has an empty reason,
// and (false, nil) when no marker is found.
func parseOrphanOk(content string) (bool, error) {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "#") {
			continue
		}
		rest := strings.TrimPrefix(trimmed, "#")
		rest = strings.TrimSpace(rest)
		if !strings.HasPrefix(rest, "orphan-ok:") {
			continue
		}
		reason := strings.TrimSpace(strings.TrimPrefix(rest, "orphan-ok:"))
		if reason == "" {
			return false, fmt.Errorf("empty reason")
		}
		return true, nil
	}
	return false, nil
}

// deriveRepoRootAndVersion turns chartDir = .../charts/camunda-platform-<X.Y>
// into (repoRoot, "X.Y"). Returns an error if the path shape doesn't match.
func deriveRepoRootAndVersion(chartDir string) (string, string, error) {
	abs, err := filepath.Abs(chartDir)
	if err != nil {
		return "", "", fmt.Errorf("resolve chartDir %q: %w", chartDir, err)
	}
	base := filepath.Base(abs)
	const prefix = "camunda-platform-"
	if !strings.HasPrefix(base, prefix) {
		return "", "", fmt.Errorf("chartDir basename %q does not match %s<version>", base, prefix)
	}
	version := strings.TrimPrefix(base, prefix)
	repoRoot := filepath.Dir(filepath.Dir(abs)) // chartDir → charts/ → repoRoot
	return repoRoot, version, nil
}
