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
