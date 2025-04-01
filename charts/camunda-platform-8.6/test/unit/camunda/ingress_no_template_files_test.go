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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

type IngressNoTemplateTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
	templates []string
	extraArgs []string
}

func TestIngressNoTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &IngressNoTemplateTest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
	})
}

func (s *IngressNoTemplateTest) TestDifferentValuesInputs() {
	testCases := []testhelpers.TestCase{{
		Name:                    "TestIngressEnabledAndKeycloakChartProxyForwardingEnabled",
		HelmOptionsExtraArgs:    map[string][]string{"install": {"--debug"}},
		RenderTemplateExtraArgs: []string{"--show-only", "charts/identityKeycloak/templates/statefulset.yaml"},
		Values: map[string]string{
			"global.ingress.tls.enabled": "true",
			"identity.contextPath":       "/identity",
			"identityKeycloak.enabled":   "true",
		},
		Verifier: func(t *testing.T, output string, err error) {
			var statefulSet appsv1.StatefulSet
			helm.UnmarshalK8SYaml(t, output, &statefulSet)

			// then
			env := statefulSet.Spec.Template.Spec.Containers[0].Env
			require.Contains(t, env,
				corev1.EnvVar{
					Name:  "KEYCLOAK_PROXY_ADDRESS_FORWARDING",
					Value: "true",
				})
		},
	}, {
		Name:                    "TestIngressEnabledWithKeycloakCustomContextPathWithTemplateArgs",
		HelmOptionsExtraArgs:    map[string][]string{"install": {"--debug"}},
		RenderTemplateExtraArgs: []string{"--show-only", "charts/identityKeycloak/templates/statefulset.yaml"},
		Values: map[string]string{
			"global.ingress.enabled":               "true",
			"global.identity.keycloak.contextPath": "/custom",
			"identityKeycloak.enabled":             "true",
			"identityKeycloak.httpRelativePath":    "/custom",
			"identity.contextPath":                 "/identity",
		},
		Verifier: func(t *testing.T, output string, err error) {
			var statefulSet appsv1.StatefulSet
			helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)

			// then
			env := statefulSet.Spec.Template.Spec.Containers[0].Env
			require.Contains(t, env,
				corev1.EnvVar{
					Name:  "KEYCLOAK_HTTP_RELATIVE_PATH",
					Value: "/custom",
				})
		},
	}}
	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}
