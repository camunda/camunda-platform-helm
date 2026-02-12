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

package orchestration

import (
	"camunda-platform/test/unit/testhelpers"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type GRPCRouteTemplateTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
	templates []string
}

func TestGRPCRouteTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &GRPCRouteTemplateTest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
		templates: []string{"templates/orchestration/grpcroute.yaml"},
	})
}

func (s *GRPCRouteTemplateTest) TestDifferentValuesInputs() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestGRPCRouteNotRenderedWhenGatewayDisabled",
			Values: map[string]string{
				"global.gateway.enabled": "false",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NotContains(t, output, "kind: GRPCRoute")
			},
		},
		{
			Name: "TestGRPCRouteNotRenderedWhenGatewayExternal",
			Values: map[string]string{
				"global.gateway.enabled":          "true",
				"global.gateway.external":         "true",
				"global.host":         "camunda.example.com",
				"orchestration.ingress.grpc.host": "grpc-camunda.example.com",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NotContains(t, output, "kind: GRPCRoute")
			},
		},
		{
			Name: "TestGRPCRouteNotRenderedWhenOrchestrationDisabled",
			Values: map[string]string{
				"global.gateway.enabled":          "true",
				"global.host":         "camunda.example.com",
				"orchestration.enabled":           "false",
				"orchestration.ingress.grpc.host": "grpc-camunda.example.com",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NotContains(t, output, "kind: GRPCRoute")
			},
		},
		{
			Name: "TestGRPCRouteRendered",
			Values: map[string]string{
				"global.gateway.enabled":          "true",
				"global.host":         "camunda.example.com",
				"orchestration.enabled":           "true",
				"orchestration.ingress.grpc.host": "grpc-camunda.example.com",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "kind: GRPCRoute")
				require.Contains(t, output, "apiVersion: gateway.networking.k8s.io/v1")
				require.Contains(t, output, "name: camunda-platform-test-grpc")
				require.Contains(t, output, "grpc-camunda.example.com")
				require.Contains(t, output, "port: 26500")
			},
		},
		{
			Name: "TestGRPCRouteParentRef",
			Values: map[string]string{
				"global.gateway.enabled":          "true",
				"global.host":         "camunda.example.com",
				"orchestration.enabled":           "true",
				"orchestration.ingress.grpc.host": "grpc-camunda.example.com",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				// parentRef should reference the Gateway by name
				require.Contains(t, output, "name: camunda-platform-test")
			},
		},
		{
			Name: "TestGRPCRouteWithGlobalAnnotations",
			Values: map[string]string{
				"global.gateway.enabled":          "true",
				"global.host":         "camunda.example.com",
				"orchestration.enabled":           "true",
				"orchestration.ingress.grpc.host": "grpc-camunda.example.com",
				"global.annotations.global-key":   "global-value",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "global-key: global-value")
			},
		},
		{
			Name: "TestGRPCRouteWithGatewayAnnotations",
			Values: map[string]string{
				"global.gateway.enabled":                 "true",
				"global.host":                "camunda.example.com",
				"orchestration.enabled":                  "true",
				"orchestration.ingress.grpc.host":        "grpc-camunda.example.com",
				"global.gateway.annotations.gateway-key": "gateway-value",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "gateway-key: gateway-value")
			},
		},
		{
			Name: "TestGRPCRouteWithBothAnnotations",
			Values: map[string]string{
				"global.gateway.enabled":                 "true",
				"global.host":                "camunda.example.com",
				"orchestration.enabled":                  "true",
				"orchestration.ingress.grpc.host":        "grpc-camunda.example.com",
				"global.annotations.global-key":          "global-value",
				"global.gateway.annotations.gateway-key": "gateway-value",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "global-key: global-value")
				require.Contains(t, output, "gateway-key: gateway-value")
			},
		},
		{
			Name: "TestGRPCRouteCustomHost",
			Values: map[string]string{
				"global.gateway.enabled":          "true",
				"global.host":         "camunda.example.com",
				"orchestration.enabled":           "true",
				"orchestration.ingress.grpc.host": "custom-grpc.example.com",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "custom-grpc.example.com")
			},
		},
		{
			Name: "TestGRPCRouteBackendRefPort",
			Values: map[string]string{
				"global.gateway.enabled":          "true",
				"global.host":         "camunda.example.com",
				"orchestration.enabled":           "true",
				"orchestration.ingress.grpc.host": "grpc-camunda.example.com",
				"orchestration.service.grpcPort":  "9090",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "port: 9090")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}
