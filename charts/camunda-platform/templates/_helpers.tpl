{{/* vim: set filetype=mustache: */}}
{{/*
Expand the name of the chart.
*/}}
{{- define "camundaPlatform.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (for example,
by the DNS naming spec). If release name contains chart name it will be used as a full name.
*/}}
{{- define "camundaPlatform.fullname" -}}
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
Define common labels, combining the match labels and transient labels, which might change on updating
(version depending). These labels should not be used on matchLabels selector, since the selectors are immutable.
*/}}
{{- define "camundaPlatform.labels" -}}
{{- template "camundaPlatform.matchLabels" . }}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version | replace "+" "_" }}
{{- if .Values.image }}
    {{- if .Values.image.tag }}
app.kubernetes.io/version: {{ .Values.image.tag | quote }}
    {{- else }}
app.kubernetes.io/version: {{ .Values.global.image.tag | quote }}
    {{- end }}
{{- else }}
app.kubernetes.io/version: {{ .Values.global.image.tag | quote }}
{{- end }}
{{- end -}}

{{/*
Common match labels, which are extended by sub-charts and should be used in matchLabels selectors.
*/}}
{{- define "camundaPlatform.matchLabels" -}}
{{- if .Values.global.labels -}}
{{ toYaml .Values.global.labels }}
{{- end }}
app.kubernetes.io/name: {{ template "camundaPlatform.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: camunda-platform
{{- end -}}

{{/*
Set image according the values of "base" or "overlay" values.
If the "overlay" values exist, they will override the "base" values, otherwise the "base" values will be used.
Usage: {{ include "camundaPlatform.imageByParams" (dict "base" .Values.global "overlay" .Values.retentionPolicy) }}
*/}}
{{- define "camundaPlatform.imageByParams" -}}
    {{- $imageRegistry := .overlay.image.registry | default .base.image.registry -}}
    {{- printf "%s%s%s:%s"
        $imageRegistry
        (empty $imageRegistry | ternary "" "/")
        (.overlay.image.repository | default .base.image.repository)
        (.overlay.image.tag | default .base.image.tag)
    -}}
{{- end -}}

{{/*
Set image according the values of "global" or "subchart" values.
Usage: {{ include "camundaPlatform.image" . }}
*/}}
{{- define "camundaPlatform.image" -}}
    {{ include "camundaPlatform.imageByParams" (dict "base" .Values.global "overlay" .Values) }}
{{- end -}}

{{/*
Set imagePullSecrets according the values of global, subchart, or empty.
*/}}
{{- define "camundaPlatform.imagePullSecrets" -}}
{{- if (.Values.image.pullSecrets) -}}
{{ .Values.image.pullSecrets | toYaml }}
{{- else if (.Values.global.image.pullSecrets) -}}
{{ .Values.global.image.pullSecrets | toYaml }}
{{- else -}}
[]
{{- end -}}
{{- end -}}

{{/*
[camunda-platform] Keycloak default URL.
*/}}

{{- define "camundaPlatform.keycloakDefaultHost" -}}
    {{- $keycloakDefaultHost := include "common.names.dependency.fullname" (dict "chartName" "keycloak" "chartValues" . "context" $) -}}
    {{- $keycloakDefaultHost -}}
{{- end -}}

{{/*
[camunda-platform] Keycloak URL which could be external.
*/}}

{{- define "camundaPlatform.keycloakURLBase" -}}
    http://{{- include "camundaPlatform.keycloakDefaultHost" . -}}:80
{{- end -}}

{{- define "camundaPlatform.keycloakURL" -}}
    {{- if .Values.global.identity.keycloak.url -}}
        {{- include "identity.keycloak.url" . -}}
    {{- else -}}
        {{- include "camundaPlatform.keycloakURLBase" . -}}{{- include "identity.keycloak.contextPath" . -}}
    {{- end -}}
{{- end -}}

{{/*
[camunda-platform] Keycloak issuer backend URL which used internally for Camunda apps.
*/}}

{{- define "camundaPlatform.issuerBackendUrl" -}}
    {{- include "camundaPlatform.keycloakURL" . -}}{{- .Values.global.identity.keycloak.realm -}}
{{- end -}}

{{/*
[camunda-platform] Elasticsearch URL which could be external.
*/}}

{{- define "camundaPlatform.elasticsearchURL" -}}
    {{- if .Values.global.elasticsearch.url -}}
        {{- .Values.global.elasticsearch.url -}}
    {{- else -}}
        {{ .Values.global.elasticsearch.protocol }}://{{ .Values.global.elasticsearch.host }}:{{ .Values.global.elasticsearch.port }}
    {{- end -}}
{{- end -}}

{{ define "camundaPlatform.operateURL" }}
  {{- if .Values.operate.enabled -}}
    {{- print "http://" -}}{{- include "operate.fullname" .Subcharts.operate -}}:{{- .Values.operate.service.port -}}
    {{- .Values.operate.contextPath -}}
  {{- end -}}
{{- end -}}
