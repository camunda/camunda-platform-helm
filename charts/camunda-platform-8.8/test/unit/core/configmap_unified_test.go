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

package core

import (
	"camunda-platform/test/unit/testhelpers"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func TestConfigmapUnifiedTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &ConfigmapTemplateTest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
		templates: []string{"templates/core/configmap-unified.yaml"},
	})
}

func (s *ConfigmapTemplateTest) TestDifferentValuesInputsUnified() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestApplicationYamlShouldContainMinimumAge",
			Values: map[string]string{
				"core.history.retention.enabled":    "true",
				"core.history.retention.minimumAge": "7d",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.data.retention.minimum-age": "7d",
			},
		},
		{
			Name: "TestApplicationYamlShouldContainEnabledProfilesBroker",
			Values: map[string]string{
				"core.profiles.broker":              "false",
			},
			Expected: map[string]string{
				"configmapApplication.spring.profiles.active": "identity,operate,tasklist",
			},
		},
		{
			Name: "TestApplicationYamlShouldContainEnabledProfilesOperate",
			Values: map[string]string{
				"core.profiles.operate":             "false",
			},
			Expected: map[string]string{
				"configmapApplication.spring.profiles.active": "broker,identity,tasklist",
			},
		},
		{
			Name: "TestApplicationYamlShouldContainEnabledProfilesTasklist",
			Values: map[string]string{
				"core.profiles.tasklist":            "false",
			},
			Expected: map[string]string{
				"configmapApplication.spring.profiles.active": "broker,identity,operate",
			},
		},
		{
			Name: "TestApplicationYamlShouldContainContextPath",
			Values: map[string]string{
				"core.contextPath": "/custom",
			},
			Expected: map[string]string{
				"configmapApplication.management.endpoint": "/custom/actuator",
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
				"global.security.authentication.method": "oidc",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.security.authentication.oidc.client-id": "core",
			},
		},
	}

	testhelpers.RunTestCases(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *ConfigmapTemplateTest) TestDifferentValuesInputsUnifiedCompatibility() {
	testCases := []testhelpers.TestCase{
		// TODO: Update the retention values after review the correct path with the dev team.
		// {
		// 	Name: "TestApplicationYamlShouldContainMinimumAge",
		// 	Values: map[string]string{
		// 		"global.compatibility.core.enabled": "true",
		// 		"zeebe.enabled":                     "true",
		// 		"zeebe.retention.enabled":           "true",
		// 		"zeebe.retention.minimumAge":        "7d",
		// 	},
		// 	Expected: map[string]string{
		// 		"configmapApplication.camunda.data.retention.minimum-age": "7d",
		// 	},
		// },
		{
			Name: "TestApplicationYamlShouldContainEnabledProfilesBroker",
			Values: map[string]string{
				"global.compatibility.core.enabled": "true",
				"core.profiles.broker":              "null",
				"zeebe.enabled":                     "false",
			},
			Expected: map[string]string{
				"configmapApplication.spring.profiles.active": "identity,operate,tasklist",
			},
		},
		{
			Name: "TestApplicationYamlShouldContainEnabledProfilesOperate",
			Values: map[string]string{
				"global.compatibility.core.enabled": "true",
				"core.profiles.operate":             "null",
				"operate.enabled":                   "true",
			},
			Expected: map[string]string{
				"configmapApplication.spring.profiles.active": "broker,identity,operate,tasklist",
			},
		},
		{
			Name: "TestApplicationYamlShouldContainEnabledProfilesTasklist",
			Values: map[string]string{
				"global.compatibility.core.enabled": "true",
				"core.profiles.tasklist":            "null",
				"tasklist.enabled":                  "true",
			},
			Expected: map[string]string{
				"configmapApplication.spring.profiles.active": "broker,identity,operate,tasklist",
			},
		},
		{
			Name: "TestApplicationYamlShouldContainContextPath",
			Values: map[string]string{
				"global.compatibility.core.enabled": "true",
				"zeebeGateway.enabled":              "true",
				"zeebeGateway.contextPath":          "/custom",
			},
			Expected: map[string]string{
				"configmapApplication.management.endpoint": "/custom/actuator",
			},
		},
		{
			Name: "TestApplicationYamlShouldContainPortServer",
			Values: map[string]string{
				"global.compatibility.core.enabled": "true",
				"zeebeGateway.enabled":              "true",
				"zeebeGateway.service.restPort":     "1111",
			},
			Expected: map[string]string{
				"configmapApplication.server.port": "1111",
			},
		},
		{
			Name: "TestApplicationYamlShouldContainPortGRPC",
			Values: map[string]string{
				"global.compatibility.core.enabled": "true",
				"zeebeGateway.enabled":              "true",
				"zeebeGateway.service.grpcPort":     "1111",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.api.grpc.port": "1111",
			},
		},
		{
			Name: "TestApplicationYamlShouldContainPortCommandAPI",
			Values: map[string]string{
				"global.compatibility.core.enabled": "true",
				"zeebeGateway.enabled":              "true",
				"zeebeGateway.service.commandPort":  "1111",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.cluster.network.command-api.port": "1111",
			},
		},
		{
			Name: "TestApplicationYamlShouldContainPortInternalAPI",
			Values: map[string]string{
				"global.compatibility.core.enabled": "true",
				"zeebeGateway.enabled":              "true",
				"zeebeGateway.service.internalPort": "1111",
			},
			Expected: map[string]string{
				"configmapApplication.camunda.cluster.network.internal-api.port": "1111",
			},
		},
	}

	testhelpers.RunTestCases(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}
