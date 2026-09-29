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

package companion

import (
	"camunda-platform/test/unit/utils"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func TestGoldenDefaultsTemplateCompanionPostgresql(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../../internal-postgresql")
	require.NoError(t, err)

	suite.Run(t, &utils.TemplateGoldenTest{
		ChartPath:      chartPath,
		Release:        "camunda-platform-test",
		Namespace:      "camunda-platform",
		GoldenFileName: "internal-postgresql-statefulset",
		Templates:      []string{"templates/statefulset.yaml"},
	})
}

func TestGoldenDefaultsTemplateCompanionKeycloakPostgresql(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../../internal-keycloak-26")
	require.NoError(t, err)

	suite.Run(t, &utils.TemplateGoldenTest{
		ChartPath:      chartPath,
		Release:        "camunda-platform-test",
		Namespace:      "camunda-platform",
		GoldenFileName: "internal-keycloak-26-postgresql-statefulset",
		Templates:      []string{"templates/postgresql-statefulset.yaml"},
	})
}

func TestGoldenARMSchedulingTemplateCompanionKeycloakPostgresql(test *testing.T) {
	test.Parallel()

	chartPath, err := filepath.Abs("../../../../internal-keycloak-26")
	require.NoError(test, err)

	suite.Run(test, &utils.TemplateGoldenTest{
		ChartPath:      chartPath,
		Release:        "camunda-platform-test",
		Namespace:      "camunda-platform",
		GoldenFileName: "internal-keycloak-26-postgresql-statefulset-arm",
		Templates:      []string{"templates/postgresql-statefulset.yaml"},
		SetValues: map[string]string{
			"nodeSelector.workload":               "arm-processor",
			"tolerations[0].key":                  "workload",
			"tolerations[0].operator":             "Equal",
			"tolerations[0].value":                "arm-processor",
			"tolerations[0].effect":               "NoSchedule",
			"tolerations[1].key":                  "kubernetes.io/arch",
			"tolerations[1].operator":             "Equal",
			"tolerations[1].value":                "arm64",
			"tolerations[1].effect":               "NoSchedule",
			"postgresql.storage.storageClassName": "hyperdisk-balanced",
			"postgresql.storage.size":             "4Gi",
		},
	})
}
