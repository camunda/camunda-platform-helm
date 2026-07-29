package kube

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestDeleteNamespaceOwnedBy(t *testing.T) {
	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name: "owned", UID: types.UID("uid-1"), Labels: map[string]string{"deploy-camunda-run": "run-1"},
	}}
	client := newTestClient(namespace)
	if err := client.DeleteNamespaceOwnedBy(context.Background(), "owned", "deploy-camunda-run", "run-1"); err != nil {
		t.Fatal(err)
	}
	_, err := client.clientset.CoreV1().Namespaces().Get(context.Background(), "owned", metav1.GetOptions{})
	if !apierrors.IsNotFound(err) {
		t.Fatalf("namespace still exists: %v", err)
	}
}

func TestDeleteNamespaceOwnedByRejectsMismatch(t *testing.T) {
	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name: "owned", UID: types.UID("uid-1"), Labels: map[string]string{"deploy-camunda-run": "other-run"},
	}}
	client := newTestClient(namespace)
	if err := client.DeleteNamespaceOwnedBy(context.Background(), "owned", "deploy-camunda-run", "run-1"); err == nil {
		t.Fatal("expected ownership mismatch")
	}
}

func TestDeleteNamespaceOwnedByAllowsMissing(t *testing.T) {
	if err := newTestClient().DeleteNamespaceOwnedBy(context.Background(), "missing", "deploy-camunda-run", "run-1"); err != nil {
		t.Fatal(err)
	}
}
