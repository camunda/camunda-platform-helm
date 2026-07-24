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
	"testing"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestListNamespacedResources(t *testing.T) {
	resource := schema.GroupVersionResource{Group: "gateway.networking.k8s.io", Version: "v1", Resource: "httproutes"}
	route := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "gateway.networking.k8s.io/v1",
		"kind":       "HTTPRoute",
		"metadata": map[string]any{
			"name":      "integration-route",
			"namespace": "test-namespace",
		},
	}}
	dynamicClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
		runtime.NewScheme(),
		map[schema.GroupVersionResource]string{resource: "HTTPRouteList"},
		route,
	)

	client := &Client{dynamicClient: dynamicClient}
	resources, err := client.ListNamespacedResources(context.Background(), "test-namespace", resource)
	if err != nil {
		t.Fatalf("ListNamespacedResources() error = %v", err)
	}
	if len(resources.Items) != 1 || resources.Items[0].GetName() != "integration-route" {
		t.Fatalf("ListNamespacedResources() items = %#v", resources.Items)
	}
}

func TestListNamespacedResourcesRejectsEmptyNamespace(t *testing.T) {
	client := &Client{dynamicClient: dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())}
	_, err := client.ListNamespacedResources(
		context.Background(),
		"",
		schema.GroupVersionResource{Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"},
	)
	if err == nil {
		t.Fatal("ListNamespacedResources() expected an error for an empty namespace")
	}
}

func TestCheckConnectivity_InvalidContext(t *testing.T) {
	err := CheckConnectivity(context.Background(), "nonexistent-context-that-should-not-exist")
	if err == nil {
		t.Fatal("expected error for non-existent kube context, got nil")
	}
}

func TestApplyManifestGatewayUsesDiscoveredResource(t *testing.T) {
	groupVersion := schema.GroupVersion{Group: "gateway.networking.k8s.io", Version: "v1"}
	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{groupVersion})
	mapper.AddSpecific(
		groupVersion.WithKind("Gateway"),
		groupVersion.WithResource("gateways"),
		groupVersion.WithResource("gateway"),
		meta.RESTScopeNamespace,
	)

	dynamicClient := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
	var appliedResource string
	dynamicClient.PrependReactor("patch", "*", func(action k8stesting.Action) (bool, runtime.Object, error) {
		appliedResource = action.GetResource().Resource
		return true, &unstructured.Unstructured{}, nil
	})

	client := &Client{dynamicClient: dynamicClient, restMapper: mapper}
	err := applySingleManifestObject(context.Background(), client, "test-namespace", map[string]any{
		"apiVersion": groupVersion.String(),
		"kind":       "Gateway",
		"metadata": map[string]any{
			"name": "test-gateway",
		},
	}, 1)
	if err != nil {
		t.Fatalf("applySingleManifestObject() error = %v", err)
	}
	if appliedResource != "gateways" {
		t.Fatalf("applied resource = %q, want %q", appliedResource, "gateways")
	}
}
