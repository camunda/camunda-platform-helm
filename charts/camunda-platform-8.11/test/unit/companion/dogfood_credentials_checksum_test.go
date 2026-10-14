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

package companion

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

const (
	checksumAnnotation = "camunda.io/credentials-checksum"
	checksumVar        = "${DOGFOOD_CREDENTIALS_CHECKSUM}"
	testChecksum       = "0123456789abcdef"
)

// substitutedLayer writes path with the checksum variable expanded, as
// deploy-camunda does before passing a layer to Helm.
func substitutedLayer(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(raw), checksumVar)
	out := filepath.Join(t.TempDir(), filepath.Base(path))
	require.NoError(t, os.WriteFile(out, []byte(strings.ReplaceAll(string(raw), checksumVar, testChecksum)), 0o600))
	return out
}

// workloadChecksums maps every rendered Deployment and StatefulSet to its pod
// template's checksum annotation.
func workloadChecksums(t *testing.T, output string) map[string]string {
	t.Helper()
	got := map[string]string{}
	dec := yaml.NewDecoder(strings.NewReader(output))
	for {
		var obj struct {
			Kind     string `yaml:"kind"`
			Metadata struct {
				Name string `yaml:"name"`
			} `yaml:"metadata"`
			Spec struct {
				Template struct {
					Metadata struct {
						Annotations map[string]string `yaml:"annotations"`
					} `yaml:"metadata"`
				} `yaml:"template"`
			} `yaml:"spec"`
		}
		if err := dec.Decode(&obj); err != nil {
			break
		}
		if obj.Kind == "Deployment" || obj.Kind == "StatefulSet" {
			got[obj.Kind+"/"+obj.Metadata.Name] = obj.Spec.Template.Metadata.Annotations[checksumAnnotation]
		}
	}
	return got
}

func TestDogfoodCredentialsChecksumReachesEveryConsumer(t *testing.T) {
	t.Parallel()

	chart, err := filepath.Abs("../../..")
	require.NoError(t, err)
	common := substitutedLayer(t, filepath.Join(chart, "test", "integration", "scenarios", "chart-full-setup", "values", "features", "dogfood-common.yaml"))
	topology := func(name string) string { return filepath.Join(chart, "test", "unit", "topology", "testdata", name) }

	for _, role := range []struct {
		name      string
		values    []string
		set       map[string]string
		workloads []string
	}{
		{
			name:      "hub",
			values:    []string{topology("hub-keycloak.yaml"), common},
			set:       map[string]string{"webModeler.enabled": "true", "webModeler.restapi.mail.fromAddress": "noreply@example.com"},
			workloads: []string{"Deployment/camunda-identity", "Deployment/camunda-web-modeler-restapi", "Deployment/camunda-web-modeler-websockets"},
		},
		{
			name:      "orchestration",
			values:    []string{topology("orchestration.yaml"), common},
			workloads: []string{"StatefulSet/camunda-zeebe", "Deployment/camunda-connectors", "Deployment/camunda-optimize"},
		},
		{
			name:      "optimize",
			values:    []string{topology("optimize.yaml"), common},
			workloads: []string{"Deployment/camunda-optimize"},
		},
	} {
		t.Run(role.name, func(t *testing.T) {
			output, err := helm.RenderTemplateE(t, &helm.Options{ValuesFiles: role.values, SetValues: role.set}, chart, "camunda", nil)
			require.NoError(t, err)
			got := workloadChecksums(t, output)
			for _, w := range role.workloads {
				require.Contains(t, got, w, "rendered workloads: %v", keys(got))
				require.Equal(t, testChecksum, got[w], "%s must restart when the dogfood credentials rotate", w)
			}
		})
	}

	t.Run("keycloak", func(t *testing.T) {
		keycloakChart, err := filepath.Abs("../../../../internal-keycloak-26")
		require.NoError(t, err)
		values := substitutedLayer(t, filepath.Join(chart, "..", "..", "test", "integration", "companion-values", "keycloak-dogfood.yaml"))
		output, err := helm.RenderTemplateE(t, &helm.Options{ValuesFiles: []string{values}}, keycloakChart, "keycloak", []string{"templates/deployment.yaml"})
		require.NoError(t, err)
		require.Equal(t, testChecksum, workloadChecksums(t, output)["Deployment/keycloak"])
	})

	// The elastic/elasticsearch chart is fetched from a remote repository, so
	// check the key its StatefulSet template reads pod annotations from.
	t.Run("elasticsearch", func(t *testing.T) {
		raw, err := os.ReadFile(filepath.Join(chart, "..", "..", "test", "integration", "companion-values", "elasticsearch-dogfood.yaml"))
		require.NoError(t, err)
		var values struct {
			PodAnnotations map[string]string `yaml:"podAnnotations"`
		}
		require.NoError(t, yaml.Unmarshal(raw, &values))
		require.Equal(t, checksumVar, values.PodAnnotations[checksumAnnotation])
	})
}

func keys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
