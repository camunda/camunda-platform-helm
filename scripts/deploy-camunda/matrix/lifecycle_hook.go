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

// registerDeclarativePreInstallHook validates the declarative pre-install
// LifecycleHook and appends an appropriate PreInstallHook to the supplied
// deploy flags. The repoRoot/appVersion pair scopes script and fixture lookup
// to a specific chart version, which lets two-step upgrade flows target the
// version actually being installed in step 1.
func registerDeclarativePreInstallHook(flags *config.RuntimeFlags, hook *LifecycleHook, repoRoot, appVersion, scenario string) error {
	if hook == nil {
		return nil
	}

	hasFixtures := len(hook.Fixtures) > 0
	hasScript := hook.Script != ""
	if hasFixtures == hasScript {
		return fmt.Errorf("scenario %q: pre-install must specify exactly one of fixtures or script", scenario)
	}

	chartPath := filepath.Join(repoRoot, "charts", "camunda-platform-"+appVersion)

	switch {
	case hasFixtures:
		fixtures := append([]string(nil), hook.Fixtures...)
		scn := scenario
		ver := appVersion
		flags.PreInstallHooks = append(flags.PreInstallHooks, func(hookCtx context.Context) error {
			namespace := flags.EffectiveNamespace()
			release := flags.Deployment.Release
			scenarioCtx := &deploy.ScenarioContext{
				ScenarioName: scn,
				Namespace:    namespace,
				Release:      release,
			}
			vars := lifecycleVars(namespace, release)
			logging.Logger.Info().
				Str("scenario", scn).
				Str("appVersion", ver).
				Strs("fixtures", fixtures).
				Str("namespace", namespace).
				Msg("Applying lifecycle fixtures (PreInstallHook, declarative)")
			return deploy.ApplyLifecycleManifests(hookCtx, scenarioCtx, chartPath, flags.Test.KubeContext, fixtures, vars)
		})
	case hasScript:
		scriptName := hook.Script
		scriptPath := versionmatrix.PreSetupScriptPath(repoRoot, appVersion, scriptName)
		if !versionmatrix.HasPreSetupScript(repoRoot, appVersion, scriptName) {
			return fmt.Errorf("scenario %q: pre-install script %q not found at %s",
				scenario, scriptName, scriptPath)
		}
		scn := scenario
		ver := appVersion
		flags.PreInstallHooks = append(flags.PreInstallHooks, func(hookCtx context.Context) error {
			namespace := flags.EffectiveNamespace()
			logging.Logger.Info().
				Str("script", scriptPath).
				Str("scenario", scn).
				Str("appVersion", ver).
				Str("namespace", namespace).
				Msg("Running scenario-specific pre-install script (PreInstallHook, declarative)")

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
				return fmt.Errorf("pre-install script %s failed: %w", scriptPath, err)
			}
			logging.Logger.Info().
				Str("script", scriptPath).
				Msg("Pre-install script completed successfully")
			return nil
		})
	}
	return nil
}

// registerDeclarativePostDeployHook validates the declarative post-deploy
// LifecycleHook and appends an appropriate PostDeployHook to the supplied
// deploy flags. The hook fires after helm upgrade/install completes
// successfully.
func registerDeclarativePostDeployHook(flags *config.RuntimeFlags, hook *LifecycleHook, repoRoot, appVersion, scenario string) error {
	if hook == nil {
		return nil
	}

	hasFixtures := len(hook.Fixtures) > 0
	hasScript := hook.Script != ""
	if hasFixtures == hasScript {
		return fmt.Errorf("scenario %q: post-deploy must specify exactly one of fixtures or script", scenario)
	}

	chartPath := filepath.Join(repoRoot, "charts", "camunda-platform-"+appVersion)

	switch {
	case hasFixtures:
		fixtures := append([]string(nil), hook.Fixtures...)
		scn := scenario
		ver := appVersion
		flags.PostDeployHooks = append(flags.PostDeployHooks, func(hookCtx context.Context) error {
			namespace := flags.EffectiveNamespace()
			release := flags.Deployment.Release
			scenarioCtx := &deploy.ScenarioContext{
				ScenarioName: scn,
				Namespace:    namespace,
				Release:      release,
			}
			vars := lifecycleVars(namespace, release)
			logging.Logger.Info().
				Str("scenario", scn).
				Str("appVersion", ver).
				Strs("fixtures", fixtures).
				Str("namespace", namespace).
				Msg("Applying lifecycle fixtures (PostDeployHook, declarative)")
			return deploy.ApplyLifecycleManifests(hookCtx, scenarioCtx, chartPath, flags.Test.KubeContext, fixtures, vars)
		})
	case hasScript:
		scriptName := hook.Script
		scriptPath := versionmatrix.PreSetupScriptPath(repoRoot, appVersion, scriptName)
		if !versionmatrix.HasPreSetupScript(repoRoot, appVersion, scriptName) {
			return fmt.Errorf("scenario %q: post-deploy script %q not found at %s",
				scenario, scriptName, scriptPath)
		}
		scn := scenario
		ver := appVersion
		flags.PostDeployHooks = append(flags.PostDeployHooks, func(hookCtx context.Context) error {
			namespace := flags.EffectiveNamespace()
			logging.Logger.Info().
				Str("script", scriptPath).
				Str("scenario", scn).
				Str("appVersion", ver).
				Str("namespace", namespace).
				Msg("Running scenario-specific post-deploy script (PostDeployHook, declarative)")

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
				return fmt.Errorf("post-deploy script %s failed: %w", scriptPath, err)
			}
			logging.Logger.Info().
				Str("script", scriptPath).
				Msg("Post-deploy script completed successfully")
			return nil
		})
	}
	return nil
}

// runDeclarativePreUpgradeHook executes the declarative pre-upgrade lifecycle
// hook configured under integration.flows.<flow>.pre-upgrade in the target
// app version's ci-test-config.yaml, between Step 1 and Step 2 of a two-step
// upgrade flow. Returns (handled=true) when the declarative path executed (or
// errored), so the caller can skip the legacy filename-derived discovery.
func runDeclarativePreUpgradeHook(ctx context.Context, repoRoot, appVersion, flow string, flags *config.RuntimeFlags) (bool, error) {
	chartDir := filepath.Join(repoRoot, "charts", "camunda-platform-"+appVersion)
	cfg, err := LoadCITestConfig(chartDir)
	if err != nil {
		// Missing or unparseable config — fall back to legacy discovery.
		return false, nil
	}
	if cfg.Integration.Flows == nil {
		return false, nil
	}
	flowHooks := cfg.Integration.Flows[flow]
	if flowHooks == nil || flowHooks.PreUpgrade == nil {
		return false, nil
	}
	hook := flowHooks.PreUpgrade

	if (len(hook.Fixtures) > 0) == (hook.Script != "") {
		return true, fmt.Errorf("flow %q v%s: pre-upgrade must specify exactly one of fixtures or script", flow, appVersion)
	}

	namespace := flags.EffectiveNamespace()

	switch {
	case len(hook.Fixtures) > 0:
		release := flags.Deployment.Release
		scenarioCtx := &deploy.ScenarioContext{
			ScenarioName: "flow:" + flow,
			Namespace:    namespace,
			Release:      release,
		}
		vars := lifecycleVars(namespace, release)
		logging.Logger.Info().
			Str("flow", flow).
			Str("appVersion", appVersion).
			Strs("fixtures", hook.Fixtures).
			Str("namespace", namespace).
			Msg("Applying lifecycle fixtures (pre-upgrade, declarative)")
		if err := deploy.ApplyLifecycleManifests(ctx, scenarioCtx, chartDir, flags.Test.KubeContext, hook.Fixtures, vars); err != nil {
			return true, fmt.Errorf("pre-upgrade fixtures for flow %q failed: %w", flow, err)
		}
	case hook.Script != "":
		scriptPath := versionmatrix.PreSetupScriptPath(repoRoot, appVersion, hook.Script)
		if !versionmatrix.HasPreSetupScript(repoRoot, appVersion, hook.Script) {
			return true, fmt.Errorf("flow %q v%s: pre-upgrade script %q not found at %s",
				flow, appVersion, hook.Script, scriptPath)
		}
		logging.Logger.Info().
			Str("script", scriptPath).
			Str("flow", flow).
			Str("appVersion", appVersion).
			Str("namespace", namespace).
			Msg("Running pre-upgrade script (declarative)")
		scriptEnv := []string{"TEST_NAMESPACE=" + namespace}
		if flags.Test.KubeContext != "" {
			scriptEnv = append(scriptEnv, "KUBE_CONTEXT="+flags.Test.KubeContext)
		}
		for _, k := range lifecycleVarPassthrough {
			if v := os.Getenv(k); v != "" {
				scriptEnv = append(scriptEnv, k+"="+v)
			}
		}
		if err := executil.RunCommand(ctx, "bash", []string{"-x", scriptPath}, scriptEnv, ""); err != nil {
			return true, fmt.Errorf("pre-upgrade script %s failed: %w", scriptPath, err)
		}
		logging.Logger.Info().
			Str("script", scriptPath).
			Msg("Pre-upgrade script completed successfully")
	}
	return true, nil
}
