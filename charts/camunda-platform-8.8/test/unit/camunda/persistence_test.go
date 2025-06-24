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

package camunda

import (
	"camunda-platform/test/unit/testhelpers"
	"path/filepath"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type PersistenceTemplateTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
	templates []string
}

func TestPersistenceTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &PersistenceTemplateTest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
		templates: []string{"templates/camunda/persistence.yaml"},
	})
}

func (s *PersistenceTemplateTest) TestPVCCreation() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestNoPVCWhenConnectorsDisabled",
			Values: map[string]string{
				"connectors.enabled": "false",
				// persistence.enabled defaults to false
			},
			Verifier: func(t *testing.T, output string, err error) {
				// No PVC should be created when component is disabled
				s.Require().Empty(output, "no PVC should be created when component is disabled")
			},
		},
		{
			Name: "TestNoPVCWhenPersistenceDisabled",
			Values: map[string]string{
				"connectors.enabled": "true",
				// persistence.enabled defaults to false
			},
			Verifier: func(t *testing.T, output string, err error) {
				// No PVC should be created when persistence is disabled
				s.Require().Empty(output, "no PVC should be created when persistence is disabled")
			},
		},
		{
			Name: "TestPVCCreatedWhenPersistenceEnabled",
			Values: map[string]string{
				"connectors.enabled":                    "true",
				"connectors.persistence.enabled":        "true",
				"connectors.persistence.size":           "5Gi",
				"connectors.persistence.accessModes[0]": "ReadWriteOnce",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var pvc corev1.PersistentVolumeClaim
				helm.UnmarshalK8SYaml(s.T(), output, &pvc)
				// then
				s.Require().Equal("camunda-platform-test-connectors-data", pvc.Name)
				s.Require().Equal("5Gi", pvc.Spec.Resources.Requests.Storage().String())
				s.Require().Equal(corev1.ReadWriteOnce, pvc.Spec.AccessModes[0])
			},
		},
		{
			Name: "TestPVCCreatedForCore",
			Values: map[string]string{
				"core.enabled":                    "true",
				"core.persistence.enabled":        "true",
				"core.persistence.size":           "10Gi",
				"core.persistence.accessModes[0]": "ReadWriteOnce",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var pvc corev1.PersistentVolumeClaim
				helm.UnmarshalK8SYaml(s.T(), output, &pvc)
				// then
				s.Require().Equal("camunda-platform-test-core-data", pvc.Name)
				s.Require().Equal("10Gi", pvc.Spec.Resources.Requests.Storage().String())
				s.Require().Equal(corev1.ReadWriteOnce, pvc.Spec.AccessModes[0])
			},
		},
		{
			Name: "TestPVCCreatedForIdentity",
			Values: map[string]string{
				"identity.enabled":                    "true",
				"identity.persistence.enabled":        "true",
				"identity.persistence.size":           "3Gi",
				"identity.persistence.accessModes[0]": "ReadWriteOnce",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var pvc corev1.PersistentVolumeClaim
				helm.UnmarshalK8SYaml(s.T(), output, &pvc)
				// then
				s.Require().Equal("camunda-platform-test-identity-data", pvc.Name)
				s.Require().Equal("3Gi", pvc.Spec.Resources.Requests.Storage().String())
				s.Require().Equal(corev1.ReadWriteOnce, pvc.Spec.AccessModes[0])
			},
		},
		{
			Name: "TestPVCCreatedForOptimize",
			Values: map[string]string{
				"optimize.enabled":                    "true",
				"optimize.persistence.enabled":        "true",
				"optimize.persistence.size":           "8Gi",
				"optimize.persistence.accessModes[0]": "ReadWriteOnce",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var pvc corev1.PersistentVolumeClaim
				helm.UnmarshalK8SYaml(s.T(), output, &pvc)
				// then
				s.Require().Equal("camunda-platform-test-optimize-data", pvc.Name)
				s.Require().Equal("8Gi", pvc.Spec.Resources.Requests.Storage().String())
				s.Require().Equal(corev1.ReadWriteOnce, pvc.Spec.AccessModes[0])
			},
		},
		{
			Name: "TestPVCWithStorageClass",
			Values: map[string]string{
				"connectors.enabled":                    "true",
				"connectors.persistence.enabled":        "true",
				"connectors.persistence.size":           "5Gi",
				"connectors.persistence.storageClassName": "fast-ssd",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var pvc corev1.PersistentVolumeClaim
				helm.UnmarshalK8SYaml(s.T(), output, &pvc)
				// then
				s.Require().Equal("camunda-platform-test-connectors-data", pvc.Name)
				s.Require().Equal("fast-ssd", *pvc.Spec.StorageClassName)
			},
		},
		{
			Name: "TestPVCWithAnnotations",
			Values: map[string]string{
				"connectors.enabled":                    "true",
				"connectors.persistence.enabled":        "true",
				"connectors.persistence.size":           "5Gi",
				"connectors.persistence.annotations.foo": "bar",
				"connectors.persistence.annotations.baz": "qux",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var pvc corev1.PersistentVolumeClaim
				helm.UnmarshalK8SYaml(s.T(), output, &pvc)
				// then
				s.Require().Equal("camunda-platform-test-connectors-data", pvc.Name)
				s.Require().Equal("bar", pvc.Annotations["foo"])
				s.Require().Equal("qux", pvc.Annotations["baz"])
			},
		},
		{
			Name: "TestPVCWithSelector",
			Values: map[string]string{
				"connectors.enabled":                    "true",
				"connectors.persistence.enabled":        "true",
				"connectors.persistence.size":           "5Gi",
				"connectors.persistence.selector.matchLabels.storage": "fast",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var pvc corev1.PersistentVolumeClaim
				helm.UnmarshalK8SYaml(s.T(), output, &pvc)
				// then
				s.Require().Equal("camunda-platform-test-connectors-data", pvc.Name)
				s.Require().NotNil(pvc.Spec.Selector)
				s.Require().Equal("fast", pvc.Spec.Selector.MatchLabels["storage"])
			},
		},
		{
			Name: "TestNoPVCWithExistingClaim",
			Values: map[string]string{
				"connectors.enabled":                    "true",
				"connectors.persistence.enabled":        "true",
				"connectors.persistence.existingClaim":  "my-existing-pvc",
			},
			Verifier: func(t *testing.T, output string, err error) {
				// No PVC should be created when using existing claim
				s.Require().Empty(output, "no PVC should be created when using existing claim")
			},
		},
		{
			Name: "TestMultipleComponentsWithPersistence",
			Values: map[string]string{
				"connectors.enabled":                    "true",
				"connectors.persistence.enabled":        "true",
				"connectors.persistence.size":           "5Gi",
				"core.enabled":                         "true",
				"core.persistence.enabled":             "true",
				"core.persistence.size":                "10Gi",
			},
			Verifier: func(t *testing.T, output string, err error) {
				// Should create multiple PVCs
				lines := strings.Split(strings.TrimSpace(output), "\n---\n")
				s.Require().Equal(2, len(lines), "should create 2 PVCs")
				// Check connectors PVC
				var connectorsPVC corev1.PersistentVolumeClaim
				helm.UnmarshalK8SYaml(s.T(), lines[0], &connectorsPVC)
				s.Require().Equal("camunda-platform-test-connectors-data", connectorsPVC.Name)
				s.Require().Equal("5Gi", connectorsPVC.Spec.Resources.Requests.Storage().String())
				// Check core PVC
				var corePVC corev1.PersistentVolumeClaim
				helm.UnmarshalK8SYaml(s.T(), lines[1], &corePVC)
				s.Require().Equal("camunda-platform-test-core-data", corePVC.Name)
				s.Require().Equal("10Gi", corePVC.Spec.Resources.Requests.Storage().String())
			},
		},
		{
			Name: "TestNoPVCWhenCoreDisabled",
			Values: map[string]string{
				"core.enabled": "false",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().Empty(output, "no PVC should be created when core is disabled")
			},
		},
		{
			Name: "TestNoPVCWhenCorePersistenceDisabled",
			Values: map[string]string{
				"core.enabled": "true",
				// persistence.enabled defaults to false
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().Empty(output, "no PVC should be created when core persistence is disabled")
			},
		},
		{
			Name: "TestPVCCreatedWhenCorePersistenceEnabled",
			Values: map[string]string{
				"core.enabled":                    "true",
				"core.persistence.enabled":        "true",
				"core.persistence.size":           "10Gi",
				"core.persistence.accessModes[0]": "ReadWriteOnce",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var pvc corev1.PersistentVolumeClaim
				helm.UnmarshalK8SYaml(s.T(), output, &pvc)
				s.Require().Equal("camunda-platform-test-core-data", pvc.Name)
				s.Require().Equal("10Gi", pvc.Spec.Resources.Requests.Storage().String())
				s.Require().Equal(corev1.ReadWriteOnce, pvc.Spec.AccessModes[0])
			},
		},
		{
			Name: "TestCorePVCWithStorageClass",
			Values: map[string]string{
				"core.enabled":                    "true",
				"core.persistence.enabled":        "true",
				"core.persistence.size":           "10Gi",
				"core.persistence.storageClassName": "fast-ssd",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var pvc corev1.PersistentVolumeClaim
				helm.UnmarshalK8SYaml(s.T(), output, &pvc)
				s.Require().Equal("camunda-platform-test-core-data", pvc.Name)
				s.Require().Equal("fast-ssd", *pvc.Spec.StorageClassName)
			},
		},
		{
			Name: "TestCorePVCWithAnnotations",
			Values: map[string]string{
				"core.enabled":                    "true",
				"core.persistence.enabled":        "true",
				"core.persistence.size":           "10Gi",
				"core.persistence.annotations.foo": "bar",
				"core.persistence.annotations.baz": "qux",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var pvc corev1.PersistentVolumeClaim
				helm.UnmarshalK8SYaml(s.T(), output, &pvc)
				s.Require().Equal("camunda-platform-test-core-data", pvc.Name)
				s.Require().Equal("bar", pvc.Annotations["foo"])
				s.Require().Equal("qux", pvc.Annotations["baz"])
			},
		},
		{
			Name: "TestCorePVCWithSelector",
			Values: map[string]string{
				"core.enabled":                    "true",
				"core.persistence.enabled":        "true",
				"core.persistence.size":           "10Gi",
				"core.persistence.selector.matchLabels.storage": "fast",
			},
			Verifier: func(t *testing.T, output string, err error) {
				var pvc corev1.PersistentVolumeClaim
				helm.UnmarshalK8SYaml(s.T(), output, &pvc)
				s.Require().Equal("camunda-platform-test-core-data", pvc.Name)
				s.Require().NotNil(pvc.Spec.Selector)
				s.Require().Equal("fast", pvc.Spec.Selector.MatchLabels["storage"])
			},
		},
		{
			Name: "TestNoCorePVCWithExistingClaim",
			Values: map[string]string{
				"core.enabled":                    "true",
				"core.persistence.enabled":        "true",
				"core.persistence.existingClaim":  "my-existing-pvc",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().Empty(output, "no PVC should be created when using existing claim for core")
			},
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		s.Run(testCase.Name, func() {
			s.T().Parallel()
			helmChartPath, err := filepath.Abs(s.chartPath)
			s.Require().NoError(err)

			output, err := helm.RenderTemplateE(
				s.T(),
				&helm.Options{
					SetValues: testCase.Values,
				},
				helmChartPath,
				s.release,
				s.templates,
			)

			testCase.Verifier(s.T(), output, err)
		})
	}
} 