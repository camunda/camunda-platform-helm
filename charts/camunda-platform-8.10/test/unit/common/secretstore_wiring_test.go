// secretstore_wiring_test.go
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

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

type secretStoreWiringTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
	templates []string
}

func TestSecretStoreWiringTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &secretStoreWiringTest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
		templates: []string{"templates/orchestration/serviceaccount.yaml"},
	})
}

func containerEnvNames(containers []corev1.Container) map[string]bool {
	names := make(map[string]bool)
	for _, c := range containers {
		for _, e := range c.Env {
			names[e.Name] = true
		}
	}
	return names
}

func (s *secretStoreWiringTest) TestServiceAccountAnnotations() {
	testCases := []testhelpers.TestCase{
		{
			Name:     "AWS store sets the IRSA role-arn annotation",
			Template: "templates/orchestration/serviceaccount.yaml",
			Values: mergeValues(baseValues(), map[string]string{
				"global.secretStore.aws.primary.region":  "us-east-1",
				"global.secretStore.aws.primary.roleArn": "arn:aws:iam::123456789012:role/camunda-secrets",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var sa corev1.ServiceAccount
				helm.UnmarshalK8SYaml(t, output, &sa)
				s.Require().Equal("arn:aws:iam::123456789012:role/camunda-secrets",
					sa.Annotations["eks.amazonaws.com/role-arn"])
			},
		},
		{
			Name:     "GCP store sets the Workload Identity annotation",
			Template: "templates/orchestration/serviceaccount.yaml",
			Values: mergeValues(baseValues(), map[string]string{
				"global.secretStore.gcp.primary.projectId":         "my-project",
				"global.secretStore.gcp.primary.gcpServiceAccount": "camunda@my-project.iam.gserviceaccount.com",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var sa corev1.ServiceAccount
				helm.UnmarshalK8SYaml(t, output, &sa)
				s.Require().Equal("camunda@my-project.iam.gserviceaccount.com",
					sa.Annotations["iam.gke.io/gcp-service-account"])
			},
		},
		{
			Name:     "Secret store annotations coexist with user-provided annotations",
			Template: "templates/orchestration/serviceaccount.yaml",
			Values: mergeValues(baseValues(), map[string]string{
				"global.secretStore.aws.primary.roleArn":             "arn:aws:iam::123456789012:role/camunda-secrets",
				"orchestration.serviceAccount.annotations.customKey": "customValue",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var sa corev1.ServiceAccount
				helm.UnmarshalK8SYaml(t, output, &sa)
				s.Require().Equal("customValue", sa.Annotations["customKey"])
				s.Require().Equal("arn:aws:iam::123456789012:role/camunda-secrets",
					sa.Annotations["eks.amazonaws.com/role-arn"])
			},
		},
		{
			Name:     "No secret store leaves identity annotations unset",
			Template: "templates/orchestration/serviceaccount.yaml",
			Values:   baseValues(),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var sa corev1.ServiceAccount
				helm.UnmarshalK8SYaml(t, output, &sa)
				_, hasAws := sa.Annotations["eks.amazonaws.com/role-arn"]
				_, hasGcp := sa.Annotations["iam.gke.io/gcp-service-account"]
				s.Require().False(hasAws)
				s.Require().False(hasGcp)
			},
		},
		{
			Name:     "Physical-tenant AWS store sets the IRSA role-arn annotation",
			Template: "templates/orchestration/serviceaccount.yaml",
			Values: mergeValues(baseValues(), map[string]string{
				"global.secretStore.physicalTenants.tenantA.aws.store.region":  "us-east-1",
				"global.secretStore.physicalTenants.tenantA.aws.store.roleArn": "arn:aws:iam::123456789012:role/camunda-secrets",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var sa corev1.ServiceAccount
				helm.UnmarshalK8SYaml(t, output, &sa)
				s.Require().Equal("arn:aws:iam::123456789012:role/camunda-secrets",
					sa.Annotations["eks.amazonaws.com/role-arn"])
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *secretStoreWiringTest) TestStatefulSetWiring() {
	testCases := []testhelpers.TestCase{
		{
			Name:     "File store mounts the existing secret as a volume",
			Template: "templates/orchestration/statefulset.yaml",
			Values: mergeValues(baseValues(), map[string]string{
				"global.secretStore.file.primary.path":           "/etc/camunda/secrets",
				"global.secretStore.file.primary.existingSecret": "my-secrets",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var sts appsv1.StatefulSet
				helm.UnmarshalK8SYaml(t, output, &sts)

				var found bool
				for _, v := range sts.Spec.Template.Spec.Volumes {
					if v.Name == "secretstore-file-primary" {
						found = true
						s.Require().NotNil(v.Secret)
						s.Require().Equal("my-secrets", v.Secret.SecretName)
					}
				}
				s.Require().True(found, "expected secretstore-file-primary volume")

				var mounted bool
				for _, c := range sts.Spec.Template.Spec.Containers {
					for _, m := range c.VolumeMounts {
						if m.Name == "secretstore-file-primary" {
							mounted = true
							s.Require().Equal("/etc/camunda/secrets", m.MountPath)
						}
					}
				}
				s.Require().True(mounted, "expected secretstore-file-primary volume mount")
			},
		},
		{
			Name:     "AWS store injects no static credentials",
			Template: "templates/orchestration/statefulset.yaml",
			Values: mergeValues(baseValues(), map[string]string{
				"global.secretStore.aws.primary.region":  "us-east-1",
				"global.secretStore.aws.primary.roleArn": "arn:aws:iam::123456789012:role/camunda-secrets",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var sts appsv1.StatefulSet
				helm.UnmarshalK8SYaml(t, output, &sts)
				envNames := containerEnvNames(sts.Spec.Template.Spec.Containers)
				s.Require().False(envNames["AWS_ACCESS_KEY_ID"])
				s.Require().False(envNames["AWS_SECRET_ACCESS_KEY"])
			},
		},
		{
			Name:     "GCP store mounts no credentials volume",
			Template: "templates/orchestration/statefulset.yaml",
			Values: mergeValues(baseValues(), map[string]string{
				"global.secretStore.gcp.primary.projectId":         "my-project",
				"global.secretStore.gcp.primary.gcpServiceAccount": "camunda@my-project.iam.gserviceaccount.com",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var sts appsv1.StatefulSet
				helm.UnmarshalK8SYaml(t, output, &sts)
				for _, v := range sts.Spec.Template.Spec.Volumes {
					s.Require().NotEqual("gcp-credentials-volume", v.Name)
				}
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}
