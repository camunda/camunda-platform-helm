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
)

type ConstraintsTemplateTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
}

func TestConstraintsTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../../")
	require.NoError(t, err)

	suite.Run(t, &ConstraintsTemplateTest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
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

	output, err := helm.RenderTemplateE(s.T(), options, s.chartPath, s.release, []string{"templates/camunda/constraints.tpl"})

	s.Require().Error(err, "Should fail if identityKeycloak is enabled but identity is disabled")
	s.Require().Contains(output, "[camunda][error] Identity is disabled but identityKeycloak is enabled")
}

func (s *ConstraintsTemplateTest) TestIdentityKeycloakConstraintSuccess() {
	options := &helm.Options{
		SetValues: map[string]string{
			"identity.enabled":         "true",
			"identityKeycloak.enabled": "false",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", s.namespace),
	}

	output, err := helm.RenderTemplateE(s.T(), options, s.chartPath, s.release, []string{"templates/camunda/constraints.tpl"})

	s.Require().NoError(err, "Should not fail if identity is enabled or identityKeycloak is disabled")
	s.Require().NotContains(output, "[camunda][error] Identity is disabled but identityKeycloak is enabled")
}
