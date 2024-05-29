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

	suite.Run(t, &configMapTemplateTest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
		templates: []string{},
	})
}

func (s *configMapTemplateTest) TestExistingSecretConstraintDisplays() {
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
	s.Require().ErrorContains(err, "As of appVersion 8.7, the camunda helm chart will NOT perform automatic passwords")
}
func (s *configMapTemplateTest) TestExistingSecretConstraintDoesNotDisplayErrorForComponentWithExistingSecret() {
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
func (s *configMapTemplateTest) TestExistingSecretConstraintDoesNotDisplayErrorForComponentThatsDisabled() {
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
func (s *configMapTemplateTest) TestExistingSecretConstraintInWarningModeDoesNotPreventInstall() {
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
