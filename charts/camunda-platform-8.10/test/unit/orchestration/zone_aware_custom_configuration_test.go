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

package orchestration

import (
	"camunda-platform/test/unit/testhelpers"
	"maps"
	"testing"

	"github.com/gruntwork-io/terratest/modules/helm"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
)

const customApplication = "custom:\n  owner: operator\n"

func (s *ConfigmapTemplateTest) TestZonedCustomConfigurationOwnership() {
	zonedValues := map[string]string{
		"orchestration.partitioning.scheme":                    "zone-aware",
		"orchestration.partitioning.zone":                      "zone-a",
		"orchestration.partitioning.zones[0].name":             "zone-a",
		"orchestration.partitioning.zones[0].numberOfBrokers":  "2",
		"orchestration.partitioning.zones[0].numberOfReplicas": "2",
		"orchestration.partitioning.zones[0].priority":         "100",
		"orchestration.profiles.broker":                        "true",
	}

	testCases := []testhelpers.TestCase{
		{
			Name: "FullConfigurationReplacesGeneratedApplicationYaml",
			Values: func() map[string]string {
				values := maps.Clone(zonedValues)
				values["orchestration.configuration"] = customApplication
				return values
			}(),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)

				var configMap corev1.ConfigMap
				helm.UnmarshalK8SYaml(t, output, &configMap)

				s.Require().Equal(customApplication, configMap.Data["application.yaml"])
			},
		},
		{
			Name: "ExtraConfigurationKeepsGeneratedZoneConfiguration",
			Values: func() map[string]string {
				values := maps.Clone(zonedValues)
				values["orchestration.extraConfiguration[0].file"] = "operator.yaml"
				values["orchestration.extraConfiguration[0].content"] = customApplication
				return values
			}(),
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)

				var configMap corev1.ConfigMap
				helm.UnmarshalK8SYaml(t, output, &configMap)

				var application struct {
					Camunda struct {
						Cluster struct {
							Partitioning struct {
								Scheme    string `yaml:"scheme"`
								ZoneAware struct {
									Zones []struct {
										Name string `yaml:"name"`
									} `yaml:"zones"`
								} `yaml:"zone-aware"`
							} `yaml:"partitioning"`
						} `yaml:"cluster"`
					} `yaml:"camunda"`
				}
				s.Require().NoError(yaml.Unmarshal([]byte(configMap.Data["application.yaml"]), &application))

				var operatorConfig struct {
					Custom struct {
						Owner string `yaml:"owner"`
					} `yaml:"custom"`
				}
				s.Require().NoError(yaml.Unmarshal([]byte(configMap.Data["operator.yaml"]), &operatorConfig))

				s.Require().Equal("ZONE_AWARE", application.Camunda.Cluster.Partitioning.Scheme)
				s.Require().Len(application.Camunda.Cluster.Partitioning.ZoneAware.Zones, 1)
				s.Require().Equal("zone-a", application.Camunda.Cluster.Partitioning.ZoneAware.Zones[0].Name)
				s.Require().Equal("operator", operatorConfig.Custom.Owner)
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}
