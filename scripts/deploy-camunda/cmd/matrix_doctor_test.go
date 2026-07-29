package cmd

import (
	"context"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"k8s.io/apimachinery/pkg/types"
	"scripts/deploy-camunda/entra"
	"scripts/deploy-camunda/matrix"
)

type fakeCleanupClient struct{ deleted bool }

func (f *fakeCleanupClient) OwnedNamespaceIdentity(context.Context, string, string, string) (types.UID, string, error) {
	return types.UID("uid-1"), "rv-1", nil
}

func (f *fakeCleanupClient) DeleteNamespaceWithIdentity(context.Context, string, types.UID, string) error {
	f.deleted = true
	return nil
}

func TestImportMatrixDockerAuthPreservesEnvironmentPair(t *testing.T) {
	t.Setenv("HARBOR_USERNAME", "environment-user")
	t.Setenv("HARBOR_PASSWORD", "environment-password")
	t.Setenv("DOCKERHUB_USERNAME", "")
	t.Setenv("DOCKERHUB_PASSWORD", "")
	t.Setenv("TEST_DOCKER_USERNAME", "")
	t.Setenv("TEST_DOCKER_PASSWORD", "")
	t.Setenv("TEST_DOCKER_USERNAME_CAMUNDA_CLOUD", "")
	t.Setenv("TEST_DOCKER_PASSWORD_CAMUNDA_CLOUD", "")
	t.Setenv("NEXUS_USERNAME", "")
	t.Setenv("NEXUS_PASSWORD", "")
	path := filepath.Join(t.TempDir(), "config.json")
	auth := base64.StdEncoding.EncodeToString([]byte("docker-user:docker-password"))
	if err := os.WriteFile(path, []byte(`{"auths":{"registry.camunda.cloud":{"auth":"`+auth+`"}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	user, password, hubUser, hubPassword := "", "", "", ""
	if err := resolveMatrixDockerCredentialPairs(&user, &password, &hubUser, &hubPassword); err != nil {
		t.Fatal(err)
	}
	if err := importMatrixDockerAuth(path, &user, &password, &hubUser, &hubPassword); err != nil {
		t.Fatal(err)
	}
	if user != "environment-user" || password != "environment-password" {
		t.Fatalf("Docker auth overrode environment pair: %q/%q", user, password)
	}
}

func TestImportMatrixDockerAuthDoesNotMixExplicitPair(t *testing.T) {
	t.Setenv("DOCKERHUB_USERNAME", "")
	t.Setenv("DOCKERHUB_PASSWORD", "")
	t.Setenv("TEST_DOCKER_USERNAME", "")
	t.Setenv("TEST_DOCKER_PASSWORD", "")
	t.Setenv("HARBOR_USERNAME", "")
	t.Setenv("HARBOR_PASSWORD", "")
	t.Setenv("TEST_DOCKER_USERNAME_CAMUNDA_CLOUD", "")
	t.Setenv("TEST_DOCKER_PASSWORD_CAMUNDA_CLOUD", "")
	t.Setenv("NEXUS_USERNAME", "")
	t.Setenv("NEXUS_PASSWORD", "")
	path := filepath.Join(t.TempDir(), "config.json")
	auth := base64.StdEncoding.EncodeToString([]byte("docker-user:docker-password"))
	if err := os.WriteFile(path, []byte(`{"auths":{"registry.camunda.cloud":{"auth":"`+auth+`"}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	user, password, hubUser, hubPassword := "explicit-user", "", "", ""
	if err := resolveMatrixDockerCredentialPairs(&user, &password, &hubUser, &hubPassword); err == nil {
		t.Fatal("expected partial explicit credential pair failure")
	}
}

func TestMatrixCleanupMarksOwnedEntryCleaned(t *testing.T) {
	root := t.TempDir()
	entry := matrix.Entry{Version: "8.10", Shortname: "one", Scenario: "first", Flow: "install"}
	store := matrix.NewRunStateStore(root, "run-1")
	_, err := store.Create([]matrix.Entry{entry}, matrix.RunOptions{NamespacePrefix: "matrix", KubeContext: "test"})
	if err != nil {
		t.Fatal(err)
	}
	fake := &fakeCleanupClient{}
	original := newCleanupKubeClient
	newCleanupKubeClient = func(string) (cleanupKubeClient, error) { return fake, nil }
	t.Cleanup(func() { newCleanupKubeClient = original })

	cmd := newMatrixCleanupCommand()
	cmd.SetArgs([]string{"run-1", "--state-dir", root, "--yes"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !fake.deleted {
		t.Fatal("namespace was not deleted")
	}
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !state.Entries[0].Cleaned {
		t.Fatal("entry was not marked cleaned")
	}
}

func TestMatrixCleanupPreservesNamespaceOnIdentityFailure(t *testing.T) {
	root := t.TempDir()
	envFile := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(envFile, []byte("ENTRA_APP_DIRECTORY_ID=tenant\nENTRA_APP_CLIENT_ID=client\nENTRA_APP_CLIENT_SECRET=secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	entry := matrix.Entry{Version: "8.10", Shortname: "oidc", Scenario: "oidc", Flow: "install", Auth: "oidc", Identity: "oidc"}
	store := matrix.NewRunStateStore(root, "run-1")
	_, err := store.Create([]matrix.Entry{entry}, matrix.RunOptions{NamespacePrefix: "matrix", KubeContext: "test", EnvFile: envFile})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.RecordExternalResources(entry, "", "tenant", "", nil); err != nil {
		t.Fatal(err)
	}
	fake := &fakeCleanupClient{}
	providerCalled := false
	originalClient, originalEntra := newCleanupKubeClient, cleanupEntraResources
	newCleanupKubeClient = func(string) (cleanupKubeClient, error) { return fake, nil }
	cleanupEntraResources = func(context.Context, entra.Options) error {
		providerCalled = true
		return errors.New("provider unavailable")
	}
	t.Cleanup(func() { newCleanupKubeClient, cleanupEntraResources = originalClient, originalEntra })
	cmd := newMatrixCleanupCommand()
	cmd.SetArgs([]string{"run-1", "--state-dir", root, "--yes"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected identity cleanup failure")
	}
	if fake.deleted {
		t.Fatal("namespace deleted after identity cleanup failure")
	}
	if !providerCalled {
		t.Fatal("provider cleanup was not called")
	}
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if state.Entries[0].Cleaned {
		t.Fatal("entry marked cleaned after identity cleanup failure")
	}
}
