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

package orchestration

import (
	"camunda-platform/test/unit/utils"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func TestGoldenDefaultsTemplateOrchestration(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)
	templateNames := []string{
		"service",
		"service-headless",
		"serviceaccount",
		"statefulset",
		"configmap",
	}

	for _, name := range templateNames {
		suite.Run(t, &utils.TemplateGoldenTest{
			ChartPath:      chartPath,
			Release:        "camunda-platform-test",
			Namespace:      "camunda-platform-" + strings.ToLower(random.UniqueId()),
			GoldenFileName: name,
			Templates:      []string{"templates/orchestration/" + name + ".yaml"},
			SetValues: map[string]string{
				"global.elasticsearch.enabled": "true",
				"elasticsearch.enabled":        "true",
			},
			IgnoredLines: []string{
				`\s+checksum/.+?:\s+.*`, // ignore configmap checksum.
			},
		})
	}
}

func TestGoldenAzureDocumentStoreTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &utils.TemplateGoldenTest{
		ChartPath:      chartPath,
		Release:        "camunda-platform-test",
		Namespace:      "camunda-platform-" + strings.ToLower(random.UniqueId()),
		GoldenFileName: "azure-documentstore",
		Templates: []string{
			"templates/common/configmap-documentstore.yaml",
			"templates/orchestration/configmap.yaml",
			"templates/orchestration/statefulset.yaml",
		},
		SetValues: map[string]string{
			"orchestration.data.secondaryStorage.type":                                  "elasticsearch",
			"global.documentStore.activeStoreId":                                        "az1",
			"global.documentStore.type.azure.connectionString.secret.existingSecret":    "azure-credentials",
			"global.documentStore.type.azure.connectionString.secret.existingSecretKey": "connection-string",
			"orchestration.env[0].name":                                                 "DOCUMENT_STORE_AZ1_CLASS",
			"orchestration.env[0].value":                                                "io.camunda.document.store.azure.AzureBlobDocumentStoreProvider",
			"orchestration.env[1].name":                                                 "DOCUMENT_STORE_AZ1_CONTAINER",
			"orchestration.env[1].value":                                                "documents",
		},
		IgnoredLines: []string{`\s+checksum/.+?:\s+.*`},
	})
}

func TestGoldenDefaultsTemplateOrchestrationMigrationIdentity(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)
	templateNames := []string{}

	for _, name := range templateNames {
		suite.Run(t, &utils.TemplateGoldenTest{
			ChartPath:      chartPath,
			Release:        "camunda-platform-test",
			Namespace:      "camunda-platform-" + strings.ToLower(random.UniqueId()),
			GoldenFileName: name,
			Templates:      []string{"templates/orchestration/" + name + ".yaml"},
			SetValues: map[string]string{
				"global.elasticsearch.enabled": "true",
				"elasticsearch.enabled":        "true",
			},
			IgnoredLines: []string{
				`\s+checksum/.+?:\s+.*`, // ignore configmap checksum.
			},
		})
	}
}
