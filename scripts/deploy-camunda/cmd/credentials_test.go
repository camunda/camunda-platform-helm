package cmd

import (
	"bytes"
	"errors"
	"testing"

	"scripts/deploy-camunda/credentials"
)

type memoryCredentialStore struct {
	values map[string]credentials.Credential
}

func (s *memoryCredentialStore) Get(registry string) (credentials.Credential, bool, error) {
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
	t.Helper()
	store := &memoryCredentialStore{values: map[string]credentials.Credential{}}
	original := credentialStore
	credentialStore = store
	t.Cleanup(func() { credentialStore = original })
	return store
}

func TestResolveKeyringCredentialPairs(t *testing.T) {
	store := useMemoryCredentialStore(t)
	store.values[credentials.HarborRegistry] = credentials.Credential{Username: "robot", Password: "token"}
	harborUser, harborPassword, hubUser, hubPassword := "", "", "", ""
	if err := resolveKeyringCredentialPairs(&harborUser, &harborPassword, &hubUser, &hubPassword, true, false); err != nil {
		t.Fatal(err)
	}
	if harborUser != "robot" || harborPassword != "token" {
		t.Fatalf("unexpected Harbor credentials %q/%q", harborUser, harborPassword)
	}
}

func TestResolveKeyringPreservesExplicitPair(t *testing.T) {
	store := useMemoryCredentialStore(t)
	store.values[credentials.HarborRegistry] = credentials.Credential{Username: "stored", Password: "stored-token"}
	harborUser, harborPassword, hubUser, hubPassword := "explicit", "explicit-token", "", ""
	if err := resolveKeyringCredentialPairs(&harborUser, &harborPassword, &hubUser, &hubPassword, true, false); err != nil {
		t.Fatal(err)
	}
	if harborUser != "explicit" || harborPassword != "explicit-token" {
		t.Fatal("keyring replaced explicit credentials")
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
	if !bytes.Contains(out.Bytes(), []byte("robot")) {
		t.Fatal("status omitted username")
	}
}

func TestCredentialsCommandRegistered(t *testing.T) {
	root := NewRootCommand()
	registerRootCommands(root)
	command, _, err := root.Find([]string{"credentials"})
	if err != nil || command.Name() != "credentials" {
		t.Fatalf("credentials command not registered: %v", err)
	}
	status, _, err := command.Find([]string{"status"})
	if err != nil || status.Name() != "status" {
		t.Fatalf("credentials status not registered: %v", err)
	}
}
