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
				"connectors.enabled":                               "true",
				"global.tls.connectors.enabled":                    "true",
				"global.tls.connectors.cert.secret.existingSecret": "connectors-ks",
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
				"connectors.enabled":                                              "true",
				"global.tls.connectors.enabled":                                   "true",
				"global.tls.connectors.cert.secret.existingSecret":                "connectors-ks",
				"global.tls.connectors.keystorePassword.secret.existingSecretKey": "ks-pw",
				"global.tls.connectors.keyAlias":                                  "connectors-rest",
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
				"connectors.enabled":                               "true",
				"global.tls.connectors.enabled":                    "true",
				"global.tls.connectors.cert.secret.existingSecret": "cert-manager-tls",
				"global.tls.connectors.type":                       "pem",
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
				"connectors.enabled":                                        "true",
				"global.tls.connectors.enabled":                             "true",
				"global.tls.connectors.cert.secret.existingSecret":          "connectors-pem",
				"global.tls.connectors.type":                                "pem",
				"global.tls.connectors.cert.secret.existingSecretKey":       "server.crt",
				"global.tls.connectors.privateKey.secret.existingSecretKey": "server.key",
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
				"connectors.enabled":                               "true",
				"global.tls.connectors.enabled":                    "true",
				"global.tls.connectors.cert.secret.existingSecret": "connectors-ks",
				"connectors.env[0].name":                           "SERVER_SSL_ENABLED",
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
				"connectors.enabled": "true",
				"global.tls.connectors.cert.secret.existingSecret": "should-not-mount",
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
				"connectors.enabled":                               "true",
				"global.tls.connectors.enabled":                    "true",
				"global.tls.connectors.cert.secret.existingSecret": "connectors-ks",
				"connectors.startupProbe.enabled":                  "true",
				"connectors.livenessProbe.enabled":                 "true",
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
			Name: "Probe scheme explicit override wins over TLS auto-flip for the overridden probe only",
			Values: map[string]string{
				"connectors.enabled":                               "true",
				"global.tls.connectors.enabled":                    "true",
				"global.tls.connectors.cert.secret.existingSecret": "connectors-ks",
				"connectors.startupProbe.enabled":                  "true",
				"connectors.livenessProbe.enabled":                 "true",
				"connectors.readinessProbe.scheme":                 "HTTP",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
				var deployment appsv1.Deployment
				helm.UnmarshalK8SYaml(s.T(), output, &deployment)

				container := deployment.Spec.Template.Spec.Containers[0]
				s.Require().NotNil(container.StartupProbe)
				s.Require().Equal(corev1.URIScheme("HTTPS"), container.StartupProbe.HTTPGet.Scheme)
				s.Require().NotNil(container.ReadinessProbe)
				s.Require().Equal(corev1.URIScheme("HTTP"), container.ReadinessProbe.HTTPGet.Scheme)
				s.Require().NotNil(container.LivenessProbe)
				s.Require().Equal(corev1.URIScheme("HTTPS"), container.LivenessProbe.HTTPGet.Scheme)
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
			Name: "Constraint fails when TLS enabled only via connectors.env with existingSecret set but global.tls.connectors.enabled false",
			Values: map[string]string{
				"connectors.enabled":                               "true",
				"connectors.env[0].name":                           "SERVER_SSL_ENABLED",
				"global.tls.connectors.cert.secret.existingSecret": "connectors-ks",
			},
			RenderTemplateExtraArgs: []string{
				"--set-string", "connectors.env[0].value=true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "global.tls.connectors.enabled: true")
			},
		},
		{
			Name: "Constraint allows TLS enabled via connectors.env when existingSecret set and global.tls.connectors.enabled is also true",
			Values: map[string]string{
				"connectors.enabled":                               "true",
				"global.tls.connectors.enabled":                    "true",
				"connectors.env[0].name":                           "SERVER_SSL_ENABLED",
				"global.tls.connectors.cert.secret.existingSecret": "connectors-ks",
			},
			RenderTemplateExtraArgs: []string{
				"--set-string", "connectors.env[0].value=true",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
			},
		},
		{
			Name: "Probe scheme switches to HTTPS when TLS enabled only via connectors.env SERVER_SSL_ENABLED",
			Values: map[string]string{
				"connectors.enabled":               "true",
				"connectors.startupProbe.enabled":  "true",
				"connectors.livenessProbe.enabled": "true",
				"connectors.env[0].name":           "SERVER_SSL_ENABLED",
				"connectors.env[1].name":           "SERVER_SSL_KEY_STORE",
				"connectors.env[1].value":          "file:/custom/keystore.p12",
			},
			RenderTemplateExtraArgs: []string{
				"--set-string", "connectors.env[0].value=true",
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
			// The BackendTLSPolicy warning is emitted from camunda.constraints.warnings
			// via NOTES.txt, which `helm template` (the framework these tests use, see
			// renderTemplateE / helm.RenderTemplateE) does not render or expose via
			// --show-only — same limitation documented in
			// common/constraints_test.go's TestLegacyJksTruststoreFieldsRenderWithoutCrash.
			// Verified manually via `helm install --dry-run` that the warning text
			// ("[camunda][warning] Connectors TLS is enabled ... BackendTLSPolicy ...")
			// renders when connectors.enabled + global.gateway.enabled (with
			// global.gateway.external unset/false) + Connectors TLS are all set, and does
			// NOT render when global.gateway.enabled is false. This test only asserts the
			// template still renders successfully with this combination of values.
			Name: "Gateway HTTPRoute + Connectors TLS combination renders without crash",
			Values: map[string]string{
				"connectors.enabled":                               "true",
				"global.gateway.enabled":                           "true",
				"global.host":                                      "camunda.example.com",
				"global.tls.connectors.enabled":                    "true",
				"global.tls.connectors.cert.secret.existingSecret": "connectors-ks",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.NoError(t, err)
			},
		},
		{
			Name: "Constraint fails on unsupported type",
			Values: map[string]string{
				"connectors.enabled":                               "true",
				"global.tls.connectors.enabled":                    "true",
				"global.tls.connectors.cert.secret.existingSecret": "connectors-ks",
				"global.tls.connectors.type":                       "jks",
			},
			Verifier: func(t *testing.T, output string, err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "global.tls.connectors.type=\"jks\" is not supported")
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *ConnectorsTLSTest) TestTLSChecksumAnnotation() {
	testCases := []testhelpers.TestCase{
		{
			Name:     "TLS enabled + autoRollout stamps a checksum/connectors-tls pod annotation",
			Template: "templates/connectors/deployment.yaml",
			Values: map[string]string{
				"connectors.enabled":                               "true",
				"orchestration.data.secondaryStorage.type":         "elasticsearch",
				"global.tls.connectors.enabled":                    "true",
				"global.tls.connectors.autoRollout":                "true",
				"global.tls.connectors.cert.secret.existingSecret": "connectors-ks",
			},
			Expected: map[string]string{
				// lookup is empty under `helm template`, so the value is the stable
				// sha256 of an empty object — presence is what we assert here.
				"spec.template.metadata.annotations.checksum/connectors-tls": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			},
		},
		{
			Name:     "no checksum/connectors-tls annotation when TLS is set but autoRollout is off (default)",
			Template: "templates/connectors/deployment.yaml",
			Values: map[string]string{
				"connectors.enabled":                               "true",
				"orchestration.data.secondaryStorage.type":         "elasticsearch",
				"global.tls.connectors.enabled":                    "true",
				"global.tls.connectors.cert.secret.existingSecret": "connectors-ks",
			},
			Expected: map[string]string{
				"spec.template.metadata.annotations.checksum/connectors-tls": "",
			},
		},
		{
			Name:     "no checksum/connectors-tls annotation when TLS is disabled",
			Template: "templates/connectors/deployment.yaml",
			Values: map[string]string{
				"connectors.enabled":                       "true",
				"orchestration.data.secondaryStorage.type": "elasticsearch",
			},
			Expected: map[string]string{
				"spec.template.metadata.annotations.checksum/connectors-tls": "",
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}
