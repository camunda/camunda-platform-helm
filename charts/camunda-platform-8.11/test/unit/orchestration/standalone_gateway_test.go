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

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gopkg.in/yaml.v3"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
)

const (
	standaloneGatewayName       = "camunda-platform-test-zeebe-standalone-gateway"
	orchestrationMainService    = "camunda-platform-test-zeebe-gateway"
	standaloneGatewayDeployment = "templates/orchestration/standalone-gateway-deployment.yaml"
	standaloneGatewayConfigmap  = "templates/orchestration/standalone-gateway-configmap.yaml"
)

var standaloneGatewayTemplates = []string{
	standaloneGatewayDeployment,
	standaloneGatewayConfigmap,
	"templates/orchestration/standalone-gateway-service.yaml",
	"templates/orchestration/standalone-gateway-serviceaccount.yaml",
	"templates/orchestration/standalone-gateway-poddisruptionbudget.yaml",
	"templates/orchestration/standalone-gateway-hpa.yaml",
	"templates/orchestration/standalone-gateway-httproute.yaml",
	"templates/common/ingress-standalone-gateway-rest.yaml",
	"templates/service-monitor/orchestration-standalone-gateway-service-monitor.yaml",
}

type StandaloneGatewayTemplateTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
}

func TestStandaloneGatewayTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &StandaloneGatewayTemplateTest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
	})
}

func withStandaloneGateway(values map[string]string) map[string]string {
	merged := map[string]string{"orchestration.standaloneGateway.enabled": "true"}
	for key, value := range values {
		merged[key] = value
	}
	return merged
}

func standaloneGatewayApplication(t *testing.T, output string) map[string]any {
	t.Helper()
	var configmap corev1.ConfigMap
	helm.UnmarshalK8SYaml(t, output, &configmap)
	var application map[string]any
	require.NoError(t, yaml.Unmarshal([]byte(configmap.Data["application.yaml"]), &application))
	return application
}

func nested(t *testing.T, root map[string]any, path ...string) any {
	t.Helper()
	var current any = root
	for _, key := range path {
		currentMap, ok := current.(map[string]any)
		require.Truef(t, ok, "path %v: %q is not a map", path, key)
		current = currentMap[key]
	}
	return current
}

func (s *StandaloneGatewayTemplateTest) TestDisabledByDefault() {
	for _, template := range standaloneGatewayTemplates {
		s.Run(template, func() {
			output, _ := testhelpers.RenderTestCaseE(s.T(), s.chartPath, s.release, s.namespace, nil, testhelpers.TestCase{
				Template: template,
				Values: map[string]string{
					"prometheusServiceMonitor.enabled":                     "true",
					"global.gateway.enabled":                               "true",
					"global.host":                                          "camunda.example.com",
					"orchestration.standaloneGateway.gateway.rest.enabled": "true",
					"orchestration.standaloneGateway.ingress.rest.enabled": "true",
					"orchestration.standaloneGateway.ingress.rest.host":    "api.example.com",
					"orchestration.standaloneGateway.autoscaling.enabled":  "true",
				},
			})
			s.Require().NotContains(output, standaloneGatewayName)
		})
	}
}

func (s *StandaloneGatewayTemplateTest) TestDifferentValuesInputs() {
	testCases := []testhelpers.TestCase{
		{
			Name:     "TestReplicasAreIndependentOfClusterSize",
			Template: standaloneGatewayDeployment,
			Values: withStandaloneGateway(map[string]string{
				"orchestration.standaloneGateway.replicas": "4",
			}),
			RenderTemplateExtraArgs: []string{
				"--set-string", "orchestration.clusterSize=1",
				"--set-string", "orchestration.partitionCount=1",
				"--set-string", "orchestration.replicationFactor=1",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)
				require.Equal(t, standaloneGatewayName, deployment.Name)
				require.NotNil(t, deployment.Spec.Replicas)
				require.EqualValues(t, 4, *deployment.Spec.Replicas)
				require.Equal(t, "zeebe-standalone-gateway", deployment.Spec.Selector.MatchLabels["app.kubernetes.io/component"])
				require.Equal(t, "zeebe-standalone-gateway", deployment.Spec.Template.Labels["app.kubernetes.io/component"])
			},
		},
		{
			Name:     "TestBrokerStatefulSetFollowsClusterSize",
			Template: "templates/orchestration/statefulset.yaml",
			Values: withStandaloneGateway(map[string]string{
				"orchestration.standaloneGateway.replicas": "4",
			}),
			RenderTemplateExtraArgs: []string{
				"--set-string", "orchestration.clusterSize=1",
				"--set-string", "orchestration.partitionCount=1",
				"--set-string", "orchestration.replicationFactor=1",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var statefulSet appsv1.StatefulSet
				helm.UnmarshalK8SYaml(t, output, &statefulSet)
				require.EqualValues(t, 1, *statefulSet.Spec.Replicas)
			},
		},
		{
			Name:     "TestDeploymentContainer",
			Template: standaloneGatewayDeployment,
			Values:   withStandaloneGateway(nil),
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)
				require.Len(t, deployment.Spec.Template.Spec.Containers, 1)
				container := deployment.Spec.Template.Spec.Containers[0]
				require.Empty(t, container.Command, "the image entrypoint starts the gateway")

				env := map[string]corev1.EnvVar{}
				for _, entry := range container.Env {
					env[entry.Name] = entry
				}
				require.Equal(t, "status.podIP", env["K8S_POD_IP"].ValueFrom.FieldRef.FieldPath)
				require.Equal(t, "metadata.name", env["K8S_NAME"].ValueFrom.FieldRef.FieldPath)
				require.Equal(t, "camunda-platform-test-zeebe", env["K8S_SERVICE_NAME"].Value)
				require.Contains(t, env["JAVA_TOOL_OPTIONS"].Value, "-XX:HeapDumpPath=/tmp")

				ports := map[string]int32{}
				for _, port := range container.Ports {
					ports[port.Name] = port.ContainerPort
				}
				require.Equal(t, map[string]int32{"http": 8080, "internal": 26502, "server": 9600, "grpc": 26500}, ports)

				require.NotNil(t, container.ReadinessProbe)
				require.Equal(t, "/actuator/health/readiness", container.ReadinessProbe.HTTPGet.Path)
				require.EqualValues(t, 9600, container.ReadinessProbe.HTTPGet.Port.IntValue())

				mounts := map[string]string{}
				for _, mount := range container.VolumeMounts {
					mounts[mount.MountPath] = mount.Name
				}
				require.Equal(t, "config", mounts["/usr/local/camunda/config/application.yaml"])
				require.NotContains(t, mounts, "/usr/local/camunda/data")
				require.Equal(t, "camunda-platform-test-zeebe", deployment.Spec.Template.Spec.ServiceAccountName)
			},
		},
		{
			Name:     "TestDeploymentReceivesOrchestrationAndGatewayEnv",
			Template: standaloneGatewayDeployment,
			Values: withStandaloneGateway(map[string]string{
				"orchestration.env[0].name":                    "FROM_ORCHESTRATION",
				"orchestration.env[0].value":                   "a",
				"orchestration.standaloneGateway.env[0].name":  "FROM_GATEWAY",
				"orchestration.standaloneGateway.env[0].value": "b",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)
				names := []string{}
				for _, entry := range deployment.Spec.Template.Spec.Containers[0].Env {
					names = append(names, entry.Name)
				}
				require.Contains(t, names, "FROM_ORCHESTRATION")
				require.Contains(t, names, "FROM_GATEWAY")
			},
		},
		{
			Name:     "TestStatefulSetDoesNotReceiveGatewayEnv",
			Template: "templates/orchestration/statefulset.yaml",
			Values: withStandaloneGateway(map[string]string{
				"orchestration.standaloneGateway.env[0].name":  "FROM_GATEWAY",
				"orchestration.standaloneGateway.env[0].value": "b",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.NotContains(t, output, "FROM_GATEWAY")
			},
		},
		{
			Name:     "TestAutoscalingOmitsReplicas",
			Template: standaloneGatewayDeployment,
			Values: withStandaloneGateway(map[string]string{
				"orchestration.standaloneGateway.autoscaling.enabled": "true",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)
				require.Nil(t, deployment.Spec.Replicas)
			},
		},
		{
			Name:     "TestAutoscalingRendersHPA",
			Template: "templates/orchestration/standalone-gateway-hpa.yaml",
			Values: withStandaloneGateway(map[string]string{
				"orchestration.standaloneGateway.autoscaling.enabled":     "true",
				"orchestration.standaloneGateway.autoscaling.minReplicas": "3",
				"orchestration.standaloneGateway.autoscaling.maxReplicas": "7",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var hpa autoscalingv2.HorizontalPodAutoscaler
				helm.UnmarshalK8SYaml(t, output, &hpa)
				require.Equal(t, "Deployment", hpa.Spec.ScaleTargetRef.Kind)
				require.Equal(t, standaloneGatewayName, hpa.Spec.ScaleTargetRef.Name)
				require.EqualValues(t, 3, *hpa.Spec.MinReplicas)
				require.EqualValues(t, 7, hpa.Spec.MaxReplicas)
				require.Len(t, hpa.Spec.Metrics, 1)
			},
		},
		{
			Name:     "TestServiceSelectsGatewayPods",
			Template: "templates/orchestration/standalone-gateway-service.yaml",
			Values:   withStandaloneGateway(nil),
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var service corev1.Service
				helm.UnmarshalK8SYaml(t, output, &service)
				require.Equal(t, standaloneGatewayName, service.Name)
				require.Equal(t, "zeebe-standalone-gateway", service.Spec.Selector["app.kubernetes.io/component"])
				ports := map[string]int32{}
				for _, port := range service.Spec.Ports {
					ports[port.Name] = port.Port
				}
				require.Equal(t, map[string]int32{"http": 8080, "server": 9600, "grpc": 26500}, ports)
			},
		},
		{
			Name:     "TestMainServiceStillSelectsBrokers",
			Template: "templates/orchestration/service.yaml",
			Values:   withStandaloneGateway(nil),
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var service corev1.Service
				helm.UnmarshalK8SYaml(t, output, &service)
				require.Equal(t, orchestrationMainService, service.Name)
				require.Equal(t, "zeebe-broker", service.Spec.Selector["app.kubernetes.io/component"])
			},
		},
		{
			Name:     "TestGatewayConfigurationBasicAuth",
			Template: standaloneGatewayConfigmap,
			Values:   withStandaloneGateway(nil),
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				application := standaloneGatewayApplication(t, output)
				require.Equal(t, "gateway,standalone,consolidated-auth", nested(t, application, "spring", "profiles", "active"))
				require.Equal(t, "${K8S_NAME}", nested(t, application, "camunda", "cluster", "gateway-id"))
				require.Equal(t, "${K8S_POD_IP}", nested(t, application, "camunda", "cluster", "network", "advertised-host"))
				require.Equal(t, 26502, nested(t, application, "camunda", "cluster", "network", "internal-api", "port"))
				require.Equal(t, []any{
					"camunda-platform-test-zeebe-0.${K8S_SERVICE_NAME}:26502",
					"camunda-platform-test-zeebe-1.${K8S_SERVICE_NAME}:26502",
					"camunda-platform-test-zeebe-2.${K8S_SERVICE_NAME}:26502",
				}, nested(t, application, "camunda", "cluster", "initial-contact-points"))
				require.Equal(t, "basic", nested(t, application, "camunda", "security", "authentication", "method"))
				require.Equal(t, "elasticsearch", nested(t, application, "camunda", "data", "secondary-storage", "type"))

				cluster := nested(t, application, "camunda", "cluster").(map[string]any)
				for _, brokerOnly := range []string{"node-id", "size", "replication-factor", "partition-count"} {
					require.NotContains(t, cluster, brokerOnly)
				}
				require.NotContains(t, nested(t, application, "camunda", "cluster", "network").(map[string]any), "command-api")
				require.NotContains(t, nested(t, application, "camunda").(map[string]any), "system")
				require.NotContains(t, nested(t, application, "camunda", "data").(map[string]any), "primary-storage")
				require.NotContains(t, application, "zeebe")
				require.NotContains(t, application, "operate")
				require.NotContains(t, application, "tasklist")
			},
		},
		{
			Name:     "TestGatewayConfigurationOIDC",
			Template: standaloneGatewayConfigmap,
			Values: withStandaloneGateway(map[string]string{
				"global.security.authentication.method":                          "oidc",
				"orchestration.security.authentication.oidc.secret.inlineSecret": "secret",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				application := standaloneGatewayApplication(t, output)
				require.Equal(t, "gateway,standalone,consolidated-auth", nested(t, application, "spring", "profiles", "active"))
				require.Equal(t, "oidc", nested(t, application, "camunda", "security", "authentication", "method"))
				require.NotNil(t, nested(t, application, "camunda", "security", "authentication", "oidc", "client-id"))
			},
		},
		{
			Name:     "TestGatewayConfigurationOverride",
			Template: standaloneGatewayConfigmap,
			Values: withStandaloneGateway(map[string]string{
				"orchestration.configuration":                   "broker: only",
				"orchestration.standaloneGateway.configuration": "gateway: only",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(t, output, &configmap)
				require.Equal(t, "gateway: only", strings.TrimSpace(configmap.Data["application.yaml"]))
			},
		},
		{
			Name:     "TestBrokerConfigurationUnchangedWhenEnabled",
			Template: "templates/orchestration/configmap.yaml",
			Values:   withStandaloneGateway(nil),
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var configmap corev1.ConfigMap
				helm.UnmarshalK8SYaml(t, output, &configmap)
				var application map[string]any
				require.NoError(t, yaml.Unmarshal([]byte(configmap.Data["application.yaml"]), &application))
				require.Equal(t, true, nested(t, application, "zeebe", "broker", "gateway", "enable"))
				require.NotContains(t, nested(t, application, "spring", "profiles", "active"), "gateway")
			},
		},
		{
			Name:     "TestGRPCIngressTargetsStandaloneGateway",
			Template: "templates/common/ingress-grpc.yaml",
			Values: withStandaloneGateway(map[string]string{
				"orchestration.ingress.grpc.enabled": "true",
				"orchestration.ingress.grpc.host":    "grpc.example.com",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var ingress networkingv1.Ingress
				helm.UnmarshalK8SYaml(t, output, &ingress)
				require.Equal(t, standaloneGatewayName, ingress.Spec.Rules[0].HTTP.Paths[0].Backend.Service.Name)
			},
		},
		{
			Name:     "TestGRPCIngressTargetsMainServiceWhenDisabled",
			Template: "templates/common/ingress-grpc.yaml",
			Values: map[string]string{
				"orchestration.ingress.grpc.enabled": "true",
				"orchestration.ingress.grpc.host":    "grpc.example.com",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var ingress networkingv1.Ingress
				helm.UnmarshalK8SYaml(t, output, &ingress)
				require.Equal(t, orchestrationMainService, ingress.Spec.Rules[0].HTTP.Paths[0].Backend.Service.Name)
			},
		},
		{
			Name:     "TestWebIngressStaysOnMainService",
			Template: "templates/common/ingress-http.yaml",
			Values: withStandaloneGateway(map[string]string{
				"global.ingress.enabled":    "true",
				"global.host":               "camunda.example.com",
				"orchestration.contextPath": "/orchestration",
				"orchestration.standaloneGateway.ingress.rest.enabled": "true",
				"orchestration.standaloneGateway.ingress.rest.host":    "api.example.com",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var ingress networkingv1.Ingress
				helm.UnmarshalK8SYaml(t, output, &ingress)
				backends := map[string]string{}
				for _, path := range ingress.Spec.Rules[0].HTTP.Paths {
					backends[path.Path] = path.Backend.Service.Name
				}
				require.Equal(t, orchestrationMainService, backends["/orchestration"])
				require.NotContains(t, output, standaloneGatewayName)
			},
		},
		{
			Name:     "TestGRPCRouteTargetsStandaloneGateway",
			Template: "templates/orchestration/grpcroute.yaml",
			Values: withStandaloneGateway(map[string]string{
				"global.gateway.enabled":             "true",
				"global.host":                        "camunda.example.com",
				"orchestration.gateway.grpc.enabled": "true",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "- name: "+standaloneGatewayName)
			},
		},
		{
			Name:     "TestWebHTTPRouteStaysOnMainService",
			Template: "templates/orchestration/httproute.yaml",
			Values: withStandaloneGateway(map[string]string{
				"global.gateway.enabled": "true",
				"global.host":            "camunda.example.com",
				"orchestration.standaloneGateway.gateway.rest.enabled": "true",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "- name: "+orchestrationMainService)
				require.NotContains(t, output, standaloneGatewayName)
			},
		},
		{
			Name:     "TestRESTIngressNotRenderedUnlessEnabled",
			Template: "templates/common/ingress-standalone-gateway-rest.yaml",
			Values:   withStandaloneGateway(map[string]string{"orchestration.standaloneGateway.ingress.rest.host": "api.example.com"}),
			Verifier: func(t *testing.T, output string, err error) {
				require.NotContains(t, output, "kind: Ingress")
			},
		},
		{
			Name:     "TestRESTIngressTargetsStandaloneGateway",
			Template: "templates/common/ingress-standalone-gateway-rest.yaml",
			Values: withStandaloneGateway(map[string]string{
				"orchestration.standaloneGateway.ingress.rest.enabled":     "true",
				"orchestration.standaloneGateway.ingress.rest.host":        "api.example.com",
				"orchestration.standaloneGateway.ingress.rest.tls.enabled": "true",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var ingress networkingv1.Ingress
				helm.UnmarshalK8SYaml(t, output, &ingress)
				require.Equal(t, "camunda-platform-test-standalone-gateway-rest", ingress.Name)
				require.Equal(t, "api.example.com", ingress.Spec.Rules[0].Host)
				backend := ingress.Spec.Rules[0].HTTP.Paths[0].Backend.Service
				require.Equal(t, standaloneGatewayName, backend.Name)
				require.EqualValues(t, 8080, backend.Port.Number)
				require.Equal(t, []string{"api.example.com"}, ingress.Spec.TLS[0].Hosts)
			},
		},
		{
			Name:     "TestRESTIngressNotRenderedWhenExternal",
			Template: "templates/common/ingress-standalone-gateway-rest.yaml",
			Values: withStandaloneGateway(map[string]string{
				"orchestration.standaloneGateway.ingress.rest.enabled":  "true",
				"orchestration.standaloneGateway.ingress.rest.external": "true",
				"orchestration.standaloneGateway.ingress.rest.host":     "api.example.com",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				require.NotContains(t, output, "kind: Ingress")
			},
		},
		{
			Name:     "TestRESTHTTPRouteNotRenderedUnlessEnabled",
			Template: "templates/orchestration/standalone-gateway-httproute.yaml",
			Values: withStandaloneGateway(map[string]string{
				"global.gateway.enabled": "true",
				"global.host":            "camunda.example.com",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				require.NotContains(t, output, "kind: HTTPRoute")
			},
		},
		{
			Name:     "TestRESTHTTPRouteTargetsStandaloneGateway",
			Template: "templates/orchestration/standalone-gateway-httproute.yaml",
			Values: withStandaloneGateway(map[string]string{
				"global.gateway.enabled":     "true",
				"global.gateway.tls.enabled": "true",
				"global.host":                "camunda.example.com",
				"orchestration.standaloneGateway.gateway.rest.enabled": "true",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "kind: HTTPRoute")
				require.Contains(t, output, `- "api-camunda.example.com"`)
				require.Contains(t, output, "sectionName: standalone-rests")
				require.Contains(t, output, "- name: "+standaloneGatewayName)
				require.Contains(t, output, "port: 8080")
			},
		},
		{
			Name:     "TestGatewayResourceListenerForRESTRoute",
			Template: "templates/common/gateway.yaml",
			Values: withStandaloneGateway(map[string]string{
				"global.gateway.enabled":                               "true",
				"global.gateway.createGatewayResource":                 "true",
				"global.host":                                          "camunda.example.com",
				"orchestration.standaloneGateway.gateway.rest.enabled": "true",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "- name: standalone-rest\n")
				require.Contains(t, output, "hostname: api-camunda.example.com")
			},
		},
		{
			Name:     "TestConnectorsTargetStandaloneGateway",
			Template: "templates/connectors/configmap.yaml",
			Values:   withStandaloneGateway(nil),
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "rest-address: http://"+standaloneGatewayName+":8080")
				require.Contains(t, output, "grpc-address: http://"+standaloneGatewayName+":26500")
			},
		},
		{
			Name:     "TestReleaseInfoShowsRESTEntryPoint",
			Template: "templates/common/configmap-release.yaml",
			Values: withStandaloneGateway(map[string]string{
				"orchestration.standaloneGateway.ingress.rest.enabled":     "true",
				"orchestration.standaloneGateway.ingress.rest.host":        "api.example.com",
				"orchestration.standaloneGateway.ingress.rest.tls.enabled": "true",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "rest: https://api.example.com/")
			},
		},
		{
			Name:     "TestServiceMonitorSelectsGatewayService",
			Template: "templates/service-monitor/orchestration-standalone-gateway-service-monitor.yaml",
			Values:   withStandaloneGateway(map[string]string{"prometheusServiceMonitor.enabled": "true"}),
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "app.kubernetes.io/component: zeebe-standalone-gateway")
				require.Contains(t, output, "path: /actuator/prometheus")
			},
		},
		{
			Name:     "TestDedicatedServiceAccount",
			Template: "templates/orchestration/standalone-gateway-serviceaccount.yaml",
			Values: withStandaloneGateway(map[string]string{
				"orchestration.standaloneGateway.serviceAccount.enabled": "true",
			}),
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var serviceAccount corev1.ServiceAccount
				helm.UnmarshalK8SYaml(t, output, &serviceAccount)
				require.Equal(t, standaloneGatewayName, serviceAccount.Name)
			},
		},
		{
			Name:     "TestConstraintRequiresAuthentication",
			Template: standaloneGatewayDeployment,
			Values: withStandaloneGateway(map[string]string{
				"global.security.authentication.method": "none",
			}),
			Expected: map[string]string{"ERROR": "orchestration.standaloneGateway.enabled requires the Orchestration Cluster authentication method to be basic or oidc"},
		},
		{
			Name:     "TestConstraintRequiresSecondaryStorage",
			Template: standaloneGatewayDeployment,
			Values: withStandaloneGateway(map[string]string{
				"global.noSecondaryStorage":                "true",
				"orchestration.data.secondaryStorage.type": "null",
				"optimize.enabled":                         "false",
			}),
			Expected: map[string]string{"ERROR": "orchestration.standaloneGateway.enabled is not supported with global.noSecondaryStorage=true"},
		},
		{
			Name:     "TestConstraintRequiresBrokerProfile",
			Template: standaloneGatewayDeployment,
			Values: withStandaloneGateway(map[string]string{
				"orchestration.profiles.broker": "false",
			}),
			Expected: map[string]string{"ERROR": "orchestration.standaloneGateway.enabled requires orchestration.profiles.broker=true"},
		},
		{
			Name:     "TestConstraintAutoscalingBounds",
			Template: standaloneGatewayDeployment,
			Values: withStandaloneGateway(map[string]string{
				"orchestration.standaloneGateway.autoscaling.enabled":     "true",
				"orchestration.standaloneGateway.autoscaling.minReplicas": "6",
				"orchestration.standaloneGateway.autoscaling.maxReplicas": "5",
			}),
			Expected: map[string]string{"ERROR": "orchestration.standaloneGateway.autoscaling.minReplicas must not be greater than"},
		},
		{
			Name:     "TestConstraintRESTIngressNeedsHost",
			Template: standaloneGatewayDeployment,
			Values: withStandaloneGateway(map[string]string{
				"orchestration.standaloneGateway.ingress.rest.enabled": "true",
			}),
			Expected: map[string]string{"ERROR": "orchestration.standaloneGateway.ingress.rest.enabled requires orchestration.standaloneGateway.ingress.rest.host"},
		},
		{
			Name:     "TestConstraintRESTIngressHostDiffersFromGlobalHost",
			Template: standaloneGatewayDeployment,
			Values: withStandaloneGateway(map[string]string{
				"global.host": "camunda.example.com",
				"orchestration.standaloneGateway.ingress.rest.enabled": "true",
				"orchestration.standaloneGateway.ingress.rest.host":    "camunda.example.com",
			}),
			Expected: map[string]string{"ERROR": "orchestration.standaloneGateway.ingress.rest.host must differ from global.host"},
		},
		{
			Name:     "TestConstraintRESTRouteHostDiffersFromGlobalHost",
			Template: standaloneGatewayDeployment,
			Values: withStandaloneGateway(map[string]string{
				"global.gateway.enabled": "true",
				"global.host":            "camunda.example.com",
				"orchestration.standaloneGateway.gateway.rest.enabled": "true",
				"orchestration.standaloneGateway.gateway.rest.host":    "camunda.example.com",
			}),
			Expected: map[string]string{"ERROR": "orchestration.standaloneGateway.gateway.rest.host must be set and differ from global.host"},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, nil, testCases)
}
