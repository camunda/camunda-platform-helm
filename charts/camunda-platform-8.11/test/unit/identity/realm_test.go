// Copyright 2026 Camunda Services GmbH
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
	"maps"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
)

type RealmTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
	templates []string
}

func TestRealmTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &RealmTest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
		templates: []string{"templates/identity/configmap.yaml"},
	})
}

func realmValues(extra map[string]string) map[string]string {
	values := map[string]string{
		"identity.enabled":             "true",
		"global.identity.auth.enabled": "true",
	}
	maps.Copy(values, extra)
	return values
}

func (s *RealmTest) keycloakRealm(output string) *string {
	var configmap corev1.ConfigMap
	helm.UnmarshalK8SYaml(s.T(), output, &configmap)

	var application struct {
		Keycloak struct {
			Realm *string `yaml:"realm"`
		} `yaml:"keycloak"`
	}
	s.Require().NoError(yaml.Unmarshal([]byte(configmap.Data["application.yaml"]), &application))

	return application.Keycloak.Realm
}

func (s *RealmTest) TestEnabledRealmRequiresId() {
	testCases := []testhelpers.TestCase{
		{
			Name:     "TestEnabledWithoutIdIsRejected",
			Values:   realmValues(map[string]string{"identity.realm.enabled": "true"}),
			Expected: map[string]string{"ERROR": "'/identity/realm/id': minLength: got 0, want 1"},
		},
		{
			Name: "TestEnabledWithIdRendersRealm",
			Values: realmValues(map[string]string{
				"identity.realm.enabled": "true",
				"identity.realm.id":      "camunda-realm",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				realm := s.keycloakRealm(output)
				s.Require().NotNil(realm)
				s.Require().Equal("camunda-realm", *realm)
			},
		},
		{
			Name:   "TestDisabledWithoutIdOmitsRealm",
			Values: realmValues(nil),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				s.Require().Nil(s.keycloakRealm(output))
			},
		},
		{
			Name: "TestDisabledWithIdOmitsRealm",
			Values: realmValues(map[string]string{
				"identity.realm.enabled": "false",
				"identity.realm.id":      "camunda-realm",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				s.Require().Nil(s.keycloakRealm(output))
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}
