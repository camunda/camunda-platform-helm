package cmd

import (
	"bytes"
	"errors"
	"os"
	"testing"

	"scripts/deploy-camunda/config"
	"scripts/deploy-camunda/credentials"
	"scripts/deploy-camunda/matrix"
)

type memoryCredentialStore struct {
	values map[string]credentials.Credential
	err    error
}

func (s *memoryCredentialStore) Get(registry string) (credentials.Credential, bool, error) {
	if s.err != nil {
		return credentials.Credential{}, false, s.err
	}
	value, ok := s.values[registry]
	return value, ok, nil
}
func (s *memoryCredentialStore) Set(registry string, credential credentials.Credential) error {
	if credential.Username == "" || credential.Password == "" {
		return errors.New("incomplete")
	}
	s.values[registry] = credential
	return nil
}
func (s *memoryCredentialStore) Delete(registry string) error { delete(s.values, registry); return nil }
func useMemoryCredentialStore(t *testing.T) *memoryCredentialStore {
	store := &memoryCredentialStore{values: map[string]credentials.Credential{}}
	original := credentialStore
	credentialStore = store
	t.Cleanup(func() { credentialStore = original })
	return store
}

func TestResolveRegistryCredentials(t *testing.T) {
	store := useMemoryCredentialStore(t)
	store.values[credentials.HarborRegistry] = credentials.Credential{Username: "robot", Password: "token"}
	flags := config.DockerFlags{EnsureDockerRegistry: true}
	if err := resolveRegistryCredentials(&flags); err != nil {
		t.Fatal(err)
	}
	if flags.DockerUsername != "robot" || flags.DockerPassword != "token" {
		t.Fatalf("flags=%#v", flags)
	}
}

func TestResolveRegistryCredentialsPreservesExplicitPair(t *testing.T) {
	store := useMemoryCredentialStore(t)
	store.values[credentials.HarborRegistry] = credentials.Credential{Username: "stored", Password: "stored-token"}
	flags := config.DockerFlags{EnsureDockerRegistry: true, DockerUsername: "explicit", DockerPassword: "explicit-token"}
	if err := resolveRegistryCredentials(&flags); err != nil {
		t.Fatal(err)
	}
	if flags.DockerUsername != "explicit" {
		t.Fatal("stored credentials replaced explicit pair")
	}
}

func TestResolveRegistryCredentialsPrefersEnvironmentPair(t *testing.T) {
	store := useMemoryCredentialStore(t)
	store.values[credentials.HarborRegistry] = credentials.Credential{Username: "stored", Password: "stored-token"}
	t.Setenv("HARBOR_USERNAME", "environment")
	t.Setenv("HARBOR_PASSWORD", "environment-token")
	flags := config.DockerFlags{EnsureDockerRegistry: true}
	if err := resolveRegistryCredentials(&flags); err != nil {
		t.Fatal(err)
	}
	if flags.DockerUsername != "environment" || flags.DockerPassword != "environment-token" {
		t.Fatalf("flags=%#v", flags)
	}
}

func TestCredentialsStatusDoesNotPrintSecrets(t *testing.T) {
	store := useMemoryCredentialStore(t)
	store.values[credentials.HarborRegistry] = credentials.Credential{Username: "robot", Password: "secret-token"}
	cmd := newCredentialsStatusCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(out.Bytes(), []byte("secret-token")) {
		t.Fatal("status exposed secret")
	}
}

func TestCredentialsStatusReportsUnavailableKeyring(t *testing.T) {
	original := credentialStore
	credentialStore = &memoryCredentialStore{err: &credentials.UnavailableError{Err: errors.New("no session bus")}}
	t.Cleanup(func() { credentialStore = original })
	cmd := newCredentialsStatusCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected explicit status backend error")
	}
}

func TestResolveRegistryCredentialsFromVersionEnvFile(t *testing.T) {
	useMemoryCredentialStore(t)
	path := t.TempDir() + "/8.10.env"
	if err := os.WriteFile(path, []byte("HARBOR_USERNAME=robot\nHARBOR_PASSWORD=token\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	flags := config.DockerFlags{EnsureDockerRegistry: true}
	if err := resolveRegistryCredentialsFromEnvFiles(&flags, []matrix.Entry{{Version: "8.10"}}, map[string]string{"8.10": path}, ""); err != nil {
		t.Fatal(err)
	}
	if flags.DockerUsername != "robot" || flags.DockerPassword != "token" {
		t.Fatalf("flags=%#v", flags)
	}
}

func TestResolveRegistryCredentialsFromEnvFilesPreservesExplicitPair(t *testing.T) {
	useMemoryCredentialStore(t)
	path := t.TempDir() + "/8.10.env"
	if err := os.WriteFile(path, []byte("HARBOR_USERNAME=file\nHARBOR_PASSWORD=file-token\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	flags := config.DockerFlags{EnsureDockerRegistry: true, DockerUsername: "explicit", DockerPassword: "explicit-token"}
	if err := resolveRegistryCredentialsFromEnvFiles(&flags, []matrix.Entry{{Version: "8.10"}}, map[string]string{"8.10": path}, ""); err != nil {
		t.Fatal(err)
	}
	if flags.DockerUsername != "explicit" || flags.DockerPassword != "explicit-token" {
		t.Fatalf("flags=%#v", flags)
	}
}

func TestResolveRegistryCredentialsFromEnvFilesPrefersEnvironmentPair(t *testing.T) {
	useMemoryCredentialStore(t)
	path := t.TempDir() + "/8.10.env"
	if err := os.WriteFile(path, []byte("HARBOR_USERNAME=file\nHARBOR_PASSWORD=file-token\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HARBOR_USERNAME", "environment")
	t.Setenv("HARBOR_PASSWORD", "environment-token")
	flags := config.DockerFlags{EnsureDockerRegistry: true}
	if err := resolveRegistryCredentialsFromEnvFiles(&flags, []matrix.Entry{{Version: "8.10"}}, map[string]string{"8.10": path}, ""); err != nil {
		t.Fatal(err)
	}
	if flags.DockerUsername != "environment" || flags.DockerPassword != "environment-token" {
		t.Fatalf("flags=%#v", flags)
	}
}

func TestCredentialsConfigureStoresPairWithoutPrintingSecret(t *testing.T) {
	store := useMemoryCredentialStore(t)
	cmd := newCredentialsConfigureCommand()
	cmd.SetArgs([]string{"--registry", "harbor"})
	cmd.SetIn(bytes.NewBufferString("robot\nsecret-token\n"))
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if store.values[credentials.HarborRegistry].Password != "secret-token" {
		t.Fatal("token not stored")
	}
	if bytes.Contains(out.Bytes(), []byte("secret-token")) {
		t.Fatal("configure output exposed secret")
	}
}
