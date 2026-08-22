// Copyright 2026 Camunda Services GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

// CorruptError marks a keyring entry that exists but cannot be used (unparseable
// or missing a username/password). Like UnavailableError, implicit deploy lookup
// treats it as "not configured" so a single bad entry never blocks every command,
// while explicit credentials status/configure/delete still surface it.
type CorruptError struct{ Err error }

func (e *CorruptError) Error() string { return e.Err.Error() }
func (e *CorruptError) Unwrap() error { return e.Err }

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
		return Credential{}, false, &CorruptError{Err: fmt.Errorf("decode %s credential from OS keyring: %w; run 'deploy-camunda credentials delete --registry %s' to reset", registry, err, registry)}
	}
	if credential.Username == "" || credential.Password == "" {
		return Credential{}, false, &CorruptError{Err: fmt.Errorf("OS keyring entry for %s is incomplete; run 'deploy-camunda credentials delete --registry %s' to reset", registry, registry)}
	}
	return credential, true, nil
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

func GetOptional(store Store, registry string) (Credential, bool, error) {
	credential, found, err := store.Get(registry)
	var unavailable *UnavailableError
	var corrupt *CorruptError
	if errors.As(err, &unavailable) || errors.As(err, &corrupt) {
		return Credential{}, false, nil
	}
	return credential, found, err
}
