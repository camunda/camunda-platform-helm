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
	"testing"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/stretchr/testify/require"
)

func chartPath(t *testing.T) string {
	t.Helper()
	path, err := filepath.Abs("../../../")
	require.NoError(t, err)
	return path
}

func TestOrchestrationTopology(t *testing.T) {
	t.Parallel()

	valuesFile := filepath.Join("testdata", "orchestration.yaml")
	options := &helm.Options{ValuesFiles: []string{valuesFile}}
	output := helm.RenderTemplate(t, options, chartPath(t), "camunda", nil)

	for _, resource := range []string{
		"templates/orchestration/statefulset.yaml",
		"templates/connectors/deployment.yaml",
		"templates/optimize/deployment.yaml",
		"templates/service-monitor/orchestration-service-monitor.yaml",
		"templates/service-monitor/connectors-service-monitor.yaml",
		"templates/service-monitor/optimize-service-monitor.yaml",
	} {
		require.Contains(t, output, resource)
	}
	for _, resource := range []string{
		"templates/identity/deployment.yaml",
		"templates/console/deployment.yaml",
		"templates/web-modeler/deployment-restapi.yaml",
		"templates/web-modeler/deployment-websockets.yaml",
		"templates/service-monitor/identity-service-monitor.yaml",
		"templates/service-monitor/console-service-monitor.yaml",
		"templates/service-monitor/web-modeler-service-monitor.yaml",
	} {
		require.NotContains(t, output, resource)
	}
}

func TestOrchestrationTopologyConstraints(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		values  map[string]string
		message string
	}{
		{"auth disabled", map[string]string{"global.identity.auth.enabled": "false"}, "requires global.identity.auth.enabled=true"},
		{"identity enabled", map[string]string{"identity.enabled": "true"}, "requires identity.enabled=false"},
		{"identity URL empty", map[string]string{"global.identity.service.url": ""}, "requires global.identity.service.url"},
		{"orchestration disabled", map[string]string{"orchestration.enabled": "false"}, "requires orchestration.enabled=true"},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			options := &helm.Options{
				ValuesFiles: []string{filepath.Join("testdata", "orchestration.yaml")},
				SetValues:   test.values,
			}
			_, err := helm.RenderTemplateE(t, options, chartPath(t), "camunda", []string{"templates/orchestration/configmap.yaml"})
			require.ErrorContains(t, err, test.message)
		})
	}
}

func TestCombinedTopologyPreservesEnabledValues(t *testing.T) {
	t.Parallel()

	options := &helm.Options{
		ValuesFiles: []string{filepath.Join("testdata", "orchestration.yaml")},
		SetValues: map[string]string{
			"global.topology.mode":                   "combined",
			"identity.enabled":                       "true",
			"webModeler.restapi.mail.fromAddress":    "noreply@example.com",
			"webModeler.restapi.mail.smtpHost":       "smtp.example.com",
			"webModeler.restapi.mail.smtpPort":       "587",
			"webModeler.restapi.mail.smtpUser":       "noreply@example.com",
			"webModeler.restapi.mail.smtpTlsEnabled": "true",
		},
	}

	for _, template := range []string{
		"templates/identity/service.yaml",
		"templates/console/service.yaml",
		"templates/web-modeler/service-restapi.yaml",
	} {
		output, err := helm.RenderTemplateE(t, options, chartPath(t), "camunda", []string{template})
		require.NoError(t, err)
		require.Contains(t, output, "kind: Service")
	}
}
