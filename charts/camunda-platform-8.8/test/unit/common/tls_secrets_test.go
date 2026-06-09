package camunda

import (
	"testing"

	"camunda-platform/test/unit/testhelpers"

	"github.com/stretchr/testify/suite"
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
				// dropped → extractor returns "" for an absent path
				"spec.template.spec.initContainers[?(@.name=='ca-bundle-truststore-init')].securityContext.runAsUser":  "",
				"spec.template.spec.initContainers[?(@.name=='ca-bundle-truststore-init')].securityContext.runAsGroup": "",
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
				"spec.template.metadata.annotations.checksum/ca-bundle": "",
			},
		},
		{
			Name:     "no checksum/ca-bundle annotation when caBundle is unset",
			Template: "templates/orchestration/statefulset.yaml",
			Values: map[string]string{
				"orchestration.enabled": "true",
			},
			Expected: map[string]string{
				"spec.template.metadata.annotations.checksum/ca-bundle": "",
			},
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
				"spec.template.metadata.annotations.checksum/ca-bundle": "",
			},
		},
	}

	testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}

func TestTLSSecretsTestSuite(t *testing.T) {
	suite.Run(t, new(tlsSecretsTest))
}
