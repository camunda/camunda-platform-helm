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

package connectors

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

type ConnectorsTLSTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
	templates []string
}

func TestConnectorsTLS(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &ConnectorsTLSTest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
		templates: []string{"templates/connectors/deployment.yaml"},
	})
}

func (s *ConnectorsTLSTest) TestTLSEnvAndVolumeWiring() {
	testCases := []testhelpers.TestCase{
		{
			Name: "TLS disabled (default) — no SSL env, no volume, no annotation",
			Values: map[string]string{
				"connectors.enabled": "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.NotContains(t, output, "SERVER_SSL_ENABLED")
				require.NotContains(t, output, "connectors-tls")
				require.NotContains(t, output, "checksum/connectors-tls")
			},
		},
		{
			Name: "TLS enabled via global.tls.connectors.enabled (PKCS12 defaults)",
			Values: map[string]string{
				"connectors.enabled":                          "true",
				"global.tls.connectors.enabled":               "true",
				"global.tls.connectors.secret.existingSecret": "connectors-ks",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				env := deployment.Spec.Template.Spec.Containers[0].Env
				s.Require().Contains(env, corev1.EnvVar{Name: "SERVER_SSL_ENABLED", Value: "true"})
				s.Require().Contains(env, corev1.EnvVar{Name: "SERVER_SSL_KEY_STORE", Value: "file:/usr/local/camunda/certificates/connectors/keystore.p12"})
				s.Require().Contains(env, corev1.EnvVar{Name: "SERVER_SSL_KEY_STORE_TYPE", Value: "PKCS12"})
				s.Require().Contains(env, corev1.EnvVar{
					Name: "SERVER_SSL_KEY_STORE_PASSWORD",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "connectors-ks"},
							Key:                  "keystore-password",
						},
					},
				})

				mounts := deployment.Spec.Template.Spec.Containers[0].VolumeMounts
				var foundMount bool
				for _, m := range mounts {
					if m.Name == "connectors-tls" {
						foundMount = true
						s.Require().Equal("/usr/local/camunda/certificates/connectors", m.MountPath)
						s.Require().True(m.ReadOnly)
					}
				}
				s.Require().True(foundMount, "expected connectors-tls volumeMount")

				vols := deployment.Spec.Template.Spec.Volumes
				var foundVol bool
				for _, v := range vols {
					if v.Name == "connectors-tls" {
						foundVol = true
						s.Require().Equal("connectors-ks", v.Secret.SecretName)
					}
				}
				s.Require().True(foundVol, "expected connectors-tls volume")
			},
		},
		{
			Name: "PKCS12 mode with keyAlias",
			Values: map[string]string{
				"connectors.enabled":                                     "true",
				"global.tls.connectors.enabled":                          "true",
				"global.tls.connectors.secret.existingSecret":            "connectors-ks",
				"global.tls.connectors.secret.existingSecretPasswordKey": "ks-pw",
				"global.tls.connectors.secret.keyAlias":                  "connectors-rest",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				env := deployment.Spec.Template.Spec.Containers[0].Env
				s.Require().Contains(env, corev1.EnvVar{Name: "SERVER_SSL_KEY_ALIAS", Value: "connectors-rest"})
				s.Require().Contains(env, corev1.EnvVar{
					Name: "SERVER_SSL_KEY_STORE_PASSWORD",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "connectors-ks"},
							Key:                  "ks-pw",
						},
					},
				})
			},
		},
		{
			Name: "PEM mode auto-substitutes tls.crt when existingSecretKey left at PKCS12 default",
			Values: map[string]string{
				"connectors.enabled":                          "true",
				"global.tls.connectors.enabled":               "true",
				"global.tls.connectors.secret.existingSecret": "cert-manager-tls",
				"global.tls.connectors.secret.type":           "pem",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				env := deployment.Spec.Template.Spec.Containers[0].Env
				s.Require().Contains(env, corev1.EnvVar{Name: "SERVER_SSL_CERTIFICATE", Value: "/usr/local/camunda/certificates/connectors/tls.crt"})
				s.Require().Contains(env, corev1.EnvVar{Name: "SERVER_SSL_CERTIFICATE_PRIVATE_KEY", Value: "/usr/local/camunda/certificates/connectors/tls.key"})
				require.NotContains(t, output, "SERVER_SSL_KEY_STORE")
			},
		},
		{
			Name: "PEM mode with explicit overridden keys",
			Values: map[string]string{
				"connectors.enabled":                                       "true",
				"global.tls.connectors.enabled":                            "true",
				"global.tls.connectors.secret.existingSecret":              "connectors-pem",
				"global.tls.connectors.secret.type":                        "pem",
				"global.tls.connectors.secret.existingSecretKey":           "server.crt",
				"global.tls.connectors.secret.existingSecretPrivateKeyKey": "server.key",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				env := deployment.Spec.Template.Spec.Containers[0].Env
				s.Require().Contains(env, corev1.EnvVar{Name: "SERVER_SSL_CERTIFICATE", Value: "/usr/local/camunda/certificates/connectors/server.crt"})
				s.Require().Contains(env, corev1.EnvVar{Name: "SERVER_SSL_CERTIFICATE_PRIVATE_KEY", Value: "/usr/local/camunda/certificates/connectors/server.key"})
			},
		},
		{
			Name: "Override precedence: explicit connectors.env wins last",
			Values: map[string]string{
				"connectors.enabled":                          "true",
				"global.tls.connectors.enabled":               "true",
				"global.tls.connectors.secret.existingSecret": "connectors-ks",
				"connectors.env[0].name":                      "SERVER_SSL_ENABLED",
			},
			RenderTemplateExtraArgs: []string{
				"--set-string", "connectors.env[0].value=false",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				env := deployment.Spec.Template.Spec.Containers[0].Env
				var positions []int
				for i, e := range env {
					if e.Name == "SERVER_SSL_ENABLED" {
						positions = append(positions, i)
					}
				}
				s.Require().Len(positions, 2, "both entries should be rendered so the user-supplied one wins last")
				s.Require().Equal("true", env[positions[0]].Value)
				s.Require().Equal("false", env[positions[1]].Value)
			},
		},
		{
			Name: "Secret block is inert when global.tls.connectors.enabled is false",
			Values: map[string]string{
				"connectors.enabled":                          "true",
				"global.tls.connectors.secret.existingSecret": "should-not-mount",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				require.NotContains(t, output, "connectors-tls")
				require.NotContains(t, output, "SERVER_SSL_KEY_STORE")
			},
		},
		{
			Name: "Probe scheme switches to HTTPS when TLS enabled",
			Values: map[string]string{
				"connectors.enabled":                          "true",
				"global.tls.connectors.enabled":               "true",
				"global.tls.connectors.secret.existingSecret": "connectors-ks",
				"connectors.startupProbe.enabled":             "true",
				"connectors.livenessProbe.enabled":            "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				container := deployment.Spec.Template.Spec.Containers[0]
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
				"connectors.enabled": "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)
				container := deployment.Spec.Template.Spec.Containers[0]
				s.Require().NotNil(container.ReadinessProbe)
				s.Require().Equal(corev1.URIScheme("HTTP"), container.ReadinessProbe.HTTPGet.Scheme)
			},
		},
		{
			Name: "Constraint fails when TLS enabled but no cert configured",
			Values: map[string]string{
				"connectors.enabled":            "true",
				"global.tls.connectors.enabled": "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "Connectors TLS is enabled but no server cert is configured")
			},
		},
		{
			Name: "Constraint allows TLS enabled when operator hand-wires SERVER_SSL_KEY_STORE via connectors.env",
			Values: map[string]string{
				"connectors.enabled":            "true",
				"global.tls.connectors.enabled": "true",
				"connectors.env[0].name":        "SERVER_SSL_KEY_STORE",
				"connectors.env[0].value":       "file:/custom/keystore.p12",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
			},
		},
		{
			Name: "Constraint fails when proxyVerify.enabled is true but TLS is off",
			Values: map[string]string{
				"connectors.enabled":                        "true",
				"global.tls.connectors.proxyVerify.enabled": "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "global.tls.connectors.proxyVerify.enabled is true but Connectors TLS is not enabled")
			},
		},
		{
			Name: "Constraint fails when proxyVerify.enabled is true but caSecret is empty",
			Values: map[string]string{
				"connectors.enabled":                          "true",
				"global.tls.connectors.enabled":               "true",
				"global.tls.connectors.secret.existingSecret": "connectors-ks",
				"global.tls.connectors.proxyVerify.enabled":   "true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "caSecret.existingSecret is empty")
			},
		},
		{
			Name: "Constraint fails on unsupported secret.type",
			Values: map[string]string{
				"connectors.enabled":                          "true",
				"global.tls.connectors.enabled":               "true",
				"global.tls.connectors.secret.existingSecret": "connectors-ks",
				"global.tls.connectors.secret.type":           "jks",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "global.tls.connectors.secret.type=\"jks\" is not supported")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}
