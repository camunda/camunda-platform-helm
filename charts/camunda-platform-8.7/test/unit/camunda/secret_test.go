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
	coreV1 "k8s.io/api/core/v1"
)

type secretTest struct {
	suite.Suite
	chartPath  string
	release    string
	namespace  string
	templates  []string
	secretName []string
}

func TestSecretTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &secretTest{
		chartPath:  chartPath,
		release:    "camunda-platform-test",
		namespace:  "camunda-platform-" + strings.ToLower(random.UniqueId()),
		templates:  []string{},
		secretName: []string{},
	})
}

func (s *secretTest) TestContainerGenerateSecret() {
	// given
	options := &helm.Options{
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	s.templates = []string{
		"templates/camunda/secret-connectors.yaml",
		"templates/camunda/secret-console.yaml",
		"templates/camunda/secret-operate.yaml",
		"templates/camunda/secret-optimize.yaml",
		"templates/camunda/secret-tasklist.yaml",
		"templates/camunda/secret-zeebe.yaml",
	}

	s.secretName = []string{
		"connectors-secret",
		"console-secret",
		"operate-secret",
		"optimize-secret",
		"tasklist-secret",
		"zeebe-secret",
	}

	s.Require().GreaterOrEqual(6, len(s.templates))
	for idx, template := range s.templates {
		// when
		output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, []string{template})
		var secret coreV1.Secret
		helm.UnmarshalK8SYaml(s.T(), output, &secret)

		// then
		s.Require().NotNil(secret.Data)
		s.Require().NotNil(secret.Data[s.secretName[idx]])
		s.Require().NotEmpty(secret.Data[s.secretName[idx]])
	}
}

func (s *deploymentTemplateTest) TestContainerCamundaLicenseWithExistingSecret() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"global.license.existingSecret":    "ownExistingSecretForLicense",
			"global.license.existingSecretKey": "camunda-license",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
		ExtraArgs:      map[string][]string{"install": {"--debug"}},
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)

	// then
	// TODO: TODO: Extend this test to ensure that the key is available for all components.
	s.Require().Contains(output, "name: CAMUNDA_LICENSE_KEY")
	s.Require().Contains(output, "name: ownExistingSecretForLicense")
	s.Require().Contains(output, "key: camunda-license")
}
