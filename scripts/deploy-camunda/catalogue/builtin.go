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

package catalogue

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"scripts/deploy-camunda/matrix"
)

const chartDirPrefix = "camunda-platform-"

// BuiltinCatalogueName returns the catalogue name for a chart version.
func BuiltinCatalogueName(version string) string {
	return "builtin/" + version
}

// LoadBuiltin maps the scenario registry of every chart under
// <repoRoot>/charts onto Recipes, ordered by chart directory name and then
// manifest order. Flow fan-out entries of one manifest row collapse into a
// single Recipe.
func LoadBuiltin(repoRoot string) ([]Recipe, error) {
	chartsDir := filepath.Join(repoRoot, "charts")
	entries, err := os.ReadDir(chartsDir)
	if err != nil {
		return nil, fmt.Errorf("read charts dir: %w", err)
	}
	var recipes []Recipe
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), chartDirPrefix) {
			continue
		}
		chartDir := filepath.Join(chartsDir, e.Name())
		if !matrix.HasRegistry(chartDir) {
			continue
		}
		version := strings.TrimPrefix(e.Name(), chartDirPrefix)
		rs, err := LoadBuiltinVersion(chartDir, version, filepath.ToSlash(filepath.Join("charts", e.Name())))
		if err != nil {
			return nil, err
		}
		recipes = append(recipes, rs...)
	}
	return recipes, nil
}

type manifestRow struct {
	ID string `yaml:"id"`
}

type registryManifest struct {
	Integration struct {
		Scenarios []manifestRow `yaml:"scenarios"`
	} `yaml:"integration"`
}

type scenarioFlows struct {
	Flows []string `yaml:"flows"`
}

// LoadBuiltinVersion maps one chart's scenario registry onto Recipes, one
// per manifest row. matrix.LoadRegistry fans each row out into one
// CIScenario per flow, so rows are re-identified by their flow count.
func LoadBuiltinVersion(chartDir, version, chartPath string) ([]Recipe, error) {
	cfg, err := matrix.LoadRegistry(chartDir)
	if err != nil {
		return nil, fmt.Errorf("load registry %s: %w", version, err)
	}
	registryDir := filepath.Join(chartDir, "test", matrix.RegistryDirName)
	var manifest registryManifest
	if err := readYAML(filepath.Join(registryDir, "manifest.yaml"), &manifest); err != nil {
		return nil, err
	}
	chart := ChartRef{Version: version, Path: chartPath}
	scenarios := cfg.Integration.Case.PR.Scenarios
	recipes := make([]Recipe, 0, len(manifest.Integration.Scenarios))
	next := 0
	for _, row := range manifest.Integration.Scenarios {
		var sf scenarioFlows
		if err := readYAML(filepath.Join(registryDir, "scenarios", row.ID+".yaml"), &sf); err != nil {
			return nil, err
		}
		width := max(len(sf.Flows), 1)
		if next+width > len(scenarios) {
			return nil, fmt.Errorf("registry %s: scenario %q has no loaded entry", version, row.ID)
		}
		group := scenarios[next : next+width]
		next += width
		flows := make([]string, 0, width)
		for _, s := range group {
			flows = append(flows, s.Flow)
		}
		recipes = append(recipes, toRecipe(row.ID, group[0], flows, chart))
	}
	if next != len(scenarios) {
		return nil, fmt.Errorf("registry %s: %d loaded entries not mapped to a manifest row", version, len(scenarios)-next)
	}
	return recipes, nil
}

func readYAML(path string, out any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if err := yaml.Unmarshal(data, out); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	return nil
}

func toRecipe(id string, s matrix.CIScenario, flows []string, chart ChartRef) Recipe {
	return Recipe{
		APIVersion: APIVersion,
		Kind:       KindRecipe,
		Metadata: RecipeMetadata{
			Name:        id,
			Catalogue:   BuiltinCatalogueName(chart.Version),
			Description: fmt.Sprintf("registry scenario %s (shortname %s, tier %d, enabled %t)", s.Name, s.Shortname, s.Tier, s.Enabled),
		},
		Spec: RecipeSpec{
			Chart:     chart,
			Values:    valueRefs(s),
			Requires:  requires(s, flows),
			Providers: providers(s),
		},
	}
}

func valueRefs(s matrix.CIScenario) []string {
	var refs []string
	add := func(kind, v string) {
		if v != "" {
			refs = append(refs, kind+":"+v)
		}
	}
	add("auth", s.Auth)
	add("identity", s.Identity)
	add("persistence", s.Persistence)
	for _, f := range s.Features {
		add("feature", f)
	}
	for _, f := range s.ExtraValues {
		add("file", f)
	}
	return refs
}

func requires(s matrix.CIScenario, flows []string) []SlotConstraint {
	var out []SlotConstraint
	if len(s.Platforms) > 0 {
		out = append(out, SlotConstraint{Slot: "platform", OneOf: append([]string(nil), s.Platforms...)})
	}
	var fs []string
	for _, f := range flows {
		if f != "" {
			fs = append(fs, f)
		}
	}
	if len(fs) > 0 {
		out = append(out, SlotConstraint{Slot: "flow", OneOf: fs})
	}
	return out
}

func providers(s matrix.CIScenario) map[string]string {
	if len(s.InfraType) == 0 {
		return nil
	}
	out := make(map[string]string, len(s.InfraType))
	for platform, infra := range s.InfraType {
		out["infra."+platform] = infra
	}
	return out
}
