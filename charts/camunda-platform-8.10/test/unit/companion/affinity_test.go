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

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

// antiAffinityValues sets a preferred host anti-affinity under prefix, the
// shape the dogfood companion values use.
func antiAffinityValues(prefix string) map[string]string {
	term := prefix + "podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[0]."
	return map[string]string{
		term + "weight":                                        "100",
		term + "podAffinityTerm.topologyKey":                   "kubernetes.io/hostname",
		term + "podAffinityTerm.labelSelector.matchLabels.app": "dogfood",
	}
}

func assertAntiAffinity(t *testing.T, affinity *corev1.Affinity) {
	t.Helper()
	require.NotNil(t, affinity, "affinity must be rendered into the pod spec")
	require.NotNil(t, affinity.PodAntiAffinity)
	terms := affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution
	require.Len(t, terms, 1)
	require.Equal(t, int32(100), terms[0].Weight)
	require.Equal(t, "kubernetes.io/hostname", terms[0].PodAffinityTerm.TopologyKey)
	require.Equal(t, map[string]string{"app": "dogfood"}, terms[0].PodAffinityTerm.LabelSelector.MatchLabels)
}

func TestCompanionAffinityRendering(t *testing.T) {
	t.Parallel()

	keycloakChart, err := filepath.Abs("../../../../internal-keycloak-26")
	require.NoError(t, err)
	postgresqlChart, err := filepath.Abs("../../../../internal-postgresql")
	require.NoError(t, err)
	namespace := "camunda-platform-" + strings.ToLower(random.UniqueId())

	t.Run("KeycloakDeployment", func(t *testing.T) {
		testhelpers.RunTestCasesE(t, keycloakChart, "keycloak-test", namespace, []string{"templates/deployment.yaml"}, []testhelpers.TestCase{
			{
				Name: "AbsentByDefault",
				Verifier: func(t *testing.T, output string, err error) {
					require.NoError(t, err)
					var deployment appsv1.Deployment
					helm.UnmarshalK8SYaml(t, output, &deployment)
					require.Nil(t, deployment.Spec.Template.Spec.Affinity)
				},
			},
			{
				Name:   "TopLevelAffinityRendered",
				Values: antiAffinityValues("affinity."),
				Verifier: func(t *testing.T, output string, err error) {
					require.NoError(t, err)
					var deployment appsv1.Deployment
					helm.UnmarshalK8SYaml(t, output, &deployment)
					assertAntiAffinity(t, deployment.Spec.Template.Spec.Affinity)
				},
			},
		})
	})

	t.Run("KeycloakPostgresqlStatefulSet", func(t *testing.T) {
		testhelpers.RunTestCasesE(t, keycloakChart, "keycloak-test", namespace, []string{"templates/postgresql-statefulset.yaml"}, []testhelpers.TestCase{
			{
				Name: "AbsentByDefault",
				Verifier: func(t *testing.T, output string, err error) {
					require.Nil(t, unmarshalStatefulSet(t, output, err).Spec.Template.Spec.Affinity)
				},
			},
			{
				Name:   "PostgresqlAffinityRendered",
				Values: antiAffinityValues("postgresql.affinity."),
				Verifier: func(t *testing.T, output string, err error) {
					assertAntiAffinity(t, unmarshalStatefulSet(t, output, err).Spec.Template.Spec.Affinity)
				},
			},
		})
	})

	t.Run("PostgresqlStatefulSet", func(t *testing.T) {
		testhelpers.RunTestCasesE(t, postgresqlChart, "postgresql-test", namespace, []string{"templates/statefulset.yaml"}, []testhelpers.TestCase{
			{
				Name:   "AffinityRendered",
				Values: antiAffinityValues("affinity."),
				Verifier: func(t *testing.T, output string, err error) {
					assertAntiAffinity(t, unmarshalStatefulSet(t, output, err).Spec.Template.Spec.Affinity)
				},
			},
		})
	})
}
