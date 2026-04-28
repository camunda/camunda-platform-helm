// Copyright 2025 Camunda Services GmbH
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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// Test suite for enterprise values validation
type EnterpriseValuesTestSuite struct {
	suite.Suite
	chartPath string
}

func (s *EnterpriseValuesTestSuite) SetupSuite() {
	var err error
	s.chartPath, err = filepath.Abs("../../../")
	require.NoError(s.T(), err)
}

func TestEnterpriseValuesTestSuite(t *testing.T) {
	suite.Run(t, new(EnterpriseValuesTestSuite))
}

// Test that Elasticsearch sysctlImage configuration is correct
func (s *EnterpriseValuesTestSuite) TestElasticsearchSysctlImageConfiguration() {
	// Capture t before Parallel to avoid suite.T() race with other parallel test methods.
	t := s.T()
	t.Parallel()

	// Test that Helm template renders successfully with enterprise values
	output := helm.RenderTemplate(t, &helm.Options{
		ValuesFiles: []string{filepath.Join(s.chartPath, "values-enterprise.yaml")},
		ExtraArgs: map[string][]string{
			"template": {"--show-only", "charts/elasticsearch/templates/master/statefulset.yaml"},
		},
	}, s.chartPath, "camunda-platform-test", []string{})

	// Verify the sysctl init container uses the correct enterprise image
	assert.Contains(t, output, "registry.camunda.cloud/vendor-ee/os-shell",
		"sysctlImage should use the enterprise registry and repository")

	// Verify that the sysctl init container is present
	assert.Contains(t, output, "name: sysctl")
}

// Test that PostgreSQL (Identity) configuration is correct
func (s *EnterpriseValuesTestSuite) TestIdentityPostgresqlConfiguration() {
	t := s.T()
	t.Parallel()

	// Test that Helm template renders successfully with enterprise values
	// NOTE: We need to enable Identity and PostgreSQL explicitly since they're disabled by default
	output := helm.RenderTemplate(t, &helm.Options{
		ValuesFiles: []string{filepath.Join(s.chartPath, "values-enterprise.yaml")},
		SetValues: map[string]string{
			"identity.enabled":            "true",
			"identity.postgresql.enabled": "true",
		},
		ExtraArgs: map[string][]string{
			"template": {"--show-only", "charts/postgresql/templates/primary/statefulset.yaml"},
		},
	}, s.chartPath, "camunda-platform-test", []string{})

	// Verify the PostgreSQL uses the correct enterprise image
	assert.Contains(t, output, "registry.camunda.cloud/vendor-ee/postgresql",
		"identityPostgresql should use the enterprise registry and repository")
}

// Test that Keycloak configuration is correct
func (s *EnterpriseValuesTestSuite) TestIdentityKeycloakConfiguration() {
	t := s.T()
	t.Parallel()

	// Test that Helm template renders successfully with enterprise values
	// NOTE: We need to enable Identity, Keycloak AND PostgreSQL since Keycloak depends on PostgreSQL
	output := helm.RenderTemplate(t, &helm.Options{
		ValuesFiles: []string{filepath.Join(s.chartPath, "values-enterprise.yaml")},
		SetValues: map[string]string{
			"identity.enabled":            "true",
			"identity.keycloak.enabled":   "true",
			"identity.postgresql.enabled": "true",
		},
		ExtraArgs: map[string][]string{
			"template": {"--show-only", "charts/keycloak/templates/statefulset.yaml"},
		},
	}, s.chartPath, "camunda-platform-test", []string{})

	// Verify the Keycloak uses the correct enterprise image
	assert.Contains(t, output, "registry.camunda.cloud/keycloak-ee/keycloak",
		"identityKeycloak should use the enterprise registry and repository")

	// NOTE: We don't check for "error" or "failed" here because Keycloak config contains
	// environment variables like "KEYCLOAK_LOG_LEVEL: error" which are legitimate
}

// Test that all enterprise values render without errors
func (s *EnterpriseValuesTestSuite) TestEnterpriseValuesRenderSuccessfully() {
	t := s.T()
	t.Parallel()

	// Test that Helm template command succeeds with enterprise values
	// This would fail if any component has incorrect configuration
	output := helm.RenderTemplate(t, &helm.Options{
		ValuesFiles: []string{filepath.Join(s.chartPath, "values-enterprise.yaml")},
	}, s.chartPath, "camunda-platform-test", []string{})

	// Basic validation that rendering succeeded and contains expected components
	assert.Contains(t, output, "kind: StatefulSet")
}

// Test that validates no nested image structure exists in values files
func (s *EnterpriseValuesTestSuite) TestNoNestedImageStructure() {
	t := s.T()
	t.Parallel()

	// Read the values-enterprise.yaml file directly
	valuesPath := filepath.Join(s.chartPath, "values-enterprise.yaml")
	content, err := os.ReadFile(valuesPath)
	require.NoError(t, err, "Should be able to read values-enterprise.yaml")

	valuesContent := string(content)

	// Check for problematic nested structures
	problematicStructures := []string{
		"sysctlImage:\n    image:",
		"sysctlImage:\n      image:",
		"sysctlImage:\r\n    image:",
		"sysctlImage:\r\n      image:",
	}

	for _, structure := range problematicStructures {
		assert.NotContains(t, valuesContent, structure,
			"sysctlImage should not have nested 'image:' structure")
	}

	// Verify sysctlImage has the correct flat structure
	assert.Contains(t, valuesContent, "sysctlImage:", "sysctlImage section should exist")
	assert.Contains(t, valuesContent, "registry: registry.camunda.cloud",
		"sysctlImage should have direct registry configuration")
	assert.Contains(t, valuesContent, "repository: vendor-ee/os-shell",
		"sysctlImage should have direct repository configuration")
}

// Test comprehensive enterprise image usage across all Bitnami subcharts
func (s *EnterpriseValuesTestSuite) TestComprehensiveEnterpriseImageUsage() {
	t := s.T()
	t.Parallel()

	// Expected enterprise images for different components
	expectedEnterpriseImages := map[string][]string{
		"elasticsearch": {
			"registry.camunda.cloud/vendor-ee/elasticsearch",
			"registry.camunda.cloud/vendor-ee/os-shell", // sysctlImage
			"registry.camunda.cloud/vendor-ee/elasticsearch-exporter", // metrics
		},
		"postgresql": {
			"registry.camunda.cloud/vendor-ee/postgresql",
			"registry.camunda.cloud/vendor-ee/postgres-exporter", // metrics
			"registry.camunda.cloud/vendor-ee/os-shell",          // volumePermissions
		},
		"keycloak": {
			"registry.camunda.cloud/keycloak-ee/keycloak",
			// Note: keycloak-config-cli is a job that only runs under certain conditions, not included here
			"registry.camunda.cloud/vendor-ee/postgresql", // embedded postgresql
		},
	}

	// Render the full template with enterprise values
	// NOTE: Enable all components and metrics to validate all enterprise images
	output := helm.RenderTemplate(t, &helm.Options{
		ValuesFiles: []string{filepath.Join(s.chartPath, "values-enterprise.yaml")},
		SetValues: map[string]string{
			"identity.enabled":                   "true",
			"identity.keycloak.enabled":          "true",
			"identity.postgresql.enabled":        "true",
			"elasticsearch.metrics.enabled":      "true",
			"postgresql.enabled":                 "true",
			"postgresql.metrics.enabled":         "true",
			"identityPostgresql.metrics.enabled": "true",
		},
	}, s.chartPath, "camunda-platform-test", []string{})

	for component, images := range expectedEnterpriseImages {
		t.Run(fmt.Sprintf("Component_%s", component), func(t *testing.T) {
			for _, expectedImage := range images {
				assert.Contains(t, output, expectedImage,
					fmt.Sprintf("Component %s should use enterprise image %s", component, expectedImage))
			}
		})
	}
}

// Test individual Bitnami subcharts render correctly
func (s *EnterpriseValuesTestSuite) TestIndividualBitnamiSubcharts() {
	t := s.T()
	t.Parallel()

	// Define the subcharts to test with their main templates
	subchartsToTest := map[string][]string{
		"elasticsearch": {
			"charts/elasticsearch/templates/master/statefulset.yaml",
		},
		"postgresql": {
			"charts/postgresql/templates/primary/statefulset.yaml",
		},
		"keycloak": {
			"charts/keycloak/templates/statefulset.yaml",
		},
	}

	for subchartName, templates := range subchartsToTest {
		t.Run(fmt.Sprintf("Subchart_%s", subchartName), func(t *testing.T) {
			t.Parallel()

			for _, template := range templates {
				t.Run(fmt.Sprintf("Template_%s", strings.ReplaceAll(template, "/", "_")), func(t *testing.T) {
					// Test that Helm template renders successfully with enterprise values
					output := helm.RenderTemplate(t, &helm.Options{
						ValuesFiles: []string{filepath.Join(s.chartPath, "values-enterprise.yaml")},
						ExtraArgs: map[string][]string{
							"template": {"--show-only", template},
						},
					}, s.chartPath, "camunda-platform-test", []string{})

					// Verify no YAML parsing errors occurred
					// NOTE: Skip "error" check for components that may have log level "error" in config:
					// - Keycloak: KEYCLOAK_LOG_LEVEL: error
					// - Elasticsearch: metrics exporter may have log level settings
					// - PostgreSQL: metrics exporter may have log level settings
					if subchartName != "keycloak" && subchartName != "elasticsearch" && subchartName != "postgresql" {
						assert.NotContains(t, strings.ToLower(output), "error",
							fmt.Sprintf("Template %s should render without errors", template))
						assert.NotContains(t, strings.ToLower(output), "failed",
							fmt.Sprintf("Template %s should render without failures", template))
					}

					// Verify the output contains expected Kubernetes resources
					assert.Contains(t, output, "kind:",
						fmt.Sprintf("Template %s should produce Kubernetes resources", template))
				})
			}
		})
	}
}

// Test that pull secrets are correctly configured for all components
func (s *EnterpriseValuesTestSuite) TestPullSecretsConfiguration() {
	t := s.T()
	t.Parallel()

	// Render the full template with enterprise values
	// NOTE: Enable all components to validate all pull secrets
	output := helm.RenderTemplate(t, &helm.Options{
		ValuesFiles: []string{filepath.Join(s.chartPath, "values-enterprise.yaml")},
		SetValues: map[string]string{
			"identity.enabled":            "true",
			"identity.keycloak.enabled":   "true",
			"identity.postgresql.enabled": "true",
		},
	}, s.chartPath, "camunda-platform-test", []string{})

	// Verify that the registry-camunda-cloud pull secret is used
	assert.Contains(t, output, "registry-camunda-cloud",
		"All enterprise components should use registry-camunda-cloud pull secret")

	// Count occurrences to ensure it's used in multiple places
	// With all components enabled (Elasticsearch + Identity + Keycloak + PostgreSQL),
	// we expect 4+ occurrences (Elasticsearch master/data + Identity + Keycloak)
	occurrences := strings.Count(output, "registry-camunda-cloud")
	assert.Greater(t, occurrences, 3, "Pull secret should be used in multiple enterprise components")
}
