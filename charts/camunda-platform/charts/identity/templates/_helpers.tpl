{{/* vim: set filetype=mustache: */}}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "identity.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{/*
Defines extra labels for identity.
*/}}
{{- define "identity.extraLabels" -}}
app.kubernetes.io/component: identity
{{- end -}}

{{/*
Define common labels for identity, combining the match labels and transient labels, which might change on updating
(version depending). These labels shouldn't be used on matchLabels selector, since the selectors are immutable.
*/}}
{{- define "identity.labels" -}}
{{- template "camundaPlatform.labels" . }}
{{ template "identity.extraLabels" . }}
{{- end -}}

{{/*
Defines match labels for identity, which are extended by sub-charts and should be used in matchLabels selectors.
*/}}
{{- define "identity.matchLabels" -}}
{{- template "camundaPlatform.matchLabels" . }}
{{ template "identity.extraLabels" . }}
{{- end -}}

{{/*
[identity] Create the name of the service account to use
*/}}
{{- define "identity.serviceAccountName" -}}
{{- if .Values.serviceAccount.enabled }}
{{- default (include "identity.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
[identity] Create the name of the operate-identity secret
*/}}
{{- define "identity.secretNameOperateIdentity" -}}
{{- $name := .Release.Name -}}
{{- printf "%s-operate-identity-secret" $name | trunc 63 | trimSuffix "-" | quote -}}
{{- end }}

{{/*
[identity] Create the name of the tasklist-identity secret
*/}}
{{- define "identity.secretNameTasklistIdentity" -}}
{{- $name := .Release.Name -}}
{{- printf "%s-tasklist-identity-secret" $name | trunc 63 | trimSuffix "-" | quote -}}
{{- end }}

{{/*
[identity] Create the name of the optimize-identity secret
*/}}
{{- define "identity.secretNameOptimizeIdentity" -}}
{{- $name := .Release.Name -}}
{{- printf "%s-optimize-identity-secret" $name | trunc 63 | trimSuffix "-" | quote -}}
{{- end }}