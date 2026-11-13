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

package camunda

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUpgradeSchemaDeclarations(t *testing.T) {
	t.Parallel()
	source, err := os.ReadFile("../../../values.schema.json")
	require.NoError(t, err)
	inputs := []struct{ path, kind string }{
		{"identityKeycloak.postgresql.enabled", "boolean"},
	}
	for _, image := range []string{
		"identityKeycloak.image", "identityKeycloak.keycloakConfigCli.image",
		"identityKeycloak.postgresql.image", "identityKeycloak.postgresql.metrics.image", "identityKeycloak.postgresql.volumePermissions.image",
		"identityPostgresql.image", "identityPostgresql.metrics.image", "identityPostgresql.volumePermissions.image",
		"webModelerPostgresql.image", "webModelerPostgresql.metrics.image", "webModelerPostgresql.volumePermissions.image",
		"elasticsearch.image", "elasticsearch.metrics.image", "elasticsearch.volumePermissions.image", "elasticsearch.sysctlImage", "elasticsearch.copyTlsCerts.image",
	} {
		inputs = append(inputs, struct{ path, kind string }{image + ".registry", "string"}, struct{ path, kind string }{image + ".pullSecrets", "array"})
	}
	for _, input := range inputs {
		t.Run(input.path, func(t *testing.T) {
			node := source
			for _, part := range strings.Split(input.path, ".") {
				var object struct {
					Properties map[string]json.RawMessage `json:"properties"`
				}
				require.NoError(t, json.Unmarshal(node, &object))
				require.Contains(t, object.Properties, part)
				node = object.Properties[part]
			}
			var declaration struct {
				Type string `json:"type"`
			}
			require.NoError(t, json.Unmarshal(node, &declaration))
			require.Equal(t, input.kind, declaration.Type)
		})
	}
}
