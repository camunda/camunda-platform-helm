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
	"testing"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/stretchr/testify/require"
)

func TestOptimizeTopologyWarnsForEnabledBackendWithoutAHost(t *testing.T) {
	valuesFile := filepath.Join("testdata", "optimize.yaml")
	for _, tc := range []struct {
		values  map[string]string
		warning string
	}{
		{
			values:  map[string]string{"optimize.database.elasticsearch.url.host": ""},
			warning: "Optimize uses Elasticsearch, but optimize.database.elasticsearch.url.host is empty",
		},
		{
			values: map[string]string{
				"optimize.database.elasticsearch.enabled": "false",
				"optimize.database.opensearch.enabled":    "true",
				"optimize.database.opensearch.url.host":   "",
			},
			warning: "Optimize uses OpenSearch, but optimize.database.opensearch.url.host is empty",
		},
	} {
		options := &helm.Options{ValuesFiles: []string{valuesFile}, SetValues: tc.values}
		output := helm.RenderTemplate(t, options, chartPath(t), "camunda", []string{"templates/optimize/configmap.yaml", "templates/common/configmap-warnings.yaml"})
		require.Contains(t, output, "nodes:")
		require.NotContains(t, output, "- host:")
		require.Contains(t, output, tc.warning)
	}
}

func TestOptimizeTopologyAcceptsTemplatedBackendHost(t *testing.T) {
	valuesFile := filepath.Join("testdata", "optimize.yaml")
	options := &helm.Options{
		ValuesFiles:   []string{valuesFile},
		SetJSONValues: map[string]string{"optimize.database.elasticsearch.url.host": `"{{ .Release.Name }}-search"`},
	}

	output := helm.RenderTemplate(t, options, chartPath(t), "camunda", []string{"templates/optimize/configmap.yaml"})
	require.Contains(t, output, `host: "camunda-search"`)
}
