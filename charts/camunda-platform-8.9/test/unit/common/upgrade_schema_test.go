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
	"camunda-platform/test/unit/testhelpers"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
)

func TestUpgradeSchemaDeclarations(t *testing.T) {
	t.Parallel()
	source, err := os.ReadFile("../../../values.schema.json")
	require.NoError(t, err)
	for _, input := range []struct{ path, kind string }{
		{"global.image.tag", "string"},
		{"orchestration.security.initialization.defaultRoles.admin.mappingRules", "array"},
		{"orchestration.security.initialization.defaultRoles.connectors.mappingRules", "array"},
	} {
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

func TestUpgradeRoleMappingRulesTemplate(t *testing.T) {
	t.Parallel()
	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)
	testCases := []testhelpers.TestCase{}
	for _, role := range []string{"admin", "connectors"} {
		testCases = append(testCases, testhelpers.TestCase{
			Name: role,
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type":                                        "elasticsearch",
				"orchestration.security.initialization.defaultRoles." + role + ".mappingRules[0]": "custom-rule",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var config corev1.ConfigMap
				helm.UnmarshalK8SYaml(t, output, &config)
				var application struct {
					Camunda struct {
						Security struct {
							Initialization struct {
								DefaultRoles map[string]struct {
									MappingRules []string `yaml:"mappingRules"`
								} `yaml:"default-roles"`
							} `yaml:"initialization"`
						} `yaml:"security"`
					} `yaml:"camunda"`
				}
				require.NoError(t, yaml.Unmarshal([]byte(config.Data["application.yaml"]), &application))
				require.Equal(t, []string{"custom-rule"}, application.Camunda.Security.Initialization.DefaultRoles[role].MappingRules)
			},
		})
	}
	testhelpers.RunTestCasesE(t, chartPath, "camunda-platform-test", "upgrade-schema", []string{"templates/orchestration/configmap.yaml"}, testCases)
}
