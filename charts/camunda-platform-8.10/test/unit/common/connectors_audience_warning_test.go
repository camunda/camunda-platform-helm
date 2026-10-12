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
	"io"
	"maps"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"
)

const connectorsAudienceKey = "connectors.security.authentication.oidc.audience"

type ConnectorsAudienceWarningTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
}

func TestConnectorsAudienceWarningTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &ConnectorsAudienceWarningTest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
	})
}

func connectorsAudienceValues(extra map[string]string) map[string]string {
	values := map[string]string{
		"identity.enabled":                      "true",
		"connectors.enabled":                    "true",
		"global.identity.auth.enabled":          "true",
		"global.identity.auth.type":             "KEYCLOAK",
		"global.security.authentication.method": "oidc",
		connectorsAudienceKey:                   "custom-audience",
	}
	maps.Copy(values, extra)
	return values
}

func (s *ConnectorsAudienceWarningTest) renderedWarnings(output string) string {
	decoder := k8syaml.NewYAMLOrJSONDecoder(strings.NewReader(output), 4096)
	for {
		var resource corev1.ConfigMap
		err := decoder.Decode(&resource)
		if err == io.EOF {
			return ""
		}
		s.Require().NoError(err)
		if resource.Kind == "ConfigMap" && resource.Name == s.release+"-warnings" {
			return resource.Data["warnings"]
		}
	}
}

func (s *ConnectorsAudienceWarningTest) TestWarnsOnlyWhenIdentityProvisionsAnotherAudience() {
	var testCases []testhelpers.TestCase
	for _, scenario := range []struct {
		name   string
		values map[string]string
		warn   bool
	}{
		{name: "TestWarnsWhenIdentityProvisionsTheKeycloakClient", warn: true},
		{
			name: "TestWarnsWhenCentralIdentityAlwaysRegistersTheClient",
			values: map[string]string{
				"connectors.enabled":                             "false",
				"global.identity.auth.connectors.alwaysRegister": "true",
			},
			warn: true,
		},
		{name: "TestSilentWhenAudienceIsUnset", values: map[string]string{connectorsAudienceKey: ""}},
		{name: "TestSilentWhenAudienceMatchesTheOrchestrationDefault", values: map[string]string{connectorsAudienceKey: "orchestration-api"}},
		{
			name:   "TestSilentWhenAudienceMatchesTheOverriddenOrchestrationAudience",
			values: map[string]string{"orchestration.security.authentication.oidc.audience": "custom-audience"},
		},
		{name: "TestSilentForAnExternalOidcProvider", values: map[string]string{"global.identity.auth.type": "GENERIC"}},
		{name: "TestSilentWhenIdentityIsNotDeployed", values: map[string]string{"identity.enabled": "false"}},
		{name: "TestSilentWhenConnectorsUseBasicAuth", values: map[string]string{"connectors.security.authentication.method": "basic"}},
	} {
		testCases = append(testCases, testhelpers.TestCase{
			Name:   scenario.name,
			Values: connectorsAudienceValues(scenario.values),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				warnings := s.renderedWarnings(output)
				if !scenario.warn {
					s.Require().NotContains(warnings, connectorsAudienceKey)
					return
				}
				s.Require().Contains(warnings, connectorsAudienceKey)
				s.Require().Contains(warnings, `"custom-audience"`)
				s.Require().Contains(warnings, `"orchestration-api"`)
			},
		})
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, nil, testCases)
}
