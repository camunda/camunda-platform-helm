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
	"path/filepath"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	appsv1 "k8s.io/api/apps/v1"
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

func (s *secretTest) TestSecretExternalDatabaseEnabledWithDefinedPassword() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"identityPostgresql.enabled":         "false",
			"identity.externalDatabase.enabled":  "true",
			"identity.externalDatabase.password": "super-secure-ext",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	s.templates = []string{
		"templates/identity/postgresql-secret.yaml",
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var secret coreV1.Secret
	helm.UnmarshalK8SYaml(s.T(), output, &secret)

	// then
	s.NotEmpty(secret.Data)
	s.Require().Equal("super-secure-ext", string(secret.Data["password"]))
}

func (s *secretTest) TestSupport21601() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"identity.postgresql.enabled":       "true",
			"identity.externalDatabase.enabled": "false",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// There are 2 possible ways for this secret to be rendered:
	// 1. through charts/identityPostgresql/templates/secrets.yaml for situations when identityPostgresql is enabled
	// 2. through templates/identity/postgresql-secret.yaml for situations when identityPostgresql is disabled but
	//    there is an externalDatabase configured
	postgresqlSubchartSecretTemplatePath := []string{
		"charts/identityPostgresql/templates/secrets.yaml",
	}

	identityPostgresqlMainChartSecretTemplatePath := []string{
		"templates/identity/postgresql-secret.yaml",
	}

	deploymentTemplateName := []string{
		"templates/identity/deployment.yaml",
	}

	// when
	postgresqlSubchartSecret := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, postgresqlSubchartSecretTemplatePath)
	_, mainChartSecretError := helm.RenderTemplateE(s.T(), options, s.chartPath, s.release, identityPostgresqlMainChartSecretTemplatePath)
	deploymentTemplate := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, deploymentTemplateName)
	var secret coreV1.Secret
	var deployment appsv1.Deployment

	helm.UnmarshalK8SYaml(s.T(), postgresqlSubchartSecret, &secret)
	helm.UnmarshalK8SYaml(s.T(), deploymentTemplate, &deployment)

	envVars := deployment.Spec.Template.Spec.Containers[0].Env
	var identityDatabasePassword coreV1.EnvVar
	for _, envVar := range envVars {
		if envVar.Name == "IDENTITY_DATABASE_PASSWORD" {
			identityDatabasePassword = envVar
		}
	}

	// then

	// I expect Deployment to be rendered
	// I expect Secret to NOT be rendered via charts/identityPostgresql/templates/secrets.yaml
	// I expect Secret to be rendered via templates/identity/postgresql-secret.yaml
	// I expect Deployment NOT to have a referenced password?
	// I expect Deployment environment variable to reference the secret that is rendered
	s.Require().ErrorContains(mainChartSecretError, "could not find template templates/identity/postgresql-secret.yaml in chart")
	s.Require().Equal("IDENTITY_DATABASE_PASSWORD", identityDatabasePassword.Name)
	s.Require().Equal("camunda-platform-test-identity-postgresql", identityDatabasePassword.ValueFrom.SecretKeyRef.Name)
	s.Require().Equal("camunda-platform-test-identity-postgresql", secret.ObjectMeta.Name)
	s.Require().NotEmpty(string(secret.Data["password"]))
}
