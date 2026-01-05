package injector

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/camunda/camunda-platform-helm/scripts/values-injector/internal/chartcomponents"
	"gopkg.in/yaml.v3"
)

// MergeImageTags86 merges image tag overrides into a values.yaml string for chart 8.6
// Returns error if a tag is specified for a component that doesn't exist in the YAML
func MergeImageTags86(valuesYAML string, overrides *chartcomponents.ValuesYAML86) (string, error) {
	if overrides == nil {
		return valuesYAML, nil
	}

	result := valuesYAML

	// Apply overrides for each component
	if overrides.Console != nil {
		var err error
		result, err = replaceImageTag(result, "console", overrides.Console.Image.Tag)
		if err != nil {
			return "", err
		}
	}
	if overrides.Zeebe != nil {
		var err error
		result, err = replaceImageTag(result, "zeebe", overrides.Zeebe.Image.Tag)
		if err != nil {
			return "", err
		}
	}
	if overrides.ZeebeGateway != nil {
		var err error
		result, err = replaceImageTag(result, "zeebeGateway", overrides.ZeebeGateway.Image.Tag)
		if err != nil {
			return "", err
		}
	}
	if overrides.Operate != nil {
		var err error
		result, err = replaceImageTag(result, "operate", overrides.Operate.Image.Tag)
		if err != nil {
			return "", err
		}
	}
	if overrides.Tasklist != nil {
		var err error
		result, err = replaceImageTag(result, "tasklist", overrides.Tasklist.Image.Tag)
		if err != nil {
			return "", err
		}
	}
	if overrides.Optimize != nil {
		var err error
		result, err = replaceImageTag(result, "optimize", overrides.Optimize.Image.Tag)
		if err != nil {
			return "", err
		}
	}
	if overrides.Identity != nil {
		var err error
		result, err = replaceImageTag(result, "identity", overrides.Identity.Image.Tag)
		if err != nil {
			return "", err
		}
	}
	if overrides.WebModeler != nil {
		var err error
		result, err = replaceImageTag(result, "webModeler", overrides.WebModeler.Image.Tag)
		if err != nil {
			return "", err
		}
	}
	if overrides.Connectors != nil {
		var err error
		result, err = replaceImageTag(result, "connectors", overrides.Connectors.Image.Tag)
		if err != nil {
			return "", err
		}
	}

	return result, nil
}

// MergeImageTags87 merges image tag overrides into a values.yaml string for chart 8.7
// Chart 8.7 has the same structure as 8.6
func MergeImageTags87(valuesYAML string, overrides *chartcomponents.ValuesYAML87) (string, error) {
	return MergeImageTags86(valuesYAML, overrides)
}

// MergeImageTags88 merges image tag overrides into a values.yaml string for chart 8.8
// Returns error if a tag is specified for a component that doesn't exist in the YAML
func MergeImageTags88(valuesYAML string, overrides *chartcomponents.ValuesYAML88) (string, error) {
	if overrides == nil {
		return valuesYAML, nil
	}

	result := valuesYAML

	// Apply overrides for each component
	if overrides.Identity != nil {
		var err error
		result, err = replaceImageTag(result, "identity", overrides.Identity.Image.Tag)
		if err != nil {
			return "", err
		}
	}
	if overrides.Console != nil {
		var err error
		result, err = replaceImageTag(result, "console", overrides.Console.Image.Tag)
		if err != nil {
			return "", err
		}
	}
	if overrides.WebModeler != nil {
		var err error
		result, err = replaceImageTag(result, "webModeler", overrides.WebModeler.Image.Tag)
		if err != nil {
			return "", err
		}
	}
	if overrides.Connectors != nil {
		var err error
		result, err = replaceImageTag(result, "connectors", overrides.Connectors.Image.Tag)
		if err != nil {
			return "", err
		}
	}
	if overrides.Orchestration != nil {
		var err error
		result, err = replaceImageTag(result, "orchestration", overrides.Orchestration.Image.Tag)
		if err != nil {
			return "", err
		}
	}
	if overrides.Optimize != nil {
		var err error
		result, err = replaceImageTag(result, "optimize", overrides.Optimize.Image.Tag)
		if err != nil {
			return "", err
		}
	}

	return result, nil
}

// MergeImageTags89 merges image tag overrides into a values.yaml string for chart 8.9
// Chart 8.9 has the same structure as 8.8
func MergeImageTags89(valuesYAML string, overrides *chartcomponents.ValuesYAML89) (string, error) {
	return MergeImageTags88(valuesYAML, overrides)
}

// replaceImageTag replaces the image.tag value for a specific component
// It first validates the component exists using YAML parsing, then uses
// regex replacement to preserve the original formatting
func replaceImageTag(content string, componentName string, newTag string) (string, error) {
	// First, validate the component and image.tag exist using YAML parsing
	if err := validateComponentImageTag(content, componentName); err != nil {
		return "", err
	}

	// Find the component section and replace the tag within it
	// We need to find the component, then find its image section, then replace the tag

	// Strategy: Find the line number range for the component, then within that range
	// find the image section, then within that find and replace the tag line

	lines := strings.Split(content, "\n")
	componentStart, componentEnd := findComponentRange(lines, componentName)
	if componentStart == -1 {
		return "", fmt.Errorf("component '%s' not found in values.yaml", componentName)
	}

	// Within the component range, find the image section
	imageStart, imageEnd := findImageRange(lines, componentStart, componentEnd)
	if imageStart == -1 {
		return "", fmt.Errorf("component '%s' does not have an 'image' section in values.yaml", componentName)
	}

	// Within the image range, find and replace the tag line
	tagReplaced := false
	tagRegex := regexp.MustCompile(`^(\s*tag:\s*)(.*)$`)

	for i := imageStart; i <= imageEnd && i < len(lines); i++ {
		if tagRegex.MatchString(lines[i]) {
			// Replace the tag value, preserving the prefix (indentation and "tag: ")
			lines[i] = tagRegex.ReplaceAllString(lines[i], "${1}"+newTag)
			tagReplaced = true
			break
		}
	}

	if !tagReplaced {
		return "", fmt.Errorf("component '%s.image' does not have a 'tag' field in values.yaml", componentName)
	}

	return strings.Join(lines, "\n"), nil
}

// validateComponentImageTag validates that a component with image.tag exists using YAML parsing
func validateComponentImageTag(content string, componentName string) error {
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(content), &root); err != nil {
		return fmt.Errorf("failed to parse values.yaml: %w", err)
	}

	// Handle document node wrapper
	rootNode := &root
	if root.Kind == yaml.DocumentNode {
		if len(root.Content) == 0 {
			return fmt.Errorf("empty YAML document")
		}
		rootNode = root.Content[0]
	}

	if rootNode.Kind != yaml.MappingNode {
		return fmt.Errorf("expected mapping node at root")
	}

	// Find the component
	componentNode := findMapValue(rootNode, componentName)
	if componentNode == nil {
		return fmt.Errorf("component '%s' not found in values.yaml", componentName)
	}

	if componentNode.Kind != yaml.MappingNode {
		return fmt.Errorf("component '%s' is not a valid map in values.yaml", componentName)
	}

	// Find the image section
	imageNode := findMapValue(componentNode, "image")
	if imageNode == nil {
		return fmt.Errorf("component '%s' does not have an 'image' section in values.yaml", componentName)
	}

	if imageNode.Kind != yaml.MappingNode {
		return fmt.Errorf("component '%s.image' is not a valid map in values.yaml", componentName)
	}

	// Find the tag
	tagNode := findMapValue(imageNode, "tag")
	if tagNode == nil {
		return fmt.Errorf("component '%s.image' does not have a 'tag' field in values.yaml", componentName)
	}

	return nil
}

// findMapValue finds a value node by key in a mapping node
func findMapValue(node *yaml.Node, key string) *yaml.Node {
	if node.Kind != yaml.MappingNode {
		return nil
	}

	for i := 0; i < len(node.Content)-1; i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

// findComponentRange finds the line range for a top-level component
// Returns start and end line indices (0-based), or -1, -1 if not found
func findComponentRange(lines []string, componentName string) (int, int) {
	// Pattern for top-level component (no leading whitespace)
	componentPattern := regexp.MustCompile(`^` + regexp.QuoteMeta(componentName) + `:`)

	start := -1
	for i, line := range lines {
		if componentPattern.MatchString(line) {
			start = i
			break
		}
	}

	if start == -1 {
		return -1, -1
	}

	// Find where the component ends (next top-level key or end of file)
	// A top-level key is a line that starts with a non-space character and contains ':'
	topLevelPattern := regexp.MustCompile(`^[a-zA-Z].*:`)
	end := len(lines) - 1

	for i := start + 1; i < len(lines); i++ {
		if topLevelPattern.MatchString(lines[i]) {
			end = i - 1
			break
		}
	}

	return start, end
}

// findImageRange finds the line range for the image section within a component range
// Returns start and end line indices, or -1, -1 if not found
func findImageRange(lines []string, componentStart, componentEnd int) (int, int) {
	// Find "image:" line within the component (must be indented)
	imagePattern := regexp.MustCompile(`^(\s+)image:\s*$`)

	start := -1
	var imageIndent string

	for i := componentStart + 1; i <= componentEnd && i < len(lines); i++ {
		match := imagePattern.FindStringSubmatch(lines[i])
		if match != nil {
			start = i
			imageIndent = match[1]
			break
		}
	}

	if start == -1 {
		return -1, -1
	}

	// Find where the image section ends
	// It ends when we hit a line with same or less indentation (that's not empty/comment)
	end := componentEnd
	imageIndentLen := len(imageIndent)

	for i := start + 1; i <= componentEnd && i < len(lines); i++ {
		line := lines[i]

		// Skip empty lines and comments
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Check indentation - if it's same or less than image:, we've left the image section
		lineIndent := len(line) - len(strings.TrimLeft(line, " \t"))
		if lineIndent <= imageIndentLen {
			end = i - 1
			break
		}
	}

	return start, end
}
