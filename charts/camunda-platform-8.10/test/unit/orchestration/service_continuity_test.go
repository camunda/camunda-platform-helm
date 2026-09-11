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
	"path/filepath"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

type ServiceContinuityTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
	templates []string
}

func TestServiceContinuityTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &ServiceContinuityTest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
		templates: []string{
			"templates/orchestration/service.yaml",
			"templates/orchestration/service-headless.yaml",
			"templates/orchestration/statefulset.yaml",
		},
	})
}

func (s *ServiceContinuityTest) TestSharedServicesSelectBothBrokerGenerations() {
	values := map[string]string{
		"global.labels.camunda\\.io/zone":                     "legacy-zone",
		"orchestration.multiregion.mode":                      "zoned",
		"orchestration.multiregion.zone":                      "zone-a",
		"orchestration.multiregion.zones[0].name":             "zone-a",
		"orchestration.multiregion.zones[0].numberOfBrokers":  "1",
		"orchestration.multiregion.zones[0].numberOfReplicas": "1",
		"orchestration.multiregion.zones[0].priority":         "100",
		"orchestration.multiregion.keepUnzonedBrokers":        "true",
		"orchestration.multiregion.regions":                   "1",
		"orchestration.multiregion.regionId":                  "0",
	}

	output, err := testhelpers.RenderTestCaseE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testhelpers.TestCase{Values: values})
	s.Require().NoError(err)

	services := map[string]corev1.Service{}
	statefulSets := map[string]appsv1.StatefulSet{}
	for _, document := range strings.Split(output, "\n---\n") {
		if strings.Contains(document, "kind: Service\n") {
			var service corev1.Service
			helm.UnmarshalK8SYaml(s.T(), document, &service)
			services[service.Name] = service
		}
		if strings.Contains(document, "kind: StatefulSet\n") {
			var statefulSet appsv1.StatefulSet
			helm.UnmarshalK8SYaml(s.T(), document, &statefulSet)
			statefulSets[statefulSet.Name] = statefulSet
		}
	}

	s.Require().Contains(statefulSets, s.release+"-zeebe")
	s.Require().Contains(statefulSets, s.release+"-zeebe-zone-a")
	numberedLabels := labels.Set(statefulSets[s.release+"-zeebe"].Spec.Template.Labels)
	zonedLabels := labels.Set(statefulSets[s.release+"-zeebe-zone-a"].Spec.Template.Labels)
	for _, serviceName := range []string{s.release + "-zeebe-gateway", s.release + "-zeebe"} {
		s.Require().Contains(services, serviceName)
		service := services[serviceName]
		s.Require().NotEmpty(service.Spec.Selector)
		selector, selectorErr := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{MatchLabels: service.Spec.Selector})
		s.Require().NoError(selectorErr)
		s.Require().True(selector.Matches(numberedLabels), serviceName)
		s.Require().True(labels.SelectorFromSet(service.Spec.Selector).Matches(zonedLabels), serviceName)
		s.Require().NotContains(service.Spec.Selector, "camunda.io/zone")
	}
	s.Require().Contains(services, s.release+"-zeebe-zone-a")
	zonedServiceSelector := labels.SelectorFromSet(services[s.release+"-zeebe-zone-a"].Spec.Selector)
	s.Require().True(zonedServiceSelector.Matches(zonedLabels))
	s.Require().False(zonedServiceSelector.Matches(numberedLabels))
}
