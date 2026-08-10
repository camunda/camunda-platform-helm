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
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// BootstrapResult reports what a bootstrap run did.
type BootstrapResult struct {
	Created  []string
	Existing []string
}

// Bootstrap prepares a transition directory for every archetype, writing an
// empty delta for each. An empty delta is the fork-day contract: as of today,
// customers on this path need do nothing.
//
// Every archetype's layers are resolved against the source chart first, so a
// layer renamed between versions fails here rather than midway through a run.
func Bootstrap(repoRoot, from, to string, archetypes []string) (BootstrapResult, error) {
	var res BootstrapResult

	// The CI matrix discovers transitions from this directory, so bootstrapping
	// a pair whose target chart does not exist yet would add a job that can
	// never render. Bootstrap belongs at fork time, once the chart is cut.
	for _, v := range []string{from, to} {
		if _, err := os.Stat(ChartDir(repoRoot, v)); err != nil {
			return res, fmt.Errorf("chart %s does not exist yet (%s); bootstrap a transition only "+
				"once both charts are cut, or CI will discover a job it cannot run",
				v, ChartDir(repoRoot, v))
		}
	}

	// Validate every archetype before writing anything: a failure partway
	// through would leave some paths bootstrapped and others not.
	loaded := make([]Archetype, 0, len(archetypes))
	vd := valuesDir(repoRoot, from)
	for _, name := range archetypes {
		a, err := LoadArchetype(repoRoot, name)
		if err != nil {
			return res, err
		}
		for _, layer := range a.Layers {
			if _, err := os.Stat(filepath.Join(vd, layer)); err != nil {
				return res, fmt.Errorf(
					"archetype %s: layer %q missing in chart %s; the layer was renamed or removed "+
						"and the archetype needs updating before %s-to-%s can run",
					name, layer, from, from, to)
			}
		}
		loaded = append(loaded, a)
	}

	for _, a := range loaded {
		dir := TransitionDir(repoRoot, from, to, a.Name)
		delta := filepath.Join(dir, "delta.values.yaml")
		if fileExists(delta) {
			res.Existing = append(res.Existing, delta)
			continue
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return res, fmt.Errorf("create %s: %w", dir, err)
		}
		if err := os.WriteFile(delta, []byte(emptyDelta(a.Name, from, to)), 0o644); err != nil {
			return res, fmt.Errorf("write %s: %w", delta, err)
		}
		res.Created = append(res.Created, delta)
	}

	return res, nil
}

func emptyDelta(archetype, from, to string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Delta for %s, %s-to-%s\n", archetype, from, to)
	b.WriteString(`#
# Empty means: a customer on this path upgrades with no values changes. Every
# entry added here is a change the customer MUST make, and is emitted into the
# upgrade guide, so keep it minimal and justified.
#
# Directives:
#   $rename        moves a value to a new path, preserving it. Use where the
#                  value is environment-specific and cannot be restated.
#   $remove        deletes dotted paths. Chart constraints test key presence,
#                  so these cannot be cleared by setting them to null.
#   $scaffolding   harness-only values, applied but excluded from the change
#                  list. For CI fixtures that no customer has.
#
# Remaining keys merge in as normal values.
`)
	return b.String()
}
