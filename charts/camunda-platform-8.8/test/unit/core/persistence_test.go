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

package core

import (
	"camunda-platform/test/unit/testhelpers"
	"path/filepath"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type PersistenceTemplateTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
	templates []string
}

func TestPersistenceTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &PersistenceTemplateTest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
		templates: []string{"templates/core/statefulset.yaml"},
	})
}

func (s *PersistenceTemplateTest) TestPersistenceConfiguration() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestPersistenceDisabledUsesEmptyDir",
			Values: map[string]string{
				"core.enabled": "true",
				"core.persistenceType": "disk",
				// persistence.enabled defaults to false
			},
			Verifier: func(t *testing.T, output string, err error) {
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)

				// Should use legacy PVC configuration when persistenceType=disk and persistence.enabled=false
				s.Require().Equal(1, len(statefulSet.Spec.VolumeClaimTemplates))
				pvc := statefulSet.Spec.VolumeClaimTemplates[0]
				s.Require().Equal("data", pvc.Name)
				s.Require().Equal("32Gi", pvc.Spec.Resources.Requests.Storage().String())

				// Check volume mount
				volumeMounts := statefulSet.Spec.Template.Spec.Containers[0].VolumeMounts
				dataMountFound := false
				for _, mount := range volumeMounts {
					if mount.Name == "data" && mount.MountPath == "/usr/local/camunda/data" {
						dataMountFound = true
						break
					}
				}
				s.Require().True(dataMountFound, "Data volume mount should be present")
			},
		},
		{
			Name: "TestPersistenceEnabledCreatesPVC",
			Values: map[string]string{
				"core.enabled":                    "true",
				"core.persistence.enabled":        "true",
				"core.persistence.size":           "10Gi",
				"core.persistence.accessModes[0]": "ReadWriteOnce",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)

				// Should use new persistence configuration
				s.Require().Equal(1, len(statefulSet.Spec.VolumeClaimTemplates))
				pvc := statefulSet.Spec.VolumeClaimTemplates[0]
				s.Require().Equal("data", pvc.Name)
				s.Require().Equal("10Gi", pvc.Spec.Resources.Requests.Storage().String())
				s.Require().Equal(corev1.ReadWriteOnce, pvc.Spec.AccessModes[0])

				// Check volume mount
				volumeMounts := statefulSet.Spec.Template.Spec.Containers[0].VolumeMounts
				dataMountFound := false
				for _, mount := range volumeMounts {
					if mount.Name == "data" && mount.MountPath == "/usr/local/camunda/data" {
						dataMountFound = true
						break
					}
				}
				s.Require().True(dataMountFound, "Data volume mount should be present")
			},
		},
		{
			Name: "TestPersistenceTypeMemoryUsesMemory",
			Values: map[string]string{
				"core.enabled":         "true",
				"core.persistenceType": "memory",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)

				// Should use memory-based emptyDir
				s.Require().Equal(0, len(statefulSet.Spec.VolumeClaimTemplates))

				volumes := statefulSet.Spec.Template.Spec.Volumes
				dataVolumeFound := false
				for _, volume := range volumes {
					if volume.Name == "data" && volume.EmptyDir != nil && volume.EmptyDir.Medium == "Memory" {
						dataVolumeFound = true
						break
					}
				}
				s.Require().True(dataVolumeFound, "Memory-based data volume should be present")

				// Check volume mount
				volumeMounts := statefulSet.Spec.Template.Spec.Containers[0].VolumeMounts
				dataMountFound := false
				for _, mount := range volumeMounts {
					if mount.Name == "data" && mount.MountPath == "/usr/local/camunda/data" {
						dataMountFound = true
						break
					}
				}
				s.Require().True(dataMountFound, "Data volume mount should be present")
			},
		},
		{
			Name: "TestPersistenceTypeLocalNoVolume",
			Values: map[string]string{
				"core.enabled":         "true",
				"core.persistenceType": "local",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)

				// Should not have volumeClaimTemplates
				s.Require().Equal(0, len(statefulSet.Spec.VolumeClaimTemplates))

				// Should not have data volume mount
				volumeMounts := statefulSet.Spec.Template.Spec.Containers[0].VolumeMounts
				dataMountFound := false
				for _, mount := range volumeMounts {
					if mount.Name == "data" {
						dataMountFound = true
						break
					}
				}
				s.Require().False(dataMountFound, "Data volume mount should not be present")
			},
		},
		{
			Name: "TestPersistenceWithStorageClass",
			Values: map[string]string{
				"core.enabled":                      "true",
				"core.persistence.enabled":          "true",
				"core.persistence.size":             "10Gi",
				"core.persistence.storageClassName": "fast-ssd",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)

				pvc := statefulSet.Spec.VolumeClaimTemplates[0]
				s.Require().Equal("fast-ssd", *pvc.Spec.StorageClassName)
			},
		},
		{
			Name: "TestPersistenceWithAnnotations",
			Values: map[string]string{
				"core.enabled":                     "true",
				"core.persistence.enabled":         "true",
				"core.persistence.size":            "10Gi",
				"core.persistence.annotations.foo": "bar",
				"core.persistence.annotations.baz": "qux",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)

				pvc := statefulSet.Spec.VolumeClaimTemplates[0]
				s.Require().Equal("bar", pvc.Annotations["foo"])
				s.Require().Equal("qux", pvc.Annotations["baz"])
			},
		},
		{
			Name: "TestPersistenceWithSelector",
			Values: map[string]string{
				"core.enabled":                                  "true",
				"core.persistence.enabled":                      "true",
				"core.persistence.size":                         "10Gi",
				"core.persistence.selector.matchLabels.storage": "fast",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)

				pvc := statefulSet.Spec.VolumeClaimTemplates[0]
				s.Require().NotNil(pvc.Spec.Selector)
				s.Require().Equal("fast", pvc.Spec.Selector.MatchLabels["storage"])
			},
		},
		{
			Name: "TestPersistenceDisabledWhenComponentDisabled",
			Values: map[string]string{
				"core.enabled":                    "false",
				"core.persistence.enabled":        "true",
				"core.persistence.size":           "10Gi",
			},
			Verifier: func(t *testing.T, output string, err error) {
				// When component is disabled, no statefulset should be created
				s.Require().Empty(output, "no statefulset should be created when component is disabled")
			},
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		s.Run(testCase.Name, func() {
			s.T().Parallel()
			helmChartPath, err := filepath.Abs(s.chartPath)
			s.Require().NoError(err)

			output, err := helm.RenderTemplateE(
				s.T(),
				&helm.Options{
					SetValues: testCase.Values,
				},
				helmChartPath,
				s.release,
				s.templates,
			)

			testCase.Verifier(s.T(), output, err)
		})
	}
}

func TestPVCManifestCreated(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	testCase := testhelpers.TestCase{
		Name: "TestPVCManifestCreated",
		Values: map[string]string{
			"core.enabled":             "true",
			"core.persistence.enabled": "true",
			"core.persistence.size":    "10Gi",
		},
		Verifier: func(t *testing.T, output string, err error) {
			var statefulSet appsv1.StatefulSet
			helm.UnmarshalK8SYaml(t, output, &statefulSet)
			require.Equal(t, 1, len(statefulSet.Spec.VolumeClaimTemplates))
			pvc := statefulSet.Spec.VolumeClaimTemplates[0]
			require.Equal(t, "data", pvc.Name)
			require.Equal(t, "10Gi", pvc.Spec.Resources.Requests.Storage().String())
		},
	}

	testhelpers.RunTestCasesE(t, chartPath, "camunda-platform-test", "camunda-platform-core", []string{"templates/core/statefulset.yaml"}, []testhelpers.TestCase{testCase})
}
