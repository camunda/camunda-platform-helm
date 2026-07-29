package matrix

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
)

func TestImportPlaintextDockerAuth(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	auth := base64.StdEncoding.EncodeToString([]byte("user:password"))
	content := `{"auths":{"https://index.docker.io/v1/":{"auth":"` + auth + `"}}}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := ImportPlaintextDockerAuth(path, "docker.io")
	if err != nil {
		t.Fatal(err)
	}
	if got := result["docker.io"]; got.Username != "user" || got.Password != "password" {
		t.Fatalf("auth = %#v", got)
	}
}

func TestImportPlaintextDockerAuthRejectsHelpers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	content := `{"credsStore":"desktop","auths":{}}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := ImportPlaintextDockerAuth(path, "docker.io")
	if err == nil {
		t.Fatal("expected credential helper rejection")
	}
}

func TestImportPlaintextDockerAuthRejectsHelperDespitePlaintextEntry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	auth := base64.StdEncoding.EncodeToString([]byte("stale:credential"))
	content := `{"credHelpers":{"https://index.docker.io/v1/":"desktop"},"auths":{"https://index.docker.io/v1/":{"auth":"` + auth + `"}}}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := ImportPlaintextDockerAuth(path, "docker.io")
	if err == nil {
		t.Fatal("expected matching credential helper rejection")
	}
}
