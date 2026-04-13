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
