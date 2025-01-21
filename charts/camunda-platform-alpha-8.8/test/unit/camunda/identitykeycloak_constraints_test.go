package camunda

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	appsv1 "k8s.io/api/apps/v1"
)

type ConstraintsTemplateTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
	templates []string
}

func TestConstraintsTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &ConstraintsTemplateTest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
		templates: []string{},
	})
}

func (s *ConstraintsTemplateTest) TestIdentityKeycloakConstraintFailure() {
	options := &helm.Options{
		SetValues: map[string]string{
			"identity.enabled":         "false",
			"identityKeycloak.enabled": "true",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	output, err := helm.RenderTemplateE(s.T(), options, s.chartPath, s.release, s.templates)
	var deployment appsv1.Deployment
	helm.UnmarshalK8SYaml(s.T(), output, &deployment)

	s.Require().Error(err, "[camunda][error] Identity is disabled but identityKeycloak is enabled")

}

func (s *ConstraintsTemplateTest) TestIdentityKeycloakConstraintSuccess() {
	options := &helm.Options{
		SetValues: map[string]string{
			"identity.enabled":                      "true",
			"identityKeycloak.enabled":              "false",
			"global.identity.keycloak.url.protocol": "https",
			"global.identity.keycloak.url.host":     "keycloak.prod.svc.cluster.local",
			"global.identity.keycloak.url.port":     "8443",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	output, err := helm.RenderTemplateE(s.T(), options, s.chartPath, s.release, s.templates)
	var deployment appsv1.Deployment
	helm.UnmarshalK8SYaml(s.T(), output, &deployment)

	s.Require().NoError(err, "[camunda][error] Identity is disabled but identityKeycloak is enabled")

}
