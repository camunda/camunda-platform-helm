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
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
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
			writeFile(t, filepath.Join(suite, "blob-report", "report.zip"), leg.ID)
			writeFile(t, filepath.Join(suite, "test-results", "playwright-results.json"), leg.ID)
			if leg.ID == "full" {
				assert.Contains(t, env, "PLAYWRIGHT_E2E_RETRIES=0")
				return errors.New("boom")
			}
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
	assertFile(t, filepath.Join(artifacts, "blob-report", "smoke-report.zip"), "smoke")
	assertFile(t, filepath.Join(artifacts, "blob-report", "full-report.zip"), "full")
	assertFile(t, filepath.Join(artifacts, "test-results", "smoke", "playwright-results.json"), "smoke")
	assertFile(t, filepath.Join(artifacts, "test-results", "full", "playwright-results.json"), "full")
	assertFile(t, filepath.Join(artifacts, "diagnostics", "full.txt"), "diag")
	assert.NoDirExists(t, filepath.Join(suite, "blob-report", "report.zip"))

	summary := Summary(result)
	assert.Contains(t, summary, "| `smoke` | `ns` | true | passed |")
	assert.Contains(t, summary, "| `full` | `ns` | false | failed |")
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
	assert.Contains(t, result.Legs[0].Err.Error(), "timed out")
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
	require.NoError(t, SaveResult(dir, 1, LegResult{Leg: legs[1], Err: errors.New("boom")}))

	result := LoadResults(dir, legs)
	require.Len(t, result.Legs, 3)
	assert.NoError(t, result.Legs[0].Err)
	assert.Equal(t, 3*time.Second, result.Legs[0].Duration)
	assert.EqualError(t, result.Legs[1].Err, "boom")
	assert.ErrorContains(t, result.Legs[2].Err, "did not run")
	assert.True(t, result.BlockingFailed())
	assert.True(t, result.NonBlockingFailed())
}

func TestLoadResultsRejectsResultOfDifferentLeg(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, SaveResult(dir, 0, LegResult{Leg: Leg{ID: "other"}}))
	result := LoadResults(dir, []Leg{{ID: "planned", Blocking: true}})
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
