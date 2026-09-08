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

package camunda

import (
	"maps"
	"strconv"
	"testing"

	"camunda-platform/test/unit/testhelpers"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
)

type tlsSecretsTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
	templates []string
}

func (s *tlsSecretsTest) SetupTest() {
	s.chartPath = "../../../"
	s.release = "test-release"
	s.namespace = "test-namespace"
	s.templates = []string{"templates"}
}

// global.tls.caBundle tests

func (s *tlsSecretsTest) TestCaBundleOrchestration() {
	testCases := []testhelpers.TestCase{
		{
			Name:     "caBundle injects SSL_CERT_FILE + NODE_EXTRA_CA_CERTS env, volume, and mount into orchestration",
			Template: "templates/orchestration/statefulset.yaml",
			Values: map[string]string{
				"orchestration.enabled":                        "true",
				"global.tls.caBundle.secret.existingSecret":    "my-ca-bundle",
				"global.tls.caBundle.secret.existingSecretKey": "ca.crt",
			},
			Expected: map[string]string{
				"spec.template.spec.volumes[?(@.name=='ca-bundle')].secret.secretName":            "my-ca-bundle",
				"spec.template.spec.containers[0].volumeMounts[?(@.name=='ca-bundle')].mountPath": "/etc/camunda/tls",
				"spec.template.spec.containers[0].env[?(@.name=='SSL_CERT_FILE')].value":          "/etc/camunda/tls/ca.crt",
				"spec.template.spec.containers[0].env[?(@.name=='NODE_EXTRA_CA_CERTS')].value":    "/etc/camunda/tls/ca.crt",
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *tlsSecretsTest) TestCaBundleWebModelerWebsockets() {
	testCases := []testhelpers.TestCase{
		{
			Name:     "caBundle injects NODE_EXTRA_CA_CERTS into web-modeler websockets",
			Template: "templates/web-modeler/deployment-websockets.yaml",
			Values: map[string]string{
				"webModeler.enabled":                           "true",
				"webModeler.restapi.mail.fromAddress":          "test@example.com",
				"identity.enabled":                             "true",
				"global.tls.caBundle.secret.existingSecret":    "my-ca-bundle",
				"global.tls.caBundle.secret.existingSecretKey": "ca.crt",
			},
			Expected: map[string]string{
				"spec.template.spec.volumes[?(@.name=='ca-bundle')].secret.secretName":         "my-ca-bundle",
				"spec.template.spec.containers[0].env[?(@.name=='NODE_EXTRA_CA_CERTS')].value": "/etc/camunda/tls/ca.crt",
				"spec.template.spec.containers[0].env[?(@.name=='SSL_CERT_FILE')].value":       "/etc/camunda/tls/ca.crt",
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *tlsSecretsTest) TestCaBundleInitContainerUsesComponentImage() {
	testCases := []testhelpers.TestCase{
		{
			Name:     "init container reuses the component's own image (registry + tag), not a pinned JRE image",
			Template: "templates/orchestration/statefulset.yaml",
			Values: map[string]string{
				"orchestration.enabled":                     "true",
				"orchestration.image.tag":                   "t1",
				"global.image.registry":                     "reg.test",
				"global.tls.caBundle.secret.existingSecret": "my-ca-bundle",
			},
			Expected: map[string]string{
				// init container image must equal the main container image
				"spec.template.spec.initContainers[?(@.name=='ca-bundle-truststore-init')].image": "reg.test/camunda/camunda:t1",
				"spec.template.spec.containers[0].image":                                          "reg.test/camunda/camunda:t1",
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *tlsSecretsTest) TestCaBundleInitContainerImageOverrideVerbatim() {
	testCases := []testhelpers.TestCase{
		{
			Name:     "explicit caBundle.image override is used verbatim and NOT prefixed with global.image.registry",
			Template: "templates/orchestration/statefulset.yaml",
			Values: map[string]string{
				"orchestration.enabled":                     "true",
				"global.image.registry":                     "reg.test",
				"global.tls.caBundle.secret.existingSecret": "my-ca-bundle",
				"global.tls.caBundle.image":                 "custom.io/myjre:1",
			},
			Expected: map[string]string{
				// verbatim — must NOT become reg.test/custom.io/myjre:1
				"spec.template.spec.initContainers[?(@.name=='ca-bundle-truststore-init')].image": "custom.io/myjre:1",
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *tlsSecretsTest) TestCaBundleInitContainerSecurityContext() {
	testCases := []testhelpers.TestCase{
		{
			Name:     "init container pins runAsUser by default",
			Template: "templates/orchestration/statefulset.yaml",
			Values: map[string]string{
				"orchestration.enabled":                     "true",
				"global.tls.caBundle.secret.existingSecret": "my-ca-bundle",
			},
			Expected: map[string]string{
				"spec.template.spec.initContainers[?(@.name=='ca-bundle-truststore-init')].securityContext.runAsUser":    "1000",
				"spec.template.spec.initContainers[?(@.name=='ca-bundle-truststore-init')].securityContext.runAsNonRoot": "true",
			},
		},
		{
			Name:     "OpenShift adaptSecurityContext=force drops runAsUser from the init container",
			Template: "templates/orchestration/statefulset.yaml",
			Values: map[string]string{
				"orchestration.enabled":                               "true",
				"global.tls.caBundle.secret.existingSecret":           "my-ca-bundle",
				"global.compatibility.openshift.adaptSecurityContext": "force",
			},
			Expected: map[string]string{
				"spec.template.spec.initContainers[?(@.name=='ca-bundle-truststore-init')].name": "ca-bundle-truststore-init",
			},
			Unexpected: []string{
				"spec.template.spec.initContainers[?(@.name=='ca-bundle-truststore-init')].securityContext.runAsUser",
				"spec.template.spec.initContainers[?(@.name=='ca-bundle-truststore-init')].securityContext.runAsGroup",
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *tlsSecretsTest) TestCaBundleChecksumAnnotation() {
	testCases := []testhelpers.TestCase{
		{
			Name:     "caBundle + autoRollout stamps a checksum/ca-bundle pod annotation",
			Template: "templates/orchestration/statefulset.yaml",
			Values: map[string]string{
				"orchestration.enabled":                     "true",
				"global.tls.caBundle.secret.existingSecret": "my-ca-bundle",
				"global.tls.caBundle.autoRollout":           "true",
			},
			Expected: map[string]string{
				// lookup is empty under `helm template`, so the value is the stable
				// sha256 of an empty object — presence is what we assert here.
				"spec.template.metadata.annotations.checksum/ca-bundle": "12ae32cb1ec02d01eda3581b127c1fee3b0dc53572ed6baf239721a03d82e126",
			},
		},
		{
			Name:     "no checksum/ca-bundle annotation when caBundle is set but autoRollout is off (default)",
			Template: "templates/orchestration/statefulset.yaml",
			Values: map[string]string{
				"orchestration.enabled":                     "true",
				"global.tls.caBundle.secret.existingSecret": "my-ca-bundle",
			},
			Expected: map[string]string{
				"kind": "StatefulSet",
			},
			Unexpected: []string{"spec.template.metadata.annotations.checksum/ca-bundle"},
		},
		{
			Name:     "no checksum/ca-bundle annotation when caBundle is unset",
			Template: "templates/orchestration/statefulset.yaml",
			Values: map[string]string{
				"orchestration.enabled": "true",
			},
			Expected: map[string]string{
				"kind": "StatefulSet",
			},
			Unexpected: []string{"spec.template.metadata.annotations.checksum/ca-bundle"},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *tlsSecretsTest) TestCaBundleChecksumAnnotationWebModeler() {
	const sentinel = "12ae32cb1ec02d01eda3581b127c1fee3b0dc53572ed6baf239721a03d82e126"
	testCases := []testhelpers.TestCase{
		{
			Name:     "web-modeler restapi gets checksum/ca-bundle even with no user podAnnotations (restructured block)",
			Template: "templates/web-modeler/deployment-restapi.yaml",
			Values: map[string]string{
				"webModeler.enabled":                        "true",
				"webModeler.restapi.mail.fromAddress":       "test@example.com",
				"identity.enabled":                          "true",
				"global.tls.caBundle.secret.existingSecret": "my-ca-bundle",
				"global.tls.caBundle.autoRollout":           "true",
			},
			Expected: map[string]string{
				"spec.template.metadata.annotations.checksum/ca-bundle": sentinel,
			},
		},
		{
			Name:     "web-modeler restapi keeps caBundle checksum alongside user podAnnotations",
			Template: "templates/web-modeler/deployment-restapi.yaml",
			Values: map[string]string{
				"webModeler.enabled":                        "true",
				"webModeler.restapi.mail.fromAddress":       "test@example.com",
				"identity.enabled":                          "true",
				"global.tls.caBundle.secret.existingSecret": "my-ca-bundle",
				"global.tls.caBundle.autoRollout":           "true",
				"webModeler.restapi.podAnnotations.my-anno": "v1",
			},
			Expected: map[string]string{
				"spec.template.metadata.annotations.checksum/ca-bundle": sentinel,
				"spec.template.metadata.annotations.my-anno":            "v1",
			},
		},
		{
			Name:     "web-modeler restapi has no checksum annotation when caBundle is set but autoRollout is off (no empty annotations block)",
			Template: "templates/web-modeler/deployment-restapi.yaml",
			Values: map[string]string{
				"webModeler.enabled":                        "true",
				"webModeler.restapi.mail.fromAddress":       "test@example.com",
				"identity.enabled":                          "true",
				"global.tls.caBundle.secret.existingSecret": "my-ca-bundle",
			},
			Expected: map[string]string{
				"kind": "Deployment",
			},
			Unexpected: []string{"spec.template.metadata.annotations.checksum/ca-bundle"},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *tlsSecretsTest) TestCaBundleConsole() {
	testCases := []testhelpers.TestCase{
		{
			Name:     "caBundle injects SSL_CERT_FILE + NODE_EXTRA_CA_CERTS into Console (Node.js trusts custom CA via NODE_EXTRA_CA_CERTS)",
			Template: "templates/console/deployment.yaml",
			Values: map[string]string{
				"console.enabled":                           "true",
				"identity.enabled":                          "true",
				"global.tls.caBundle.secret.existingSecret": "my-ca-bundle",
			},
			Expected: map[string]string{
				"spec.template.spec.volumes[?(@.name=='ca-bundle')].secret.secretName":         "my-ca-bundle",
				"spec.template.spec.containers[0].env[?(@.name=='SSL_CERT_FILE')].value":       "/etc/camunda/tls/ca.crt",
				"spec.template.spec.containers[0].env[?(@.name=='NODE_EXTRA_CA_CERTS')].value": "/etc/camunda/tls/ca.crt",
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *tlsSecretsTest) TestCaBundleConnectors() {
	testCases := []testhelpers.TestCase{
		{
			Name:     "caBundle wires connectors: volume, SSL_CERT_FILE, NODE_EXTRA_CA_CERTS, truststore JAVA_TOOL_OPTIONS, and init container",
			Template: "templates/connectors/deployment.yaml",
			Values: map[string]string{
				"connectors.enabled":                        "true",
				"global.tls.caBundle.secret.existingSecret": "my-ca-bundle",
			},
			Expected: map[string]string{
				"spec.template.spec.volumes[?(@.name=='ca-bundle')].secret.secretName":                                                                "my-ca-bundle",
				"spec.template.spec.containers[0].volumeMounts[?(@.name=='ca-bundle')].mountPath":                                                     "/etc/camunda/tls",
				"spec.template.spec.containers[0].env[?(@.name=='SSL_CERT_FILE')].value":                                                              "/etc/camunda/tls/ca.crt",
				"spec.template.spec.containers[0].env[?(@.name=='NODE_EXTRA_CA_CERTS')].value":                                                        "/etc/camunda/tls/ca.crt",
				"spec.template.spec.containers[0].env[?(@.name=='JAVA_TOOL_OPTIONS')].value":                                                          "-Djavax.net.ssl.trustStore=/var/camunda/tls-truststore/cacerts -Djavax.net.ssl.trustStorePassword=changeit",
				"spec.template.spec.initContainers[?(@.name=='ca-bundle-truststore-init')].volumeMounts[?(@.name=='ca-bundle-truststore')].mountPath": "/var/camunda/tls-truststore",
			},
		},
	}
	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *tlsSecretsTest) TestCaBundleIdentity() {
	testCases := []testhelpers.TestCase{
		{
			Name:     "caBundle wires identity: volume, SSL_CERT_FILE, truststore JAVA_TOOL_OPTIONS, and init container",
			Template: "templates/identity/deployment.yaml",
			Values: map[string]string{
				"identity.enabled":                          "true",
				"global.security.authentication.method":     "oidc",
				"global.tls.caBundle.secret.existingSecret": "my-ca-bundle",
			},
			Expected: map[string]string{
				"spec.template.spec.volumes[?(@.name=='ca-bundle')].secret.secretName":                                                                "my-ca-bundle",
				"spec.template.spec.containers[0].env[?(@.name=='SSL_CERT_FILE')].value":                                                              "/etc/camunda/tls/ca.crt",
				"spec.template.spec.containers[0].env[?(@.name=='JAVA_TOOL_OPTIONS')].value":                                                          "-Djavax.net.ssl.trustStore=/var/camunda/tls-truststore/cacerts -Djavax.net.ssl.trustStorePassword=changeit",
				"spec.template.spec.initContainers[?(@.name=='ca-bundle-truststore-init')].volumeMounts[?(@.name=='ca-bundle-truststore')].mountPath": "/var/camunda/tls-truststore",
			},
		},
	}
	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *tlsSecretsTest) TestCaBundleOptimize() {
	testCases := []testhelpers.TestCase{
		{
			Name:     "caBundle wires optimize: volume, SSL_CERT_FILE, truststore mount, and init container",
			Template: "templates/optimize/deployment.yaml",
			Values: map[string]string{
				"optimize.enabled":                          "true",
				"global.tls.caBundle.secret.existingSecret": "my-ca-bundle",
			},
			Expected: map[string]string{
				"spec.template.spec.volumes[?(@.name=='ca-bundle')].secret.secretName":                       "my-ca-bundle",
				"spec.template.spec.containers[0].env[?(@.name=='SSL_CERT_FILE')].value":                     "/etc/camunda/tls/ca.crt",
				"spec.template.spec.containers[0].volumeMounts[?(@.name=='ca-bundle-truststore')].mountPath": "/var/camunda/tls-truststore",
			},
		},
	}
	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *tlsSecretsTest) TestCaBundleImporter() {
	testCases := []testhelpers.TestCase{
		{
			Name:     "caBundle wires the migration importer deployment (JVM)",
			Template: "templates/orchestration/importer-deployment.yaml",
			Values: map[string]string{
				"orchestration.migration.data.enabled":      "true",
				"global.tls.caBundle.secret.existingSecret": "my-ca-bundle",
			},
			Expected: map[string]string{
				"spec.template.spec.volumes[?(@.name=='ca-bundle')].secret.secretName":                                                                "my-ca-bundle",
				"spec.template.spec.containers[0].env[?(@.name=='SSL_CERT_FILE')].value":                                                              "/etc/camunda/tls/ca.crt",
				"spec.template.spec.initContainers[?(@.name=='ca-bundle-truststore-init')].volumeMounts[?(@.name=='ca-bundle-truststore')].mountPath": "/var/camunda/tls-truststore",
			},
		},
	}
	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *tlsSecretsTest) TestCaBundleMigrationDataJob() {
	testCases := []testhelpers.TestCase{
		{
			Name:     "caBundle wires the data-migration job (JVM): init container + SSL_CERT_FILE + truststore JAVA_TOOL_OPTIONS",
			Template: "templates/orchestration/migration-data-job.yaml",
			Values: map[string]string{
				"orchestration.migration.data.enabled":      "true",
				"global.tls.caBundle.secret.existingSecret": "my-ca-bundle",
			},
			Expected: map[string]string{
				"spec.template.spec.volumes[?(@.name=='ca-bundle')].secret.secretName":                                                                "my-ca-bundle",
				"spec.template.spec.containers[0].env[?(@.name=='SSL_CERT_FILE')].value":                                                              "/etc/camunda/tls/ca.crt",
				"spec.template.spec.initContainers[?(@.name=='ca-bundle-truststore-init')].volumeMounts[?(@.name=='ca-bundle-truststore')].mountPath": "/var/camunda/tls-truststore",
			},
		},
	}
	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *tlsSecretsTest) TestCaBundleMigrationIdentityJob() {
	testCases := []testhelpers.TestCase{
		{
			Name:     "caBundle wires the identity-migration job (JVM): init container + SSL_CERT_FILE + truststore JAVA_TOOL_OPTIONS",
			Template: "templates/orchestration/migration-identity-job.yaml",
			Values: map[string]string{
				"orchestration.migration.identity.enabled":  "true",
				"global.tls.caBundle.secret.existingSecret": "my-ca-bundle",
			},
			Expected: map[string]string{
				"spec.template.spec.volumes[?(@.name=='ca-bundle')].secret.secretName":                                                                "my-ca-bundle",
				"spec.template.spec.containers[0].env[?(@.name=='SSL_CERT_FILE')].value":                                                              "/etc/camunda/tls/ca.crt",
				"spec.template.spec.containers[0].env[?(@.name=='JAVA_TOOL_OPTIONS')].value":                                                          "-Djavax.net.ssl.trustStore=/var/camunda/tls-truststore/cacerts -Djavax.net.ssl.trustStorePassword=changeit",
				"spec.template.spec.initContainers[?(@.name=='ca-bundle-truststore-init')].volumeMounts[?(@.name=='ca-bundle-truststore')].mountPath": "/var/camunda/tls-truststore",
			},
		},
	}
	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *tlsSecretsTest) TestCaBundleWebModelerWebapp() {
	testCases := []testhelpers.TestCase{
		{
			Name:     "caBundle wires web-modeler webapp (Node): SSL_CERT_FILE + NODE_EXTRA_CA_CERTS + bundle volume, no truststore",
			Template: "templates/web-modeler/deployment-webapp.yaml",
			Values: map[string]string{
				"webModeler.enabled":                        "true",
				"webModeler.restapi.mail.fromAddress":       "test@example.com",
				"identity.enabled":                          "true",
				"global.tls.caBundle.secret.existingSecret": "my-ca-bundle",
			},
			Expected: map[string]string{
				"spec.template.spec.volumes[?(@.name=='ca-bundle')].secret.secretName":         "my-ca-bundle",
				"spec.template.spec.containers[0].env[?(@.name=='SSL_CERT_FILE')].value":       "/etc/camunda/tls/ca.crt",
				"spec.template.spec.containers[0].env[?(@.name=='NODE_EXTRA_CA_CERTS')].value": "/etc/camunda/tls/ca.crt",
			},
		},
	}
	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func (s *tlsSecretsTest) TestInitContainersRendering() {
	workloads := []struct {
		name          string
		template      string
		initKey       string
		containerName string
		values        map[string]string
	}{
		{
			name: "connectors", template: "templates/connectors/deployment.yaml",
			initKey: "connectors.initContainers", containerName: "connectors",
			values: map[string]string{"connectors.enabled": "true"},
		},
		{
			name: "identity", template: "templates/identity/deployment.yaml",
			initKey: "identity.initContainers", containerName: "camunda-platform",
		},
		{
			name: "orchestration", template: "templates/orchestration/statefulset.yaml",
			initKey: "orchestration.initContainers", containerName: "orchestration",
			values: map[string]string{"orchestration.enabled": "true"},
		},
		{
			name: "orchestration-legacy", template: "templates/orchestration/statefulset.yaml",
			initKey: "orchestration.extraInitContainers", containerName: "orchestration",
			values: map[string]string{"orchestration.enabled": "true"},
		},
		{
			name: "importer", template: "templates/orchestration/importer-deployment.yaml",
			initKey: "orchestration.initContainers", containerName: "orchestration-migration-importer",
			values: map[string]string{"orchestration.migration.data.enabled": "true"},
		},
		{
			name: "importer-legacy", template: "templates/orchestration/importer-deployment.yaml",
			initKey: "orchestration.extraInitContainers", containerName: "orchestration-migration-importer",
			values: map[string]string{"orchestration.migration.data.enabled": "true"},
		},
		{
			name: "optimize", template: "templates/optimize/deployment.yaml",
			initKey: "optimize.initContainers", containerName: "optimize",
			values: map[string]string{"optimize.enabled": "true"},
		},
		{
			name: "optimize-migration", template: "templates/optimize/deployment.yaml",
			initKey: "optimize.initContainers", containerName: "optimize",
			values: map[string]string{"optimize.enabled": "true", "optimize.migration.enabled": "true"},
		},
		{
			name: "restapi", template: "templates/web-modeler/deployment-restapi.yaml",
			initKey: "webModeler.restapi.initContainers", containerName: "web-modeler-restapi",
			values: map[string]string{
				"webModeler.enabled":                  "true",
				"webModeler.restapi.mail.fromAddress": "test@example.com",
			},
		},
	}

	var testCases []testhelpers.TestCase
	for _, workload := range workloads {
		for _, scenario := range []struct {
			name     string
			caBundle bool
			custom   bool
		}{
			{name: "empty"},
			{name: "ca-only", caBundle: true},
			{name: "custom-only", custom: true},
			{name: "ca-and-custom", caBundle: true, custom: true},
		} {
			values := map[string]string{
				"identity.enabled":                          "true",
				"global.noSecondaryStorage":                 "false",
				"global.tls.caBundle.secret.existingSecret": "",
				"optimize.migration.enabled":                "false",
			}
			maps.Copy(values, workload.values)
			var expectedNames []string
			if scenario.caBundle {
				values["global.tls.caBundle.secret.existingSecret"] = "test-ca-bundle"
				expectedNames = append(expectedNames, "ca-bundle-truststore-init")
			}
			customContainer := corev1.Container{
				Name: "custom-" + s.release, Image: "busybox:1.36",
				Command: []string{"sh", "-c", "echo ready"},
			}
			if scenario.custom {
				values[workload.initKey+"[0].name"] = "custom-{{ .Release.Name }}"
				values[workload.initKey+"[0].image"] = customContainer.Image
				for index, command := range customContainer.Command {
					values[workload.initKey+"[0].command["+strconv.Itoa(index)+"]"] = command
				}
				if workload.initKey == "orchestration.initContainers" {
					values["orchestration.extraInitContainers[0].name"] = "legacy-must-not-render"
					values["orchestration.extraInitContainers[0].image"] = "busybox:1.36"
				}
				expectedNames = append(expectedNames, customContainer.Name)
			}
			if values["optimize.migration.enabled"] == "true" {
				expectedNames = append(expectedNames, "migration")
			}
			testCase := testhelpers.TestCase{
				Name: workload.name + "/" + scenario.name, Template: workload.template, Values: values,
			}
			if len(expectedNames) == 0 {
				testCase.Expected = map[string]string{
					"spec.template.spec.containers[0].name": workload.containerName,
				}
				testCase.Unexpected = []string{"spec.template.spec.initContainers"}
			} else {
				testCase.Verifier = func(t *testing.T, output string, err error) {
					require.NoError(t, err)
					var resource struct {
						Spec struct {
							Template corev1.PodTemplateSpec
						}
					}
					helm.UnmarshalK8SYaml(t, output, &resource)
					podSpec := resource.Spec.Template.Spec
					require.NotEmpty(t, podSpec.Containers)
					require.Equal(t, workload.containerName, podSpec.Containers[0].Name)
					var actualNames []string
					for _, container := range podSpec.InitContainers {
						actualNames = append(actualNames, container.Name)
					}
					require.Equal(t, expectedNames, actualNames)
					if scenario.custom {
						require.Contains(t, podSpec.InitContainers, customContainer)
					}
				}
			}
			testCases = append(testCases, testCase)
		}
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func TestTLSSecretsTestSuite(t *testing.T) {
	suite.Run(t, new(tlsSecretsTest))
}
