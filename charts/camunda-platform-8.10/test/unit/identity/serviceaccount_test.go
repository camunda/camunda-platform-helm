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

package identity

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

type serviceAccountTemplateTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
	templates []string
}

func TestServiceAccountTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &serviceAccountTemplateTest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
		templates: []string{"templates/identity/serviceaccount.yaml"},
	})
}

func (s *serviceAccountTemplateTest) TestAutomountServiceAccountToken() {
	baseValues := func() map[string]string {
		return map[string]string{
			"identity.enabled":             "true",
			"global.identity.auth.enabled": "true",
		}
	}

	optInValues := baseValues()
	optInValues["identity.serviceAccount.automountServiceAccountToken"] = "true"

	testCases := []testhelpers.TestCase{
		{
			Name:   "TestServiceAccountDisablesAutomountByDefault",
			Values: baseValues(),
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var sa corev1.ServiceAccount
				helm.UnmarshalK8SYaml(t, output, &sa)
				require.NotNil(t, sa.AutomountServiceAccountToken)
				require.False(t, *sa.AutomountServiceAccountToken)
			},
		}, {
			Name:   "TestServiceAccountAllowsAutomountOptIn",
			Values: optInValues,
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var sa corev1.ServiceAccount
				helm.UnmarshalK8SYaml(t, output, &sa)
				require.NotNil(t, sa.AutomountServiceAccountToken)
				require.True(t, *sa.AutomountServiceAccountToken)
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}
