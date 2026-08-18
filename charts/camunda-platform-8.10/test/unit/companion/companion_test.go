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
	"testing"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

var _ = flag.Bool("update-golden", false, "accepted for chart-wide golden updates")

// unmarshalStatefulSet fails the test case when rendering errored, then decodes
// the rendered manifest.
func unmarshalStatefulSet(t *testing.T, output string, err error) appsv1.StatefulSet {
	t.Helper()
	require.NoError(t, err)

	var statefulSet appsv1.StatefulSet
	helm.UnmarshalK8SYaml(t, output, &statefulSet)
	return statefulSet
}

// podVolume returns the named pod-level volume, or nil when the StatefulSet does
// not declare one. A nil result for "data" means the volume comes from a
// volumeClaimTemplate instead.
func podVolume(statefulSet appsv1.StatefulSet, name string) *corev1.Volume {
	for i := range statefulSet.Spec.Template.Spec.Volumes {
		if statefulSet.Spec.Template.Spec.Volumes[i].Name == name {
			return &statefulSet.Spec.Template.Spec.Volumes[i]
		}
	}
	return nil
}
