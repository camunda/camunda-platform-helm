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

	"scripts/camunda-core/pkg/valuesinjector"
)

// runInjectValues overrides component image tags in a chart's values.yaml in
// place, from the *_IMAGE_TAG environment variables. CHART_VERSION selects the
// chart (and the version-gated component set).
//
//	CHART_VERSION=8.10 ORCHESTRATION_IMAGE_TAG=... release-tools inject-values
//
// Image-tag inputs (set only the ones to override):
//
//	CONSOLE_IMAGE_TAG, ZEEBE_IMAGE_TAG, ZEEBE_GATEWAY_IMAGE_TAG, OPERATE_IMAGE_TAG,
//	TASKLIST_IMAGE_TAG, OPTIMIZE_IMAGE_TAG, IDENTITY_IMAGE_TAG, WEB_MODELER_IMAGE_TAG,
//	CONNECTORS_IMAGE_TAG (8.6/8.7) plus ORCHESTRATION_IMAGE_TAG (8.8+).
func runInjectValues(args []string) error {
	chartVersion := os.Getenv("CHART_VERSION")
	if chartVersion == "" {
		return fmt.Errorf("CHART_VERSION environment variable is required")
	}
	validVersions := map[string]bool{"8.6": true, "8.7": true, "8.8": true, "8.9": true, "8.10": true}
	if !validVersions[chartVersion] {
		return fmt.Errorf("unsupported CHART_VERSION: %s (supported: 8.6, 8.7, 8.8, 8.9, 8.10)", chartVersion)
	}

	chartDir := fmt.Sprintf("charts/camunda-platform-%s", chartVersion)
	for _, name := range []string{"values.yaml", "values-latest.yaml"} {
		file := filepath.Join(chartDir, name)
		content, err := os.ReadFile(file)
		if name != "values.yaml" && os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("read %s: %w", file, err)
		}
		result, err := mergeImageTags(chartVersion, string(content), name != "values.yaml")
		if err != nil {
			return fmt.Errorf("merge image tags into %s: %w", file, err)
		}
		if err := os.WriteFile(file, []byte(result), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", file, err)
		}
		fmt.Printf("Successfully updated %s\n", file)
	}
	return nil
}

func mergeImageTags(chartVersion, content string, presentOnly bool) (string, error) {
	switch chartVersion {
	case "8.6", "8.7":
		o := buildOverridesClassic()
		if presentOnly {
			dropMissing(content, map[string]**valuesinjector.ComponentImage{
				"console": &o.Console, "zeebe": &o.Zeebe, "zeebeGateway": &o.ZeebeGateway,
				"operate": &o.Operate, "tasklist": &o.Tasklist, "optimize": &o.Optimize,
				"identity": &o.Identity, "webModeler": &o.WebModeler, "connectors": &o.Connectors,
			})
		}
		if chartVersion == "8.6" {
			return valuesinjector.MergeImageTags86(content, o)
		}
		return valuesinjector.MergeImageTags87(content, o)
	}
	o := buildOverridesOrchestration()
	if presentOnly {
		dropMissing(content, map[string]**valuesinjector.ComponentImage{
			"identity": &o.Identity, "console": &o.Console, "webModeler": &o.WebModeler,
			"connectors": &o.Connectors, "orchestration": &o.Orchestration, "optimize": &o.Optimize,
		})
	}
	switch chartVersion {
	case "8.8":
		return valuesinjector.MergeImageTags88(content, o)
	case "8.9":
		return valuesinjector.MergeImageTags89(content, o)
	}
	return valuesinjector.MergeImageTags810(content, o)
}

func dropMissing(content string, components map[string]**valuesinjector.ComponentImage) {
	for name, img := range components {
		if *img != nil && !valuesinjector.HasImageTag(content, name) {
			*img = nil
		}
	}
}

func envImage(envVar string) *valuesinjector.ComponentImage {
	if tag := os.Getenv(envVar); tag != "" {
		return &valuesinjector.ComponentImage{Image: valuesinjector.ImageTag{Tag: tag}}
	}
	return nil
}

// buildOverridesClassic reads the 8.6/8.7 image-tag overrides from the
// environment. When ZEEBE_IMAGE_TAG is set but ZEEBE_GATEWAY_IMAGE_TAG is not,
// the gateway inherits the zeebe tag (they share the camunda/zeebe image).
func buildOverridesClassic() *valuesinjector.ValuesYAML86 {
	o := &valuesinjector.ValuesYAML86{
		Console:      envImage("CONSOLE_IMAGE_TAG"),
		Zeebe:        envImage("ZEEBE_IMAGE_TAG"),
		ZeebeGateway: envImage("ZEEBE_GATEWAY_IMAGE_TAG"),
		Operate:      envImage("OPERATE_IMAGE_TAG"),
		Tasklist:     envImage("TASKLIST_IMAGE_TAG"),
		Optimize:     envImage("OPTIMIZE_IMAGE_TAG"),
		Identity:     envImage("IDENTITY_IMAGE_TAG"),
		WebModeler:   envImage("WEB_MODELER_IMAGE_TAG"),
		Connectors:   envImage("CONNECTORS_IMAGE_TAG"),
	}
	if o.ZeebeGateway == nil && o.Zeebe != nil {
		o.ZeebeGateway = o.Zeebe
	}
	return o
}

// buildOverridesOrchestration reads the 8.8+ image-tag overrides from the environment.
func buildOverridesOrchestration() *valuesinjector.ValuesYAML88 {
	return &valuesinjector.ValuesYAML88{
		Identity:      envImage("IDENTITY_IMAGE_TAG"),
		Console:       envImage("CONSOLE_IMAGE_TAG"),
		WebModeler:    envImage("WEB_MODELER_IMAGE_TAG"),
		Connectors:    envImage("CONNECTORS_IMAGE_TAG"),
		Orchestration: envImage("ORCHESTRATION_IMAGE_TAG"),
		Optimize:      envImage("OPTIMIZE_IMAGE_TAG"),
	}
}
