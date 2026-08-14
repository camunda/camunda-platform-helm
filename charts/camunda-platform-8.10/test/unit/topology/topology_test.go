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
	_ "camunda-platform/test/unit/utils"
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

func TestHubTopologyRendersRemoteIdentityPresetsAndHubInventory(t *testing.T) {
	output := render(t, "hub-generic.yaml",
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
	require.Contains(t, output, `authentication: "BEARER_TOKEN"`)
	require.Contains(t, output, `grpc://camunda-zeebe-gateway.camunda-east.svc.cluster.local:26500`)
	require.Contains(t, output, `grpc://camunda-west-zeebe-gateway.camunda-west.svc.cluster.local:26500`)
	require.NotContains(t, output, `keycloak:\n`)
}

func TestHubTopologyOptimizeRedirectUrisIncludesRoot(t *testing.T) {
	valuesFile := filepath.Join("testdata", "hub-keycloak.yaml")
	options := &helm.Options{
		ValuesFiles: []string{valuesFile},
		SetValues: map[string]string{
			"global.topology.clusters[0].components.optimize.enabled":                  "true",
			"global.topology.clusters[0].components.optimize.clientId":                 "optimize-east",
			"global.topology.clusters[0].components.optimize.audience":                 "optimize-east-api",
			"global.topology.clusters[0].components.optimize.redirectUrl":              "https://east.example.com/optimize",
			"global.topology.clusters[0].components.optimize.secret.existingSecret":    "east-oidc",
			"global.topology.clusters[0].components.optimize.secret.existingSecretKey": "optimize-secret",
		},
	}

	output := helm.RenderTemplate(t, options, chartPath(t), "camunda", []string{"templates/identity/configmap.yaml"})

	require.Regexp(t, `redirect-uris:\s*\n\s*-\s*"/api/authentication/callback"\s*\n\s*-\s*"/"\s*\n`, output)
}

func TestHubTopologySuppressesDefaultWorkloadPlane(t *testing.T) {
	output := render(t, "hub-generic.yaml")

	require.Contains(t, output, "name: camunda-identity")
	require.NotContains(t, output, "name: camunda-zeebe")
	require.NotContains(t, output, "name: camunda-connectors")
}

func TestHubTopologyKeycloakRendersInitAndSecretReferences(t *testing.T) {
	output := render(t, "hub-keycloak.yaml",
		"templates/identity/configmap.yaml",
		"templates/identity/deployment.yaml",
	)

	require.Contains(t, output, `"topology-east": {}`)
	require.Contains(t, output, "VALUES_TOPOLOGY_EAST_ORCHESTRATION_SECRET")
	require.Contains(t, output, "name: east-oidc")
	require.Contains(t, output, "key: orchestration-secret")
}

func TestHubTopologyKeycloakRendersHubPingAudience(t *testing.T) {
	valuesFile := filepath.Join("testdata", "hub-keycloak.yaml")
	options := &helm.Options{
		ValuesFiles: []string{valuesFile},
		SetValues: map[string]string{
			"global.identity.auth.orchestration.hubPingAuthorizationEnabled": "true",
		},
	}

	output := helm.RenderTemplate(t, options, chartPath(t), "camunda", []string{"templates/identity/configmap.yaml"})
	require.Contains(t, output, `audience: "web-modeler-public-api"`)
	require.Contains(t, output, `definition: create:*`)
	require.Contains(t, output, `definition: update:*`)
	require.Contains(t, output, `id: "orchestration-east"`)
	require.NotContains(t, output, `id: ${CAMUNDA_ORCHESTRATION_CLIENT_ID:`)
}

func TestHubTopologyPreservesLegacyAlwaysRegister(t *testing.T) {
	valuesFile := filepath.Join("testdata", "hub-keycloak.yaml")
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

func TestHubTopologyRejectsLegacyRegistrationCollision(t *testing.T) {
	valuesFile := filepath.Join("testdata", "hub-keycloak.yaml")
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

func TestHubTopologyRejectsReservedSharedRoleName(t *testing.T) {
	valuesFile := filepath.Join("testdata", "hub-generic.yaml")
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
	require.Contains(t, output, `CAMUNDA_IDENTITY_BASEURL: "http://camunda-identity.camunda-hub.svc.cluster.local:80/identity"`)

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

func TestHubTopologyRejectsDuplicateClientIds(t *testing.T) {
	valuesFile := filepath.Join("testdata", "hub-generic.yaml")
	options := &helm.Options{
		ValuesFiles: []string{valuesFile},
		SetValues: map[string]string{
			"global.topology.clusters[1].components.orchestration.clientId": "orchestration-east",
		},
	}

	_, err := helm.RenderTemplateE(t, options, chartPath(t), "camunda", []string{"templates/identity/configmap.yaml"})
	require.ErrorContains(t, err, `duplicate topology client or audience id "orchestration-east"`)
}

func TestHubTopologyRejectsAdminClientCollision(t *testing.T) {
	valuesFile := filepath.Join("testdata", "hub-generic.yaml")
	options := &helm.Options{
		ValuesFiles: []string{valuesFile},
		SetValues: map[string]string{
			"global.identity.auth.admin.enabled":  "true",
			"global.identity.auth.admin.clientId": "orchestration-east",
		},
	}

	_, err := helm.RenderTemplateE(t, options, chartPath(t), "camunda", []string{"templates/identity/configmap.yaml"})
	require.ErrorContains(t, err, `duplicate topology client or audience id "orchestration-east"`)
}

func TestHubTopologyRejectsCustomClientCollision(t *testing.T) {
	valuesFile := filepath.Join("testdata", "hub-generic.yaml")
	options := &helm.Options{
		ValuesFiles: []string{valuesFile},
		SetValues: map[string]string{
			"identity.clients[0].id":                  "orchestration-east",
			"identity.clients[0].name":                "Duplicate",
			"identity.clients[0].type":                "public",
			"identity.clients[0].redirectUris":        "/dummy",
			"identity.clients[0].rootUrl":             "http://dummy",
			"identity.clients[0].secret.inlineSecret": "unused",
		},
	}

	_, err := helm.RenderTemplateE(t, options, chartPath(t), "camunda", []string{"templates/identity/configmap.yaml"})
	require.ErrorContains(t, err, `duplicate topology client or audience id "orchestration-east"`)
}

func TestHubTopologyRejectsReservedHubClusterId(t *testing.T) {
	valuesFile := filepath.Join("testdata", "hub-generic.yaml")
	options := &helm.Options{
		ValuesFiles: []string{valuesFile},
		SetValues: map[string]string{
			"global.topology.clusters[0].id": "management-cluster",
		},
	}

	_, err := helm.RenderTemplateE(t, options, chartPath(t), "camunda", []string{"templates/web-modeler/configmap-restapi.yaml"})
	require.ErrorContains(t, err, `normalize to the same key "management-cluster"`)
}

func TestHubTopologyRequiresOIDC(t *testing.T) {
	valuesFile := filepath.Join("testdata", "hub-generic.yaml")
	options := &helm.Options{
		ValuesFiles: []string{valuesFile},
		SetValues: map[string]string{
			"global.security.authentication.method": "basic",
		},
	}

	_, err := helm.RenderTemplateE(t, options, chartPath(t), "camunda", []string{"templates/web-modeler/configmap-restapi.yaml"})
	require.ErrorContains(t, err, "global.topology.mode=hub requires OIDC authentication")
}

func TestHubTopologyUsesHubEndpointOverrides(t *testing.T) {
	valuesFile := filepath.Join("testdata", "hub-generic.yaml")
	options := &helm.Options{
		ValuesFiles: []string{valuesFile},
		SetValues: map[string]string{
			"global.topology.clusters[0].components.orchestration.grpcUrl":      "grpcs://grpc.example.com:443",
			"global.topology.clusters[0].components.orchestration.restUrl":      "https://api.example.com/orchestration",
			"global.topology.clusters[0].components.orchestration.readinessUrl": "https://health.example.com/orchestration",
			"global.topology.clusters[0].components.orchestration.operateUrl":   "https://apps.example.com/operate",
			"global.topology.clusters[0].components.orchestration.tasklistUrl":  "https://apps.example.com/tasklist",
			"global.topology.clusters[0].components.orchestration.adminUrl":     "https://apps.example.com/admin",
			"global.topology.clusters[0].components.optimize.webappUrl":         "https://apps.example.com/optimize",
			"global.topology.clusters[0].components.optimize.readinessUrl":      "https://health.example.com/optimize",
			"global.topology.clusters[0].components.connectors.restUrl":         "https://api.example.com/connectors",
			"global.topology.clusters[0].components.connectors.readinessUrl":    "https://health.example.com/connectors",
		},
	}

	output := helm.RenderTemplate(t, options, chartPath(t), "camunda", []string{"templates/web-modeler/configmap-restapi.yaml"})
	for _, endpoint := range []string{
		"grpcs://grpc.example.com:443",
		"https://api.example.com/orchestration",
		"https://health.example.com/orchestration",
		"https://apps.example.com/operate",
		"https://apps.example.com/tasklist",
		"https://apps.example.com/admin",
		"https://apps.example.com/optimize",
		"https://health.example.com/optimize",
		"https://api.example.com/connectors",
		"https://health.example.com/connectors",
	} {
		require.Contains(t, output, endpoint)
	}
}

func TestHubTopologyKeycloakRejectsMissingSecret(t *testing.T) {
	valuesFile := filepath.Join("testdata", "hub-keycloak.yaml")
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
			"global.identity.service.url": "https://hub.example.com/identity",
		},
	}

	output := helm.RenderTemplate(t, options, chartPath(t), "camunda", []string{"templates/common/configmap-identity-auth.yaml"})
	require.Contains(t, output, `CAMUNDA_IDENTITY_BASEURL: "https://hub.example.com/identity"`)
}

func TestTopologyPreservesSuppressedPersistentVolumeClaims(t *testing.T) {
	hubOptions := &helm.Options{
		ValuesFiles: []string{filepath.Join("testdata", "hub-generic.yaml")},
		SetValues: map[string]string{
			"connectors.persistence.enabled": "true",
			"optimize.persistence.enabled":   "true",
		},
	}
	hubTemplates := []string{
		"templates/connectors/persistentvolumeclaim.yaml",
		"templates/optimize/persistentvolumeclaim.yaml",
	}
	for _, args := range [][]string{nil, {"--is-upgrade"}} {
		hubOutput := helm.RenderTemplate(t, hubOptions, chartPath(t), "camunda", hubTemplates, args...)
		require.Contains(t, hubOutput, "name: camunda-camunda-platform-connectors-data")
		require.Contains(t, hubOutput, "name: camunda-camunda-platform-optimize-data")
	}

	orchestrationOptions := &helm.Options{
		ValuesFiles: []string{filepath.Join("testdata", "orchestration.yaml")},
		SetValues: map[string]string{
			"identity.persistence.enabled": "true",
		},
	}
	for _, args := range [][]string{nil, {"--is-upgrade"}} {
		orchestrationOutput := helm.RenderTemplate(t, orchestrationOptions, chartPath(t), "camunda", []string{
			"templates/identity/persistentvolumeclaim.yaml",
		}, args...)
		require.Contains(t, orchestrationOutput, "name: camunda-camunda-platform-identity-data")
	}
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
