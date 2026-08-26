{{/* vim: set filetype=mustache: */}}

{{/*
Create a default fully qualified app name.
*/}}

{{- define "optimize.fullname" -}}
    {{- include "camundaPlatform.componentFullname" (dict
        "componentName" "optimize"
        "componentValues" .Values.optimize
        "context" $
    ) -}}
{{- end -}}

{{- define "optimize.extraLabels" -}}
    {{- include "camundaPlatform.componentExtraLabels" (dict "componentName" "optimize" "componentValuesKey" "optimize" "context" $) -}}
{{- end -}}

{{- define "optimize.labels" -}}
    {{- include "camundaPlatform.componentLabels" (dict "componentName" "optimize" "componentValuesKey" "optimize" "context" $) -}}
{{- end -}}

{{- define "optimize.matchLabels" -}}
    {{- include "camundaPlatform.componentMatchLabels" (dict "componentName" "optimize" "context" $) -}}
{{- end -}}

{{/*
[optimize] Create the name of the service account to use
*/}}
{{- define "optimize.serviceAccountName" -}}
    {{- include "camundaPlatform.serviceAccountName" (dict
        "component" "optimize"
        "context" $
    ) -}}
{{- end -}}

{{/*
[optimize] Get the image pull secrets.
*/}}
{{- define "optimize.imagePullSecrets" -}}
    {{- include "camundaPlatform.imagePullSecrets" (dict
        "component" "optimize"
        "context" $
    ) -}}
{{- end }}

{{- define "optimize.effectiveAuthClientId" -}}
  {{- $oidc := dig "security" "authentication" "oidc" dict .Values.optimize -}}
  {{- $oidc.clientId | default .Values.global.identity.auth.optimize.clientId -}}
{{- end -}}

{{- define "optimize.effectiveAuthRedirectUrl" -}}
  {{- $oidc := dig "security" "authentication" "oidc" dict .Values.optimize -}}
  {{- $oidc.redirectUrl | default .Values.global.identity.auth.optimize.redirectUrl -}}
{{- end -}}

{{/*
[optimize] Overlay the release-scoped Optimize client secret onto `global.identity.auth.optimize.secret`.
The overlay is per-field: `existingSecretKey` alone renames the key inside the inherited
`existingSecret`, while `inlineSecret` or `existingSecret` replaces the global source and drops the
other form, which `camundaPlatform.normalizeSecretConfiguration` would otherwise still prefer.
A rename needs a name to rename: when the inherited block carries only an `inlineSecret` there is no
`existingSecret` to point at, so the release-scoped key is inert and the inherited inline value
still wins. Name the Secret on the release too if it must override the inherited inline value.
*/}}
{{- define "optimize.effectiveAuthSecret" -}}
  {{- $oidc := dig "security" "authentication" "oidc" dict .Values.optimize -}}
  {{- $secret := $oidc.secret | default dict -}}
  {{- $global := .Values.global.identity.auth.optimize.secret | default dict -}}
  {{- if $secret.inlineSecret -}}
    {{- toYaml (dict "inlineSecret" $secret.inlineSecret) -}}
  {{- else if $secret.existingSecret -}}
    {{- toYaml (dict
        "existingSecret" $secret.existingSecret
        "existingSecretKey" ($secret.existingSecretKey | default $global.existingSecretKey | default "")
    ) -}}
  {{- else if $secret.existingSecretKey -}}
    {{- $merged := deepCopy $global -}}
    {{- $_ := set $merged "existingSecretKey" $secret.existingSecretKey -}}
    {{- toYaml $merged -}}
  {{- else -}}
    {{- toYaml $global -}}
  {{- end -}}
{{- end -}}

{{/*
[optimize] Resolve whether this Optimize release authenticates. A release-scoped method wins so an
Optimize-only release can decide for itself; empty follows global.identity.auth.enabled, which keeps
every existing values file rendering unchanged.
*/}}
{{- define "optimize.authMethod" -}}
  {{- $method := dig "security" "authentication" "method" "" .Values.optimize -}}
  {{- $method | default (ternary "oidc" "none" .Values.global.identity.auth.enabled) -}}
{{- end -}}

{{- define "optimize.authEnabled" -}}
  {{- if eq (include "optimize.authMethod" .) "oidc" -}}true{{- else -}}false{{- end -}}
{{- end -}}

{{- define "optimize.effectiveAuthType" -}}
  {{- $oidc := dig "security" "authentication" "oidc" dict .Values.optimize -}}
  {{- $oidc.type | default (include "camundaPlatform.authIssuerType" .) -}}
{{- end -}}

{{- define "optimize.effectiveAuthIssuer" -}}
  {{- $oidc := dig "security" "authentication" "oidc" dict .Values.optimize -}}
  {{- $oidc.issuer | default (include "camundaPlatform.authIssuerUrlWithFallback" .) -}}
{{- end -}}

{{- define "optimize.effectiveAuthIssuerBackendUrl" -}}
  {{- $oidc := dig "security" "authentication" "oidc" dict .Values.optimize -}}
  {{- $oidc.issuerBackendUrl | default (include "camundaPlatform.authIssuerBackendUrl" .) -}}
{{- end -}}

{{/*
[optimize] Resolve `api.jwtSetUri`. Only KEYCLOAK derives from the issuer backend URL; any other
type renders empty unless jwksUrl is set on the release or on global.

KEYCLOAK derives it only while that backend URL resolves. Appending the endpoint to an empty one
yields the relative "/protocol/openid-connect/certs", which is not a URL Optimize can fetch from -
and being non-empty it would satisfy the JWKS guard, hiding exactly the state that guard reports. A
KEYCLOAK release may legitimately reach here with nothing to derive from: a configured public issuer
answers the issuer guard on its own, leaving the backend URL empty. Render nothing then, so the
guard asks for an explicit jwksUrl.
*/}}
{{- define "optimize.effectiveAuthJwksUrl" -}}
  {{- $oidc := dig "security" "authentication" "oidc" dict .Values.optimize -}}
  {{- if $oidc.jwksUrl -}}
    {{- tpl $oidc.jwksUrl . -}}
  {{- else if .Values.global.identity.auth.jwksUrl -}}
    {{- tpl .Values.global.identity.auth.jwksUrl . -}}
  {{- else if eq (include "optimize.effectiveAuthType" .) "KEYCLOAK" -}}
    {{- $issuerBackendUrl := include "optimize.effectiveAuthIssuerBackendUrl" . -}}
    {{- if $issuerBackendUrl -}}
      {{- printf "%s/protocol/openid-connect/certs" $issuerBackendUrl -}}
    {{- end -}}
  {{- end -}}
{{- end -}}

{{/*
[optimize] Release-scoped counterparts of the `camundaPlatform.authIssuer*` endpoint helpers, for the
`camunda.security.authentication.oidc` block. That block is the config Optimize actually authenticates
with, so it has to read the same release-scoped overrides as `camunda.identity` above it; left on the
global helpers, a Physical Tenant's Optimize would authenticate against the shared release provider it
was configured away from. Each one keeps the global helper's shape and falls back to it, so a release
that overrides nothing renders exactly as before.
*/}}
{{- define "optimize.effectiveAuthIssuerUrl" -}}
  {{- $oidc := dig "security" "authentication" "oidc" dict .Values.optimize -}}
  {{- if $oidc.issuer -}}
    {{- tpl $oidc.issuer . -}}
  {{- else -}}
    {{- include "camundaPlatform.authIssuerUrl" . -}}
  {{- end -}}
{{- end -}}

{{- define "optimize.effectiveAuthUrlEndpointAuth" -}}
  {{- if .Values.global.identity.auth.authUrl -}}
    {{- tpl .Values.global.identity.auth.authUrl . -}}
  {{- else if eq (include "optimize.effectiveAuthType" .) "KEYCLOAK" -}}
    {{- include "optimize.effectiveAuthIssuer" . -}}/protocol/openid-connect/auth
  {{- end -}}
{{- end -}}

{{- define "optimize.effectiveAuthBackendUrlEndpointToken" -}}
  {{- if .Values.global.identity.auth.tokenUrl -}}
    {{- tpl .Values.global.identity.auth.tokenUrl . -}}
  {{- else if eq (include "optimize.effectiveAuthType" .) "KEYCLOAK" -}}
    {{- include "optimize.effectiveAuthIssuerBackendUrl" . -}}/protocol/openid-connect/token
  {{- end -}}
{{- end -}}

{{- define "optimize.effectiveIdentityUrl" -}}
  {{- $url := dig "identity" "service" "url" "" .Values.optimize -}}
  {{- $url | default (include "camundaPlatform.identityURL" .) -}}
{{- end -}}

{{/*
[optimize] True when this release overrides a release-shared identity value, which is what makes the
component-scoped identity env ConfigMap render.
*/}}
{{- define "optimize.hasIdentityOverrides" -}}
  {{- $oidc := dig "security" "authentication" "oidc" dict .Values.optimize -}}
  {{- if or $oidc.type $oidc.issuer $oidc.issuerBackendUrl (dig "identity" "service" "url" "" .Values.optimize) -}}
true
  {{- end -}}
{{- end -}}

{{/*
[optimize] The identity variables a release may declare its optimize.envFrom sources supply, mapped
to the guard each one exempts. Names are the container env vars the chart itself would otherwise
have to resolve: the two the identity env ConfigMap carries, and the documented override for
api.jwtSetUri.

The api.jwtSetUri name is the one Optimize documents for that config key
(self-managed/components/optimize/configuration/system-configuration), not a mapping derived from
the key path: Optimize's env aliases are not mechanical (es.security.password is
CAMUNDA_OPTIMIZE_ELASTICSEARCH_SECURITY_PASSWORD), so deriving one would name a variable Optimize
never reads and exempt a guard nothing answered.
*/}}
{{- define "optimize.identityIssuerEnvNames" -}}
CAMUNDA_IDENTITY_ISSUER CAMUNDA_IDENTITY_ISSUER_BACKEND_URL
{{- end -}}

{{- define "optimize.jwksEnvNames" -}}
SPRING_SECURITY_OAUTH2_RESOURCESERVER_JWT_JWK_SET_URI
{{- end -}}

{{- define "optimize.clientSecretEnvNames" -}}
CAMUNDA_IDENTITY_CLIENT_SECRET
{{- end -}}

{{- define "optimize.identityUrlEnvNames" -}}
CAMUNDA_IDENTITY_BASEURL
{{- end -}}

{{- define "optimize.declarableEnvNames" -}}
{{- printf "%s %s %s %s" (include "optimize.identityIssuerEnvNames" .) (include "optimize.jwksEnvNames" .) (include "optimize.clientSecretEnvNames" .) (include "optimize.identityUrlEnvNames" .) -}}
{{- end -}}

{{/*
[optimize] Whether optimize.env names any of `names` with a value the container will actually read:
a non-empty literal value, or any valueFrom. The Deployment renders optimize.env as container env,
which the kubelet resolves over every envFrom source, so such an entry supersedes both the identity
env ConfigMap and the rendered config file.

The Deployment passes optimize.env through tpl, so the value is compared after rendering: an entry
whose template resolves to nothing reaches the container as an empty variable and answers no guard,
which the pre-tpl string cannot tell apart from a real endpoint.
Call with (dict "ctx" $ "names" (list "NAME" ...)).
*/}}
{{- define "optimize.envSetsAnyOf" -}}
  {{- $names := .names -}}
  {{- $set := false -}}
  {{- range $entry := (.ctx.Values.optimize.env | default list) -}}
    {{- if has $entry.name $names -}}
      {{- $value := tpl (toString ($entry.value | default "")) $.ctx -}}
      {{- if or (not (empty $value)) (not (empty $entry.valueFrom)) -}}
        {{- $set = true -}}
      {{- end -}}
    {{- end -}}
  {{- end -}}
  {{- if $set -}}true{{- else -}}false{{- end -}}
{{- end -}}

{{/*
[optimize] Whether optimize.security.authentication.oidc.envFromProvides declares any of `names`.
An envFrom source's keys are unreadable at render time, so its presence proves nothing - naming a
variable here is the release's own statement that one of its sources carries it, and that statement
is what exempts the matching guard. An unrelated ConfigMap or Secret exempts nothing.

The declaration is a statement *about* optimize.envFrom, so it holds only while there is a source
for it to describe: with optimize.envFrom empty there is nothing the named variable could arrive
from, and the exemption would leave the container with the empty value this chart rendered.
Presence of a source is necessary here and still never sufficient - constraints.tpl rejects the
declaration outright in that state, and this keeps the helper honest on its own.
Call with (dict "ctx" $ "names" (list "NAME" ...)).
*/}}
{{- define "optimize.envFromDeclaresAnyOf" -}}
  {{- $declared := (dig "security" "authentication" "oidc" "envFromProvides" list .ctx.Values.optimize) | default list -}}
  {{- $found := false -}}
  {{- if not (empty .ctx.Values.optimize.envFrom) -}}
    {{- range $name := $declared -}}
      {{- if has $name $.names -}}
        {{- $found = true -}}
      {{- end -}}
    {{- end -}}
  {{- end -}}
  {{- if $found -}}true{{- else -}}false{{- end -}}
{{- end -}}

{{/*
[optimize] Whether the issuer may reach the container past the identity env ConfigMap. Both
optimize.env and optimize.envFrom are listed after it in the Deployment, so either supersedes it -
but only a declared envFrom variable counts, never mere presence of a source.
*/}}
{{- define "optimize.identityIssuerMayComeFromEnv" -}}
  {{- $names := splitList " " (include "optimize.identityIssuerEnvNames" .) -}}
  {{- if or
        (eq (include "optimize.envSetsAnyOf" (dict "ctx" . "names" $names)) "true")
        (eq (include "optimize.envFromDeclaresAnyOf" (dict "ctx" . "names" $names)) "true") -}}
true
  {{- else -}}
false
  {{- end -}}
{{- end -}}

{{/*
[optimize] Whether the Identity base URL may reach the container without this chart resolving it.
The identity env ConfigMap always states CAMUNDA_IDENTITY_BASEURL, but the Deployment lists
optimize.env and optimize.envFrom after it, so either supersedes what the chart resolved - and as
everywhere else here, an envFrom source counts only once the release declares the variable it
carries.
*/}}
{{- define "optimize.identityUrlMayComeFromEnv" -}}
  {{- $names := splitList " " (include "optimize.identityUrlEnvNames" .) -}}
  {{- if or
        (eq (include "optimize.envSetsAnyOf" (dict "ctx" . "names" $names)) "true")
        (eq (include "optimize.envFromDeclaresAnyOf" (dict "ctx" . "names" $names)) "true") -}}
true
  {{- else -}}
false
  {{- end -}}
{{- end -}}

{{/*
[optimize] Whether api.jwtSetUri may be supplied as container env instead of by this chart. The
Optimize config file the chart renders is overridden by the env var for the same config path, so an
explicit optimize.env entry or a declared envFrom variable both count.
*/}}
{{- define "optimize.jwksMayComeFromEnv" -}}
  {{- $names := splitList " " (include "optimize.jwksEnvNames" .) -}}
  {{- if or
        (eq (include "optimize.envSetsAnyOf" (dict "ctx" . "names" $names)) "true")
        (eq (include "optimize.envFromDeclaresAnyOf" (dict "ctx" . "names" $names)) "true") -}}
true
  {{- else -}}
false
  {{- end -}}
{{- end -}}

{{/*
[optimize] Whether the client secret may reach the container without this chart resolving it. With an
existingSecret and no key the Deployment emits no CAMUNDA_IDENTITY_CLIENT_SECRET at all
(camundaPlatform.emitEnvVarFromSecretConfig matches neither branch), so nothing of the chart's is
there to outrank: an optimize.env entry - including a valueFrom secretKeyRef, which is how a release
names the key itself - and a declared envFrom variable both land it.
*/}}
{{- define "optimize.clientSecretMayComeFromEnv" -}}
  {{- $names := splitList " " (include "optimize.clientSecretEnvNames" .) -}}
  {{- if or
        (eq (include "optimize.envSetsAnyOf" (dict "ctx" . "names" $names)) "true")
        (eq (include "optimize.envFromDeclaresAnyOf" (dict "ctx" . "names" $names)) "true") -}}
true
  {{- else -}}
false
  {{- end -}}
{{- end -}}

{{/*
[optimize] Whether the release itself supplies api.jwtSetUri, leaving the chart nothing to resolve.
Two config sources count besides container env, and both are readable at render time, so for them
the exemption is the value itself rather than a declaration about it:

  - optimize.configuration replaces environment-config.yaml wholesale
    (templates/optimize/configmap.yaml), so the chart renders no api.jwtSetUri at all and the key the
    release wrote there is the only one Optimize reads.
  - optimize.extraConfiguration is imported through spring.config.import
    (optimize.springConfigImport). Since 8.9 Optimize's own configuration loader applies those files
    too, in import order and after environment-config.yaml, so a later file overrides the empty
    api.jwtSetUri this chart would render
    (camunda-docs self-managed/deployment/helm/configure/application-configs, "Optimize").

This is where api.jwtSetUri parts company with the issuer: the identity env ConfigMap sets
CAMUNDA_IDENTITY_ISSUER as container env, empty value included, and container env outranks every
imported file - so extraConfiguration can never be an issuer exemption. api.jwtSetUri has no
chart-rendered env var, so file order is what decides it.

Either source must bind the key to a non-empty scalar. "api.jwtSetUri:" with no value parses as null
and leaves Optimize with no endpoint to fetch signing keys from, which is the state the guard exists
to report - the key being present says nothing about that.
*/}}
{{- define "optimize.jwksSuppliedByRelease" -}}
  {{- $configuration := .Values.optimize.configuration | default "" -}}
  {{- $asExtraConfig := dict "extraConfiguration" (list (dict "file" "environment-config.yaml" "content" $configuration)) "path" (list "api" "jwtSetUri") -}}
  {{- $imported := dict "extraConfiguration" (.Values.optimize.extraConfiguration | default list) "path" (list "api" "jwtSetUri") -}}
  {{- if or
        (eq (include "optimize.jwksMayComeFromEnv" .) "true")
        (and (not (empty $configuration))
          (eq (include "camundaPlatform.extraConfigHasNonEmptyValueAtPath" $asExtraConfig) "true"))
        (eq (include "camundaPlatform.extraConfigHasNonEmptyValueAtPath" $imported) "true") -}}
true
  {{- else -}}
false
  {{- end -}}
{{- end -}}

{{/*
[optimize] True when any leaf of a values subtree is stated. Recursive, so a subtree of empty
defaults - which every release carries for optimize.security.authentication - reads as unstated.
*/}}
{{- define "optimize._subtreeStatesAnything" -}}
  {{- $found := "" -}}
  {{- if kindIs "map" . -}}
    {{- range $key, $value := . -}}
      {{- if eq (include "optimize._subtreeStatesAnything" $value) "true" -}}
        {{- $found = "true" -}}
      {{- end -}}
    {{- end -}}
  {{- else if not (empty .) -}}
    {{- $found = "true" -}}
  {{- end -}}
  {{- $found -}}
{{- end -}}

{{/*
[optimize] True when the release configures Optimize's own identity or authentication instead of
inheriting global.identity.auth wholesale. This is what scopes the Optimize identity guards outside
global.topology.mode=optimize: a component-scoped OIDC config reaches an empty issuer or an empty
api.jwtSetUri in combined mode with no mode to signal it, while a release that never mentions
optimize.security.authentication or optimize.identity renders exactly as it did before these keys
existed. Widening the guards to every global.identity.auth.enabled release is a chart-wide change,
tracked separately in issue #6929.
*/}}
{{- define "optimize.hasComponentScopedAuth" -}}
  {{- if or
        (eq (include "optimize._subtreeStatesAnything" (dig "security" "authentication" dict .Values.optimize)) "true")
        (eq (include "optimize.hasIdentityOverrides" .) "true") -}}
true
  {{- else -}}
false
  {{- end -}}
{{- end -}}

{{/*
[optimize] Whether the component-scoped identity env ConfigMap must render: either it carries
overrides, or it is the only source because the release-shared ConfigMap is gated off.
*/}}
{{- define "optimize.needsIdentityConfigMap" -}}
  {{- if and (eq (include "optimize.authEnabled" .) "true") (or (eq (include "optimize.hasIdentityOverrides" .) "true") (not .Values.global.identity.auth.enabled)) -}}true{{- else -}}false{{- end -}}
{{- end -}}

{{- define "optimize.authClientId" -}}
  {{- include "optimize.effectiveAuthClientId" . -}}
{{- end -}}

{{- define "optimize.authAudience" -}}
  {{- include "camundaPlatform.authAudienceOptimize" . -}}
{{- end -}}

{{- define "optimize.authSecretConfig" -}}
  {{- $global := deepCopy .Values.global.identity.auth.optimize -}}
  {{- $_ := set $global "secret" (include "optimize.effectiveAuthSecret" . | fromYaml) -}}
  {{- $_ := set $global "clientId" (include "optimize.effectiveAuthClientId" .) -}}
  {{- $_ := set $global "redirectUrl" (include "optimize.effectiveAuthRedirectUrl" .) -}}
  {{- toYaml $global -}}
{{- end -}}

{{/*
[optimize] Resolve the effective TLS config: Elasticsearch first, OpenSearch
when only that one carries actual secret config.
*/}}
{{- define "optimize.effectiveTlsConfig" -}}
{{- if eq (include "camundaPlatform.hasSecretConfig" (dict "config" .Values.optimize.database.elasticsearch.tls)) "true" -}}
  {{- toYaml .Values.optimize.database.elasticsearch.tls -}}
{{- else if eq (include "camundaPlatform.hasSecretConfig" (dict "config" .Values.optimize.database.opensearch.tls)) "true" -}}
  {{- toYaml .Values.optimize.database.opensearch.tls -}}
{{- else -}}
  {{- toYaml (dict) -}}
{{- end -}}
{{- end -}}

{{/*
[optimize] Check if TLS is configured on the optimize database config
for either Elasticsearch or OpenSearch. Returns "true" or "false".
*/}}
{{- define "optimize.hasTlsConfig" -}}
{{- $tlsConfig := include "optimize.effectiveTlsConfig" . | fromYaml -}}
{{- if eq (include "camundaPlatform.hasSecretConfig" (dict "config" $tlsConfig)) "true" -}}
true
{{- else -}}
false
{{- end -}}
{{- end -}}

{{/*
[optimize] Resolve zeebe prefix.
Precedence matches optimize.defaultConfig: ES is checked first, OS only when ES is off.
Uses the component-specific key (optimize.database.<backend>.prefix) with a hardcoded
"zeebe-record" fallback. When neither backend is explicitly enabled, falls back to
"zeebe-record".
*/}}
{{- define "optimize.indexPrefix" -}}
{{- if .Values.optimize.database.elasticsearch.enabled -}}
  {{- .Values.optimize.database.elasticsearch.prefix | default "zeebe-record" -}}
{{- else if .Values.optimize.database.opensearch.enabled -}}
  {{- .Values.optimize.database.opensearch.prefix | default "zeebe-record" -}}
{{- else -}}
  {{- "zeebe-record" -}}
{{- end -}}
{{- end -}}

{{/*
[optimize] Resolve the Elasticsearch URL from the optimize database config.
*/}}
{{- define "optimize.effectiveEsURL" -}}
{{- .Values.optimize.database.elasticsearch.url.protocol }}://{{ include "camundaPlatform.elasticsearchHost" . }}:{{ .Values.optimize.database.elasticsearch.url.port -}}
{{- end -}}

{{/*
[optimize] Resolve the OpenSearch URL from the optimize database config.
*/}}
{{- define "optimize.effectiveOsURL" -}}
{{- .Values.optimize.database.opensearch.url.protocol }}://{{ include "camundaPlatform.opensearchHost" . }}:{{ .Values.optimize.database.opensearch.url.port -}}
{{- end -}}

{{/*
[optimize] Build a comma-separated spring.config.import line from extraConfiguration files.
Entries with springImport: false are excluded.
*/}}
{{- define "optimize.springConfigImport" -}}
{{- $imports := list -}}
{{- range .Values.optimize.extraConfiguration -}}
  {{- if not (and (hasKey . "springImport") (eq .springImport false)) -}}
    {{- $imports = append $imports (printf "optional:file:/optimize/config/%s" .file) -}}
  {{- end -}}
{{- end -}}
{{- join "," $imports -}}
{{- end -}}
