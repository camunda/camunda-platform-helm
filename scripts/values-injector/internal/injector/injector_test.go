package injector

import (
	"strings"
	"testing"

	"github.com/camunda/camunda-platform-helm/scripts/values-injector/internal/chartcomponents"
)

// Sample values.yaml content for testing chart 8.6/8.7
const sampleValues86 = `
global:
  image:
    tag: "8.6.0"
console:
  enabled: true
  image:
    registry: ""
    repository: camunda/console
    tag: 8.6.94
zeebe:
  enabled: true
  image:
    registry: ""
    repository: camunda/zeebe
    tag: 8.6.34
zeebeGateway:
  replicas: 2
  image:
    registry: ""
    repository: camunda/zeebe
    tag: 8.6.34
operate:
  enabled: true
  image:
    registry: ""
    repository: camunda/operate
    tag: 8.6.34
tasklist:
  enabled: true
  image:
    registry: ""
    repository: camunda/tasklist
    tag: 8.6.34
optimize:
  enabled: true
  image:
    registry: ""
    repository: camunda/optimize
    tag: 8.6.22
identity:
  enabled: true
  image:
    registry: ""
    repository: camunda/identity
    tag: 8.6.25
webModeler:
  enabled: false
  image:
    registry: ""
    tag: 8.6.23
connectors:
  enabled: true
  image:
    registry: ""
    repository: camunda/connectors-bundle
    tag: 8.6.22
`

// Sample values.yaml content for testing chart 8.8/8.9
const sampleValues88 = `
global:
  image:
    tag: ""
identity:
  enabled: false
  image:
    registry: ""
    repository: camunda/identity
    tag: 8.8.5
console:
  enabled: false
  image:
    registry: ""
    repository: camunda/console
    tag: 8.8.69
webModeler:
  enabled: false
  image:
    registry: ""
    tag: 8.8.4
connectors:
  enabled: true
  image:
    registry: ""
    repository: camunda/connectors-bundle
    tag: 8.8.4
orchestration:
  enabled: true
  image:
    registry: ""
    repository: camunda/camunda
    tag: 8.8.8
optimize:
  enabled: false
  image:
    registry: ""
    repository: camunda/optimize
    tag: 8.8.3
`

func TestMergeImageTags86_SingleComponent(t *testing.T) {
	overrides := &chartcomponents.ValuesYAML86{
		Console: &chartcomponents.ComponentImage{
			Image: chartcomponents.ImageTag{Tag: "1.2.3"},
		},
	}

	result, err := MergeImageTags86(sampleValues86, overrides)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "tag: \"1.2.3\"") && !strings.Contains(result, "tag: 1.2.3") {
		t.Errorf("expected console tag to be updated to 1.2.3, got:\n%s", result)
	}

	// Verify other tags are unchanged
	if !strings.Contains(result, "tag: 8.6.34") {
		t.Errorf("expected zeebe tag to remain 8.6.34")
	}
}

func TestMergeImageTags86_MultipleComponents(t *testing.T) {
	overrides := &chartcomponents.ValuesYAML86{
		Console: &chartcomponents.ComponentImage{
			Image: chartcomponents.ImageTag{Tag: "console-new"},
		},
		Zeebe: &chartcomponents.ComponentImage{
			Image: chartcomponents.ImageTag{Tag: "zeebe-new"},
		},
		Identity: &chartcomponents.ComponentImage{
			Image: chartcomponents.ImageTag{Tag: "identity-new"},
		},
	}

	result, err := MergeImageTags86(sampleValues86, overrides)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "console-new") {
		t.Errorf("expected console tag to be updated")
	}
	if !strings.Contains(result, "zeebe-new") {
		t.Errorf("expected zeebe tag to be updated")
	}
	if !strings.Contains(result, "identity-new") {
		t.Errorf("expected identity tag to be updated")
	}
}

func TestMergeImageTags86_NoOverrides(t *testing.T) {
	result, err := MergeImageTags86(sampleValues86, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != sampleValues86 {
		t.Errorf("expected no changes when overrides is nil")
	}
}

func TestMergeImageTags86_EmptyOverrides(t *testing.T) {
	overrides := &chartcomponents.ValuesYAML86{}

	result, err := MergeImageTags86(sampleValues86, overrides)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should still contain original tags
	if !strings.Contains(result, "8.6.94") {
		t.Errorf("expected console tag to remain unchanged")
	}
}

func TestMergeImageTags86_ComponentNotFound(t *testing.T) {
	// YAML without the console component
	yamlWithoutConsole := `
zeebe:
  image:
    tag: 8.6.34
`

	overrides := &chartcomponents.ValuesYAML86{
		Console: &chartcomponents.ComponentImage{
			Image: chartcomponents.ImageTag{Tag: "1.2.3"},
		},
	}

	_, err := MergeImageTags86(yamlWithoutConsole, overrides)
	if err == nil {
		t.Fatal("expected error when component not found")
	}

	if !strings.Contains(err.Error(), "console") {
		t.Errorf("expected error message to mention 'console', got: %v", err)
	}
}

func TestMergeImageTags86_ComponentWithoutImageSection(t *testing.T) {
	// YAML with console but no image section
	yamlWithoutImage := `
console:
  enabled: true
zeebe:
  image:
    tag: 8.6.34
`

	overrides := &chartcomponents.ValuesYAML86{
		Console: &chartcomponents.ComponentImage{
			Image: chartcomponents.ImageTag{Tag: "1.2.3"},
		},
	}

	_, err := MergeImageTags86(yamlWithoutImage, overrides)
	if err == nil {
		t.Fatal("expected error when image section not found")
	}

	if !strings.Contains(err.Error(), "image") {
		t.Errorf("expected error message to mention 'image', got: %v", err)
	}
}

func TestMergeImageTags87_UsesChart86Logic(t *testing.T) {
	overrides := &chartcomponents.ValuesYAML87{
		Console: &chartcomponents.ComponentImage{
			Image: chartcomponents.ImageTag{Tag: "8.7-test"},
		},
	}

	result, err := MergeImageTags87(sampleValues86, overrides)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "8.7-test") {
		t.Errorf("expected console tag to be updated")
	}
}

func TestMergeImageTags88_SingleComponent(t *testing.T) {
	overrides := &chartcomponents.ValuesYAML88{
		Orchestration: &chartcomponents.ComponentImage{
			Image: chartcomponents.ImageTag{Tag: "orchestration-new"},
		},
	}

	result, err := MergeImageTags88(sampleValues88, overrides)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "orchestration-new") {
		t.Errorf("expected orchestration tag to be updated")
	}
}

func TestMergeImageTags88_MultipleComponents(t *testing.T) {
	overrides := &chartcomponents.ValuesYAML88{
		Identity: &chartcomponents.ComponentImage{
			Image: chartcomponents.ImageTag{Tag: "identity-88"},
		},
		Console: &chartcomponents.ComponentImage{
			Image: chartcomponents.ImageTag{Tag: "console-88"},
		},
		Orchestration: &chartcomponents.ComponentImage{
			Image: chartcomponents.ImageTag{Tag: "orchestration-88"},
		},
	}

	result, err := MergeImageTags88(sampleValues88, overrides)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "identity-88") {
		t.Errorf("expected identity tag to be updated")
	}
	if !strings.Contains(result, "console-88") {
		t.Errorf("expected console tag to be updated")
	}
	if !strings.Contains(result, "orchestration-88") {
		t.Errorf("expected orchestration tag to be updated")
	}
}

func TestMergeImageTags88_NoOverrides(t *testing.T) {
	result, err := MergeImageTags88(sampleValues88, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != sampleValues88 {
		t.Errorf("expected no changes when overrides is nil")
	}
}

func TestMergeImageTags89_UsesChart88Logic(t *testing.T) {
	overrides := &chartcomponents.ValuesYAML89{
		Orchestration: &chartcomponents.ComponentImage{
			Image: chartcomponents.ImageTag{Tag: "8.9-test"},
		},
	}

	result, err := MergeImageTags89(sampleValues88, overrides)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "8.9-test") {
		t.Errorf("expected orchestration tag to be updated")
	}
}

func TestMergeImageTags_InvalidYAML(t *testing.T) {
	invalidYAML := `
this is not valid yaml:
  - missing colon here
    nested: [unclosed
`

	overrides := &chartcomponents.ValuesYAML86{
		Console: &chartcomponents.ComponentImage{
			Image: chartcomponents.ImageTag{Tag: "1.2.3"},
		},
	}

	_, err := MergeImageTags86(invalidYAML, overrides)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestMergeImageTags_PreservesOtherFields(t *testing.T) {
	overrides := &chartcomponents.ValuesYAML86{
		Console: &chartcomponents.ComponentImage{
			Image: chartcomponents.ImageTag{Tag: "new-tag"},
		},
	}

	result, err := MergeImageTags86(sampleValues86, overrides)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check that other fields are preserved
	if !strings.Contains(result, "repository: camunda/console") {
		t.Errorf("expected repository field to be preserved")
	}
	if !strings.Contains(result, "enabled: true") {
		t.Errorf("expected enabled field to be preserved")
	}
}
