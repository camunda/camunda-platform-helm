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

type ConfigMapWarningsTemplateTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
	templates []string
}

func TestConfigMapWarningsTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &ConfigMapWarningsTemplateTest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
		templates: []string{"templates/camunda/configmap-warnings.yaml"},
	})
}

func (s *ConfigMapWarningsTemplateTest) TestDifferentValuesInputs() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestWarningsConfigMapRendersWhenWarningPresent",
			Values: map[string]string{
				"global.testDeprecationFlags.existingSecretsMustBeSet": "warning",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)
				s.Require().True(strings.HasSuffix(configmap.Name, "-warnings"))
				s.Require().Contains(configmap.Data["warnings"],
					"the Camunda Helm chart will no longer automatically generate passwords for the Identity component")
			},
		},
		{
			Name: "TestWarningsConfigMapAbsentWhenNoWarnings",
			// identityKeycloak is enabled by default on 8.7 and its image is affected by
			// CVE-2026-18963, so it must be disabled for a warning-free render.
			Values: map[string]string{
				"global.testDeprecationFlags.existingSecretsMustBeSet": "false",
				"identityKeycloak.enabled":                             "false",
			},
			Verifier: func(t *testing.T, output string, err error) {
				// With no active warnings the helper renders nothing, so --show-only finds no manifest.
				s.Require().Error(err)
				s.Require().NotContains(output, "kind: ConfigMap")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *ConfigMapWarningsTemplateTest) TestBundledKeycloakCveWarning() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestAffectedVersionWarns",
			Values: map[string]string{
				"identityKeycloak.enabled": "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)
				s.Require().Contains(configmap.Data["warnings"], "CVE-2026-18963")
			},
		},
		{
			Name: "TestBitnamiRevisionSuffixIsParsed",
			Values: map[string]string{
				"identityKeycloak.enabled":   "true",
				"identityKeycloak.image.tag": "26.3.3-debian-12-r0-2026-08-27-001",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(s.T(), output, &configmap)
				s.Require().Contains(configmap.Data["warnings"], "CVE-2026-18963")
			},
		},
		{
			Name: "TestFixedVersionDoesNotWarn",
			Values: map[string]string{
				"identityKeycloak.enabled":   "true",
				"identityKeycloak.image.tag": "26.7.2",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NotContains(output, "CVE-2026-18963")
			},
		},
		{
			Name: "TestUnparseableTagDoesNotWarn",
			Values: map[string]string{
				"identityKeycloak.enabled":   "true",
				"identityKeycloak.image.tag": "latest",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NotContains(output, "CVE-2026-18963")
			},
		},
		{
			Name: "TestDisabledKeycloakDoesNotWarn",
			Values: map[string]string{
				"identityKeycloak.enabled": "false",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NotContains(output, "CVE-2026-18963")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}
