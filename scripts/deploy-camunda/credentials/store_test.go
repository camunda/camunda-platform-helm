package credentials

import (
	"errors"
	"testing"
)

func TestCredentialRequiresCompletePair(t *testing.T) {
	for _, credential := range []Credential{{}, {Username: "user"}, {Password: "token"}} {
		if err := (KeyringStore{}).Set("", credential); err == nil {
			t.Fatal("expected incomplete credential error")
		}
	}
}

type unavailableStore struct{}

func (unavailableStore) Get(string) (Credential, bool, error) {
	return Credential{}, false, &UnavailableError{Err: errors.New("no session bus")}
}
func (unavailableStore) Set(string, Credential) error { return nil }
func (unavailableStore) Delete(string) error          { return nil }

func TestGetOptionalIgnoresUnavailableBackend(t *testing.T) {
	_, found, err := GetOptional(unavailableStore{}, HarborRegistry)
	if err != nil || found {
		t.Fatalf("found=%v err=%v", found, err)
	}
}

type corruptStore struct{}

func (corruptStore) Get(string) (Credential, bool, error) {
	return Credential{}, false, &CorruptError{Err: errors.New("bad entry")}
}
func (corruptStore) Set(string, Credential) error { return nil }
func (corruptStore) Delete(string) error          { return nil }

func TestGetOptionalIgnoresCorruptEntry(t *testing.T) {
	_, found, err := GetOptional(corruptStore{}, HarborRegistry)
	if err != nil || found {
		t.Fatalf("found=%v err=%v", found, err)
	}
}
