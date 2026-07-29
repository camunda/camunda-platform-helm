package cmd

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
)

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
