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

package connectors

import (
	"camunda-platform/test/unit/testhelpers"
	"path/filepath"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	appsv1 "k8s.io/api/apps/v1"
)

type AwsCredentialsFileTemplateTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
	templates []string
}

func TestAwsCredentialsFileTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &AwsCredentialsFileTemplateTest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
		templates: []string{"templates/connectors/deployment.yaml"},
	})
}

func findContainerByName(containers []corev1.Container, name string) *corev1.Container {
	for i := range containers {
		if containers[i].Name == name {
			return &containers[i]
		}
	}
	return nil
}

func findVolumeByName(volumes []corev1.Volume, name string) *corev1.Volume {
	for i := range volumes {
		if volumes[i].Name == name {
			return &volumes[i]
		}
	}
	return nil
}

func findVolumeMountByName(mounts []corev1.VolumeMount, name string) *corev1.VolumeMount {
	for i := range mounts {
		if mounts[i].Name == name {
			return &mounts[i]
		}
	}
	return nil
}

func awsDocumentStoreCredentialsValues() map[string]string {
	return map[string]string{
		"connectors.enabled":                               "true",
		"global.documentStore.activeStoreId":               "aws",
		"global.documentStore.type.aws.enabled":            "true",
		"global.documentStore.type.aws.bucket":             "test-bucket",
		"global.documentStore.type.aws.region":             "us-east-1",
		"global.documentStore.type.aws.irsa.enabled":       "false",
		"global.documentStore.type.aws.existingSecret":     "aws-credentials",
		"global.documentStore.type.aws.accessKeyIdKey":     "awsAccessKeyId",
		"global.documentStore.type.aws.secretAccessKeyKey": "awsSecretAccessKey",
	}
}

func (s *AwsCredentialsFileTemplateTest) TestDifferentValuesInputs() {
	testCases := []testhelpers.TestCase{
		{
			Name:   "TestInitContainerPresentWithCorrectShape",
			Values: awsDocumentStoreCredentialsValues(),
			Verifier: func(t *testing.T, output string, err error) {
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				initContainer := findContainerByName(deployment.Spec.Template.Spec.InitContainers, "aws-credentials-file-init")
				s.Require().NotNil(initContainer, "aws-credentials-file-init should be present")
				s.Require().Equal("camunda/connectors-bundle:8.7.23", initContainer.Image)
				s.Require().NotNil(initContainer.SecurityContext)
				s.Require().True(*initContainer.SecurityContext.RunAsNonRoot)

				var accessKeyEnv, secretKeyEnv *corev1.EnvVar
				for i := range initContainer.Env {
					switch initContainer.Env[i].Name {
					case "AWS_ACCESS_KEY_ID":
						accessKeyEnv = &initContainer.Env[i]
					case "AWS_SECRET_ACCESS_KEY":
						secretKeyEnv = &initContainer.Env[i]
					}
				}
				s.Require().NotNil(accessKeyEnv)
				s.Require().Equal("aws-credentials", accessKeyEnv.ValueFrom.SecretKeyRef.Name)
				s.Require().Equal("awsAccessKeyId", accessKeyEnv.ValueFrom.SecretKeyRef.Key)
				s.Require().NotNil(secretKeyEnv)
				s.Require().Equal("aws-credentials", secretKeyEnv.ValueFrom.SecretKeyRef.Name)
				s.Require().Equal("awsSecretAccessKey", secretKeyEnv.ValueFrom.SecretKeyRef.Key)

				mount := findVolumeMountByName(initContainer.VolumeMounts, "aws-credentials-file")
				s.Require().NotNil(mount, "init container should mount aws-credentials-file")
				s.Require().Equal("/var/camunda/aws-credentials", mount.MountPath)
			},
		},
		{
			Name:   "TestMainContainerHasFileEnvVarsNotKeys",
			Values: awsDocumentStoreCredentialsValues(),
			Verifier: func(t *testing.T, output string, err error) {
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				container := findContainerByName(deployment.Spec.Template.Spec.Containers, "connectors")
				s.Require().NotNil(container)

				var hasAccessKey, hasSecretKey, hasSharedFile, hasProfilesFile bool
				for _, env := range container.Env {
					switch env.Name {
					case "AWS_ACCESS_KEY_ID":
						hasAccessKey = true
					case "AWS_SECRET_ACCESS_KEY":
						hasSecretKey = true
					case "AWS_SHARED_CREDENTIALS_FILE":
						hasSharedFile = env.Value == "/var/camunda/aws-credentials/credentials"
					case "AWS_CREDENTIAL_PROFILES_FILE":
						hasProfilesFile = env.Value == "/var/camunda/aws-credentials/credentials"
					}
				}
				s.Require().False(hasAccessKey, "main container should not have AWS_ACCESS_KEY_ID")
				s.Require().False(hasSecretKey, "main container should not have AWS_SECRET_ACCESS_KEY")
				s.Require().True(hasSharedFile, "main container should have AWS_SHARED_CREDENTIALS_FILE")
				s.Require().True(hasProfilesFile, "main container should have AWS_CREDENTIAL_PROFILES_FILE")

				mount := findVolumeMountByName(container.VolumeMounts, "aws-credentials-file")
				s.Require().NotNil(mount, "main container should mount aws-credentials-file")
				s.Require().True(mount.ReadOnly)
			},
		},
		{
			Name:   "TestVolumeIsEmptyDirMemory",
			Values: awsDocumentStoreCredentialsValues(),
			Verifier: func(t *testing.T, output string, err error) {
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				volume := findVolumeByName(deployment.Spec.Template.Spec.Volumes, "aws-credentials-file")
				s.Require().NotNil(volume, "aws-credentials-file volume should be present")
				s.Require().NotNil(volume.EmptyDir)
				s.Require().Equal(corev1.StorageMediumMemory, volume.EmptyDir.Medium)
			},
		},
		{
			Name: "TestNothingRendersWhenIrsaEnabled",
			Values: func() map[string]string {
				v := awsDocumentStoreCredentialsValues()
				v["global.documentStore.type.aws.irsa.enabled"] = "true"
				return v
			}(),
			Verifier: func(t *testing.T, output string, err error) {
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				s.Require().Nil(findContainerByName(deployment.Spec.Template.Spec.InitContainers, "aws-credentials-file-init"))
				s.Require().Nil(findVolumeByName(deployment.Spec.Template.Spec.Volumes, "aws-credentials-file"))
			},
		},
		{
			Name: "TestNothingRendersWhenAwsDisabled",
			Values: map[string]string{
				"connectors.enabled":                    "true",
				"global.documentStore.type.aws.enabled": "false",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				s.Require().Nil(findContainerByName(deployment.Spec.Template.Spec.InitContainers, "aws-credentials-file-init"))
				s.Require().Nil(findVolumeByName(deployment.Spec.Template.Spec.Volumes, "aws-credentials-file"))
			},
		},
		{
			Name: "TestCustomSecretKeyNamesStillResolve",
			Values: func() map[string]string {
				v := awsDocumentStoreCredentialsValues()
				v["global.documentStore.type.aws.existingSecret"] = "custom-secret"
				v["global.documentStore.type.aws.accessKeyIdKey"] = "custom-access-key"
				v["global.documentStore.type.aws.secretAccessKeyKey"] = "custom-secret-key"
				return v
			}(),
			Verifier: func(t *testing.T, output string, err error) {
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				initContainer := findContainerByName(deployment.Spec.Template.Spec.InitContainers, "aws-credentials-file-init")
				s.Require().NotNil(initContainer)

				var accessKeyEnv *corev1.EnvVar
				for i := range initContainer.Env {
					if initContainer.Env[i].Name == "AWS_ACCESS_KEY_ID" {
						accessKeyEnv = &initContainer.Env[i]
					}
				}
				s.Require().NotNil(accessKeyEnv)
				s.Require().Equal("custom-secret", accessKeyEnv.ValueFrom.SecretKeyRef.Name)
				s.Require().Equal("custom-access-key", accessKeyEnv.ValueFrom.SecretKeyRef.Key)
			},
		},
		{
			Name: "TestCoexistsWithUserInitContainers",
			Values: func() map[string]string {
				v := awsDocumentStoreCredentialsValues()
				v["connectors.initContainers[0].name"] = "nginx"
				v["connectors.initContainers[0].image"] = "nginx:latest"
				return v
			}(),
			Verifier: func(t *testing.T, output string, err error) {
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				initContainers := deployment.Spec.Template.Spec.InitContainers
				s.Require().NotNil(findContainerByName(initContainers, "aws-credentials-file-init"))
				s.Require().NotNil(findContainerByName(initContainers, "nginx"))
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}
