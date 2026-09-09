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

package optimize

import (
	"camunda-platform/test/unit/testhelpers"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

type OptimizeTLSTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
	templates []string
}

func TestOptimizeTLS(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &OptimizeTLSTest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
		templates: []string{"templates/optimize/deployment.yaml"},
	})
}

func (s *OptimizeTLSTest) mainContainer(deployment *appsv1.Deployment) *corev1.Container {
	for i := range deployment.Spec.Template.Spec.Containers {
		c := &deployment.Spec.Template.Spec.Containers[i]
		if c.Name == "optimize" {
			return c
		}
	}
	s.Require().Fail("main optimize container not found")
	return nil
}

func (s *OptimizeTLSTest) TestTLSEnvAndVolumeWiring() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TLS disabled (default) — no SSL env, no volume, no annotation",
			Values: map[string]string{
				"optimize.enabled": "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.NotContains(t, output, "SERVER_SSL_ENABLED")
				require.NotContains(t, output, "optimize-server-tls")
				require.NotContains(t, output, "checksum/optimize-tls")
			},
		},
		{
			Name: "TLS enabled via global.tls.optimize.enabled (PKCS12 defaults)",
			Values: map[string]string{
				"optimize.enabled":                               "true",
				"global.tls.optimize.enabled":                    "true",
				"global.tls.optimize.cert.secret.existingSecret": "optimize-ks",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				container := s.mainContainer(&deployment)
				// Optimize installs its own HTTPS connector from container.keystore.*;
				// server.ssl.* would add a duplicate SSLHostConfig and abort startup.
				require.NotContains(t, output, "SERVER_SSL_")
				s.Require().Contains(container.Env, corev1.EnvVar{Name: "CAMUNDA_OPTIMIZE_CONTAINER_KEYSTORE_LOCATION", Value: "/usr/local/camunda/certificates/optimize/keystore.p12"})
				// TLS takes over the existing named port; plaintext is refused.
				s.Require().Contains(container.Env, corev1.EnvVar{Name: "CAMUNDA_OPTIMIZE_CONTAINER_PORTS_HTTPS", Value: "8090"})
				s.Require().Contains(container.Env, corev1.EnvVar{Name: "CAMUNDA_OPTIMIZE_CONTAINER_PORTS_HTTP", Value: "-1"})
				s.Require().Contains(container.Env, corev1.EnvVar{
					Name: "CAMUNDA_OPTIMIZE_CONTAINER_KEYSTORE_PASSWORD",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "optimize-ks"},
							Key:                  "keystore-password",
						},
					},
				})

				var foundMount bool
				for _, m := range container.VolumeMounts {
					if m.Name == "optimize-server-tls" {
						foundMount = true
						s.Require().Equal("/usr/local/camunda/certificates/optimize", m.MountPath)
						s.Require().True(m.ReadOnly)
					}
				}
				s.Require().True(foundMount, "expected optimize-server-tls volumeMount")

				var foundVol bool
				for _, v := range deployment.Spec.Template.Spec.Volumes {
					if v.Name == "optimize-server-tls" {
						foundVol = true
						s.Require().Equal("optimize-ks", v.Secret.SecretName)
					}
				}
				s.Require().True(foundVol, "expected optimize-server-tls volume")
			},
		},
		{
			Name: "PKCS12 mode honors a custom keystore password key",
			Values: map[string]string{
				"optimize.enabled":                                              "true",
				"global.tls.optimize.enabled":                                   "true",
				"global.tls.optimize.cert.secret.existingSecret":                "optimize-ks",
				"global.tls.optimize.keystorePassword.secret.existingSecretKey": "ks-pw",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				container := s.mainContainer(&deployment)
				s.Require().Contains(container.Env, corev1.EnvVar{
					Name: "CAMUNDA_OPTIMIZE_CONTAINER_KEYSTORE_PASSWORD",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "optimize-ks"},
							Key:                  "ks-pw",
						},
					},
				})
			},
		},
		{
			// Optimize's SSLHostConfigCertificate is built with Type.UNDEFINED and
			// only a keystore file plus password, so there is nowhere to send an
			// alias. Failing beats serving a cert the operator did not select.
			Name: "keyAlias is rejected because Optimize has no key-alias setting",
			Values: map[string]string{
				"optimize.enabled":                               "true",
				"global.tls.optimize.enabled":                    "true",
				"global.tls.optimize.cert.secret.existingSecret": "optimize-ks",
				"global.tls.optimize.keyAlias":                   "optimize-rest",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "global.tls.optimize.keyAlias is not supported")
			},
		},
		{
			// Optimize loads its cert from a PKCS12 keystore only; a bare PEM
			// cert/key pair cannot be converted at render time.
			Name: "PEM type is rejected for the Optimize server",
			Values: map[string]string{
				"optimize.enabled":                               "true",
				"global.tls.optimize.enabled":                    "true",
				"global.tls.optimize.cert.secret.existingSecret": "cert-manager-tls",
				"global.tls.optimize.type":                       "pem",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), `global.tls.optimize.type="pem" is not supported for the Optimize server`)
			},
		},
		{
			Name: "cert block is inert when global.tls.optimize.enabled is false",
			Values: map[string]string{
				"optimize.enabled": "true",
				"global.tls.optimize.cert.secret.existingSecret": "should-not-mount",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.NotContains(t, output, "optimize-server-tls")
				require.NotContains(t, output, "SERVER_SSL_KEY_STORE")
			},
		},
		{
			Name: "Probe scheme switches to HTTPS when TLS enabled",
			Values: map[string]string{
				"optimize.enabled":                               "true",
				"global.tls.optimize.enabled":                    "true",
				"global.tls.optimize.cert.secret.existingSecret": "optimize-ks",
				"optimize.startupProbe.enabled":                  "true",
				"optimize.livenessProbe.enabled":                 "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				container := s.mainContainer(&deployment)
				s.Require().NotNil(container.StartupProbe)
				s.Require().Equal(corev1.URIScheme("HTTPS"), container.StartupProbe.HTTPGet.Scheme)
				s.Require().NotNil(container.ReadinessProbe)
				s.Require().Equal(corev1.URIScheme("HTTPS"), container.ReadinessProbe.HTTPGet.Scheme)
				s.Require().NotNil(container.LivenessProbe)
				s.Require().Equal(corev1.URIScheme("HTTPS"), container.LivenessProbe.HTTPGet.Scheme)
			},
		},
		{
			Name: "Probe scheme stays HTTP when TLS disabled",
			Values: map[string]string{
				"optimize.enabled": "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)
				container := s.mainContainer(&deployment)
				s.Require().NotNil(container.ReadinessProbe)
				s.Require().Equal(corev1.URIScheme("HTTP"), container.ReadinessProbe.HTTPGet.Scheme)
			},
		},
		{
			Name: "Probe scheme explicit override wins over TLS auto-flip for the overridden probe only",
			Values: map[string]string{
				"optimize.enabled":                               "true",
				"global.tls.optimize.enabled":                    "true",
				"global.tls.optimize.cert.secret.existingSecret": "optimize-ks",
				"optimize.startupProbe.enabled":                  "true",
				"optimize.livenessProbe.enabled":                 "true",
				"optimize.readinessProbe.scheme":                 "HTTP",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				container := s.mainContainer(&deployment)
				s.Require().NotNil(container.StartupProbe)
				s.Require().Equal(corev1.URIScheme("HTTPS"), container.StartupProbe.HTTPGet.Scheme)
				s.Require().NotNil(container.ReadinessProbe)
				s.Require().Equal(corev1.URIScheme("HTTP"), container.ReadinessProbe.HTTPGet.Scheme)
				s.Require().NotNil(container.LivenessProbe)
				s.Require().Equal(corev1.URIScheme("HTTPS"), container.LivenessProbe.HTTPGet.Scheme)
			},
		},
		{
			Name: "Regression: server-side optimize-server-tls coexists with client-side keystore volume for ES TLS",
			Values: map[string]string{
				"optimize.enabled":                                             "true",
				"global.tls.optimize.enabled":                                  "true",
				"global.tls.optimize.cert.secret.existingSecret":               "opt-ks",
				"optimize.database.elasticsearch.tls.secret.existingSecret":    "es-trust",
				"optimize.database.elasticsearch.tls.secret.existingSecretKey": "ca.crt",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				var optServerVol, keystoreVol *corev1.Volume
				for i := range deployment.Spec.Template.Spec.Volumes {
					v := &deployment.Spec.Template.Spec.Volumes[i]
					switch v.Name {
					case "optimize-server-tls":
						optServerVol = v
					case "keystore":
						keystoreVol = v
					}
				}
				s.Require().NotNil(optServerVol, "expected optimize-server-tls volume to be present")
				s.Require().NotNil(keystoreVol, "expected client-side keystore volume to still be present alongside server TLS")
				s.Require().Equal("opt-ks", optServerVol.Secret.SecretName)

				container := s.mainContainer(&deployment)
				var foundServerMount, foundKeystoreMount bool
				for _, m := range container.VolumeMounts {
					switch m.Name {
					case "optimize-server-tls":
						foundServerMount = true
					case "keystore":
						foundKeystoreMount = true
						s.Require().NotEmpty(m.SubPath, "client-side keystore mount must retain its subPath")
					}
				}
				s.Require().True(foundServerMount, "main container missing optimize-server-tls mount")
				s.Require().True(foundKeystoreMount, "main container missing client-side keystore mount")
			},
		},
		{
			Name: "Constraint fails when TLS enabled but no cert configured",
			Values: map[string]string{
				"optimize.enabled":            "true",
				"global.tls.optimize.enabled": "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "Optimize server TLS is enabled but no server cert is configured")
			},
		},
		{
			// The BackendTLSPolicy warning is emitted from camunda.constraints.warnings
			// via NOTES.txt, which `helm template` does not render or expose via
			// --show-only — same limitation as the Connectors TLS suite. This case only
			// asserts the template still renders with the gateway + TLS combination.
			Name: "Gateway HTTPRoute + Optimize TLS combination renders without crash",
			Values: map[string]string{
				"optimize.enabled":                               "true",
				"global.gateway.enabled":                         "true",
				"global.host":                                    "camunda.example.com",
				"global.tls.optimize.enabled":                    "true",
				"global.tls.optimize.cert.secret.existingSecret": "optimize-ks",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
			},
		},
		{
			Name: "Constraint fails on unsupported type",
			Values: map[string]string{
				"optimize.enabled":                               "true",
				"global.tls.optimize.enabled":                    "true",
				"global.tls.optimize.cert.secret.existingSecret": "optimize-ks",
				"global.tls.optimize.type":                       "jks",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "global.tls.optimize.type=\"jks\" is not supported")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *OptimizeTLSTest) TestTLSAutoRollout() {
	testCases := []testhelpers.TestCase{
		{
			Name: "autoRollout emits a stable checksum without cluster access",
			Values: map[string]string{
				"optimize.enabled":                               "true",
				"global.tls.optimize.enabled":                    "true",
				"global.tls.optimize.autoRollout":                "true",
				"global.tls.optimize.cert.secret.existingSecret": "optimize-ks",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(t, output, &deployment)
				require.NotEmpty(t, deployment.Spec.Template.Annotations["checksum/optimize-tls"])
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

// TestTLSDetectionFromConfigSources covers the config sources Optimize server
// TLS state is resolved from beyond optimize.env, plus the form the chart cannot
// read at render time.
// TestServerSslIsRejected pins the render-time rejection of server.ssl config.
// Optimize builds its TLS connector from container.keystore.* and never reads
// server.ssl.*, so accepting these keys would yield a pod that installs
// cleanly and then crash-loops on a duplicate SSLHostConfig.
func (s *OptimizeTLSTest) TestServerSslIsRejected() {
	const envMsg = "which the Optimize server does not support"
	const yamlMsg = "declares server.ssl, which the Optimize server does not support"

	testCases := []testhelpers.TestCase{
		{
			Name: "optimize.env SERVER_SSL_ENABLED is rejected",
			Values: map[string]string{
				"optimize.enabled":     "true",
				"optimize.env[0].name": "SERVER_SSL_ENABLED",
			},
			RenderTemplateExtraArgs: []string{"--set-string", "optimize.env[0].value=true"},
			Verifier: func(t *testing.T, output string, err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "SERVER_SSL_ENABLED")
				require.Contains(t, err.Error(), envMsg)
				// The message must name the knob that actually works.
				require.Contains(t, err.Error(), "CAMUNDA_OPTIMIZE_CONTAINER_KEYSTORE_LOCATION")
			},
		},
		{
			Name: "optimize.env SERVER_SSL_KEY_STORE is rejected",
			Values: map[string]string{
				"optimize.enabled":      "true",
				"optimize.env[0].name":  "SERVER_SSL_KEY_STORE",
				"optimize.env[0].value": "/certs/keystore.p12",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "SERVER_SSL_KEY_STORE")
				require.Contains(t, err.Error(), envMsg)
			},
		},
		{
			Name:        "nested server.ssl in optimize.configuration is rejected",
			ValuesFiles: []string{"testdata/values-optimize-tls-configuration.yaml"},
			Verifier: func(t *testing.T, output string, err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), yamlMsg)
			},
		},
		{
			Name:        "nested server.ssl in optimize.extraConfiguration is rejected",
			ValuesFiles: []string{"testdata/values-optimize-tls-extra-configuration.yaml"},
			Verifier: func(t *testing.T, output string, err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), yamlMsg)
			},
		},
		{
			// The nested-key walk cannot see dotted keys, so this form is the
			// one that would silently render plaintext if the check missed it.
			Name:        "dotted server.ssl keys are rejected too",
			ValuesFiles: []string{"testdata/values-optimize-tls-dotted-key.yaml"},
			Verifier: func(t *testing.T, output string, err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), yamlMsg)
			},
		},
		{
			// A placeholder or an activation condition means the value is a
			// runtime decision, so the old code derived plaintext and warned.
			// Rejection covers it without needing that guesswork.
			Name:        "server.ssl behind a property placeholder is rejected",
			ValuesFiles: []string{"testdata/values-optimize-tls-placeholder.yaml"},
			Verifier: func(t *testing.T, output string, err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), yamlMsg)
			},
		},
		{
			Name:        "unrelated multi-document configuration is not rejected",
			ValuesFiles: []string{"testdata/values-optimize-tls-multi-document-plain.yaml"},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
			},
		},
		{
			// Other components ARE stock Spring Boot; the check must not leak.
			Name: "SERVER_SSL_ENABLED on connectors.env is untouched",
			Values: map[string]string{
				"optimize.enabled":                                 "true",
				"connectors.enabled":                               "true",
				"global.tls.connectors.enabled":                    "true",
				"global.tls.connectors.cert.secret.existingSecret": "connectors-ks",
				"connectors.env[0].name":                           "SERVER_SSL_ENABLED",
			},
			RenderTemplateExtraArgs: []string{"--set-string", "connectors.env[0].value=true"},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

// TestTLSDerivedSurfaces pins the two values derived from Optimize TLS state --
// probe scheme and the /optimize Ingress backend protocol -- now that
// global.tls.optimize.enabled is their only source.
func (s *OptimizeTLSTest) TestTLSDerivedSurfaces() {
	requireProbeScheme := func(t *testing.T, output string, scheme corev1.URIScheme) {
		var deployment appsv1.Deployment
		helm.UnmarshalK8SYaml(t, output, &deployment)

		container := s.mainContainer(&deployment)
		require.NotNil(t, container.ReadinessProbe, "readiness probe must be set")
		require.NotNil(t, container.ReadinessProbe.HTTPGet, "readiness probe must be an httpGet")
		require.Equal(t, scheme, container.ReadinessProbe.HTTPGet.Scheme)
	}

	testCases := []testhelpers.TestCase{
		{
			Name: "probes use HTTPS when the global flag is on",
			Values: map[string]string{
				"optimize.enabled":                               "true",
				"global.tls.optimize.enabled":                    "true",
				"global.tls.optimize.cert.secret.existingSecret": "optimize-ks",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				requireProbeScheme(t, output, corev1.URISchemeHTTPS)
			},
		},
		{
			Name: "probes stay HTTP when the global flag is off",
			Values: map[string]string{
				"optimize.enabled": "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				requireProbeScheme(t, output, corev1.URISchemeHTTP)
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

// TestTLSIngressBackend pins the split-out Optimize Ingress. When TLS is on,
// Optimize drops out of the combined Ingress path list and gets its own
// Ingress annotated backend-protocol: HTTPS instead.
func (s *OptimizeTLSTest) TestTLSIngressBackend() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TLS Optimize gets its own Ingress with an HTTPS backend",
			Values: map[string]string{
				"optimize.enabled":                               "true",
				"global.ingress.enabled":                         "true",
				"optimize.contextPath":                           "/optimize",
				"global.tls.optimize.enabled":                    "true",
				"global.tls.optimize.cert.secret.existingSecret": "optimize-ks",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.Contains(t, output, "kind: Ingress")
				require.Contains(t, output, "nginx.ingress.kubernetes.io/backend-protocol: HTTPS")
				require.Contains(t, output, "-optimize-http")
			},
		},
		{
			Name: "plaintext Optimize stays on the combined Ingress",
			Values: map[string]string{
				"optimize.enabled":       "true",
				"global.ingress.enabled": "true",
				"optimize.contextPath":   "/optimize",
			},
			Verifier: func(t *testing.T, output string, err error) {
				// The split-out template is gated off, so it renders nothing.
				require.Error(t, err)
				require.Contains(t, err.Error(), "could not find template")
			},
		},
	}

	testhelpers.RunTestCasesE(
		s.T(), s.chartPath, s.release, s.namespace,
		[]string{"templates/common/ingress-optimize-http.yaml"}, testCases,
	)
}
