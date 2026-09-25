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
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlanSuiteLegsKeepRegistryOrderAndBlocking(t *testing.T) {
	legs, err := Plan(PlanInput{
		RepoRoot:  "/repo",
		ChartDir:  "camunda-platform-8.10",
		Namespace: "ns",
		Stage:     "after-install",
		SuiteLegs: `[{"suite":"smoke","blocking":true,"shard_index":1,"shard_total":1},{"suite":"full","blocking":false,"shard_index":1,"shard_total":1}]`,
	})
	require.NoError(t, err)
	require.Len(t, legs, 2)

	assert.Equal(t, "after-install-smoke-1", legs[0].ID)
	assert.True(t, legs[0].Blocking)
	assert.True(t, legs[0].RunSmokeTests)
	assert.Equal(t, smokeTimeout, legs[0].Timeout)
	assert.Equal(t, "/repo/charts/camunda-platform-8.10", legs[0].ChartPath)

	assert.Equal(t, "after-install-full-1", legs[1].ID)
	assert.False(t, legs[1].Blocking)
	assert.False(t, legs[1].RunSmokeTests)
	assert.Equal(t, fullTimeout, legs[1].Timeout)
}

func TestPlanEmptyInputsYieldNoLegs(t *testing.T) {
	for _, raw := range []string{"", "[]", "null"} {
		legs, err := Plan(PlanInput{Namespace: "ns", SuiteLegs: raw, TopologyLegs: raw})
		require.NoError(t, err)
		assert.Empty(t, legs)
	}
}

func TestPlanRejectsUnknownSuite(t *testing.T) {
	_, err := Plan(PlanInput{Namespace: "ns", SuiteLegs: `[{"suite":"nope"}]`})
	require.Error(t, err)
}

func TestPlanTopologyLegsDeriveReleaseNamespaces(t *testing.T) {
	legs, err := Plan(PlanInput{
		RepoRoot:          "/repo",
		ChartDir:          "camunda-platform-8.10",
		Namespace:         "base",
		Stage:             "after-install",
		TopologyHubSuffix: "hub",
		TopologyLegs: `[{"orchestration_suffix":"orcha","shard_index":"1","chart_dir":"camunda-platform-8.9","test_chart_dir":"camunda-platform-8.9",` +
			`"suite":"orchestration","playwright_project":"topology-orchestration","optimize_suffix":"opta","optimize_context_path":"/optimize-orcha","modeler_cluster_name":"Cluster A"}]`,
	})
	require.NoError(t, err)
	require.Len(t, legs, 1)
	leg := legs[0]

	assert.Equal(t, "after-install-topology-1-orcha-orchestration", leg.ID)
	assert.True(t, leg.Blocking)
	assert.Equal(t, "base-orcha", leg.Namespace)
	assert.Equal(t, "base-hub", leg.HubNamespace)
	assert.Equal(t, "base-opta", leg.OptimizeNamespace)
	assert.Equal(t, "/repo/charts/camunda-platform-8.9", leg.ChartPath)
	assert.Equal(t, "/repo/charts/camunda-platform-8.9/test/e2e", leg.SuiteDir())
	assert.Equal(t, []string{"base-orcha", "base-hub", "base-opta"}, leg.Namespaces())
}

func TestPlanTopologyTruncatesLongBaseNamespace(t *testing.T) {
	base := strings.Repeat("a", 62)
	legs, err := Plan(PlanInput{
		Namespace:         base,
		TopologyHubSuffix: "hub",
		TopologyLegs:      `[{"orchestration_suffix":"orcha","chart_dir":"camunda-platform-8.10"}]`,
	})
	require.NoError(t, err)
	assert.LessOrEqual(t, len(legs[0].Namespace), 63)
	assert.True(t, strings.HasSuffix(legs[0].Namespace, "-orcha"))
}

func TestPlanTopologyRequiresHubSuffix(t *testing.T) {
	_, err := Plan(PlanInput{Namespace: "base", TopologyLegs: `[{"orchestration_suffix":"orcha","chart_dir":"camunda-platform-8.10"}]`})
	require.Error(t, err)
}

func TestPlanShadowLegIsNonBlockingFullSuite(t *testing.T) {
	legs, err := Plan(PlanInput{ChartDir: "camunda-platform-8.10", Namespace: "ns", Stage: "after-install", Shadow: true})
	require.NoError(t, err)
	require.Len(t, legs, 1)
	assert.Equal(t, "after-install-shadow-full", legs[0].ID)
	assert.False(t, legs[0].Blocking)
	assert.False(t, legs[0].RunSmokeTests)
	assert.NotContains(t, legs[0].Env, "PLAYWRIGHT_E2E_RETRIES")
	assert.Equal(t, "1", legs[0].Env["PLAYWRIGHT_POD_RETRY_MAX_ATTEMPTS"])
}

func TestScriptArgs(t *testing.T) {
	smoke := Leg{ChartPath: "/c", Namespace: "ns", RunSmokeTests: true}
	for _, tc := range []struct {
		name     string
		leg      Leg
		scenario string
		want     []string
	}{
		{"smoke", smoke, "elasticsearch", []string{"--absolute-chart-path", "/c", "--namespace", "ns", "--run-smoke-tests", "--trace", "retain-on-failure", "--verbose"}},
		{"opensearch", smoke, "opensearch-self-signed", []string{"--absolute-chart-path", "/c", "--namespace", "ns", "--run-smoke-tests", "--trace", "retain-on-failure", "--verbose", "--opensearch"}},
		{"rba", smoke, "keycloak-rba", []string{"--absolute-chart-path", "/c", "--namespace", "ns", "--run-smoke-tests", "--trace", "retain-on-failure", "--verbose", "--rba"}},
		{"multitenancy", smoke, "multitenancy", []string{"--absolute-chart-path", "/c", "--namespace", "ns", "--run-smoke-tests", "--trace", "retain-on-failure", "--verbose", "--mt"}},
		{"auth0 drops smoke", smoke, "auth0", []string{"--absolute-chart-path", "/c", "--namespace", "ns", "--trace", "retain-on-failure", "--verbose", "--auth0"}},
		{
			"topology",
			Leg{ChartPath: "/c", TestChartPath: "/t", Namespace: "ns-orcha", HubNamespace: "ns-hub", OptimizeNamespace: "ns-opt", OptimizeContextPath: "/o", ModelerClusterName: "A", PlaywrightProject: "p"},
			"mns",
			[]string{"--absolute-chart-path", "/c", "--namespace", "ns-orcha", "--test-chart-path", "/t", "--trace", "retain-on-failure", "--verbose",
				"--hub-namespace", "ns-hub", "--optimize-namespace", "ns-opt", "--optimize-context-path", "/o", "--modeler-cluster-name", "A", "--playwright-project", "p"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, ScriptArgs(tc.leg, tc.scenario))
		})
	}
}

func TestRunExecutesEveryLegAndCollectsArtifacts(t *testing.T) {
	chart := t.TempDir()
	artifacts := t.TempDir()
	suite := filepath.Join(chart, "test", "e2e")
	t.Setenv("KEYCLOAK_REALM", "leak")

	legs := []Leg{
		{ID: "smoke", Blocking: true, Namespace: "ns", ChartPath: chart},
		{ID: "full", Blocking: false, Namespace: "ns", ChartPath: chart, Env: map[string]string{"PLAYWRIGHT_E2E_RETRIES": "0"}},
	}
	var ran []string
	var diagnosed []string
	runner := Runner{
		ArtifactsDir: artifacts,
		Scenario:     "elasticsearch",
		Auth:         "keycloak",
		Log:          io.Discard,
		Exec: func(_ context.Context, leg Leg, _, env []string) error {
			ran = append(ran, leg.ID)
			assert.Contains(t, env, "TEST_AUTH_TYPE=keycloak")
			assert.Contains(t, env, "TEST_NAMESPACE=ns")
			assert.False(t, slices.ContainsFunc(env, func(kv string) bool { return strings.HasPrefix(kv, "KEYCLOAK_REALM=") }))
			writeZip(t, filepath.Join(suite, "blob-report", "report.zip"), leg.ID)
			if leg.ID == "full" {
				assert.Contains(t, env, "PLAYWRIGHT_E2E_RETRIES=0")
				writeFile(t, filepath.Join(suite, "test-results", "playwright-results.json"), failingReport)
				return errors.New("exit status 1")
			}
			writeFile(t, filepath.Join(suite, "test-results", "playwright-results.json"), passingReport)
			return nil
		},
		Diagnostics: func(_ context.Context, leg Leg, path string) error {
			diagnosed = append(diagnosed, leg.ID)
			return os.WriteFile(path, []byte("diag"), 0o644)
		},
	}

	result := runner.Run(context.Background(), legs)

	assert.Equal(t, []string{"smoke", "full"}, ran)
	assert.Equal(t, []string{"full"}, diagnosed)
	assert.False(t, result.BlockingFailed())
	assert.True(t, result.NonBlockingFailed())
	assert.FileExists(t, filepath.Join(artifacts, "blob-report", "smoke-report.zip"))
	assert.FileExists(t, filepath.Join(artifacts, "blob-report", "full-report.zip"))
	assert.FileExists(t, filepath.Join(artifacts, "test-results", "smoke", "playwright-results.json"))
	assertFile(t, filepath.Join(artifacts, "diagnostics", "full.txt"), "diag")
	assert.NoFileExists(t, filepath.Join(suite, "blob-report", "report.zip"))

	assert.Equal(t, CategoryPassed, result.Legs[0].Category)
	assert.Empty(t, result.Legs[0].Warnings)
	assert.Equal(t, CategoryTestsFailed, result.Legs[1].Category)
	assert.Equal(t, []string{"specs/login.spec.ts:12 › Login › logs in [full-suite]"}, result.Legs[1].Stats.FailedTests)
	assert.ErrorContains(t, result.Legs[1].Err, "1 failed")

	summary := Summary(result)
	assert.Contains(t, summary, "| 1 | `smoke` | `ns` | true | ✅ passed | 3 passed, 0 failed, 0 flaky, 0 skipped |")
	assert.Contains(t, summary, "| 2 | `full` | `ns` | false | ❌ tests-failed |")
	assert.Contains(t, summary, "- `specs/login.spec.ts:12 › Login › logs in [full-suite]`")
	assert.Contains(t, summary, "E2E - 🧪 Run Playwright leg 2 🧪")
	assert.Contains(t, summary, "diagnostics-e2e-*")
}

func TestRunLegClassifiesFailures(t *testing.T) {
	for _, tc := range []struct {
		name         string
		report       string
		exec         func(ctx context.Context) error
		timeout      time.Duration
		preflightErr error
		cancelParent bool
		wantCategory string
		wantExec     bool
		wantDiag     bool
	}{
		{name: "tests failed", report: failingReport, exec: func(context.Context) error { return errors.New("exit status 1") }, wantCategory: CategoryTestsFailed, wantExec: true, wantDiag: true},
		{name: "global error counts as test failure", report: globalErrorReport, exec: func(context.Context) error { return errors.New("exit status 1") }, wantCategory: CategoryTestsFailed, wantExec: true, wantDiag: true},
		{name: "script failed before playwright", exec: func(context.Context) error { return errors.New("exit status 1") }, wantCategory: CategorySetupFailed, wantExec: true, wantDiag: true},
		{name: "timed out", timeout: 10 * time.Millisecond, exec: func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() }, wantCategory: CategoryTimedOut, wantExec: true, wantDiag: true},
		{name: "cancelled", cancelParent: true, exec: func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() }, wantCategory: CategoryCancelled, wantExec: true},
		{name: "preflight failed skips the leg", preflightErr: errors.New("namespace ns not found"), wantCategory: CategoryPreflightFailed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			chart := t.TempDir()
			suite := filepath.Join(chart, "test", "e2e")
			executed, diagnosed := false, false
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			runner := Runner{
				ArtifactsDir: t.TempDir(),
				Log:          io.Discard,
				Preflight:    func(context.Context, Leg) error { return tc.preflightErr },
				Exec: func(ctx context.Context, _ Leg, _, _ []string) error {
					executed = true
					if tc.report != "" {
						writeFile(t, filepath.Join(suite, "test-results", "playwright-results.json"), tc.report)
					}
					if tc.cancelParent {
						cancel()
					}
					return tc.exec(ctx)
				},
				Diagnostics: func(context.Context, Leg, string) error { diagnosed = true; return nil },
			}
			lr := runner.RunLeg(ctx, Leg{ID: "leg", Blocking: true, Namespace: "ns", ChartPath: chart, Timeout: tc.timeout})
			assert.Equal(t, tc.wantCategory, lr.Category)
			assert.Error(t, lr.Err)
			assert.Equal(t, tc.wantExec, executed)
			assert.Equal(t, tc.wantDiag, diagnosed)
		})
	}
}

func TestRunLegWarnsOnSilentPasses(t *testing.T) {
	for _, tc := range []struct {
		name, report, blob, want string
	}{
		{name: "no tests executed", report: emptyReport, want: "without executing a test"},
		{name: "flaky tests", report: flakyReport, want: "1 flaky test(s) passed on retry: specs/a.spec.ts:3 › a [smoke-tests]"},
		{name: "missing json results", want: "no readable Playwright JSON results"},
		{name: "corrupt blob zip", report: passingReport, blob: "truncated", want: "truncated and was left out"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			chart := t.TempDir()
			artifacts := t.TempDir()
			suite := filepath.Join(chart, "test", "e2e")
			runner := Runner{
				ArtifactsDir: artifacts,
				Log:          io.Discard,
				Exec: func(context.Context, Leg, []string, []string) error {
					if tc.report != "" {
						writeFile(t, filepath.Join(suite, "test-results", "playwright-results.json"), tc.report)
					}
					if tc.blob != "" {
						writeFile(t, filepath.Join(suite, "blob-report", "report.zip"), tc.blob)
					}
					return nil
				},
			}
			lr := runner.RunLeg(context.Background(), Leg{ID: "leg", Blocking: true, ChartPath: chart})
			require.NoError(t, lr.Err)
			assert.Equal(t, CategoryPassed, lr.Category)
			require.NotEmpty(t, lr.Warnings)
			assert.Contains(t, strings.Join(lr.Warnings, "\n"), tc.want)
			if tc.blob != "" {
				assert.FileExists(t, filepath.Join(artifacts, "blob-report", "leg-report.zip.corrupt"))
				assert.NoFileExists(t, filepath.Join(artifacts, "blob-report", "leg-report.zip"))
			}
		})
	}
}

func TestPlanRejectsDuplicateLegIDs(t *testing.T) {
	_, err := Plan(PlanInput{Namespace: "ns", SuiteLegs: `[{"suite":"smoke","shard_index":1},{"suite":"smoke","shard_index":1}]`})
	require.ErrorContains(t, err, "duplicate e2e leg id")
}

func TestAnnotationsLevelsAndEscaping(t *testing.T) {
	got := Annotations(Result{Legs: []LegResult{
		{Leg: Leg{ID: "a", Blocking: true}, Category: CategorySetupFailed, Err: errors.New("line1\nline2 100%")},
		{Leg: Leg{ID: "b", Blocking: false}, Category: CategoryTestsFailed, Err: errors.New("x")},
		{Leg: Leg{ID: "c", Blocking: true}, Category: CategoryPassed, Warnings: []string{"flaky"}},
	}})
	assert.Equal(t, []string{
		"::error title=E2E leg a setup-failed::line1%0Aline2 100%25",
		"::warning title=E2E leg b tests-failed::x",
		"::warning title=E2E leg c::flaky",
	}, got)
}

func TestScriptExecStopsTheWholeProcessGroupOnTimeout(t *testing.T) {
	repo := t.TempDir()
	pidFile := filepath.Join(repo, "child.pid")
	writeFile(t, filepath.Join(repo, "scripts", "run-e2e-tests.sh"), "sleep 300 &\necho $! > "+pidFile+"\nwait\n")

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	err := ScriptExec(repo, io.Discard, io.Discard)(ctx, Leg{}, nil, os.Environ())
	require.Error(t, err)

	data, readErr := os.ReadFile(pidFile)
	require.NoError(t, readErr)
	pid, convErr := strconv.Atoi(strings.TrimSpace(string(data)))
	require.NoError(t, convErr)
	require.Eventually(t, func() bool { return syscall.Kill(pid, 0) != nil }, 5*time.Second, 50*time.Millisecond,
		"the script's child process survived the timeout")
}

func TestReadTestStatsWalksNestedSuites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "r.json")
	writeFile(t, path, failingReport)
	stats, err := ReadTestStats(path)
	require.NoError(t, err)
	assert.Equal(t, 2, stats.Passed)
	assert.Equal(t, 1, stats.Failed)
	assert.Equal(t, 3, stats.Executed())
}

const passingReport = `{"suites":[],"errors":[],"stats":{"expected":3,"unexpected":0,"flaky":0,"skipped":0}}`

const emptyReport = `{"suites":[],"errors":[],"stats":{"expected":0,"unexpected":0,"flaky":0,"skipped":4}}`

const globalErrorReport = `{"suites":[],"errors":[{"message":"globalSetup failed"}],"stats":{"expected":0,"unexpected":0,"flaky":0,"skipped":0}}`

const failingReport = `{"suites":[{"title":"specs/login.spec.ts","specs":[],"suites":[{"title":"Login","specs":[
  {"title":"logs in","file":"specs/login.spec.ts","line":12,"tests":[{"projectName":"full-suite","status":"unexpected"}]},
  {"title":"logs out","file":"specs/login.spec.ts","line":30,"tests":[{"projectName":"full-suite","status":"expected"}]}
]}]}],"errors":[],"stats":{"expected":2,"unexpected":1,"flaky":0,"skipped":0}}`

const flakyReport = `{"suites":[{"title":"specs/a.spec.ts","specs":[
  {"title":"a","file":"specs/a.spec.ts","line":3,"tests":[{"projectName":"smoke-tests","status":"flaky"}]}
]}],"errors":[],"stats":{"expected":1,"unexpected":0,"flaky":1,"skipped":0}}`

func writeZip(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	f, err := os.Create(path)
	require.NoError(t, err)
	w := zip.NewWriter(f)
	entry, err := w.Create("report.jsonl")
	require.NoError(t, err)
	_, err = entry.Write([]byte(content))
	require.NoError(t, err)
	require.NoError(t, w.Close())
	require.NoError(t, f.Close())
}

func TestRunBlockingFailureDoesNotStopLaterLegs(t *testing.T) {
	var ran []string
	runner := Runner{
		ArtifactsDir: t.TempDir(),
		Log:          io.Discard,
		Exec: func(_ context.Context, leg Leg, _, _ []string) error {
			ran = append(ran, leg.ID)
			if leg.ID == "a" {
				return errors.New("fail")
			}
			return nil
		},
	}
	chart := t.TempDir()
	result := runner.Run(context.Background(), []Leg{
		{ID: "a", Blocking: true, ChartPath: chart},
		{ID: "b", Blocking: true, ChartPath: chart},
	})
	assert.Equal(t, []string{"a", "b"}, ran)
	assert.True(t, result.BlockingFailed())
}

func TestRunTimesOutLeg(t *testing.T) {
	runner := Runner{
		ArtifactsDir: t.TempDir(),
		Log:          io.Discard,
		Exec: func(ctx context.Context, _ Leg, _, _ []string) error {
			<-ctx.Done()
			return ctx.Err()
		},
	}
	result := runner.Run(context.Background(), []Leg{{ID: "slow", Blocking: true, ChartPath: t.TempDir(), Timeout: 10 * time.Millisecond}})
	require.Error(t, result.Legs[0].Err)
	assert.Equal(t, CategoryTimedOut, result.Legs[0].Category)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func assertFile(t *testing.T, path, content string) {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, content, string(data))
}

func TestSaveAndLoadResultsTreatsMissingLegAsFailed(t *testing.T) {
	dir := t.TempDir()
	legs := []Leg{{ID: "a", Blocking: true}, {ID: "b", Blocking: false}, {ID: "c", Blocking: true}}
	require.NoError(t, SaveResult(dir, 0, LegResult{Leg: legs[0], Duration: 3 * time.Second}))
	require.NoError(t, SaveResult(dir, 1, LegResult{Leg: legs[1], Err: errors.New("boom"), Category: CategoryTestsFailed,
		Stats: &TestStats{Failed: 1, FailedTests: []string{"t"}}, Warnings: []string{"w"}}))

	result := LoadResults(dir, legs, "success")
	require.Len(t, result.Legs, 3)
	assert.NoError(t, result.Legs[0].Err)
	assert.Equal(t, 3*time.Second, result.Legs[0].Duration)
	assert.EqualError(t, result.Legs[1].Err, "boom")
	assert.Equal(t, CategoryTestsFailed, result.Legs[1].Category)
	assert.Equal(t, []string{"t"}, result.Legs[1].Stats.FailedTests)
	assert.Equal(t, []string{"w"}, result.Legs[1].Warnings)
	assert.ErrorContains(t, result.Legs[2].Err, "no result recorded")
	assert.Equal(t, CategoryNotRun, result.Legs[2].Category)

	cancelled := LoadResults(dir, legs, "cancelled")
	assert.Equal(t, CategoryCancelled, cancelled.Legs[2].Category)
	assert.True(t, result.BlockingFailed())
	assert.True(t, result.NonBlockingFailed())
}

func TestLoadResultsRejectsResultOfDifferentLeg(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, SaveResult(dir, 0, LegResult{Leg: Leg{ID: "other"}}))
	result := LoadResults(dir, []Leg{{ID: "planned", Blocking: true}}, "")
	assert.ErrorContains(t, result.Legs[0].Err, "does not match")
}

func TestMappedEnvNames(t *testing.T) {
	mapping := `
secret/data/products/distribution/ci ENTRA_APP_CLIENT_ID;
secret/data/products/distribution/ci HARBOR_REGISTRY_USER | TEST_DOCKER_USERNAME;

secret/data/products/distribution/ci AUTH0_DOMAIN`
	assert.Equal(t, []string{
		"ENTRA_APP_CLIENT_ID", "ENTRA_APP_CLIENT_ID",
		"TEST_DOCKER_USERNAME", "TEST_DOCKER_USERNAME",
		"AUTH0_DOMAIN", "AUTH0_DOMAIN",
	}, MappedEnvNames(mapping))
	assert.Empty(t, MappedEnvNames(""))
}

func TestRunLegScrubsMappedSecrets(t *testing.T) {
	t.Setenv("ENTRA_APP_CLIENT_SECRET", "s3cret")
	t.Setenv("UNRELATED", "kept")
	runner := Runner{
		ArtifactsDir: t.TempDir(),
		Log:          io.Discard,
		ScrubEnv:     MappedEnvNames("secret/data/x ENTRA_APP_CLIENT_SECRET;"),
		Exec: func(_ context.Context, _ Leg, _, env []string) error {
			assert.Contains(t, env, "UNRELATED=kept")
			assert.False(t, slices.ContainsFunc(env, func(kv string) bool { return strings.HasPrefix(kv, "ENTRA_APP_CLIENT_SECRET=") }))
			return nil
		},
	}
	require.NoError(t, runner.RunLeg(context.Background(), Leg{ID: "a", ChartPath: t.TempDir()}).Err)
}
