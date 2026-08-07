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
	"context"
	"fmt"
	"path/filepath"
	"regexp"

	"scripts/camunda-core/pkg/chartvalues"
)

var (
	reKeyRemoved = regexp.MustCompile(`The Helm values file key \"([^\"]+)\" has been removed`)
	reKeyRenamed = regexp.MustCompile(`The Helm values file key changed from \"([^\"]+)\" to \"([^\"]+)\"`)
)

// KeyChange is one values key the customer must act on.
type KeyChange struct {
	Old  string `json:"old"`
	New  string `json:"new,omitempty"` // empty for a pure removal
	Kind string `json:"kind"`          // "removed" | "renamed"
}

// DiscoveryResult is the outcome of iterating a path to a fixpoint.
type DiscoveryResult struct {
	Changes []KeyChange `json:"changes"`
	// Final is the render result after every discovered key was deleted.
	Final RenderResult `json:"final"`
	// Residual is the error remaining once deletion stops making progress.
	Residual string `json:"residual,omitempty"`
	// Truncated reports that the loop hit maxDiscoveryRounds, so Changes is
	// incomplete.
	Truncated bool `json:"truncated"`
}

const maxDiscoveryRounds = 50

// Discover renders a path repeatedly, deleting each removed or renamed key it
// reports, until the render succeeds or fails for another reason. Renames are
// recorded but not applied: the replacement value cannot be inferred.
func Discover(ctx context.Context, r Renderer, t Transition, repoRoot, workDir string) (DiscoveryResult, error) {
	var out DiscoveryResult
	seen := map[string]bool{}

	base := append([]string{}, t.BaselineLayers...)
	if t.DeltaPath != "" {
		base = append(base, t.DeltaPath)
	}

	// Guards test key presence, so keys must be deleted from a consolidated
	// document rather than nulled in an overlay.
	merged, err := chartvalues.MergeFiles(base)
	if err != nil {
		return out, err
	}

	for round := 0; round < maxDiscoveryRounds; round++ {
		p := filepath.Join(workDir, fmt.Sprintf("consolidated-%s-%d.yaml", t.Archetype.Name, round))
		if err := chartvalues.WriteFile(p, merged); err != nil {
			return out, err
		}

		res := Render(ctx, r, Transition{
			From: t.From, To: t.To, Archetype: t.Archetype, BaselineLayers: []string{p},
		}, RunA, repoRoot, workDir)

		if res.Succeeded {
			out.Final = res
			return out, nil
		}

		change, ok := parseKeyChange(res.Stderr)
		if !ok {
			out.Final = res
			out.Residual = Signature(res.Stderr).Title
			return out, nil
		}
		if seen[change.Old] {
			out.Final = res
			out.Residual = fmt.Sprintf("guard for %q did not clear after deletion", change.Old)
			return out, nil
		}

		seen[change.Old] = true
		out.Changes = append(out.Changes, change)

		if !chartvalues.DeleteKey(merged, change.Old) {
			out.Final = res
			out.Residual = fmt.Sprintf("guard names %q but it is absent from the merged values; "+
				"the condition is not simple key presence", change.Old)
			return out, nil
		}
	}

	out.Truncated = true
	out.Residual = fmt.Sprintf("stopped after %d rounds; more keys may remain", maxDiscoveryRounds)
	return out, nil
}

// parseKeyChange extracts a removed or renamed key from render stderr.
func parseKeyChange(stderr string) (KeyChange, bool) {
	if m := reKeyRenamed.FindStringSubmatch(stderr); m != nil {
		return KeyChange{Old: m[1], New: m[2], Kind: "renamed"}, true
	}
	if m := reKeyRemoved.FindStringSubmatch(stderr); m != nil {
		return KeyChange{Old: m[1], Kind: "removed"}, true
	}
	return KeyChange{}, false
}
