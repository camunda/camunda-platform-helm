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

package camunda

import (
	"camunda-platform/test/unit/testhelpers"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
)

type MultiOptimizeReferenceTemplateTest struct {
	suite.Suite
	chartPath          string
	platformValues     string
	optimizeOnlyValues string
}

func TestMultiOptimizeReferenceTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	valuesPath := filepath.Join(chartPath, "test/integration/scenarios/chart-full-setup/values/features/multi-optimize")
	suite.Run(t, &MultiOptimizeReferenceTemplateTest{
		chartPath:          chartPath,
		platformValues:     filepath.Join(valuesPath, "values-platform.yaml"),
		optimizeOnlyValues: filepath.Join(valuesPath, "values-optimize-only.yaml"),
	})
}

func (s *MultiOptimizeReferenceTemplateTest) TestPlatformReleaseRegistersSecondOptimize() {
	testCases := []testhelpers.TestCase{
		{
			Name:        "TestIdentityClientConfiguration",
			Template:    "templates/identity/configmap.yaml",
			ValuesFiles: []string{s.platformValues},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)

				var configMap corev1.ConfigMap
				helm.UnmarshalK8SYaml(t, output, &configMap)
				application := configMap.Data["application.yaml"]

				s.Require().Contains(application, "id: optimize_team_b")
				s.Require().Contains(application, "secret: ${VALUES_OPTIMIZE_TEAM_B_CLIENT_SECRET:}")
				s.Require().Contains(application, "redirectUris: /api/authentication/callback")
				s.Require().Contains(application, "rootUrl: https://camunda.example.com/optimize-team-b")
				s.Require().Contains(application, "resourceServerId: optimize-api")
				s.Require().Contains(application, "resourceServerId: camunda-identity-resource-server")
				s.Require().Contains(application, `root-url: "https://camunda.example.com/modeler"`)
			},
		},
		{
			Name:        "TestIdentityClientSecretReference",
			Template:    "templates/identity/deployment.yaml",
			ValuesFiles: []string{s.platformValues},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)

				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)
				secretEnv := findEnvVar(deployment.Spec.Template.Spec.Containers[0].Env, "VALUES_OPTIMIZE_TEAM_B_CLIENT_SECRET")

				s.Require().NotNil(secretEnv.ValueFrom)
				s.Require().NotNil(secretEnv.ValueFrom.SecretKeyRef)
				s.Require().Equal("multi-optimize-credentials", secretEnv.ValueFrom.SecretKeyRef.Name)
				s.Require().Equal("optimize-team-b-client-secret", secretEnv.ValueFrom.SecretKeyRef.Key)
			},
		},
		{
			Name:        "TestPlatformOptimizeDeployment",
			Template:    "templates/optimize/deployment.yaml",
			ValuesFiles: []string{s.platformValues},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)

				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)
				env := deployment.Spec.Template.Spec.Containers[0].Env

				s.Require().Equal("elasticsearch-master", findEnvVar(env, "OPTIMIZE_ELASTICSEARCH_HOST").Value)
				s.Require().Equal("optimize-team-a", findEnvVar(env, "CAMUNDA_OPTIMIZE_ELASTICSEARCH_SETTINGS_INDEX_PREFIX").Value)

				clientSecret := findEnvVar(env, "CAMUNDA_IDENTITY_CLIENT_SECRET")
				s.Require().NotNil(clientSecret.ValueFrom)
				s.Require().NotNil(clientSecret.ValueFrom.SecretKeyRef)
				s.Require().Equal("multi-optimize-credentials", clientSecret.ValueFrom.SecretKeyRef.Name)
				s.Require().Equal("optimize-team-a-client-secret", clientSecret.ValueFrom.SecretKeyRef.Key)
			},
		},
		{
			Name:        "TestPlatformIngress",
			Template:    "templates/common/ingress-http.yaml",
			ValuesFiles: []string{s.platformValues},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)

				var ingress netv1.Ingress
				helm.UnmarshalK8SYaml(t, output, &ingress)
				backends := ingressBackends(ingress)

				s.Require().Equal("keycloak", backends["/auth/"])
				s.Require().Equal("platform-identity", backends["/identity"])
				s.Require().Equal("platform-optimize", backends["/optimize-team-a"])
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, "platform", "multi-optimize-reference", nil, testCases)
}

func (s *MultiOptimizeReferenceTemplateTest) TestOptimizeOnlyRelease() {
	testCases := []testhelpers.TestCase{
		{
			Name:        "TestOptimizeConfiguration",
			Template:    "templates/optimize/configmap.yaml",
			ValuesFiles: []string{s.optimizeOnlyValues},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)

				var configMap corev1.ConfigMap
				helm.UnmarshalK8SYaml(t, output, &configMap)
				configuration := configMap.Data["environment-config.yaml"]
				authConfiguration := configMap.Data["application-ccsm.yaml"]

				s.Require().Contains(configuration, `contextPath: "/optimize-team-b"`)
				s.Require().Contains(configuration, `host: "elasticsearch-master"`)
				s.Require().Contains(configuration, `redirectRootUrl: "https://camunda.example.com/optimize-team-b"`)
				s.Require().Contains(authConfiguration, `clientId: "optimize_team_b"`)
			},
		},
		{
			Name:        "TestIdentityServiceConfiguration",
			Template:    "templates/common/configmap-identity-auth.yaml",
			ValuesFiles: []string{s.optimizeOnlyValues},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)

				var configMap corev1.ConfigMap
				helm.UnmarshalK8SYaml(t, output, &configMap)
				s.Require().Equal("http://platform-identity:80/identity", configMap.Data["CAMUNDA_IDENTITY_BASEURL"])
				s.Require().Equal("http://keycloak:80/auth/realms/camunda-platform", configMap.Data["CAMUNDA_IDENTITY_ISSUER_BACKEND_URL"])
			},
		},
		{
			Name:        "TestOptimizeDeployment",
			Template:    "templates/optimize/deployment.yaml",
			ValuesFiles: []string{s.optimizeOnlyValues},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)

				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)
				env := deployment.Spec.Template.Spec.Containers[0].Env

				s.Require().Equal("elasticsearch-master", findEnvVar(env, "OPTIMIZE_ELASTICSEARCH_HOST").Value)
				s.Require().Equal("optimize-team-b", findEnvVar(env, "CAMUNDA_OPTIMIZE_ELASTICSEARCH_SETTINGS_INDEX_PREFIX").Value)

				clientSecret := findEnvVar(env, "CAMUNDA_IDENTITY_CLIENT_SECRET")
				s.Require().NotNil(clientSecret.ValueFrom)
				s.Require().NotNil(clientSecret.ValueFrom.SecretKeyRef)
				s.Require().Equal("multi-optimize-credentials", clientSecret.ValueFrom.SecretKeyRef.Name)
				s.Require().Equal("optimize-team-b-client-secret", clientSecret.ValueFrom.SecretKeyRef.Key)
			},
		},
		{
			Name:        "TestOptimizeIngress",
			Template:    "templates/common/ingress-http.yaml",
			ValuesFiles: []string{s.optimizeOnlyValues},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)

				var ingress netv1.Ingress
				helm.UnmarshalK8SYaml(t, output, &ingress)
				s.Require().Equal("camunda.example.com", ingress.Spec.Rules[0].Host)
				s.Require().Len(ingress.Spec.Rules[0].HTTP.Paths, 1)
				s.Require().Equal("/optimize-team-b", ingress.Spec.Rules[0].HTTP.Paths[0].Path)
				s.Require().Equal("optimize-team-b", ingress.Spec.Rules[0].HTTP.Paths[0].Backend.Service.Name)
			},
		},
		{
			Name:        "TestOnlyOptimizeWorkloadRenders",
			ValuesFiles: []string{s.optimizeOnlyValues},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				s.Require().Equal([]string{"optimize-team-b"}, workloadNames(t, output))
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, "optimize-team-b", "multi-optimize-reference", nil, testCases)
}

func findEnvVar(env []corev1.EnvVar, name string) corev1.EnvVar {
	for _, envVar := range env {
		if envVar.Name == name {
			return envVar
		}
	}

	return corev1.EnvVar{}
}

func ingressBackends(ingress netv1.Ingress) map[string]string {
	backends := map[string]string{}
	for _, path := range ingress.Spec.Rules[0].HTTP.Paths {
		backends[path.Path] = path.Backend.Service.Name
	}
	return backends
}

func workloadNames(t *testing.T, output string) []string {
	decoder := yaml.NewYAMLOrJSONDecoder(strings.NewReader(output), 4096)
	workloads := []string{}

	for {
		var object unstructured.Unstructured
		err := decoder.Decode(&object)
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		if object.GetKind() == "Deployment" || object.GetKind() == "StatefulSet" {
			workloads = append(workloads, object.GetName())
		}
	}

	return workloads
}
