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
//   - no two post-fan-out CIScenario entries collide on
//     (Shortname, Flow, Platform) after applying matrix.Generate's flow
//     defaulting (empty flow treated as "install") — the natural CI namespace key;
//   - every scenario's (Platform, Flow) is not denied by
//     .github/config/permitted-flows.yaml for this chart version.
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
	featuresDir := filepath.Join(scenariosDir, "chart-full-setup", "values", "features")

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
			scriptPath := filepath.Join(scriptsDir, hook.Script)
			if info, err := os.Stat(scriptPath); err != nil || info.IsDir() {
				problems = append(problems, fmt.Sprintf("%s: script %q: missing or not a file at %s", ctx, hook.Script, scriptPath))
			}
		}
		for _, fx := range hook.Fixtures {
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
		checkHook(label+" post-deploy", scn.PostDeploy)
		for _, feat := range scn.Features {
			checkFeature(label, feat)
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

	if len(problems) == 0 {
		return nil
	}
	sort.Strings(problems)
	return fmt.Errorf("registry validation failed:\n  - %s", strings.Join(problems, "\n  - "))
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
