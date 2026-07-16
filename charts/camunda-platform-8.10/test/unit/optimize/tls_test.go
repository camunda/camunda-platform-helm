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
				s.Require().Contains(container.Env, corev1.EnvVar{Name: "SERVER_SSL_ENABLED", Value: "true"})
				s.Require().Contains(container.Env, corev1.EnvVar{Name: "SERVER_SSL_KEY_STORE", Value: "file:/usr/local/camunda/certificates/optimize/keystore.p12"})
				s.Require().Contains(container.Env, corev1.EnvVar{Name: "SERVER_SSL_KEY_STORE_TYPE", Value: "PKCS12"})
				s.Require().Contains(container.Env, corev1.EnvVar{
					Name: "SERVER_SSL_KEY_STORE_PASSWORD",
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
			Name: "PKCS12 mode with keyAlias",
			Values: map[string]string{
				"optimize.enabled":                                              "true",
				"global.tls.optimize.enabled":                                   "true",
				"global.tls.optimize.cert.secret.existingSecret":                "optimize-ks",
				"global.tls.optimize.keystorePassword.secret.existingSecretKey": "ks-pw",
				"global.tls.optimize.keyAlias":                                  "optimize-rest",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				container := s.mainContainer(&deployment)
				s.Require().Contains(container.Env, corev1.EnvVar{Name: "SERVER_SSL_KEY_ALIAS", Value: "optimize-rest"})
				s.Require().Contains(container.Env, corev1.EnvVar{
					Name: "SERVER_SSL_KEY_STORE_PASSWORD",
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
			Name: "PEM mode auto-substitutes tls.crt when existingSecretKey left empty",
			Values: map[string]string{
				"optimize.enabled":                               "true",
				"global.tls.optimize.enabled":                    "true",
				"global.tls.optimize.cert.secret.existingSecret": "cert-manager-tls",
				"global.tls.optimize.type":                       "pem",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				container := s.mainContainer(&deployment)
				s.Require().Contains(container.Env, corev1.EnvVar{Name: "SERVER_SSL_CERTIFICATE", Value: "/usr/local/camunda/certificates/optimize/tls.crt"})
				s.Require().Contains(container.Env, corev1.EnvVar{Name: "SERVER_SSL_CERTIFICATE_PRIVATE_KEY", Value: "/usr/local/camunda/certificates/optimize/tls.key"})
				require.NotContains(t, output, "SERVER_SSL_KEY_STORE")
			},
		},
		{
			Name: "PEM mode with explicit overridden keys",
			Values: map[string]string{
				"optimize.enabled":                                        "true",
				"global.tls.optimize.enabled":                             "true",
				"global.tls.optimize.cert.secret.existingSecret":          "optimize-pem",
				"global.tls.optimize.type":                                "pem",
				"global.tls.optimize.cert.secret.existingSecretKey":       "server.crt",
				"global.tls.optimize.privateKey.secret.existingSecretKey": "server.key",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				container := s.mainContainer(&deployment)
				s.Require().Contains(container.Env, corev1.EnvVar{Name: "SERVER_SSL_CERTIFICATE", Value: "/usr/local/camunda/certificates/optimize/server.crt"})
				s.Require().Contains(container.Env, corev1.EnvVar{Name: "SERVER_SSL_CERTIFICATE_PRIVATE_KEY", Value: "/usr/local/camunda/certificates/optimize/server.key"})
			},
		},
		{
			Name: "PEM mode with private key in a different secret",
			Values: map[string]string{
				"optimize.enabled":                                        "true",
				"global.tls.optimize.enabled":                             "true",
				"global.tls.optimize.cert.secret.existingSecret":          "optimize-cert",
				"global.tls.optimize.type":                                "pem",
				"global.tls.optimize.cert.secret.existingSecretKey":       "server.crt",
				"global.tls.optimize.privateKey.secret.existingSecret":    "optimize-key",
				"global.tls.optimize.privateKey.secret.existingSecretKey": "server.key",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				container := s.mainContainer(&deployment)
				s.Require().Contains(container.Env, corev1.EnvVar{Name: "SERVER_SSL_CERTIFICATE", Value: "/usr/local/camunda/certificates/optimize/server.crt"})
				s.Require().Contains(container.Env, corev1.EnvVar{Name: "SERVER_SSL_CERTIFICATE_PRIVATE_KEY", Value: "/usr/local/camunda/certificates/optimize/server.key"})

				var volume *corev1.Volume
				for i := range deployment.Spec.Template.Spec.Volumes {
					if deployment.Spec.Template.Spec.Volumes[i].Name == "optimize-server-tls" {
						volume = &deployment.Spec.Template.Spec.Volumes[i]
					}
				}
				s.Require().NotNil(volume, "expected optimize-server-tls volume")
				s.Require().Nil(volume.Secret, "split PEM secrets should use a projected volume")
				s.Require().NotNil(volume.Projected, "expected projected optimize-server-tls volume")
				s.Require().NotNil(volume.Projected.DefaultMode)
				s.Require().Equal(int32(0440), *volume.Projected.DefaultMode)
				s.Require().Len(volume.Projected.Sources, 2)
				s.Require().Equal("optimize-cert", volume.Projected.Sources[0].Secret.Name)
				s.Require().Equal("server.crt", volume.Projected.Sources[0].Secret.Items[0].Key)
				s.Require().Equal("server.crt", volume.Projected.Sources[0].Secret.Items[0].Path)
				s.Require().Equal("optimize-key", volume.Projected.Sources[1].Secret.Name)
				s.Require().Equal("server.key", volume.Projected.Sources[1].Secret.Items[0].Key)
				s.Require().Equal("server.key", volume.Projected.Sources[1].Secret.Items[0].Path)
			},
		},
		{
			Name: "Override precedence: explicit optimize.env wins last",
			Values: map[string]string{
				"optimize.enabled":                               "true",
				"global.tls.optimize.enabled":                    "true",
				"global.tls.optimize.cert.secret.existingSecret": "optimize-ks",
				"optimize.env[0].name":                           "SERVER_SSL_ENABLED",
			},
			RenderTemplateExtraArgs: []string{
				"--set-string", "optimize.env[0].value=false",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				container := s.mainContainer(&deployment)
				var positions []int
				for i, e := range container.Env {
					if e.Name == "SERVER_SSL_ENABLED" {
						positions = append(positions, i)
					}
				}
				s.Require().Len(positions, 2, "both entries should be rendered so the user-supplied one wins last")
				s.Require().Equal("true", container.Env[positions[0]].Value)
				s.Require().Equal("false", container.Env[positions[1]].Value)
			},
		},
		{
			Name: "Probe scheme honors optimize.env SERVER_SSL_ENABLED=false override even when global TLS is enabled",
			Values: map[string]string{
				"optimize.enabled":                               "true",
				"global.tls.optimize.enabled":                    "true",
				"global.tls.optimize.cert.secret.existingSecret": "optimize-ks",
				"optimize.startupProbe.enabled":                  "true",
				"optimize.livenessProbe.enabled":                 "true",
				"optimize.env[0].name":                           "SERVER_SSL_ENABLED",
			},
			RenderTemplateExtraArgs: []string{
				"--set-string", "optimize.env[0].value=false",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				container := s.mainContainer(&deployment)
				s.Require().NotNil(container.StartupProbe)
				s.Require().Equal(corev1.URIScheme("HTTP"), container.StartupProbe.HTTPGet.Scheme)
				s.Require().NotNil(container.ReadinessProbe)
				s.Require().Equal(corev1.URIScheme("HTTP"), container.ReadinessProbe.HTTPGet.Scheme)
				s.Require().NotNil(container.LivenessProbe)
				s.Require().Equal(corev1.URIScheme("HTTP"), container.LivenessProbe.HTTPGet.Scheme)
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
			Name: "Constraint allows TLS enabled when operator hand-wires SERVER_SSL_KEY_STORE via optimize.env",
			Values: map[string]string{
				"optimize.enabled":            "true",
				"global.tls.optimize.enabled": "true",
				"optimize.env[0].name":        "SERVER_SSL_KEY_STORE",
				"optimize.env[0].value":       "file:/custom/keystore.p12",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
			},
		},
		{
			Name: "Constraint fails when proxyVerify.enabled is true but TLS is off",
			Values: map[string]string{
				"optimize.enabled":                        "true",
				"global.tls.optimize.proxyVerify.enabled": "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "global.tls.optimize.proxyVerify.enabled is true but Optimize server TLS is not enabled")
			},
		},
		{
			Name: "Constraint fails when proxyVerify.enabled is true but caSecret is empty",
			Values: map[string]string{
				"optimize.enabled":                               "true",
				"global.tls.optimize.enabled":                    "true",
				"global.tls.optimize.cert.secret.existingSecret": "optimize-ks",
				"global.tls.optimize.proxyVerify.enabled":        "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "caSecret.secret.existingSecret is empty")
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
