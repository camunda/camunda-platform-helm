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

package controller

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/yaml"

	operatorhelm "operator/internal/helm"
)

// driftResult reports what a drift check found.
type driftResult struct {
	Drifted bool
	Reason  string
	Message string
	Digest  string
}

// detectDrift compares the desired release against what is actually running.
//
// Two things are checked, both chosen because they cannot produce a false
// positive:
//
//   - A rendered object that no longer exists in the cluster. Someone deleted it,
//     and an upgrade will put it back.
//   - A rendered manifest whose digest differs from the one last applied. Chart
//     content can change under a mutable tag without the chart version or values
//     moving, which the revision comparison alone would miss.
//
// Field-level comparison of live objects is deliberately not attempted. Defaulted
// fields, admission webhooks and other field managers all rewrite parts of an
// object legitimately, so a naive diff reports drift that is not drift. Detecting
// hand-edits to individual fields therefore remains a gap rather than a
// half-working feature.
func (r *CamundaHubReconciler) detectDrift(
	ctx context.Context, desiredManifest, lastDigest, namespace string,
) driftResult {
	digest := operatorhelm.ManifestDigest(desiredManifest)

	if lastDigest != "" && digest != lastDigest {
		return driftResult{
			Drifted: true, Reason: "RenderedManifestChanged", Digest: digest,
			Message: "the chart renders differently than when it was last applied, " +
				"even though its version and values are unchanged",
		}
	}

	missing, err := r.missingObjects(ctx, desiredManifest, namespace)
	if err != nil {
		// A failed check is not evidence of drift.
		return driftResult{Digest: digest}
	}
	if len(missing) > 0 {
		return driftResult{
			Drifted: true, Reason: "ObjectsMissing", Digest: digest,
			Message: fmt.Sprintf("%d object(s) from the release are no longer present: %s",
				len(missing), strings.Join(missing, ", ")),
		}
	}

	return driftResult{Digest: digest}
}

// missingObjects returns the identities of rendered objects absent from the cluster.
func (r *CamundaHubReconciler) missingObjects(
	ctx context.Context, manifest, namespace string,
) ([]string, error) {
	objects, err := decodeManifest(manifest, namespace)
	if err != nil {
		return nil, err
	}

	var missing []string
	for _, obj := range objects {
		var live unstructured.Unstructured
		live.SetGroupVersionKind(obj.GroupVersionKind())

		err := r.Get(ctx, types.NamespacedName{
			Namespace: obj.GetNamespace(), Name: obj.GetName(),
		}, &live)
		switch {
		case err == nil:
			continue
		case errors.IsNotFound(err):
			missing = append(missing, fmt.Sprintf("%s/%s", obj.GetKind(), obj.GetName()))
		case meta.IsNoMatchError(err) || errors.IsMethodNotSupported(err):
			// A kind the cluster does not serve cannot be compared; skip rather
			// than report drift the operator could not fix anyway.
			continue
		default:
			return nil, fmt.Errorf("read %s/%s: %w", obj.GetKind(), obj.GetName(), err)
		}
	}
	return missing, nil
}

func decodeManifest(manifest, namespace string) ([]*unstructured.Unstructured, error) {
	var objects []*unstructured.Unstructured

	for _, chunk := range strings.Split(manifest, "\n---") {
		trimmed := strings.TrimSpace(chunk)
		if trimmed == "" {
			continue
		}

		var raw map[string]any
		if err := yaml.Unmarshal([]byte(trimmed), &raw); err != nil {
			return nil, fmt.Errorf("parse rendered manifest: %w", err)
		}
		if len(raw) == 0 {
			continue
		}

		obj := &unstructured.Unstructured{Object: raw}
		if obj.GetKind() == "" || obj.GetName() == "" {
			continue
		}
		if obj.GetNamespace() == "" {
			obj.SetNamespace(namespace)
		}
		objects = append(objects, obj)
	}
	return objects, nil
}
