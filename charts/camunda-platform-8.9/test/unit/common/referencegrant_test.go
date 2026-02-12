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

	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type ReferenceGrantTemplateTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
	templates []string
}

func TestReferenceGrantTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &ReferenceGrantTemplateTest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
		templates: []string{"templates/common/referencegrant.yaml"},
	})
}

func (s *ReferenceGrantTemplateTest) TestDifferentValuesInputs() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestReferenceGrantDisabledWhenGatewayDisabled",
			Values: map[string]string{
				"global.gateway.enabled": "false",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NotContains(t, output, "kind: ReferenceGrant")
			},
		},
		{
			Name: "TestReferenceGrantNotRenderedWhenExternal",
			Values: map[string]string{
				"global.gateway.enabled":  "true",
				"global.gateway.external": "true",
				"global.gateway.hostname": "camunda.example.com",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NotContains(t, output, "kind: ReferenceGrant")
			},
		},
		{
			Name: "TestReferenceGrantRendered",
			Values: map[string]string{
				"global.gateway.enabled":  "true",
				"global.gateway.hostname": "camunda.example.com",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "kind: ReferenceGrant")
				require.Contains(t, output, "apiVersion: gateway.networking.k8s.io/v1beta1")
				require.Contains(t, output, "name: camunda-platform-test")
			},
		},
		{
			Name: "TestReferenceGrantFromHTTPRoute",
			Values: map[string]string{
				"global.gateway.enabled":  "true",
				"global.gateway.hostname": "camunda.example.com",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "group: gateway.networking.k8s.io")
				require.Contains(t, output, "kind: HTTPRoute")
			},
		},
		{
			Name: "TestReferenceGrantToService",
			Values: map[string]string{
				"global.gateway.enabled":  "true",
				"global.gateway.hostname": "camunda.example.com",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "kind: Service")
			},
		},
		{
			Name: "TestReferenceGrantUsesControllerNamespace",
			Values: map[string]string{
				"global.gateway.enabled":             "true",
				"global.gateway.hostname":            "camunda.example.com",
				"global.gateway.controllerNamespace": "gateway-system",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "namespace: gateway-system")
			},
		},
		{
			Name: "TestReferenceGrantDefaultsControllerNamespaceToReleaseNamespace",
			Values: map[string]string{
				"global.gateway.enabled":  "true",
				"global.gateway.hostname": "camunda.example.com",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				// The from namespace should default to .Release.Namespace
				require.Contains(t, output, "kind: ReferenceGrant")
				// Namespace in the from spec should be the release namespace
				require.Contains(t, output, "namespace:")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}
