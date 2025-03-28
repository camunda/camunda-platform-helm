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

// NOTE: This is test should be part of Identity package, but it's added here because Helm 3 (v3.10.x)
// 			 still doesn't have "export-values" option to share data between the parent (Identity) and sub-chart (Keycloak).
//			 For more details: https://github.com/camunda/camunda-platform-helm/pull/487
// TODO: Move this to Identity subchart once "export-values" is implemented.
//       For more details: https://github.com/helm/helm/pull/10804

package camunda

import (
	"camunda-platform/test/unit/utils"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func TestGoldenKeycloakDefaults(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &utils.TemplateGoldenTest{
		ChartPath:      chartPath,
		Release:        "camunda-platform-test",
		Namespace:      "camunda",
		GoldenFileName: "keycloak-statefulset",
		IgnoredLines: []string{
			`\s+.*-secret:\s+.*`,    // secrets are auto-generated and need to be ignored.
			`\s+checksum/.+?:\s+.*`, // ignore configmap checksum.
		},
		// ExtraHelmArgs is used instead of Templates here because Keycloak is a dependency chart.
		ExtraHelmArgs: []string{"--show-only", "charts/identityKeycloak/templates/statefulset.yaml"},
	})
}
