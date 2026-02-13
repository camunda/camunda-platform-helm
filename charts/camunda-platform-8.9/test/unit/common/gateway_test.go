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
	"path/filepath"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type GatewayTemplateTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
	templates []string
}

func TestGatewayTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &GatewayTemplateTest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
		templates: []string{"templates/common/gateway.yaml"},
	})
}

func (s *GatewayTemplateTest) TestDifferentValuesInputs() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TestGatewayDisabledByDefault",
			Values: map[string]string{
				"global.gateway.enabled": "false",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NotContains(t, output, "kind: Gateway")
			},
		},
		{
			Name: "TestGatewayNotRenderedWhenExternal",
			Values: map[string]string{
				"global.gateway.enabled":               "true",
				"global.gateway.external":              "true",
				"global.gateway.createGatewayResource": "true",
				"global.host":                          "camunda.example.com",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NotContains(t, output, "kind: Gateway")
			},
		},
		{
			Name: "TestGatewayNotRenderedWhenCreateGatewayResourceFalse",
			Values: map[string]string{
				"global.gateway.enabled":               "true",
				"global.gateway.external":              "false",
				"global.gateway.createGatewayResource": "false",
				"global.host":                          "camunda.example.com",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NotContains(t, output, "kind: Gateway")
			},
		},
		{
			Name: "TestGatewayRenderedWithHTTPListenersNoGRPC",
			Values: map[string]string{
				"global.gateway.enabled":               "true",
				"global.gateway.createGatewayResource": "true",
				"global.host":                          "camunda.example.com",
				"global.gateway.tls.enabled":           "false",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "kind: Gateway")
				require.Contains(t, output, "apiVersion: gateway.networking.k8s.io/v1")
				require.Contains(t, output, "name: camunda-platform-test")
				require.Contains(t, output, "gatewayClassName: nginx")
				// HTTP listener
				require.Contains(t, output, "name: http")
				require.Contains(t, output, "port: 80")
				require.Contains(t, output, "protocol: HTTP")
				require.Contains(t, output, "hostname: camunda.example.com")
				// gRPC listener should NOT be present when grpc.enabled is false (default)
				require.NotContains(t, output, "name: grpc")
				// Should NOT contain HTTPS-related config
				require.NotContains(t, output, "name: https")
				require.NotContains(t, output, "name: grpcs")
				require.NotContains(t, output, "port: 443")
				require.NotContains(t, output, "protocol: HTTPS")
			},
		},
		{
			Name: "TestGatewayRenderedWithHTTPListenersAndGRPC",
			Values: map[string]string{
				"global.gateway.enabled":               "true",
				"global.gateway.createGatewayResource": "true",
				"global.host":                          "camunda.example.com",
				"global.gateway.tls.enabled":           "false",
				"orchestration.gateway.grpc.enabled":   "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "kind: Gateway")
				// HTTP listener
				require.Contains(t, output, "name: http")
				require.Contains(t, output, "port: 80")
				require.Contains(t, output, "protocol: HTTP")
				require.Contains(t, output, "hostname: camunda.example.com")
				// gRPC listener should be present when grpc.enabled is true
				require.Contains(t, output, "name: grpc")
				require.Contains(t, output, "hostname: grpc-camunda.example.com")
			},
		},
		{
			Name: "TestGatewayRenderedWithHTTPSListenersNoGRPC",
			Values: map[string]string{
				"global.gateway.enabled":               "true",
				"global.gateway.createGatewayResource": "true",
				"global.host":                          "camunda.example.com",
				"global.gateway.tls.enabled":           "true",
				"global.gateway.tls.secretName":        "my-tls-secret",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "kind: Gateway")
				// HTTPS listener
				require.Contains(t, output, "name: https")
				require.Contains(t, output, "port: 443")
				require.Contains(t, output, "protocol: HTTPS")
				require.Contains(t, output, "hostname: camunda.example.com")
				require.Contains(t, output, "mode: Terminate")
				require.Contains(t, output, "name: my-tls-secret")
				// gRPCs listener should NOT be present when grpc.enabled is false (default)
				require.NotContains(t, output, "name: grpcs")
				// Should NOT contain HTTP-only listener config
				require.NotContains(t, output, "port: 80")
				require.NotContains(t, output, "protocol: HTTP\n")
			},
		},
		{
			Name: "TestGatewayRenderedWithHTTPSListenersAndGRPC",
			Values: map[string]string{
				"global.gateway.enabled":               "true",
				"global.gateway.createGatewayResource": "true",
				"global.host":                          "camunda.example.com",
				"global.gateway.tls.enabled":           "true",
				"global.gateway.tls.secretName":        "my-tls-secret",
				"orchestration.gateway.grpc.enabled":   "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "kind: Gateway")
				// HTTPS listener
				require.Contains(t, output, "name: https")
				require.Contains(t, output, "port: 443")
				require.Contains(t, output, "protocol: HTTPS")
				require.Contains(t, output, "hostname: camunda.example.com")
				require.Contains(t, output, "mode: Terminate")
				require.Contains(t, output, "name: my-tls-secret")
				// gRPCs listener should be present when grpc.enabled is true
				require.Contains(t, output, "name: grpcs")
				require.Contains(t, output, "hostname: grpc-camunda.example.com")
				// Should NOT contain HTTP-only listener config
				require.NotContains(t, output, "port: 80")
				require.NotContains(t, output, "protocol: HTTP\n")
			},
		},
		{
			Name: "TestGatewayGRPCListenerCustomHost",
			Values: map[string]string{
				"global.gateway.enabled":               "true",
				"global.gateway.createGatewayResource": "true",
				"global.host":                          "camunda.example.com",
				"global.gateway.tls.enabled":           "false",
				"orchestration.gateway.grpc.enabled":   "true",
				"orchestration.gateway.grpc.host":      "custom-grpc.example.com",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "name: grpc")
				require.Contains(t, output, "hostname: custom-grpc.example.com")
			},
		},
		{
			Name: "TestGatewayCustomClassName",
			Values: map[string]string{
				"global.gateway.enabled":               "true",
				"global.gateway.createGatewayResource": "true",
				"global.host":                          "camunda.example.com",
				"global.gateway.className":             "istio",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "gatewayClassName: istio")
			},
		},
		{
			Name: "TestGatewayWithGlobalAnnotations",
			Values: map[string]string{
				"global.gateway.enabled":               "true",
				"global.gateway.createGatewayResource": "true",
				"global.host":                          "camunda.example.com",
				"global.annotations.global-key":        "global-value",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "global-key: global-value")
			},
		},
		{
			Name: "TestGatewayWithGatewayAnnotations",
			Values: map[string]string{
				"global.gateway.enabled":                 "true",
				"global.gateway.createGatewayResource":   "true",
				"global.host":                            "camunda.example.com",
				"global.gateway.annotations.gateway-key": "gateway-value",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "gateway-key: gateway-value")
			},
		},
		{
			Name: "TestGatewayWithBothGlobalAndGatewayAnnotations",
			Values: map[string]string{
				"global.gateway.enabled":                 "true",
				"global.gateway.createGatewayResource":   "true",
				"global.host":                            "camunda.example.com",
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
			Name: "TestGatewayAndIngressCannotBothBeEnabled",
			Values: map[string]string{
				"global.gateway.enabled":               "true",
				"global.gateway.createGatewayResource": "true",
				"global.host":                          "camunda.example.com",
				"global.ingress.enabled":               "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.ErrorContains(t, err, "Gateway API and Ingress cannot both be enabled at the same time")
			},
		},
		{
			Name: "TestGatewayTLSDefaultSecretName",
			Values: map[string]string{
				"global.gateway.enabled":               "true",
				"global.gateway.createGatewayResource": "true",
				"global.host":                          "camunda.example.com",
				"global.gateway.tls.enabled":           "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "name: camunda-platform")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}
