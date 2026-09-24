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
	"reflect"
	"testing"
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
