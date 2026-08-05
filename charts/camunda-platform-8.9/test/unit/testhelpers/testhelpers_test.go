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

package testhelpers

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

var _ = flag.Bool("update-golden", false, "accepted for chart-wide golden updates")

func writeStorageValuesFile(t *testing.T, name, content string) string {
	t.Helper()

	valuesFile := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(valuesFile, []byte(content), 0o600))
	return valuesFile
}

func TestSetupHelmOptionsClonesValues(t *testing.T) {
	t.Parallel()

	values := map[string]string{
		"orchestration.data.secondaryStorage.type": "opensearch",
	}
	options, err := setupHelmOptions("test", values, nil, nil)
	require.NoError(t, err)

	options.SetValues["orchestration.data.secondaryStorage.type"] = "elasticsearch"
	require.Equal(t, "opensearch", values["orchestration.data.secondaryStorage.type"])
}

func TestStorageFixturePreservesHelmPrecedence(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)
	templates := []string{"templates/orchestration/configmap.yaml"}

	baseValues := writeStorageValuesFile(t, "base.yaml", `global:
  elasticsearch:
    enabled: true
elasticsearch:
  enabled: true
orchestration:
  data:
    secondaryStorage:
      type: elasticsearch
`)
	overlayValues := writeStorageValuesFile(t, "overlay.yaml", `global:
  elasticsearch:
    enabled: false
elasticsearch:
  enabled: false
orchestration:
  data:
    secondaryStorage:
      type: opensearch
`)
	directOpenSearchValues := map[string]string{
		"global.elasticsearch.enabled":             "false",
		"elasticsearch.enabled":                    "false",
		"orchestration.data.secondaryStorage.type": "opensearch",
	}

	fileOutput, err := renderTemplateE(t, chartPath, "test", "test", templates, nil, []string{baseValues, overlayValues}, nil, nil)
	require.NoError(t, err)
	directOutput, err := renderTemplateE(t, chartPath, "test", "test", templates, directOpenSearchValues, nil, nil, nil)
	require.NoError(t, err)
	require.Equal(t, directOutput, fileOutput)
	require.Contains(t, fileOutput, `type: "opensearch"`)

	directElasticsearchValues := map[string]string{
		"global.elasticsearch.enabled":             "false",
		"elasticsearch.enabled":                    "false",
		"orchestration.data.secondaryStorage.type": "elasticsearch",
	}
	mixedOutput, err := renderTemplateE(t, chartPath, "test", "test", templates, directElasticsearchValues, []string{baseValues, overlayValues}, nil, nil)
	require.NoError(t, err)
	directOutput, err = renderTemplateE(t, chartPath, "test", "test", templates, directElasticsearchValues, nil, nil, nil)
	require.NoError(t, err)
	require.Equal(t, directOutput, mixedOutput)
	require.Contains(t, mixedOutput, `type: "elasticsearch"`)
}
