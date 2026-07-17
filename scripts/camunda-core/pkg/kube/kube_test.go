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
