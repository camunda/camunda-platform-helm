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

package companion

import (
	"camunda-platform/test/unit/testhelpers"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
)

type KeycloakPostgresqlStatefulSetTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
	templates []string
}

func TestKeycloakPostgresqlStatefulSetTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../../internal-keycloak-26")
	require.NoError(t, err)

	suite.Run(t, &KeycloakPostgresqlStatefulSetTest{
		chartPath: chartPath,
		release:   "keycloak-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
		templates: []string{"templates/postgresql-statefulset.yaml"},
	})
}

func (s *KeycloakPostgresqlStatefulSetTest) TestStorageDifferentValuesInputs() {
	testCases := []testhelpers.TestCase{
		{
			Name:   "TestStorageEnabledByDefaultUsesVolumeClaimTemplate",
			Values: map[string]string{},
			Verifier: func(t *testing.T, output string, err error) {
				statefulSet := unmarshalStatefulSet(t, output, err)

				require.Equal(t, "StatefulSet", statefulSet.Kind)
				require.NotEmpty(t, statefulSet.Spec.ServiceName)
				require.Len(t, statefulSet.Spec.VolumeClaimTemplates, 1)

				claim := statefulSet.Spec.VolumeClaimTemplates[0]
				require.Equal(t, "data", claim.Name)
				require.Equal(t, "2Gi", claim.Spec.Resources.Requests.Storage().String())
				require.Nil(t, claim.Spec.StorageClassName)
				require.Equal(t, []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}, claim.Spec.AccessModes)

				require.Nil(t, podVolume(statefulSet, "data"),
					"data must come from the claim template, not a pod volume")

				require.Equal(t, "statefulset", statefulSet.Spec.Template.Labels["camunda.io/controller"])
				require.NotContains(t, statefulSet.Spec.Selector.MatchLabels, "camunda.io/controller",
					"spec.selector must stay free of the discriminator; it is immutable after creation")
			},
		}, {
			Name: "TestStorageDisabledFallsBackToEmptyDir",
			Values: map[string]string{
				"postgresql.storage.enabled": "false",
			},
			Verifier: func(t *testing.T, output string, err error) {
				statefulSet := unmarshalStatefulSet(t, output, err)

				require.Empty(t, statefulSet.Spec.VolumeClaimTemplates)

				volume := podVolume(statefulSet, "data")
				require.NotNil(t, volume)
				require.NotNil(t, volume.EmptyDir)
			},
		}, {
			Name: "TestStorageClassNameIsRendered",
			Values: map[string]string{
				"postgresql.storage.storageClassName": "standard-rwo",
				"postgresql.storage.size":             "5Gi",
			},
			Verifier: func(t *testing.T, output string, err error) {
				statefulSet := unmarshalStatefulSet(t, output, err)

				require.Len(t, statefulSet.Spec.VolumeClaimTemplates, 1)
				claim := statefulSet.Spec.VolumeClaimTemplates[0]
				require.NotNil(t, claim.Spec.StorageClassName)
				require.Equal(t, "standard-rwo", *claim.Spec.StorageClassName)
				require.Equal(t, "5Gi", claim.Spec.Resources.Requests.Storage().String())
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (suiteTest *KeycloakPostgresqlStatefulSetTest) TestSchedulingDifferentValuesInputs() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestSchedulingAbsentByDefault",
			Verifier: func(test *testing.T, output string, err error) {
				statefulSet := unmarshalStatefulSet(test, output, err)

				require.NotEmpty(test, statefulSet.Spec.Template.Spec.Containers)
				require.Nil(test, statefulSet.Spec.Template.Spec.NodeSelector)
				require.Nil(test, statefulSet.Spec.Template.Spec.Tolerations)
			},
		},
		{
			Name:   "TestNodeSelectorWithoutTolerations",
			Values: map[string]string{"nodeSelector.workload": "distroci"},
			Verifier: func(test *testing.T, output string, err error) {
				statefulSet := unmarshalStatefulSet(test, output, err)

				require.Equal(test, map[string]string{"workload": "distroci"}, statefulSet.Spec.Template.Spec.NodeSelector)
				require.Nil(test, statefulSet.Spec.Template.Spec.Tolerations)
			},
		},
		{
			Name: "TestTolerationsWithoutNodeSelector",
			Values: map[string]string{
				"tolerations[0].key":      "workload",
				"tolerations[0].operator": "Exists",
			},
			Verifier: func(test *testing.T, output string, err error) {
				statefulSet := unmarshalStatefulSet(test, output, err)

				require.Nil(test, statefulSet.Spec.Template.Spec.NodeSelector)
				require.Equal(test, []corev1.Toleration{{Key: "workload", Operator: corev1.TolerationOpExists}}, statefulSet.Spec.Template.Spec.Tolerations)
			},
		},
	}

	for _, workload := range []string{"distroci", "qa-workloads", "arm-processor"} {
		values := map[string]string{
			"nodeSelector.workload":   workload,
			"tolerations[0].effect":   "NoSchedule",
			"tolerations[0].key":      "workload",
			"tolerations[0].operator": "Equal",
			"tolerations[0].value":    workload,
		}
		tolerations := []corev1.Toleration{{
			Key:      "workload",
			Operator: corev1.TolerationOpEqual,
			Value:    workload,
			Effect:   corev1.TaintEffectNoSchedule,
		}}
		var storageClass *string
		if workload == "arm-processor" {
			values["tolerations[1].key"] = "kubernetes.io/arch"
			values["tolerations[1].operator"] = "Equal"
			values["tolerations[1].value"] = "arm64"
			values["tolerations[1].effect"] = "NoSchedule"
			values["postgresql.storage.storageClassName"] = "hyperdisk-balanced"
			values["postgresql.storage.size"] = "4Gi"
			tolerations = append(tolerations, corev1.Toleration{
				Key:      "kubernetes.io/arch",
				Operator: corev1.TolerationOpEqual,
				Value:    "arm64",
				Effect:   corev1.TaintEffectNoSchedule,
			})
			storageClassName := "hyperdisk-balanced"
			storageClass = &storageClassName
		}

		testCases = append(testCases, testhelpers.TestCase{
			Name:   "TestSchedulingInherits_" + workload,
			Values: values,
			Verifier: func(test *testing.T, output string, err error) {
				statefulSet := unmarshalStatefulSet(test, output, err)

				require.Equal(test, map[string]string{"workload": workload}, statefulSet.Spec.Template.Spec.NodeSelector)
				require.Equal(test, tolerations, statefulSet.Spec.Template.Spec.Tolerations)
				require.Len(test, statefulSet.Spec.VolumeClaimTemplates, 1)
				require.Equal(test, storageClass, statefulSet.Spec.VolumeClaimTemplates[0].Spec.StorageClassName)
				if workload == "arm-processor" {
					require.Equal(test, "4Gi", statefulSet.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests.Storage().String())
				}
			},
		})
	}

	testhelpers.RunTestCasesE(suiteTest.T(), suiteTest.chartPath, suiteTest.release, suiteTest.namespace, suiteTest.templates, testCases)
}
