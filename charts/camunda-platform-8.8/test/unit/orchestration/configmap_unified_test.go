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

package orchestration

import (
	"camunda-platform/test/unit/testhelpers"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type ConfigmapTemplateTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
	templates []string
}

func TestConfigmapUnifiedTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &ConfigmapTemplateTest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
		templates: []string{"templates/orchestration/configmap-unified.yaml"},
	})
}

func (s *ConfigmapTemplateTest) TestDifferentValuesInputsUnified() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestApplicationYamlShouldContainEnabledProfilesBroker",
			Values: map[string]string{
				"orchestration.profiles.broker": "false",
			},
			Expected: map[string]string{
				"configmapApplication.spring.profiles.active": "identity,operate,tasklist,consolidated-auth",
			},
		},
		{
			Name: "TestApplicationYamlShouldContainEnabledProfilesOperate",
			Values: map[string]string{
				"orchestration.profiles.operate": "false",
			},
			Expected: map[string]string{
				"configmapApplication.spring.profiles.active": "broker,identity,tasklist,consolidated-auth",
			},
		},
		{
			Name: "TestApplicationYamlShouldContainEnabledProfilesTasklist",
			Values: map[string]string{
				"orchestration.profiles.tasklist": "false",
			},
			Expected: map[string]string{
				"configmapApplication.spring.profiles.active": "broker,identity,operate,consolidated-auth",
			},
		},
		{
			Name: "TestApplicationYamlShouldContainContextPath",
			Values: map[string]string{
				"orchestration.contextPath": "/custom",
			},
			Expected: map[string]string{
				"configmapApplication.server.servlet.context-path": "/custom",
				"configmapApplication.management.server.base-path": "/custom",
			},
		},
		{
			Name: "TestApplicationYamlShouldContainSecondaryStorageOpenSearchEnabled",
			Values: map[string]string{
				"global.opensearch.enabled":  "true",
				"global.opensearch.url.host": "opensearch.example.com",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.secondary-storage.opensearch.url": "https://opensearch.example.com:443",
			},
		},
		{
			Name: "TestApplicationYamlShouldContainAuthOIDCClientId",
			Values: map[string]string{
				"orchestration.security.authentication.method": "oidc",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.security.authentication.oidc.client-id": "orchestration",
			},
		},
	}

	testhelpers.RunTestCases(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *ConfigmapTemplateTest) TestDifferentValuesInputsUnifiedCompatibility() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestApplicationYamlShouldContainEnabledProfilesBroker",
			Values: map[string]string{
				"global.compatibility.orchestration.enabled": "true",
				"orchestration.profiles.broker":              "false",
				"zeebe.enabled":                              "false",
			},
			Expected: map[string]string{
				"configmapApplication.spring.profiles.active": "identity,operate,tasklist,consolidated-auth",
			},
		},
		{
			Name: "TestApplicationYamlShouldContainEnabledProfilesOperate",
			Values: map[string]string{
				"global.compatibility.orchestration.enabled": "true",
				"orchestration.profiles.operate":             "false",
				"operate.enabled":                            "true",
			},
			Expected: map[string]string{
				"configmapApplication.spring.profiles.active": "broker,identity,operate,tasklist,consolidated-auth",
			},
		},
		{
			Name: "TestApplicationYamlShouldContainEnabledProfilesTasklist",
			Values: map[string]string{
				"global.compatibility.orchestration.enabled": "true",
				"orchestration.profiles.tasklist":            "false",
				"tasklist.enabled":                           "true",
			},
			Expected: map[string]string{
				"configmapApplication.spring.profiles.active": "broker,identity,operate,tasklist,consolidated-auth",
			},
		},
		{
			Name: "TestApplicationYamlShouldContainContextPath",
			Values: map[string]string{
				"global.compatibility.orchestration.enabled": "true",
				"zeebeGateway.enabled":                       "true",
				"zeebeGateway.contextPath":                   "/custom",
			},
			Expected: map[string]string{
				"configmapApplication.server.servlet.context-path": "/custom",
				"configmapApplication.management.server.base-path": "/custom",
			},
		},
		{
			Name: "TestApplicationYamlShouldContainPortServer",
			Values: map[string]string{
				"global.compatibility.orchestration.enabled": "true",
				"zeebeGateway.enabled":                       "true",
				"zeebeGateway.service.restPort":              "1111",
			},
			Expected: map[string]string{
				"configmapApplication.server.port": "1111",
			},
		},
		{
			Name: "TestApplicationYamlShouldContainPortGRPC",
			Values: map[string]string{
				"global.compatibility.orchestration.enabled": "true",
				"zeebeGateway.enabled":                       "true",
				"zeebeGateway.service.grpcPort":              "1111",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.api.grpc.port": "1111",
			},
		},
	}

	testhelpers.RunTestCases(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}
