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
	"testing"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

func requireTLSCustomKey(t *testing.T, podSpec corev1.PodSpec, secretName, secretKey, mountDirectory string, expectedMounts int) {
	t.Helper()

	var tlsVolume *corev1.Volume
	for index := range podSpec.Volumes {
		if podSpec.Volumes[index].Name == "keystore" {
			tlsVolume = &podSpec.Volumes[index]
			break
		}
	}
	require.NotNil(t, tlsVolume)
	require.NotNil(t, tlsVolume.Secret)
	require.Equal(t, secretName, tlsVolume.Secret.SecretName)

	containers := append([]corev1.Container{}, podSpec.InitContainers...)
	containers = append(containers, podSpec.Containers...)
	mounts := 0
	for _, container := range containers {
		var tlsMount *corev1.VolumeMount
		for index := range container.VolumeMounts {
			if container.VolumeMounts[index].Name == "keystore" {
				tlsMount = &container.VolumeMounts[index]
				break
			}
		}
		if tlsMount == nil {
			continue
		}

		mounts++
		require.Equal(t, secretKey, tlsMount.SubPath)
		require.Equal(t, mountDirectory+"/"+secretKey, tlsMount.MountPath)

		javaOptions := ""
		for _, envVar := range container.Env {
			require.NotEqual(t, "TRUSTSTORE_PASSWORD", envVar.Name)
			if envVar.Name == "JAVA_TOOL_OPTIONS" {
				javaOptions = envVar.Value
			}
		}
		require.Contains(t, javaOptions, "-Djavax.net.ssl.trustStore="+mountDirectory+"/"+secretKey)
		require.NotContains(t, javaOptions, "-Djavax.net.ssl.trustStorePassword=")
	}
	require.Equal(t, expectedMounts, mounts)
}

func verifyOrchestrationTLSCustomKey(secretName, secretKey string) func(t *testing.T, output string, err error) {
	return func(t *testing.T, output string, err error) {
		require.NoError(t, err)

		var statefulSet appsv1.StatefulSet
		helm.UnmarshalK8SYaml(t, output, &statefulSet)
		requireTLSCustomKey(t, statefulSet.Spec.Template.Spec, secretName, secretKey, "/usr/local/camunda/certificates", 1)
	}
}

func verifyOptimizeTLSCustomKey(secretName, secretKey string) func(t *testing.T, output string, err error) {
	return func(t *testing.T, output string, err error) {
		require.NoError(t, err)

		var deployment appsv1.Deployment
		helm.UnmarshalK8SYaml(t, output, &deployment)
		requireTLSCustomKey(t, deployment.Spec.Template.Spec, secretName, secretKey, "/optimize/certificates", 2)
	}
}

func (s *tlsSecretsTest) TestComponentDatastoreTLSCustomKey() {
	testCases := []testhelpers.TestCase{
		{
			Name:     "Orchestration Elasticsearch uses component TLS custom key",
			Template: "templates/orchestration/statefulset.yaml",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type":                                       "elasticsearch",
				"orchestration.data.secondaryStorage.elasticsearch.tls.secret.existingSecret":    "orchestration-elasticsearch-tls",
				"orchestration.data.secondaryStorage.elasticsearch.tls.secret.existingSecretKey": "orchestration-elasticsearch.jks",
			},
			Verifier: verifyOrchestrationTLSCustomKey("orchestration-elasticsearch-tls", "orchestration-elasticsearch.jks"),
		},
		{
			Name:     "Orchestration OpenSearch uses component TLS custom key",
			Template: "templates/orchestration/statefulset.yaml",
			Values: map[string]string{
				"orchestration.data.secondaryStorage.type":                                    "opensearch",
				"orchestration.data.secondaryStorage.opensearch.tls.secret.existingSecret":    "orchestration-opensearch-tls",
				"orchestration.data.secondaryStorage.opensearch.tls.secret.existingSecretKey": "orchestration-opensearch.jks",
			},
			Verifier: verifyOrchestrationTLSCustomKey("orchestration-opensearch-tls", "orchestration-opensearch.jks"),
		},
		{
			Name:     "Optimize Elasticsearch uses component TLS custom key",
			Template: "templates/optimize/deployment.yaml",
			Values: map[string]string{
				"identity.enabled":                                             "true",
				"global.identity.auth.enabled":                                 "true",
				"optimize.enabled":                                             "true",
				"optimize.database.elasticsearch.enabled":                      "true",
				"optimize.database.elasticsearch.external":                     "true",
				"optimize.database.elasticsearch.tls.enabled":                  "true",
				"optimize.database.elasticsearch.tls.secret.existingSecret":    "optimize-elasticsearch-tls",
				"optimize.database.elasticsearch.tls.secret.existingSecretKey": "optimize-elasticsearch.jks",
				"optimize.database.opensearch.enabled":                         "false",
			},
			Verifier: verifyOptimizeTLSCustomKey("optimize-elasticsearch-tls", "optimize-elasticsearch.jks"),
		},
		{
			Name:     "Optimize OpenSearch uses component TLS custom key",
			Template: "templates/optimize/deployment.yaml",
			Values: map[string]string{
				"identity.enabled":                                          "true",
				"global.identity.auth.enabled":                              "true",
				"optimize.enabled":                                          "true",
				"optimize.database.elasticsearch.enabled":                   "false",
				"optimize.database.opensearch.enabled":                      "true",
				"optimize.database.opensearch.tls.enabled":                  "true",
				"optimize.database.opensearch.tls.secret.existingSecret":    "optimize-opensearch-tls",
				"optimize.database.opensearch.tls.secret.existingSecretKey": "optimize-opensearch.jks",
			},
			Verifier: verifyOptimizeTLSCustomKey("optimize-opensearch-tls", "optimize-opensearch.jks"),
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}
