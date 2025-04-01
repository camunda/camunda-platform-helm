package testhelpers

import (
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
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

// VerifyConfigMap checks whether the generated ConfigMap contains the expected key-value pairs
func VerifyConfigMap(t *testing.T, testCase string, configmap corev1.ConfigMap, expectedValues map[string]string) {
	for keyPath, expectedValue := range expectedValues {
		var actualValue string
		if strings.HasPrefix(keyPath, "configmapApplication.") {
            var configmapApplication map[string]interface{}
            err := yaml.Unmarshal([]byte(configmap.Data["application.yaml"]), &configmapApplication)
            require.NoError(t, err)
            actualValue = getConfigMapFieldValue(configmapApplication, strings.Split(keyPath, ".")[1:])
        } else {
            actualValue = strings.TrimSpace(configmap.Data[keyPath])
        }
		require.Equal(t, expectedValue, actualValue, "Test case '%s': Expected key '%s' to have value '%s', but got '%s'", testCase, keyPath, expectedValue, actualValue)
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

// getConfigMapFieldValue function traverses a nested map structure based on a given key path.
// It handles maps with both interface{} and string keys, converting them as necessary to retrieve the desired value.
// If the key is not found or the final value is not a string, the function returns an empty string.
func getConfigMapFieldValue(configmapApplication map[string]interface{}, keyPath []string) string {
	var current interface{} = configmapApplication

	for _, key := range keyPath {
        if nestedMap, ok := current.(map[interface{}]interface{}); ok {
            // Convert map[interface{}]interface{} to map[string]interface{}
            stringMap := make(map[string]interface{})
            for k, v := range nestedMap {
                if strKey, isString := k.(string); isString {
                    stringMap[strKey] = v
                }
            }
            // Move to the next level in the map
            current = stringMap[key]
        } else if nestedMap, ok := current.(map[string]interface{}); ok {
            // If the current level is already a map with string keys, move to the next level
            current = nestedMap[key]
        } else {
            // If the key is not found, return an empty string
            return ""
        }
    }

    // If the final value is a string, return it
    if value, ok := current.(string); ok {
        return value
    }
    // If the final value is not a string, return an empty string
    return ""
}
