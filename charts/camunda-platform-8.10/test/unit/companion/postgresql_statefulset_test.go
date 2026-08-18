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
	"flag"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

var _ = flag.Bool("update-golden", false, "accepted for chart-wide golden updates")

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

func (s *PostgresqlStatefulSetTest) render(values map[string]string) appsv1.StatefulSet {
	s.T().Helper()

	output := helm.RenderTemplate(
		s.T(),
		&helm.Options{
			SetValues:      values,
			KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
		},
		s.chartPath,
		s.release,
		s.templates,
	)

	var statefulSet appsv1.StatefulSet
	helm.UnmarshalK8SYaml(s.T(), output, &statefulSet)
	return statefulSet
}

func (s *PostgresqlStatefulSetTest) dataVolume(statefulSet appsv1.StatefulSet) *corev1.Volume {
	for i := range statefulSet.Spec.Template.Spec.Volumes {
		if statefulSet.Spec.Template.Spec.Volumes[i].Name == "data" {
			return &statefulSet.Spec.Template.Spec.Volumes[i]
		}
	}
	return nil
}

func (s *PostgresqlStatefulSetTest) TestPersistenceEnabledByDefaultUsesVolumeClaimTemplate() {
	statefulSet := s.render(map[string]string{})

	s.Require().Equal("StatefulSet", statefulSet.Kind)
	s.Require().NotEmpty(statefulSet.Spec.ServiceName)
	s.Require().Len(statefulSet.Spec.VolumeClaimTemplates, 1)

	claim := statefulSet.Spec.VolumeClaimTemplates[0]
	s.Require().Equal("data", claim.Name)
	s.Require().Equal([]corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}, claim.Spec.AccessModes)
	s.Require().Equal("1Gi", claim.Spec.Resources.Requests.Storage().String())
	s.Require().Nil(claim.Spec.StorageClassName)

	s.Require().Nil(s.dataVolume(statefulSet), "data must come from the claim template, not a pod volume")
}

func (s *PostgresqlStatefulSetTest) TestPersistenceDisabledFallsBackToEmptyDir() {
	statefulSet := s.render(map[string]string{"persistence.enabled": "false"})

	s.Require().Empty(statefulSet.Spec.VolumeClaimTemplates)

	volume := s.dataVolume(statefulSet)
	s.Require().NotNil(volume)
	s.Require().NotNil(volume.EmptyDir)
}

func (s *PostgresqlStatefulSetTest) TestPersistenceStorageClassIsRendered() {
	statefulSet := s.render(map[string]string{
		"persistence.storageClass": "hyperdisk-balanced",
		"persistence.size":         "2Gi",
	})

	s.Require().Len(statefulSet.Spec.VolumeClaimTemplates, 1)
	claim := statefulSet.Spec.VolumeClaimTemplates[0]
	s.Require().NotNil(claim.Spec.StorageClassName)
	s.Require().Equal("hyperdisk-balanced", *claim.Spec.StorageClassName)
	s.Require().Equal("2Gi", claim.Spec.Resources.Requests.Storage().String())
}
