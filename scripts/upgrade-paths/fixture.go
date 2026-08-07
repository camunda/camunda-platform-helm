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
	"sort"

	"gopkg.in/yaml.v3"

	"scripts/camunda-core/pkg/chartvalues"
)

// Archetype describes a shape of customer deployment, independent of any
// particular version transition. Layers name values files in the source
// chart's chart-full-setup/values directory; the baseline is composed from
// them rather than checked in.
type Archetype struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Tier        int      `yaml:"tier"`
	Platforms   []string `yaml:"platforms"`
	// Layers are paths relative to the source chart's values dir, applied in order.
	Layers []string `yaml:"layers"`
}

// Transition is one (from, to) version pair for one archetype.
type Transition struct {
	From      string
	To        string
	Archetype Archetype

	// BaselineLayers are absolute paths to the source chart's values layers.
	BaselineLayers []string
	// DeltaPath is the absolute path to delta.values.yaml, or "" when absent or empty.
	DeltaPath string
	// RemedyPath is the absolute path to remedy.sh, or "" when none.
	RemedyPath string
	// Delta is the parsed delta, empty when DeltaPath is unset.
	Delta chartvalues.Delta
}

// TransitionDir is the on-disk home of a (from,to,archetype) fixture.
func TransitionDir(repoRoot, from, to, archetype string) string {
	return filepath.Join(repoRoot, "test", "upgrade-paths", "transitions",
		fmt.Sprintf("%s-to-%s", from, to), archetype)
}

// ArchetypeDir is the on-disk home of an archetype definition.
func ArchetypeDir(repoRoot, name string) string {
	return filepath.Join(repoRoot, "test", "upgrade-paths", "archetypes", name)
}

// ChartDir returns the chart directory for a given app version.
func ChartDir(repoRoot, version string) string {
	return filepath.Join(repoRoot, "charts", "camunda-platform-"+version)
}

// valuesDir returns a chart's scenario values directory, the source of baseline layers.
func valuesDir(repoRoot, version string) string {
	return filepath.Join(ChartDir(repoRoot, version),
		"test", "integration", "scenarios", "chart-full-setup", "values")
}

// LoadArchetype reads and validates an archetype definition.
func LoadArchetype(repoRoot, name string) (Archetype, error) {
	var a Archetype
	p := filepath.Join(ArchetypeDir(repoRoot, name), "archetype.yaml")
	b, err := os.ReadFile(p)
	if err != nil {
		return a, fmt.Errorf("read archetype %s: %w", name, err)
	}
	if err := yaml.Unmarshal(b, &a); err != nil {
		return a, fmt.Errorf("parse archetype %s: %w", name, err)
	}
	if a.Name == "" {
		return a, fmt.Errorf("archetype %s: name is required", name)
	}
	if a.Name != name {
		return a, fmt.Errorf("archetype %s: name field %q does not match directory", name, a.Name)
	}
	if len(a.Layers) == 0 {
		return a, fmt.Errorf("archetype %s: at least one values layer is required", name)
	}
	return a, nil
}

// ListArchetypes returns all archetype names, sorted.
func ListArchetypes(repoRoot string) ([]string, error) {
	root := filepath.Join(repoRoot, "test", "upgrade-paths", "archetypes")
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("list archetypes: %w", err)
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

// LoadTransition resolves an archetype against a version pair. A baseline
// layer missing from the source chart is an error, not a skip.
func LoadTransition(repoRoot, from, to, archetypeName string) (Transition, error) {
	t := Transition{From: from, To: to}

	a, err := LoadArchetype(repoRoot, archetypeName)
	if err != nil {
		return t, err
	}
	t.Archetype = a

	vd := valuesDir(repoRoot, from)
	if _, err := os.Stat(vd); err != nil {
		return t, fmt.Errorf("source chart values dir not found for %s: %w", from, err)
	}
	for _, layer := range a.Layers {
		p := filepath.Join(vd, layer)
		if _, err := os.Stat(p); err != nil {
			return t, fmt.Errorf("archetype %s: layer %q missing in chart %s (%s)",
				archetypeName, layer, from, p)
		}
		t.BaselineLayers = append(t.BaselineLayers, p)
	}

	td := TransitionDir(repoRoot, from, to, archetypeName)
	if p := filepath.Join(td, "delta.values.yaml"); fileHasContent(p) {
		t.DeltaPath = p
		d, err := chartvalues.LoadDelta(p)
		if err != nil {
			return t, err
		}
		t.Delta = d
	}
	if p := filepath.Join(td, "remedy.sh"); fileExists(p) {
		t.RemedyPath = p
	}

	return t, nil
}

// fileHasContent reports whether a path exists and holds more than whitespace.
// Empty and absent are treated identically.
func fileHasContent(p string) bool {
	b, err := os.ReadFile(p)
	if err != nil {
		return false
	}
	for _, c := range b {
		if c != ' ' && c != '\t' && c != '\n' && c != '\r' {
			return true
		}
	}
	return false
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
