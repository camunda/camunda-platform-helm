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

// Package e2erun runs every Playwright leg of one integration scenario
// sequentially against an already deployed namespace, so a CI job can deploy
// and test in one unit that "Re-run failed jobs" restarts from the deploy.
package e2erun

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"scripts/deploy-camunda/deploy"
	"scripts/deploy-camunda/matrix"
)

const (
	smokeTimeout    = 25 * time.Minute
	fullTimeout     = 50 * time.Minute
	topologyTimeout = 40 * time.Minute
)

// Leg is one run-e2e-tests.sh invocation.
type Leg struct {
	ID                  string
	Blocking            bool
	Namespace           string
	ChartPath           string
	TestChartPath       string
	RunSmokeTests       bool
	PlaywrightProject   string
	HubNamespace        string
	OptimizeNamespace   string
	OptimizeContextPath string
	ModelerClusterName  string
	Timeout             time.Duration
	Env                 map[string]string
}

// SuiteDir is the Playwright suite directory the leg writes its reports into.
func (l Leg) SuiteDir() string {
	base := l.TestChartPath
	if base == "" {
		base = l.ChartPath
	}
	return filepath.Join(base, "test", "e2e")
}

// PlanInput describes the scenario whose legs are planned.
type PlanInput struct {
	RepoRoot  string
	ChartDir  string
	Namespace string
	Stage     string
	// SuiteLegs is the JSON emitted by `deploy-camunda ci e2e-matrix`.
	SuiteLegs string
	// TopologyLegs is the topology smoke matrix JSON emitted by `matrix plan`.
	TopologyLegs      string
	TopologyHubSuffix string
	// Shadow appends the non-blocking full-suite leg of the shadow-e2e scenario.
	Shadow bool
}

type topologyLeg struct {
	OrchestrationSuffix string `json:"orchestration_suffix"`
	ModelerClusterName  string `json:"modeler_cluster_name"`
	ShardIndex          string `json:"shard_index"`
	OptimizeSuffix      string `json:"optimize_suffix"`
	OptimizeContextPath string `json:"optimize_context_path"`
	ChartDir            string `json:"chart_dir"`
	Suite               string `json:"suite"`
	TestChartDir        string `json:"test_chart_dir"`
	PlaywrightProject   string `json:"playwright_project"`
}

// Plan returns the legs in execution order: suite legs, then topology legs,
// then the shadow leg.
func Plan(in PlanInput) ([]Leg, error) {
	if in.Namespace == "" {
		return nil, fmt.Errorf("namespace is required")
	}
	chartPath := filepath.Join(in.RepoRoot, "charts", in.ChartDir)
	legs := []Leg{}

	suiteLegs, err := decode[matrix.E2ELeg](in.SuiteLegs)
	if err != nil {
		return nil, fmt.Errorf("parse suite legs: %w", err)
	}
	for _, s := range suiteLegs {
		switch s.Suite {
		case matrix.SuiteSmoke, matrix.SuiteFull:
		default:
			return nil, fmt.Errorf("unknown e2e suite %q", s.Suite)
		}
		leg := Leg{
			ID:            artifactID(in.Stage, s.Suite, fmt.Sprint(s.ShardIndex)),
			Blocking:      s.Blocking,
			Namespace:     in.Namespace,
			ChartPath:     chartPath,
			RunSmokeTests: s.Suite == matrix.SuiteSmoke,
			Timeout:       smokeTimeout,
		}
		if s.Suite == matrix.SuiteFull {
			leg.Timeout = fullTimeout
		}
		legs = append(legs, leg)
	}

	topologyLegs, err := decode[topologyLeg](in.TopologyLegs)
	if err != nil {
		return nil, fmt.Errorf("parse topology legs: %w", err)
	}
	if len(topologyLegs) > 0 && in.TopologyHubSuffix == "" {
		return nil, fmt.Errorf("topology legs require a hub namespace suffix")
	}
	for _, t := range topologyLegs {
		leg, err := planTopologyLeg(in, t)
		if err != nil {
			return nil, err
		}
		legs = append(legs, leg)
	}

	if in.Shadow {
		legs = append(legs, Leg{
			ID:        artifactID(in.Stage, "shadow", "full"),
			Namespace: in.Namespace,
			ChartPath: chartPath,
			Timeout:   fullTimeout,
			Env: map[string]string{
				"PLAYWRIGHT_POD_RETRY_MAX_ATTEMPTS": "1",
			},
		})
	}
	return legs, nil
}

func planTopologyLeg(in PlanInput, t topologyLeg) (Leg, error) {
	if t.OrchestrationSuffix == "" || t.ChartDir == "" {
		return Leg{}, fmt.Errorf("topology leg %+v needs orchestration_suffix and chart_dir", t)
	}
	namespace, err := deploy.DeriveReleaseNamespace(in.Namespace, t.OrchestrationSuffix)
	if err != nil {
		return Leg{}, err
	}
	hub, err := deploy.DeriveReleaseNamespace(in.Namespace, in.TopologyHubSuffix)
	if err != nil {
		return Leg{}, err
	}
	leg := Leg{
		ID:                  artifactID(in.Stage, "topology", t.ShardIndex, t.OrchestrationSuffix, t.Suite),
		Blocking:            true,
		Namespace:           namespace,
		ChartPath:           filepath.Join(in.RepoRoot, "charts", t.ChartDir),
		PlaywrightProject:   t.PlaywrightProject,
		HubNamespace:        hub,
		OptimizeContextPath: t.OptimizeContextPath,
		ModelerClusterName:  t.ModelerClusterName,
		Timeout:             topologyTimeout,
	}
	if t.TestChartDir != "" {
		leg.TestChartPath = filepath.Join(in.RepoRoot, "charts", t.TestChartDir)
	}
	if t.OptimizeSuffix != "" {
		if leg.OptimizeNamespace, err = deploy.DeriveReleaseNamespace(in.Namespace, t.OptimizeSuffix); err != nil {
			return Leg{}, err
		}
	}
	return leg, nil
}

// ScriptArgs builds the run-e2e-tests.sh arguments for a leg of scenario.
func ScriptArgs(leg Leg, scenario string) []string {
	args := []string{"--absolute-chart-path", leg.ChartPath, "--namespace", leg.Namespace}
	if leg.TestChartPath != "" {
		args = append(args, "--test-chart-path", leg.TestChartPath)
	}
	scenarioFlag := ""
	smoke := leg.RunSmokeTests
	switch {
	case strings.HasPrefix(scenario, "opensearch"):
		scenarioFlag = "--opensearch"
	case scenario == "keycloak-rba":
		scenarioFlag = "--rba"
	case scenario == "keycloak-mt" || scenario == "multitenancy":
		scenarioFlag = "--mt"
	case scenario == "auth0":
		scenarioFlag = "--auth0"
		smoke = false
	}
	if smoke {
		args = append(args, "--run-smoke-tests")
	}
	args = append(args, "--trace", "retain-on-failure", "--verbose")
	if scenarioFlag != "" {
		args = append(args, scenarioFlag)
	}
	for _, opt := range []struct{ flag, value string }{
		{"--hub-namespace", leg.HubNamespace},
		{"--optimize-namespace", leg.OptimizeNamespace},
		{"--optimize-context-path", leg.OptimizeContextPath},
		{"--modeler-cluster-name", leg.ModelerClusterName},
		{"--playwright-project", leg.PlaywrightProject},
	} {
		if opt.value != "" {
			args = append(args, opt.flag, opt.value)
		}
	}
	return args
}

// Namespaces lists every namespace a leg reads, deduplicated, for diagnostics.
func (l Leg) Namespaces() []string {
	out := []string{}
	seen := map[string]bool{}
	for _, ns := range []string{l.Namespace, l.HubNamespace, l.OptimizeNamespace} {
		if ns != "" && !seen[ns] {
			seen[ns] = true
			out = append(out, ns)
		}
	}
	return out
}

var unsafeArtifactChars = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

func artifactID(parts ...string) string {
	kept := []string{}
	for _, p := range parts {
		if p != "" {
			kept = append(kept, p)
		}
	}
	return unsafeArtifactChars.ReplaceAllString(strings.Join(kept, "-"), "-")
}

func decode[T any](raw string) ([]T, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return nil, nil
	}
	var out []T
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, err
	}
	return out, nil
}
