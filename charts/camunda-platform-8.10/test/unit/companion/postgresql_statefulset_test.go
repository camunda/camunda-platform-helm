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

type PostgresqlStatefulSetTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
	templates []string
}

func TestPostgresqlStatefulSetTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../../internal-postgresql")
	require.NoError(t, err)

	suite.Run(t, &PostgresqlStatefulSetTest{
		chartPath: chartPath,
		release:   "postgresql-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
		templates: []string{"templates/statefulset.yaml"},
	})
}

func (s *PostgresqlStatefulSetTest) TestPersistenceDifferentValuesInputs() {
	testCases := []testhelpers.TestCase{
		{
			Name:   "TestPersistenceEnabledByDefaultUsesVolumeClaimTemplate",
			Values: map[string]string{},
			Verifier: func(t *testing.T, output string, err error) {
				statefulSet := unmarshalStatefulSet(t, output, err)

				require.Equal(t, "StatefulSet", statefulSet.Kind)
				require.NotEmpty(t, statefulSet.Spec.ServiceName)
				require.Len(t, statefulSet.Spec.VolumeClaimTemplates, 1)

				claim := statefulSet.Spec.VolumeClaimTemplates[0]
				require.Equal(t, "data", claim.Name)
				require.Equal(t, []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}, claim.Spec.AccessModes)
				require.Equal(t, "1Gi", claim.Spec.Resources.Requests.Storage().String())
				require.Nil(t, claim.Spec.StorageClassName)

				require.Nil(t, podVolume(statefulSet, "data"),
					"data must come from the claim template, not a pod volume")

				require.Equal(t, "statefulset", statefulSet.Spec.Template.Labels["camunda.io/controller"])
				require.NotContains(t, statefulSet.Spec.Selector.MatchLabels, "camunda.io/controller",
					"spec.selector must stay free of the discriminator; it is immutable after creation")
			},
		}, {
			Name: "TestPersistenceDisabledFallsBackToEmptyDir",
			Values: map[string]string{
				"persistence.enabled": "false",
			},
			Verifier: func(t *testing.T, output string, err error) {
				statefulSet := unmarshalStatefulSet(t, output, err)

				require.Empty(t, statefulSet.Spec.VolumeClaimTemplates)

				volume := podVolume(statefulSet, "data")
				require.NotNil(t, volume)
				require.NotNil(t, volume.EmptyDir)
			},
		}, {
			Name: "TestPersistenceStorageClassIsRendered",
			Values: map[string]string{
				"persistence.storageClass": "hyperdisk-balanced",
				"persistence.size":         "2Gi",
			},
			Verifier: func(t *testing.T, output string, err error) {
				statefulSet := unmarshalStatefulSet(t, output, err)

				require.Len(t, statefulSet.Spec.VolumeClaimTemplates, 1)
				claim := statefulSet.Spec.VolumeClaimTemplates[0]
				require.NotNil(t, claim.Spec.StorageClassName)
				require.Equal(t, "hyperdisk-balanced", *claim.Spec.StorageClassName)
				require.Equal(t, "2Gi", claim.Spec.Resources.Requests.Storage().String())
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}
