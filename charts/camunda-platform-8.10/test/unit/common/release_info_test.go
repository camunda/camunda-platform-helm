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

// ReleaseInfoTest verifies the release-info ConfigMap rendered by
// camundaPlatform.releaseInfo.
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

// TestKeycloakComponentEntry verifies the Keycloak entry is emitted only when a
// Keycloak URL is resolvable. After the Bitnami subchart removal, identity is
// always enabled independently of a bundled Keycloak, so a non-Keycloak (external
// OIDC) deployment must not emit a Keycloak component with an empty url.
func (s *ReleaseInfoTest) TestKeycloakComponentEntry() {
	testCases := []testhelpers.TestCase{
		{
			Name: "KeycloakEntryPresentWhenKeycloakUrlSet",
			Values: map[string]string{
				"identity.enabled":                      "true",
				"global.identity.keycloak.url.protocol": "https",
				"global.identity.keycloak.url.host":     "keycloak.example.com",
				"global.identity.keycloak.url.port":     "443",
			},
			Template: "templates/common/configmap-release.yaml",
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "id: keycloak",
					"Keycloak component entry should be present when a Keycloak URL is configured")
			},
		},
		{
			Name: "KeycloakEntryAbsentForExternalOidc",
			Values: map[string]string{
				"identity.enabled":          "true",
				"global.identity.auth.type": "MICROSOFT",
			},
			Template: "templates/common/configmap-release.yaml",
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.NotContains(t, output, "id: keycloak",
					"Keycloak component entry should be absent when no Keycloak URL is configured (external OIDC)")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, nil, testCases)
}
