// secretstore_config_test.go
// Copyright 2022 Camunda Services GmbH
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
	"path/filepath"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
)

type secretStoreConfigTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
	templates []string
}

func TestSecretStoreConfigTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &secretStoreConfigTest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
		templates: []string{"templates/orchestration/configmap.yaml"},
	})
}

// mergeValues returns a new map combining base with overrides (overrides win).
func mergeValues(base, overrides map[string]string) map[string]string {
	merged := make(map[string]string, len(base)+len(overrides))
	for k, v := range base {
		merged[k] = v
	}
	for k, v := range overrides {
		merged[k] = v
	}
	return merged
}

// applicationConfig extracts the rendered application.yaml from the orchestration
// configuration ConfigMap.
func (s *secretStoreConfigTest) applicationConfig(output string) string {
	var configmap corev1.ConfigMap
	helm.UnmarshalK8SYaml(s.T(), output, &configmap)
	return configmap.Data["application.yaml"]
}

func (s *secretStoreConfigTest) TestDifferentValuesInputs() {
	testCases := []testhelpers.TestCase{
		{
			Name:     "File secret store renders camunda.secrets.stores.file",
			Template: "templates/orchestration/configmap.yaml",
			Values: mergeValues(baseValues(), map[string]string{
				"orchestration.secretStore.file.primary.path":           "/etc/camunda/secrets",
				"orchestration.secretStore.file.primary.existingSecret": "my-secrets",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				config := s.applicationConfig(output)
				s.Require().Contains(config, "secrets:")
				s.Require().Contains(config, "file:")
				s.Require().Contains(config, "path: /etc/camunda/secrets")
				// existingSecret is chart-only plumbing and must not leak into app config.
				s.Require().NotContains(config, "existingSecret")
			},
		},
		{
			Name:     "AWS secret store renders kebab-case keys and strips roleArn",
			Template: "templates/orchestration/configmap.yaml",
			Values: mergeValues(baseValues(), map[string]string{
				"orchestration.secretStore.aws.primary.region":       "us-east-1",
				"orchestration.secretStore.aws.primary.pathPrefix":   "camunda/",
				"orchestration.secretStore.aws.primary.batchEnabled": "true",
				"orchestration.secretStore.aws.primary.batchSize":    "10",
				"orchestration.secretStore.aws.primary.roleArn":      "arn:aws:iam::123456789012:role/camunda-secrets",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				config := s.applicationConfig(output)
				s.Require().Contains(config, "aws:")
				s.Require().Contains(config, "region: us-east-1")
				s.Require().Contains(config, "path-prefix: camunda/")
				s.Require().Contains(config, "batch-enabled: true")
				s.Require().Contains(config, "batch-size: 10")
				// roleArn is chart-only plumbing and must not leak into app config.
				s.Require().NotContains(config, "roleArn")
				s.Require().NotContains(config, "role-arn")
			},
		},
		{
			Name:     "No secret store renders no camunda.secrets block",
			Template: "templates/orchestration/configmap.yaml",
			Values:   baseValues(),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				config := s.applicationConfig(output)
				s.Require().NotContains(config, "secrets:")
			},
		},
		{
			Name:     "Physical tenant override renders camunda.physical-tenants.<id>.secrets",
			Template: "templates/orchestration/configmap.yaml",
			Values: mergeValues(baseValues(), map[string]string{
				"orchestration.secretStore.aws.primary.region":                         "us-east-1",
				"orchestration.secretStore.physicalTenants.tenanta.aws.primary.region": "us-west-2",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				config := s.applicationConfig(output)
				s.Require().Contains(config, "physical-tenants:")
				s.Require().Contains(config, "tenanta:")
				s.Require().Contains(config, "region: us-west-2")
				// The default-tenant store still renders.
				s.Require().Contains(config, "region: us-east-1")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}
