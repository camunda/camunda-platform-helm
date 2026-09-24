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

package deployer

import (
	"errors"
	"reflect"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestLifecycleAnnotations(t *testing.T) {
	cases := []struct {
		name     string
		existing map[string]string
		ttl      string
		want     map[string]string
	}{
		{
			name:     "new namespace gets the deploy TTL",
			existing: nil,
			ttl:      "2h",
			want:     map[string]string{"cleaner/ttl": "2h", "janitor/ttl": "2h", "camunda.cloud/ephemeral": "true"},
		},
		{
			name:     "empty TTL defaults to 1h",
			existing: map[string]string{},
			ttl:      " ",
			want:     map[string]string{"cleaner/ttl": "1h", "janitor/ttl": "1h", "camunda.cloud/ephemeral": "true"},
		},
		{
			name:     "ephemeral namespace is re-stamped",
			existing: map[string]string{"cleaner/ttl": "1h", "janitor/ttl": "1h", "camunda.cloud/ephemeral": "true"},
			ttl:      "2h",
			want:     map[string]string{"cleaner/ttl": "2h", "janitor/ttl": "2h", "camunda.cloud/ephemeral": "true"},
		},
		{
			name:     "persisted namespace keeps its long TTL",
			existing: map[string]string{"cleaner/ttl": "8760h", "janitor/ttl": "8760h", "camunda.cloud/ephemeral": "false", "other": "x"},
			ttl:      "2h",
			want:     map[string]string{"cleaner/ttl": "8760h", "janitor/ttl": "8760h", "camunda.cloud/ephemeral": "false"},
		},
		{
			name:     "persisted namespace without TTLs stays without them",
			existing: map[string]string{"camunda.cloud/ephemeral": "false"},
			ttl:      "2h",
			want:     map[string]string{"camunda.cloud/ephemeral": "false"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := lifecycleAnnotations(tc.existing, tc.ttl); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestDecideLifecycle(t *testing.T) {
	persisted := map[string]string{"cleaner/ttl": "8760h", "camunda.cloud/ephemeral": "false"}
	ephemeral := map[string]string{"cleaner/ttl": "1h", "camunda.cloud/ephemeral": "true"}
	forbidden := apierrors.NewForbidden(schema.GroupResource{Resource: "namespaces"}, "ns", errors.New("no get"))
	boom := errors.New("connection reset")
	read := func(a map[string]string, err error) func() (map[string]string, error) {
		return func() (map[string]string, error) { return a, err }
	}
	cases := []struct {
		name       string
		created    bool
		initial    map[string]string
		initialErr error
		reread     func() (map[string]string, error)
		want       map[string]string
		wantStamp  bool
		wantErr    bool
	}{
		{name: "created by this deploy is new, even if unreadable", created: true, reread: read(nil, forbidden), wantStamp: true},
		{name: "persisted since the deploy started is seen as persisted", initial: ephemeral, reread: read(persisted, nil), want: persisted, wantStamp: true},
		{name: "absent at first, created and persisted meanwhile", initial: nil, reread: read(persisted, nil), want: persisted, wantStamp: true},
		{name: "recovers from an initial transient failure", initialErr: boom, reread: read(persisted, nil), want: persisted, wantStamp: true},
		{name: "falls back to the initial read when the reread fails", initial: persisted, reread: read(nil, boom), want: persisted, wantStamp: true},
		{name: "unreadable pre-existing namespace is left alone", initialErr: forbidden, reread: read(nil, forbidden), wantStamp: false},
		{name: "any other persistent failure aborts the deploy", initialErr: boom, reread: read(nil, boom), wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, stamp, err := decideLifecycle(tc.created, tc.initial, tc.initialErr, tc.reread)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if stamp != tc.wantStamp || !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got (%v, stamp=%v), want (%v, stamp=%v)", got, stamp, tc.want, tc.wantStamp)
			}
			if stamp && tc.want != nil && lifecycleAnnotations(got, "2h")["cleaner/ttl"] != "8760h" {
				t.Error("a persisted namespace would be re-stamped with the deploy TTL")
			}
		})
	}
}
