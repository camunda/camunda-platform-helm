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
