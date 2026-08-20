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

{{- define "optimize.effectiveAuthJwksUrl" -}}
  {{- if .Values.global.identity.auth.jwksUrl -}}
    {{- tpl .Values.global.identity.auth.jwksUrl . -}}
  {{- else if eq (include "optimize.effectiveAuthType" .) "KEYCLOAK" -}}
    {{- include "optimize.effectiveAuthIssuerBackendUrl" . -}}/protocol/openid-connect/certs
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
[optimize] Resolve the effective Elasticsearch TLS config.
Prefers optimize.database.elasticsearch.tls if it has actual secret config,
otherwise falls back to global.elasticsearch.tls.
Note: We cannot use `| default` on maps because a map with empty-string values
is still "non-empty" in Helm and `default` will never fall through.
*/}}
{{- define "optimize.effectiveEsTlsConfig" -}}
{{- if eq (include "camundaPlatform.hasSecretConfig" (dict "config" .Values.optimize.database.elasticsearch.tls)) "true" -}}
  {{- toYaml .Values.optimize.database.elasticsearch.tls -}}
{{- else -}}
  {{- toYaml .Values.global.elasticsearch.tls -}}
{{- end -}}
{{- end -}}

{{/*
[optimize] Resolve the effective OpenSearch TLS config.
*/}}
{{- define "optimize.effectiveOsTlsConfig" -}}
{{- if eq (include "camundaPlatform.hasSecretConfig" (dict "config" .Values.optimize.database.opensearch.tls)) "true" -}}
  {{- toYaml .Values.optimize.database.opensearch.tls -}}
{{- else -}}
  {{- toYaml .Values.global.opensearch.tls -}}
{{- end -}}
{{- end -}}

{{- define "optimize.effectiveTlsConfig" -}}
{{- $esTls := include "optimize.effectiveEsTlsConfig" . | fromYaml -}}
{{- $osTls := include "optimize.effectiveOsTlsConfig" . | fromYaml -}}
{{- if eq (include "camundaPlatform.hasSecretConfig" (dict "config" $esTls)) "true" -}}
  {{- toYaml $esTls -}}
{{- else if eq (include "camundaPlatform.hasSecretConfig" (dict "config" $osTls)) "true" -}}
  {{- toYaml $osTls -}}
{{- else -}}
  {{- toYaml (dict) -}}
{{- end -}}
{{- end -}}

{{/*
[optimize] Check if TLS is configured at either the optimize-database or global level
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
[optimize] Resolve the effective Elasticsearch auth config.
Prefers optimize.database.elasticsearch.auth if it has actual secret config,
otherwise falls back to global.elasticsearch.auth.
*/}}
{{- define "optimize.effectiveEsAuthConfig" -}}
{{- if eq (include "camundaPlatform.hasSecretConfig" (dict "config" .Values.optimize.database.elasticsearch.auth)) "true" -}}
  {{- toYaml .Values.optimize.database.elasticsearch.auth -}}
{{- else -}}
  {{- toYaml .Values.global.elasticsearch.auth -}}
{{- end -}}
{{- end -}}

{{/*
[optimize] Resolve zeebe prefix.
Precedence matches optimize.defaultConfig: ES is checked first, OS only when ES is off.
In 8.10 the global prefix keys are deprecated, so this helper uses the component-specific
key directly (optimize.database.<backend>.prefix) with a hardcoded "zeebe-record" fallback.
When neither backend is explicitly enabled, falls back to "zeebe-record".
*/}}
{{- define "optimize.indexPrefix" -}}
{{- if or .Values.global.elasticsearch.enabled .Values.optimize.database.elasticsearch.enabled -}}
  {{- .Values.optimize.database.elasticsearch.prefix | default "zeebe-record" -}}
{{- else if or .Values.global.opensearch.enabled .Values.optimize.database.opensearch.enabled -}}
  {{- .Values.optimize.database.opensearch.prefix | default "zeebe-record" -}}
{{- else -}}
  {{- "zeebe-record" -}}
{{- end -}}
{{- end -}}

{{/*
[optimize] Resolve the effective OpenSearch auth config.
*/}}
{{- define "optimize.effectiveOsAuthConfig" -}}
{{- if eq (include "camundaPlatform.hasSecretConfig" (dict "config" .Values.optimize.database.opensearch.auth)) "true" -}}
  {{- toYaml .Values.optimize.database.opensearch.auth -}}
{{- else -}}
  {{- toYaml .Values.global.opensearch.auth -}}
{{- end -}}
{{- end -}}

{{/*
[optimize] Resolve the effective OpenSearch AWS mode from the component-first chain.
*/}}
{{- define "optimize.effectiveOsAwsEnabled" -}}
{{- or .Values.optimize.database.opensearch.aws.enabled .Values.global.opensearch.aws.enabled -}}
{{- end -}}

{{/*
[optimize] Resolve the effective Elasticsearch username from the component-first chain.
*/}}
{{- define "optimize.effectiveEsUsername" -}}
{{- .Values.optimize.database.elasticsearch.auth.username | default .Values.global.elasticsearch.auth.username -}}
{{- end -}}

{{/*
[optimize] Resolve the effective OpenSearch username from the component-first chain.
*/}}
{{- define "optimize.effectiveOsUsername" -}}
{{- .Values.optimize.database.opensearch.auth.username | default .Values.global.opensearch.auth.username -}}
{{- end -}}

{{/*
[optimize] Resolve the effective Elasticsearch URL from the component-first host/port/protocol chain.
*/}}
{{- define "optimize.effectiveEsURL" -}}
{{- .Values.optimize.database.elasticsearch.url.protocol | default .Values.global.elasticsearch.url.protocol }}://{{ include "camundaPlatform.elasticsearchHost" . }}:{{ include "camundaPlatform.elasticsearchPort" . -}}
{{- end -}}

{{/*
[optimize] Resolve the effective OpenSearch URL from the component-first host/port/protocol chain.
*/}}
{{- define "optimize.effectiveOsURL" -}}
{{- .Values.optimize.database.opensearch.url.protocol | default .Values.global.opensearch.url.protocol }}://{{ include "camundaPlatform.opensearchHost" . }}:{{ include "camundaPlatform.opensearchPort" . -}}
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
