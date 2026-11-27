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
	"os"
	"path/filepath"
	"slices"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestKeysMatchCIExternalSecret(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", ".github", "config", "external-secret", "external-secret-integration-test-credentials.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Spec struct {
			Target struct {
				Name     string `yaml:"name"`
				Template struct {
					Data yaml.Node `yaml:"data"`
				} `yaml:"template"`
			} `yaml:"target"`
		} `yaml:"spec"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Spec.Target.Name != SecretName {
		t.Errorf("target.name = %q, want %q", doc.Spec.Target.Name, SecretName)
	}
	var keys []string
	for i := 0; i < len(doc.Spec.Target.Template.Data.Content); i += 2 {
		keys = append(keys, doc.Spec.Target.Template.Data.Content[i].Value)
	}
	if !slices.Equal(keys, Keys) {
		t.Errorf("Keys drifted from the CI ExternalSecret:\n got  %v\n want %v", Keys, keys)
	}
	for _, k := range RequiredKeys {
		if !slices.Contains(Keys, k) {
			t.Errorf("required key %q is not in the schema", k)
		}
	}
}

func TestMissingKeys(t *testing.T) {
	got := MissingKeys(map[string]string{"a": "1", "b": ""}, []string{"a", "b", "c"})
	if !slices.Equal(got, []string{"b", "c"}) {
		t.Errorf("MissingKeys = %v, want [b c]", got)
	}
}
