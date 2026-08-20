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
  {{- include "camundaPlatform.authAudienceOptimize" . -}}
{{- end -}}

{{- define "optimize.authSecretConfig" -}}
  {{- toYaml .Values.global.identity.auth.optimize -}}
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
