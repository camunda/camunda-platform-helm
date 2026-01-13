package main

import (
	"fmt"
	"log"
	"os"

	"github.com/camunda/camunda-platform-helm/scripts/values-injector/internal/chartcomponents"
	"github.com/camunda/camunda-platform-helm/scripts/values-injector/internal/injector"
)

func main() {
	// 1. Read and validate CHART_VERSION
	chartVersion := os.Getenv("CHART_VERSION")
	if chartVersion == "" {
		log.Fatal("CHART_VERSION environment variable is required")
	}

	// 2. Validate chart version and derive file path
	validVersions := map[string]bool{"8.6": true, "8.7": true, "8.8": true, "8.9": true}
	if !validVersions[chartVersion] {
		log.Fatalf("unsupported CHART_VERSION: %s (supported: 8.6, 8.7, 8.8, 8.9)", chartVersion)
	}

	valuesFile := fmt.Sprintf("charts/camunda-platform-%s/values.yaml", chartVersion)

	// 3. Read values.yaml content
	content, err := os.ReadFile(valuesFile)
	if err != nil {
		log.Fatalf("failed to read %s: %v", valuesFile, err)
	}

	// 4. Build overrides and call appropriate merge function based on chart version
	var result string
	switch chartVersion {
	case "8.6":
		overrides := buildOverrides86()
		result, err = injector.MergeImageTags86(string(content), overrides)
	case "8.7":
		overrides := buildOverrides87()
		result, err = injector.MergeImageTags87(string(content), overrides)
	case "8.8":
		overrides := buildOverrides88()
		result, err = injector.MergeImageTags88(string(content), overrides)
	case "8.9":
		overrides := buildOverrides89()
		result, err = injector.MergeImageTags89(string(content), overrides)
	}

	if err != nil {
		log.Fatalf("failed to merge image tags: %v", err)
	}

	// 5. Write back to file (in-place)
	if err := os.WriteFile(valuesFile, []byte(result), 0644); err != nil {
		log.Fatalf("failed to write %s: %v", valuesFile, err)
	}

	fmt.Printf("Successfully updated %s\n", valuesFile)
}

// buildOverrides86 builds the overrides struct for chart 8.6 from environment variables
func buildOverrides86() *chartcomponents.ValuesYAML86 {
	overrides := &chartcomponents.ValuesYAML86{}

	if tag := os.Getenv("CONSOLE_IMAGE_TAG"); tag != "" {
		overrides.Console = &chartcomponents.ComponentImage{
			Image: chartcomponents.ImageTag{Tag: tag},
		}
	}
	if tag := os.Getenv("ZEEBE_IMAGE_TAG"); tag != "" {
		overrides.Zeebe = &chartcomponents.ComponentImage{
			Image: chartcomponents.ImageTag{Tag: tag},
		}
	}
	if tag := os.Getenv("ZEEBE_GATEWAY_IMAGE_TAG"); tag != "" {
		overrides.ZeebeGateway = &chartcomponents.ComponentImage{
			Image: chartcomponents.ImageTag{Tag: tag},
		}
	}
	if tag := os.Getenv("OPERATE_IMAGE_TAG"); tag != "" {
		overrides.Operate = &chartcomponents.ComponentImage{
			Image: chartcomponents.ImageTag{Tag: tag},
		}
	}
	if tag := os.Getenv("TASKLIST_IMAGE_TAG"); tag != "" {
		overrides.Tasklist = &chartcomponents.ComponentImage{
			Image: chartcomponents.ImageTag{Tag: tag},
		}
	}
	if tag := os.Getenv("OPTIMIZE_IMAGE_TAG"); tag != "" {
		overrides.Optimize = &chartcomponents.ComponentImage{
			Image: chartcomponents.ImageTag{Tag: tag},
		}
	}
	if tag := os.Getenv("IDENTITY_IMAGE_TAG"); tag != "" {
		overrides.Identity = &chartcomponents.ComponentImage{
			Image: chartcomponents.ImageTag{Tag: tag},
		}
	}
	if tag := os.Getenv("WEB_MODELER_IMAGE_TAG"); tag != "" {
		overrides.WebModeler = &chartcomponents.ComponentImage{
			Image: chartcomponents.ImageTag{Tag: tag},
		}
	}
	if tag := os.Getenv("CONNECTORS_IMAGE_TAG"); tag != "" {
		overrides.Connectors = &chartcomponents.ComponentImage{
			Image: chartcomponents.ImageTag{Tag: tag},
		}
	}

	return overrides
}

// buildOverrides87 builds the overrides struct for chart 8.7 from environment variables
// Chart 8.7 has the same structure as 8.6
func buildOverrides87() *chartcomponents.ValuesYAML87 {
	return buildOverrides86()
}

// buildOverrides88 builds the overrides struct for chart 8.8 from environment variables
func buildOverrides88() *chartcomponents.ValuesYAML88 {
	overrides := &chartcomponents.ValuesYAML88{}

	if tag := os.Getenv("IDENTITY_IMAGE_TAG"); tag != "" {
		overrides.Identity = &chartcomponents.ComponentImage{
			Image: chartcomponents.ImageTag{Tag: tag},
		}
	}
	if tag := os.Getenv("CONSOLE_IMAGE_TAG"); tag != "" {
		overrides.Console = &chartcomponents.ComponentImage{
			Image: chartcomponents.ImageTag{Tag: tag},
		}
	}
	if tag := os.Getenv("WEB_MODELER_IMAGE_TAG"); tag != "" {
		overrides.WebModeler = &chartcomponents.ComponentImage{
			Image: chartcomponents.ImageTag{Tag: tag},
		}
	}
	if tag := os.Getenv("CONNECTORS_IMAGE_TAG"); tag != "" {
		overrides.Connectors = &chartcomponents.ComponentImage{
			Image: chartcomponents.ImageTag{Tag: tag},
		}
	}
	if tag := os.Getenv("ORCHESTRATION_IMAGE_TAG"); tag != "" {
		overrides.Orchestration = &chartcomponents.ComponentImage{
			Image: chartcomponents.ImageTag{Tag: tag},
		}
	}
	if tag := os.Getenv("OPTIMIZE_IMAGE_TAG"); tag != "" {
		overrides.Optimize = &chartcomponents.ComponentImage{
			Image: chartcomponents.ImageTag{Tag: tag},
		}
	}

	return overrides
}

// buildOverrides89 builds the overrides struct for chart 8.9 from environment variables
// Chart 8.9 has the same structure as 8.8
func buildOverrides89() *chartcomponents.ValuesYAML89 {
	return buildOverrides88()
}
