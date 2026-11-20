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
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/stretchr/testify/require"
)

func renderCapacityManagerTenantGuard(t *testing.T, values map[string]string) error {
	t.Helper()
	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)
	values["orchestration.data.secondaryStorage.type"] = "elasticsearch"
	values["global.identity.auth.enabled"] = "true"
	values["global.identity.auth.issuer"] = "https://issuer.example.com"
	values["global.identity.auth.issuerBackendUrl"] = "https://issuer.example.com"
	values["orchestration.security.authentication.method"] = "oidc"
	if _, ok := values["capacityManager.enabled"]; !ok {
		values["capacityManager.enabled"] = "true"
	}
	jsonValues := map[string]string{}
	for key := range values {
		if strings.HasPrefix(key, "global.topology.clusters[") && strings.HasSuffix(key, "].id") {
			jsonValues[strings.TrimSuffix(key, ".id")+".components"] = "{}"
		}
	}
	_, err = helm.RenderTemplateE(t, &helm.Options{
		SetValues:     values,
		SetJsonValues: jsonValues,
	}, chartPath, "capacity-test", []string{"templates/orchestration/configmap.yaml"})
	return err
}

func capacityManagerTopologyTenants(ids ...string) map[string]string {
	values := map[string]string{
		"global.topology.clusters[0].id":          "east",
		"global.topology.clusters[0].namespace":   "camunda-east",
		"global.topology.clusters[0].releaseName": "camunda",
		"global.topology.clusters[0].host":        "east.example.com",
		"global.topology.clusters[0].version":     "8.11.0",
	}
	for i, id := range ids {
		values[fmt.Sprintf("global.topology.clusters[0].physicalTenants[%d].id", i)] = id
	}
	return values
}

func capacityManagerExtraConfigTenants(file, content string) map[string]string {
	return map[string]string{
		"orchestration.extraConfiguration[0].file":    file,
		"orchestration.extraConfiguration[0].content": content,
	}
}

func TestCapacityManagerPhysicalTenantGuard(t *testing.T) {
	const blocked = "capacityManager does not yet support physical tenants other than \"default\""
	cases := []struct {
		name    string
		values  map[string]string
		blocked string
	}{
		{name: "NoTenants", values: map[string]string{}},
		{name: "DefaultOnlyExtraConfiguration", values: capacityManagerExtraConfigTenants("tenants.yaml", "camunda:\n  physical-tenants:\n    default:\n      name: Default\n")},
		{name: "DefaultOnlySecretStore", values: map[string]string{
			"orchestration.secretStore.aws.default.region":                         "us-east-1",
			"orchestration.secretStore.physicalTenants.default.aws.default.region": "us-west-2",
		}},
		{name: "DefaultOnlyTopology", values: capacityManagerTopologyTenants("default")},
		{name: "DefaultAndOtherExtraConfiguration", values: capacityManagerExtraConfigTenants("tenants.yaml", "camunda:\n  physical-tenants:\n    default:\n      name: Default\n    tenanta:\n      name: A\n"), blocked: "declared: tenanta"},
		{name: "DefaultAndOtherDottedExtraConfiguration", values: capacityManagerExtraConfigTenants("tenants.yaml", "camunda.physical-tenants.default.name: Default\ncamunda.physical-tenants.tenanta.name: A\n"), blocked: "declared: tenanta"},
		{name: "DefaultAndOtherSecretStore", values: map[string]string{
			"orchestration.secretStore.aws.default.region":                         "us-east-1",
			"orchestration.secretStore.physicalTenants.default.aws.default.region": "us-west-2",
			"orchestration.secretStore.physicalTenants.tenanta.aws.default.region": "us-west-2",
		}, blocked: "declared: tenanta"},
		{name: "DefaultAndOtherTopology", values: capacityManagerTopologyTenants("default", "tenanta"), blocked: "declared: tenanta"},
		{name: "NonDefaultOnlyExtraConfiguration", values: capacityManagerExtraConfigTenants("tenants.yaml", "camunda:\n  physical-tenants:\n    tenanta:\n      name: A\n"), blocked: "declared: tenanta"},
		{name: "NonDefaultOnlyProperties", values: capacityManagerExtraConfigTenants("tenants.properties", "camunda.physical-tenants.tenanta.name=A\n"), blocked: "declared: tenanta"},
		{name: "NonDefaultOnlySecretStore", values: map[string]string{
			"orchestration.secretStore.aws.default.region":                         "us-east-1",
			"orchestration.secretStore.physicalTenants.tenanta.aws.default.region": "us-west-2",
		}, blocked: "declared: tenanta"},
		{name: "NonDefaultOnlyTopology", values: capacityManagerTopologyTenants("tenanta"), blocked: "declared: tenanta"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := renderCapacityManagerTenantGuard(t, tc.values)
			if tc.blocked == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.Contains(t, err.Error(), blocked)
			require.Contains(t, err.Error(), tc.blocked)
		})
	}
}

func TestCapacityManagerPhysicalTenantGuardInertWhenDisabled(t *testing.T) {
	cases := map[string]map[string]string{
		"SecretStore": {
			"orchestration.secretStore.aws.default.region":                         "us-east-1",
			"orchestration.secretStore.physicalTenants.tenanta.aws.default.region": "us-west-2",
		},
		"Topology": capacityManagerTopologyTenants("tenanta"),
	}
	for name, values := range cases {
		t.Run(name, func(t *testing.T) {
			values["capacityManager.enabled"] = "false"
			require.NoError(t, renderCapacityManagerTenantGuard(t, values))
		})
	}
}

func TestCapacityManagerPhysicalTenantGuardInertWithoutCapacityManagerValues(t *testing.T) {
	values := capacityManagerTopologyTenants("tenanta")
	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)
	values["orchestration.data.secondaryStorage.type"] = "elasticsearch"
	_, err = helm.RenderTemplateE(t, &helm.Options{
		SetValues:     values,
		SetJsonValues: map[string]string{"global.topology.clusters[0].components": "{}", "global.topology.clusters[0].physicalTenants[0].components": "{}"},
	}, chartPath, "capacity-test", []string{"templates/orchestration/configmap.yaml"})
	require.NoError(t, err)
}
