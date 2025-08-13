{{/* vim: set filetype=mustache: */}}

{{/*
[orchestration] Create a default fully qualified app name.
*/}}
{{- define "orchestration.fullname" -}}
    {{- include "camundaPlatform.componentFullname" (dict
        "componentName" "orchestration"
        "componentValues" .Values.orchestration
        "context" $
    ) -}}
{{- end -}}

{{/*
[orchestration] The old name used in PVC which is used to avoid upgrade downtime.
*/}}
{{- define "orchestration.legacyName" -}}
    {{- printf "%s-zeebe" .Release.Name -}}
{{- end -}}

{{/*
[orchestration] Defines extra labels for orchestration.
*/}}
{{ define "orchestration.extraLabels" -}}
app.kubernetes.io/component: orchestration
app.kubernetes.io/version: {{ include "camundaPlatform.versionLabel" (dict "base" .Values.global "overlay" .Values.orchestration "chart" .Chart) | quote }}
{{- end }}

{{/*
[orchestration] Define common labels for orchestration, combining the match labels and transient labels, which might change on updating
(version depending). These labels shouldn't be used on matchLabels selector, since the selectors are immutable.
*/}}
{{- define "orchestration.labels" -}}
    {{- include "camundaPlatform.labels" . }}
    {{- "\n" }}
    {{- include "orchestration.extraLabels" . }}
{{- end -}}

{{/*
[orchestration] Defines match labels for orchestration, which are extended by sub-charts and should be used in matchLabels selectors.
*/}}
{{- define "orchestration.matchLabels" -}}
    {{- include "camundaPlatform.matchLabels" . }}
app.kubernetes.io/component: orchestration
{{- end -}}

{{/*
[orchestration] Create the name of the service account to use.
*/}}
{{- define "orchestration.serviceAccountName" -}}
    {{- if .Values.orchestration.serviceAccount.enabled -}}
        {{- default (include "orchestration.fullname" .) .Values.orchestration.serviceAccount.name -}}
    {{- else -}}
        {{- default "default" .Values.orchestration.serviceAccount.name -}}
    {{- end -}}
{{- end -}}


{{/*
[web-modeler] Define variables related to authentication.
*/}}
{{- define "orchestration.authClientId" -}}
    {{- .Values.global.identity.auth.orchestration.clientId | default "orchestration" -}}
{{- end -}}

{{- define "orchestration.authClientSecretName" -}}
    {{- if and .Values.global.identity.auth.orchestration.existingSecret (not (typeIs "string" .Values.global.identity.auth.orchestration.existingSecret)) -}}
        {{- include "common.secrets.name" (dict "existingSecret" .Values.global.identity.auth.orchestration.existingSecret "context" $) -}}
    {{- else -}}
        {{- include "camundaPlatform.identitySecretName" (dict "context" . "component" "orchestration") -}}
    {{- end -}}
{{- end -}}

{{- define "orchestration.authClientSecretKey" -}}
    {{ .Values.global.identity.auth.orchestration.existingSecretKey }}
{{- end -}}

{{- define "orchestration.authAudience" -}}
    {{- .Values.global.identity.auth.orchestration.audience | default "orchestration-api" -}}
{{- end -}}

{{- define "orchestration.authTokenScope" -}}
    {{- .Values.global.identity.auth.orchestration.tokenScope -}}
{{- end -}}

{{- define "orchestration.enabledProfiles" -}}
    {{- $enabledProfiles := list -}}
    {{- range $k, $v := .Values.orchestration.profiles }}
    {{- if eq $v true }}
        {{- $enabledProfiles = append $enabledProfiles $k }}
    {{- end }}
    {{- end }}
    {{- join "," $enabledProfiles }}
{{- end -}}

{{- define "orchestration.secondaryStorage" -}}
    {{- if .Values.global.noSecondaryStorage -}}
        none
    {{- else if .Values.global.elasticsearch.enabled -}}
        elasticsearch
    {{- else if .Values.global.opensearch.enabled -}}
        opensearch
    {{- else -}}
        {{- fail "Please enable a secondary storage type. Either Elasticsearch or OpenSearch" -}}
    {{- end -}}
{{- end -}}
