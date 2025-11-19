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
	"camunda-platform/test/unit/testhelpers"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	batchv1 "k8s.io/api/batch/v1"
)

type MigrationDataJobTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
	templates []string
}

func TestMigrationDataJobTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &MigrationDataJobTest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
		templates: []string{"templates/orchestration/migration-data-job.yaml"},
	})
}

func (s *MigrationDataJobTest) TestCustomTrustStoreConfiguration() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestElasticsearchTLSExistingSecret",
			Values: map[string]string{
				"orchestration.migration.data.enabled":    "true",
				"global.elasticsearch.tls.existingSecret": "elasticsearch-tls-secret",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var job batchv1.Job
				helm.UnmarshalK8SYaml(s.T(), output, &job)

				// Verify JAVA_TOOL_OPTIONS env var is set
				containers := job.Spec.Template.Spec.Containers
				s.Require().Equal(1, len(containers))

				found := false
				for _, env := range containers[0].Env {
					if env.Name == "JAVA_TOOL_OPTIONS" {
						found = true
						s.Require().Equal("-Djavax.net.ssl.trustStore=/usr/local/camunda/certificates/externaldb.jks", env.Value)
					}
				}
				s.Require().True(found, "JAVA_TOOL_OPTIONS env var should be set")

				// Verify volume mount for keystore
				volumeMounts := containers[0].VolumeMounts
				found = false
				for _, vm := range volumeMounts {
					if vm.Name == "keystore" {
						found = true
						s.Require().Equal("/usr/local/camunda/certificates/externaldb.jks", vm.MountPath)
						s.Require().Equal("externaldb.jks", vm.SubPath)
					}
				}
				s.Require().True(found, "keystore volume mount should exist")

				// Verify volume for keystore
				volumes := job.Spec.Template.Spec.Volumes
				found = false
				for _, vol := range volumes {
					if vol.Name == "keystore" {
						found = true
						s.Require().NotNil(vol.Secret)
						s.Require().Equal("elasticsearch-tls-secret", vol.Secret.SecretName)
						s.Require().False(*vol.Secret.Optional)
					}
				}
				s.Require().True(found, "keystore volume should exist")
			},
		},
		{
			Name: "TestOpenSearchTLSExistingSecret",
			Values: map[string]string{
				"orchestration.migration.data.enabled": "true",
				"global.opensearch.tls.existingSecret": "opensearch-tls-secret",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var job batchv1.Job
				helm.UnmarshalK8SYaml(s.T(), output, &job)

				// Verify JAVA_TOOL_OPTIONS env var is set
				containers := job.Spec.Template.Spec.Containers
				s.Require().Equal(1, len(containers))

				found := false
				for _, env := range containers[0].Env {
					if env.Name == "JAVA_TOOL_OPTIONS" {
						found = true
						s.Require().Equal("-Djavax.net.ssl.trustStore=/usr/local/camunda/certificates/externaldb.jks", env.Value)
					}
				}
				s.Require().True(found, "JAVA_TOOL_OPTIONS env var should be set")

				// Verify volume mount for keystore
				volumeMounts := containers[0].VolumeMounts
				found = false
				for _, vm := range volumeMounts {
					if vm.Name == "keystore" {
						found = true
						s.Require().Equal("/usr/local/camunda/certificates/externaldb.jks", vm.MountPath)
						s.Require().Equal("externaldb.jks", vm.SubPath)
					}
				}
				s.Require().True(found, "keystore volume mount should exist")

				// Verify volume for keystore
				volumes := job.Spec.Template.Spec.Volumes
				found = false
				for _, vol := range volumes {
					if vol.Name == "keystore" {
						found = true
						s.Require().NotNil(vol.Secret)
						s.Require().Equal("opensearch-tls-secret", vol.Secret.SecretName)
						s.Require().False(*vol.Secret.Optional)
					}
				}
				s.Require().True(found, "keystore volume should exist")
			},
		},
		{
			Name: "TestNoTLSExistingSecretDoesNotAddTrustStore",
			Values: map[string]string{
				"orchestration.migration.data.enabled": "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var job batchv1.Job
				helm.UnmarshalK8SYaml(s.T(), output, &job)

				// Verify JAVA_TOOL_OPTIONS env var is NOT set
				containers := job.Spec.Template.Spec.Containers
				s.Require().Equal(1, len(containers))

				for _, env := range containers[0].Env {
					s.Require().NotEqual("JAVA_TOOL_OPTIONS", env.Name, "JAVA_TOOL_OPTIONS should not be set when no TLS secret is configured")
				}

				// Verify NO volume mount for keystore
				volumeMounts := containers[0].VolumeMounts
				for _, vm := range volumeMounts {
					s.Require().NotEqual("keystore", vm.Name, "keystore volume mount should not exist when no TLS secret is configured")
				}

				// Verify NO volume for keystore
				volumes := job.Spec.Template.Spec.Volumes
				for _, vol := range volumes {
					s.Require().NotEqual("keystore", vol.Name, "keystore volume should not exist when no TLS secret is configured")
				}
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}
