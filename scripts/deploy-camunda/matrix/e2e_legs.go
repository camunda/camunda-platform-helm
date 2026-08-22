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

package matrix

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"scripts/camunda-core/pkg/logging"
)

// E2E leg suite names. SuiteSmoke selects the Playwright "smoke-tests" project,
// SuiteFull the "full-suite" project.
const (
	SuiteSmoke = "smoke"
	SuiteFull  = "full"
)

// Default blocking behavior per suite, applied when a scenario leaves the
// corresponding *bool unset. Smoke is the long-standing required signal; the
// full suite starts as an informational one.
const (
	defaultSmokeBlocking = true
	defaultFullBlocking  = false
)

// E2ELeg is one entry of the e2e job's GitHub Actions matrix: which Playwright
// project the leg runs and whether its failure fails the workflow run. Field
// names are snake_case to match the topology smoke matrix already consumed by
// test-integration-runner.yaml.
type E2ELeg struct {
	Suite      string `json:"suite"`
	Blocking   bool   `json:"blocking"`
	ShardIndex int    `json:"shard_index"`
	ShardTotal int    `json:"shard_total"`
}

// ResolveE2ELegs returns the e2e matrix legs for a scenario in the registry
// under <repoRoot>/charts/<chartDir>, smoke first.
//
// Resolution prefers shortname, because scenario Name is NOT unique: 8.8 has
// three registry files declaring `name: elasticsearch` with different
// shortnames and different skip-e2e values, and 8.7 has two declaring
// `name: keycloak-original`. Shortname is the handle the validator enforces as
// unique (with flow and platform) and the one CI namespaces are built from, so
// it identifies exactly one scenario file. Name lookup remains as a fallback
// for callers that only know the scenario name (the AlwaysGreen gate passes
// both, but an external caller may not).
//
// A scenario that declares nothing gets one blocking smoke leg — today's
// behavior for every scenario. `skip-e2e: true` returns no legs, which GitHub
// Actions treats as a skipped job. Chart versions without a registry, and
// scenarios absent from it, also resolve to the default smoke leg so an
// unrecognised caller keeps running what it runs today rather than losing e2e
// coverage or failing.
//
// Lookup ignores the manifest's `enabled` flag: scenarios invoked directly by
// an external caller are deliberately absent from the generated PR matrix.
func ResolveE2ELegs(repoRoot, chartDir, shortname, scenario string) ([]E2ELeg, error) {
	absChartDir := filepath.Join(repoRoot, "charts", chartDir)
	if !HasRegistry(absChartDir) {
		logging.Logger.Warn().
			Str("chartDir", chartDir).
			Msg("No CI scenario registry — defaulting to a blocking smoke leg")
		return defaultE2ELegs(), nil
	}

	cfg, err := LoadRegistry(absChartDir)
	if err != nil {
		return nil, err
	}

	if shortname != "" {
		for _, scn := range cfg.Integration.Case.PR.Scenarios {
			if scn.Shortname == shortname {
				return scenarioE2ELegs(scn), nil
			}
		}
	}

	for _, scn := range cfg.Integration.Case.PR.Scenarios {
		if scn.Name == scenario {
			if shortname != "" {
				logging.Logger.Warn().
					Str("chartDir", chartDir).
					Str("shortname", shortname).
					Str("scenario", scenario).
					Msg("No scenario matched the shortname — resolved e2e legs by scenario name instead")
			}
			return scenarioE2ELegs(scn), nil
		}
	}

	logging.Logger.Warn().
		Str("chartDir", chartDir).
		Str("shortname", shortname).
		Str("scenario", scenario).
		Msg("Scenario not found in the CI scenario registry — defaulting to a blocking smoke leg")
	return defaultE2ELegs(), nil
}

// E2ELegsJSON marshals legs for the workflow's `matrix.include`. A nil or empty
// slice marshals to "[]" rather than "null", which is what GitHub Actions needs
// to skip the job.
func E2ELegsJSON(legs []E2ELeg) (string, error) {
	if legs == nil {
		legs = []E2ELeg{}
	}
	b, err := json.Marshal(legs)
	if err != nil {
		return "", fmt.Errorf("marshal e2e legs: %w", err)
	}
	return string(b), nil
}

func scenarioE2ELegs(scn CIScenario) []E2ELeg {
	if scn.SkipE2E {
		return nil
	}

	legs := []E2ELeg{newE2ELeg(SuiteSmoke, e2eBlocking(scn.E2ESmokeBlocking, defaultSmokeBlocking))}
	if scn.E2EFullSuite {
		legs = append(legs, newE2ELeg(SuiteFull, e2eBlocking(scn.E2EFullSuiteBlocking, defaultFullBlocking)))
	}
	return legs
}

func defaultE2ELegs() []E2ELeg {
	return []E2ELeg{newE2ELeg(SuiteSmoke, defaultSmokeBlocking)}
}

// newE2ELeg builds a single-shard leg. Sharding is scaffolded in the workflow
// matrix but never fanned out, so both fields are 1.
func newE2ELeg(suite string, blocking bool) E2ELeg {
	return E2ELeg{Suite: suite, Blocking: blocking, ShardIndex: 1, ShardTotal: 1}
}

func e2eBlocking(declared *bool, fallback bool) bool {
	if declared == nil {
		return fallback
	}
	return *declared
}
