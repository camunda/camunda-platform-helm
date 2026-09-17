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
	"context"
	"slices"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

// The replicate-from stub is applied with empty tls.crt/tls.key, so a present-but-empty key
// must count as not yet replicated. Treating presence as success would let the tests mount an
// empty certificate and fail later, during the TLS handshake, instead of here.
func TestEmptyKeys(t *testing.T) {
	t.Parallel()

	keys := []string{"tls.crt", "tls.key"}

	cases := []struct {
		name string
		data map[string][]byte
		want []string
	}{
		{
			name: "freshly applied stub has both keys empty",
			data: map[string][]byte{"tls.crt": {}, "tls.key": {}},
			want: []string{"tls.crt", "tls.key"},
		},
		{
			name: "replicator populated both",
			data: map[string][]byte{"tls.crt": []byte("cert"), "tls.key": []byte("key")},
			want: nil,
		},
		{
			name: "partially replicated",
			data: map[string][]byte{"tls.crt": []byte("cert"), "tls.key": {}},
			want: []string{"tls.key"},
		},
		{
			name: "key absent entirely",
			data: map[string][]byte{"tls.crt": []byte("cert")},
			want: []string{"tls.key"},
		},
		{
			name: "secret has no data at all",
			data: nil,
			want: []string{"tls.crt", "tls.key"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := emptyKeys(tc.data, keys)
			if !slices.Equal(got, tc.want) {
				t.Fatalf("emptyKeys() = %v, want %v", got, tc.want)
			}
		})
	}
}

func externalSecret(name, target string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "external-secrets.io/v1",
		"kind":       "ExternalSecret",
		"metadata":   map[string]any{"name": name, "namespace": "ns"},
		"spec":       map[string]any{"target": map[string]any{"name": target}},
	}}
}

// Only the ExternalSecrets writing into the replicated Secret may be deleted. Removing the
// others would take out the integration-test credentials that the same namespace depends on.
func TestDeleteExternalSecretsTargeting(t *testing.T) {
	t.Parallel()

	dynamicClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
		runtime.NewScheme(),
		map[schema.GroupVersionResource]string{externalSecretGVR: "ExternalSecretList"},
		externalSecret("external-secret-eks-tls", "aws-camunda-cloud-tls"),
		externalSecret("another-tls-writer", "aws-camunda-cloud-tls"),
		externalSecret("external-secret-credentials", "integration-test-credentials"),
	)
	client := &Client{dynamicClient: dynamicClient}

	if err := deleteExternalSecretsTargeting(context.Background(), client, "ns", "aws-camunda-cloud-tls"); err != nil {
		t.Fatalf("deleteExternalSecretsTargeting() error = %v", err)
	}

	list, err := dynamicClient.Resource(externalSecretGVR).Namespace("ns").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	var left []string
	for _, item := range list.Items {
		left = append(left, item.GetName())
	}
	if !slices.Equal(left, []string{"external-secret-credentials"}) {
		t.Fatalf("remaining ExternalSecrets = %v, want [external-secret-credentials]", left)
	}
}
