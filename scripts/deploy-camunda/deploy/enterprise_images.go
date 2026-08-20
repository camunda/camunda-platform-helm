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
	"sync"

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

// nonEmptyScalar returns a YAML scalar as a string, and only accepts strings.
//
// Non-string scalars are rejected rather than formatted. YAML types an unquoted
// `tag: 8.10` as a float, and by the time it reaches here it is float64(8.1) —
// the trailing zero is gone and the original text is unrecoverable. Rendering it
// would produce "repo:8.1", a reference nobody wrote, and the check would fail an
// image that is perfectly fine. Skipping is the safe direction: a tag we cannot
// reconstruct faithfully is simply not asserted.
//
// In practice this costs nothing — real tags are either quoted or contain a
// hyphen or a second dot ("15.18.0-debian-12-r17", "8.19.20"), so YAML already
// types them as strings.
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

// imageResolution is the outcome of validating one image reference.
//
// Err and Unverified are deliberately distinct. Err means the registry answered
// and the answer was "this is not here", which is actionable. Unverified means
// we could not get a trustworthy answer at all — no docker binary, no
// credentials, no network — which says nothing about the image and must never
// fail a deploy.
type imageResolution struct {
	Ref      string
	Platform string
	Digest   string
	Err      error
	// Unverified carries the reason the check could not run, when it could not.
	Unverified error
}

// resolveImageForPlatform resolves an image index and asserts that the child
// manifest for the requested platform is actually fetchable.
//
// Only one condition is reported as a hard failure: **the index resolves and its
// child for the target platform does not**. That is unambiguous — we
// authenticated, the registry served the index, and it then denied a digest its
// own index advertises. It is also exactly the incident shape (#6804).
//
// Everything else is Unverified, not a failure:
//   - no docker binary, no credentials, no network — we learned nothing;
//   - the index itself not resolving — indistinguishable from the above without
//     parsing vendor-specific error text, and the in-flight guard catches a
//     genuinely absent image anyway, with the kubelet's own message.
//
// Two outcomes are plain success: a single-platform manifest with no children,
// and an index that advertises no entry for the target platform (an image never
// built for it is not a broken image).
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

	// This is the request an index-only check never makes, and the one that
	// actually failed in the incidents: fetch the child by digest.
	childRef := repositoryOf(ref) + "@" + digest
	if _, err := dockerManifestInspect(ctx, childRef); err != nil {
		res.Err = fmt.Errorf(
			"the index resolves but its %s child manifest %s is not fetchable: %w",
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

// imageCheckConcurrency bounds the parallel registry round-trips. Each
// `docker manifest inspect` carries its own auth handshake and takes seconds, so
// checking a dozen images serially would add minutes to every deploy. Bounded so
// a large values chain cannot hammer the registry.
const imageCheckConcurrency = 6

// checkedImage pairs a resolution outcome with the layer the image came from.
type checkedImage struct {
	ref        string
	source     string
	err        error
	unverified error
}

// resolveImagesConcurrently validates images in parallel and returns the results
// in the input order, so the reported list stays deterministic.
func resolveImagesConcurrently(ctx context.Context, images []pinnedImage, platform string) []checkedImage {
	results := make([]checkedImage, len(images))
	sem := make(chan struct{}, imageCheckConcurrency)
	var wg sync.WaitGroup

	for i, img := range images {
		wg.Add(1)
		go func(i int, img pinnedImage) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			res := resolveImageForPlatform(ctx, img.Ref, platform)
			results[i] = checkedImage{
				ref: img.Ref, source: img.Source,
				err: res.Err, unverified: res.Unverified,
			}
		}(i, img)
	}
	wg.Wait()
	return results
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

// enterpriseValuesLayers keeps only the enterprise overlay out of a resolved
// values chain.
//
// Scoping matters: ordinary scenario layers pin images completely too — for
// example values/identity/keycloak.yaml pins docker.io/camunda/keycloak — so
// validating every fully-pinned image would put registry round-trips on the
// critical path of every deploy, for images that have never exhibited this
// defect. The enterprise overlay is the one that goes through the vendor-ee
// proxy cache, which is where partially-cached indexes come from.
//
// The overlay reaches the chain as values/features/enterprise.yaml, a symlink to
// the chart-root values-enterprise.yaml, so both names are accepted and symlinks
// are resolved before matching.
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
// pullable on the target platform, child manifests included.
//
// Failure semantics are deliberately asymmetric. Only "the registry served the
// index and then denied the child it advertises" is a hard failure — the deploy
// genuinely cannot succeed. Anything that merely prevented the check from
// running (no docker, no credentials, no network) is a warning: it says nothing
// about the images, and a preflight must not block a deploy over its own
// inability to look.
func checkPinnedImages(ctx context.Context, valuesFiles []string, platform string) Check {
	const name = "enterprise image manifests"
	if platform == "" {
		platform = defaultImagePlatform
	}

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
		case res.err != nil:
			broken = append(broken, fmt.Sprintf("%s (from %s): %v", res.ref, res.source, res.err))
		case res.unverified != nil:
			unverified = append(unverified, fmt.Sprintf("%s: %v", res.ref, res.unverified))
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
