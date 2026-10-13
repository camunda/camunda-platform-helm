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
	corev1 "k8s.io/api/core/v1"
)

func (s *ConfigMapWarningsTemplateTest) TestGlobalImageMigrationWhenExactKeysSet() {
	for _, field := range []string{"tag", "repository"} {
		for _, target := range []struct {
			path     string
			template string
		}{
			{"orchestration.image", "orchestration/statefulset.yaml"},
			{"connectors.image", "connectors/deployment.yaml"},
			{"identity.image", "identity/deployment.yaml"},
			{"optimize.image", "optimize/deployment.yaml"},
			{"webModeler.restapi.image", "web-modeler/deployment-restapi.yaml"},
			{"webModeler.websockets.image", "web-modeler/deployment-websockets.yaml"},
			{"camundaHub.restapi.image", "web-modeler/deployment-restapi.yaml"},
			{"camundaHub.websockets.image", "web-modeler/deployment-websockets.yaml"},
			{"webModeler.image", "web-modeler/deployment-restapi.yaml"},
			{"camundaHub.image", "web-modeler/deployment-restapi.yaml"},
		} {
			if field == "repository" && (target.path == "webModeler.image" || target.path == "camundaHub.image") {
				continue
			}
			key := target.path + "." + field
			warningValues := map[string]string{
				"global.image." + field:                    "legacy",
				"orchestration.data.secondaryStorage.type": "elasticsearch",
			}
			testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, []testhelpers.TestCase{{
				Name: "WarningNames_" + key, Values: warningValues,
				Verifier: func(t *testing.T, output string, err error) {
					s.Require().NoError(err)
					var configmap corev1.ConfigMap
					helm.UnmarshalK8SYaml(t, output, &configmap)
					s.Contains(configmap.Data["warnings"], `"`+key+`"`)
				},
			}})
			testCases := []testhelpers.TestCase{}
			for _, mode := range []string{"Fallback", "Override", "Migrated"} {
				values := map[string]string{
					"orchestration.data.secondaryStorage.type": "elasticsearch",
					"identity.enabled":                         "true", "optimize.enabled": "true", "camundaHub.enabled": "true",
					"camundaHub.restapi.mail.fromAddress": "example@example.com",
					"global.strictValues":                 "true",
				}
				repository, tag := "example/component", "component-tag"
				component := strings.Split(target.path, ".")[0]
				values[component+".image.tag"] = tag
				repositoryPath := target.path
				if target.path == "webModeler.image" || target.path == "camundaHub.image" {
					repositoryPath = component + ".restapi.image"
				}
				values[repositoryPath+".repository"] = repository
				values[key] = ""
				if field == "tag" {
					tag = "legacy-tag"
					values[component+".image.tag"] = ""
					values[key] = "null"
					if strings.Count(target.path, ".") == 1 {
						values[key] = ""
					}
					if component == "webModeler" && strings.Count(target.path, ".") == 2 {
						delete(values, key)
					}
					values["global.image.tag"] = tag
				} else {
					repository = "example/legacy"
					values["global.image.repository"] = repository
				}
				switch mode {
				case "Fallback":
				case "Override", "Migrated":
					if field == "tag" {
						tag = "component-tag"
						values[key] = tag
					} else {
						repository = "example/component"
						values[key] = repository
					}
					if mode == "Migrated" {
						delete(values, "global.image."+field)
						if field == "tag" && strings.Count(target.path, ".") == 2 {
							values[component+".image.tag"] = "sibling-tag"
						}
					}
				}
				expected := map[string]string{"spec.template.spec.containers[0].image": repository + ":" + tag}
				if target.path == "orchestration.image" {
					values["global.tls.caBundle.secret.existingSecret"] = "custom-ca"
					expected["spec.template.spec.initContainers[?(@.name=='ca-bundle-truststore-init')].image"] = repository + ":" + tag
				}
				testCases = append(testCases, testhelpers.TestCase{
					Name: mode + "_" + key, Values: values,
					Expected: expected,
				})
			}
			testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace,
				[]string{"templates/" + target.template}, testCases)
		}
	}
}
