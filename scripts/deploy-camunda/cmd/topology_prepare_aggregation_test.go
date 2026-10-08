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

package cmd

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"scripts/deploy-camunda/config"
	"scripts/deploy-camunda/deploy"
	"scripts/deploy-camunda/matrix"
)

func TestPrepareTopologyReleases_AggregatesAllReleaseFailures(t *testing.T) {
	errRenderStage := errors.New("values render stage exploded")
	errPreflightStage := errors.New("preflight stage exploded")

	failures := map[string]error{
		"orcha": errRenderStage,
		"orchc": errPreflightStage,
	}
	healthySuffixes := []string{"hub", "orchb"}

	releases := []matrix.TopologyRelease{
		{
			Role:            "hub",
			NamespaceSuffix: "hub",
			Features:        []string{"multinamespace-hub"},
			Identity:        "keycloak",
			ResolvedDependencies: []matrix.ChartDependency{
				{ReleaseName: "elasticsearch"},
			},
		},
		{
			Role:            "orchestration",
			NamespaceSuffix: "orcha",
			Features:        []string{"multinamespace-orchestration"},
			Identity:        "keycloak-external",
			Persistence:     "elasticsearch-external",
			DependsOn:       "hub",
		},
		{
			Role:            "orchestration",
			NamespaceSuffix: "orchb",
			Features:        []string{"multinamespace-orchestration"},
			Identity:        "keycloak-external",
			Persistence:     "elasticsearch-external",
			DependsOn:       "hub",
		},
		{
			Role:            "orchestration",
			NamespaceSuffix: "orchc",
			Features:        []string{"multinamespace-orchestration"},
			Identity:        "keycloak-external",
			Persistence:     "elasticsearch-external",
			DependsOn:       "hub",
		},
	}

	entry := matrix.Entry{
		Version:   "8.10",
		ChartPath: "charts/camunda-platform-8.10",
		Scenario:  "multinamespace",
		Shortname: "mns",
		Auth:      "keycloak",
		Flow:      "install",
		Platform:  "gke",
		Topology: &matrix.Topology{
			SharedStorage: "elasticsearch",
			Releases:      releases,
		},
	}
	opts := matrix.RunOptions{
		Platform: "gke",
		RepoRoot: t.TempDir(),
	}

	var attempted []string
	tempDirs := make(map[string]string, len(releases))
	preparedBySuffix := make(map[string]*deploy.PreparedScenario, len(releases))

	realPrepareScenarioFn := prepareScenarioFn
	t.Cleanup(func() { prepareScenarioFn = realPrepareScenarioFn })
	prepareScenarioFn = func(_ context.Context, scenarioCtx *deploy.ScenarioContext, _ *config.RuntimeFlags) (*deploy.PreparedScenario, error) {
		suffix := ""
		for _, rel := range releases {
			if strings.HasSuffix(scenarioCtx.Namespace, "-"+rel.NamespaceSuffix) {
				suffix = rel.NamespaceSuffix
				break
			}
		}
		if suffix == "" {
			t.Errorf("prepare stub: namespace %q matches no topology release", scenarioCtx.Namespace)
			return nil, errors.New("prepare stub: unmatched release")
		}

		attempted = append(attempted, suffix)
		tempDir := t.TempDir()
		tempDirs[suffix] = tempDir

		if prepareErr, injected := failures[suffix]; injected {
			_ = os.RemoveAll(tempDir)
			return nil, prepareErr
		}

		prepared := &deploy.PreparedScenario{ScenarioCtx: scenarioCtx, TempDir: tempDir}
		preparedBySuffix[suffix] = prepared
		return prepared, nil
	}

	err := runTopologyEntry(context.Background(), entry, opts)

	if err == nil {
		t.Fatalf("runTopologyEntry() = nil, want an error reporting both the %q and %q prepare failures", "orcha", "orchc")
	}

	wantAttempted := []string{"hub", "orcha", "orchb", "orchc"}
	if strings.Join(attempted, ",") != strings.Join(wantAttempted, ",") {
		t.Errorf("prepared releases %v, want %v: a failing release must not stop the loop", attempted, wantAttempted)
	}

	if !errors.Is(err, errRenderStage) {
		t.Errorf("errors.Is(err, errRenderStage) = false, want true: %q must be reported; err = %v", "orcha", err)
	}
	if !errors.Is(err, errPreflightStage) {
		t.Errorf("errors.Is(err, errPreflightStage) = false, want true: %q must be reported alongside %q, not swallowed by the early return; err = %v", "orchc", "orcha", err)
	}

	message := err.Error()
	for _, suffix := range []string{"orcha", "orchc"} {
		if !strings.Contains(message, suffix) {
			t.Errorf("error message does not name failing release %q; got: %s", suffix, message)
		}
	}
	if strings.Contains(message, "orchb") {
		t.Errorf("error message names healthy release %q; got: %s", "orchb", message)
	}

	for suffix, tempDir := range tempDirs {
		if _, statErr := os.Stat(tempDir); !os.IsNotExist(statErr) {
			t.Errorf("release %q leaked temp dir %s (os.Stat error = %v), want it removed", suffix, tempDir, statErr)
		}
	}

	for _, suffix := range healthySuffixes {
		prepared, ok := preparedBySuffix[suffix]
		if !ok {
			t.Errorf("healthy release %q was never prepared", suffix)
			continue
		}
		if prepared.TempDir != "" {
			t.Errorf("healthy release %q has TempDir = %q after the call, want it drained by the deferred prepared.Cleanup(): the release never reached preparedReleases", suffix, prepared.TempDir)
		}
	}
}
