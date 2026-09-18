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
	"camunda-platform/test/unit/testhelpers"
	_ "camunda-platform/test/unit/utils"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// orchestrationValues is the topology fixture every case starts from: a
// workload-only release in orchestration mode, pointed at a Hub namespace's
// Management Identity. Cases layer --set values on top of it.
var orchestrationValues = []string{"-f", filepath.Join("testdata", "orchestration.yaml")}

type TopologyTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
	templates []string
}

func TestTopologyTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &TopologyTest{
		chartPath: chartPath,
		release:   "camunda",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
		// The release ConfigMap renders in every mode, so it is enough to
		// reach the constraints in templates/camunda/constraints.tpl.
		templates: []string{"templates/camunda/configmap-release.yaml"},
	})
}

// TestOrchestrationTopologyRendersWorkloadComponentsOnly asserts the split
// itself: an orchestration-mode release renders the workload components and
// none of the Hub-plane ones.
func (s *TopologyTest) TestOrchestrationTopologyRendersWorkloadComponentsOnly() {
	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, nil, []testhelpers.TestCase{
		{
			Name:                    "TestOrchestrationModeRendersWorkloadComponentsOnly",
			CaseTemplates:           &testhelpers.CaseTemplate{Templates: nil},
			RenderTemplateExtraArgs: orchestrationValues,
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)

				for _, resource := range []string{
					"templates/zeebe/statefulset.yaml",
					"templates/zeebe-gateway/deployment.yaml",
					"templates/operate/deployment.yaml",
					"templates/tasklist/deployment.yaml",
					"templates/connectors/deployment.yaml",
					"templates/optimize/deployment.yaml",
					"templates/service-monitor/zeebe-service-monitor.yaml",
					"templates/service-monitor/zeebe-gateway-service-monitor.yaml",
					"templates/service-monitor/operate-service-monitor.yaml",
					"templates/service-monitor/tasklist-service-monitor.yaml",
					"templates/service-monitor/connectors-service-monitor.yaml",
					"templates/service-monitor/optimize-service-monitor.yaml",
				} {
					s.Require().Contains(output, resource)
				}
				for _, resource := range []string{
					"templates/identity/deployment.yaml",
					"templates/identity/postgresql-secret.yaml",
					"templates/console/deployment.yaml",
					"templates/web-modeler/deployment-restapi.yaml",
					"templates/web-modeler/deployment-webapp.yaml",
					"templates/web-modeler/deployment-websockets.yaml",
					"templates/service-monitor/identity-service-monitor.yaml",
					"templates/service-monitor/console-service-monitor.yaml",
					"templates/service-monitor/web-modeler-service-monitor.yaml",
				} {
					s.Require().NotContains(output, resource)
				}

				for _, name := range []string{
					"name: camunda-zeebe-gateway",
					"name: camunda-operate",
					"name: camunda-tasklist",
				} {
					s.Require().Contains(output, name)
				}
				for _, name := range []string{
					"name: camunda-identity",
					"name: camunda-console",
					"name: camunda-web-modeler",
				} {
					s.Require().NotContains(output, name)
				}
			},
		},
	})
}

// TestOrchestrationTopologyConstraints covers each guard that keeps an
// orchestration-mode release from deploying, or silently omitting, something
// the Hub release owns.
func (s *TopologyTest) TestOrchestrationTopologyConstraints() {
	testCases := []testhelpers.TestCase{
		{
			Name:     "TestAuthDisabled",
			Values:   map[string]string{"global.identity.auth.enabled": "false"},
			Expected: map[string]string{"ERROR": "requires global.identity.auth.enabled=true"},
		}, {
			Name:     "TestIdentityEnabled",
			Values:   map[string]string{"identity.enabled": "true"},
			Expected: map[string]string{"ERROR": "requires identity.enabled=false"},
		}, {
			Name:     "TestKeycloakEnabled",
			Values:   map[string]string{"identityKeycloak.enabled": "true"},
			Expected: map[string]string{"ERROR": "requires identityKeycloak.enabled=false"},
		}, {
			Name:     "TestIdentityDatabaseEnabled",
			Values:   map[string]string{"identityPostgresql.enabled": "true"},
			Expected: map[string]string{"ERROR": "requires identityPostgresql.enabled=false"},
		}, {
			Name:     "TestModelerDatabaseEnabled",
			Values:   map[string]string{"postgresql.enabled": "true"},
			Expected: map[string]string{"ERROR": "requires postgresql.enabled=false"},
		}, {
			Name:     "TestExecutionIdentityEnabled",
			Values:   map[string]string{"executionIdentity.enabled": "true"},
			Expected: map[string]string{"ERROR": "requires executionIdentity.enabled=false"},
		}, {
			Name:     "TestIdentityURLEmpty",
			Values:   map[string]string{"global.identity.service.url": ""},
			Expected: map[string]string{"ERROR": "requires global.identity.service.url"},
		}, {
			Name:     "TestZeebeDisabled",
			Values:   map[string]string{"zeebe.enabled": "false"},
			Expected: map[string]string{"ERROR": "requires zeebe.enabled=true"},
		}, {
			Name:     "TestOperateDisabled",
			Values:   map[string]string{"operate.enabled": "false"},
			Expected: map[string]string{"ERROR": "requires operate.enabled=true"},
		}, {
			Name:     "TestTasklistDisabled",
			Values:   map[string]string{"tasklist.enabled": "false"},
			Expected: map[string]string{"ERROR": "requires tasklist.enabled=true"},
		},
	}

	for i := range testCases {
		testCases[i].RenderTemplateExtraArgs = orchestrationValues
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

// TestCombinedTopologyPreservesEnabledValues is the counterweight to the
// constraints above: switching the same values back to combined mode must
// leave the Hub-plane components deployable.
func (s *TopologyTest) TestCombinedTopologyPreservesEnabledValues() {
	var testCases []testhelpers.TestCase
	for _, tc := range []struct{ component, template string }{
		{"Identity", "templates/identity/service.yaml"},
		{"Console", "templates/console/service.yaml"},
		{"WebModeler", "templates/web-modeler/service-restapi.yaml"},
	} {
		template := tc.template
		testCases = append(testCases, testhelpers.TestCase{
			Name:          "TestCombinedModeRenders" + tc.component + "Service",
			CaseTemplates: &testhelpers.CaseTemplate{Templates: []string{template}},
			Values: map[string]string{
				"global.topology.mode":                "combined",
				"identity.enabled":                    "true",
				"webModeler.restapi.mail.fromAddress": "noreply@example.com",
			},
			RenderTemplateExtraArgs: orchestrationValues,
			Verifier: func(t *testing.T, output string, err error) {
				s.Require().NoError(err)
				s.Require().Contains(output, "kind: Service")
			},
		})
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}
