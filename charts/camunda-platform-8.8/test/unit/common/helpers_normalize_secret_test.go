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

type normalizeSecretConfigTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
	templates []string
}

func TestNormalizeSecretConfigTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &normalizeSecretConfigTest{
		chartPath: chartPath,
		release:   "test",
		namespace: "test-" + strings.ToLower(random.UniqueId()),
		templates: []string{"templates/orchestration/statefulset.yaml"},
	})
}

func (s *normalizeSecretConfigTest) TestSecretHelperFunctionsWithOpenSearch() {
	testCases := []testhelpers.TestCase{
		{
			Name: "opensearch new style secret creates env vars",
			Values: map[string]string{
				"orchestration.enabled":                              "true",
				"global.opensearch.enabled":                 "true",
				"global.opensearch.auth.secret.existingSecret":    "my-opensearch-secret",
				"global.opensearch.auth.secret.existingSecretKey": "my-key",
			},
			Expected: map[string]string{
				"spec.template.spec.containers[0].env[?(@.name=='CAMUNDA_OPERATE_ZEEBE_OPENSEARCH_PASSWORD')].valueFrom.secretKeyRef.name": "my-opensearch-secret",
				"spec.template.spec.containers[0].env[?(@.name=='CAMUNDA_OPERATE_ZEEBE_OPENSEARCH_PASSWORD')].valueFrom.secretKeyRef.key":  "my-key",
			},
		},
		{
			Name: "opensearch inline secret creates env vars with direct values",
			Values: map[string]string{
				"orchestration.enabled":                          "true",
				"global.opensearch.enabled":             "true",
				"global.opensearch.auth.secret.inlineSecret": "my-password",
			},
			Expected: map[string]string{
				"spec.template.spec.containers[0].env[?(@.name=='CAMUNDA_OPERATE_ZEEBE_OPENSEARCH_PASSWORD')].value": "my-password",
			},
		},
		{
			Name: "opensearch legacy secret format creates env vars",
			Values: map[string]string{
				"orchestration.enabled":                      "true",
				"global.opensearch.enabled":         "true",
				"global.opensearch.auth.existingSecret":    "legacy-secret",
				"global.opensearch.auth.existingSecretKey": "legacy-key",
			},
			Expected: map[string]string{
				"spec.template.spec.containers[0].env[?(@.name=='CAMUNDA_OPERATE_ZEEBE_OPENSEARCH_PASSWORD')].valueFrom.secretKeyRef.name": "legacy-secret",
				"spec.template.spec.containers[0].env[?(@.name=='CAMUNDA_OPERATE_ZEEBE_OPENSEARCH_PASSWORD')].valueFrom.secretKeyRef.key":  "legacy-key",
			},
		},
		{
			Name: "opensearch plaintext password creates env vars",
			Values: map[string]string{
				"orchestration.enabled":                   "true",
				"global.opensearch.enabled":      "true",
				"global.opensearch.auth.password": "plain-password",
			},
			Expected: map[string]string{
				"spec.template.spec.containers[0].env[?(@.name=='CAMUNDA_OPERATE_ZEEBE_OPENSEARCH_PASSWORD')].value": "plain-password",
			},
		},
		{
			Name: "no opensearch config means no env vars",
			Values: map[string]string{
				"orchestration.enabled":              "true",
				"global.opensearch.enabled": "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				// Should not create any opensearch password env vars
				require.NotContains(t, output, "CAMUNDA_OPERATE_ZEEBE_OPENSEARCH_PASSWORD")
			},
		},
		{
			Name: "opensearch disabled means no env vars",
			Values: map[string]string{
				"orchestration.enabled":                             "true",
				"global.opensearch.enabled":                "false",
				"global.opensearch.auth.secret.inlineSecret": "password",
			},
			Verifier: func(t *testing.T, output string, err error) {
				// Should not create any opensearch password env vars when opensearch is disabled
				require.NotContains(t, output, "CAMUNDA_OPERATE_ZEEBE_OPENSEARCH_PASSWORD")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}
