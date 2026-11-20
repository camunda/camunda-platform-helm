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
				require.Contains(t, output, "global.networkPolicy.egress.enabled=true blocks all egress except the release namespace and DNS, but no global.networkPolicy.egress.extraRules are set. The release connects to secondary storage (elasticsearch), the OIDC provider, which is unreachable unless it runs in the release namespace. Allow it with global.networkPolicy.egress.extraRules or the component networkPolicy.egress.extraRules; NetworkPolicies match IP blocks and labels, not hostnames.")
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
	}
}
