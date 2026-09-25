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

package e2erun

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Leg outcome categories. They separate product failures from environment
// failures so the report says where to look.
const (
	CategoryPassed          = "passed"
	CategoryTestsFailed     = "tests-failed"
	CategorySetupFailed     = "setup-failed"
	CategoryPreflightFailed = "preflight-failed"
	CategoryTimedOut        = "timed-out"
	CategoryCancelled       = "cancelled"
	CategoryNotRun          = "not-run"
)

const (
	preflightTimeout   = time.Minute
	diagnosticsTimeout = 5 * time.Minute
)

// ExecFunc runs one leg's script and returns its error.
type ExecFunc func(ctx context.Context, leg Leg, args, env []string) error

// DiagnosticsFunc writes namespace diagnostics for a failed leg to path.
type DiagnosticsFunc func(ctx context.Context, leg Leg, path string) error

// PreflightFunc checks that every namespace of a leg is reachable.
type PreflightFunc func(ctx context.Context, leg Leg) error

// Runner executes planned legs and collects their artifacts.
type Runner struct {
	RepoRoot     string
	ArtifactsDir string
	Scenario     string
	Auth         string
	Exclude      string
	Exec         ExecFunc
	Diagnostics  DiagnosticsFunc
	Preflight    PreflightFunc
	Log          io.Writer
	// ScrubEnv names variables removed from every leg's environment.
	ScrubEnv []string
}

// LegResult is the outcome of one leg.
type LegResult struct {
	Leg      Leg
	Err      error
	Category string
	Duration time.Duration
	Stats    *TestStats
	Warnings []string
}

// Failed reports whether the leg did not pass.
func (l LegResult) Failed() bool { return l.Err != nil }

// Result is the outcome of all legs.
type Result struct {
	Legs []LegResult
}

// BlockingFailed reports whether a blocking leg failed.
func (r Result) BlockingFailed() bool { return r.failed(true) }

// NonBlockingFailed reports whether a non-blocking leg failed.
func (r Result) NonBlockingFailed() bool { return r.failed(false) }

func (r Result) failed(blocking bool) bool {
	for _, l := range r.Legs {
		if l.Failed() && l.Leg.Blocking == blocking {
			return true
		}
	}
	return false
}

// Run executes every leg even when an earlier one fails, so one failure
// cannot hide the result of another.
func (r Runner) Run(ctx context.Context, legs []Leg) Result {
	result := Result{}
	for i, leg := range legs {
		fmt.Fprintf(r.Log, "==> e2e leg %d/%d", i+1, len(legs))
		result.Legs = append(result.Legs, r.RunLeg(ctx, leg))
	}
	return result
}

// RunLeg executes one leg, writes diagnostics when it fails, moves its reports
// into ArtifactsDir and classifies the outcome.
func (r Runner) RunLeg(ctx context.Context, leg Leg) LegResult {
	fmt.Fprintf(r.Log, " %s (namespace %s, blocking %t, timeout %s)\n", leg.ID, leg.Namespace, leg.Blocking, leg.Timeout)
	start := time.Now()
	lr := LegResult{Leg: leg}

	if r.Preflight != nil {
		pctx, cancel := context.WithTimeout(ctx, preflightTimeout)
		err := r.Preflight(pctx, leg)
		cancel()
		if err != nil {
			lr.Err, lr.Category = err, CategoryPreflightFailed
			lr.Duration = time.Since(start).Round(time.Second)
			r.logOutcome(lr)
			return lr
		}
	}

	if err := clearSuiteOutputs(leg); err != nil {
		lr.Warnings = append(lr.Warnings, err.Error())
	}

	legCtx, cancel := ctx, context.CancelFunc(func() {})
	if leg.Timeout > 0 {
		legCtx, cancel = context.WithTimeout(ctx, leg.Timeout)
	}
	err := r.Exec(legCtx, leg, ScriptArgs(leg, r.Scenario), r.env(leg))
	timedOut := errors.Is(legCtx.Err(), context.DeadlineExceeded)
	cancel()
	lr.Duration = time.Since(start).Round(time.Second)

	collected, cErr := collectArtifacts(leg, r.ArtifactsDir)
	lr.Warnings = append(lr.Warnings, collected...)
	if cErr != nil {
		lr.Warnings = append(lr.Warnings, "collect artifacts: "+cErr.Error())
	}
	stats, sErr := ReadTestStats(filepath.Join(r.ArtifactsDir, "test-results", leg.ID, "playwright-results.json"))
	lr.Stats = stats

	switch {
	case err == nil:
		lr.Category = CategoryPassed
		switch {
		case sErr != nil:
			lr.Warnings = append(lr.Warnings, "no readable Playwright JSON results: "+sErr.Error())
		case stats.Executed() == 0:
			lr.Warnings = append(lr.Warnings, "the leg passed without executing a test; check the project name and --grep-invert exclusions")
		}
		if stats != nil && stats.Flaky > 0 {
			lr.Warnings = append(lr.Warnings, fmt.Sprintf("%d flaky test(s) passed on retry: %s", stats.Flaky, firstN(stats.FlakyTests, 3)))
		}
	case ctx.Err() != nil:
		lr.Err, lr.Category = fmt.Errorf("cancelled while running: %w", err), CategoryCancelled
	case timedOut:
		lr.Err, lr.Category = fmt.Errorf("exceeded its %s limit; the Playwright process group was stopped: %w", leg.Timeout, err), CategoryTimedOut
	case stats != nil && (stats.Failed > 0 || stats.GlobalErrors > 0):
		lr.Err, lr.Category = fmt.Errorf("%d failed, %d flaky, %d global error(s): %s", stats.Failed, stats.Flaky, stats.GlobalErrors, firstN(stats.FailedTests, 3)), CategoryTestsFailed
	default:
		lr.Err, lr.Category = fmt.Errorf("run-e2e-tests.sh failed (%v) before Playwright reported a failing test; check env rendering, ingress readiness and npm install in the step log", err), CategorySetupFailed
	}

	if lr.Failed() && r.Diagnostics != nil && ctx.Err() == nil {
		path := filepath.Join(r.ArtifactsDir, "diagnostics", leg.ID+".txt")
		if mkErr := os.MkdirAll(filepath.Dir(path), 0o755); mkErr == nil {
			dctx, dcancel := context.WithTimeout(ctx, diagnosticsTimeout)
			if dErr := r.Diagnostics(dctx, leg, path); dErr != nil {
				lr.Warnings = append(lr.Warnings, "namespace diagnostics incomplete: "+dErr.Error())
			}
			dcancel()
		}
	}

	r.logOutcome(lr)
	return lr
}

func (r Runner) logOutcome(lr LegResult) {
	status := lr.Category
	if lr.Err != nil {
		status += ": " + lr.Err.Error()
	}
	if lr.Stats != nil {
		status += fmt.Sprintf(" (tests: %s)", lr.Stats)
	}
	fmt.Fprintf(r.Log, "<== e2e leg %s %s in %s\n", lr.Leg.ID, status, lr.Duration)
	for _, w := range lr.Warnings {
		fmt.Fprintf(r.Log, "    warning: %s\n", w)
	}
}

func (r Runner) env(leg Leg) []string {
	overrides := map[string]string{
		"TEST_NAMESPACE":                     leg.Namespace,
		"TEST_AUTH_TYPE":                     r.Auth,
		"TEST_EXCLUDE":                       r.Exclude,
		"REQUIRE_SM_810_TEST_SUITE":          fmt.Sprint(leg.HubNamespace != ""),
		"REQUIRE_HUB_WEB_MODELER_TEST_SUITE": fmt.Sprint(leg.PlaywrightProject == "hub-web-modeler"),
	}
	for k, v := range leg.Env {
		overrides[k] = v
	}
	drop := map[string]bool{"KEYCLOAK_REALM": true}
	for _, k := range r.ScrubEnv {
		drop[k] = true
	}
	env := []string{}
	for _, kv := range os.Environ() {
		key, _, _ := strings.Cut(kv, "=")
		if _, replaced := overrides[key]; replaced || drop[key] {
			continue
		}
		env = append(env, kv)
	}
	for k, v := range overrides {
		env = append(env, k+"="+v)
	}
	return env
}

// ScriptExec runs scripts/run-e2e-tests.sh from repoRoot/scripts in its own
// process group, so a timeout or cancellation stops Playwright and its browsers
// rather than only the bash wrapper.
func ScriptExec(repoRoot string, stdout, stderr io.Writer) ExecFunc {
	return func(ctx context.Context, leg Leg, args, env []string) error {
		dir := filepath.Join(repoRoot, "scripts")
		cmd := exec.CommandContext(ctx, "bash", append([]string{filepath.Join(dir, "run-e2e-tests.sh")}, args...)...)
		cmd.Dir = dir
		cmd.Env = env
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		cmd.WaitDelay = 30 * time.Second
		setProcessGroup(cmd)
		err := cmd.Run()
		killProcessGroup(cmd)
		return err
	}
}

// KubectlPreflight checks every namespace of a leg with kubectl, telling an
// expired or missing cluster credential apart from a reaped namespace.
func KubectlPreflight() PreflightFunc {
	return func(ctx context.Context, leg Leg) error {
		for _, ns := range leg.Namespaces() {
			out, err := exec.CommandContext(ctx, "kubectl", "get", "namespace", ns, "-o", "name").CombinedOutput()
			if err == nil {
				continue
			}
			msg := firstLine(string(out))
			if strings.Contains(msg, "NotFound") || strings.Contains(msg, "not found") {
				return fmt.Errorf("namespace %s not found; its cleaner/janitor TTL may have reaped it", ns)
			}
			if msg == "" {
				msg = err.Error()
			}
			return fmt.Errorf("cannot reach the cluster for namespace %s (credentials may have expired): %s", ns, msg)
		}
		return nil
	}
}

// DeployCamundaDiagnostics dumps the leg's namespaces with the running
// deploy-camunda binary.
func DeployCamundaDiagnostics() DiagnosticsFunc {
	return func(ctx context.Context, leg Leg, path string) error {
		self, err := os.Executable()
		if err != nil {
			return err
		}
		out, err := os.Create(path)
		if err != nil {
			return err
		}
		defer out.Close()
		args := []string{"diagnostics", "print", "--include-ready"}
		for _, ns := range leg.Namespaces() {
			args = append(args, "--namespace", ns)
		}
		cmd := exec.CommandContext(ctx, self, args...)
		cmd.Stdout = out
		cmd.Stderr = out
		return cmd.Run()
	}
}

func clearSuiteOutputs(leg Leg) error {
	dir := leg.SuiteDir()
	for _, name := range []string{"blob-report", "test-results"} {
		if err := os.RemoveAll(filepath.Join(dir, name)); err != nil {
			return fmt.Errorf("clear %s: %w", name, err)
		}
	}
	return nil
}

// collectArtifacts moves a leg's reports out of the shared suite directory,
// which the next leg would otherwise overwrite:
//
//	<artifacts>/blob-report/<leg-id>-<file>   merged into one HTML report
//	<artifacts>/test-results/<leg-id>/...     JSON results, traces, screenshots
//
// A blob zip that does not open, left by a killed Playwright, is renamed to
// *.corrupt so it cannot fail the merge of every other leg's report. The
// returned strings are warnings for the leg result.
func collectArtifacts(leg Leg, artifactsDir string) ([]string, error) {
	dir := leg.SuiteDir()
	var warnings []string
	var errs []error

	blobDir := filepath.Join(dir, "blob-report")
	if entries, err := os.ReadDir(blobDir); err == nil {
		target := filepath.Join(artifactsDir, "blob-report")
		if err := os.MkdirAll(target, 0o755); err != nil {
			return nil, err
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			dest := filepath.Join(target, leg.ID+"-"+e.Name())
			if err := os.Rename(filepath.Join(blobDir, e.Name()), dest); err != nil {
				errs = append(errs, err)
				continue
			}
			if strings.HasSuffix(dest, ".zip") && !validZip(dest) {
				if err := os.Rename(dest, dest+".corrupt"); err != nil {
					errs = append(errs, err)
				}
				warnings = append(warnings, fmt.Sprintf("blob report %s is truncated and was left out of the merged HTML report", e.Name()))
			}
		}
	} else if !os.IsNotExist(err) {
		errs = append(errs, err)
	}

	results := filepath.Join(dir, "test-results")
	if _, err := os.Stat(results); err == nil {
		target := filepath.Join(artifactsDir, "test-results", leg.ID)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return warnings, err
		}
		if err := os.RemoveAll(target); err != nil {
			errs = append(errs, err)
		}
		if err := os.Rename(results, target); err != nil {
			errs = append(errs, err)
		}
	} else if !os.IsNotExist(err) {
		errs = append(errs, err)
	}
	return warnings, errors.Join(errs...)
}

func validZip(path string) bool {
	r, err := zip.OpenReader(path)
	if err != nil {
		return false
	}
	_ = r.Close()
	return true
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	const max = 300
	if len(s) > max {
		s = s[:max] + "..."
	}
	return s
}

func firstN(items []string, n int) string {
	if len(items) == 0 {
		return "no test titles recorded"
	}
	if len(items) <= n {
		return strings.Join(items, "; ")
	}
	return strings.Join(items[:n], "; ") + fmt.Sprintf("; and %d more", len(items)-n)
}
