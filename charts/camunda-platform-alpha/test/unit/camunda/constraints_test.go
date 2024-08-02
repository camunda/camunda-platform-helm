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
	"path/filepath"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type constraintTemplateTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
	templates []string
}

func TestConstraintTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &constraintTemplateTest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
		templates: []string{},
	})
}

func (s *constraintTemplateTest) TestExistingSecretConstraintDisplays() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"global.identity.auth.issuerBackendUrl":                "http://keycloak:80/auth/realms/camunda-platform",
			"global.testDeprecationFlags.existingSecretsMustBeSet": "error",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	_, err := helm.RenderTemplateE(s.T(), options, s.chartPath, s.release, s.templates)

	// then
	s.Require().ErrorContains(err, "the Camunda Helm chart will no longer automatically generate passwords for the Identity component")
}
func (s *constraintTemplateTest) TestExistingSecretConstraintDoesNotDisplayErrorForComponentWithExistingSecret() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"global.identity.auth.issuerBackendUrl":                "http://keycloak:80/auth/realms/camunda-platform",
			"global.testDeprecationFlags.existingSecretsMustBeSet": "error",
			"global.identity.auth.zeebe.existingSecret.name":       "zeebe-secret",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	_, err := helm.RenderTemplateE(s.T(), options, s.chartPath, s.release, s.templates)

	// then
	s.Require().NotContains(err.Error(), "global.identity.auth.zeebe.existingSecret")
}
func (s *constraintTemplateTest) TestExistingSecretConstraintDoesNotDisplayErrorForComponentThatsDisabled() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"global.identity.auth.issuerBackendUrl":                "http://keycloak:80/auth/realms/camunda-platform",
			"global.testDeprecationFlags.existingSecretsMustBeSet": "error",
			"operate.enabled": "false",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	_, err := helm.RenderTemplateE(s.T(), options, s.chartPath, s.release, s.templates)

	// then
	s.Require().NotContains(err.Error(), "global.identity.auth.operate.existingSecret")
}
func (s *constraintTemplateTest) TestExistingSecretConstraintInWarningModeDoesNotPreventInstall() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"global.identity.auth.issuerBackendUrl":                "http://keycloak:80/auth/realms/camunda-platform",
			"global.testDeprecationFlags.existingSecretsMustBeSet": "warning",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	_, err := helm.RenderTemplateE(s.T(), options, s.chartPath, s.release, s.templates)

	// then
	s.Require().Nil(err)
}

func (s *ConstraintsTemplateTest) TestContextPathAndRestPathForZeebeGatewayConstraintBothValuesShouldBeTheSame() {
	options := &helm.Options{
		SetValues: map[string]string{
			"zeebeGateway.ingress.rest.enabled": "true",
			"zeebeGateway.ingress.rest.path":    "/zeebe",
			"zeebeGateway.contextPath":          "/zeebeRest",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	_, err := helm.RenderTemplateE(s.T(), options, s.chartPath, s.release, s.templates)

	s.Require().Error(err, "[camunda][error] zeebeGateway.ingress.rest.path and zeebeGateway.contextPath must have the same value.")

}
