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

package optimize

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type AuthIdentityTemplateTest struct {
	suite.Suite
	chartPath string
	release   string
	namespace string
}

func TestAuthIdentityTemplate(t *testing.T) {
	t.Parallel()

	chartPath, err := filepath.Abs("../../../")
	require.NoError(t, err)

	suite.Run(t, &AuthIdentityTemplateTest{
		chartPath: chartPath,
		release:   "camunda-platform-test",
		namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
	})
}

func (s *AuthIdentityTemplateTest) render(extra map[string]string, templates []string) string {
	values := map[string]string{
		"global.identity.auth.enabled":             "true",
		"optimize.enabled":                         "true",
		"orchestration.data.secondaryStorage.type": "elasticsearch",
	}
	for k, v := range extra {
		values[k] = v
	}
	options := &helm.Options{SetValues: values}
	output, err := helm.RenderTemplateE(s.T(), options, s.chartPath, s.release, templates)
	s.Require().NoError(err)
	return output
}

// The component-level key must win over the global one so an Optimize-only release owns its
// identity; otherwise every Optimize in a topology shares global.identity.auth.optimize.
func (s *AuthIdentityTemplateTest) TestComponentAudienceOverridesGlobal() {
	out := s.render(map[string]string{
		"global.identity.auth.optimize.audience":         "global-api",
		"optimize.security.authentication.oidc.audience": "component-api",
	}, []string{"templates/optimize/configmap.yaml"})

	s.Require().Contains(out, "component-api")
	s.Require().NotContains(out, "global-api")
}

func (s *AuthIdentityTemplateTest) TestComponentRedirectUrlOverridesGlobal() {
	out := s.render(map[string]string{
		"global.identity.auth.optimize.redirectUrl":         "https://global.example.com",
		"optimize.security.authentication.oidc.redirectUrl": "https://component.example.com",
	}, []string{"templates/optimize/configmap.yaml"})

	s.Require().Contains(out, "https://component.example.com")
	s.Require().NotContains(out, "https://global.example.com")
}

// Falling back to the global key keeps every pre-existing values file working unchanged.
func (s *AuthIdentityTemplateTest) TestFallsBackToGlobalWhenComponentUnset() {
	out := s.render(map[string]string{
		"global.identity.auth.optimize.audience": "global-api",
	}, []string{"templates/optimize/configmap.yaml"})

	s.Require().Contains(out, "global-api")
}

// The release-shared identity ConfigMap is consumed by Connectors too, so a per-release override
// must not rewrite it. Optimize instead gains its own ConfigMap, listed after the shared one in
// envFrom, where the kubelet takes the last value for a duplicate key.
func (s *AuthIdentityTemplateTest) TestIdentityOverridesRenderAComponentScopedConfigMap() {
	out := s.render(map[string]string{
		"global.identity.service.url":                  "http://identity.hub.svc/identity",
		"global.identity.auth.issuer":                  "https://global-issuer.example.com",
		"optimize.security.authentication.oidc.issuer": "https://tenant-issuer.example.com",
		"optimize.identity.service.url":                "http://identity.tenant.svc/identity",
	}, []string{"templates/optimize/configmap-identity-env.yaml"})

	s.Require().Contains(out, "-optimize-identity-env-vars")
	s.Require().Contains(out, "https://tenant-issuer.example.com")
	s.Require().Contains(out, "http://identity.tenant.svc/identity")
	s.Require().NotContains(out, "https://global-issuer.example.com")
}

// Without an override the component ConfigMap must not exist, so existing releases render exactly
// as before and only the release-shared ConfigMap is used.
func (s *AuthIdentityTemplateTest) TestNoComponentConfigMapWithoutOverrides() {
	options := &helm.Options{SetValues: map[string]string{
		"global.identity.auth.enabled":             "true",
		"optimize.enabled":                         "true",
		"orchestration.data.secondaryStorage.type": "elasticsearch",
		"global.identity.service.url":              "http://identity.hub.svc/identity",
	}}
	_, err := helm.RenderTemplateE(s.T(), options, s.chartPath, s.release,
		[]string{"templates/optimize/configmap-identity-env.yaml"})
	s.Require().Error(err, "the component ConfigMap must render nothing without overrides")
}

// Every ConfigMap the Optimize Deployment lists in envFrom must actually render. The
// release-shared identity ConfigMap is produced by global.identity.auth.enabled, so a release
// that enables auth only on the component must not reference it: a configMapRef without
// optional: true blocks the pod from starting.
func (s *AuthIdentityTemplateTest) TestComponentAuthReferencesOnlyRenderedConfigMaps() {
	extra := map[string]string{
		"global.identity.auth.enabled":            "false",
		"optimize.security.authentication.method": "oidc",
	}
	deployment := s.render(extra, []string{"templates/optimize/deployment.yaml"})
	configMap := s.render(extra, []string{"templates/optimize/configmap-identity-env.yaml"})

	s.Require().NotContains(deployment, s.release+"-identity-env-vars")
	s.Require().Contains(deployment, s.release+"-optimize-identity-env-vars")
	s.Require().Contains(configMap, "kind: ConfigMap")
	s.Require().Contains(configMap, "CAMUNDA_IDENTITY_ISSUER")
}

// With auth enabled globally and no component overrides the shared ConfigMap remains the single
// source, so nothing changes for a release that does not use the component keys.
func (s *AuthIdentityTemplateTest) TestGlobalAuthKeepsTheSharedConfigMapAsSoleSource() {
	deployment := s.render(nil, []string{"templates/optimize/deployment.yaml"})

	s.Require().Contains(deployment, s.release+"-identity-env-vars")
	s.Require().NotContains(deployment, s.release+"-optimize-identity-env-vars")
}
