// Copyright 2026 Camunda Services GmbH
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
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/helm"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

func (s *ConfigMapWarningsTemplateTest) TestGlobalImageDeprecationWhenKeysSet() {
	keys := []string{"tag", "repository", "digest"}
	testCases := []testhelpers.TestCase{}
	for _, scenario := range []struct {
		name string
		keys []string
	}{
		{name: "Unset"},
		{name: "Tag", keys: []string{"tag"}},
		{name: "Repository", keys: []string{"repository"}},
		{name: "Digest", keys: []string{"digest"}},
		{name: "AllKeys", keys: keys},
		{name: "EmptyTag", keys: []string{"tag"}},
	} {
		values := map[string]string{
			"orchestration.data.secondaryStorage.type": "elasticsearch",
			"orchestration.replicasx":                  "warning-anchor",
		}
		for _, key := range scenario.keys {
			values["global.image."+key] = "legacy"
		}
		if scenario.name == "EmptyTag" {
			values["global.image.tag"] = ""
		}
		testCases = append(testCases, testhelpers.TestCase{
			Name:   scenario.name,
			Values: values,
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(t, output, &configmap)
				warnings := configmap.Data["warnings"]
				s.Contains(warnings, "UNKNOWN VALUES KEY: orchestration.replicasx.")
				s.NotContains(warnings, "UNKNOWN VALUES KEY: global.image.")
				for _, key := range keys {
					_, set := values["global.image."+key]
					set = set && key != "digest"
					marker := `DEPRECATION: The Helm values file key "global.image.` + key + `"`
					s.Equal(map[bool]int{false: 0, true: 1}[set], strings.Count(warnings, marker), key)
					if !set {
						continue
					}
					_, tail, _ := strings.Cut(warnings, marker)
					message, _, _ := strings.Cut(tail, "\n")
					s.Contains(message, "chart v17 (Camunda 8.12)", key)
					s.Contains(message, "orchestration.image."+key, key)
					if key == "tag" {
						s.Contains(message, "ADR 0042", key)
					} else {
						s.NotContains(message, "0042", key)
					}
				}
			},
		})
	}
	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *ConfigMapWarningsTemplateTest) TestGlobalImageRenderingWhenComponentOverrides() {
	testCases := []testhelpers.TestCase{}
	for _, scenario := range []struct {
		name   string
		values map[string]string
		image  string
	}{
		{name: "GlobalTagFallback", values: map[string]string{"global.image.tag": "legacy-tag", "orchestration.image.tag": ""}, image: "camunda/camunda:legacy-tag"},
		{name: "ComponentTagWins", values: map[string]string{"global.image.tag": "legacy-tag", "orchestration.image.tag": "component-tag"}, image: "camunda/camunda:component-tag"},
		{name: "GlobalRepositoryFallback", values: map[string]string{"global.image.repository": "legacy/camunda", "orchestration.image.repository": "", "orchestration.image.tag": "component-tag"}, image: "legacy/camunda:component-tag"},
		{name: "ComponentRepositoryWins", values: map[string]string{"global.image.repository": "legacy/camunda", "orchestration.image.repository": "custom/camunda", "orchestration.image.tag": "component-tag"}, image: "custom/camunda:component-tag"},
		{name: "GlobalDigestFallback", values: map[string]string{"global.image.digest": "sha256:aaa111", "orchestration.image.digest": ""}, image: "camunda/camunda@sha256:aaa111"},
		{name: "ComponentDigestWins", values: map[string]string{"global.image.digest": "sha256:aaa111", "orchestration.image.digest": "sha256:bbb222"}, image: "camunda/camunda@sha256:bbb222"},
	} {
		scenario.values["orchestration.data.secondaryStorage.type"] = "elasticsearch"
		scenario.values["global.strictValues"] = "true"
		testCases = append(testCases, testhelpers.TestCase{
			Name:   scenario.name,
			Values: scenario.values,
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				var statefulset appsv1.StatefulSet
				helm.UnmarshalK8SYaml(t, output, &statefulset)
				s.Require().NotEmpty(statefulset.Spec.Template.Spec.Containers)
				s.Equal(scenario.image, statefulset.Spec.Template.Spec.Containers[0].Image)
			},
		})
	}
	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace,
		[]string{"templates/orchestration/statefulset.yaml"}, testCases)
}
