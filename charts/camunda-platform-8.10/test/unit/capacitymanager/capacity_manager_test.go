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

package capacitymanager

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
)

func chartPath(t *testing.T) string {
	t.Helper()
	path, err := filepath.Abs("../../../")
	require.NoError(t, err)
	return path
}

func render(t *testing.T, template string, values map[string]string) string {
	t.Helper()
	if values == nil {
		values = map[string]string{}
	}
	values["global.elasticsearch.enabled"] = "true"
	return helm.RenderTemplate(t, &helm.Options{
		SetValues:   values,
		ValuesFiles: []string{filepath.Join(chartPath(t), "test/unit/common/testdata/values-capacity-manager.yaml")},
	}, chartPath(t), "capacity-test", []string{template})
}

func TestCapacityManagerDisabledByDefault(t *testing.T) {
	output := helm.RenderTemplate(t, &helm.Options{SetValues: map[string]string{"global.elasticsearch.enabled": "true"}}, chartPath(t), "capacity-test", nil)
	require.NotContains(t, strings.TrimSpace(output), "Source: camunda-platform/templates/capacity-manager/")
}

func TestCapacityManagerDeployment(t *testing.T) {
	output := render(t, "templates/capacity-manager/deployment.yaml", map[string]string{
		"capacityManager.enabled":          "true",
		"capacityManager.image.registry":   "registry.example.com",
		"capacityManager.image.repository": "team/capacity-manager",
		"capacityManager.image.tag":        "test",
		"capacityManager.image.digest":     "",
	})
	var deployment appsv1.Deployment
	helm.UnmarshalK8SYaml(t, output, &deployment)

	require.Equal(t, "registry.example.com/team/capacity-manager:test", deployment.Spec.Template.Spec.Containers[0].Image)
	require.Equal(t, "capacity-test-capacity-manager", deployment.Spec.Template.Spec.ServiceAccountName)
	require.True(t, *deployment.Spec.Template.Spec.Containers[0].SecurityContext.ReadOnlyRootFilesystem)
	require.Contains(t, deployment.Spec.Template.Spec.Containers[0].Args, "--statefulset=capacity-test-zeebe")
	require.Contains(t, deployment.Spec.Template.Spec.Containers[0].Args, "--zeebe-url=http://capacity-test-zeebe-0.capacity-test-zeebe:9600")

	output = render(t, "templates/capacity-manager/deployment.yaml", map[string]string{
		"capacityManager.enabled":   "true",
		"orchestration.contextPath": "/orchestration",
	})
	helm.UnmarshalK8SYaml(t, output, &deployment)
	require.Contains(t, deployment.Spec.Template.Spec.Containers[0].Args, "--zeebe-url=http://capacity-test-zeebe-0.capacity-test-zeebe:9600/orchestration")
}

func TestCapacityManagerPolicy(t *testing.T) {
	output := render(t, "templates/capacity-manager/configmap.yaml", map[string]string{
		"capacityManager.enabled": "true",
	})
	var configMap corev1.ConfigMap
	helm.UnmarshalK8SYaml(t, output, &configMap)
	var policy map[string]any
	require.NoError(t, json.Unmarshal([]byte(configMap.Data["policy.json"]), &policy))
	require.Equal(t, "recommend", policy["mode"])
	require.Equal(t, float64(1), policy["minBrokers"])
}

func TestCapacityManagerRBAC(t *testing.T) {
	values := map[string]string{"capacityManager.enabled": "true", "capacityManager.replicaOwnership": "capacityManager"}
	var role rbacv1.Role
	helm.UnmarshalK8SYaml(t, render(t, "templates/capacity-manager/role.yaml", values), &role)
	require.Equal(t, []string{"statefulsets"}, role.Rules[0].Resources)
	require.Equal(t, []string{"capacity-test-zeebe"}, role.Rules[0].ResourceNames)
	require.Equal(t, []string{"get", "patch", "update"}, role.Rules[0].Verbs)

	var binding rbacv1.RoleBinding
	helm.UnmarshalK8SYaml(t, render(t, "templates/capacity-manager/rolebinding.yaml", values), &binding)
	require.Equal(t, "capacity-test-capacity-manager", binding.Subjects[0].Name)
	require.Equal(t, "default", binding.Subjects[0].Namespace)
}

func TestCapacityManagerReadOnlyRBAC(t *testing.T) {
	values := map[string]string{"capacityManager.enabled": "true"}
	var role rbacv1.Role
	helm.UnmarshalK8SYaml(t, render(t, "templates/capacity-manager/role.yaml", values), &role)
	require.Equal(t, []string{"get"}, role.Rules[0].Verbs)
}
