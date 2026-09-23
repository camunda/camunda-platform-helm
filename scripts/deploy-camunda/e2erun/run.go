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

// ExecFunc runs one leg's script and returns its error.
type ExecFunc func(ctx context.Context, leg Leg, args, env []string) error

// DiagnosticsFunc writes namespace diagnostics for a failed leg to path.
type DiagnosticsFunc func(ctx context.Context, leg Leg, path string) error

// Runner executes planned legs and collects their artifacts.
type Runner struct {
	RepoRoot     string
	ArtifactsDir string
	Scenario     string
	Auth         string
	Exclude      string
	Exec         ExecFunc
	Diagnostics  DiagnosticsFunc
	Log          io.Writer
	// ScrubEnv names variables removed from every leg's environment.
	ScrubEnv []string
}

// LegResult is the outcome of one leg.
type LegResult struct {
	Leg      Leg
	Err      error
	Duration time.Duration
}

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
		if l.Err != nil && l.Leg.Blocking == blocking {
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

// RunLeg executes one leg, writes diagnostics when it fails and moves its
// reports into ArtifactsDir.
func (r Runner) RunLeg(ctx context.Context, leg Leg) LegResult {
	fmt.Fprintf(r.Log, " %s (namespace %s, blocking %t)\n", leg.ID, leg.Namespace, leg.Blocking)
	if err := clearSuiteOutputs(leg); err != nil {
		fmt.Fprintf(r.Log, "warning: %v\n", err)
	}

	legCtx, cancel := ctx, context.CancelFunc(func() {})
	if leg.Timeout > 0 {
		legCtx, cancel = context.WithTimeout(ctx, leg.Timeout)
	}
	start := time.Now()
	err := r.Exec(legCtx, leg, ScriptArgs(leg, r.Scenario), r.env(leg))
	if err != nil && errors.Is(legCtx.Err(), context.DeadlineExceeded) {
		err = fmt.Errorf("timed out after %s: %w", leg.Timeout, err)
	}
	cancel()
	lr := LegResult{Leg: leg, Err: err, Duration: time.Since(start).Round(time.Second)}

	if err != nil && r.Diagnostics != nil {
		path := filepath.Join(r.ArtifactsDir, "diagnostics", leg.ID+".txt")
		if mkErr := os.MkdirAll(filepath.Dir(path), 0o755); mkErr == nil {
			if dErr := r.Diagnostics(ctx, leg, path); dErr != nil {
				fmt.Fprintf(r.Log, "warning: diagnostics for %s: %v\n", leg.ID, dErr)
			}
		}
	}
	if cErr := collectArtifacts(leg, r.ArtifactsDir); cErr != nil {
		fmt.Fprintf(r.Log, "warning: collect artifacts for %s: %v\n", leg.ID, cErr)
	}

	status := "passed"
	if err != nil {
		status = fmt.Sprintf("failed: %v", err)
	}
	fmt.Fprintf(r.Log, "<== e2e leg %s %s in %s\n", leg.ID, status, lr.Duration)
	return lr
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

// ScriptExec runs scripts/run-e2e-tests.sh from repoRoot/scripts.
func ScriptExec(repoRoot string, stdout, stderr io.Writer) ExecFunc {
	return func(ctx context.Context, leg Leg, args, env []string) error {
		dir := filepath.Join(repoRoot, "scripts")
		cmd := exec.CommandContext(ctx, "bash", append([]string{filepath.Join(dir, "run-e2e-tests.sh")}, args...)...)
		cmd.Dir = dir
		cmd.Env = env
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		cmd.WaitDelay = 30 * time.Second
		return cmd.Run()
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
func collectArtifacts(leg Leg, artifactsDir string) error {
	dir := leg.SuiteDir()
	var errs []error

	blobDir := filepath.Join(dir, "blob-report")
	if entries, err := os.ReadDir(blobDir); err == nil {
		target := filepath.Join(artifactsDir, "blob-report")
		if err := os.MkdirAll(target, 0o755); err != nil {
			return err
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			if err := os.Rename(filepath.Join(blobDir, e.Name()), filepath.Join(target, leg.ID+"-"+e.Name())); err != nil {
				errs = append(errs, err)
			}
		}
	} else if !os.IsNotExist(err) {
		errs = append(errs, err)
	}

	results := filepath.Join(dir, "test-results")
	if _, err := os.Stat(results); err == nil {
		target := filepath.Join(artifactsDir, "test-results", leg.ID)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
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
	return errors.Join(errs...)
}

// Summary renders a Markdown table of leg outcomes.
func Summary(r Result) string {
	var b strings.Builder
	b.WriteString("## E2E legs\n\n| Leg | Namespace | Blocking | Result | Duration |\n|---|---|---|---|---|\n")
	for _, l := range r.Legs {
		status := "passed"
		if l.Err != nil {
			status = "failed"
		}
		fmt.Fprintf(&b, "| `%s` | `%s` | %t | %s | %s |\n", l.Leg.ID, l.Leg.Namespace, l.Leg.Blocking, status, l.Duration)
	}
	return b.String()
}
