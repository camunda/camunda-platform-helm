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

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// Outcome is the result of the A/B state machine for one path.
type Outcome string

const (
	// OutcomeClean: neither run failed.
	OutcomeClean Outcome = "CLEAN"
	// OutcomeRemediated: Run A failed, Run B succeeded.
	OutcomeRemediated Outcome = "REMEDIATED"
	// OutcomeUnremediated: both runs failed.
	OutcomeUnremediated Outcome = "UNREMEDIATED"
	// OutcomeStaleDelta: Run A succeeded, Run B failed.
	OutcomeStaleDelta Outcome = "STALE_DELTA"
)

// Class buckets a finding by what it means for the customer.
type Class string

const (
	ClassBlocks   Class = "BLOCKS"
	ClassDestroys Class = "DESTROYS"
	ClassChanges  Class = "CHANGES"
	ClassNoise    Class = "NOISE"
)

// Finding is one reportable item.
type Finding struct {
	ID          string   `json:"id"`
	Transition  string   `json:"transition"`
	Path        string   `json:"path"`
	Detector    string   `json:"detector"`
	Class       Class    `json:"class"`
	Severity    string   `json:"severity"`
	Title       string   `json:"title"`
	Evidence    Evidence `json:"evidence"`
	Remedy      Remedy   `json:"remedy"`
	Confidence  string   `json:"confidence"`
	Fingerprint string   `json:"fingerprint"`
}

type Evidence struct {
	Stage  string `json:"stage"`
	Error  string `json:"error,omitempty"`
	Source string `json:"source,omitempty"`
}

type Remedy struct {
	Kind      string `json:"kind"`
	Available bool   `json:"available"`
	OutOfBand bool   `json:"outOfBand"`
	Diff      string `json:"diff,omitempty"`
}

// PathResult is the full A/B result for one archetype in one transition.
type PathResult struct {
	Transition   string           `json:"transition"`
	Path         string           `json:"path"`
	Outcome      Outcome          `json:"outcome"`
	RunA         RenderResult     `json:"runA"`
	RunB         RenderResult     `json:"runB"`
	HasDelta     bool             `json:"hasDelta"`
	HasRemedy    bool             `json:"hasRemedy"`
	Findings     []Finding        `json:"findings"`
	Discovery    *DiscoveryResult `json:"discovery,omitempty"`
	DocsCoverage DocsCoverage     `json:"docsCoverage"`
	// ScaffoldingKeys are top-level delta keys that are harness-only.
	ScaffoldingKeys []string `json:"scaffoldingKeys,omitempty"`
}

// Classify applies the A/B state machine.
func Classify(a, b RenderResult) Outcome {
	switch {
	case a.Succeeded && b.Succeeded:
		return OutcomeClean
	case !a.Succeeded && b.Succeeded:
		return OutcomeRemediated
	case !a.Succeeded && !b.Succeeded:
		return OutcomeUnremediated
	default:
		return OutcomeStaleDelta
	}
}

// Fingerprint produces a stable ID for de-duplicating a failure across runs.
// Derived from the archetype and error title only, never from stderr.
func Fingerprint(path, title string) string {
	sum := sha256.Sum256([]byte(path + "\x00" + title))
	return hex.EncodeToString(sum[:])[:12]
}

// BuildFindings turns an A/B result into reportable findings. Only failures
// produce findings; a clean path appears in the summary matrix only.
func BuildFindings(t Transition, a, b RenderResult, outcome Outcome) []Finding {
	transition := fmt.Sprintf("%s-to-%s", t.From, t.To)
	path := t.Archetype.Name

	switch outcome {
	case OutcomeClean:
		return nil

	case OutcomeRemediated:
		sig := Signature(a.Stderr)
		return []Finding{{
			ID:         findingID(transition, path, sig.Title),
			Transition: transition,
			Path:       path,
			Detector:   "upgrade-path-render",
			Class:      ClassBlocks,
			Severity:   "high",
			Title:      sig.Title,
			Evidence:   Evidence{Stage: "render", Error: sig.Raw, Source: sig.Source},
			Remedy: Remedy{
				Kind:      "values-delta",
				Available: true,
				OutOfBand: t.RemedyPath != "",
				Diff:      t.DeltaPath,
			},
			Confidence:  "confirmed",
			Fingerprint: Fingerprint(path, sig.Title),
		}}

	case OutcomeUnremediated:
		sig := Signature(b.Stderr)
		return []Finding{{
			ID:         findingID(transition, path, sig.Title),
			Transition: transition,
			Path:       path,
			Detector:   "upgrade-path-render",
			Class:      ClassBlocks,
			Severity:   "critical",
			Title:      sig.Title,
			Evidence:   Evidence{Stage: "render", Error: sig.Raw, Source: sig.Source},
			Remedy: Remedy{
				Kind:      "none",
				Available: false,
				OutOfBand: false,
			},
			Confidence:  "confirmed",
			Fingerprint: Fingerprint(path, sig.Title),
		}}

	default: // OutcomeStaleDelta
		sig := Signature(b.Stderr)
		return []Finding{{
			ID:         findingID(transition, path, "stale-delta"),
			Transition: transition,
			Path:       path,
			Detector:   "upgrade-path-render",
			Class:      ClassNoise,
			Severity:   "medium",
			Title:      "Delta is stale: baseline renders but baseline+delta fails",
			Evidence:   Evidence{Stage: "render", Error: sig.Raw, Source: sig.Source},
			Remedy: Remedy{
				Kind:      "fix-fixture",
				Available: false,
			},
			Confidence:  "confirmed",
			Fingerprint: Fingerprint(path, "stale-delta"),
		}}
	}
}

func findingID(transition, path, title string) string {
	return fmt.Sprintf("%s/%s/%s", transition, path, slug(title))
}

func slug(s string) string {
	s = strings.ToLower(collapseSpace(s))
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '-' || r == '_':
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	for strings.Contains(out, "--") {
		out = strings.ReplaceAll(out, "--", "-")
	}
	if len(out) > 60 {
		out = strings.Trim(out[:60], "-")
	}
	if out == "" {
		out = "unknown"
	}
	return out
}
