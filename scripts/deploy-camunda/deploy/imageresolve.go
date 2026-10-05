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

package deploy

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"scripts/camunda-core/pkg/logging"

	"gopkg.in/yaml.v3"
)

// ChartRootOverlayFiles returns the existing <chartPath>/values-<name>.yaml files, in order.
func ChartRootOverlayFiles(chartPath string, names []string) []string {
	if chartPath == "" {
		return nil
	}
	var files []string
	for _, name := range names {
		path := filepath.Join(chartPath, "values-"+name+".yaml")
		if _, err := os.Stat(path); err != nil {
			logging.Logger.Debug().Str("overlay", name).Str("path", path).Msg("Chart-root overlay not found, skipping")
			continue
		}
		files = append(files, path)
	}
	return files
}

// resolveImages binds every digest in chain to the registry, repository and tag it was pinned
// with and writes each image's resolved identity to one values file that the caller applies last.
// A later layer that changes those coordinates without its own digest clears the digest; without
// a tag it is rejected unless allowDigestShadow keeps the digest. Layers from firstScenarioLayer on
// may not set an empty tag without a digest.
func resolveImages(chain []string, firstScenarioLayer int, allowDigestShadow bool, tempDir string) (string, error) {
	images := map[string]map[string]any{}
	sources := map[string]string{}
	pins := map[string]string{}
	var problems []string
	for i, file := range chain {
		doc, err := loadValuesDoc(file)
		if err != nil {
			return "", fmt.Errorf("reading values %q: %w", file, err)
		}
		walkImageBlocks(doc, func(path string, img map[string]any) {
			digest, hasDigest := img["digest"]
			if tag, hasTag := img["tag"]; hasTag && isBlankScalar(tag) && !hasDigest && i >= firstScenarioLayer {
				problems = append(problems, fmt.Sprintf("%s: %s sets an empty tag and no digest; set image.tag or image.digest", path, file))
				return
			}
			resolved := images[path]
			if resolved == nil {
				resolved = map[string]any{}
				images[path] = resolved
			}
			moved := false
			for _, key := range []string{"registry", "repository", "tag"} {
				if v, ok := img[key]; ok {
					moved = moved || scalar(v) != scalar(resolved[key])
					resolved[key] = v
				}
			}
			if moved || hasDigest {
				sources[path] = file
			}
			switch {
			case hasDigest:
				resolved["digest"] = digest
				pins[path] = file
				if isBlankScalar(digest) {
					delete(pins, path)
				}
			case !moved || pins[path] == "":
			case !isBlankScalar(img["tag"]):
				resolved["digest"] = nil
				delete(pins, path)
			case !allowDigestShadow:
				problems = append(problems, fmt.Sprintf("%s: %s changes registry/repository without a tag, so the digest from %s would follow it; set image.tag or image.digest, or pass --allow-digest-shadow", path, file, pins[path]))
			}
		})
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return "", fmt.Errorf("conflicting image overrides:\n  %s", strings.Join(problems, "\n  "))
	}

	out := map[string]any{}
	var refs []string
	for path, resolved := range images {
		if sources[path] != "" {
			setImage(out, path, resolved)
			refs = append(refs, fmt.Sprintf("%s=%s (%s)", path, imageRef(resolved), filepath.Base(sources[path])))
		}
	}
	if len(refs) == 0 {
		return "", nil
	}
	resolvedPath := filepath.Join(tempDir, "values-resolved-images.yaml")
	if err := writeValuesDoc(resolvedPath, out); err != nil {
		return "", fmt.Errorf("writing resolved images: %w", err)
	}
	sort.Strings(refs)
	logging.Logger.Info().Strs("images", refs).Msg("Resolved images")
	return resolvedPath, nil
}

// writeImageOverrides writes --image-override component=[registry/]repository(:tag|@digest) values as one values file.
func writeImageOverrides(overrides []string, chartPath, tempDir string) (string, error) {
	known := map[string]bool{}
	if chartPath != "" {
		chartValues, err := loadValuesDoc(filepath.Join(chartPath, "values.yaml"))
		if err != nil {
			return "", fmt.Errorf("reading %s/values.yaml: %w", chartPath, err)
		}
		walkImageBlocks(chartValues, func(path string, _ map[string]any) { known[path] = true })
	}
	doc := map[string]any{}
	for _, override := range overrides {
		component, ref, _ := strings.Cut(override, "=")
		img := parseImageRef(ref)
		if img == nil || component == "" {
			return "", fmt.Errorf("--image-override %q: want component=registry/repository:tag or component=registry/repository@sha256:digest", override)
		}
		if chartPath != "" && !known[component] {
			return "", fmt.Errorf("--image-override %q: the chart has no %s.image", override, component)
		}
		setImage(doc, component, img)
	}
	path := filepath.Join(tempDir, "values-image-overrides.yaml")
	return path, writeValuesDoc(path, doc)
}

// parseImageRef splits [registry/]repository[:tag][@digest] into image values, or returns nil without a tag or digest.
func parseImageRef(ref string) map[string]any {
	name, digest, _ := strings.Cut(ref, "@")
	img := map[string]any{"registry": "", "tag": "", "digest": digest}
	if i := strings.LastIndex(name, ":"); i > strings.LastIndex(name, "/") {
		name, img["tag"] = name[:i], name[i+1:]
	}
	if registry, repository, ok := strings.Cut(name, "/"); ok && (strings.ContainsAny(registry, ".:") || registry == "localhost") {
		img["registry"], name = registry, repository
	}
	if name == "" || img["tag"] == "" && digest == "" {
		return nil
	}
	img["repository"] = name
	return img
}

// setImage stores img as the image block of the dotted component path in doc.
func setImage(doc map[string]any, path string, img map[string]any) {
	for _, key := range strings.Split(path, ".") {
		child, _ := doc[key].(map[string]any)
		if child == nil {
			child = map[string]any{}
			doc[key] = child
		}
		doc = child
	}
	doc["image"] = img
}

// imageRef renders an image block as a reference; an empty repository is the chart default.
func imageRef(img map[string]any) string {
	ref := scalar(img["repository"])
	if registry := scalar(img["registry"]); registry != "" {
		ref = registry + "/" + ref
	}
	if digest := scalar(img["digest"]); digest != "" {
		return ref + "@" + digest
	}
	return ref + ":" + scalar(img["tag"])
}

func scalar(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

// isBlankScalar reports whether a YAML scalar is unset: a nil value (a bare
// "tag:" key) or an empty string ("tag: \"\"").
func isBlankScalar(v any) bool {
	return v == nil || v == ""
}

// walkImageBlocks recursively walks a values document and invokes fn for every
// map node that contains an "image" sub-map, passing the dotted path of the
// component owning the image (e.g. "orchestration", "webModeler.restapi") and the
// image map itself. The image map is the live reference, so mutating it (e.g.
// deleting "digest") edits the underlying document.
func walkImageBlocks(node map[string]any, fn func(path string, img map[string]any)) {
	walkImageBlocksRec("", node, fn)
}

func walkImageBlocksRec(path string, node map[string]any, fn func(path string, img map[string]any)) {
	if img, ok := node["image"].(map[string]any); ok {
		fn(path, img)
	}
	for key, val := range node {
		child, ok := val.(map[string]any)
		if !ok {
			continue
		}
		childPath := key
		if path != "" {
			childPath = path + "." + key
		}
		walkImageBlocksRec(childPath, child, fn)
	}
}

// loadValuesDoc reads a YAML file into a generic map. An empty document yields a
// nil map and no error.
func loadValuesDoc(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	return doc, nil
}

// writeValuesDoc marshals a generic map and writes it to path.
func writeValuesDoc(path string, doc map[string]any) error {
	out, err := yaml.Marshal(doc)
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o644)
}
