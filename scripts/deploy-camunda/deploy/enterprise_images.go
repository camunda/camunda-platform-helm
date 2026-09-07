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
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/sync/errgroup"

	"scripts/camunda-core/pkg/executil"
)

// defaultImagePlatform is the platform asserted when none is configured.
const defaultImagePlatform = "linux/amd64"

// pinnedImage is an image reference that a values layer pins completely.
type pinnedImage struct {
	Ref    string // registry/repository:tag
	Source string // values file it came from
}

// manifestDescriptor is the subset of an OCI/Docker image index entry we need.
type manifestDescriptor struct {
	Digest   string `json:"digest"`
	Platform struct {
		OS           string `json:"os"`
		Architecture string `json:"architecture"`
		Variant      string `json:"variant"`
	} `json:"platform"`
}

// imageIndex is the subset of `docker manifest inspect` output we need. A
// single-platform image has no "manifests" key, which unmarshals to nil.
type imageIndex struct {
	Manifests []manifestDescriptor `json:"manifests"`
}

// dockerManifestInspect is overridable so tests can drive the check without
// Docker or network access.
var dockerManifestInspect = func(ctx context.Context, ref string) ([]byte, error) {
	return executil.RunCommandCapture(ctx, "docker", []string{"manifest", "inspect", ref}, nil, "")
}

// collectPinnedImages walks the given values layers and returns every image
// block that pins registry, repository and tag together, deduplicated and sorted.
// The predicate mirrors the yq expression in check-values-enterprise.sh.
func collectPinnedImages(valuesFiles []string) []pinnedImage {
	seen := map[string]string{}
	for _, file := range valuesFiles {
		doc, err := loadValuesDoc(file)
		if err != nil || doc == nil {
			continue
		}
		walkImageBlocks(doc, func(_ string, img map[string]any) {
			ref, ok := fullyPinnedRef(img)
			if !ok {
				return
			}
			if _, exists := seen[ref]; !exists {
				seen[ref] = file
			}
		})
	}

	out := make([]pinnedImage, 0, len(seen))
	for ref, source := range seen {
		out = append(out, pinnedImage{Ref: ref, Source: source})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Ref < out[j].Ref })
	return out
}

// fullyPinnedRef renders "registry/repository:tag" when all three are present
// and non-empty scalars.
func fullyPinnedRef(img map[string]any) (string, bool) {
	registry, okR := nonEmptyScalar(img["registry"])
	repository, okP := nonEmptyScalar(img["repository"])
	tag, okT := nonEmptyScalar(img["tag"])
	if !okR || !okP || !okT {
		return "", false
	}
	return fmt.Sprintf("%s/%s:%s", registry, repository, tag), true
}

// nonEmptyScalar returns a YAML scalar as a string. Only strings are accepted:
// an unquoted `tag: 8.10` reaches here as float64(8.1), whose original text is
// unrecoverable, so it is skipped rather than rendered.
func nonEmptyScalar(v any) (string, bool) {
	s, ok := v.(string)
	if !ok {
		return "", false
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return "", false
	}
	return s, true
}

// imageResolution is the outcome of validating one image reference. Err means
// the registry answered and the answer was negative; Unverified means no
// trustworthy answer was obtained. At most one is set.
type imageResolution struct {
	Ref        string
	Source     string // values file the reference came from
	Platform   string
	Digest     string
	Err        error
	Unverified error
}

// resolveImageForPlatform resolves an image index and asserts that the child
// manifest for the requested platform is fetchable.
//
// Err is set only when the index resolves and its child for the target platform
// does not. A failure to resolve or parse the index sets Unverified. A
// single-platform manifest, and an index advertising no entry for the platform,
// both succeed with neither set.
func resolveImageForPlatform(ctx context.Context, ref, platform string) imageResolution {
	res := imageResolution{Ref: ref, Platform: platform}

	raw, err := dockerManifestInspect(ctx, ref)
	if err != nil {
		res.Unverified = fmt.Errorf("could not resolve the index: %w", err)
		return res
	}

	var index imageIndex
	if err := json.Unmarshal(raw, &index); err != nil {
		res.Unverified = fmt.Errorf("could not parse the manifest: %w", err)
		return res
	}
	if len(index.Manifests) == 0 {
		return res // single-platform image, nothing further to assert
	}

	digest, ok := childDigestForPlatform(index, platform)
	if !ok {
		return res // not built for this platform
	}
	res.Digest = digest

	childRef := repositoryOf(ref) + "@" + digest
	if _, err := dockerManifestInspect(ctx, childRef); err != nil {
		res.Err = fmt.Errorf(
			"the index resolves but its %s child manifest %s is not fetchable: %w",
			platform, digest, err)
	}
	return res
}

// childDigestForPlatform finds the descriptor matching "os/arch". The exact
// match skips the "unknown/unknown" entries attestation manifests carry.
func childDigestForPlatform(index imageIndex, platform string) (string, bool) {
	for _, m := range index.Manifests {
		if m.Platform.OS+"/"+m.Platform.Architecture == platform {
			return m.Digest, true
		}
	}
	return "", false
}

// imageCheckConcurrency bounds the parallel registry round-trips.
const imageCheckConcurrency = 6

// resolveImagesConcurrently validates images in parallel and returns the results
// in the input order, so the reported list stays deterministic.
func resolveImagesConcurrently(ctx context.Context, images []pinnedImage, platform string) []imageResolution {
	results := make([]imageResolution, len(images))
	var g errgroup.Group
	g.SetLimit(imageCheckConcurrency)

	for i, img := range images {
		g.Go(func() error {
			res := resolveImageForPlatform(ctx, img.Ref, platform)
			res.Source = img.Source
			results[i] = res
			return nil
		})
	}
	_ = g.Wait() // every worker records its outcome in results and returns nil
	return results
}

// repositoryOf strips the tag from a reference, leaving registry/repository so a
// digest can be appended. A colon before the last slash is a registry port, not
// a tag separator.
func repositoryOf(ref string) string {
	slash := strings.LastIndex(ref, "/")
	colon := strings.LastIndex(ref, ":")
	if colon > slash {
		return ref[:colon]
	}
	return ref
}

// enterpriseValuesLayers keeps only the enterprise overlay out of a resolved
// values chain. The overlay reaches the chain as values/features/enterprise.yaml,
// a symlink to the chart-root values-enterprise.yaml, so symlinks are resolved
// before matching and both names are accepted.
func enterpriseValuesLayers(files []string) []string {
	var out []string
	for _, f := range files {
		name := filepath.Base(f)
		if resolved, err := filepath.EvalSymlinks(f); err == nil {
			name = filepath.Base(resolved)
		}
		if name == "values-enterprise.yaml" || name == "enterprise.yaml" {
			out = append(out, f)
		}
	}
	return out
}

// checkPinnedImages asserts that every image pinned by the enterprise overlay is
// pullable on the target platform, child manifests included. It reports
// StatusFail only for images the registry actively denied, and StatusWarn when
// the check could not run.
func checkPinnedImages(ctx context.Context, valuesFiles []string, platform string) Check {
	const name = "enterprise image manifests"

	images := collectPinnedImages(enterpriseValuesLayers(valuesFiles))
	if len(images) == 0 {
		return Check{
			Name:   name,
			Status: StatusOK,
			Detail: "no enterprise image overlay in the resolved values chain",
		}
	}

	var broken, unverified []string
	checked := 0
	for _, res := range resolveImagesConcurrently(ctx, images, platform) {
		switch {
		case res.Err != nil:
			broken = append(broken, fmt.Sprintf("%s (from %s): %v", res.Ref, res.Source, res.Err))
		case res.Unverified != nil:
			unverified = append(unverified, fmt.Sprintf("%s: %v", res.Ref, res.Unverified))
		default:
			checked++
		}
	}

	if len(broken) > 0 {
		return Check{
			Name:   name,
			Status: StatusFail,
			Detail: fmt.Sprintf("%d of %d enterprise image(s) not pullable on %s:\n    %s",
				len(broken), len(images), platform, strings.Join(broken, "\n    ")),
			Remediation: "the registry serves the index but not the platform child it advertises; a pin change " +
				"will not help — see camunda/camunda-platform-helm#6804. " +
				"Set DEPLOY_CAMUNDA_SKIP_IMAGE_MANIFEST_CHECK=true to deploy anyway",
		}
	}

	if len(unverified) > 0 {
		return Check{
			Name:   name,
			Status: StatusWarn,
			Detail: fmt.Sprintf("could not verify %d of %d enterprise image(s) on %s:\n    %s",
				len(unverified), len(images), platform, strings.Join(unverified, "\n    ")),
			Remediation: "install Docker and log in to the registry to enable this check; " +
				"the in-flight guard still aborts the wait if an image turns out to be unpullable",
		}
	}

	return Check{
		Name:   name,
		Status: StatusOK,
		Detail: fmt.Sprintf("%d enterprise image(s) pullable on %s, child manifests included", checked, platform),
	}
}
