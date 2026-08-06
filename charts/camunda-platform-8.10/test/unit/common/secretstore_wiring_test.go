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
				"orchestration.secretStore.aws.primary.region":  "us-east-1",
				"orchestration.secretStore.aws.primary.roleArn": "arn:aws:iam::123456789012:role/camunda-secrets",
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
				"orchestration.secretStore.gcp.primary.projectId":         "my-project",
				"orchestration.secretStore.gcp.primary.gcpServiceAccount": "camunda@my-project.iam.gserviceaccount.com",
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
				"orchestration.secretStore.aws.primary.roleArn":      "arn:aws:iam::123456789012:role/camunda-secrets",
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
				"orchestration.secretStore.physicalTenants.tenanta.aws.store.region":  "us-east-1",
				"orchestration.secretStore.physicalTenants.tenanta.aws.store.roleArn": "arn:aws:iam::123456789012:role/camunda-secrets",
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
				"orchestration.secretStore.file.primary.path":           "/etc/camunda/secrets",
				"orchestration.secretStore.file.primary.existingSecret": "my-secrets",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var sts appsv1.StatefulSet
				helm.UnmarshalK8SYaml(t, output, &sts)

				var foundName string
				for _, v := range sts.Spec.Template.Spec.Volumes {
					if v.Secret != nil && v.Secret.SecretName == "my-secrets" {
						foundName = v.Name
						s.Require().NotNil(v.Secret)
						s.Require().Equal("my-secrets", v.Secret.SecretName)
					}
				}
				s.Require().NotEmpty(foundName, "expected file secret-store volume")

				var mounted bool
				for _, c := range sts.Spec.Template.Spec.Containers {
					for _, m := range c.VolumeMounts {
						if m.Name == foundName {
							mounted = true
							s.Require().Equal("/etc/camunda/secrets", m.MountPath)
						}
					}
				}
				s.Require().True(mounted, "expected secretstore-file-primary volume mount")
			},
		},
		{
			Name:     "Path-only file store creates no chart-managed volume",
			Template: "templates/orchestration/statefulset.yaml",
			Values: mergeValues(baseValues(), map[string]string{
				"orchestration.secretStore.file.primary.path": "/external/secrets",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var sts appsv1.StatefulSet
				helm.UnmarshalK8SYaml(t, output, &sts)
				for _, v := range sts.Spec.Template.Spec.Volumes {
					s.Require().NotContains(v.Name, "secretstore")
				}
			},
		},
		{
			Name:     "AWS store injects no static credentials",
			Template: "templates/orchestration/statefulset.yaml",
			Values: mergeValues(baseValues(), map[string]string{
				"orchestration.secretStore.aws.primary.region":  "us-east-1",
				"orchestration.secretStore.aws.primary.roleArn": "arn:aws:iam::123456789012:role/camunda-secrets",
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
			Name:     "GCP store injects no static credentials",
			Template: "templates/orchestration/statefulset.yaml",
			Values: mergeValues(baseValues(), map[string]string{
				"orchestration.secretStore.gcp.primary.projectId":         "my-project",
				"orchestration.secretStore.gcp.primary.gcpServiceAccount": "camunda@my-project.iam.gserviceaccount.com",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var sts appsv1.StatefulSet
				helm.UnmarshalK8SYaml(t, output, &sts)
				envNames := containerEnvNames(sts.Spec.Template.Spec.Containers)
				s.Require().False(envNames["GOOGLE_APPLICATION_CREDENTIALS"])
				for _, v := range sts.Spec.Template.Spec.Volumes {
					s.Require().NotEqual("gcp-credentials-volume", v.Name)
				}
			},
		},
		{
			Name:     "Physical tenant file store inherits existingSecret",
			Template: "templates/orchestration/statefulset.yaml",
			Values: mergeValues(baseValues(), map[string]string{
				"orchestration.secretStore.file.shared.path":                         "/root/secrets",
				"orchestration.secretStore.file.shared.existingSecret":               "shared-secret",
				"orchestration.secretStore.physicalTenants.tenanta.file.shared.path": "/tenant/secrets",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var sts appsv1.StatefulSet
				helm.UnmarshalK8SYaml(t, output, &sts)
				var tenantVolumeName string
				for _, v := range sts.Spec.Template.Spec.Volumes {
					if v.Secret != nil && v.Secret.SecretName == "shared-secret" && strings.Contains(v.Name, "tenanta") {
						tenantVolumeName = v.Name
					}
				}
				s.Require().NotEmpty(tenantVolumeName)
				for _, mount := range sts.Spec.Template.Spec.Containers[0].VolumeMounts {
					if mount.Name == tenantVolumeName {
						s.Require().Equal("/tenant/secrets", mount.MountPath)
						return
					}
				}
				s.Fail("tenant file store mount was not rendered")
			},
		},
		{
			Name:     "Inherited identical file mount is deduplicated",
			Template: "templates/orchestration/statefulset.yaml",
			Values: mergeValues(baseValues(), map[string]string{
				"orchestration.secretStore.file.shared.path":                                   "/etc/camunda/secrets",
				"orchestration.secretStore.file.shared.existingSecret":                         "shared-secret",
				"orchestration.secretStore.physicalTenants.tenanta.file.shared.existingSecret": "shared-secret",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var sts appsv1.StatefulSet
				helm.UnmarshalK8SYaml(t, output, &sts)
				count := 0
				for _, mount := range sts.Spec.Template.Spec.Containers[0].VolumeMounts {
					if mount.MountPath == "/etc/camunda/secrets" {
						count++
					}
				}
				s.Require().Equal(1, count)
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *secretStoreWiringTest) TestIdentityChangeUpdatesPodTemplateChecksum() {
	render := func(roleArn string) appsv1.StatefulSet {
		output := helm.RenderTemplate(s.T(), &helm.Options{
			SetValues: mergeValues(baseValues(), map[string]string{
				"global.noSecondaryStorage":                     "true",
				"orchestration.secretStore.aws.primary.region":  "us-east-1",
				"orchestration.secretStore.aws.primary.roleArn": roleArn,
			}),
		}, s.chartPath, s.release, []string{"templates/orchestration/statefulset.yaml"})
		var sts appsv1.StatefulSet
		helm.UnmarshalK8SYaml(s.T(), output, &sts)
		return sts
	}

	before := render("arn:aws:iam::111111111111:role/old")
	after := render("arn:aws:iam::222222222222:role/new")
	s.Require().NotEqual(
		before.Spec.Template.Annotations["checksum/secret-store-identity"],
		after.Spec.Template.Annotations["checksum/secret-store-identity"],
	)
}
