package testhelpers

import (
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

type TestCase struct {
	Name     string
	Values   map[string]string
	Expected map[string]string
}

// RenderTemplate renders the specified Helm templates into a Kubernetes ConfigMap
func RenderTemplate(t *testing.T, chartPath, release string, namespace string, templates []string, values map[string]string) corev1.ConfigMap {
	options := &helm.Options{
		SetValues:      values,
		KubectlOptions: k8s.NewKubectlOptions("", "", namespace),
	}

	output := helm.RenderTemplate(t, options, chartPath, release, templates)
	var configmap corev1.ConfigMap
	helm.UnmarshalK8SYaml(t, output, &configmap)
	return configmap
}

// VerifyConfigMap checks whether the generated ConfigMap contains expected key-value pairs
func VerifyConfigMap(t *testing.T, testCase string, configmap corev1.ConfigMap, expectedValues map[string]string) {
	for key, expectedValue := range expectedValues {
		actualValue := strings.TrimSpace(configmap.Data[key])
		require.Equal(t, expectedValue, actualValue, "Test case '%s': Expected key '%s' to have value '%s', but got '%s'", testCase, key, expectedValue, actualValue)
	}
}

// RunTestCases executes multiple test cases using the provided Helm chart and ConfigMap validation
func RunTestCases(t *testing.T, chartPath, release, namespace string, templates []string, testCases []TestCase) {
	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			configmap := RenderTemplate(t, chartPath, release, namespace, templates, tc.Values)
			VerifyConfigMap(t, tc.Name, configmap, tc.Expected)
		})
	}
}
