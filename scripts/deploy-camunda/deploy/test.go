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
	"strconv"
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
	Type   string // "e2e"
	Error  error
	Output string // Captured stdout+stderr from the test script.
}

// RunTests executes tests after deployment based on flags.
//
// On failure, the returned error is a *TestError containing the captured output
// from the test scripts. Callers can use errors.As to extract it.
func RunTests(ctx context.Context, flags *config.RuntimeFlags, namespace string) error {
	runE2E := flags.Test.RunE2ETests || flags.Test.RunAllTests

	if !runE2E {
		return nil
	}

	if flags.OnPhase != nil {
		flags.OnPhase("testing")
	}

	logging.Logger.Info().
		Bool("e2eTests", runE2E).
		Str("namespace", namespace).
		Msg("Starting post-deployment tests")

	// Bound total post-deployment test runtime so matrix entries cannot hang
	// indefinitely after Helm has already completed.
	// Keep this well above Helm timeout because e2e tests (DNS + ingress
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

	if runE2E {
		wg.Add(1)
		go func() {
			defer wg.Done()
			output, err := runE2ETests(testCtx, repoRoot, chartPath, namespace, flags.Test.KubeContext, flags.Test.TestExclude, flags.Selection.Persistence, topologyTarget{
				HubNamespace:        flags.Test.HubNamespace,
				OptimizeNamespace:   flags.Test.OptimizeNamespace,
				OptimizeContextPath: flags.Test.OptimizeContextPath,
				ModelerClusterName:  flags.Test.ModelerClusterName,
			}, flags.E2EOutputWriter)
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

// topologyTarget points the e2e env at the other namespaces of a multi-namespace topology. Zero value
// means a single-release deployment, where every app shares the namespace under test.
type topologyTarget struct {
	// HubNamespace runs the central Identity, Keycloak and Web Modeler. Setting it is what makes
	// run-e2e-tests.sh merge the Hub's absolute URLs into the env instead of deriving every app from
	// the orchestration host.
	HubNamespace string
	// OptimizeNamespace and OptimizeContextPath locate an Optimize that runs as its own release.
	OptimizeNamespace   string
	OptimizeContextPath string
	// ModelerClusterName selects this leg's cluster in the Hub's Web Modeler deploy dialog.
	ModelerClusterName string
}

func (o topologyTarget) isSet() bool {
	return o.HubNamespace != "" || o.OptimizeNamespace != "" || o.OptimizeContextPath != "" ||
		o.ModelerClusterName != ""
}

func runE2ETests(ctx context.Context, repoRoot, chartPath, namespace, kubeContext, testExclude, persistence string, topology topologyTarget, outputSink io.Writer) (string, error) {
	scriptPath := filepath.Join(repoRoot, "scripts", "run-e2e-tests.sh")

	if _, err := os.Stat(scriptPath); err != nil {
		return "", fmt.Errorf("e2e test script not found at %s: %w", scriptPath, err)
	}

	event := logging.Logger.Info().
		Str("script", scriptPath).
		Str("chartPath", chartPath).
		Str("namespace", namespace).
		Str("kubeContext", kubeContext).
		Str("persistence", persistence)
	if topology.isSet() {
		event = event.
			Str("hubNamespace", topology.HubNamespace).
			Str("optimizeNamespace", topology.OptimizeNamespace).
			Str("optimizeContextPath", topology.OptimizeContextPath).
			Str("modelerClusterName", topology.ModelerClusterName)
	}
	event.Msg("Running e2e tests")

	args := e2eScriptArgs(chartPath, namespace, kubeContext, testExclude, persistence, topology)

	return executeScript(ctx, scriptPath, args, "e2e", outputSink)
}

// e2eScriptArgs builds the run-e2e-tests.sh argument list. Extracted so the topology arguments are
// unit-testable: omitting --hub-namespace silently rendered the e2e env from the orchestration
// namespace alone, which surfaced as a Web Modeler timeout rather than as a missing argument.
func e2eScriptArgs(chartPath, namespace, kubeContext, testExclude, persistence string, topology topologyTarget) []string {
	args := []string{
		"--absolute-chart-path", chartPath,
		"--namespace", namespace,
	}

	// For chart versions < 8.10, run only the smoke-tests project.
	// 8.10+ runs the full-suite project (the script's default) which
	// exercises the full E2E suite with proper exclusions configured
	// in the chart's playwright.config.ts.
	if !isFullSuiteChart(chartPath) {
		args = append(args, "--run-smoke-tests")
	}

	if kubeContext != "" {
		args = append(args, "--kube-context", kubeContext)
	}
	if testExclude != "" {
		args = append(args, "--test-exclude", testExclude)
	}
	if strings.Contains(persistence, "opensearch") {
		args = append(args, "--opensearch")
	}
	if topology.HubNamespace != "" {
		args = append(args, "--hub-namespace", topology.HubNamespace)
	}
	if topology.OptimizeNamespace != "" {
		args = append(args, "--optimize-namespace", topology.OptimizeNamespace)
	}
	if topology.OptimizeContextPath != "" {
		args = append(args, "--optimize-context-path", topology.OptimizeContextPath)
	}
	if topology.ModelerClusterName != "" {
		args = append(args, "--modeler-cluster-name", topology.ModelerClusterName)
	}
	return args
}

// isFullSuiteChart returns true if the chart path indicates version 8.10 or
// later, which should run the full E2E suite instead of just smoke tests.
// Chart directories follow the naming pattern "camunda-platform-8.<minor>".
func isFullSuiteChart(chartPath string) bool {
	base := filepath.Base(chartPath)
	const prefix = "camunda-platform-8."
	idx := strings.Index(base, prefix)
	if idx < 0 {
		return false
	}
	minorStr := base[idx+len(prefix):]
	// Trim any non-digit suffix (e.g., "-alpha1")
	for i, c := range minorStr {
		if c < '0' || c > '9' {
			minorStr = minorStr[:i]
			break
		}
	}
	minor, err := strconv.Atoi(minorStr)
	if err != nil {
		return false
	}
	return minor >= 10
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
		if _, err := os.Stat(filepath.Join(dir, "scripts", "run-e2e-tests.sh")); err == nil {
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
