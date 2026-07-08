package deploy

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"scripts/camunda-core/pkg/logging"

	"gopkg.in/yaml.v3"
)

// neutralizeOverriddenDigests rewrites the chart-root digest overlay so that it
// does not pin image.digest for any component whose image coordinates
// (registry/repository/tag) are explicitly supplied via --extra-values without an
// accompanying digest.
//
// Why this is needed: the chart image helper (camundaPlatform.imageByParams)
// prefers digest over tag — whenever image.digest is non-empty the tag is never
// consulted. The values chain applies the digest overlay before --extra-values,
// but Helm's map merge only overrides keys the later file actually sets. So an
// --extra-values file that sets registry/repository/tag (but no digest) leaves the
// overlay's digest in place, and the helper silently renders
// "<extra registry>/<extra repository>@<overlay digest>". In the monorepo nightly
// this glues the Harbor repo onto a DockerHub snapshot digest, producing a
// reference that does not exist and an ImagePullBackOff.
//
// Stripping the conflicting digest lets the helper fall through to the tag path so
// the --extra-values override actually takes effect. Components not overridden by
// --extra-values keep their digest pins. If an --extra-values file supplies its own
// digest, that digest is preserved (it wins via the normal merge), so we do not
// strip it.
//
// It returns the original overlay path unchanged when nothing needs stripping
// (no extra-values image overrides, or none of them collide with a pinned digest).
func neutralizeOverriddenDigests(overlayPath string, extraValues []string, tempDir string) (string, error) {
	overlay, err := loadValuesDoc(overlayPath)
	if err != nil {
		return "", fmt.Errorf("reading digest overlay %q: %w", overlayPath, err)
	}
	if overlay == nil {
		return overlayPath, nil
	}

	// Collect the dotted image-paths overridden by --extra-values: an image block
	// that sets registry/repository/tag but does not pin its own digest.
	overridden := map[string]bool{}
	var emptyTagOverrides []string
	for _, f := range extraValues {
		doc, readErr := loadValuesDoc(f)
		if readErr != nil {
			return "", fmt.Errorf("reading extra-values %q: %w", f, readErr)
		}
		walkImageBlocks(doc, func(path string, img map[string]any) {
			if _, hasDigest := img["digest"]; hasDigest {
				return // caller pinned an explicit digest — let it win via merge
			}
			// Present-but-empty tag with no digest: stripping the overlay digest
			// would render a version-less "<repository>:" (invalid YAML). Flag it.
			if tag, hasTag := img["tag"]; hasTag && isBlankScalar(tag) {
				emptyTagOverrides = append(emptyTagOverrides, path)
				return
			}
			if img["registry"] != nil || img["repository"] != nil || img["tag"] != nil {
				overridden[path] = true
			}
		})
	}
	if len(emptyTagOverrides) > 0 {
		sort.Strings(emptyTagOverrides)
		return "", fmt.Errorf("--extra-values image override for %v sets registry/repository with an empty tag and no digest; "+
			"set image.tag or image.digest for these component(s)", emptyTagOverrides)
	}
	if len(overridden) == 0 {
		return overlayPath, nil
	}

	// Strip the digest from any overlay image block the caller overrode.
	changed := false
	stripped := make([]string, 0, len(overridden))
	walkImageBlocks(overlay, func(path string, img map[string]any) {
		if !overridden[path] {
			return
		}
		if _, ok := img["digest"]; ok {
			delete(img, "digest")
			changed = true
			stripped = append(stripped, path)
		}
	})
	if !changed {
		return overlayPath, nil
	}

	sanitizedPath := filepath.Join(tempDir, "values-digest.sanitized.yaml")
	if err := writeValuesDoc(sanitizedPath, overlay); err != nil {
		return "", fmt.Errorf("writing sanitized digest overlay: %w", err)
	}

	sort.Strings(stripped)
	logging.Logger.Warn().
		Str("originalOverlay", overlayPath).
		Strs("componentsOverridden", stripped).
		Str("sanitizedOverlay", sanitizedPath).
		Msg("Stripped digest overlay pins shadowed by --extra-values image overrides")

	return sanitizedPath, nil
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
