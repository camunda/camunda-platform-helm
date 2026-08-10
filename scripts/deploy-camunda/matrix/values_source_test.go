// Copyright 2026 Camunda Services GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
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
	"reflect"
	"testing"
	"time"

	"scripts/camunda-core/pkg/survival"
	"scripts/camunda-core/pkg/versionmatrix"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValuesSourceZeroValueIsNative(t *testing.T) {
	var opts RunOptions
	require.Equal(t, ValuesNative, opts.ValuesSource,
		"the zero value must preserve existing upgrade-minor and upgrade-patch behaviour")
	assert.Empty(t, opts.UpgradeDelta)
	assert.False(t, opts.SuppressUpgradeHooks)
}

func TestValuesSourceConstants(t *testing.T) {
	assert.Equal(t, ValuesSource(""), ValuesNative)
	assert.Equal(t, ValuesSource("carryover"), ValuesCarryover)
	assert.NotEqual(t, ValuesNative, ValuesCarryover)
}

func TestFilterKnownFeaturesPreservesRequestedOrder(t *testing.T) {
	kept, dropped := filterKnownFeatures(
		[]string{"z", "new-in-target", "a"},
		[]string{"a", "z"},
	)
	assert.Equal(t, []string{"z", "a"}, kept,
		"carryover reuses this ordering for Step 2, so it must follow the requested list")
	assert.Equal(t, []string{"new-in-target"}, dropped)
}

func TestUpgradePathFlowClassification(t *testing.T) {
	tests := []struct {
		flow           string
		isUpgrade      bool
		isTwoStep      bool
		isMinorUpgrade bool
		isUpgradeOnly  bool
	}{
		{"install", false, false, false, false},
		{"upgrade-patch", true, true, false, false},
		{"upgrade-minor", true, true, true, false},
		{"modular-upgrade-minor", true, false, true, true},
		{"upgrade-path", true, true, true, false},
	}

	for _, tt := range tests {
		t.Run(tt.flow, func(t *testing.T) {
			assert.Equal(t, tt.isUpgrade, versionmatrix.IsUpgradeFlow(tt.flow), "IsUpgradeFlow")
			assert.Equal(t, tt.isTwoStep, versionmatrix.IsTwoStepUpgradeFlow(tt.flow), "IsTwoStepUpgradeFlow")
			assert.Equal(t, tt.isMinorUpgrade, versionmatrix.IsMinorUpgradeFlow(tt.flow), "IsMinorUpgradeFlow")
			assert.Equal(t, tt.isUpgradeOnly, versionmatrix.IsUpgradeOnlyFlow(tt.flow), "IsUpgradeOnlyFlow")
		})
	}
}

func TestUpgradePathHasNamespaceAbbrev(t *testing.T) {
	abbrev := flowAbbrev("upgrade-path")
	assert.Equal(t, "upth", abbrev)
	seen := map[string]string{}
	for flow, a := range flowAbbrevMap {
		if prev, dup := seen[a]; dup {
			t.Fatalf("abbreviation %q collides between %q and %q", a, prev, flow)
		}
		seen[a] = flow
	}
}

func TestApplyManualFlowAcceptsUpgradePath(t *testing.T) {
	entries := []Entry{{Version: "8.10", Scenario: "elasticsearch"}}

	out, err := applyManualFlow(entries, "upgrade-path")
	require.NoError(t, err)
	require.Len(t, out, 1)
	assert.Equal(t, "upgrade-path", out[0].Flow)

	out, err = applyManualFlow(entries, "install,upgrade-path")
	require.NoError(t, err)
	require.Len(t, out, 2)

	_, err = applyManualFlow(entries, "not-a-flow")
	require.Error(t, err)
}

func TestValidateUpgradeOptions(t *testing.T) {
	delta := filepath.Join(t.TempDir(), "delta.values.yaml")
	require.NoError(t, os.WriteFile(delta, []byte("key: value\n"), 0o644))

	tests := []struct {
		name    string
		opts    RunOptions
		wantErr string
	}{
		{
			name: "defaults are valid",
			opts: RunOptions{},
		},
		{
			name: "carryover without delta is valid",
			opts: RunOptions{ValuesSource: ValuesCarryover},
		},
		{
			name: "carryover with an existing delta is valid",
			opts: RunOptions{ValuesSource: ValuesCarryover, UpgradeDelta: delta},
		},
		{
			name:    "unknown values source",
			opts:    RunOptions{ValuesSource: ValuesSource("bogus")},
			wantErr: "invalid values source",
		},
		{
			name:    "delta without carryover",
			opts:    RunOptions{UpgradeDelta: delta},
			wantErr: "requires --values-source=carryover",
		},
		{
			name:    "delta file missing",
			opts:    RunOptions{ValuesSource: ValuesCarryover, UpgradeDelta: "/nonexistent/delta.yaml"},
			wantErr: "--upgrade-delta",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.opts.ValidateUpgradeOptions()
			if tt.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestUpgradeBudgetDefaultsToUnbudgeted(t *testing.T) {
	var e Entry
	assert.Zero(t, e.UpgradeBudgetMinutes,
		"a scenario without a declared budget must not be treated as having one")
}

func TestUpgradeBudgetCarriesOntoEntry(t *testing.T) {
	e := Entry{Scenario: "s", UpgradeBudgetMinutes: 12, DataProbes: []survival.Probe{{Name: "n"}}}
	assert.Equal(t, 12, e.UpgradeBudgetMinutes)
	assert.Len(t, e.DataProbes, 1)
}

func TestReportUpgradeDurationHandlesBothSides(t *testing.T) {
	// Exercises the branches; the assertion is that neither panics and that an
	// unbudgeted scenario is a distinct path from a budgeted one.
	reportUpgradeDuration(Entry{Scenario: "unbudgeted"}, 3*time.Minute)
	reportUpgradeDuration(Entry{Scenario: "within", UpgradeBudgetMinutes: 10}, 3*time.Minute)
	reportUpgradeDuration(Entry{Scenario: "over", UpgradeBudgetMinutes: 1}, 30*time.Minute)
}

// registryScenario mirrors CIScenario field-for-field. A field added to
// CIScenario but not to registryScenario is silently dropped: the YAML key
// parses into nothing and the feature is inert with no error anywhere. That
// happened to data-probes and cost a cluster run to find, so the mirror is
// asserted rather than trusted to a comment.
func TestRegistryScenarioMirrorsCIScenario(t *testing.T) {
	// Fields on CIScenario that legitimately have no registryScenario
	// counterpart: they come from the manifest entry or are derived.
	fromManifestOrDerived := map[string]bool{
		"Enabled": true, "Shortname": true, "Tier": true, "Flow": true,
		"Dependencies": true, "PreInstall": true, "PostInfra": true, "PostDeploy": true,
	}
	// registryScenario fields that carry IDs resolved into richer CIScenario types.
	idCarriers := map[string]bool{
		"Flows": true, "PreInstallID": true, "PostInfraID": true,
		"PostDeployID": true, "DependencyIDs": true,
		"DataProbes": true, "UpgradeBudgetMinutes": true,
	}

	ci := reflect.TypeOf(CIScenario{})
	rs := reflect.TypeOf(registryScenario{})

	rsFields := map[string]bool{}
	for i := 0; i < rs.NumField(); i++ {
		rsFields[rs.Field(i).Name] = true
	}

	var missing []string
	for i := 0; i < ci.NumField(); i++ {
		name := ci.Field(i).Name
		if fromManifestOrDerived[name] || rsFields[name] {
			continue
		}
		missing = append(missing, name)
	}
	assert.Empty(t, missing,
		"these CIScenario fields have no registryScenario counterpart, so their YAML keys "+
			"parse into nothing: %v", missing)

	var orphaned []string
	ciFields := map[string]bool{}
	for i := 0; i < ci.NumField(); i++ {
		ciFields[ci.Field(i).Name] = true
	}
	for i := 0; i < rs.NumField(); i++ {
		name := rs.Field(i).Name
		if idCarriers[name] || ciFields[name] {
			continue
		}
		orphaned = append(orphaned, name)
	}
	assert.Empty(t, orphaned, "registryScenario fields with no CIScenario destination: %v", orphaned)
}
