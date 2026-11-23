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
	"camunda-platform/test/unit/utils"
	"maps"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type NetworkPolicyTemplateTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
	templates []string
}

func TestNetworkPolicyTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &NetworkPolicyTemplateTest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
		templates: []string{},
	})
}

var networkPolicyAllComponents = map[string]string{
	"identity.enabled":                    "true",
	"optimize.enabled":                    "true",
	"connectors.enabled":                  "true",
	"camundaHub.enabled":                  "true",
	"webModeler.restapi.mail.fromAddress": "test@test.com",
}

var networkPolicyNamePattern = regexp.MustCompile(`(?m)^kind: NetworkPolicy\nmetadata:\n  name: (\S+)`)

func networkPolicyNames(output string) []string {
	names := []string{}
	for _, m := range networkPolicyNamePattern.FindAllStringSubmatch(output, -1) {
		names = append(names, m[1])
	}
	sort.Strings(names)
	return names
}

func networkPolicyDoc(t *testing.T, output, name string) string {
	t.Helper()
	for _, doc := range strings.Split(output, "\n---\n") {
		if strings.Contains(doc, "kind: NetworkPolicy\nmetadata:\n  name: "+name+"\n") {
			return doc + "\n"
		}
	}
	require.FailNow(t, "NetworkPolicy not found", name)
	return ""
}

func networkPolicyValues(extra map[string]string) map[string]string {
	values := maps.Clone(networkPolicyAllComponents)
	maps.Copy(values, extra)
	return values
}

func networkPolicyEgress(extra map[string]string) map[string]string {
	values := map[string]string{
		"global.networkPolicy.enabled":        "true",
		"global.networkPolicy.egress.enabled": "true",
	}
	maps.Copy(values, extra)
	return values
}

func (s *NetworkPolicyTemplateTest) TestDifferentValuesInputs() {
	allPolicies := []string{
		"camunda-platform-test-connectors",
		"camunda-platform-test-identity",
		"camunda-platform-test-optimize",
		"camunda-platform-test-web-modeler-restapi",
		"camunda-platform-test-web-modeler-websockets",
		"camunda-platform-test-zeebe",
	}

	testCases := []testhelpers.TestCase{
		{
			Name:   "TestNoNetworkPolicyByDefault",
			Values: networkPolicyValues(nil),
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Empty(t, networkPolicyNames(output))
			},
		},
		{
			Name:   "TestAllNetworkPoliciesWhenGloballyEnabled",
			Values: networkPolicyValues(map[string]string{"global.networkPolicy.enabled": "true"}),
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Equal(t, allPolicies, networkPolicyNames(output))
			},
		},
		{
			Name: "TestNoNetworkPolicyWhenIngressDisabled",
			Values: networkPolicyValues(map[string]string{
				"global.networkPolicy.enabled":         "true",
				"global.networkPolicy.ingress.enabled": "false",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Empty(t, networkPolicyNames(output))
			},
		},
		{
			Name: "TestComponentTogglesDisableTheirPolicy",
			Values: networkPolicyValues(map[string]string{
				"global.networkPolicy.enabled":                "true",
				"orchestration.networkPolicy.enabled":         "false",
				"connectors.networkPolicy.enabled":            "false",
				"identity.networkPolicy.enabled":              "false",
				"optimize.networkPolicy.enabled":              "false",
				"camundaHub.restapi.networkPolicy.enabled":    "false",
				"camundaHub.websockets.networkPolicy.enabled": "false",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Empty(t, networkPolicyNames(output))
			},
		},
		{
			Name: "TestNoPolicyForDisabledComponents",
			Values: map[string]string{
				"global.networkPolicy.enabled": "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Equal(t, []string{"camunda-platform-test-connectors", "camunda-platform-test-zeebe"}, networkPolicyNames(output))
			},
		},
		{
			Name: "TestExtraRulesAppended",
			Values: map[string]string{
				"global.networkPolicy.enabled": "true",
				"global.networkPolicy.ingress.extraRules[0].from[0].namespaceSelector.matchLabels.kubernetes\\.io/metadata\\.name": "ingress-nginx",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "kubernetes.io/metadata.name: ingress-nginx")
				require.Contains(t, output, "- podSelector: {}")
			},
		},
		{
			Name:   "TestEgressOffByDefault",
			Values: networkPolicyValues(map[string]string{"global.networkPolicy.enabled": "true"}),
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.NotContains(t, output, "- Egress")
				require.NotContains(t, output, "egress:")
			},
		},
		{
			Name:   "TestEgressAddedToEveryPolicy",
			Values: networkPolicyValues(networkPolicyEgress(nil)),
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Equal(t, allPolicies, networkPolicyNames(output))
				require.Equal(t, len(allPolicies), strings.Count(output, "    - Ingress\n    - Egress\n"))
				require.Equal(t, len(allPolicies), strings.Count(output, "k8s-app: kube-dns"))
			},
		},
		{
			Name: "TestEgressRequiresMasterSwitch",
			Values: networkPolicyValues(map[string]string{
				"global.networkPolicy.egress.enabled": "true",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Empty(t, networkPolicyNames(output))
			},
		},
		{
			Name: "TestEgressOnlyWhenIngressDisabled",
			Values: networkPolicyValues(networkPolicyEgress(map[string]string{
				"global.networkPolicy.ingress.enabled": "false",
			})),
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Equal(t, allPolicies, networkPolicyNames(output))
				require.NotContains(t, output, "- Ingress")
				require.NotContains(t, output, "ingress:")
				require.Equal(t, len(allPolicies), strings.Count(output, "  policyTypes:\n    - Egress\n"))
			},
		},
		{
			Name: "TestComponentEgressToggleKeepsIngress",
			Values: networkPolicyValues(networkPolicyEgress(map[string]string{
				"connectors.networkPolicy.egress.enabled": "false",
			})),
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Equal(t, allPolicies, networkPolicyNames(output))
				require.Equal(t, len(allPolicies)-1, strings.Count(output, "    - Egress\n"))
			},
		},
		{
			Name: "TestComponentEgressToggleWithIngressDisabledRemovesPolicy",
			Values: networkPolicyValues(networkPolicyEgress(map[string]string{
				"global.networkPolicy.ingress.enabled":               "false",
				"connectors.networkPolicy.egress.enabled":            "false",
				"camundaHub.websockets.networkPolicy.egress.enabled": "false",
			})),
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Equal(t, []string{
					"camunda-platform-test-identity",
					"camunda-platform-test-optimize",
					"camunda-platform-test-web-modeler-restapi",
					"camunda-platform-test-zeebe",
				}, networkPolicyNames(output))
			},
		},
		{
			Name: "TestComponentToggleRemovesEgressPolicy",
			Values: networkPolicyValues(networkPolicyEgress(map[string]string{
				"optimize.networkPolicy.enabled": "false",
			})),
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.NotContains(t, networkPolicyNames(output), "camunda-platform-test-optimize")
			},
		},
		{
			Name: "TestEgressWithoutDNS",
			Values: networkPolicyValues(networkPolicyEgress(map[string]string{
				"global.networkPolicy.egress.allowDNS": "false",
			})),
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "- Egress")
				require.NotContains(t, output, "kube-dns")
				require.NotContains(t, output, "port: 53")
			},
		},
		{
			Name: "TestEgressExtraRulesGlobalThenComponent",
			Values: networkPolicyValues(networkPolicyEgress(map[string]string{
				"global.networkPolicy.egress.extraRules[0].to[0].namespaceSelector.matchLabels.kubernetes\\.io/metadata\\.name": "camunda-db",
				"orchestration.networkPolicy.egress.extraRules[0].to[0].ipBlock.cidr":                                           "10.20.0.0/16",
			})),
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Equal(t, len(allPolicies), strings.Count(output, "kubernetes.io/metadata.name: camunda-db\n"))
				require.Equal(t, 1, strings.Count(output, "cidr: 10.20.0.0/16"))
				for _, doc := range strings.Split(output, "\n---\n") {
					if strings.Contains(doc, "cidr: 10.20.0.0/16") {
						require.Contains(t, doc, "name: camunda-platform-test-zeebe\n")
						require.Less(t, strings.Index(doc, "k8s-app: kube-dns"), strings.Index(doc, "camunda-db"))
						require.Less(t, strings.Index(doc, "camunda-db"), strings.Index(doc, "cidr: 10.20.0.0/16"))
					}
				}
			},
		},
		{
			Name:   "TestEgressWarningWithExternalDependencies",
			Values: networkPolicyValues(networkPolicyEgress(map[string]string{"global.identity.auth.enabled": "true"})),
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "global.networkPolicy.egress.enabled=true blocks all egress")
			},
		},
		{
			Name:   "TestEgressWarningWhenExternalDependenciesUnreachable",
			Values: networkPolicyValues(networkPolicyEgress(map[string]string{"global.identity.auth.enabled": "true"})),
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "global.networkPolicy.egress.enabled=true blocks all egress except the release namespace and DNS, but no global.networkPolicy.egress.extraRules or egress allowedNamespaces are set. The release connects to secondary storage (elasticsearch), the OIDC provider, which is unreachable unless it runs in the release namespace. Allow it with global.networkPolicy.egress.allowedNamespaces for in-cluster namespaces, or global.networkPolicy.egress.extraRules or the component networkPolicy.egress.extraRules; NetworkPolicies match IP blocks and labels, not hostnames.")
			},
		},
		{
			Name:        "TestAllowedNamespacesEmptyMatchesToday",
			Values:      networkPolicyValues(nil),
			ValuesFiles: []string{"testdata/networkpolicy-allowed-empty.yaml"},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Equal(t, allPolicies, networkPolicyNames(output))
				require.NotContains(t, output, "namespaceSelector")
				require.Equal(t, len(allPolicies), strings.Count(output, "    - from:\n"))
			},
		},
		{
			Name:        "TestIngressAllowedNamespacesLiteral",
			Values:      networkPolicyValues(nil),
			ValuesFiles: []string{"testdata/networkpolicy-allowed-literal.yaml"},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Equal(t, len(allPolicies), strings.Count(output, "kubernetes.io/metadata.name: \""+s.namespace+"-allowed\"\n"))
				orchestration := networkPolicyDoc(t, output, "camunda-platform-test-zeebe")
				rule := orchestration[strings.Index(orchestration, "namespaceSelector"):]
				require.Equal(t, "- protocol: TCP\n          port: 8080\n        - protocol: TCP\n          port: 26500\n", rule[strings.Index(rule, "- protocol"):])
				require.Contains(t, networkPolicyDoc(t, output, "camunda-platform-test-connectors"), "namespaceSelector:\n            matchLabels:\n              kubernetes.io/metadata.name: \""+s.namespace+"-allowed\"\n      ports:\n        - protocol: TCP\n          port: 8080\n")
				require.Contains(t, networkPolicyDoc(t, output, "camunda-platform-test-identity"), "-allowed\"\n      ports:\n        - protocol: TCP\n          port: 8084\n")
				require.Contains(t, networkPolicyDoc(t, output, "camunda-platform-test-optimize"), "-allowed\"\n      ports:\n        - protocol: TCP\n          port: 8090\n")
				require.Contains(t, networkPolicyDoc(t, output, "camunda-platform-test-web-modeler-restapi"), "-allowed\"\n      ports:\n        - protocol: TCP\n          port: 8081\n")
				require.Contains(t, networkPolicyDoc(t, output, "camunda-platform-test-web-modeler-websockets"), "-allowed\"\n      ports:\n        - protocol: TCP\n          port: 8060\n")
			},
		},
		{
			Name:        "TestIngressAllowedNamespacesTpl",
			Values:      networkPolicyValues(nil),
			ValuesFiles: []string{"testdata/networkpolicy-allowed-selector.yaml"},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.NotContains(t, output, "{{")
				require.Equal(t, len(allPolicies), strings.Count(output, "camunda.io/tenant-of: "+s.namespace+"\n"))
			},
		},
		{
			Name:        "TestIngressAllowedNamespacesSelectorWithPodSelector",
			Values:      networkPolicyValues(nil),
			ValuesFiles: []string{"testdata/networkpolicy-allowed-selector.yaml"},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Equal(t, len(allPolicies), strings.Count(output, "    - from:\n        - namespaceSelector:\n            matchLabels:\n              camunda.io/tenant-of: "+s.namespace+"\n          podSelector:\n            matchLabels:\n              app.kubernetes.io/name: hub-client\n"))
			},
		},
		{
			Name:        "TestEntryPortsOverrideDefaults",
			Values:      networkPolicyValues(nil),
			ValuesFiles: []string{"testdata/networkpolicy-allowed-ports.yaml"},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				orchestration := networkPolicyDoc(t, output, "camunda-platform-test-zeebe")
				rule := orchestration[strings.Index(orchestration, "namespaceSelector"):]
				require.Equal(t, "- endPort: 9601\n          port: 9600\n          protocol: TCP\n", rule[strings.Index(rule, "- endPort"):])
				require.Equal(t, 1, strings.Count(orchestration, "port: 26500"))
			},
		},
		{
			Name: "TestComponentListReplacesGlobal",
			Values: networkPolicyValues(map[string]string{
				"connectors.networkPolicy.ingress.allowedNamespaces[0]": "connectors-only",
			}),
			ValuesFiles: []string{"testdata/networkpolicy-allowed-literal.yaml"},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				connectors := networkPolicyDoc(t, output, "camunda-platform-test-connectors")
				require.Contains(t, connectors, "kubernetes.io/metadata.name: \"connectors-only\"")
				require.NotContains(t, connectors, "-allowed")
				require.Contains(t, networkPolicyDoc(t, output, "camunda-platform-test-zeebe"), "-allowed")
				require.NotContains(t, networkPolicyDoc(t, output, "camunda-platform-test-zeebe"), "connectors-only")
			},
		},
		{
			Name:        "TestComponentEmptyListNarrowsToSameNamespace",
			Values:      networkPolicyValues(nil),
			ValuesFiles: []string{"testdata/networkpolicy-allowed-literal.yaml", "testdata/networkpolicy-allowed-connectors-empty.yaml"},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				connectors := networkPolicyDoc(t, output, "camunda-platform-test-connectors")
				require.NotContains(t, connectors, "namespaceSelector")
				require.Contains(t, connectors, "- podSelector: {}")
				require.Contains(t, networkPolicyDoc(t, output, "camunda-platform-test-zeebe"), "-allowed")
			},
		},
		{
			Name:        "TestFailOnEmptyNamespaceSelector",
			Values:      networkPolicyValues(nil),
			ValuesFiles: []string{"testdata/networkpolicy-allowed-empty-selector.yaml"},
			Verifier: func(t *testing.T, output string, err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "namespaceSelector")
			},
		},
		{
			Name:        "TestFailOnNameAndSelector",
			Values:      networkPolicyValues(nil),
			ValuesFiles: []string{"testdata/networkpolicy-allowed-name-and-selector.yaml"},
			Verifier: func(t *testing.T, output string, err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "allowedNamespaces")
			},
		},
		{
			Name: "TestFailOnNeitherNameNorSelector",
			Values: networkPolicyValues(map[string]string{
				"global.networkPolicy.enabled": "true",
				"identity.networkPolicy.ingress.allowedNamespaces[0].podSelector.matchLabels.app": "x",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "allowedNamespaces")
			},
		},
		{
			Name: "TestHubAllowedNamespaces",
			Values: networkPolicyValues(map[string]string{
				"global.networkPolicy.enabled":                                  "true",
				"camundaHub.restapi.networkPolicy.ingress.allowedNamespaces[0]": "hub-clients",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Equal(t, 1, strings.Count(output, "kubernetes.io/metadata.name: \"hub-clients\""))
				require.Contains(t, networkPolicyDoc(t, output, "camunda-platform-test-web-modeler-restapi"), "\"hub-clients\"\n      ports:\n        - protocol: TCP\n          port: 8081\n")
			},
		},
		{
			Name: "TestAllowedNamespacesBeforeExtraRules",
			Values: networkPolicyValues(map[string]string{
				"global.networkPolicy.ingress.extraRules[0].from[0].namespaceSelector.matchLabels.kubernetes\\.io/metadata\\.name": "ingress-nginx",
			}),
			ValuesFiles: []string{"testdata/networkpolicy-allowed-literal.yaml"},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				for _, name := range allPolicies {
					doc := networkPolicyDoc(t, output, name)
					require.Less(t, strings.Index(doc, "- podSelector: {}"), strings.Index(doc, "-allowed"))
					require.Less(t, strings.Index(doc, "-allowed"), strings.Index(doc, "ingress-nginx"))
				}
			},
		},
		{
			Name:        "TestEgressAllowedNamespacesLiteralAllowsEveryPort",
			Values:      networkPolicyValues(nil),
			ValuesFiles: []string{"testdata/networkpolicy-egress-allowed-literal.yaml"},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.NotContains(t, output, "{{")
				for _, name := range allPolicies {
					doc := networkPolicyDoc(t, output, name)
					require.True(t, strings.HasSuffix(doc, "    - to:\n        - namespaceSelector:\n            matchLabels:\n              kubernetes.io/metadata.name: \""+s.namespace+"-allowed\"\n"), name)
					require.NotContains(t, doc, "    - from:\n        - namespaceSelector")
				}
			},
		},
		{
			Name:        "TestEgressAllowedNamespacesSelectorAndPorts",
			Values:      networkPolicyValues(nil),
			ValuesFiles: []string{"testdata/networkpolicy-egress-allowed-namespaces.yaml"},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.NotContains(t, output, "{{")
				require.Equal(t, len(allPolicies), strings.Count(output, "    - to:\n        - namespaceSelector:\n            matchLabels:\n              camunda.io/tenant-of: "+s.namespace+"\n          podSelector:\n            matchLabels:\n              app.kubernetes.io/name: hub-server\n"))
				require.Equal(t, len(allPolicies), strings.Count(output, "kubernetes.io/metadata.name: \"elastic-system\"\n      ports:\n        - port: 9200\n          protocol: TCP\n"))
			},
		},
		{
			Name: "TestEgressComponentListReplacesGlobal",
			Values: networkPolicyValues(map[string]string{
				"connectors.networkPolicy.egress.allowedNamespaces[0]": "connectors-only",
			}),
			ValuesFiles: []string{"testdata/networkpolicy-egress-allowed-literal.yaml"},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				connectors := networkPolicyDoc(t, output, "camunda-platform-test-connectors")
				require.Contains(t, connectors, "    - to:\n        - namespaceSelector:\n            matchLabels:\n              kubernetes.io/metadata.name: \"connectors-only\"\n")
				require.NotContains(t, connectors, "-allowed")
				require.Contains(t, networkPolicyDoc(t, output, "camunda-platform-test-zeebe"), "-allowed")
				require.NotContains(t, networkPolicyDoc(t, output, "camunda-platform-test-zeebe"), "connectors-only")
			},
		},
		{
			Name:        "TestEgressComponentEmptyListNarrows",
			Values:      networkPolicyValues(nil),
			ValuesFiles: []string{"testdata/networkpolicy-egress-allowed-literal.yaml", "testdata/networkpolicy-egress-allowed-connectors-empty.yaml"},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				connectors := networkPolicyDoc(t, output, "camunda-platform-test-connectors")
				require.NotContains(t, connectors, "-allowed")
				require.Contains(t, connectors, "k8s-app: kube-dns")
				require.Contains(t, networkPolicyDoc(t, output, "camunda-platform-test-zeebe"), "-allowed")
			},
		},
		{
			Name:        "TestEgressAllowedNamespacesDoNotChangeIngress",
			Values:      networkPolicyValues(nil),
			ValuesFiles: []string{"testdata/networkpolicy-egress-allowed-literal.yaml"},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Equal(t, len(allPolicies), strings.Count(output, "-allowed\""))
				require.Equal(t, len(allPolicies), strings.Count(output, "    - from:\n"))
			},
		},
		{
			Name: "TestEgressWarningSuppressedByAllowedNamespaces",
			Values: networkPolicyValues(networkPolicyEgress(map[string]string{
				"global.identity.auth.enabled":                     "true",
				"global.networkPolicy.egress.allowedNamespaces[0]": "camunda-db",
			})),
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.NotContains(t, output, "blocks all egress")
			},
		},
		{
			Name: "TestEgressWarningSuppressedByComponentAllowedNamespaces",
			Values: networkPolicyValues(networkPolicyEgress(map[string]string{
				"global.identity.auth.enabled":                            "true",
				"orchestration.networkPolicy.egress.allowedNamespaces[0]": "camunda-db",
			})),
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.NotContains(t, output, "blocks all egress")
			},
		},
		{
			Name: "TestEgressHubAllowedNamespaces",
			Values: networkPolicyValues(networkPolicyEgress(map[string]string{
				"camundaHub.websockets.networkPolicy.egress.allowedNamespaces[0]": "hub-backend",
			})),
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Equal(t, 1, strings.Count(output, "kubernetes.io/metadata.name: \"hub-backend\""))
				require.Contains(t, networkPolicyDoc(t, output, "camunda-platform-test-web-modeler-websockets"), "    - to:\n        - namespaceSelector:\n            matchLabels:\n              kubernetes.io/metadata.name: \"hub-backend\"\n")
			},
		},
		{
			Name: "TestEgressAllowedNamespacesRuleOrder",
			Values: networkPolicyValues(map[string]string{
				"global.networkPolicy.egress.extraRules[0].to[0].ipBlock.cidr":        "10.20.0.0/16",
				"orchestration.networkPolicy.egress.extraRules[0].to[0].ipBlock.cidr": "10.30.0.0/16",
			}),
			ValuesFiles: []string{"testdata/networkpolicy-egress-allowed-literal.yaml"},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				for _, name := range allPolicies {
					doc := networkPolicyDoc(t, output, name)
					egress := doc[strings.Index(doc, "  egress:\n"):]
					require.Less(t, strings.Index(egress, "- podSelector: {}"), strings.Index(egress, "k8s-app: kube-dns"))
					require.Less(t, strings.Index(egress, "k8s-app: kube-dns"), strings.Index(egress, "-allowed"))
					require.Less(t, strings.Index(egress, "-allowed"), strings.Index(egress, "10.20.0.0/16"))
				}
				zeebe := networkPolicyDoc(t, output, "camunda-platform-test-zeebe")
				require.Less(t, strings.Index(zeebe, "10.20.0.0/16"), strings.Index(zeebe, "10.30.0.0/16"))
			},
		},
		{
			Name:                    "TestEgressFailOnEmptyNamespaceSelector",
			RenderTemplateExtraArgs: []string{"--skip-schema-validation"},
			Values:                  networkPolicyValues(nil),
			ValuesFiles:             []string{"testdata/networkpolicy-egress-allowed-empty-selector.yaml"},
			Verifier: func(t *testing.T, output string, err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "global.networkPolicy.egress.allowedNamespaces has an entry with an empty namespaceSelector")
			},
		},
		{
			Name:                    "TestEgressFailOnNameAndSelector",
			RenderTemplateExtraArgs: []string{"--skip-schema-validation"},
			Values: networkPolicyValues(map[string]string{
				"global.networkPolicy.enabled":                                                          "true",
				"optimize.networkPolicy.egress.allowedNamespaces[0].name":                               "db",
				"optimize.networkPolicy.egress.allowedNamespaces[0].namespaceSelector.matchLabels.team": "db",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "optimize.networkPolicy.egress.allowedNamespaces must set exactly one")
			},
		},
		{
			Name:                    "TestEgressFailOnNeitherNameNorSelector",
			RenderTemplateExtraArgs: []string{"--skip-schema-validation"},
			Values: networkPolicyValues(map[string]string{
				"global.networkPolicy.enabled": "true",
				"camundaHub.restapi.networkPolicy.egress.allowedNamespaces[0].podSelector.matchLabels.app": "x",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "camundaHub.restapi.networkPolicy.egress.allowedNamespaces must set exactly one")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func TestGoldenNetworkPolicies(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	for name, template := range map[string]string{
		"networkpolicy-orchestration":          "templates/orchestration/networkpolicy.yaml",
		"networkpolicy-connectors":             "templates/connectors/networkpolicy.yaml",
		"networkpolicy-identity":               "templates/identity/networkpolicy.yaml",
		"networkpolicy-optimize":               "templates/optimize/networkpolicy.yaml",
		"networkpolicy-web-modeler-restapi":    "templates/web-modeler/networkpolicy-restapi.yaml",
		"networkpolicy-web-modeler-websockets": "templates/web-modeler/networkpolicy-websockets.yaml",
	} {
		suite.Run(t, &utils.TemplateGoldenTest{
			ChartPath:      chartPath,
			Release:        "camunda-platform-test",
			Namespace:      "camunda-platform-test",
			GoldenFileName: name,
			Templates:      []string{template},
			SetValues:      networkPolicyValues(map[string]string{"global.networkPolicy.enabled": "true"}),
		})
		suite.Run(t, &utils.TemplateGoldenTest{
			ChartPath:      chartPath,
			Release:        "camunda-platform-test",
			Namespace:      "camunda-platform-test",
			GoldenFileName: name + "-egress",
			Templates:      []string{template},
			SetValues:      networkPolicyValues(networkPolicyEgress(nil)),
		})
		suite.Run(t, &utils.TemplateGoldenTest{
			ChartPath:      chartPath,
			Release:        "camunda-platform-test",
			Namespace:      "camunda-platform-test",
			GoldenFileName: name + "-allowed-namespaces",
			Templates:      []string{template},
			SetValues:      networkPolicyValues(nil),
			ValuesFiles:    []string{"testdata/networkpolicy-allowed-namespaces.yaml"},
		})
		suite.Run(t, &utils.TemplateGoldenTest{
			ChartPath:      chartPath,
			Release:        "camunda-platform-test",
			Namespace:      "camunda-platform-test",
			GoldenFileName: name + "-egress-allowednamespaces",
			Templates:      []string{template},
			SetValues:      networkPolicyValues(nil),
			ValuesFiles:    []string{"testdata/networkpolicy-egress-allowed-namespaces.yaml"},
		})
	}
}
