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

// Package catalogue models deploy-camunda Recipes and the catalogues that
// supply them.
package catalogue

const (
	APIVersion = "deploy-camunda/v1"
	KindRecipe = "Recipe"
)

type Recipe struct {
	APIVersion string         `yaml:"apiVersion"`
	Kind       string         `yaml:"kind"`
	Metadata   RecipeMetadata `yaml:"metadata"`
	Spec       RecipeSpec     `yaml:"spec"`
}

type RecipeMetadata struct {
	Name        string `yaml:"name"`
	Catalogue   string `yaml:"catalogue"`
	Description string `yaml:"description,omitempty"`
}

type RecipeSpec struct {
	Chart     ChartRef          `yaml:"chart"`
	Values    []string          `yaml:"values,omitempty"`
	Requires  []SlotConstraint  `yaml:"requires,omitempty"`
	Providers map[string]string `yaml:"providers,omitempty"`
}

type ChartRef struct {
	Version string `yaml:"version"`
	Path    string `yaml:"path"`
}

// SlotConstraint restricts a slot to the providers listed in OneOf.
type SlotConstraint struct {
	Slot  string   `yaml:"slot"`
	OneOf []string `yaml:"oneOf"`
}
