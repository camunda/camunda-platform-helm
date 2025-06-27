package core

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

type CorePersistenceTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
	templates []string
}

func TestCorePersistence(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &CorePersistenceTest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
		templates: []string{"templates/camunda/persistence.yaml"},
	})
}

func (s *CorePersistenceTest) TestCorePVCCreation() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestNoPVCWhenCoreDisabled",
			Values: map[string]string{
				"core.enabled": "false",
				"webModeler.restapi.mail.fromAddress": "test@example.com",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().Empty(output, "no PVC should be created when core is disabled")
			},
		},
		{
			Name: "TestNoPVCWhenCorePersistenceDisabled",
			Values: map[string]string{
				"core.enabled": "true",
				"webModeler.restapi.mail.fromAddress": "test@example.com",
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
				"webModeler.restapi.mail.fromAddress": "test@example.com",
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
				"webModeler.restapi.mail.fromAddress": "test@example.com",
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
				"webModeler.restapi.mail.fromAddress": "test@example.com",
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
				"webModeler.restapi.mail.fromAddress": "test@example.com",
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
				"webModeler.restapi.mail.fromAddress": "test@example.com",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().Empty(output, "no PVC should be created when using existing claim for core")
			},
		},
		{
			Name: "TestNoPVCWithOnlyEmptyDir",
			Values: map[string]string{
				"core.enabled": "true",
				"core.persistence.emptyDir": "{}",
				"webModeler.restapi.mail.fromAddress": "test@example.com",
			},
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().Empty(output, "no PVC should be created when only emptyDir is set for core")
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
