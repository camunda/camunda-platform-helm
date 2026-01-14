package deploy

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"scripts/camunda-core/pkg/logging"
	"scripts/deploy-camunda/config"
	"strings"
	"sync"
)

// TestResult holds the result of a test execution.
type TestResult struct {
	Type  string // "integration" or "e2e"
	Error error
}

// RunTests executes tests after deployment based on flags.
// Tests are run in parallel if both --test-it and --test-e2e (or --test-all) are specified.
func RunTests(ctx context.Context, flags *config.RuntimeFlags, namespace string) error {
	runIT := flags.RunIntegrationTests || flags.RunAllTests
	runE2E := flags.RunE2ETests || flags.RunAllTests

	if !runIT && !runE2E {
		return nil
	}

	logging.Logger.Info().
		Bool("integrationTests", runIT).
		Bool("e2eTests", runE2E).
		Str("namespace", namespace).
		Msg("Starting post-deployment tests")

	// Resolve paths
	repoRoot := flags.RepoRoot
	if repoRoot == "" {
		// Try to determine repo root from chart path
		repoRoot = findRepoRoot(flags.ChartPath)
	}

	if repoRoot == "" {
		return fmt.Errorf("unable to determine repository root; set --repo-root flag")
	}

	chartPath, err := filepath.Abs(flags.ChartPath)
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
			err := runIntegrationTests(ctx, repoRoot, chartPath, namespace, flags.Platform)
			resultCh <- TestResult{Type: "integration", Error: err}
		}()
	}

	if runE2E {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := runE2ETests(ctx, repoRoot, chartPath, namespace)
			resultCh <- TestResult{Type: "e2e", Error: err}
		}()
	}

	// Wait for all tests to complete
	wg.Wait()
	close(resultCh)

	// Collect results
	var errors []string
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
	}

	if len(errors) > 0 {
		return fmt.Errorf("test failures:\n  - %s", strings.Join(errors, "\n  - "))
	}

	logging.Logger.Info().Msg("All post-deployment tests passed")
	return nil
}

// runIntegrationTests executes the integration test script.
func runIntegrationTests(ctx context.Context, repoRoot, chartPath, namespace, platform string) error {
	scriptPath := filepath.Join(repoRoot, "scripts", "run-integration-tests.sh")

	if _, err := os.Stat(scriptPath); err != nil {
		return fmt.Errorf("integration test script not found at %s: %w", scriptPath, err)
	}

	logging.Logger.Info().
		Str("script", scriptPath).
		Str("chartPath", chartPath).
		Str("namespace", namespace).
		Str("platform", platform).
		Msg("Running integration tests")

	args := []string{
		"--absolute-chart-path", chartPath,
		"--namespace", namespace,
		"--platform", platform,
	}

	return executeScript(ctx, scriptPath, args, "integration")
}

// runE2ETests executes the e2e test script.
func runE2ETests(ctx context.Context, repoRoot, chartPath, namespace string) error {
	scriptPath := filepath.Join(repoRoot, "scripts", "run-e2e-tests.sh")

	if _, err := os.Stat(scriptPath); err != nil {
		return fmt.Errorf("e2e test script not found at %s: %w", scriptPath, err)
	}

	logging.Logger.Info().
		Str("script", scriptPath).
		Str("chartPath", chartPath).
		Str("namespace", namespace).
		Msg("Running e2e tests")

	args := []string{
		"--absolute-chart-path", chartPath,
		"--namespace", namespace,
	}

	return executeScript(ctx, scriptPath, args, "e2e")
}

// executeScript runs a shell script with the given arguments.
func executeScript(ctx context.Context, scriptPath string, args []string, testType string) error {
	cmd := exec.CommandContext(ctx, scriptPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()

	logging.Logger.Debug().
		Str("command", scriptPath).
		Strs("args", args).
		Str("testType", testType).
		Msg("Executing test script")

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("%s tests failed with exit code %d", testType, exitErr.ExitCode())
		}
		return fmt.Errorf("failed to execute %s tests: %w", testType, err)
	}

	return nil
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
