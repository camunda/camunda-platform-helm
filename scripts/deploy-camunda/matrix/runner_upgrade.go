package matrix

import (
	"context"
	"fmt"
	"path/filepath"

	"scripts/camunda-core/pkg/helm"
	"scripts/camunda-core/pkg/kube"
	"scripts/camunda-core/pkg/logging"
	"scripts/camunda-core/pkg/versionmatrix"
	"scripts/deploy-camunda/config"
	"scripts/deploy-camunda/deploy"
)

// bitnamiPGPasswordMapping maps Kubernetes Secret keys (from the "integration-test-credentials"
// secret) to Helm value paths that satisfy Bitnami PostgreSQL's password validation during upgrades.
//
// Bitnami's common.secrets.passwords.manage function does a `lookup` of the existing Secret. If the
// lookup returns nil during a render (transient mid-upgrade state), it triggers a fail() unless an
// explicit password is provided via `providedValues`. By extracting passwords from the cluster
// secret and passing them as --set overrides, we satisfy the `honorProvidedValues` check and bypass
// the lookup/fail path entirely.
var bitnamiPGPasswordMapping = map[string][]string{
	"identity-keycloak-postgresql-user-password":  {"identityKeycloak.postgresql.auth.password", "identityPostgresql.auth.password"},
	"identity-keycloak-postgresql-admin-password": {"identityKeycloak.postgresql.auth.postgresPassword", "identityPostgresql.auth.postgresPassword"},
	"webmodeler-postgresql-user-password":         {"webModelerPostgresql.auth.password"},
	"webmodeler-postgresql-admin-password":        {"webModelerPostgresql.auth.postgresPassword"},
}

func shouldExtractBitnamiPGPasswords(targetVersion string) bool {
	return compareVersions(targetVersion, "8.10") < 0
}

// extractBitnamiPGPasswords reads the "integration-test-credentials" Kubernetes Secret from the
// given namespace and returns a map of Helm --set key=value pairs that provide Bitnami PostgreSQL
// passwords explicitly. This prevents the PASSWORDS ERROR that occurs when Bitnami's template
// lookup returns nil for a temporarily-absent Secret during an upgrade render.
//
// The function is intentionally lenient: if the secret doesn't exist, or individual keys are missing,
// it logs warnings and returns whatever it could extract. Callers should merge the result into
// ExtraHelmSets.
func extractBitnamiPGPasswords(ctx context.Context, namespace, kubeContext string) map[string]string {
	const secretName = "integration-test-credentials"

	kubeClient, err := kube.NewClient("", kubeContext)
	if err != nil {
		logging.Logger.Warn().Err(err).
			Str("namespace", namespace).
			Msg("Failed to create kube client for Bitnami PG password extraction; upgrade may fail with PASSWORDS ERROR")
		return nil
	}

	secretData, err := kubeClient.GetSecretData(ctx, namespace, secretName)
	if err != nil {
		logging.Logger.Warn().Err(err).
			Str("namespace", namespace).
			Str("secret", secretName).
			Msg("Failed to read secret for Bitnami PG password extraction; upgrade may fail with PASSWORDS ERROR")
		return nil
	}
	if secretData == nil {
		logging.Logger.Warn().
			Str("namespace", namespace).
			Str("secret", secretName).
			Msg("Secret not found for Bitnami PG password extraction; upgrade may fail with PASSWORDS ERROR")
		return nil
	}

	helmSets := make(map[string]string)
	for secretKey, helmPaths := range bitnamiPGPasswordMapping {
		value, ok := secretData[secretKey]
		if !ok || value == "" {
			logging.Logger.Warn().
				Str("namespace", namespace).
				Str("secret", secretName).
				Str("key", secretKey).
				Msg("Secret key missing or empty; corresponding Bitnami PG password override skipped")
			continue
		}
		for _, helmPath := range helmPaths {
			helmSets[helmPath] = value
		}
	}

	if len(helmSets) > 0 {
		logging.Logger.Info().
			Str("namespace", namespace).
			Int("overrides", len(helmSets)).
			Msg("Extracted Bitnami PG passwords from cluster secret for upgrade --set overrides")
	}

	return helmSets
}

// executeTwoStepUpgrade performs a two-step upgrade deployment:
//
//	Step 1: Install the previously released chart version from the Helm repository.
//	Step 2: Upgrade to the current on-disk chart (the branch version) with
//	        upgrade-specific flags (allowPreReleaseImages).
//
// Values file resolution for Step 1 depends on the flow:
//   - upgrade-patch: uses the CURRENT chart's values files (same app version, older release).
//   - upgrade-minor: uses the PREVIOUS app version's chart values files (e.g., 8.7 for an 8.8 entry).
//     This matches CI behavior where test-type-vars sets CHART_PATH to the previous version's
//     chart directory for upgrade-minor.
//
// The "from" chart version is resolved from version-matrix JSON files:
//   - upgrade-patch: latest stable chart for the SAME app version
//   - upgrade-minor: latest stable chart for the PREVIOUS app version
func executeTwoStepUpgrade(ctx context.Context, entry Entry, flags *config.RuntimeFlags, opts RunOptions) error {
	// Resolve the "from" chart version for the upgrade.
	// If UpgradeFromVersion is set via CLI flag, use it directly; otherwise auto-resolve.
	var fromVersion string
	if opts.UpgradeFromVersion != "" {
		fromVersion = opts.UpgradeFromVersion
		logging.Logger.Info().
			Str("flow", entry.Flow).
			Str("fromVersion", fromVersion).
			Str("source", "cli-override").
			Msg("Two-step upgrade: using CLI-provided from-version")
	} else {
		var err error
		fromVersion, err = versionmatrix.ResolveUpgradeFromVersion(opts.RepoRoot, entry.Version, entry.Flow)
		if err != nil {
			return fmt.Errorf("resolve upgrade-from version for %s/%s: %w", entry.Version, entry.Flow, err)
		}
	}

	logging.Logger.Info().
		Str("flow", entry.Flow).
		Str("fromVersion", fromVersion).
		Str("toChart", entry.ChartPath).
		Str("version", entry.Version).
		Msg("Two-step upgrade: resolved from-version")

	// Pin index prefixes and Keycloak realm so that Step 1 and Step 2 share the
	// same values. Without this, each call to deploy.Execute() generates a new
	// random suffix, causing the upgraded components to look for indices/realm
	// that don't match what Step 1 created.
	if err := deploy.PinScenarioPrefixes(entry.Scenario, flags); err != nil {
		return fmt.Errorf("pin scenario prefixes for upgrade: %w", err)
	}
	logging.Logger.Info().
		Str("realm", flags.Auth.KeycloakRealm).
		Str("orchPrefix", flags.Index.OrchestrationIndexPrefix).
		Str("optPrefix", flags.Index.OptimizeIndexPrefix).
		Msg("Two-step upgrade: pinned index prefixes and realm for both steps")

	// --- Step 1: Install old version from Helm repo ---
	if flags.OnPhase != nil {
		flags.OnPhase("step-1")
	}
	logging.Logger.Info().
		Str("step", "1/2").
		Str("action", "install").
		Str("chart", versionmatrix.DefaultHelmChartRef).
		Str("version", fromVersion).
		Msg("Step 1: Installing previous chart version from Helm repo")

	// Ensure the Camunda Helm repo is registered and up-to-date.
	if err := helm.RepoAdd(ctx, versionmatrix.DefaultHelmRepoName, versionmatrix.DefaultHelmRepoURL); err != nil {
		return fmt.Errorf("step 1: helm repo add: %w", err)
	}
	if err := helm.RepoUpdate(ctx); err != nil {
		return fmt.Errorf("step 1: helm repo update: %w", err)
	}

	// Clone flags for Step 1: deploy from repo instead of local chart path.
	// Detach hook slices: a plain `*flags` copy shares the backing arrays with
	// the parent, so a subsequent append (when cap > len) would mutate flags
	// and leak Step 1-only hooks into Step 2's later shallow copy.
	step1Flags := *flags
	step1Flags.PreInstallHooks = append([]func(context.Context) error(nil), flags.PreInstallHooks...)
	step1Flags.PostDeployHooks = append([]func(context.Context) error(nil), flags.PostDeployHooks...)
	step1Flags.Chart.Chart = versionmatrix.DefaultHelmChartRef
	step1Flags.Chart.ChartVersion = fromVersion
	step1Flags.Chart.ChartPath = "" // Use repo chart, not local path.
	step1Flags.Deployment.Flow = "install"
	step1Flags.Selection.UpgradeFlow = false     // Step 1 is a fresh install, no base-upgrade.yaml.
	step1Flags.Chart.ChartRootOverlays = nil     // Step 1 installs old version from repo — no chart-root overlays.
	step1Flags.Chart.SkipDependencyUpdate = true // Repo charts don't need local dep update.
	// Step 1 installs the previously released chart; caller --extra-values
	// (e.g. per-PR image tag) belongs to Step 2 only.
	step1Flags.Deployment.ExtraValues = nil
	step1Flags.Test.RunIntegrationTests = false // Don't run tests after Step 1.
	step1Flags.Test.RunE2ETests = false
	step1Flags.Test.RunAllTests = false
	step1Flags.Deployment.DeleteNamespaceFirst = flags.Deployment.DeleteNamespaceFirst // Only delete on Step 1.

	// For upgrade-minor, Step 1 uses the PREVIOUS app version's values files.
	// In CI, test-type-vars sets CHART_PATH to charts/camunda-platform-<previous> for
	// the install step of upgrade-minor, so values files come from the older chart.
	// For upgrade-patch, Step 1 uses the current chart's values (same app version).
	step1AppVersion := entry.Version
	if entry.Flow == "upgrade-minor" {
		prevVersion, err := versionmatrix.PreviousAppVersion(entry.Version)
		if err != nil {
			return fmt.Errorf("step 1: resolve previous app version for %s: %w", entry.Version, err)
		}
		step1AppVersion = prevVersion
		prevChartDir := filepath.Join(opts.RepoRoot, "charts", "camunda-platform-"+prevVersion)
		prevScenarioDir := filepath.Join(prevChartDir, "test/integration/scenarios/chart-full-setup")
		step1Flags.Deployment.ScenarioPath = prevScenarioDir

		logging.Logger.Info().
			Str("flow", entry.Flow).
			Str("previousVersion", prevVersion).
			Str("scenarioDir", prevScenarioDir).
			Msg("Step 1: using previous app version's values files (matching CI behavior)")
	}

	// --- Pre-install lifecycle hook (Step 1 of two-step upgrade) ---
	// Hook is registered against step1Flags so it fires before the Step 1 helm install.
	// The app version being installed in Step 1 scopes script/fixture lookup
	// (previous version for upgrade-minor, current for upgrade-patch).
	// Append (do not nil-then-append): upstream hooks like the OIDC venom-secret
	// hook were registered against flags before the *flags shallow copy and must
	// fire in Step 1 too (helm install needs the secret already in the namespace).
	if err := registerDeclarativePreInstallHook(&step1Flags, entry.PreInstall, opts.RepoRoot, step1AppVersion, entry.Scenario); err != nil {
		return err
	}

	if err := deploy.Execute(ctx, &step1Flags); err != nil {
		return fmt.Errorf("step 1: install %s@%s failed: %w", versionmatrix.DefaultHelmChartRef, fromVersion, err)
	}

	logging.Logger.Info().
		Str("step", "1/2").
		Str("version", fromVersion).
		Msg("Step 1 complete: previous version installed successfully")

	// --- Pre-upgrade lifecycle hook ---
	// Runs the declarative pre-upgrade hook (integration.flows.<flow>.pre-upgrade)
	// resolved at matrix-generation time onto entry.PreUpgrade. Scoped to the
	// target version (entry.Version is the version being upgraded to).
	if err := runDeclarativePreUpgradeHook(ctx, flags, entry.PreUpgrade, opts.RepoRoot, entry.Version, entry.Flow); err != nil {
		return err
	}

	// --- Step 2: Upgrade to current on-disk chart (or external chart-ref when set) ---
	if flags.OnPhase != nil {
		flags.OnPhase("step-2")
	}
	step2Log := logging.Logger.Info().
		Str("step", "2/2").
		Str("action", "upgrade")
	if opts.ChartRef != "" {
		step2Log.
			Str("chartRef", opts.ChartRef).
			Str("chartVersion", opts.ChartRefVersion).
			Msg("Step 2: Upgrading to chart from --chart-ref")
	} else {
		step2Log.
			Str("chartPath", entry.ChartPath).
			Msg("Step 2: Upgrading to current chart version")
	}

	// Clone flags for Step 2: upgrade from installed state to local chart.
	// Detach hook slices for the same reason as Step 1 — keep declarative
	// post-deploy registrations isolated to this step.
	step2Flags := *flags
	step2Flags.PreInstallHooks = append([]func(context.Context) error(nil), flags.PreInstallHooks...)
	step2Flags.PostDeployHooks = append([]func(context.Context) error(nil), flags.PostDeployHooks...)
	step2Flags.Selection.UpgradeFlow = true            // Ensure base-upgrade.yaml is included.
	step2Flags.Deployment.DeleteNamespaceFirst = false // Namespace already exists from Step 1.
	step2Flags.Deployment.Flow = "install"             // Must match Step 1's Flow so $FLOW in index prefixes resolves identically.
	step2Flags.Deployment.ExtraHelmSets = mergeHelmSets(
		flags.Deployment.ExtraHelmSets,
		map[string]string{"orchestration.upgrade.allowPreReleaseImages": "true"},
	)

	// Extract Bitnami PostgreSQL passwords from the cluster secret and merge into --set overrides.
	// Defensive: prevents the PASSWORDS ERROR that Bitnami's secrets.yaml can trigger when the
	// Secret lookup returns nil during upgrades (e.g. when a Secret is being patched mid-render).
	// Skip for 8.10+ which removed all Bitnami subcharts — constraints.tpl rejects these keys.
	if shouldExtractBitnamiPGPasswords(entry.Version) {
		if pgPasswords := extractBitnamiPGPasswords(ctx, flags.EffectiveNamespace(), flags.Test.KubeContext); len(pgPasswords) > 0 {
			for k, v := range pgPasswords {
				step2Flags.Deployment.ExtraHelmSets[k] = v
			}
		}
	}

	// --- Post-infra lifecycle hook (Step 2 of two-step upgrade) ---
	// Registered against step2Flags so it fires after Step 2's companion charts
	// are deployed but before the target chart upgrade.
	if err := registerDeclarativePostInfraHook(&step2Flags, entry.PostInfra, opts.RepoRoot, entry.Version, entry.Scenario); err != nil {
		return err
	}

	// --- Post-deploy lifecycle hook (Step 2 of two-step upgrade) ---
	// Registered against step2Flags so it fires after the upgrade succeeds.
	if err := registerDeclarativePostDeployHook(&step2Flags, entry.PostDeploy, opts.RepoRoot, entry.Version, entry.Scenario); err != nil {
		return err
	}
	step2Flags.Deployment.ExtraHelmArgs = flags.Deployment.ExtraHelmArgs

	if err := deploy.Execute(ctx, &step2Flags); err != nil {
		if opts.ChartRef != "" {
			target := opts.ChartRef
			if opts.ChartRefVersion != "" {
				target += "@" + opts.ChartRefVersion
			}
			return fmt.Errorf("step 2: upgrade to %s failed: %w", target, err)
		}
		return fmt.Errorf("step 2: upgrade to local chart failed: %w", err)
	}

	logging.Logger.Info().
		Str("step", "2/2").
		Msg("Step 2 complete: upgrade to current version succeeded")

	return nil
}

// executeUpgradeOnly performs a single-step upgrade against an already-running deployment.
// This is used for "modular-upgrade-minor" which, in CI, skips the install job entirely
// and only runs the upgrade job against the namespace of a prior "install" flow.
//
// The sequence is:
//  1. Run pre-upgrade script (if declared).
//  2. Run scenario pre-install hooks immediately before helm upgrade.
//  3. Helm upgrade to the current on-disk chart with upgrade-specific flags.
//
// Unlike executeTwoStepUpgrade, there is NO Step 1 install from the Helm repo.
// The previous version must already be deployed (by a prior "install" flow entry).
func executeUpgradeOnly(ctx context.Context, entry Entry, flags *config.RuntimeFlags, opts RunOptions) error {
	if flags.OnPhase != nil {
		flags.OnPhase("upgrading")
	}
	logging.Logger.Info().
		Str("flow", entry.Flow).
		Str("namespace", flags.EffectiveNamespace()).
		Str("chartPath", entry.ChartPath).
		Msg("Upgrade-only flow: upgrading existing deployment (no install step)")

	// --- Prefix consistency validation ---
	// Read the orchestration index prefix from the live Helm release and compare
	// against what this upgrade step will use. A mismatch means the install step
	// used a different scenario/prefix-key and the upgrade will produce incorrect
	// index names, causing data loss or auth failures after upgrade.
	expectedPrefix := deploy.ComputeExpectedOrchestrationPrefix(entry.Scenario, flags)
	if expectedPrefix != "" {
		installed, err := deploy.ReadInstalledPrefixes(ctx, flags.EffectiveNamespace(), flags.Deployment.Release, flags.Test.KubeContext)
		if err != nil {
			logging.Logger.Warn().Err(err).Msg("Failed to read installed prefixes; skipping prefix validation")
		} else if installed.OrchestrationIndexPrefix == "" {
			logging.Logger.Warn().
				Str("expectedPrefix", expectedPrefix).
				Msg("Prefix validation skipped: installed release has no orchestration prefix set (fresh install or ES-only scenario)")
		} else if installed.OrchestrationIndexPrefix != expectedPrefix {
			return fmt.Errorf(
				"prefix mismatch: installed release has orchestration prefix %q but this upgrade would use %q — "+
					"check that install and upgrade scenarios share the same prefix-key in ci-test-config.yaml",
				installed.OrchestrationIndexPrefix, expectedPrefix)
		} else {
			logging.Logger.Info().
				Str("installedPrefix", installed.OrchestrationIndexPrefix).
				Str("expectedPrefix", expectedPrefix).
				Msg("Prefix validation passed: installed and expected orchestration prefixes match")
		}
	}

	// --- Pre-upgrade lifecycle hook ---
	if err := runDeclarativePreUpgradeHook(ctx, flags, entry.PreUpgrade, opts.RepoRoot, entry.Version, entry.Flow); err != nil {
		return err
	}

	// --- Upgrade to current on-disk chart ---
	logging.Logger.Info().
		Str("action", "upgrade").
		Str("chartPath", entry.ChartPath).
		Msg("Upgrading to current chart version")

	upgradeFlags := *flags
	upgradeFlags.PreInstallHooks = append([]func(context.Context) error(nil), flags.PreInstallHooks...)
	upgradeFlags.PostDeployHooks = append([]func(context.Context) error(nil), flags.PostDeployHooks...)
	upgradeFlags.Selection.UpgradeFlow = true            // Ensure base-upgrade.yaml is included.
	upgradeFlags.Deployment.DeleteNamespaceFirst = false // Namespace must already exist from prior install.
	upgradeFlags.Deployment.Flow = "install"             // Must match the prior install's Flow so $FLOW in index prefixes resolves identically.
	upgradeFlags.Deployment.ExtraHelmSets = mergeHelmSets(
		flags.Deployment.ExtraHelmSets,
		map[string]string{"orchestration.upgrade.allowPreReleaseImages": "true"},
	)

	// Upgrade-only flows reuse an existing namespace, but scenario pre-install
	// fixtures may still be required before the target chart can start.
	if err := registerDeclarativePreInstallHook(&upgradeFlags, entry.PreInstall, opts.RepoRoot, entry.Version, entry.Scenario); err != nil {
		return err
	}
	// Post-infra hook: runs after the upgrade's companion charts are deployed
	// but before the target chart upgrade. This is where a Bitnami→companion
	// data migration runs, so the upgraded chart finds its data on the
	// companion services.
	if err := registerDeclarativePostInfraHook(&upgradeFlags, entry.PostInfra, opts.RepoRoot, entry.Version, entry.Scenario); err != nil {
		return err
	}

	// Extract Bitnami PostgreSQL passwords from the cluster secret and merge into --set overrides.
	// Defensive: prevents the PASSWORDS ERROR that Bitnami's secrets.yaml can trigger when the
	// Secret lookup returns nil during upgrades (e.g. when a Secret is being patched mid-render).
	// Skip for 8.10+ which removed all Bitnami subcharts — constraints.tpl rejects these keys.
	if shouldExtractBitnamiPGPasswords(entry.Version) {
		if pgPasswords := extractBitnamiPGPasswords(ctx, flags.EffectiveNamespace(), flags.Test.KubeContext); len(pgPasswords) > 0 {
			for k, v := range pgPasswords {
				upgradeFlags.Deployment.ExtraHelmSets[k] = v
			}
		}
	}

	if err := deploy.Execute(ctx, &upgradeFlags); err != nil {
		return fmt.Errorf("upgrade-only: upgrade to local chart failed: %w", err)
	}

	logging.Logger.Info().
		Str("flow", entry.Flow).
		Msg("Upgrade-only flow completed successfully")

	return nil
}
