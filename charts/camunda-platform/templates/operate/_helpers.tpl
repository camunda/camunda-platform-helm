{{/* vim: set filetype=mustache: */}}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}

{{- define "operate.fullname" -}}
    {{- include "camundaPlatform.componentFullname" (dict
        "componentName" "operate"
        "componentValues" .Values.operate
        "context" $
    ) -}}
{{- end -}}

{{/*
Defines extra labels for operate.
*/}}
{{ define "operate.extraLabels" -}}
app.kubernetes.io/component: operate
{{- end }}

{{/*
Define common labels for operate, combining the match labels and transient labels, which might change on updating
(version depending). These labels shouldn't be used on matchLabels selector, since the selectors are immutable.
*/}}
{{- define "operate.labels" -}}
    {{- include "camundaPlatform.labels" . }}
    {{- "\n" }}
    {{- include "operate.extraLabels" . }}
{{- end -}}

{{/*
Defines match labels for operate, which are extended by sub-charts and should be used in matchLabels selectors.
*/}}
{{- define "operate.matchLabels" -}}
    {{- include "camundaPlatform.matchLabels" . }}
    {{- "\n" }}
    {{- include "operate.extraLabels" . }}
{{- end -}}

{{/*
[operate] Create the name of the service account to use
*/}}
{{- define "operate.serviceAccountName" -}}
    {{- if .Values.operate.serviceAccount.enabled }}
        {{- default (include "operate.fullname" .) .Values.operate.serviceAccount.name }}
    {{- else }}
        {{- default "default" .Values.operate.serviceAccount.name }}
    {{- end }}
{{- end }}

{{/*
[operate] Get the image pull secrets.
*/}}
{{- define "operate.imagePullSecrets" -}}
    {{- include "camundaPlatform.imagePullSecrets" (dict
        "component" "operate"
        "context" $
    ) -}}
{{- end }}
