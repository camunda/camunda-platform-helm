package deploy

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"scripts/camunda-core/pkg/logging"
	"scripts/deploy-camunda/config"
	"strings"
	"sync"
	"syscall"
	"time"
)

// TestError wraps a test failure with the captured output from the test scripts.
// Callers can use errors.As to extract the output for diagnostics.
type TestError struct {
	Err    error
	Output string // Combined stdout+stderr from the test scripts.
}

func (e *TestError) Error() string {
	return e.Err.Error()
}

func (e *TestError) Unwrap() error {
	return e.Err
}

// TestResult holds the result of a test execution.
type TestResult struct {
	Type   string // "integration" or "e2e"
	Error  error
	Output string // Captured stdout+stderr from the test script.
}

// RunTests executes tests after deployment based on flags.
// Tests are run in parallel if both --test-it and --test-e2e (or --test-all) are specified.
//
// On failure, the returned error is a *TestError containing the captured output
// from the test scripts. Callers can use errors.As to extract it.
func RunTests(ctx context.Context, flags *config.RuntimeFlags, namespace string) error {
	runIT := flags.Test.RunIntegrationTests || flags.Test.RunAllTests
	runE2E := flags.Test.RunE2ETests || flags.Test.RunAllTests

	// Skip E2E smoke tests for OIDC auth scenarios.  The upstream
	// @camunda/e2e-test-suite SM-8.8 smoke-tests.spec.js has a hard
	// dependency on Keycloak admin (setupKeycloakUser → goToKeycloak →
	// locator('#keycloak-bg')), which does not exist when using an external
	// OIDC provider like Microsoft Entra ID.  SM-8.9 already skips its own
	// smoke tests internally (test.skip(true, ...)), so this has no impact
	// on 8.9 either.  Remove this guard once the upstream test suite ships
	// OIDC-compatible smoke tests.
	if runE2E && strings.EqualFold(flags.Auth.Auth, "oidc") {
		logging.Logger.Info().
			Str("auth", flags.Auth.Auth).
			Msg("Skipping E2E tests: OIDC auth is incompatible with Keycloak-dependent smoke tests")
		runE2E = false
	}

	// Skip E2E smoke tests for chart version 8.7.  The web-modeler-restapi
	// Docker image in 8.7 has a known bug where the restapi becomes
	// unresponsive ~40 seconds after the first requests (the
	// /internal-api/organizations/.../projects endpoint hangs indefinitely).
	// The webapp's internal proxy times out at ~40s and returns 504 Gateway
	// Timeout.  This causes the Playwright smoke tests to fail on the
	// "New project" button visibility check because the Modeler page never
	// loads past the skeleton state.  The issue was fixed in the 8.8
	// application images — the identical Helm chart configuration works fine
	// on 8.8+.  Integration tests still run and validate the deployment.
	// Remove this guard once 8.7 reaches end-of-life or the restapi image
	// is patched.
	if runE2E && isChartVersion(flags.Chart.ChartPath, "8.7") {
		logging.Logger.Info().
			Str("chartPath", flags.Chart.ChartPath).
			Msg("Skipping E2E tests: chart 8.7 web-modeler-restapi has a known unresponsiveness bug")
		runE2E = false
	}

	// Skip E2E smoke tests for two specific 8.8 scenarios that have known
	// application-level bugs in the 8.8 Docker images.  Both scenarios pass
	// on 8.9 with identical Helm chart configuration, confirming these are
	// NOT chart issues.
	//
	// 1. keycloak-mt (kemt): The Playwright smoke test "Most Common Flow
	//    User Flow With All Apps" enters a retry loop on
	//    ModelerCreatePage.runProcessInstance. Each retry takes up to the
	//    full 600s test timeout.  Three retries exceed the 30-minute
	//    post-deployment test timeout, causing context deadline exceeded.
	//
	// 2. elasticsearch-arm (esarm): The "Deploy and Run" API call
	//    (POST /internal-api/files/{id}/execute) never reaches the restapi
	//    backend on ARM nodes.  The deploy dialog opens but the confirmation
	//    button click does not trigger the HTTP request — a frontend/UI
	//    interaction bug specific to 8.8 images on ARM.
	//
	// Integration tests pass for both entries.  Remove these guards once the
	// 8.8 application images are patched or 8.8 reaches end-of-life.
	if runE2E && isChartVersion(flags.Chart.ChartPath, "8.8") {
		scenario := flags.Deployment.Scenario
		if scenario == "keycloak-mt" || scenario == "elasticsearch-arm" {
			logging.Logger.Info().
				Str("chartPath", flags.Chart.ChartPath).
				Str("scenario", scenario).
				Msg("Skipping E2E tests: known 8.8 application image bug for this scenario")
			runE2E = false
		}
	}

	if !runIT && !runE2E {
		return nil
	}

	logging.Logger.Info().
		Bool("integrationTests", runIT).
		Bool("e2eTests", runE2E).
		Str("namespace", namespace).
		Msg("Starting post-deployment tests")

	// Bound total post-deployment test runtime so matrix entries cannot hang
	// indefinitely after Helm has already completed.
	// Keep this well above Helm timeout because integration tests (DNS + ingress
	// readiness + Playwright retries) can legitimately run much longer on
	// upgrade-minor flows for 8.9.
	testTimeout := 30 * time.Minute
	if flags.Deployment.Timeout > 0 {
		helmTimeout := time.Duration(flags.Deployment.Timeout) * time.Minute
		candidate := 4 * helmTimeout
		if candidate > testTimeout {
			testTimeout = candidate
		}
	}
	testCtx, cancel := context.WithTimeout(ctx, testTimeout)
	defer cancel()

	logging.Logger.Info().
		Dur("timeout", testTimeout).
		Str("namespace", namespace).
		Msg("Post-deployment tests timeout configured")

	// Resolve paths
	repoRoot := flags.Chart.RepoRoot
	if repoRoot == "" {
		// Try to determine repo root from chart path
		repoRoot = findRepoRoot(flags.Chart.ChartPath)
	}

	if repoRoot == "" {
		return fmt.Errorf("unable to determine repository root; set --repo-root flag")
	}

	chartPath, err := filepath.Abs(flags.Chart.ChartPath)
	if err != nil {
		return fmt.Errorf("failed to resolve chart path: %w", err)
	}

	// Run tests in parallel
	var wg sync.WaitGroup
	resultCh := make(chan TestResult, 2)

	if runIT {
		wg.Add(1)
		go func() {
			defer wg.Done()
			output, err := runIntegrationTests(testCtx, repoRoot, chartPath, namespace, flags.Deployment.Platform, flags.Test.KubeContext, flags.Test.TestExclude, flags.Auth.Auth)
			resultCh <- TestResult{Type: "integration", Error: err, Output: output}
		}()
	}

	if runE2E {
		wg.Add(1)
		go func() {
			defer wg.Done()
			output, err := runE2ETests(testCtx, repoRoot, chartPath, namespace, flags.Test.KubeContext, flags.Test.TestExclude)
			resultCh <- TestResult{Type: "e2e", Error: err, Output: output}
		}()
	}

	// Wait for all tests to complete
	wg.Wait()
	close(resultCh)

	// Collect results
	var errors []string
	var allOutput strings.Builder
	for result := range resultCh {
		if result.Error != nil {
			logging.Logger.Error().
				Str("testType", result.Type).
				Err(result.Error).
				Msg("Test execution failed")
			errors = append(errors, fmt.Sprintf("%s tests: %v", result.Type, result.Error))
		} else {
			logging.Logger.Info().
				Str("testType", result.Type).
				Msg("Test execution completed successfully")
		}
		if result.Output != "" {
			fmt.Fprintf(&allOutput, "=== %s test output ===\n%s\n\n", result.Type, result.Output)
		}
	}

	if len(errors) > 0 {
		return &TestError{
			Err:    fmt.Errorf("test failures:\n  - %s", strings.Join(errors, "\n  - ")),
			Output: allOutput.String(),
		}
	}

	logging.Logger.Info().Msg("All post-deployment tests passed")
	return nil
}

// runIntegrationTests executes the integration test script.
func runIntegrationTests(ctx context.Context, repoRoot, chartPath, namespace, platform, kubeContext, testExclude, testAuthType string) (string, error) {
	scriptPath := filepath.Join(repoRoot, "scripts", "run-integration-tests.sh")

	if _, err := os.Stat(scriptPath); err != nil {
		return "", fmt.Errorf("integration test script not found at %s: %w", scriptPath, err)
	}

	logging.Logger.Info().
		Str("script", scriptPath).
		Str("chartPath", chartPath).
		Str("namespace", namespace).
		Str("platform", platform).
		Str("kubeContext", kubeContext).
		Msg("Running integration tests")

	args := []string{
		"--absolute-chart-path", chartPath,
		"--namespace", namespace,
		"--platform", platform,
	}

	if kubeContext != "" {
		args = append(args, "--kube-context", kubeContext)
	}
	if testExclude != "" {
		args = append(args, "--test-exclude", testExclude)
	}
	if testAuthType != "" {
		args = append(args, "--test-auth-type", testAuthType)
	}

	return executeScript(ctx, scriptPath, args, "integration")
}

// runE2ETests executes the e2e test script.
func runE2ETests(ctx context.Context, repoRoot, chartPath, namespace, kubeContext, testExclude string) (string, error) {
	scriptPath := filepath.Join(repoRoot, "scripts", "run-e2e-tests.sh")

	if _, err := os.Stat(scriptPath); err != nil {
		return "", fmt.Errorf("e2e test script not found at %s: %w", scriptPath, err)
	}

	logging.Logger.Info().
		Str("script", scriptPath).
		Str("chartPath", chartPath).
		Str("namespace", namespace).
		Str("kubeContext", kubeContext).
		Msg("Running e2e tests")

	args := []string{
		"--absolute-chart-path", chartPath,
		"--namespace", namespace,
		"--run-smoke-tests",
	}

	if kubeContext != "" {
		args = append(args, "--kube-context", kubeContext)
	}
	if testExclude != "" {
		args = append(args, "--test-exclude", testExclude)
	}

	return executeScript(ctx, scriptPath, args, "e2e")
}

// executeScript runs a shell script with the given arguments and returns the
// captured combined output alongside any error.
//
// Output is tee'd: it streams to os.Stdout/os.Stderr in real time (so the user
// sees live progress) and is simultaneously captured into a buffer. The buffer
// contents are returned so callers can include them in diagnostics on failure.
//
// The subprocess is placed in its own process group (Setpgid) so that when
// the context is cancelled (e.g. StopOnFailure, Ctrl+C) we can send SIGTERM
// to the entire process tree — shell, node, playwright browsers, etc. —
// instead of only killing the direct child and leaving orphans behind.
//
// Without this, exec.CommandContext sends os.Kill (SIGKILL) only to the
// direct child PID, and any grandchild processes (npx, playwright, tee, etc.)
// continue running until they finish or the terminal is closed.
func executeScript(ctx context.Context, scriptPath string, args []string, testType string) (string, error) {
	var buf bytes.Buffer

	cmd := exec.CommandContext(ctx, scriptPath, args...)
	cmd.Stdout = io.MultiWriter(os.Stdout, &buf)
	cmd.Stderr = io.MultiWriter(os.Stderr, &buf)
	cmd.Env = os.Environ()
	// If context cancellation does not terminate children promptly, force-kill
	// after a short grace period to prevent hung matrix entries.
	cmd.WaitDelay = 15 * time.Second

	// Place the child in its own process group so we can signal the whole tree.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	// Override the default CommandContext kill behavior: instead of sending
	// SIGKILL to just the child PID, send SIGTERM to the entire process group
	// (negative PID). This gives the shell and its children a chance to run
	// cleanup traps before exiting.
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		pgid := cmd.Process.Pid
		logging.Logger.Info().
			Int("pgid", pgid).
			Str("testType", testType).
			Msg("Context cancelled, sending SIGTERM to process group")

		// Escalate to SIGKILL after a grace period if the process group is still alive.
		// Use a detached timer (not tied to ctx.Done) because ctx is already cancelled here.
		go func() {
			time.Sleep(10 * time.Second)
			if cmd.ProcessState == nil || !cmd.ProcessState.Exited() {
				// Signal 0 checks whether the process group still exists.
				if err := syscall.Kill(-pgid, 0); err == nil {
					logging.Logger.Warn().
						Int("pgid", pgid).
						Str("testType", testType).
						Msg("Test process group still alive after SIGTERM grace period, sending SIGKILL")
					_ = syscall.Kill(-pgid, syscall.SIGKILL)
				}
			}
		}()
		// Negative PID signals the entire process group.
		return syscall.Kill(-pgid, syscall.SIGTERM)
	}

	logging.Logger.Debug().
		Str("command", scriptPath).
		Strs("args", args).
		Str("testType", testType).
		Msg("Executing test script")

	if err := cmd.Run(); err != nil {
		output := buf.String()
		if ctx.Err() != nil {
			return output, fmt.Errorf("%s tests cancelled: %w", testType, ctx.Err())
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			return output, fmt.Errorf("%s tests failed with exit code %d", testType, exitErr.ExitCode())
		}
		return output, fmt.Errorf("failed to execute %s tests: %w", testType, err)
	}

	return buf.String(), nil
}

// findRepoRoot attempts to find the repository root from a chart path.
// It looks for typical markers like .git directory or go.mod file.
func findRepoRoot(chartPath string) string {
	if chartPath == "" {
		return ""
	}

	// Walk up the directory tree looking for repo markers
	dir := chartPath
	for {
		// Check for .git directory
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}

		// Check for scripts directory (specific to this repo)
		if _, err := os.Stat(filepath.Join(dir, "scripts", "run-integration-tests.sh")); err == nil {
			return dir
		}

		// Move up one directory
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root
			break
		}
		dir = parent
	}

	return ""
}

// isChartVersion returns true if chartPath refers to the given version.
// It matches the final directory component against "camunda-platform-<version>".
// Example: isChartVersion("charts/camunda-platform-8.7", "8.7") returns true.
func isChartVersion(chartPath, version string) bool {
	if chartPath == "" || version == "" {
		return false
	}
	base := filepath.Base(chartPath)
	return strings.HasSuffix(base, "-"+version)
}
