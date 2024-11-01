{{/* vim: set filetype=mustache: */}}

{{/*
[core] Create a default fully qualified app name.
*/}}
{{- define "core.fullname" -}}
    {{- include "camundaPlatform.componentFullname" (dict
        "componentName" "core"
        "componentValues" .Values.core
        "context" $
    ) -}}
{{- end -}}

{{/*
[core] Common names.
*/}}
{{- define "core.brokerName" -}}
    {{- if .Values.global.zeebeClusterName -}}
        {{- tpl .Values.global.zeebeClusterName . | trunc 63 | trimSuffix "-" | quote -}}
    {{- else -}}
        {{- printf "%s-broker" .Release.Name | trunc 63 | trimSuffix "-" | quote -}}
    {{- end -}}
{{- end -}}

{{/*
[core] Defines extra labels for core.
*/}}
{{ define "core.extraLabels" -}}
app.kubernetes.io/component: core
app.kubernetes.io/version: {{ include "camundaPlatform.imageTagByParams" (dict "base" .Values.global "overlay" .Values.core) | quote }}
{{- end }}

{{/*
[core] Define common labels for core, combining the match labels and transient labels, which might change on updating
(version depending). These labels shouldn't be used on matchLabels selector, since the selectors are immutable.
*/}}
{{- define "core.labels" -}}
    {{- include "camundaPlatform.labels" . }}
    {{- "\n" }}
    {{- include "core.extraLabels" . }}
{{- end -}}

{{/*
[core] Defines match labels for core, which are extended by sub-charts and should be used in matchLabels selectors.
*/}}
{{- define "core.matchLabels" -}}
    {{- include "camundaPlatform.matchLabels" . }}
app.kubernetes.io/component: core
{{- end -}}

{{/*
[core] Create the name of the service account to use.
*/}}
{{- define "core.serviceAccountName" -}}
    {{- if .Values.core.serviceAccount.enabled -}}
        {{- default (include "core.fullname" .) .Values.core.serviceAccount.name -}}
    {{- else -}}
        {{- default "default" .Values.core.serviceAccount.name -}}
    {{- end -}}
{{- end -}}

{{/*
[core] Get the image pull secrets.
*/}}
{{- define "core.imagePullSecrets" -}}
    {{- include "camundaPlatform.imagePullSecrets" (dict
        "component" "core"
        "context" $
    ) -}}
{{- end }}
