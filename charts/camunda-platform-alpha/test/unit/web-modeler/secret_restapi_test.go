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

package web_modeler

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
	chartPath string
	release   string
	namespace string
	templates []string
}

func TestSecretRestapiTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &secretTest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
		templates: []string{"templates/web-modeler/secret-restapi.yaml"},
	})
}

func (s *secretTest) TestContainerCreateExternalDatabasePasswordSecret() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"webModeler.enabled":                           "true",
			"webModeler.restapi.mail.fromAddress":          "example@example.com",
			"webModelerPostgresql.enabled":                 "false",
			"webModeler.restapi.externalDatabase.password": "secret123",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var secret coreV1.Secret
	helm.UnmarshalK8SYaml(s.T(), output, &secret)

	// then
	s.Require().NotNil(secret.Data)
	s.Require().Equal("secret123", string(secret.Data["database-password"]))
}

func (s *secretTest) TestContainerCreateSmtpPasswordSecret() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"webModeler.enabled":                   "true",
			"webModeler.restapi.mail.fromAddress":  "example@example.com",
			"webModeler.restapi.mail.smtpPassword": "secret123",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var secret coreV1.Secret
	helm.UnmarshalK8SYaml(s.T(), output, &secret)

	// then
	s.Require().NotNil(secret.Data)
	s.Require().Equal("secret123", string(secret.Data["smtp-password"]))
}
func (s *secretTest) TestDatabaseSecretAddedViaName() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"webModeler.enabled":                                      "true",
			"webModeler.restapi.mail.fromAddress":                     "example@example.com",
			"webModeler.restapi.mail.smtpPassword":                    "secret123",
			"webModeler.restapi.externalDatabase.existingSecret.name": "example-secret",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var secret coreV1.Secret
	helm.UnmarshalK8SYaml(s.T(), output, &secret)

	// then
	s.Require().NotNil(secret.Data)
	_, databaseSecretKeyExists := secret.Data["database-password"]
	s.Require().False(databaseSecretKeyExists)
}
func (s *secretTest) TestDatabaseSecretAddedViaDirectPassword() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"webModeler.enabled":                                 "true",
			"webModeler.restapi.mail.fromAddress":                "example@example.com",
			"webModeler.restapi.mail.smtpPassword":               "secret123",
			"webModeler.restapi.externalDatabase.existingSecret": "password1234",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var secret coreV1.Secret
	helm.UnmarshalK8SYaml(s.T(), output, &secret)

	// then
	s.Require().Equal("password1234", string(secret.Data["database-password"]))
}

func (s *secretTest) TestDatabaseSecretAddedViaDirectPasswordUsingOldSyntax() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"webModeler.enabled":                           "true",
			"webModeler.restapi.mail.fromAddress":          "example@example.com",
			"webModeler.restapi.mail.smtpPassword":         "secret123",
			"webModeler.restapi.externalDatabase.password": "password1234",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var secret coreV1.Secret
	helm.UnmarshalK8SYaml(s.T(), output, &secret)

	// then
	s.Require().Equal("password1234", string(secret.Data["database-password"]))
}

func (s *secretTest) TestSmtpSecretAddedViaDirectPassword() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"webModeler.enabled":                                 "true",
			"webModeler.restapi.mail.fromAddress":                "example@example.com",
			"webModeler.restapi.mail.existingSecret":             "password1234",
			"webModeler.restapi.externalDatabase.existingSecret": "otherPassword1234",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var secret coreV1.Secret
	helm.UnmarshalK8SYaml(s.T(), output, &secret)

	// then
	s.Require().Equal("password1234", string(secret.Data["smtp-password"]))
}
func (s *secretTest) TestSmtpSecretNotAddedWhenSecretNameSpecified() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"webModeler.enabled":                                 "true",
			"webModeler.restapi.mail.fromAddress":                "example@example.com",
			"webModeler.restapi.externalDatabase.existingSecret": "password1234",
			"webModeler.restapi.mail.existingSecret.name":        "mail-secret",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var secret coreV1.Secret
	helm.UnmarshalK8SYaml(s.T(), output, &secret)

	// then
	_, exists := secret.Data["smtp-password"]
	s.Require().False(exists)
}
func (s *secretTest) TestSmtpSecretViaPasswordKeyWhenSecretNameSpecified() {
	// given
	options := &helm.Options{
		SetValues: map[string]string{
			"webModeler.enabled":                                 "true",
			"webModeler.restapi.mail.fromAddress":                "example@example.com",
			"webModeler.restapi.externalDatabase.existingSecret": "password1234",
			"webModeler.restapi.mail.smtpPassword":               "password12345",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	// when
	output := helm.RenderTemplate(s.T(), options, s.chartPath, s.release, s.templates)
	var secret coreV1.Secret
	helm.UnmarshalK8SYaml(s.T(), output, &secret)

	// then
	s.Require().Equal("password12345", string(secret.Data["smtp-password"]))
}
