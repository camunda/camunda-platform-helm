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

// Package survival compares entity counts taken before and after an upgrade.
//
// An upgrade that completes is not an upgrade that preserved anything. Removing
// a subchart can strand the storage its replacement never reads, leaving a
// healthy cluster with an empty realm — a state indistinguishable from success
// unless something counted first.
package survival

import (
	"fmt"
	"sort"
)

// Snapshot maps an entity name to how many of it existed.
type Snapshot map[string]int

// Verdict classifies one entity's change across an upgrade.
type Verdict string

const (
	// Preserved: the count did not fall.
	Preserved Verdict = "preserved"
	// Lost: the count fell but is not zero.
	Lost Verdict = "lost"
	// Wiped: the count fell to zero from a non-zero start.
	Wiped Verdict = "wiped"
	// NotProbed: the entity was counted on only one side, so nothing can be said.
	NotProbed Verdict = "not-probed"
)

// Result is one entity's outcome.
type Result struct {
	Entity  string  `json:"entity"`
	Before  int     `json:"before"`
	After   int     `json:"after"`
	Verdict Verdict `json:"verdict"`
}

// Lost reports whether this result represents data loss.
func (r Result) IsLoss() bool {
	return r.Verdict == Lost || r.Verdict == Wiped
}

// Compare pairs the two snapshots, sorted by entity name.
//
// A count that rises is Preserved rather than an anomaly: an upgrade may
// legitimately create records, and treating growth as failure would make the
// check unusable on a live system.
func Compare(before, after Snapshot) []Result {
	seen := map[string]bool{}
	for k := range before {
		seen[k] = true
	}
	for k := range after {
		seen[k] = true
	}

	names := make([]string, 0, len(seen))
	for k := range seen {
		names = append(names, k)
	}
	sort.Strings(names)

	out := make([]Result, 0, len(names))
	for _, n := range names {
		b, okB := before[n]
		a, okA := after[n]
		r := Result{Entity: n, Before: b, After: a}
		switch {
		case !okB || !okA:
			r.Verdict = NotProbed
		case a >= b:
			r.Verdict = Preserved
		case a == 0:
			r.Verdict = Wiped
		default:
			r.Verdict = Lost
		}
		out = append(out, r)
	}
	return out
}

// Losses filters to entities that lost data.
func Losses(rs []Result) []Result {
	var out []Result
	for _, r := range rs {
		if r.IsLoss() {
			out = append(out, r)
		}
	}
	return out
}

// Unknown filters to entities that were not successfully probed on both sides.
func Unknown(rs []Result) []Result {
	var out []Result
	for _, r := range rs {
		if r.Verdict == NotProbed {
			out = append(out, r)
		}
	}
	return out
}

// Summary renders a one-line description of each result.
func Summary(rs []Result) []string {
	out := make([]string, 0, len(rs))
	for _, r := range rs {
		switch r.Verdict {
		case NotProbed:
			out = append(out, fmt.Sprintf("%s: not probed on both sides", r.Entity))
		default:
			out = append(out, fmt.Sprintf("%s: %d -> %d (%s)", r.Entity, r.Before, r.After, r.Verdict))
		}
	}
	return out
}
