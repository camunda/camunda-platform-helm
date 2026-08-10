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

package survival

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// The failure this package exists for: the upgrade succeeds, every pod is
// healthy, and the realm is empty.
func TestCompareCatchesAWipedRealm(t *testing.T) {
	got := Compare(
		Snapshot{"keycloak-users": 25, "process-instances": 200},
		Snapshot{"keycloak-users": 0, "process-instances": 200},
	)

	assert.Equal(t, []Result{
		{Entity: "keycloak-users", Before: 25, After: 0, Verdict: Wiped},
		{Entity: "process-instances", Before: 200, After: 200, Verdict: Preserved},
	}, got)

	losses := Losses(got)
	assert.Len(t, losses, 1)
	assert.Equal(t, "keycloak-users", losses[0].Entity)
}

func TestCompare(t *testing.T) {
	tests := []struct {
		name          string
		before, after Snapshot
		want          []Result
	}{
		{
			name:   "unchanged counts are preserved",
			before: Snapshot{"a": 5}, after: Snapshot{"a": 5},
			want: []Result{{Entity: "a", Before: 5, After: 5, Verdict: Preserved}},
		},
		{
			name:   "a partial drop is a loss",
			before: Snapshot{"a": 10}, after: Snapshot{"a": 3},
			want: []Result{{Entity: "a", Before: 10, After: 3, Verdict: Lost}},
		},
		{
			name:   "dropping to zero is a wipe",
			before: Snapshot{"a": 10}, after: Snapshot{"a": 0},
			want: []Result{{Entity: "a", Before: 10, After: 0, Verdict: Wiped}},
		},
		{
			name:   "growth is preserved, not an anomaly",
			before: Snapshot{"a": 1}, after: Snapshot{"a": 9},
			want: []Result{{Entity: "a", Before: 1, After: 9, Verdict: Preserved}},
		},
		{
			name:   "zero to zero is preserved, not a wipe",
			before: Snapshot{"a": 0}, after: Snapshot{"a": 0},
			want: []Result{{Entity: "a", Before: 0, After: 0, Verdict: Preserved}},
		},
		{
			name:   "probed only before",
			before: Snapshot{"a": 5}, after: Snapshot{},
			want: []Result{{Entity: "a", Before: 5, After: 0, Verdict: NotProbed}},
		},
		{
			name:   "probed only after",
			before: Snapshot{}, after: Snapshot{"a": 5},
			want: []Result{{Entity: "a", Before: 0, After: 5, Verdict: NotProbed}},
		},
		{
			name:   "results are sorted by entity",
			before: Snapshot{"c": 1, "a": 1, "b": 1}, after: Snapshot{"c": 1, "a": 1, "b": 1},
			want: []Result{
				{Entity: "a", Before: 1, After: 1, Verdict: Preserved},
				{Entity: "b", Before: 1, After: 1, Verdict: Preserved},
				{Entity: "c", Before: 1, After: 1, Verdict: Preserved},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, Compare(tt.before, tt.after))
		})
	}
}

func TestNotProbedIsNeverALoss(t *testing.T) {
	got := Compare(Snapshot{"a": 100}, Snapshot{})
	assert.Empty(t, Losses(got),
		"a probe that failed on one side must not be reported as data loss")
}

func TestSummary(t *testing.T) {
	got := Summary(Compare(
		Snapshot{"users": 25, "orphaned": 1},
		Snapshot{"users": 0},
	))
	assert.Equal(t, []string{
		"orphaned: not probed on both sides",
		"users: 25 -> 0 (wiped)",
	}, got)
}

func TestUnknown(t *testing.T) {
	results := []Result{
		{Entity: "ok", Verdict: Preserved},
		{Entity: "missing", Verdict: NotProbed},
	}
	assert.Equal(t, []Result{{Entity: "missing", Verdict: NotProbed}}, Unknown(results))
}

func TestCompareEmpty(t *testing.T) {
	assert.Empty(t, Compare(Snapshot{}, Snapshot{}))
}

type fakeRunner struct {
	out  map[string]string
	fail map[string]error
}

func (f fakeRunner) ExecInPod(_ context.Context, _, selector, _, _ string) (string, error) {
	if err, ok := f.fail[selector]; ok {
		return "", err
	}
	return f.out[selector], nil
}

func TestParseCount(t *testing.T) {
	tests := []struct {
		in      string
		want    int
		wantErr bool
	}{
		{in: "42", want: 42},
		{in: "\n   7 \n\n", want: 7},
		{in: "0", want: 0},
		{in: "", wantErr: true},
		{in: "   \n  \n", wantErr: true},
		{in: "not-a-number", wantErr: true},
	}
	for _, tt := range tests {
		got, err := ParseCount(tt.in)
		if tt.wantErr {
			assert.Error(t, err, "input %q", tt.in)
			continue
		}
		assert.NoError(t, err)
		assert.Equal(t, tt.want, got)
	}
}

func TestRunOmitsFailedProbes(t *testing.T) {
	r := fakeRunner{
		out:  map[string]string{"ok": "12"},
		fail: map[string]error{"broken": assert.AnError},
	}
	snap, errs := Run(context.Background(), r, "ns", []Probe{
		{Name: "good", Selector: "ok", Command: "x"},
		{Name: "bad", Selector: "broken", Command: "x"},
	}, Before)

	assert.Equal(t, Snapshot{"good": 12}, snap)
	assert.Len(t, errs, 1)

	// The failed probe must read as unknown, never as a wipe.
	res := Compare(Snapshot{"good": 12, "bad": 99}, snap)
	assert.Empty(t, Losses(res), "a broken probe must not manufacture data loss")
}

func TestRunRejectsNonNumericOutput(t *testing.T) {
	r := fakeRunner{out: map[string]string{"s": "ERROR: relation does not exist"}}
	snap, errs := Run(context.Background(), r, "ns", []Probe{{Name: "n", Selector: "s", Command: "x"}}, Before)
	assert.Empty(t, snap)
	assert.Len(t, errs, 1)
	assert.Contains(t, errs[0].Error(), "expected a count")
}

func TestProbeAfterOverride(t *testing.T) {
	// The realm moves from the bundled store to the companion during the
	// upgrade, so counting the old location afterwards would report a wipe.
	r := fakeRunner{out: map[string]string{"bundled": "25", "companion": "25"}}
	probes := []Probe{{
		Name:     "keycloak-users",
		Selector: "bundled",
		Command:  "count",
		After:    &ProbeTarget{Selector: "companion", Command: "count"},
	}}

	before, errs := Run(context.Background(), r, "ns", probes, Before)
	assert.Empty(t, errs)
	after, errs := Run(context.Background(), r, "ns", probes, After)
	assert.Empty(t, errs)

	assert.Equal(t, Snapshot{"keycloak-users": 25}, before)
	assert.Equal(t, Snapshot{"keycloak-users": 25}, after)
	assert.Empty(t, Losses(Compare(before, after)),
		"a moved entity must not be reported as lost")
}

func TestProbeWithoutAfterOverrideUsesBaseTarget(t *testing.T) {
	r := fakeRunner{out: map[string]string{"s": "7"}}
	p := []Probe{{Name: "n", Selector: "s", Command: "c"}}
	b, _ := Run(context.Background(), r, "ns", p, Before)
	a, _ := Run(context.Background(), r, "ns", p, After)
	assert.Equal(t, b, a)
}
