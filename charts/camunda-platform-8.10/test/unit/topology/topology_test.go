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

package topology

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

func chartPath(t *testing.T) string {
	t.Helper()
	path, err := filepath.Abs("../../../")
	require.NoError(t, err)
	return path
}

func render(t *testing.T, valuesFile string, templates ...string) string {
	t.Helper()
	options := &helm.Options{ValuesFiles: []string{filepath.Join("testdata", valuesFile)}}
	return helm.RenderTemplate(t, options, chartPath(t), "camunda", templates)
}

func TestManagementTopologyRendersRemoteIdentityPresetsAndHubInventory(t *testing.T) {
	output := render(t, "management-generic.yaml",
		"templates/identity/configmap.yaml",
		"templates/web-modeler/configmap-restapi.yaml",
	)

	require.Contains(t, output, `"topology-east":`)
	require.Contains(t, output, `"topology-west":`)
	require.Contains(t, output, `"topology-shared-roles":`)
	require.Contains(t, output, `name: "Orchestration"`)
	require.Contains(t, output, `audience: "orchestration-east-api"`)
	require.Contains(t, output, `audience: "orchestration-west-api"`)
	require.NotContains(t, output, `audience: "web-modeler-api"\n                  definition: create:*`)
	require.NotContains(t, output, `audience: "web-modeler-api"\n                  definition: update:*`)
	require.Contains(t, output, `id: "east"`)
	require.Contains(t, output, `id: "west"`)
	require.Contains(t, output, `grpc://camunda-zeebe-gateway.camunda-east.svc.cluster.local:26500`)
	require.Contains(t, output, `grpc://camunda-west-zeebe-gateway.camunda-west.svc.cluster.local:26500`)
	require.NotContains(t, output, `keycloak:\n`)
}

func TestManagementTopologySuppressesDefaultWorkloadPlane(t *testing.T) {
	output := render(t, "management-generic.yaml")

	require.Contains(t, output, "name: camunda-identity")
	require.NotContains(t, output, "name: camunda-zeebe")
	require.NotContains(t, output, "name: camunda-connectors")
}

func TestManagementTopologyKeycloakRendersInitAndSecretReferences(t *testing.T) {
	output := render(t, "management-keycloak.yaml",
		"templates/identity/configmap.yaml",
		"templates/identity/deployment.yaml",
	)

	require.Contains(t, output, `"topology-east": {}`)
	require.Contains(t, output, "VALUES_TOPOLOGY_EAST_ORCHESTRATION_SECRET")
	require.Contains(t, output, "name: east-oidc")
	require.Contains(t, output, "key: orchestration-secret")
}

func TestManagementTopologyPreservesLegacyAlwaysRegister(t *testing.T) {
	valuesFile := filepath.Join("testdata", "management-keycloak.yaml")
	options := &helm.Options{
		ValuesFiles: []string{valuesFile},
		SetValues: map[string]string{
			"global.identity.auth.connectors.alwaysRegister":    "true",
			"global.identity.auth.optimize.alwaysRegister":      "true",
			"global.identity.auth.orchestration.alwaysRegister": "true",
		},
	}

	output := helm.RenderTemplate(t, options, chartPath(t), "camunda", []string{
		"templates/identity/configmap.yaml",
		"templates/identity/deployment.yaml",
	})
	require.Contains(t, output, "VALUES_KEYCLOAK_INIT_CONNECTORS_SECRET")
	require.Contains(t, output, "VALUES_KEYCLOAK_INIT_OPTIMIZE_SECRET")
	require.Contains(t, output, "VALUES_KEYCLOAK_INIT_ORCHESTRATION_SECRET")
}

func TestManagementTopologyRejectsLegacyRegistrationCollision(t *testing.T) {
	valuesFile := filepath.Join("testdata", "management-keycloak.yaml")
	options := &helm.Options{
		ValuesFiles: []string{valuesFile},
		SetValues: map[string]string{
			"global.identity.auth.orchestration.alwaysRegister":             "true",
			"global.topology.clusters[0].components.orchestration.clientId": "orchestration",
		},
	}

	_, err := helm.RenderTemplateE(t, options, chartPath(t), "camunda", []string{"templates/identity/configmap.yaml"})
	require.ErrorContains(t, err, `duplicate topology client or audience id "orchestration"`)
}

func TestManagementTopologyRejectsReservedSharedRoleName(t *testing.T) {
	valuesFile := filepath.Join("testdata", "management-generic.yaml")
	options := &helm.Options{
		ValuesFiles: []string{valuesFile},
		SetValues: map[string]string{
			"global.topology.clusters[0].components.orchestration.roleName": "Orchestration",
		},
	}

	_, err := helm.RenderTemplateE(t, options, chartPath(t), "camunda", []string{"templates/identity/configmap.yaml"})
	require.ErrorContains(t, err, `duplicate or reserved topology role name "Orchestration"`)
}

func TestOrchestrationTopologyUsesExistingComponentAndIdentityConfiguration(t *testing.T) {
	output := render(t, "orchestration.yaml",
		"templates/orchestration/configmap.yaml",
		"templates/orchestration/statefulset.yaml",
		"templates/optimize/deployment.yaml",
		"templates/connectors/deployment.yaml",
		"templates/common/configmap-identity-auth.yaml",
	)

	require.Contains(t, output, `client-id: "orchestration-east"`)
	require.Contains(t, output, `- "orchestration-east-api"`)
	require.Contains(t, output, `redirect-uri: "https://east.example.com/orchestration/sso-callback"`)
	require.Contains(t, output, `CAMUNDA_IDENTITY_BASEURL: "http://camunda-identity.camunda-management.svc.cluster.local:80/identity"`)

	var statefulSet appsv1.StatefulSet
	for _, document := range splitDocuments(output) {
		if !contains(document, "kind: StatefulSet") {
			continue
		}
		helm.UnmarshalK8SYaml(t, document, &statefulSet)
	}
	require.NotEmpty(t, statefulSet.Name)
	require.Contains(t, statefulSet.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{
		Name: "VALUES_ORCHESTRATION_CLIENT_SECRET",
		ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: "east-oidc"},
			Key:                  "orchestration-secret",
		}},
	})
	require.Contains(t, output, "name: CAMUNDA_IDENTITY_CLIENT_SECRET")
	require.Contains(t, output, "name: east-oidc")
	require.Contains(t, output, "key: optimize-secret")
	require.Contains(t, output, "key: connectors-secret")
}

func TestManagementTopologyRejectsDuplicateClientIds(t *testing.T) {
	valuesFile := filepath.Join("testdata", "management-generic.yaml")
	options := &helm.Options{
		ValuesFiles: []string{valuesFile},
		SetValues: map[string]string{
			"global.topology.clusters[1].components.orchestration.clientId": "orchestration-east",
		},
	}

	_, err := helm.RenderTemplateE(t, options, chartPath(t), "camunda", []string{"templates/identity/configmap.yaml"})
	require.ErrorContains(t, err, `duplicate topology client or audience id "orchestration-east"`)
}

func TestManagementTopologyKeycloakRejectsMissingSecret(t *testing.T) {
	valuesFile := filepath.Join("testdata", "management-keycloak.yaml")
	options := &helm.Options{
		ValuesFiles: []string{valuesFile},
		SetValues: map[string]string{
			"global.topology.clusters[0].components.orchestration.secret.existingSecret":    "",
			"global.topology.clusters[0].components.orchestration.secret.existingSecretKey": "",
		},
	}

	_, err := helm.RenderTemplateE(t, options, chartPath(t), "camunda", []string{"templates/identity/configmap.yaml"})
	require.ErrorContains(t, err, "requires a complete secret configuration when Management Identity administers Keycloak")
}

func TestOrchestrationTopologyRejectsEnabledIdentity(t *testing.T) {
	valuesFile := filepath.Join("testdata", "orchestration.yaml")
	options := &helm.Options{
		ValuesFiles: []string{valuesFile},
		SetValues: map[string]string{
			"identity.enabled": "true",
		},
	}

	_, err := helm.RenderTemplateE(t, options, chartPath(t), "camunda", []string{"templates/orchestration/configmap.yaml"})
	require.ErrorContains(t, err, "global.topology.mode=orchestration requires identity.enabled=false")
}

func TestOrchestrationTopologyRequiresManagementIdentityURL(t *testing.T) {
	valuesFile := filepath.Join("testdata", "orchestration.yaml")
	options := &helm.Options{
		ValuesFiles: []string{valuesFile},
		SetValues: map[string]string{
			"global.identity.service.url": "",
		},
	}

	_, err := helm.RenderTemplateE(t, options, chartPath(t), "camunda", []string{"templates/orchestration/configmap.yaml"})
	require.ErrorContains(t, err, "global.topology.mode=orchestration requires global.identity.service.url")
}

func TestOrchestrationTopologyUsesGlobalIdentityServiceURL(t *testing.T) {
	valuesFile := filepath.Join("testdata", "orchestration.yaml")
	options := &helm.Options{
		ValuesFiles: []string{valuesFile},
		SetValues: map[string]string{
			"global.identity.service.url": "https://management.example.com/identity",
		},
	}

	output := helm.RenderTemplate(t, options, chartPath(t), "camunda", []string{"templates/common/configmap-identity-auth.yaml"})
	require.Contains(t, output, `CAMUNDA_IDENTITY_BASEURL: "https://management.example.com/identity"`)
}

func TestTopologyUpgradePreservesSuppressedPersistentVolumeClaims(t *testing.T) {
	managementOptions := &helm.Options{
		ValuesFiles: []string{filepath.Join("testdata", "management-generic.yaml")},
		SetValues: map[string]string{
			"connectors.persistence.enabled": "true",
			"optimize.persistence.enabled":   "true",
		},
	}
	managementOutput := helm.RenderTemplate(t, managementOptions, chartPath(t), "camunda", []string{
		"templates/connectors/persistentvolumeclaim.yaml",
		"templates/optimize/persistentvolumeclaim.yaml",
	}, "--is-upgrade")
	require.Contains(t, managementOutput, "name: camunda-camunda-platform-connectors-data")
	require.Contains(t, managementOutput, "name: camunda-camunda-platform-optimize-data")

	orchestrationOptions := &helm.Options{
		ValuesFiles: []string{filepath.Join("testdata", "orchestration.yaml")},
		SetValues: map[string]string{
			"identity.persistence.enabled": "true",
		},
	}
	orchestrationOutput := helm.RenderTemplate(t, orchestrationOptions, chartPath(t), "camunda", []string{
		"templates/identity/persistentvolumeclaim.yaml",
	}, "--is-upgrade")
	require.Contains(t, orchestrationOutput, "name: camunda-camunda-platform-identity-data")
}

func TestNullTopologyPreservesCombinedMode(t *testing.T) {
	options := &helm.Options{SetValues: map[string]string{
		"global.topology":                                       "null",
		"orchestration.data.secondaryStorage.type":              "elasticsearch",
		"orchestration.data.secondaryStorage.elasticsearch.url": "http://elasticsearch:9200",
	}}

	output := helm.RenderTemplate(t, options, chartPath(t), "camunda", []string{"templates/orchestration/configmap.yaml"})
	require.Contains(t, output, "kind: ConfigMap")
}

func splitDocuments(output string) []string {
	return strings.Split(output, "\n---\n")
}

func contains(value, substring string) bool {
	return strings.Contains(value, substring)
}
