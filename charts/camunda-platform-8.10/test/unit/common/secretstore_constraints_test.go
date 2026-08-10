// secretstore_constraints_test.go
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
	"camunda-platform/test/unit/testhelpers"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type secretStoreConstraintsTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
	templates []string
}

func TestSecretStoreConstraintsTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &secretStoreConstraintsTest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
		templates: []string{"templates/orchestration/serviceaccount.yaml"},
	})
}

func (s *secretStoreConstraintsTest) TestConstraintFailures() {
	testCases := []testhelpers.TestCase{
		{
			Name:     "More than one store is rejected",
			Template: "templates/orchestration/serviceaccount.yaml",
			Values: mergeValues(baseValues(), map[string]string{
				"orchestration.secretStore.file.default.secret.existingSecret": "s",
				"orchestration.secretStore.aws.default.region":                 "us-east-1",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), "supports only one secret store at a time")
			},
		},
		{
			Name:     "More than one store within a physical tenant is rejected",
			Template: "templates/orchestration/serviceaccount.yaml",
			Values: mergeValues(secretStoreBaseValues(), map[string]string{
				"orchestration.secretStore.physicalTenants.tenanta.file.default.secret.existingSecret": "s",
				"orchestration.secretStore.physicalTenants.tenanta.aws.default.region":                 "us-east-1",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), "orchestration.secretStore.physicalTenants.tenanta supports only one secret store at a time")
			},
		},
		{
			Name:     "Conflicting roleArn across tenants is rejected",
			Template: "templates/orchestration/serviceaccount.yaml",
			Values: mergeValues(secretStoreBaseValues(), map[string]string{
				"orchestration.secretStore.aws.default.roleArn":                         "arn:aws:iam::111111111111:role/one",
				"orchestration.secretStore.physicalTenants.tenanta.aws.default.roleArn": "arn:aws:iam::222222222222:role/two",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), "multiple distinct aws.*.roleArn values")
			},
		},
		{
			Name:     "GCP pathPrefix with invalid characters is rejected",
			Template: "templates/orchestration/serviceaccount.yaml",
			Values: mergeValues(secretStoreBaseValues(), map[string]string{
				"orchestration.secretStore.gcp.default.pathPrefix": "camunda/",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), "gcp.default.pathPrefix must contain only [a-zA-Z0-9_-]")
			},
		},
		{
			Name:     "Blank GCP projectId is rejected",
			Template: "templates/orchestration/serviceaccount.yaml",
			Values: mergeValues(secretStoreBaseValues(), map[string]string{
				"orchestration.secretStore.gcp.default.projectId": "",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), "projectId")
			},
		},
		{
			Name:     "Conflicting gcpServiceAccount across tenants is rejected",
			Template: "templates/orchestration/serviceaccount.yaml",
			Values: mergeValues(secretStoreBaseValues(), map[string]string{
				"orchestration.secretStore.gcp.default.gcpServiceAccount":                         "one@my-project.iam.gserviceaccount.com",
				"orchestration.secretStore.physicalTenants.tenanta.gcp.default.gcpServiceAccount": "two@my-project.iam.gserviceaccount.com",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), "multiple distinct gcp.*.gcpServiceAccount values")
			},
		},
		{
			Name:     "GCP document-store credentials are rejected",
			Template: "templates/orchestration/statefulset.yaml",
			Values: mergeValues(secretStoreBaseValues(), map[string]string{
				"orchestration.secretStore.gcp.default.gcpServiceAccount": "camunda@my-project.iam.gserviceaccount.com",
				"global.documentStore.activeStoreId":                      "gcp",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), "GOOGLE_APPLICATION_CREDENTIALS takes precedence over Workload Identity")
			},
		},
		{
			Name:     "Conflicting user ServiceAccount identity is rejected",
			Template: "templates/orchestration/serviceaccount.yaml",
			Values: mergeValues(secretStoreBaseValues(), map[string]string{
				"orchestration.secretStore.aws.default.roleArn":                           "arn:aws:iam::111111111111:role/intended",
				"orchestration.serviceAccount.annotations.eks\\.amazonaws\\.com/role-arn": "arn:aws:iam::222222222222:role/other",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), "conflicts with the identity")
			},
		},
		{
			Name:     "External ServiceAccount with workload identity is rejected",
			Template: "templates/orchestration/statefulset.yaml",
			Values: mergeValues(secretStoreBaseValues(), map[string]string{
				"orchestration.secretStore.aws.default.roleArn": "arn:aws:iam::111111111111:role/intended",
				"orchestration.serviceAccount.enabled":          "false",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), "serviceAccount.enabled=true")
			},
		},
		{
			Name:     "Custom application configuration is rejected",
			Template: "templates/orchestration/configmap.yaml",
			Values: mergeValues(secretStoreBaseValues(), map[string]string{
				"orchestration.secretStore.file.default.path": "/external/secrets",
				"orchestration.configuration":                 "camunda: {}",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), "cannot be combined with orchestration.configuration")
			},
		},
		{
			Name:     "Static AWS document-store credentials are rejected",
			Template: "templates/orchestration/statefulset.yaml",
			Values: mergeValues(secretStoreBaseValues(), map[string]string{
				"orchestration.secretStore.aws.default.roleArn": "arn:aws:iam::111111111111:role/intended",
				"global.documentStore.type.aws.enabled":         "true",
				"global.documentStore.type.aws.irsa.enabled":    "false",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), "environment credentials before IRSA")
			},
		},
		{
			Name:     "Different inherited store is rejected",
			Template: "templates/orchestration/configmap.yaml",
			Values: mergeValues(secretStoreBaseValues(), map[string]string{
				"orchestration.secretStore.aws.default.region":                        "us-east-1",
				"orchestration.secretStore.physicalTenants.tenanta.file.default.path": "/tenant/secrets",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), "physical-tenant overlays cannot remove root stores")
			},
		},
		{
			Name:     "Reserved file mount path is rejected",
			Template: "templates/orchestration/statefulset.yaml",
			Values: mergeValues(secretStoreBaseValues(), map[string]string{
				"orchestration.secretStore.file.default.path":                  "/usr/local/camunda/config",
				"orchestration.secretStore.file.default.secret.existingSecret": "s",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), "conflicts with a required Orchestration volume mount")
			},
		},
		{
			Name:     "Root file mount path is rejected",
			Template: "templates/orchestration/statefulset.yaml",
			Values: mergeValues(secretStoreBaseValues(), map[string]string{
				"orchestration.secretStore.file.default.path":                  "/",
				"orchestration.secretStore.file.default.secret.existingSecret": "s",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), "conflicts with a required Orchestration volume mount")
			},
		},
		{
			Name:     "Non-canonical file mount path is rejected",
			Template: "templates/orchestration/statefulset.yaml",
			Values: mergeValues(secretStoreBaseValues(), map[string]string{
				"orchestration.secretStore.file.default.path":                  "/etc//camunda/secrets",
				"orchestration.secretStore.file.default.secret.existingSecret": "s",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), "must be canonical")
			},
		},
		{
			Name:     "Trailing slash file mount path is rejected",
			Template: "templates/orchestration/statefulset.yaml",
			Values: mergeValues(secretStoreBaseValues(), map[string]string{
				"orchestration.secretStore.file.default.path":                  "/etc/camunda/secrets/",
				"orchestration.secretStore.file.default.secret.existingSecret": "s",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), "must not contain a trailing slash")
			},
		},
		{
			Name:     "Different Secrets at one inherited path are rejected",
			Template: "templates/orchestration/statefulset.yaml",
			Values: mergeValues(secretStoreBaseValues(), map[string]string{
				"orchestration.secretStore.file.default.path":                                          "/etc/camunda/secrets",
				"orchestration.secretStore.file.default.secret.existingSecret":                         "root-secret",
				"orchestration.secretStore.physicalTenants.tenanta.file.default.secret.existingSecret": "tenant-secret",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), "different Kubernetes Secrets at the same effective path")
			},
		},
		{
			Name:     "Extra volume mount collision is rejected",
			Template: "templates/orchestration/statefulset.yaml",
			Values: mergeValues(secretStoreBaseValues(), map[string]string{
				"orchestration.secretStore.file.default.path":                  "/custom/secrets",
				"orchestration.secretStore.file.default.secret.existingSecret": "s",
				"orchestration.extraVolumeMounts[0].name":                      "custom",
				"orchestration.extraVolumeMounts[0].mountPath":                 "/custom/secrets",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), "conflicts with orchestration.extraVolumeMounts")
			},
		},
		{
			Name:     "Non-canonical extra volume mount is rejected",
			Template: "templates/orchestration/statefulset.yaml",
			Values: mergeValues(secretStoreBaseValues(), map[string]string{
				"orchestration.secretStore.file.default.path":                  "/custom/secrets",
				"orchestration.secretStore.file.default.secret.existingSecret": "s",
				"orchestration.extraVolumeMounts[0].name":                      "custom",
				"orchestration.extraVolumeMounts[0].mountPath":                 "/custom/./secrets",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), "extraVolumeMounts[].mountPath must be canonical")
			},
		},
		{
			Name:     "Path-only file store can use extra volume mount",
			Template: "templates/orchestration/statefulset.yaml",
			Values: mergeValues(secretStoreBaseValues(), map[string]string{
				"orchestration.secretStore.file.default.path":   "/custom/secrets",
				"orchestration.extraVolumeMounts[0].name":       "custom",
				"orchestration.extraVolumeMounts[0].mountPath":  "/custom/secrets",
				"orchestration.extraVolumes[0].name":            "custom",
				"orchestration.extraVolumes[0].emptyDir.medium": "Memory",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
			},
		},
		{
			Name:     "Path-only file store can use extra volume mount subdirectory",
			Template: "templates/orchestration/statefulset.yaml",
			Values: mergeValues(secretStoreBaseValues(), map[string]string{
				"orchestration.secretStore.file.default.path":   "/custom/secrets/store",
				"orchestration.extraVolumeMounts[0].name":       "custom",
				"orchestration.extraVolumeMounts[0].mountPath":  "/custom/secrets",
				"orchestration.extraVolumes[0].name":            "custom",
				"orchestration.extraVolumes[0].emptyDir.medium": "Memory",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
			},
		},
		{
			Name:     "GCP store rejects unsupported default image",
			Template: "templates/orchestration/serviceaccount.yaml",
			Values: mergeValues(baseValues(), map[string]string{
				"orchestration.secretStore.gcp.default.projectId": "my-project",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), "requires an Orchestration image version >= 8.10.0-alpha5 or SNAPSHOT")
			},
		},
		{
			Name:     "Generated volume name collision is rejected",
			Template: "templates/orchestration/statefulset.yaml",
			Values: mergeValues(secretStoreBaseValues(), map[string]string{
				"orchestration.secretStore.file.default.secret.existingSecret": "s",
				"orchestration.extraVolumes[0].name":                           "secretstore-default-default-2fbbe682",
				"orchestration.extraVolumes[0].emptyDir.medium":                "Memory",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), "generated volume name")
			},
		},
		{
			Name:     "GCP document store mount collision is rejected",
			Template: "templates/orchestration/statefulset.yaml",
			Values: mergeValues(secretStoreBaseValues(), map[string]string{
				"orchestration.secretStore.file.default.path":                  "/custom/gcp",
				"orchestration.secretStore.file.default.secret.existingSecret": "s",
				"global.documentStore.type.gcp.enabled":                        "true",
				"global.documentStore.type.gcp.mountPath":                      "/custom/gcp",
				"global.documentStore.type.gcp.secret.existingSecret":          "gcp-secret",
				"global.documentStore.type.gcp.secret.existingSecretKey":       "service-account.json",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), "conflicts with a required Orchestration volume mount")
			},
		},
		{
			Name:     "Extra configuration mount collision is rejected",
			Template: "templates/orchestration/statefulset.yaml",
			Values: mergeValues(secretStoreBaseValues(), map[string]string{
				"orchestration.secretStore.file.default.path":                  "/usr/local/camunda/config/custom.yaml",
				"orchestration.secretStore.file.default.secret.existingSecret": "s",
				"orchestration.extraConfiguration[0].file":                     "custom.yaml",
				"orchestration.extraConfiguration[0].content":                  "camunda: {}",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), "conflicts with a required Orchestration volume mount")
			},
		},
		{
			Name:     "Custom application configuration works when secret store is empty",
			Template: "templates/orchestration/configmap.yaml",
			Values: mergeValues(secretStoreBaseValues(), map[string]string{
				"orchestration.configuration": "camunda: {}",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}
