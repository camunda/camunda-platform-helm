// secretstore_constraints_test.go
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

type secretStoreConstraintsTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
	templates []string
}

func TestSecretStoreConstraintsTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &secretStoreConstraintsTest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
		templates: []string{"templates/orchestration/serviceaccount.yaml"},
	})
}

func (s *secretStoreConstraintsTest) TestConstraintFailures() {
	testCases := []testhelpers.TestCase{
		{
			Name:     "More than one store is rejected",
			Template: "templates/orchestration/serviceaccount.yaml",
			Values: mergeValues(baseValues(), map[string]string{
				"global.secretStore.file.a.existingSecret": "s",
				"global.secretStore.aws.b.region":          "us-east-1",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), "supports only one secret store at a time")
			},
		},
		{
			Name:     "AWS batchSize above 20 is rejected",
			Template: "templates/orchestration/serviceaccount.yaml",
			Values: mergeValues(baseValues(), map[string]string{
				"global.secretStore.aws.a.batchSize": "50",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), "batchSize must be between 1 and 20")
			},
		},
		{
			Name:     "AWS batchEnabled with containerSecretId is rejected",
			Template: "templates/orchestration/serviceaccount.yaml",
			Values: mergeValues(baseValues(), map[string]string{
				"global.secretStore.aws.a.batchEnabled":      "true",
				"global.secretStore.aws.a.containerSecretId": "app-config",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), "mutually exclusive")
			},
		},
		{
			Name:     "GCP pathPrefix with invalid characters is rejected",
			Template: "templates/orchestration/serviceaccount.yaml",
			Values: mergeValues(baseValues(), map[string]string{
				"global.secretStore.gcp.a.pathPrefix": "camunda/",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), "must contain only [a-zA-Z0-9_-]")
			},
		},
		{
			Name:     "More than one store within a physical tenant is rejected",
			Template: "templates/orchestration/serviceaccount.yaml",
			Values: mergeValues(baseValues(), map[string]string{
				"global.secretStore.physicalTenants.tenantA.file.a.existingSecret": "s",
				"global.secretStore.physicalTenants.tenantA.aws.b.region":          "us-east-1",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), "global.secretStore.physicalTenants.tenantA supports only one secret store at a time")
			},
		},
		{
			Name:     "Conflicting roleArn across tenants is rejected",
			Template: "templates/orchestration/serviceaccount.yaml",
			Values: mergeValues(baseValues(), map[string]string{
				"global.secretStore.aws.a.roleArn":                         "arn:aws:iam::111111111111:role/one",
				"global.secretStore.physicalTenants.tenantA.aws.b.roleArn": "arn:aws:iam::222222222222:role/two",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), "multiple distinct aws.*.roleArn values")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}
