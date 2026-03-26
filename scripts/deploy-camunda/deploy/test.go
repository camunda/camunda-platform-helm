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

	if !runIT && !runE2E {
		return nil
	}

	if flags.OnPhase != nil {
		flags.OnPhase("testing")
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
			output, err := runIntegrationTests(testCtx, repoRoot, chartPath, namespace, flags.Deployment.Platform, flags.Test.KubeContext, flags.Test.TestExclude, flags.Auth.Auth, flags.ITOutputWriter)
			resultCh <- TestResult{Type: "integration", Error: err, Output: output}
		}()
	}

	if runE2E {
		wg.Add(1)
		go func() {
			defer wg.Done()
			output, err := runE2ETests(testCtx, repoRoot, chartPath, namespace, flags.Test.KubeContext, flags.Test.TestExclude, flags.E2EOutputWriter)
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
func runIntegrationTests(ctx context.Context, repoRoot, chartPath, namespace, platform, kubeContext, testExclude, testAuthType string, outputSink io.Writer) (string, error) {
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

	return executeScript(ctx, scriptPath, args, "integration", outputSink)
}

// runE2ETests executes the e2e test script.
func runE2ETests(ctx context.Context, repoRoot, chartPath, namespace, kubeContext, testExclude string, outputSink io.Writer) (string, error) {
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

	return executeScript(ctx, scriptPath, args, "e2e", outputSink)
}

// executeScript runs a shell script with the given arguments and returns the
// captured combined output alongside any error.
//
// Output is tee'd: it streams to the provided outputSink (or os.Stdout/os.Stderr
// when nil) in real time and is simultaneously captured into a buffer. The buffer
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
func executeScript(ctx context.Context, scriptPath string, args []string, testType string, outputSink io.Writer) (string, error) {
	var buf bytes.Buffer

	stdoutW := io.Writer(os.Stdout)
	stderrW := io.Writer(os.Stderr)
	if outputSink != nil {
		stdoutW = outputSink
		stderrW = outputSink
	}

	cmd := exec.CommandContext(ctx, scriptPath, args...)
	cmd.Stdout = io.MultiWriter(stdoutW, &buf)
	cmd.Stderr = io.MultiWriter(stderrW, &buf)
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
