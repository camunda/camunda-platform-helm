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
	"camunda-platform/test/unit/testhelpers"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	coreV1 "k8s.io/api/core/v1"
)

type secretSharedTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
	templates []string
}

func TestSecretSharedTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &secretSharedTest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
		templates: []string{"templates/web-modeler/secret-shared.yaml"},
	})
}

func (s *secretSharedTest) TestDifferentValuesInputs() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestContainerGenerateRandomPusherKeys",
			Values: map[string]string{
				"identity.enabled":                    "true",
				"webModeler.enabled":                  "true",
				"webModeler.restapi.mail.fromAddress": "example@example.com",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var secret coreV1.Secret
				helm.UnmarshalK8SYaml(s.T(), output, &secret)

				s.Require().NotNil(secret.Data)
				s.Require().Regexp("^[a-zA-Z0-9]{20}$", string(secret.Data["pusher-app-secret"]))
				s.Require().Regexp("^[a-zA-Z0-9]{20}$", string(secret.Data["pusher-app-key"]))
			},
		},
		{
			Name: "TestSecretOnlyContainsPusherAppKeyWhenSecretProvided",
			Values: map[string]string{
				"identity.enabled":                                "true",
				"webModeler.enabled":                             "true",
				"webModeler.restapi.mail.fromAddress":            "example@example.com",
				"webModeler.restapi.pusher.secret.inlineSecret":  "my-inline-secret",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var secret coreV1.Secret
				helm.UnmarshalK8SYaml(s.T(), output, &secret)

				s.Require().NotNil(secret.Data)
				s.Require().Empty(secret.Data["pusher-app-secret"])
				s.Require().Regexp("^[a-zA-Z0-9]{20}$", string(secret.Data["pusher-app-key"]))
			},
		},
		{
			Name: "TestSecretOnlyContainsPusherAppSecretWhenAppKeyProvided",
			Values: map[string]string{
				"identity.enabled":                               "true",
				"webModeler.enabled":                            "true",
				"webModeler.restapi.mail.fromAddress":           "example@example.com",
				"webModeler.restapi.pusher.client.secret.inlineSecret":                 "my-inline-app-key",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var secret coreV1.Secret
				helm.UnmarshalK8SYaml(s.T(), output, &secret)

				s.Require().NotNil(secret.Data)
				s.Require().Regexp("^[a-zA-Z0-9]{20}$", string(secret.Data["pusher-app-secret"]))
				s.Require().Empty(secret.Data["pusher-app-key"])
			},
		},
		{
			Name: "TestSecretOnlyContainsPusherAppKeyWhenExistingSecretProvided",
			Values: map[string]string{
				"identity.enabled":                                    "true",
				"webModeler.enabled":                                  "true",
				"webModeler.restapi.mail.fromAddress":                 "example@example.com",
				"webModeler.restapi.pusher.secret.existingSecret":     "my-custom-secret",
				"webModeler.restapi.pusher.secret.existingSecretKey":  "my-pusher-key",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var secret coreV1.Secret
				helm.UnmarshalK8SYaml(s.T(), output, &secret)

				s.Require().NotNil(secret.Data)
				s.Require().Empty(secret.Data["pusher-app-secret"])
				s.Require().Regexp("^[a-zA-Z0-9]{20}$", string(secret.Data["pusher-app-key"]))
			},
		},
		{
			Name: "TestSecretOnlyContainsPusherAppSecretWhenExistingAppKeyProvided",
			Values: map[string]string{
				"identity.enabled":                                          "true",
				"webModeler.enabled":                                        "true",
				"webModeler.restapi.mail.fromAddress":                       "example@example.com",
				"webModeler.restapi.pusher.client.secret.existingSecret":    "my-custom-app-key-secret",
				"webModeler.restapi.pusher.client.secret.existingSecretKey": "my-pusher-app-key",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var secret coreV1.Secret
				helm.UnmarshalK8SYaml(s.T(), output, &secret)

				s.Require().NotNil(secret.Data)
				s.Require().Regexp("^[a-zA-Z0-9]{20}$", string(secret.Data["pusher-app-secret"]))
				s.Require().Empty(secret.Data["pusher-app-key"])
			},
		},
		{
			Name: "TestSecretNotRenderedWhenBothConfigured",
			Values: map[string]string{
				"identity.enabled":                                          "true",
				"webModeler.enabled":                                        "true",
				"webModeler.restapi.mail.fromAddress":                       "example@example.com",
				"webModeler.restapi.pusher.secret.existingSecret":           "my-custom-secret",
				"webModeler.restapi.pusher.secret.existingSecretKey":        "my-pusher-key",
				"webModeler.restapi.pusher.client.secret.existingSecret":    "my-custom-app-key-secret",
				"webModeler.restapi.pusher.client.secret.existingSecretKey": "my-pusher-app-key",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), "could not find template")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}
