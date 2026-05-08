package matrix

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"scripts/camunda-core/pkg/executil"
	"scripts/camunda-core/pkg/logging"
	"scripts/camunda-core/pkg/versionmatrix"
	"scripts/deploy-camunda/config"
	"scripts/deploy-camunda/deploy"
)

// lifecycleVarPassthrough lists environment variable names that lifecycle
// manifests are allowed to reference. Empty values are not propagated, so a
// missing env var leaves placeholders intact in the rendered YAML and the
// resulting server-side apply fails with a parse/validation error rather
// than silently producing a misconfigured resource.
var lifecycleVarPassthrough = []string{
	"RDBMS_POSTGRESQL_USERNAME",
	"RDBMS_POSTGRESQL_PASSWORD",
	"GITHUB_WORKFLOW_JOB_ID",
	"POSTGRESQL_JDBC_URL",
}

// lifecycleVars builds the variable substitution map used when rendering
// lifecycle manifests for the given namespace and release.
func lifecycleVars(namespace, release string) map[string]string {
	vars := map[string]string{
		"NAMESPACE":    namespace,
		"RELEASE_NAME": release,
	}
	for _, k := range lifecycleVarPassthrough {
		if v := os.Getenv(k); v != "" {
			vars[k] = v
		}
	}
	return vars
}

// hookKind is a lifecycle phase label used in log messages and error wrapping.
type hookKind string

const (
	hookPreInstall hookKind = "pre-install"
	hookPostDeploy hookKind = "post-deploy"
	hookPreUpgrade hookKind = "pre-upgrade"
)

// buildHookFunc returns a closure that executes a validated LifecycleHook
// against the given chart version. The hook MUST have been validated
// upstream via LifecycleHook.Validate; mode dispatch here trusts that
// invariant.
func buildHookFunc(flags *config.RuntimeFlags, hook *LifecycleHook, kind hookKind, repoRoot, appVersion, scenario string) (func(context.Context) error, error) {
	chartPath := filepath.Join(repoRoot, "charts", "camunda-platform-"+appVersion)

	if len(hook.Fixtures) > 0 {
		fixtures := append([]string(nil), hook.Fixtures...)
		return func(hookCtx context.Context) error {
			namespace := flags.EffectiveNamespace()
			release := flags.Deployment.Release
			scenarioCtx := &deploy.ScenarioContext{
				ScenarioName: scenario,
				Namespace:    namespace,
				Release:      release,
			}
			vars := lifecycleVars(namespace, release)
			logging.Logger.Info().
				Str("scenario", scenario).
				Str("appVersion", appVersion).
				Strs("fixtures", fixtures).
				Str("namespace", namespace).
				Msgf("Applying lifecycle fixtures (%s, declarative)", kind)
			return deploy.ApplyLifecycleManifests(hookCtx, scenarioCtx, chartPath, flags.Test.KubeContext, fixtures, vars)
		}, nil
	}

	scriptName := hook.Script
	scriptPath := versionmatrix.PreSetupScriptPath(repoRoot, appVersion, scriptName)
	if !versionmatrix.HasPreSetupScript(repoRoot, appVersion, scriptName) {
		return nil, fmt.Errorf("scenario %q: %s script %q not found at %s", scenario, kind, scriptName, scriptPath)
	}
	return func(hookCtx context.Context) error {
		namespace := flags.EffectiveNamespace()
		logging.Logger.Info().
			Str("script", scriptPath).
			Str("scenario", scenario).
			Str("appVersion", appVersion).
			Str("namespace", namespace).
			Msgf("Running scenario-specific %s script (declarative)", kind)

		scriptEnv := []string{"TEST_NAMESPACE=" + namespace}
		if flags.Test.KubeContext != "" {
			scriptEnv = append(scriptEnv, "KUBE_CONTEXT="+flags.Test.KubeContext)
		}
		for _, k := range lifecycleVarPassthrough {
			if v := os.Getenv(k); v != "" {
				scriptEnv = append(scriptEnv, k+"="+v)
			}
		}

		if err := executil.RunCommand(hookCtx, "bash", []string{"-x", scriptPath}, scriptEnv, ""); err != nil {
			return fmt.Errorf("%s script %s failed: %w", kind, scriptPath, err)
		}
		logging.Logger.Info().
			Str("script", scriptPath).
			Msgf("%s script completed successfully", kind)
		return nil
	}, nil
}

// registerDeclarativeHook validates a LifecycleHook and appends an executor
// closure to the supplied slot (flags.PreInstallHooks or flags.PostDeployHooks).
// repoRoot/appVersion scope script and fixture lookup to a specific chart
// version, which lets two-step upgrade flows target the version actually
// being installed in step 1.
func registerDeclarativeHook(flags *config.RuntimeFlags, hook *LifecycleHook, kind hookKind, slot *[]func(context.Context) error, repoRoot, appVersion, scenario string) error {
	if hook == nil {
		return nil
	}
	if err := hook.Validate(fmt.Sprintf("scenario %q: %s", scenario, kind)); err != nil {
		return err
	}
	fn, err := buildHookFunc(flags, hook, kind, repoRoot, appVersion, scenario)
	if err != nil {
		return err
	}
	*slot = append(*slot, fn)
	return nil
}

// registerDeclarativePreInstallHook is a thin shim that pins the slot and kind
// for pre-install registrations.
func registerDeclarativePreInstallHook(flags *config.RuntimeFlags, hook *LifecycleHook, repoRoot, appVersion, scenario string) error {
	return registerDeclarativeHook(flags, hook, hookPreInstall, &flags.PreInstallHooks, repoRoot, appVersion, scenario)
}

// registerDeclarativePostDeployHook is a thin shim that pins the slot and kind
// for post-deploy registrations.
func registerDeclarativePostDeployHook(flags *config.RuntimeFlags, hook *LifecycleHook, repoRoot, appVersion, scenario string) error {
	return registerDeclarativeHook(flags, hook, hookPostDeploy, &flags.PostDeployHooks, repoRoot, appVersion, scenario)
}

// runDeclarativePreUpgradeHook executes the supplied pre-upgrade hook between
// Step 1 and Step 2 of a two-step upgrade flow, scoped to the target app
// version. A nil hook is a no-op (some flows have no pre-upgrade work).
func runDeclarativePreUpgradeHook(ctx context.Context, flags *config.RuntimeFlags, hook *LifecycleHook, repoRoot, appVersion, flow string) error {
	if hook == nil {
		return nil
	}
	if err := hook.Validate(fmt.Sprintf("flow %q v%s: pre-upgrade", flow, appVersion)); err != nil {
		return err
	}
	fn, err := buildHookFunc(flags, hook, hookPreUpgrade, repoRoot, appVersion, "flow:"+flow)
	if err != nil {
		return err
	}
	return fn(ctx)
}
