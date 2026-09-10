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

package optimize

import (
	"camunda-platform/test/unit/utils"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func TestGoldenDefaultsTemplateOptimize(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)
	templateNames := []string{"service", "serviceaccount", "deployment", "configmap"}

	for _, name := range templateNames {
		suite.Run(t, &utils.TemplateGoldenTest{
			ChartPath:      chartPath,
			Release:        "camunda-platform-test",
			Namespace:      "camunda-platform-" + strings.ToLower(random.UniqueId()),
			GoldenFileName: name,
			Templates:      []string{"templates/optimize/" + name + ".yaml"},
			IgnoredLines: []string{
				`\s+.*-secret:\s+.*`,    // secrets are auto-generated and need to be ignored.
				`\s+checksum/.+?:\s+.*`, // ignore configmap checksum.
			},
			SetValues: map[string]string{
				"optimize.enabled":                        "true",
				"optimize.database.elasticsearch.enabled": "true",
				"identity.enabled":                        "true",
			},
		})
	}
}

// The component-scoped identity ConfigMap only renders once Optimize owns its identity, so the
// default suite above never reaches it. Cover its full rendered shape under a values set that
// satisfies optimize.needsIdentityConfigMap. Identity stays out of this release: it registers
// clients only under global.identity.auth.enabled, so a local Identity alongside component-scoped
// Optimize OIDC is the combination constraints.tpl rejects, and the ConfigMap this golden covers
// exists for the release that points at a Management Identity elsewhere.
func TestGoldenIdentityEnvTemplateOptimize(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &utils.TemplateGoldenTest{
		ChartPath:      chartPath,
		Release:        "camunda-platform-test",
		Namespace:      "camunda-platform-" + strings.ToLower(random.UniqueId()),
		GoldenFileName: "configmap-identity-env",
		Templates:      []string{"templates/optimize/configmap-identity-env.yaml"},
		SetValues: map[string]string{
			"optimize.enabled":                                               "true",
			"optimize.database.elasticsearch.enabled":                        "true",
			"optimize.security.authentication.method":                        "oidc",
			"optimize.security.authentication.oidc.type":                     "KEYCLOAK",
			"optimize.security.authentication.oidc.issuer":                   "https://tenant-issuer.example.com/realms/camunda",
			"optimize.security.authentication.oidc.issuerBackendUrl":         "http://keycloak.tenant.svc:8080/realms/camunda",
			"optimize.security.authentication.oidc.secret.existingSecret":    "tenant-oidc",
			"optimize.security.authentication.oidc.secret.existingSecretKey": "client-secret",
			"optimize.identity.service.url":                                  "http://identity.tenant.svc/identity",
		},
	})
}
