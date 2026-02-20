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

{{/*
Defines extra labels for optimize.
*/}}
{{ define "optimize.extraLabels" -}}
app.kubernetes.io/component: optimize
app.kubernetes.io/version: {{ include "camundaPlatform.versionLabel" (dict "base" .Values.global "overlay" .Values.optimize "chart" .Chart) | quote }}
{{- end }}

{{/*
Define common labels for optimize, combining the match labels and transient labels, which might change on updating
(version depending). These labels shouldn't be used on matchLabels selector, since the selectors are immutable.
*/}}
{{- define "optimize.labels" -}}
    {{- include "camundaPlatform.labels" . }}
    {{- "\n" }}
    {{- include "optimize.extraLabels" . }}
{{- end -}}

{{/*
Defines match labels for optimize, which are extended by sub-charts and should be used in matchLabels selectors.
*/}}
{{- define "optimize.matchLabels" -}}
    {{- include "camundaPlatform.matchLabels" . }}
app.kubernetes.io/component: optimize
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

{{- define "optimize.authClientId" -}}
  {{- .Values.global.identity.auth.optimize.clientId -}}
{{- end -}}

{{- define "optimize.authAudience" -}}
  {{- .Values.global.identity.auth.optimize.audience | default "optimize-api" -}}
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

{{/*
[optimize] Check if TLS is configured at either the optimize-database or global level
for either Elasticsearch or OpenSearch. Returns "true" or "false".
*/}}
{{- define "optimize.hasTlsConfig" -}}
{{- $esTls := include "optimize.effectiveEsTlsConfig" . | fromYaml -}}
{{- $osTls := include "optimize.effectiveOsTlsConfig" . | fromYaml -}}
{{- if or (eq (include "camundaPlatform.hasSecretConfig" (dict "config" $esTls)) "true") (eq (include "camundaPlatform.hasSecretConfig" (dict "config" $osTls)) "true") -}}
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
[optimize] Resolve zeebe prefix
*/}}
{{- define "optimize.indexPrefix" }}
{{- if ne .Values.optimize.database.elasticsearch.prefix "zeebe-record" -}}
{{ .Values.optimize.database.elasticsearch.prefix }}
{{- else if ne .Values.optimize.database.opensearch.prefix "zeebe-record" -}}
{{ .Values.optimize.database.opensearch.prefix }}
{{- else if ne .Values.global.elasticsearch.prefix "zeebe-record" -}}
{{ .Values.global.elasticsearch.prefix }}
{{- else if ne .Values.global.opensearch.prefix "zeebe-record" -}}
{{ .Values.global.opensearch.prefix }}
{{- else -}}
zeebe-record
{{- end }}
{{- end }}

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
