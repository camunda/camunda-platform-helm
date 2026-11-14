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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

const kcAdminApiScopeContainer = "keycloak-admin-api-scope"

type kcAdminApiScopeTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
	templates []string
}

func TestKeycloakAdminApiScopeSidecar(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &kcAdminApiScopeTest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
		templates: []string{"templates/identity/deployment.yaml"},
	})
}

func findContainer(containers []corev1.Container, name string) *corev1.Container {
	for i := range containers {
		if containers[i].Name == name {
			return &containers[i]
		}
	}
	return nil
}

func envValue(container *corev1.Container, name string) string {
	for _, env := range container.Env {
		if env.Name == name {
			return env.Value
		}
	}
	return ""
}

func (s *kcAdminApiScopeTest) TestDifferentValuesInputs() {
	bundledKeycloak := map[string]string{
		"identity.enabled":             "true",
		"identityKeycloak.enabled":     "true",
		"global.identity.auth.enabled": "true",
	}
	with := func(extra map[string]string) map[string]string {
		values := map[string]string{}
		for k, v := range bundledKeycloak {
			values[k] = v
		}
		for k, v := range extra {
			values[k] = v
		}
		return values
	}

	testCases := []testhelpers.TestCase{
		{
			Name: "TestSidecarWithBundledKeycloak",
			Values: with(map[string]string{
				"identityKeycloak.image.registry":   "registry.camunda.cloud",
				"identityKeycloak.image.repository": "keycloak-ee/keycloak",
				"identityKeycloak.image.tag":        "26.8.2",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)

				spec := deployment.Spec.Template.Spec
				sidecar := findContainer(spec.Containers, kcAdminApiScopeContainer)
				s.Require().NotNil(sidecar, "the kc-admin-api sidecar should be rendered with the bundled Keycloak")
				s.Require().Equal("registry.camunda.cloud/keycloak-ee/keycloak:26.8.2", sidecar.Image)
				s.Require().Equal("camunda-platform", envValue(sidecar, "KEYCLOAK_REALM"))
				s.Require().Equal("camunda-identity", envValue(sidecar, "IDENTITY_CLIENT_ID"))
				s.Require().Equal("http://camunda-platform-test-keycloak/auth", envValue(sidecar, "KEYCLOAK_URL"))
				s.Require().Contains(spec.Volumes, corev1.Volume{
					Name:         "keycloak-admin-api-scope-tmp",
					VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
				})
			},
		}, {
			Name: "TestSidecarAddsKeycloakImagePullSecrets",
			Values: with(map[string]string{
				"global.image.pullSecrets[0].name":           "SecretName",
				"identity.image.pullSecrets[0].name":         "IdentitySecret",
				"identityKeycloak.image.pullSecrets[0].name": "registry-camunda-cloud",
				"identityKeycloak.image.pullSecrets[1].name": "IdentitySecret",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)

				s.Require().Equal([]corev1.LocalObjectReference{
					{Name: "IdentitySecret"},
					{Name: "registry-camunda-cloud"},
				}, deployment.Spec.Template.Spec.ImagePullSecrets)
			},
		}, {
			Name: "TestSidecarUsesTheConfiguredClientId",
			Values: with(map[string]string{
				"global.identity.auth.identity.clientId": "my-identity",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)

				sidecar := findContainer(deployment.Spec.Template.Spec.Containers, kcAdminApiScopeContainer)
				s.Require().NotNil(sidecar)
				s.Require().Equal("my-identity", envValue(sidecar, "IDENTITY_CLIENT_ID"))
			},
		}, {
			Name: "TestNoSidecarWithExternalKeycloak",
			Values: map[string]string{
				"identity.enabled":                                "true",
				"identityKeycloak.enabled":                        "false",
				"global.identity.auth.enabled":                    "true",
				"global.identity.keycloak.url.protocol":           "https",
				"global.identity.keycloak.url.host":               "keycloak.example.com",
				"global.identity.keycloak.url.port":               "8443",
				"global.identity.keycloak.auth.adminUser":         "testAdmin",
				"global.identity.keycloak.auth.existingSecret":    "ownExistingSecretKeycloak",
				"global.identity.keycloak.auth.existingSecretKey": "test-admin",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)

				spec := deployment.Spec.Template.Spec
				s.Require().Nil(findContainer(spec.Containers, kcAdminApiScopeContainer))
				for _, volume := range spec.Volumes {
					s.Require().NotEqual("keycloak-admin-api-scope-tmp", volume.Name)
				}
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}
