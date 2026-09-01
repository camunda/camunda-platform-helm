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

package topology

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/stretchr/testify/require"
)

func TestTopologyContractResolvesRedirectTemplatesInReleaseContext(t *testing.T) {
	options := &helm.Options{
		ValuesFiles: []string{filepath.Join("testdata", "hub-keycloak.yaml")},
		SetValues: map[string]string{
			"global.ingress.host": "hub.example.test",
			"global.topology.clusters[0].components.optimize.enabled":                  "true",
			"global.topology.clusters[0].components.optimize.clientId":                 "cluster-optimize",
			"global.topology.clusters[0].components.optimize.audience":                 "cluster-optimize-api",
			"global.topology.clusters[0].components.optimize.redirectUrl":              `https://{{ .Values.global.ingress.host }}/cluster-optimize`,
			"global.topology.clusters[0].components.optimize.secret.existingSecret":    "cluster-oidc",
			"global.topology.clusters[0].components.optimize.secret.existingSecretKey": "client-secret",
			"identity.clients[0].id":                            "tenant-optimize",
			"identity.clients[0].name":                          "Tenant Optimize",
			"identity.clients[0].type":                          "public",
			"identity.clients[0].rootUrl":                       `https://{{ .Values.global.ingress.host }}/tenant-optimize`,
			"identity.clients[0].redirectUris[0]":               `https://{{ .Values.global.ingress.host }}/tenant-optimize/callback`,
			"identity.clients[0].secret.inlineSecret":           "unused",
			"optimize.security.authentication.oidc.redirectUrl": `https://{{ .Values.global.ingress.host }}/release-optimize`,
			"orchestration.data.secondaryStorage.type":          "elasticsearch",
		},
	}
	output := helm.RenderTemplate(t, options, chartPath(t), "camunda", []string{
		"templates/common/topology-contract.yaml",
		"templates/identity/configmap.yaml",
	}, "--api-versions", "camunda.io/topology-contract")

	var contractDocument struct {
		Data map[string]string `yaml:"data"`
	}
	for _, document := range splitDocuments(output) {
		if contains(document, "topology-contract") {
			helm.UnmarshalK8SYaml(t, document, &contractDocument)
		}
	}
	var contract struct {
		Optimize struct {
			RedirectURL string `json:"redirectUrl"`
		} `json:"optimize"`
		Hub struct {
			Clusters []struct {
				Optimize struct {
					RedirectURL string `json:"redirectUrl"`
				} `json:"optimize"`
			} `json:"clusters"`
			IdentityClients []struct {
				RootURL      string   `json:"rootUrl"`
				RedirectURIs []string `json:"redirectUris"`
			} `json:"identityClients"`
		} `json:"hub"`
	}
	require.NoError(t, json.Unmarshal([]byte(contractDocument.Data["contract.json"]), &contract))
	require.Equal(t, "https://hub.example.test/release-optimize", contract.Optimize.RedirectURL)
	require.Equal(t, "https://hub.example.test/cluster-optimize", contract.Hub.Clusters[0].Optimize.RedirectURL)
	require.Equal(t, "https://hub.example.test/tenant-optimize", contract.Hub.IdentityClients[0].RootURL)
	require.Equal(t, []string{"https://hub.example.test/tenant-optimize/callback"}, contract.Hub.IdentityClients[0].RedirectURIs)
	require.NotContains(t, output, "{{ .Values.global.ingress.host }}")
	require.Contains(t, output, "rootUrl: https://hub.example.test/tenant-optimize")
	require.Contains(t, output, "- https://hub.example.test/tenant-optimize/callback")
}
