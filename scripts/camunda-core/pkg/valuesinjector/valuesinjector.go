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

// Package valuesinjector overrides component image tags in a chart's values.yaml
// while preserving the file's formatting. The set of overridable components is
// version-gated (8.6/8.7 classic: zeebe/zeebeGateway/operate/tasklist/...; 8.8+
// orchestration). Each override is validated against the parsed YAML (the
// component, its image section, and the tag must exist) before a line-scoped
// regex replaces the tag, keeping comments/indentation intact.
package valuesinjector

import (
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// ImageTag is the tag field within an image configuration.
type ImageTag struct {
	Tag string `yaml:"tag,omitempty"`
}

// ComponentImage is a component's image configuration.
type ComponentImage struct {
	Image ImageTag `yaml:"image,omitempty"`
}

// ValuesYAML86 is the set of image-bearing components for charts 8.6 / 8.7.
type ValuesYAML86 struct {
	Console      *ComponentImage `yaml:"console,omitempty"`
	Zeebe        *ComponentImage `yaml:"zeebe,omitempty"`
	ZeebeGateway *ComponentImage `yaml:"zeebeGateway,omitempty"`
	Operate      *ComponentImage `yaml:"operate,omitempty"`
	Tasklist     *ComponentImage `yaml:"tasklist,omitempty"`
	Optimize     *ComponentImage `yaml:"optimize,omitempty"`
	Identity     *ComponentImage `yaml:"identity,omitempty"`
	WebModeler   *ComponentImage `yaml:"webModeler,omitempty"`
	Connectors   *ComponentImage `yaml:"connectors,omitempty"`
}

// ValuesYAML87 has the same component structure as 8.6.
type ValuesYAML87 = ValuesYAML86

// ValuesYAML88 is the set of image-bearing components for charts 8.8+; the
// 8.6/8.7 zeebe/zeebeGateway/operate/tasklist components are replaced by
// orchestration.
type ValuesYAML88 struct {
	Identity      *ComponentImage `yaml:"identity,omitempty"`
	Console       *ComponentImage `yaml:"console,omitempty"`
	WebModeler    *ComponentImage `yaml:"webModeler,omitempty"`
	Connectors    *ComponentImage `yaml:"connectors,omitempty"`
	Orchestration *ComponentImage `yaml:"orchestration,omitempty"`
	Optimize      *ComponentImage `yaml:"optimize,omitempty"`
}

// ValuesYAML89 has the same component structure as 8.8.
type ValuesYAML89 = ValuesYAML88

// ValuesYAML810 has the same component structure as 8.8.
type ValuesYAML810 = ValuesYAML88

// override pairs a component name with its (optional) image-tag override.
type override struct {
	name string
	comp *ComponentImage
}

// applyOverrides replaces each present override's image.tag in valuesYAML,
// preserving formatting. It errors if an override names a component that has no
// image.tag.
func applyOverrides(valuesYAML string, overrides []override) (string, error) {
	result := valuesYAML
	for _, o := range overrides {
		if o.comp == nil {
			continue
		}
		var err error
		if result, err = replaceImageTag(result, o.name, o.comp.Image.Tag); err != nil {
			return "", err
		}
	}
	return result, nil
}

// MergeImageTags86 applies the 8.6/8.7 image-tag overrides to a values.yaml string.
func MergeImageTags86(valuesYAML string, overrides *ValuesYAML86) (string, error) {
	if overrides == nil {
		return valuesYAML, nil
	}
	return applyOverrides(valuesYAML, []override{
		{"console", overrides.Console},
		{"zeebe", overrides.Zeebe},
		{"zeebeGateway", overrides.ZeebeGateway},
		{"operate", overrides.Operate},
		{"tasklist", overrides.Tasklist},
		{"optimize", overrides.Optimize},
		{"identity", overrides.Identity},
		{"webModeler", overrides.WebModeler},
		{"connectors", overrides.Connectors},
	})
}

// MergeImageTags87 applies the 8.7 overrides (same structure as 8.6).
func MergeImageTags87(valuesYAML string, overrides *ValuesYAML87) (string, error) {
	return MergeImageTags86(valuesYAML, overrides)
}

// MergeImageTags88 applies the 8.8+ image-tag overrides to a values.yaml string.
func MergeImageTags88(valuesYAML string, overrides *ValuesYAML88) (string, error) {
	if overrides == nil {
		return valuesYAML, nil
	}
	return applyOverrides(valuesYAML, []override{
		{"identity", overrides.Identity},
		{"console", overrides.Console},
		{"webModeler", overrides.WebModeler},
		{"connectors", overrides.Connectors},
		{"orchestration", overrides.Orchestration},
		{"optimize", overrides.Optimize},
	})
}

// MergeImageTags89 applies the 8.9 overrides (same structure as 8.8).
func MergeImageTags89(valuesYAML string, overrides *ValuesYAML89) (string, error) {
	return MergeImageTags88(valuesYAML, overrides)
}

// MergeImageTags810 applies the 8.10 image-tag overrides; no standalone console component.
func MergeImageTags810(valuesYAML string, overrides *ValuesYAML810) (string, error) {
	if overrides == nil {
		return valuesYAML, nil
	}
	return applyOverrides(valuesYAML, []override{
		{"identity", overrides.Identity},
		{"webModeler", overrides.WebModeler},
		{"connectors", overrides.Connectors},
		{"orchestration", overrides.Orchestration},
		{"optimize", overrides.Optimize},
	})
}

// replaceImageTag validates that componentName.image.tag exists, then replaces
// the tag line within that component's image section by regex (preserving the
// surrounding formatting).
func replaceImageTag(content string, componentName string, newTag string) (string, error) {
	if err := validateComponentImageTag(content, componentName); err != nil {
		return "", err
	}

	lines := strings.Split(content, "\n")
	componentStart, componentEnd := findComponentRange(lines, componentName)
	if componentStart == -1 {
		return "", fmt.Errorf("component '%s' not found in values.yaml", componentName)
	}

	imageStart, imageEnd := findImageRange(lines, componentStart, componentEnd)
	if imageStart == -1 {
		return "", fmt.Errorf("component '%s' does not have an 'image' section in values.yaml", componentName)
	}

	tagReplaced := false
	tagRegex := regexp.MustCompile(`^(\s*tag:\s*)(.*)$`)
	for i := imageStart; i <= imageEnd && i < len(lines); i++ {
		if tagRegex.MatchString(lines[i]) {
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

// validateComponentImageTag confirms componentName.image.tag exists via YAML parse.
func validateComponentImageTag(content string, componentName string) error {
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(content), &root); err != nil {
		return fmt.Errorf("failed to parse values.yaml: %w", err)
	}
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
	componentNode := findMapValue(rootNode, componentName)
	if componentNode == nil {
		return fmt.Errorf("component '%s' not found in values.yaml", componentName)
	}
	if componentNode.Kind != yaml.MappingNode {
		return fmt.Errorf("component '%s' is not a valid map in values.yaml", componentName)
	}
	imageNode := findMapValue(componentNode, "image")
	if imageNode == nil {
		return fmt.Errorf("component '%s' does not have an 'image' section in values.yaml", componentName)
	}
	if imageNode.Kind != yaml.MappingNode {
		return fmt.Errorf("component '%s.image' is not a valid map in values.yaml", componentName)
	}
	if findMapValue(imageNode, "tag") == nil {
		return fmt.Errorf("component '%s.image' does not have a 'tag' field in values.yaml", componentName)
	}
	return nil
}

// findMapValue returns the value node for key in a mapping node, or nil.
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

// findComponentRange returns the [start,end] line indices of a top-level
// component block, or -1,-1 if not found.
func findComponentRange(lines []string, componentName string) (int, int) {
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

// findImageRange returns the [start,end] line indices of the image section
// within a component block, or -1,-1 if not found.
func findImageRange(lines []string, componentStart, componentEnd int) (int, int) {
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
	end := componentEnd
	imageIndentLen := len(imageIndent)
	for i := start + 1; i <= componentEnd && i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		lineIndent := len(line) - len(strings.TrimLeft(line, " \t"))
		if lineIndent <= imageIndentLen {
			end = i - 1
			break
		}
	}
	return start, end
}
