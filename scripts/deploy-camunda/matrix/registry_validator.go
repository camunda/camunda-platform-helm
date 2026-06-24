package matrix

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// RegistryValidator enforces the registry's load-time invariants from
// ADR 0093 §3:
//
//   - every LifecycleHook passes LifecycleHook.Validate (description set,
//     exactly one of fixtures or script, plain basenames);
//   - referenced basenames (hook fixtures under common/resources/, hook
//     scripts under pre-setup-scripts/, feature values-files, dependency
//     values-files) resolve to existing files;
//   - no two post-fan-out CIScenario entries collide on
//     (Shortname, Flow, Platform) after applying matrix.Generate's flow
//     defaulting (empty flow treated as "install") — the natural CI namespace key;
//   - every scenario's (Platform, Flow) is not denied by
//     .github/config/permitted-flows.yaml for this chart version;
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

	var problems []string
	scenariosDir := filepath.Join(v.ChartDir, "test", "integration", "scenarios")
	resourcesDir := filepath.Join(scenariosDir, "common", "resources")
	scriptsDir := filepath.Join(scenariosDir, "pre-setup-scripts")
	chartFullSetupDir := filepath.Join(scenariosDir, "chart-full-setup")
	featuresDir := filepath.Join(chartFullSetupDir, "values", "features")

	// Referenced-set tracking — feeds the orphan walks below. Populated as a
	// side effect of checkHook so every hook iteration path (PR/Nightly/
	// dependency-profile/flow) contributes to the orphan exemption set.
	referencedScripts := map[string]bool{}
	referencedFixtures := map[string]bool{}

	// Hook validity + basename resolution.
	checkHook := func(ctx string, hook *LifecycleHook) {
		if hook == nil {
			return
		}
		if err := hook.Validate(ctx); err != nil {
			problems = append(problems, err.Error())
			return
		}
		if hook.Script != "" {
			referencedScripts[hook.Script] = true
			scriptPath := filepath.Join(scriptsDir, hook.Script)
			if info, err := os.Stat(scriptPath); err != nil || info.IsDir() {
				problems = append(problems, fmt.Sprintf("%s: script %q: missing or not a file at %s", ctx, hook.Script, scriptPath))
			}
		}
		for _, fx := range hook.Fixtures {
			referencedFixtures[fx] = true
			fxPath := filepath.Join(resourcesDir, fx)
			if info, err := os.Stat(fxPath); err != nil || info.IsDir() {
				problems = append(problems, fmt.Sprintf("%s: fixture %q: missing or not a file at %s", ctx, fx, fxPath))
			}
		}
	}

	// Feature and dependency values-file resolution.
	checkFeature := func(ctx, feature string) {
		fPath := filepath.Join(featuresDir, feature+".yaml")
		if info, err := os.Stat(fPath); err != nil || info.IsDir() {
			problems = append(problems, fmt.Sprintf("%s: feature %q: missing values file at %s", ctx, feature, fPath))
		}
	}
	// Per-scenario extra-values resolution: relative paths resolve against the
	// scenario's chart-full-setup dir (matching appendScenarioExtraValues in
	// runner_execute.go); absolute paths are runtime-supplied and not validated.
	checkExtraValues := func(ctx, ev string) {
		if filepath.IsAbs(ev) {
			return
		}
		path := filepath.Join(chartFullSetupDir, ev)
		// Reject paths that escape chart-full-setup via `..` traversal.
		rel, err := filepath.Rel(chartFullSetupDir, path)
		if err != nil || strings.HasPrefix(rel, "..") {
			problems = append(problems, fmt.Sprintf("%s: extra-values %q: path escapes chart-full-setup dir", ctx, ev))
			return
		}
		if info, err := os.Stat(path); err != nil || info.IsDir() {
			problems = append(problems, fmt.Sprintf("%s: extra-values %q: missing values file at %s", ctx, ev, path))
		}
	}
	checkDep := func(ctx string, dep ChartDependency) {
		if dep.ValuesFile == "" {
			return
		}
		// values-file paths in deps are relative to the repository root,
		// matching the runner's resolution at runner.go:1742.
		path := filepath.Join(repoRoot, dep.ValuesFile)
		if info, err := os.Stat(path); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				problems = append(problems, fmt.Sprintf("%s: dependency %s: values-file %q: missing at %s", ctx, dep.ReleaseName, dep.ValuesFile, path))
				return
			}
			problems = append(problems, fmt.Sprintf("%s: dependency %s: values-file %q: stat error at %s: %v", ctx, dep.ReleaseName, dep.ValuesFile, path, err))
			return
		} else if info.IsDir() {
			problems = append(problems, fmt.Sprintf("%s: dependency %s: values-file %q: is a directory at %s", ctx, dep.ReleaseName, dep.ValuesFile, path))
		}
	}

	// Permitted-flows is shared across all scenarios for this version; load once.
	pf, err := LoadPermittedFlows(repoRoot)
	if err != nil {
		// Loader path mismatch is a deployment-time failure, not a registry
		// authoring one — surface it but don't gate the other checks.
		problems = append(problems, fmt.Sprintf("permitted-flows: %v", err))
	}

	// Track (Shortname, Flow, Platform) tuples for duplicate detection. This is
	// the exact tuple Kubernetes namespace generation uses in runner.go
	// (`<version>-<shortname>-<flow>[-<platform>]`); including the scenario
	// name in the key would let two scenarios with distinct names but the same
	// shortname collide silently at namespace generation time, since
	// runner.go's formula ignores Name entirely. Empty Platform is replaced
	// with "" in the key — matrix.Generate defaults these to "gke" downstream,
	// but the validator's job is to catch source-level duplication only.
	type key struct {
		shortname, flow, platform string
	}
	seen := map[key]string{} // key -> first occurrence label

	for _, scn := range cfg.Integration.Case.PR.Scenarios {
		label := fmt.Sprintf("scenario %q (shortname %q, flow %q)", scn.Name, scn.Shortname, scn.Flow)
		checkHook(label+" pre-install", scn.PreInstall)
		checkHook(label+" post-infra", scn.PostInfra)
		checkHook(label+" post-deploy", scn.PostDeploy)
		for _, feat := range scn.Features {
			checkFeature(label, feat)
		}
		for _, ev := range scn.ExtraValues {
			checkExtraValues(label, ev)
		}
		for _, dep := range scn.Dependencies {
			checkDep(label, dep)
		}

		platforms := scn.Platforms
		if len(platforms) == 0 {
			platforms = []string{""}
		}

		effectiveFlow := strings.TrimSpace(scn.Flow)
		if effectiveFlow == "" {
			effectiveFlow = "install"
		}

		if pf != nil {
			permitted := FilterFlows(pf, version, []string{effectiveFlow})
			if len(permitted) == 0 {
				problems = append(problems, fmt.Sprintf("%s: flow %q denied by permitted-flows for version %s", label, effectiveFlow, version))
			}
		}
		for _, plat := range platforms {
			k := key{scn.Shortname, effectiveFlow, plat}
			if prev, ok := seen[k]; ok {
				problems = append(problems, fmt.Sprintf("%s platform %q: duplicate of %s", label, plat, prev))
			} else {
				seen[k] = label + " platform " + plat
			}
		}
	}

	// Nightly scenarios contribute to the referenced-set so a fixture/script
	// only used by a nightly scenario is not flagged as orphan. Hook validity
	// and basename resolution still apply.
	for _, scn := range cfg.Integration.Case.Nightly.Scenarios {
		label := fmt.Sprintf("nightly scenario %q (shortname %q, flow %q)", scn.Name, scn.Shortname, scn.Flow)
		checkHook(label+" pre-install", scn.PreInstall)
		checkHook(label+" post-infra", scn.PostInfra)
		checkHook(label+" post-deploy", scn.PostDeploy)
		for _, ev := range scn.ExtraValues {
			checkExtraValues(label, ev)
		}
	}

	// Dependency-profile pre-install hooks: validate even profiles that no
	// scenario references yet, so a typo is caught before activation.
	for profName, prof := range cfg.Integration.DependencyProfiles {
		checkHook(fmt.Sprintf("dependency-profile %q pre-install", profName), prof.PreInstall)
	}

	// Flow-scoped pre-upgrade hooks (two-step upgrade flows).
	for flowName, hooks := range cfg.Integration.Flows {
		if hooks == nil {
			continue
		}
		checkHook(fmt.Sprintf("flow %q pre-upgrade", flowName), hooks.PreUpgrade)
	}

	// Orphan walk: every .sh in pre-setup-scripts/ must be referenced by some
	// LifecycleHook, or be exempt by one of two conventions:
	//   1. Sibling-invoked helper: another .sh in the same dir calls it via
	//      `bash "${SCRIPT_DIR}/<name>"` — detected by substring search.
	//   2. Explicit marker: the file contains a line matching
	//      `# orphan-ok: <non-empty reason>` (reason required).
	//
	// To exempt a file via marker, add to its header:
	//   # orphan-ok: <reason explaining why it has no LifecycleHook reference>
	// Alternatively, reference it from a LifecycleHook (script: <name>).
	var scriptEntries []os.DirEntry
	if entries, err := os.ReadDir(scriptsDir); err == nil {
		scriptEntries = entries
	} else if !errors.Is(err, fs.ErrNotExist) {
		problems = append(problems, fmt.Sprintf("read pre-setup-scripts/: %v", err))
	}
	// Build sibling content map (full file) for invocation detection, and a
	// separate header map (first 1KB) for orphan-ok marker scanning.
	siblingContent := map[string]string{}
	siblingHeader := map[string]string{}
	for _, e := range scriptEntries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sh") {
			continue
		}
		path := filepath.Join(scriptsDir, e.Name())
		if data, err := os.ReadFile(path); err == nil {
			siblingContent[e.Name()] = string(data)
		}
		if header, err := readFirstKB(path); err == nil {
			siblingHeader[e.Name()] = header
		}
	}
	for _, e := range scriptEntries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".sh") {
			continue
		}
		if referencedScripts[name] {
			continue
		}
		// Check sibling invocation (helper called by another script in the same dir).
		if isSiblingInvoked(name, siblingContent) {
			continue
		}
		// Check orphan-ok header marker.
		ok, reason, err := parseOrphanOk(siblingHeader[name])
		if err != nil {
			problems = append(problems, fmt.Sprintf("script %q: \"orphan-ok:\" header found but reason is empty — add a reason after the colon", name))
			continue
		}
		if ok {
			_ = reason
			continue
		}
		problems = append(problems, fmt.Sprintf(
			"orphan script %q in pre-setup-scripts/: no LifecycleHook references it and no \"# orphan-ok: <reason>\" header found"+
				" — either add it to a LifecycleHook (script: %s) or add \"# orphan-ok: <reason>\" to the file header",
			name, name))
	}

	// Orphan walk: every .yaml/.yml in common/resources/ must be referenced by
	// some LifecycleHook, or carry an explicit marker:
	//   # orphan-ok: <reason>
	// (staged-but-disabled fixtures and resources not yet wired to a hook).
	if entries, err := os.ReadDir(resourcesDir); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
				continue
			}
			if referencedFixtures[name] {
				continue
			}
			content, _ := readFirstKB(filepath.Join(resourcesDir, name))
			ok, _, err := parseOrphanOk(content)
			if err != nil {
				problems = append(problems, fmt.Sprintf("fixture %q: \"orphan-ok:\" header found but reason is empty — add a reason after the colon", name))
				continue
			}
			if ok {
				continue
			}
			problems = append(problems, fmt.Sprintf(
				"orphan fixture %q in common/resources/: no LifecycleHook references it and no \"# orphan-ok: <reason>\" header found"+
					" — either add it to a LifecycleHook (fixtures: [%s]) or add \"# orphan-ok: <reason>\" to the file header",
				name, name))
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		problems = append(problems, fmt.Sprintf("read common/resources/: %v", err))
	}

	if len(problems) == 0 {
		return nil
	}
	sort.Strings(problems)
	return fmt.Errorf("registry validation failed:\n  - %s", strings.Join(problems, "\n  - "))
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

// isSiblingInvoked returns true when any sibling script in siblingContent
// references name via a subprocess call (e.g. `bash "${SCRIPT_DIR}/name"` or
// `exec bash "...name"`). A plain basename substring match is sufficient
// because script names in this codebase are unique within a version directory.
func isSiblingInvoked(name string, siblingContent map[string]string) bool {
	for sib, content := range siblingContent {
		if sib == name {
			continue
		}
		if strings.Contains(content, name) {
			return true
		}
	}
	return false
}

// parseOrphanOk scans content for a line matching `# orphan-ok: <reason>`.
// Returns (true, reason, nil) when found with a non-empty reason,
// (false, "", fmt.Errorf) when the marker is present but has an empty reason,
// and (false, "", nil) when no marker is found.
func parseOrphanOk(content string) (bool, string, error) {
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
			return false, "", fmt.Errorf("empty reason")
		}
		return true, reason, nil
	}
	return false, "", nil
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
