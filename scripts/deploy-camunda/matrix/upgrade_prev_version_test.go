// Licensed to Camunda Services GmbH under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Camunda licenses this file to you under the Apache License,
// Version 2.0; you may not use this file except in compliance with
// the License. You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package matrix

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"scripts/camunda-core/pkg/scenarios"
	"scripts/camunda-core/pkg/versionmatrix"
)

// TestUpgradeMinorPrevVersionDimensions asserts that every upgrade-minor
// scenario's identity, persistence, and platform are each backed by a values
// file in the PREVIOUS app version's scenario dir. Step 1 of an upgrade-minor
// deploy installs the previous version using its own values files, so a
// dimension absent there fails validation at deploy time (nightly CI), not at
// PR time. This test moves that failure to PR time with an actionable message.
//
// Features are intentionally NOT checked: the runner drops target-only features
// from Step 1 (filterKnownFeatures in runner_upgrade.go), so a feature that
// exists only in the target version is valid — it is applied in Step 2.
func TestUpgradeMinorPrevVersionDimensions(t *testing.T) {
	repoRoot := findRepoRoot(t)
	chartsDir := filepath.Join(repoRoot, "charts")
	entries, err := os.ReadDir(chartsDir)
	if err != nil {
		t.Fatalf("read charts dir: %v", err)
	}

	const prefix = "camunda-platform-"
	checked := false
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), prefix) {
			continue
		}
		version := e.Name()[len(prefix):]
		chartDir := filepath.Join(chartsDir, e.Name())
		if !HasRegistry(chartDir) {
			continue
		}
		prevVersion, err := versionmatrix.PreviousAppVersion(version)
		if err != nil {
			continue
		}
		prevScenarioDir := filepath.Join(chartsDir, prefix+prevVersion, "test/integration/scenarios/chart-full-setup")
		if _, statErr := os.Stat(prevScenarioDir); statErr != nil {
			continue
		}

		cfg, err := LoadRegistry(chartDir)
		if err != nil {
			t.Fatalf("LoadRegistry(%s): %v", version, err)
		}

		ids := mustListPrev(t, prevScenarioDir, scenarios.ListIdentities)
		pers := mustListPrev(t, prevScenarioDir, scenarios.ListPersistence)
		plats := mustListPrev(t, prevScenarioDir, scenarios.ListPlatforms)

		for _, s := range cfg.Integration.Case.PR.Scenarios {
			// Flow is comma-joined at the registry level (e.g. "install,upgrade-minor");
			// match the exact upgrade-minor token, excluding modular-upgrade-minor,
			// which routes to the upgrade-only path with no prev-version install.
			if !slices.Contains(strings.Split(s.Flow, ","), "upgrade-minor") {
				continue
			}
			checked = true
			t.Run(version+"/"+s.Name, func(t *testing.T) {
				if s.Identity != "" && !slices.Contains(ids, s.Identity) {
					t.Errorf("upgrade-minor scenario %q: identity %q has no values file in previous version %s (available: %v). "+
						"Step 1 install of the previous version would fail validation. "+
						"Backport the identity to %s or restrict the scenario to flows: [install].",
						s.Name, s.Identity, prevVersion, ids, prevVersion)
				}
				if s.Persistence != "" && !slices.Contains(pers, s.Persistence) {
					t.Errorf("upgrade-minor scenario %q: persistence %q has no values file in previous version %s (available: %v). "+
						"Step 1 install of the previous version would fail validation. "+
						"Backport the persistence layer to %s or restrict the scenario to flows: [install].",
						s.Name, s.Persistence, prevVersion, pers, prevVersion)
				}
				for _, p := range s.Platforms {
					if !slices.Contains(plats, p) {
						t.Errorf("upgrade-minor scenario %q: platform %q has no values file in previous version %s (available: %v). "+
							"Step 1 install of the previous version would fail validation.",
							s.Name, p, prevVersion, plats)
					}
				}
			})
		}
	}
	if !checked {
		t.Skip("no upgrade-minor scenarios found in any registry")
	}
}

// TestUpgradeMinorFeatureFilter_ReproAndFix drives scenarios.BuildDeploymentConfig
// — the exact function whose validation aborted the nightly upgrade-minor run —
// against the 8.9 (previous) scenario dir. It asserts the original failure
// reproduces with the unfiltered feature and disappears after filterKnownFeatures.
func TestUpgradeMinorFeatureFilter_ReproAndFix(t *testing.T) {
	repoRoot := findRepoRoot(t)
	prevDir := filepath.Join(repoRoot, "charts", "camunda-platform-8.9", "test/integration/scenarios/chart-full-setup")
	if _, err := os.Stat(prevDir); err != nil {
		t.Skipf("previous version scenario dir absent: %v", err)
	}

	// Reproduce: component-persistence's target-only feature fails against 8.9.
	_, err := scenarios.BuildDeploymentConfig(prevDir, "component-persistence", scenarios.BuilderOverrides{
		Features: []string{"persistence"},
	})
	if err == nil {
		t.Fatal("expected validation error for unfiltered [persistence] against 8.9, got nil")
	}
	if !strings.Contains(err.Error(), `invalid --features value "persistence"`) {
		t.Fatalf("expected invalid-features error, got: %v", err)
	}

	// Fix: after the runner's filter, the feature is gone and the build succeeds.
	prevFeatures, ferr := scenarios.ListFeatures(prevDir)
	if ferr != nil {
		t.Fatalf("list features: %v", ferr)
	}
	kept, dropped := filterKnownFeatures([]string{"persistence"}, prevFeatures)
	if !slices.Equal(dropped, []string{"persistence"}) {
		t.Fatalf("expected persistence dropped, got kept=%v dropped=%v", kept, dropped)
	}
	if _, err := scenarios.BuildDeploymentConfig(prevDir, "component-persistence", scenarios.BuilderOverrides{
		Features: kept,
	}); err != nil {
		t.Fatalf("build after filter should succeed, got: %v", err)
	}
}

func TestFilterKnownFeatures(t *testing.T) {
	want := []string{"postgresql-companion", "documentstore", "persistence"}
	available := []string{"rba", "documentstore", "multitenancy"}

	kept, dropped := filterKnownFeatures(want, available)

	if !slices.Equal(kept, []string{"documentstore"}) {
		t.Errorf("kept = %v, want [documentstore]", kept)
	}
	if !slices.Equal(dropped, []string{"postgresql-companion", "persistence"}) {
		t.Errorf("dropped = %v, want [postgresql-companion persistence]", dropped)
	}
	if !slices.Equal(want, []string{"postgresql-companion", "documentstore", "persistence"}) {
		t.Errorf("want slice mutated: %v", want)
	}
}

func mustListPrev(t *testing.T, dir string, fn func(string) ([]string, error)) []string {
	t.Helper()
	got, err := fn(dir)
	if err != nil {
		t.Fatalf("list values in %s: %v", dir, err)
	}
	return got
}
