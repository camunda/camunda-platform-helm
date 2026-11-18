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

package camunda

import (
	"camunda-platform/test/unit/utils"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func TestGoldenElasticsearchDefaults(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &utils.TemplateGoldenTest{
		ChartPath:      chartPath,
		Release:        "camunda-platform-test",
		Namespace:      "camunda",
		GoldenFileName: "elasticsearch-statefulset",
		IgnoredLines: []string{
			`\s+.*-secret:\s+.*`,    // ignore auto-generated secrets
			`\s+checksum/.+?:\s+.*`, // ignore configmap checksums
			`\s+version:/.+?:\s+.*`, // ignore version due to wrong chart.yaml reference

		},
		ExtraHelmArgs: []string{
			"--show-only", "charts/elasticsearch/templates/master/statefulset.yaml",
		},
	})
}

func TestGoldenElasticsearchAllRoles(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &utils.TemplateGoldenTest{
		ChartPath:      chartPath,
		Release:        "camunda-platform-test",
		Namespace:      "camunda",
		GoldenFileName: "elasticsearch-roles-statefulset",
		IgnoredLines: []string{
			`\s+.*-secret:\s+.*`,    // ignore auto-generated secrets
			`\s+checksum/.+?:\s+.*`, // ignore configmap checksums
			`\s+version:/.+?:\s+.*`, // ignore version due to wrong chart.yaml reference

		},
		SetValues: map[string]string{
			"elasticsearch.data.replicaCount":         "1",
			"elasticsearch.coordinating.replicaCount": "1",
			"elasticsearch.ingest.enabled":            "true",
			"elasticsearch.ingest.replicaCount":       "1",
		},
		ExtraHelmArgs: []string{
			"--show-only", "charts/elasticsearch/templates/data/statefulset.yaml",
			"--show-only", "charts/elasticsearch/templates/coordinating/statefulset.yaml",
			"--show-only", "charts/elasticsearch/templates/ingest/statefulset.yaml",
		},
	})
}
