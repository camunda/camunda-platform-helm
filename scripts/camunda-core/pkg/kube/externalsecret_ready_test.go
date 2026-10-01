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

package kube

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func externalSecretObj(generation int64, synced string, ready string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "external-secrets.io/v1",
		"kind":       "ExternalSecret",
		"metadata":   map[string]any{"name": "es", "generation": generation},
	}}
	status := map[string]any{}
	if synced != "" {
		status["syncedResourceVersion"] = synced
	}
	if ready != "" {
		status["conditions"] = []any{map[string]any{"type": "Ready", "status": ready}}
	}
	obj.Object["status"] = status
	return obj
}

func TestExternalSecretSynced(t *testing.T) {
	cases := []struct {
		name string
		obj  *unstructured.Unstructured
		want bool
	}{
		{"ready for current generation", externalSecretObj(2, "2-abc123", "True"), true},
		// Re-applying an existing ExternalSecret with a new source bumps its
		// generation while the Ready condition from the previous sync remains.
		{"stale ready from previous generation", externalSecretObj(3, "2-abc123", "True"), false},
		{"current generation not ready", externalSecretObj(2, "2-abc123", "False"), false},
		{"never synced", externalSecretObj(1, "", "True"), false},
		{"no conditions yet", externalSecretObj(1, "1-abc123", ""), false},
		{"generation prefix only matches whole number", externalSecretObj(1, "12-abc123", "True"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := externalSecretSynced(tc.obj); got != tc.want {
				t.Errorf("externalSecretSynced() = %v, want %v", got, tc.want)
			}
		})
	}
}
