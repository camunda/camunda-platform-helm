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

type ReleaseInfoTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
}

func TestReleaseInfo(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &ReleaseInfoTest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
	})
}

func (s *ReleaseInfoTest) TestIngressProtocolOverridesExternalURLs() {
	testCases := []testhelpers.TestCase{
		{
			Name: "IngressTLSEnabledDefaultsToHTTPS",
			Values: map[string]string{
				"global.ingress.enabled":     "true",
				"global.ingress.tls.enabled": "true",
				"global.ingress.host":        "camunda.example.com",
				"identity.enabled":           "true",
				"identityKeycloak.enabled":   "true",
			},
			Template: "templates/common/configmap-release.yaml",
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "http: https://camunda.example.com")
			},
		},
		{
			Name: "ProtocolOverrideUsesHTTPSWhenIngressTLSIsDisabled",
			Values: map[string]string{
				"global.ingress.enabled":     "true",
				"global.ingress.tls.enabled": "false",
				"global.ingress.protocol":    "https",
				"global.ingress.host":        "camunda.example.com",
				"identity.enabled":           "true",
				"identityKeycloak.enabled":   "true",
			},
			Template: "templates/common/configmap-release.yaml",
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "http: https://camunda.example.com")
				require.Contains(t, output, "url: https://camunda.example.com/operate")
				require.Contains(t, output, "url: https://camunda.example.com/tasklist")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, nil, testCases)
}
