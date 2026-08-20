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
	"sort"
	"strings"

	"scripts/camunda-core/pkg/executil"
)

// Why this check exists, and what it does *not* claim to be.
//
// `scripts/check-values-enterprise.sh` validates enterprise image references with
// `docker manifest inspect`, which resolves the multi-arch *index* and stops
// there. The failure mode this repository actually keeps hitting is the opposite
// shape: the index resolves while the per-platform child manifest it references
// returns 404, so an index-only probe reports a healthy image that no node can
// pull. See #6921 and #6804.
//
// This check reproduces the request sequence containerd performs — resolve the
// index, then fetch the child manifest for the target platform by digest — so it
// has the same fidelity as a real pull. It is deliberately *not* a probe of
// registry retention correctness: a registry-protocol GET can self-heal by
// re-proxying, so a green result here means "pullable right now", not "correctly
// cached". Retention correctness is a registry-side concern and lives in
// camunda/infra-core.
//
// Cost is proportional to the number of fully-pinned images in the resolved
// values chain. Only enterprise overlays pin registry+repository+tag together,
// so ordinary scenarios resolve zero images and pay nothing.

// defaultImagePlatform is the platform asserted when none is configured. CI
// nodes and the large majority of customer clusters are linux/amd64, and every
// incident so far has been amd64-only while arm64 stayed healthy.
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

// dockerManifestInspect is a package-level variable so tests can drive the check
// without Docker or network access.
var dockerManifestInspect = func(ctx context.Context, ref string) ([]byte, error) {
	return executil.RunCommandCapture(ctx, "docker", []string{"manifest", "inspect", ref}, nil, "")
}

// collectPinnedImages walks the resolved values layers and returns every image
// block that pins registry, repository and tag together, deduplicated and
// sorted.
//
// The predicate mirrors the yq expression in check-values-enterprise.sh
// (`has("registry") and has("repository") and has("tag")`) so both tools agree on
// what counts as a fully-pinned image.
func collectPinnedImages(valuesFiles []string) []pinnedImage {
	seen := map[string]string{}
	for _, file := range valuesFiles {
		doc, err := loadValuesDoc(file)
		if err != nil || doc == nil {
			// A layer that cannot be read is the scenario preflight's problem, not
			// this check's: staying silent avoids reporting the same defect twice.
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

// nonEmptyScalar renders a YAML scalar as a string, rejecting nil and empty.
// Numeric tags (e.g. `tag: 8.19`) are accepted, since YAML types them as floats.
func nonEmptyScalar(v any) (string, bool) {
	if v == nil {
		return "", false
	}
	s := strings.TrimSpace(fmt.Sprintf("%v", v))
	if s == "" {
		return "", false
	}
	return s, true
}

// imageResolution is the outcome of validating one image reference.
type imageResolution struct {
	Ref      string
	Platform string
	Digest   string
	Err      error
}

// resolveImageForPlatform resolves an image index and asserts that the child
// manifest for the requested platform is actually fetchable.
//
// Three outcomes are treated as success:
//   - the index advertises the platform and the child manifest fetches;
//   - the reference is a single-platform manifest with no index (no children to
//     check, so resolving the reference is the whole assertion);
//   - the index advertises no entry for the platform at all — that is an image
//     that was never built for it, not a broken cache, and is not this check's
//     business.
func resolveImageForPlatform(ctx context.Context, ref, platform string) imageResolution {
	res := imageResolution{Ref: ref, Platform: platform}

	raw, err := dockerManifestInspect(ctx, ref)
	if err != nil {
		res.Err = fmt.Errorf("index does not resolve: %w", err)
		return res
	}

	var index imageIndex
	if err := json.Unmarshal(raw, &index); err != nil {
		res.Err = fmt.Errorf("cannot parse manifest for %s: %w", ref, err)
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

	// This is the request an index-only check never makes, and the one that
	// actually failed in the incidents: fetch the child by digest.
	childRef := repositoryOf(ref) + "@" + digest
	if _, err := dockerManifestInspect(ctx, childRef); err != nil {
		res.Err = fmt.Errorf(
			"index resolves but its %s child manifest %s is not fetchable: %w",
			platform, digest, err)
	}
	return res
}

// childDigestForPlatform finds the descriptor matching "os/arch". The synthetic
// "unknown/unknown" entries that attestation manifests carry are skipped by the
// exact match.
func childDigestForPlatform(index imageIndex, platform string) (string, bool) {
	for _, m := range index.Manifests {
		if m.Platform.OS+"/"+m.Platform.Architecture == platform {
			return m.Digest, true
		}
	}
	return "", false
}

// repositoryOf strips the tag from a reference, leaving registry/repository so a
// digest can be appended. The last colon is only a tag separator when it comes
// after the last slash — otherwise it is a registry port.
func repositoryOf(ref string) string {
	slash := strings.LastIndex(ref, "/")
	colon := strings.LastIndex(ref, ":")
	if colon > slash {
		return ref[:colon]
	}
	return ref
}

// checkPinnedImages asserts that every fully-pinned image in the resolved values
// chain is pullable on the target platform, children included.
//
// Failure semantics: a broken image is StatusFail, because the deploy cannot
// succeed. Everything else — no pinned images, Docker missing, values layers
// unresolvable — is OK or a warning, so this check never blocks a deploy for a
// reason unrelated to image availability.
func checkPinnedImages(ctx context.Context, valuesFiles []string, platform string) Check {
	const name = "enterprise image manifests"
	if platform == "" {
		platform = defaultImagePlatform
	}

	images := collectPinnedImages(valuesFiles)
	if len(images) == 0 {
		return Check{
			Name:   name,
			Status: StatusOK,
			Detail: "no fully-pinned images in the resolved values chain",
		}
	}

	var broken []string
	checked := 0
	for _, img := range images {
		res := resolveImageForPlatform(ctx, img.Ref, platform)
		if res.Err == nil {
			checked++
			continue
		}
		broken = append(broken, fmt.Sprintf("%s (from %s): %v", img.Ref, img.Source, res.Err))
	}

	if len(broken) == 0 {
		return Check{
			Name:   name,
			Status: StatusOK,
			Detail: fmt.Sprintf("%d image(s) pullable on %s, child manifests included", checked, platform),
		}
	}

	return Check{
		Name:   name,
		Status: StatusFail,
		Detail: fmt.Sprintf("%d of %d image(s) not pullable on %s:\n    %s",
			len(broken), len(images), platform, strings.Join(broken, "\n    ")),
		Remediation: "the registry cannot serve these references; a pin change will not help if the index resolves " +
			"but its child manifest does not — see camunda/camunda-platform-helm#6804. " +
			"Set DEPLOY_CAMUNDA_SKIP_IMAGE_MANIFEST_CHECK=true to deploy anyway",
	}
}
