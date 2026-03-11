package kube

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func newTestClient(objects ...runtime.Object) *Client {
	fakeClient := fake.NewSimpleClientset(objects...)
	return &Client{
		clientset:   fakeClient,
		kubeContext: "test-context",
	}
}

func forbiddenReactor() k8stesting.ReactionFunc {
	return func(action k8stesting.Action) (bool, runtime.Object, error) {
		if action.GetVerb() == "patch" {
			return true, nil, apierrors.NewForbidden(
				schema.GroupResource{Resource: "secrets"},
				action.(k8stesting.PatchAction).GetName(),
				nil,
			)
		}
		return false, nil, nil
	}
}

func applyCreatesReactor() k8stesting.ReactionFunc {
	return func(action k8stesting.Action) (bool, runtime.Object, error) {
		if action.GetVerb() != "patch" {
			return false, nil, nil
		}
		patchAction := action.(k8stesting.PatchAction)
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      patchAction.GetName(),
				Namespace: patchAction.GetNamespace(),
			},
			Type: corev1.SecretTypeDockerConfigJson,
			Data: map[string][]byte{corev1.DockerConfigJsonKey: patchAction.GetPatch()},
		}
		return true, secret, nil
	}
}

func TestEnsureRegistrySecret_ApplySucceeds(t *testing.T) {
	client := newTestClient()
	client.clientset.(*fake.Clientset).PrependReactor("patch", "secrets", applyCreatesReactor())

	err := client.EnsureRegistrySecret(context.Background(), "ns", "my-secret", "https://registry.example.com", "user", "pass")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestEnsureRegistrySecret_ForbiddenFallsBackToCreate(t *testing.T) {
	client := newTestClient()
	client.clientset.(*fake.Clientset).PrependReactor("patch", "secrets", forbiddenReactor())

	err := client.EnsureRegistrySecret(context.Background(), "ns", "my-secret", "https://registry.example.com", "user", "pass")
	if err != nil {
		t.Fatalf("expected fallback to succeed, got: %v", err)
	}

	secret, err := client.clientset.CoreV1().Secrets("ns").Get(context.Background(), "my-secret", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("expected secret to exist after fallback create, got: %v", err)
	}
	if secret.Type != corev1.SecretTypeDockerConfigJson {
		t.Errorf("expected type %s, got %s", corev1.SecretTypeDockerConfigJson, secret.Type)
	}
}

func TestEnsureRegistrySecret_ForbiddenFallsBackToUpdate(t *testing.T) {
	existing := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "my-secret", Namespace: "ns"},
		Type:       corev1.SecretTypeDockerConfigJson,
		Data:       map[string][]byte{corev1.DockerConfigJsonKey: []byte(`{"old":"data"}`)},
	}
	client := newTestClient(existing)
	client.clientset.(*fake.Clientset).PrependReactor("patch", "secrets", forbiddenReactor())

	err := client.EnsureRegistrySecret(context.Background(), "ns", "my-secret", "https://registry.example.com", "newuser", "newpass")
	if err != nil {
		t.Fatalf("expected fallback update to succeed, got: %v", err)
	}

	secret, err := client.clientset.CoreV1().Secrets("ns").Get(context.Background(), "my-secret", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("expected secret to exist after fallback update, got: %v", err)
	}

	var config map[string]any
	if err := json.Unmarshal(secret.Data[corev1.DockerConfigJsonKey], &config); err != nil {
		t.Fatalf("failed to unmarshal docker config: %v", err)
	}
	auths, ok := config["auths"].(map[string]any)
	if !ok {
		t.Fatal("expected auths key in docker config")
	}
	reg, ok := auths["https://registry.example.com"].(map[string]any)
	if !ok {
		t.Fatal("expected registry entry in auths")
	}
	if reg["username"] != "newuser" {
		t.Errorf("expected updated username 'newuser', got %v", reg["username"])
	}
}

func TestEnsureRegistrySecret_EmptyCredentials(t *testing.T) {
	client := newTestClient()

	tests := []struct {
		name     string
		username string
		password string
	}{
		{"empty username", "", "pass"},
		{"empty password", "user", ""},
		{"both empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := client.EnsureRegistrySecret(context.Background(), "ns", "s", "https://r.io", tt.username, tt.password)
			if err == nil {
				t.Fatal("expected error for empty credentials")
			}
		})
	}
}

func TestEnsureOpaqueSecret_ApplySucceeds(t *testing.T) {
	client := newTestClient()
	client.clientset.(*fake.Clientset).PrependReactor("patch", "secrets", applyCreatesReactor())

	err := client.EnsureOpaqueSecret(context.Background(), "ns", "my-opaque", map[string]string{"key": "value"})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestEnsureOpaqueSecret_ForbiddenFallsBackToCreate(t *testing.T) {
	client := newTestClient()
	client.clientset.(*fake.Clientset).PrependReactor("patch", "secrets", forbiddenReactor())

	err := client.EnsureOpaqueSecret(context.Background(), "ns", "my-opaque", map[string]string{"key": "value"})
	if err != nil {
		t.Fatalf("expected fallback to succeed, got: %v", err)
	}

	secret, err := client.clientset.CoreV1().Secrets("ns").Get(context.Background(), "my-opaque", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("expected secret to exist after fallback, got: %v", err)
	}
	if secret.Type != corev1.SecretTypeOpaque {
		t.Errorf("expected type %s, got %s", corev1.SecretTypeOpaque, secret.Type)
	}
}

func TestEnsureOpaqueSecret_ForbiddenFallsBackToUpdate(t *testing.T) {
	existing := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "my-opaque", Namespace: "ns"},
		Type:       corev1.SecretTypeOpaque,
		StringData: map[string]string{"old": "data"},
	}
	client := newTestClient(existing)
	client.clientset.(*fake.Clientset).PrependReactor("patch", "secrets", forbiddenReactor())

	err := client.EnsureOpaqueSecret(context.Background(), "ns", "my-opaque", map[string]string{"new": "data"})
	if err != nil {
		t.Fatalf("expected fallback update to succeed, got: %v", err)
	}

	secret, err := client.clientset.CoreV1().Secrets("ns").Get(context.Background(), "my-opaque", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("expected secret to exist after fallback update, got: %v", err)
	}
	if secret.StringData["new"] != "data" {
		t.Errorf("expected updated string data, got %v", secret.StringData)
	}
}

func TestEnsureOpaqueSecret_EmptyName(t *testing.T) {
	client := newTestClient()
	err := client.EnsureOpaqueSecret(context.Background(), "ns", "", map[string]string{"k": "v"})
	if err == nil {
		t.Fatal("expected error for empty secret name")
	}
}

func TestCheckNamespaceTerminating(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error", nil, false},
		{"unrelated error", apierrors.NewNotFound(schema.GroupResource{}, ""), false},
		{"is being terminated", apierrors.NewConflict(schema.GroupResource{}, "", fmt.Errorf("namespace is being terminated")), true},
		{"because it is being terminated", apierrors.NewConflict(schema.GroupResource{}, "", fmt.Errorf("because it is being terminated")), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := checkNamespaceTerminating(tt.err); got != tt.want {
				t.Errorf("checkNamespaceTerminating() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildDockerConfigJSON(t *testing.T) {
	data, err := buildDockerConfigJSON("https://registry.example.com", "user", "pass")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var config map[string]any
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	auths, ok := config["auths"].(map[string]any)
	if !ok {
		t.Fatal("expected auths key")
	}
	reg, ok := auths["https://registry.example.com"].(map[string]any)
	if !ok {
		t.Fatal("expected registry entry")
	}
	if reg["username"] != "user" {
		t.Errorf("expected username 'user', got %v", reg["username"])
	}
	if reg["password"] != "pass" {
		t.Errorf("expected password 'pass', got %v", reg["password"])
	}
	if _, ok := reg["auth"]; !ok {
		t.Error("expected auth field")
	}
}
