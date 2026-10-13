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

package companion

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

type CompanionServiceSelectorTest struct {
	suite.Suite
	namespace string
}

func TestCompanionServiceSelectorTemplate(t *testing.T) {
	t.Parallel()

	suite.Run(t, &CompanionServiceSelectorTest{
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
	})
}

func (s *CompanionServiceSelectorTest) assertSelectorIsolated(chart, template string) {
	s.T().Helper()

	chartPath, err := filepath.Abs(chart)
	s.Require().NoError(err)

	testCases := []testhelpers.TestCase{
		{
			Name:   "TestServiceSelectorExcludesLegacyDeploymentPods",
			Values: map[string]string{},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)

				var service corev1.Service
				helm.UnmarshalK8SYaml(t, output, &service)

				require.Equal(t, "statefulset", service.Spec.Selector["camunda.io/controller"],
					"Service must only select pods the StatefulSet owns")
				require.Equal(t, "postgresql", service.Spec.Selector["app.kubernetes.io/component"])
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), chartPath, "camunda-platform-test", s.namespace, []string{template}, testCases)
}

func (s *CompanionServiceSelectorTest) TestPostgresqlServiceSelector() {
	s.assertSelectorIsolated("../../../../internal-postgresql", "templates/service.yaml")
}

func (s *CompanionServiceSelectorTest) TestKeycloakPostgresqlServiceSelector() {
	s.assertSelectorIsolated("../../../../internal-keycloak-26", "templates/postgresql-service.yaml")
}
