package credentials

import (
	"encoding/json"
	"errors"
	"fmt"

	keyring "github.com/zalando/go-keyring"
)

const Service = "camunda-deploy-camunda"

const (
	HarborRegistry    = "registry.camunda.cloud"
	DockerHubRegistry = "docker.io"
)

type Credential struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type Store interface {
	Get(registry string) (Credential, bool, error)
	Set(registry string, credential Credential) error
	Delete(registry string) error
}

type KeyringStore struct{}

type UnavailableError struct{ Err error }

func (e *UnavailableError) Error() string { return e.Err.Error() }
func (e *UnavailableError) Unwrap() error { return e.Err }

func (KeyringStore) Get(registry string) (Credential, bool, error) {
	value, err := keyring.Get(Service, registry)
	if errors.Is(err, keyring.ErrNotFound) {
		return Credential{}, false, nil
	}
	if err != nil {
		return Credential{}, false, &UnavailableError{Err: fmt.Errorf("read %s credential from OS keyring: %w", registry, err)}
	}
	var credential Credential
	if err := json.Unmarshal([]byte(value), &credential); err != nil {
		return Credential{}, false, fmt.Errorf("decode %s credential from OS keyring: %w", registry, err)
	}
	if credential.Username == "" || credential.Password == "" {
		return Credential{}, false, fmt.Errorf("OS keyring entry for %s is incomplete", registry)
	}
	return credential, true, nil
}

func GetOptional(store Store, registry string) (Credential, bool, error) {
	credential, found, err := store.Get(registry)
	var unavailable *UnavailableError
	if errors.As(err, &unavailable) {
		return Credential{}, false, nil
	}
	return credential, found, err
}

func (KeyringStore) Set(registry string, credential Credential) error {
	if credential.Username == "" || credential.Password == "" {
		return errors.New("username and password/token are required")
	}
	value, err := json.Marshal(credential)
	if err != nil {
		return err
	}
	if err := keyring.Set(Service, registry, string(value)); err != nil {
		return fmt.Errorf("store %s credential in OS keyring: %w", registry, err)
	}
	return nil
}

func (KeyringStore) Delete(registry string) error {
	err := keyring.Delete(Service, registry)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("delete %s credential from OS keyring: %w", registry, err)
	}
	return nil
}
