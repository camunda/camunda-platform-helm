package testutils

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
)

type DocumentStoreConfigMapTest struct {
    suite.Suite
    ChartPath string
    Release   string
    Namespace string
    Templates []string
}

type TestCase struct {
    Name     string
    Values   map[string]string
    Expected map[string]string
}
func TestCreateTestSuite(t *testing.T, templates []string) {
    t.Parallel()

    chartPath, err := filepath.Abs("../../../")
    require.NoError(t, err)

    suite.Run(t, &DocumentStoreConfigMapTest{
        ChartPath: chartPath,
        Release:   "camunda-platform-test",
        Namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
        Templates: templates,
    })
}

func (s *DocumentStoreConfigMapTest) RenderTemplate(options *helm.Options) corev1.ConfigMap {
    output := helm.RenderTemplate(s.T(), options, s.ChartPath, s.Release, s.Templates)
    var configmap corev1.ConfigMap
    helm.UnmarshalK8SYaml(s.T(), output, &configmap)
    return configmap
}

func (s *DocumentStoreConfigMapTest) VerifyConfigMap(testCase string, configmap corev1.ConfigMap, expectedValues map[string]string) {
    for key, expectedValue := range expectedValues {
        actualValue := strings.TrimSpace(configmap.Data[key])
        s.Require().Equal(expectedValue, actualValue, "Test case '%s': Expected key '%s' to have value '%s', but got '%s'", testCase, key, expectedValue, actualValue)
    }
}

func (s *DocumentStoreConfigMapTest) RunTestCases(testCases []TestCase) {
    for _, tc := range testCases {
        s.Run(tc.Name, func() {
            // given
            options := &helm.Options{
                SetValues:      tc.Values,
                KubectlOptions: k8s.NewKubectlOptions("", "", s.Namespace),
            }

            // when
            configmap := s.RenderTemplate(options)

            // then
            s.VerifyConfigMap(tc.Name, configmap, tc.Expected)
        })
    }
}