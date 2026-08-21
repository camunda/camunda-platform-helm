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
		"global.identity.auth.enabled": "true",
		// This chart bundles no Keycloak, so global.identity.auth.enabled resolves no issuer on its
		// own and the issuer constraint rejects the render. Name a backend URL so every case below
		// exercises its own subject rather than that constraint.
		"global.identity.auth.issuerBackendUrl":    "http://keycloak.example.com/realms/camunda",
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
		"global.identity.auth.issuerBackendUrl":    "http://keycloak.example.com/realms/camunda",
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
		"global.identity.auth.enabled":                 "false",
		"optimize.security.authentication.method":      "oidc",
		"optimize.security.authentication.oidc.issuer": "https://tenant-issuer.example.com",
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

// Turning auth on for the component alone leaves no global.identity.auth.issuer to fall back on, so
// an unset release-scoped issuer renders an empty "iss" and an empty jwtSetUri that Optimize would
// then check every token against. The constraint is not gated on global.topology.mode=optimize
// because a combined release reaches the same state through the method override.
func (s *AuthIdentityTemplateTest) TestComponentAuthWithoutAnIssuerIsRejected() {
	options := &helm.Options{SetValues: map[string]string{
		"global.identity.auth.enabled":             "false",
		"optimize.enabled":                         "true",
		"orchestration.data.secondaryStorage.type": "elasticsearch",
		"optimize.security.authentication.method":  "oidc",
	}}
	_, err := helm.RenderTemplateE(s.T(), options, s.chartPath, s.release,
		[]string{"templates/optimize/configmap.yaml"})

	s.Require().Error(err)
	s.Require().Contains(err.Error(), "requires optimize.security.authentication.oidc.issuer")
}

// An External Keycloak leaves global.identity.auth.issuer empty and resolves the provider through
// the issuerBackendUrl derived from global.identity.keycloak.url. That has always rendered, so the
// issuer constraint must not reject it. Asserting the key alone would also pass on an empty value,
// which is the very state the constraint exists to prevent, so assert the resolved URL.
func (s *AuthIdentityTemplateTest) TestGlobalAuthWithoutAnIssuerStillRenders() {
	options := &helm.Options{SetValues: map[string]string{
		"global.identity.auth.enabled":             "true",
		"identity.enabled":                         "true",
		"optimize.enabled":                         "true",
		"orchestration.data.secondaryStorage.type": "elasticsearch",
		"global.identity.keycloak.url.protocol":    "http",
		"global.identity.keycloak.url.host":        "keycloak.example.com",
		"global.identity.keycloak.url.port":        "80",
	}}
	out := helm.RenderTemplate(s.T(), options, s.chartPath, s.release,
		[]string{"templates/optimize/configmap.yaml"})

	s.Require().Contains(out, `issuerBackendUrl: "http://keycloak.example.com:80/auth/realms/camunda-platform"`)
}

// This chart bundles no Keycloak, so global.identity.auth.enabled on its own resolves no issuer at
// all: it renders an empty issuer, an empty issuerBackendUrl and a relative jwtSetUri that Optimize
// would check every token against. Exempting the global path would let exactly the state the
// constraint exists to prevent through, so the exemption is keyed on a resolvable issuer backend
// URL instead of on which key switched authentication on.
func (s *AuthIdentityTemplateTest) TestGlobalAuthWithNoResolvableIssuerIsRejected() {
	options := &helm.Options{SetValues: map[string]string{
		"global.identity.auth.enabled":             "true",
		"identity.enabled":                         "true",
		"optimize.enabled":                         "true",
		"orchestration.data.secondaryStorage.type": "elasticsearch",
	}}
	_, err := helm.RenderTemplateE(s.T(), options, s.chartPath, s.release,
		[]string{"templates/optimize/configmap.yaml"})

	s.Require().Error(err)
	s.Require().Contains(err.Error(), "requires optimize.security.authentication.oidc.issuer")
}

// An explicit optimize.env entry for the issuer is a supported override: the Deployment renders
// optimize.env as container env, which the kubelet resolves over the identity env ConfigMap, so the
// container never reads the empty issuer the constraint guards against. Failing the render here
// would make the generic extension path unusable for the one variable it is most needed for.
func (s *AuthIdentityTemplateTest) TestEnvIssuerOverrideSatisfiesTheIssuerConstraint() {
	options := &helm.Options{SetValues: map[string]string{
		"global.identity.auth.enabled":             "true",
		"identity.enabled":                         "true",
		"optimize.enabled":                         "true",
		"orchestration.data.secondaryStorage.type": "elasticsearch",
		"optimize.env[0].name":                     "CAMUNDA_IDENTITY_ISSUER",
		"optimize.env[0].value":                    "https://keycloak.example.com/realms/camunda",
		"optimize.env[1].name":                     "CAMUNDA_IDENTITY_ISSUER_BACKEND_URL",
		"optimize.env[1].value":                    "http://keycloak.svc:8080/realms/camunda",
	}}
	out, err := helm.RenderTemplateE(s.T(), options, s.chartPath, s.release,
		[]string{"templates/optimize/deployment.yaml"})

	s.Require().NoError(err)
	s.Require().Contains(out, "value: https://keycloak.example.com/realms/camunda")
	s.Require().Contains(out, "value: http://keycloak.svc:8080/realms/camunda")
}

// Either variable alone is enough, and a valueFrom counts as much as a literal: both shapes reach
// the container the same way.
func (s *AuthIdentityTemplateTest) TestEnvIssuerOverrideFromSecretSatisfiesTheIssuerConstraint() {
	options := &helm.Options{SetValues: map[string]string{
		"global.identity.auth.enabled":                "true",
		"identity.enabled":                            "true",
		"optimize.enabled":                            "true",
		"orchestration.data.secondaryStorage.type":    "elasticsearch",
		"optimize.env[0].name":                        "CAMUNDA_IDENTITY_ISSUER_BACKEND_URL",
		"optimize.env[0].valueFrom.secretKeyRef.name": "issuer",
		"optimize.env[0].valueFrom.secretKeyRef.key":  "backend-url",
	}}
	_, err := helm.RenderTemplateE(s.T(), options, s.chartPath, s.release,
		[]string{"templates/optimize/deployment.yaml"})

	s.Require().NoError(err)
}

// A bare name with neither value nor valueFrom sets nothing, so it must not buy an exemption.
func (s *AuthIdentityTemplateTest) TestEnvIssuerOverrideWithoutAValueIsNotAnExemption() {
	options := &helm.Options{SetValues: map[string]string{
		"global.identity.auth.enabled":             "true",
		"identity.enabled":                         "true",
		"optimize.enabled":                         "true",
		"orchestration.data.secondaryStorage.type": "elasticsearch",
		"optimize.env[0].name":                     "CAMUNDA_IDENTITY_ISSUER",
		"optimize.env[0].value":                    "",
	}}
	_, err := helm.RenderTemplateE(s.T(), options, s.chartPath, s.release,
		[]string{"templates/optimize/deployment.yaml"})

	s.Require().Error(err)
	s.Require().Contains(err.Error(), "requires optimize.security.authentication.oidc.issuer")
}

// An unrelated optimize.env entry must not buy an exemption either.
func (s *AuthIdentityTemplateTest) TestUnrelatedEnvEntryIsNotAnIssuerExemption() {
	options := &helm.Options{SetValues: map[string]string{
		"global.identity.auth.enabled":             "true",
		"identity.enabled":                         "true",
		"optimize.enabled":                         "true",
		"orchestration.data.secondaryStorage.type": "elasticsearch",
		"optimize.env[0].name":                     "HTTP_PROXY",
		"optimize.env[0].value":                    "http://proxy.svc:3128",
	}}
	_, err := helm.RenderTemplateE(s.T(), options, s.chartPath, s.release,
		[]string{"templates/optimize/deployment.yaml"})

	s.Require().Error(err)
	s.Require().Contains(err.Error(), "requires optimize.security.authentication.oidc.issuer")
}

// optimize.authEnabled reads anything other than "oidc" as not-oidc, so an unconstrained typo would
// render an Optimize with authentication omitted and no error. The schema enum is what stops it.
func (s *AuthIdentityTemplateTest) TestUnknownAuthenticationMethodIsRejected() {
	options := &helm.Options{SetValues: map[string]string{
		"optimize.enabled":                         "true",
		"orchestration.data.secondaryStorage.type": "elasticsearch",
		"optimize.security.authentication.method":  "oidcc",
	}}
	_, err := helm.RenderTemplateE(s.T(), options, s.chartPath, s.release,
		[]string{"templates/optimize/configmap.yaml"})

	s.Require().Error(err)
	s.Require().Contains(err.Error(), "/optimize/security/authentication/method")
}

// A release-scoped issuerBackendUrl satisfies the constraint on its own: the release names the
// cluster-internal route even when no global key does.
func (s *AuthIdentityTemplateTest) TestComponentIssuerBackendUrlSatisfiesTheIssuerConstraint() {
	out := s.render(map[string]string{
		"optimize.security.authentication.oidc.issuerBackendUrl": "http://keycloak.tenant.svc:8080/realms/camunda",
	}, []string{"templates/optimize/configmap.yaml"})

	s.Require().Contains(out, `issuerBackendUrl: "http://keycloak.tenant.svc:8080/realms/camunda"`)
}

// method=none must win over global.identity.auth.enabled=true so an Optimize release can opt out of
// authentication without the whole release doing so; otherwise the global key is the only lever and
// it is shared with every other component.
func (s *AuthIdentityTemplateTest) TestComponentMethodNoneOverridesGlobalAuthEnabled() {
	out := s.render(map[string]string{
		"optimize.security.authentication.method": "none",
	}, []string{"templates/optimize/configmap.yaml"})

	s.Require().NotContains(out, "camunda:")
	s.Require().NotContains(out, "issuerBackendUrl:")
}

// With auth off for the component the Deployment must not reference the release-shared identity
// ConfigMap either, or Optimize keeps receiving identity env vars it no longer authenticates with.
func (s *AuthIdentityTemplateTest) TestComponentMethodNoneDropsTheIdentityEnvFrom() {
	out := s.render(map[string]string{
		"optimize.security.authentication.method": "none",
	}, []string{"templates/optimize/deployment.yaml"})

	s.Require().NotContains(out, s.release+"-identity-env-vars")
	s.Require().NotContains(out, s.release+"-optimize-identity-env-vars")
}

// A release-scoped existingSecret replaces the global source outright.
func (s *AuthIdentityTemplateTest) TestComponentSecretOverridesGlobalSecret() {
	out := s.render(map[string]string{
		"global.identity.auth.optimize.secret.existingSecret":            "global-oidc",
		"global.identity.auth.optimize.secret.existingSecretKey":         "global-key",
		"optimize.security.authentication.oidc.secret.existingSecret":    "tenant-oidc",
		"optimize.security.authentication.oidc.secret.existingSecretKey": "tenant-key",
	}, []string{"templates/optimize/deployment.yaml"})

	s.Require().Contains(out, "tenant-oidc")
	s.Require().Contains(out, "tenant-key")
	s.Require().NotContains(out, "global-oidc")
}

// existingSecretKey on its own renames the key inside the inherited secret. Treating any one field
// as a whole-block override instead would hand normalizeSecretConfiguration a name-less reference,
// which matches neither of its branches and drops CAMUNDA_IDENTITY_CLIENT_SECRET from the
// Deployment entirely - Optimize would then start with no client secret at all.
func (s *AuthIdentityTemplateTest) TestComponentSecretKeyAloneKeepsTheGlobalSecretName() {
	out := s.render(map[string]string{
		"global.identity.auth.optimize.secret.existingSecret":            "global-oidc",
		"global.identity.auth.optimize.secret.existingSecretKey":         "global-key",
		"optimize.security.authentication.oidc.secret.existingSecretKey": "tenant-key",
	}, []string{"templates/optimize/deployment.yaml"})

	s.Require().Contains(out, "CAMUNDA_IDENTITY_CLIENT_SECRET")
	s.Require().Contains(out, "global-oidc")
	s.Require().Contains(out, "tenant-key")
	s.Require().NotContains(out, "global-key")
}

// A release-scoped inlineSecret must win even when the global block names an existing Secret:
// normalizeSecretConfiguration checks existingSecret first, so an inherited one would otherwise
// keep beating the override.
func (s *AuthIdentityTemplateTest) TestComponentInlineSecretBeatsGlobalExistingSecret() {
	out := s.render(map[string]string{
		"global.identity.auth.optimize.secret.existingSecret":       "global-oidc",
		"global.identity.auth.optimize.secret.existingSecretKey":    "global-key",
		"optimize.security.authentication.oidc.secret.inlineSecret": "tenant-inline",
	}, []string{"templates/optimize/deployment.yaml"})

	s.Require().Contains(out, "tenant-inline")
	s.Require().NotContains(out, "global-oidc")
}

// A Secret reference with no key matches neither branch of normalizeSecretConfiguration, and the
// Optimize Deployment passes no defaultSecretName, so CAMUNDA_IDENTITY_CLIENT_SECRET would be
// dropped from the Deployment entirely and Optimize would start with no client secret. Failing the
// render is the only way the operator learns of it.
func (s *AuthIdentityTemplateTest) TestComponentExistingSecretWithoutAKeyIsRejected() {
	options := &helm.Options{SetValues: map[string]string{
		"global.identity.auth.enabled":                                "true",
		"global.identity.auth.issuer":                                 "https://issuer.example.com",
		"optimize.enabled":                                            "true",
		"orchestration.data.secondaryStorage.type":                    "elasticsearch",
		"optimize.security.authentication.oidc.secret.existingSecret": "tenant-oidc",
	}}
	_, err := helm.RenderTemplateE(s.T(), options, s.chartPath, s.release,
		[]string{"templates/optimize/deployment.yaml"})

	s.Require().Error(err)
	s.Require().Contains(err.Error(), "requires an existingSecretKey alongside its existingSecret")
}

// The same hole reached through the inherited block: the release names no Secret of its own, so the
// global one without a key is what would silently drop the env var.
func (s *AuthIdentityTemplateTest) TestGlobalExistingSecretWithoutAKeyIsRejected() {
	options := &helm.Options{SetValues: map[string]string{
		"global.identity.auth.enabled":                        "true",
		"global.identity.auth.issuer":                         "https://issuer.example.com",
		"global.identity.auth.optimize.secret.existingSecret": "global-oidc",
		"optimize.enabled":                                    "true",
		"orchestration.data.secondaryStorage.type":            "elasticsearch",
	}}
	_, err := helm.RenderTemplateE(s.T(), options, s.chartPath, s.release,
		[]string{"templates/optimize/deployment.yaml"})

	s.Require().Error(err)
	s.Require().Contains(err.Error(), "requires an existingSecretKey alongside its existingSecret")
}

// A release-scoped existingSecretKey renames the key inside an inherited existingSecret. There is
// nothing to rename when the inherited block carries only an inlineSecret, so the inherited inline
// value still wins - documented here so the helper's comment and its behaviour cannot drift apart.
func (s *AuthIdentityTemplateTest) TestComponentSecretKeyAloneOverAnInheritedInlineSecretIsInert() {
	out := s.render(map[string]string{
		"global.identity.auth.optimize.secret.inlineSecret":              "global-inline",
		"optimize.security.authentication.oidc.secret.existingSecretKey": "tenant-key",
	}, []string{"templates/optimize/deployment.yaml"})

	s.Require().Contains(out, "CAMUNDA_IDENTITY_CLIENT_SECRET")
	s.Require().Contains(out, "global-inline")
	s.Require().NotContains(out, "tenant-key")
}

// The Deployment must carry a checksum of the component-scoped identity ConfigMap, or an upgrade
// that only changes that ConfigMap leaves the running pod on the old identity values. The golden
// harness strips checksum lines, so this is the only guard against the annotation being dropped.
func (s *AuthIdentityTemplateTest) TestIdentityConfigMapChecksumRestartsThePod() {
	base := map[string]string{
		"optimize.security.authentication.oidc.issuer": "https://tenant-issuer.example.com",
	}
	changed := map[string]string{
		"optimize.security.authentication.oidc.issuer": "https://other-issuer.example.com",
	}

	first := s.checksumAnnotation(s.render(base, []string{"templates/optimize/deployment.yaml"}))
	second := s.checksumAnnotation(s.render(changed, []string{"templates/optimize/deployment.yaml"}))

	s.Require().NotEmpty(first, "the Deployment must annotate the identity ConfigMap checksum")
	s.Require().NotEqual(first, second, "changing the identity ConfigMap must change the checksum")
}

func (s *AuthIdentityTemplateTest) checksumAnnotation(deployment string) string {
	for _, line := range strings.Split(deployment, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "checksum/config-identity-env:") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "checksum/config-identity-env:"))
		}
	}
	return ""
}

// Identity provisions the Optimize client that Optimize then authenticates with, so both sides must
// read the same component-scoped values. Rendering only Optimize cannot catch a divergence, so this
// case sets every Optimize key on both levels with different values and asserts Identity emits the
// component one for the client id, redirect URL, audience, and secret.
func (s *AuthIdentityTemplateTest) TestCombinedModeProvisionsTheComponentScopedOptimizeClient() {
	options := &helm.Options{SetValues: map[string]string{
		"global.identity.auth.enabled":                           "true",
		"identity.enabled":                                       "true",
		"optimize.enabled":                                       "true",
		"orchestration.data.secondaryStorage.type":               "elasticsearch",
		"global.identity.keycloak.url.protocol":                  "https",
		"global.identity.keycloak.url.host":                      "keycloak.example.com",
		"global.identity.keycloak.url.port":                      "443",
		"global.identity.keycloak.auth.adminUser":                "admin",
		"global.identity.keycloak.auth.secret.existingSecret":    "keycloak",
		"global.identity.keycloak.auth.secret.existingSecretKey": "admin-password",

		"global.identity.auth.optimize.clientId":                 "global-optimize",
		"global.identity.auth.optimize.audience":                 "global-optimize-api",
		"global.identity.auth.optimize.redirectUrl":              "https://global.example.com/optimize",
		"global.identity.auth.optimize.secret.existingSecret":    "global-oidc",
		"global.identity.auth.optimize.secret.existingSecretKey": "global-key",

		"optimize.security.authentication.oidc.clientId":                 "component-optimize",
		"optimize.security.authentication.oidc.audience":                 "component-optimize-api",
		"optimize.security.authentication.oidc.redirectUrl":              "https://component.example.com/optimize",
		"optimize.security.authentication.oidc.secret.existingSecret":    "component-oidc",
		"optimize.security.authentication.oidc.secret.existingSecretKey": "component-key",
	}}

	deployment, err := helm.RenderTemplateE(s.T(), options, s.chartPath, s.release,
		[]string{"templates/identity/deployment.yaml"})
	s.Require().NoError(err)
	s.Require().Contains(deployment, `value: "component-optimize"`)
	s.Require().NotContains(deployment, "global-optimize")
	s.Require().Contains(deployment, "name: component-oidc")
	s.Require().Contains(deployment, "key: component-key")
	s.Require().NotContains(deployment, "global-oidc")

	configmap, err := helm.RenderTemplateE(s.T(), options, s.chartPath, s.release,
		[]string{"templates/identity/configmap.yaml"})
	s.Require().NoError(err)
	s.Require().Contains(configmap, "https://component.example.com/optimize")
	s.Require().Contains(configmap, `audience: "component-optimize-api"`)
	s.Require().NotContains(configmap, "global.example.com")
	s.Require().NotContains(configmap, "global-optimize-api")

	// The client secret Optimize sends must be the one Identity registered above.
	optimizeDeployment, err := helm.RenderTemplateE(s.T(), options, s.chartPath, s.release,
		[]string{"templates/optimize/deployment.yaml"})
	s.Require().NoError(err)
	s.Require().Contains(optimizeDeployment, "name: component-oidc")
	s.Require().Contains(optimizeDeployment, "key: component-key")
	s.Require().NotContains(optimizeDeployment, "global-oidc")
}

// An envFrom source's keys are unreadable at render time, so its presence proves nothing: an
// unrelated ConfigMap must not buy an exemption from the issuer guard, or Optimize deploys with an
// empty issuer and fails every token.
func (s *AuthIdentityTemplateTest) TestUndeclaredEnvFromDoesNotSatisfyTheIssuerConstraint() {
	options := &helm.Options{SetValues: map[string]string{
		"global.identity.auth.enabled":             "true",
		"identity.enabled":                         "true",
		"optimize.enabled":                         "true",
		"orchestration.data.secondaryStorage.type": "elasticsearch",
		"optimize.envFrom[0].configMapRef.name":    "unrelated-overrides",
	}}
	_, err := helm.RenderTemplateE(s.T(), options, s.chartPath, s.release,
		[]string{"templates/optimize/deployment.yaml"})

	s.Require().Error(err)
	s.Require().Contains(err.Error(), "requires optimize.security.authentication.oidc.issuer")
}

// Naming the variable is the release's own statement that one of its sources carries it, and that
// statement is what exempts the guard: optimize.envFrom layers after the identity env ConfigMap in
// the Deployment, so the value does reach the container.
func (s *AuthIdentityTemplateTest) TestDeclaredEnvFromSatisfiesTheIssuerConstraint() {
	options := &helm.Options{SetValues: map[string]string{
		"global.identity.auth.enabled":                             "true",
		"identity.enabled":                                         "true",
		"optimize.enabled":                                         "true",
		"orchestration.data.secondaryStorage.type":                 "elasticsearch",
		"optimize.envFrom[0].configMapRef.name":                    "optimize-identity-overrides",
		"optimize.security.authentication.oidc.envFromProvides[0]": "CAMUNDA_IDENTITY_ISSUER",
	}}
	out, err := helm.RenderTemplateE(s.T(), options, s.chartPath, s.release,
		[]string{"templates/optimize/deployment.yaml"})

	s.Require().NoError(err)
	s.Require().Contains(out, "name: optimize-identity-overrides")
}

// A declaration naming a variable no guard reads exempts nothing, and silently accepting it would
// leave its author believing a guard had been answered.
func (s *AuthIdentityTemplateTest) TestEnvFromProvidesRejectsAVariableItCannotExempt() {
	options := &helm.Options{SetValues: map[string]string{
		"global.identity.auth.enabled":                             "true",
		"identity.enabled":                                         "true",
		"optimize.enabled":                                         "true",
		"orchestration.data.secondaryStorage.type":                 "elasticsearch",
		"optimize.envFrom[0].configMapRef.name":                    "optimize-identity-overrides",
		"optimize.security.authentication.oidc.envFromProvides[0]": "CAMUNDA_IDENTITY_ISSUER_TYPO",
	}}
	_, err := helm.RenderTemplateE(s.T(), options, s.chartPath, s.release,
		[]string{"templates/optimize/deployment.yaml"})

	s.Require().Error(err)
	s.Require().Contains(err.Error(), "which exempts nothing")
}

// A release-scoped jwksUrl must win over the global one, so an Optimize-only release can point at
// its own provider.
func (s *AuthIdentityTemplateTest) TestComponentJwksUrlOverridesGlobal() {
	out := s.render(map[string]string{
		"global.identity.auth.jwksUrl":                  "https://global.example.com/certs",
		"optimize.security.authentication.oidc.jwksUrl": "https://component.example.com/certs",
	}, []string{"templates/optimize/configmap.yaml"})

	s.Require().Contains(out, `jwtSetUri: "https://component.example.com/certs"`)
	s.Require().NotContains(out, "global.example.com/certs")
}

// A non-Keycloak type has no endpoint layout to derive from, so without a jwksUrl at either level
// api.jwtSetUri would render empty and Optimize could validate no token. That state is rejected
// wherever Optimize authenticates - combined mode included, since a component-scoped OIDC config
// reaches it with no topology mode to signal it. The component key is the only way to name the
// endpoint per release.
func (s *AuthIdentityTemplateTest) TestComponentJwksUrlServesANonKeycloakType() {
	values := map[string]string{
		"global.identity.auth.enabled":                 "true",
		"global.identity.auth.issuerBackendUrl":        "http://keycloak.example.com/realms/camunda",
		"optimize.enabled":                             "true",
		"orchestration.data.secondaryStorage.type":     "elasticsearch",
		"optimize.security.authentication.oidc.type":   "GENERIC",
		"optimize.security.authentication.oidc.issuer": "https://issuer.example.com",
	}
	_, err := helm.RenderTemplateE(s.T(), &helm.Options{SetValues: values}, s.chartPath, s.release,
		[]string{"templates/optimize/configmap.yaml"})
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "requires optimize.security.authentication.oidc.jwksUrl")

	values["optimize.security.authentication.oidc.jwksUrl"] = "https://issuer.example.com/certs"
	out, err := helm.RenderTemplateE(s.T(), &helm.Options{SetValues: values}, s.chartPath, s.release,
		[]string{"templates/optimize/configmap.yaml"})
	s.Require().NoError(err)
	s.Require().Contains(out, `jwtSetUri: "https://issuer.example.com/certs"`)
}

// api.jwtSetUri is one config key an explicit optimize.env entry overrides in the container, so
// naming it there answers the JWKS requirement the same way the values key does.
func (s *AuthIdentityTemplateTest) TestOptimizeEnvMaySupplyTheJwksUri() {
	out := s.render(map[string]string{
		"optimize.security.authentication.oidc.type":   "GENERIC",
		"optimize.security.authentication.oidc.issuer": "https://issuer.example.com",
		"optimize.env[0].name":                         "CAMUNDA_OPTIMIZE_API_JWTSETURI",
		"optimize.env[0].value":                        "https://issuer.example.com/certs",
	}, []string{"templates/optimize/deployment.yaml"})

	s.Require().Contains(out, "CAMUNDA_OPTIMIZE_API_JWTSETURI")
}

// KEYCLOAK keeps deriving the endpoint, so releases that never set jwksUrl render unchanged.
func (s *AuthIdentityTemplateTest) TestKeycloakStillDerivesTheJwksUrl() {
	out := s.render(map[string]string{
		"optimize.security.authentication.oidc.type": "KEYCLOAK",
	}, []string{"templates/optimize/configmap.yaml"})

	s.Require().Contains(out,
		`jwtSetUri: "http://keycloak.example.com/realms/camunda/protocol/openid-connect/certs"`)
}
