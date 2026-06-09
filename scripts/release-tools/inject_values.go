package main

import (
	"fmt"
	"os"

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

	valuesFile := fmt.Sprintf("charts/camunda-platform-%s/values.yaml", chartVersion)
	content, err := os.ReadFile(valuesFile)
	if err != nil {
		return fmt.Errorf("read %s: %w", valuesFile, err)
	}

	var result string
	switch chartVersion {
	case "8.6":
		result, err = valuesinjector.MergeImageTags86(string(content), buildOverridesClassic())
	case "8.7":
		result, err = valuesinjector.MergeImageTags87(string(content), buildOverridesClassic())
	case "8.8":
		result, err = valuesinjector.MergeImageTags88(string(content), buildOverridesOrchestration())
	case "8.9":
		result, err = valuesinjector.MergeImageTags89(string(content), buildOverridesOrchestration())
	case "8.10":
		result, err = valuesinjector.MergeImageTags810(string(content), buildOverridesOrchestration())
	}
	if err != nil {
		return fmt.Errorf("merge image tags: %w", err)
	}

	if err := os.WriteFile(valuesFile, []byte(result), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", valuesFile, err)
	}
	fmt.Printf("Successfully updated %s\n", valuesFile)
	return nil
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
