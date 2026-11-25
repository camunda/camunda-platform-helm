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
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/stretchr/testify/require"
)

func renderJdkOptionsTemplate(t *testing.T, values map[string]string, showOnly string) string {
	t.Helper()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	options := &helm.Options{
		KubectlOptions: k8s.NewKubectlOptions("", "", "camunda"),
		SetValues:      values,
	}
	return helm.RenderTemplateContext(t, context.Background(), options, chartPath, "camunda-platform-test", nil, "--show-only", showOnly)
}

func TestElasticsearchJvmOptionsFile(t *testing.T) {
	t.Parallel()

	out := renderJdkOptionsTemplate(t, nil, "charts/elasticsearch/templates/configmap-jvm-options.yaml")
	require.Contains(t, out, "-XX:-UseBASE64Intrinsics")
	require.Contains(t, out, "cve-2026-114938.options")

	for _, role := range []string{"master", "data", "coordinating", "ingest"} {
		t.Run(role, func(t *testing.T) {
			values := map[string]string{}
			if role != "master" {
				values["elasticsearch."+role+".replicaCount"] = "1"
			}
			if role == "ingest" {
				values["elasticsearch.ingest.enabled"] = "true"
			}

			out := renderJdkOptionsTemplate(t, values, "charts/elasticsearch/templates/"+role+"/statefulset.yaml")
			require.Contains(t, out, "jvm.options.d/cve-2026-114938.options")
			require.NotContains(t, out, "name: JDK_JAVA_OPTIONS")
		})
	}
}

func TestElasticsearchUserEnvVarsPreserved(t *testing.T) {
	t.Parallel()

	out := renderJdkOptionsTemplate(t, map[string]string{
		"elasticsearch.extraEnvVars[0].name":  "JDK_JAVA_OPTIONS",
		"elasticsearch.extraEnvVars[0].value": "-Xmx2g",
		"elasticsearch.extraEnvVars[1].name":  "OTHER",
		"elasticsearch.extraEnvVars[1].value": "x",
	}, "charts/elasticsearch/templates/master/statefulset.yaml")

	require.Equal(t, 1, strings.Count(out, "name: JDK_JAVA_OPTIONS"))
	require.Contains(t, out, "value: -Xmx2g")
	require.Contains(t, out, "name: OTHER")
}

func TestKeycloakJdkJavaOptionsOverride(t *testing.T) {
	t.Parallel()

	out := renderJdkOptionsTemplate(t, map[string]string{
		"identityKeycloak.extraEnvVars[0].name":  "JDK_JAVA_OPTIONS",
		"identityKeycloak.extraEnvVars[0].value": "-Dfoo=bar",
		"identityKeycloak.extraEnvVars[1].name":  "OTHER",
		"identityKeycloak.extraEnvVars[1].value": "x",
	}, "charts/identityKeycloak/templates/statefulset.yaml")

	require.Equal(t, 1, strings.Count(out, "name: JDK_JAVA_OPTIONS"))
	require.Contains(t, out, `"-Dfoo=bar -XX:-UseBASE64Intrinsics"`)
	require.Contains(t, out, "name: OTHER")
}

func TestKeycloakJdkJavaOptionsValueFrom(t *testing.T) {
	t.Parallel()

	out := renderJdkOptionsTemplate(t, map[string]string{
		"identityKeycloak.extraEnvVars[0].name":                           "JDK_JAVA_OPTIONS",
		"identityKeycloak.extraEnvVars[0].valueFrom.configMapKeyRef.name": "jvm",
		"identityKeycloak.extraEnvVars[0].valueFrom.configMapKeyRef.key":  "opts",
	}, "charts/identityKeycloak/templates/statefulset.yaml")

	require.Equal(t, 1, strings.Count(out, "name: JDK_JAVA_OPTIONS"))
	require.Contains(t, out, "valueFrom:")
	require.NotContains(t, out, `"null -XX:-UseBASE64Intrinsics"`)
}

func TestKeycloakJdkJavaOptionsLiteralAndValueFrom(t *testing.T) {
	t.Parallel()

	out := renderJdkOptionsTemplate(t, map[string]string{
		"identityKeycloak.extraEnvVars[0].name":                        "JDK_JAVA_OPTIONS",
		"identityKeycloak.extraEnvVars[0].value":                       "-Dfoo=bar",
		"identityKeycloak.extraEnvVars[1].name":                        "JDK_JAVA_OPTIONS",
		"identityKeycloak.extraEnvVars[1].valueFrom.secretKeyRef.name": "jvm",
		"identityKeycloak.extraEnvVars[1].valueFrom.secretKeyRef.key":  "opts",
	}, "charts/identityKeycloak/templates/statefulset.yaml")

	// The valueFrom entry is kept after the merged literal, so Kubernetes applies it last.
	require.Equal(t, 2, strings.Count(out, "name: JDK_JAVA_OPTIONS"))
	merged := strings.Index(out, `"-Dfoo=bar -XX:-UseBASE64Intrinsics"`)
	secretRef := strings.Index(out, "secretKeyRef:")
	require.NotEqual(t, -1, merged)
	require.Greater(t, secretRef, merged)
}

func TestKeycloakJdkJavaOptionsValueFromBeforeLiteral(t *testing.T) {
	t.Parallel()

	out := renderJdkOptionsTemplate(t, map[string]string{
		"identityKeycloak.extraEnvVars[0].name":                        "JDK_JAVA_OPTIONS",
		"identityKeycloak.extraEnvVars[0].valueFrom.secretKeyRef.name": "jvm",
		"identityKeycloak.extraEnvVars[0].valueFrom.secretKeyRef.key":  "opts",
		"identityKeycloak.extraEnvVars[1].name":                        "JDK_JAVA_OPTIONS",
		"identityKeycloak.extraEnvVars[1].value":                       "-Dfoo=bar",
	}, "charts/identityKeycloak/templates/statefulset.yaml")

	// The later literal wins, as before the merge; the earlier valueFrom entry is dropped.
	require.Equal(t, 1, strings.Count(out, "name: JDK_JAVA_OPTIONS"))
	require.Contains(t, out, `"-Dfoo=bar -XX:-UseBASE64Intrinsics"`)
	require.NotContains(t, out, "secretKeyRef:")
}

func TestKeycloakEmptyJdkJavaOptionsValue(t *testing.T) {
	t.Parallel()

	out := renderJdkOptionsTemplate(t, map[string]string{
		"identityKeycloak.extraEnvVars[0].name":  "JDK_JAVA_OPTIONS",
		"identityKeycloak.extraEnvVars[0].value": "",
	}, "charts/identityKeycloak/templates/statefulset.yaml")

	require.Equal(t, 1, strings.Count(out, "name: JDK_JAVA_OPTIONS"))
	require.Contains(t, out, `"-XX:-UseBASE64Intrinsics"`)
}

func TestKeycloakEnvSourceWarnings(t *testing.T) {
	t.Parallel()

	out := renderJdkOptionsTemplate(t, map[string]string{
		"identityKeycloak.extraEnvVarsCM":                                 "my-cm",
		"identityKeycloak.extraEnvVarsSecret":                             "my-secret",
		"identityKeycloak.extraEnvVars[0].name":                           "JDK_JAVA_OPTIONS",
		"identityKeycloak.extraEnvVars[0].valueFrom.configMapKeyRef.name": "jvm",
		"identityKeycloak.extraEnvVars[0].valueFrom.configMapKeyRef.key":  "opts",
	}, "templates/camunda/configmap-warnings.yaml")

	require.Contains(t, out, "identityKeycloak.extraEnvVarsCM")
	require.Contains(t, out, "identityKeycloak.extraEnvVarsSecret")
	require.Contains(t, out, "identityKeycloak.extraEnvVars[valueFrom]")
}

func TestKeycloakValueFromBeforeLiteralNoWarning(t *testing.T) {
	t.Parallel()

	out := renderJdkOptionsTemplate(t, map[string]string{
		"identityKeycloak.extraEnvVars[0].name":                        "JDK_JAVA_OPTIONS",
		"identityKeycloak.extraEnvVars[0].valueFrom.secretKeyRef.name": "jvm",
		"identityKeycloak.extraEnvVars[0].valueFrom.secretKeyRef.key":  "opts",
		"identityKeycloak.extraEnvVars[1].name":                        "JDK_JAVA_OPTIONS",
		"identityKeycloak.extraEnvVars[1].value":                       "-Dfoo=bar",
	}, "templates/camunda/configmap-warnings.yaml")

	require.NotContains(t, out, "identityKeycloak.extraEnvVars[valueFrom]")
}
