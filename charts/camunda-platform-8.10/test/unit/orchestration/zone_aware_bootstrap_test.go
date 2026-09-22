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
	"fmt"
	"strings"
	"testing"

	"camunda-platform/test/unit/testhelpers"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
)

func (s *ConfigmapTemplateTest) TestMultiZoneBootstrapEndsWithCleanup() {
	for _, keep := range []bool{true, false} {
		testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, []testhelpers.TestCase{{
			Name: fmt.Sprintf("retention-%t", keep),
			Values: map[string]string{
				"orchestration.partitioning.scheme":                    "zone-aware",
				"orchestration.partitioning.zone":                      "zone-a",
				"orchestration.partitioning.keepUnzonedBrokers":        fmt.Sprint(keep),
				"orchestration.partitioning.numberOfZones":             "1",
				"orchestration.partitioning.zoneIndex":                 "0",
				"orchestration.partitioning.zones[0].name":             "zone-a",
				"orchestration.partitioning.zones[0].numberOfBrokers":  "2",
				"orchestration.partitioning.zones[0].numberOfReplicas": "2",
				"orchestration.partitioning.zones[0].priority":         "100",
				"orchestration.partitioning.zones[1].name":             "remote-zone",
				"orchestration.partitioning.zones[1].numberOfBrokers":  "1",
				"orchestration.partitioning.zones[1].numberOfReplicas": "1",
				"orchestration.partitioning.zones[1].priority":         "90",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				found := false
				for _, document := range strings.Split(output, "\n---\n") {
					var config corev1.ConfigMap
					helm.UnmarshalK8SYaml(t, document, &config)
					if config.Name != s.release+"-zeebe-zone-a-configuration" {
						continue
					}
					found = true
					var application struct {
						Camunda struct {
							Cluster struct {
								Contacts []string `yaml:"initial-contact-points"`
							} `yaml:"cluster"`
						} `yaml:"camunda"`
					}
					require.NoError(t, yaml.Unmarshal([]byte(config.Data["application.yaml"]), &application))
					want := []string{
						s.release + "-zeebe-zone-a-0." + s.release + "-zeebe-zone-a:26502",
						s.release + "-zeebe-zone-a-1." + s.release + "-zeebe-zone-a:26502",
					}
					if keep {
						for i := 0; i < 3; i++ {
							want = append(want, fmt.Sprintf("%s-zeebe-%d.%s-zeebe:26502", s.release, i, s.release))
						}
					} else {
						want = nil
					}
					require.Equal(t, want, application.Camunda.Cluster.Contacts)
				}
				require.True(t, found, "zoned ConfigMap must exist")
			},
		}})
	}
}

func (s *StatefulSetTest) TestMultiZoneCleanupPreservesPodTemplateAndContactOverride() {
	values := map[string]string{
		"orchestration.partitioning.scheme":                    "zone-aware",
		"orchestration.partitioning.zone":                      "zone-a",
		"orchestration.partitioning.keepUnzonedBrokers":        "true",
		"orchestration.partitioning.numberOfZones":             "1",
		"orchestration.partitioning.zoneIndex":                 "0",
		"orchestration.partitioning.zones[0].name":             "zone-a",
		"orchestration.partitioning.zones[0].numberOfBrokers":  "2",
		"orchestration.partitioning.zones[0].numberOfReplicas": "2",
		"orchestration.partitioning.zones[0].priority":         "100",
		"orchestration.partitioning.zones[1].name":             "zone-b",
		"orchestration.partitioning.zones[1].numberOfBrokers":  "1",
		"orchestration.partitioning.zones[1].numberOfReplicas": "1",
		"orchestration.partitioning.zones[1].priority":         "90",
		"orchestration.env[0].name":                            "CAMUNDA_CLUSTER_INITIALCONTACTPOINTS",
		"orchestration.env[0].value":                           "remote.example:26502",
	}
	before := s.renderStatefulSet(values, s.release+"-zeebe-zone-a")
	values["orchestration.partitioning.keepUnzonedBrokers"] = "false"
	after := s.renderStatefulSet(values, s.release+"-zeebe-zone-a")
	s.Require().NotEmpty(before.Spec.Template.Annotations["checksum/config"])
	s.Require().Equal(before.Spec.Template, after.Spec.Template)
	s.Require().Contains(after.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{
		Name: "CAMUNDA_CLUSTER_INITIALCONTACTPOINTS", Value: "remote.example:26502",
	})
}
