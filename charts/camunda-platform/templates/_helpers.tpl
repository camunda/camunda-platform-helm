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
(version depending). These labels shouldn't be used on matchLabels selector, since the selectors are immutable.
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
Set image according the values of base (global) or overlay (subchart) values.
If the "overlay" values exist, they will override the "base" values, otherwise the "base" values will be used.
Usage:
  This template has 2 syntaxes:
  1. Top-level values: It relies on ".Values.global.image" (base) and ".Values.image" (overlay) dicts.
     Example: {{ include "camundaPlatform.image" . }}
  2. Parameterized values: It relies on ".base.image" and ".overlay.image" dicts.
     Example: {{ include "camundaPlatform.image" (dict "base" .Values.global "overlay" .Values.retentionPolicy) }}
*/}}
{{- define "camundaPlatform.image" -}}
    {{/* Allow the template to work with top-level and parameterized values. */}}
    {{- $baseImage := (hasKey . "base" | ternary (.base).image (.Values).global.image) -}}
    {{- $overlayImage := (hasKey . "overlay" | ternary (.overlay).image (.Values).image) -}}

    {{- $imageRegistry := $overlayImage.registry | default $baseImage.registry -}}
    {{- printf "%s%s%s:%s"
        $imageRegistry
        (empty $imageRegistry | ternary "" "/")
        ($overlayImage.repository | default $baseImage.repository)
        ($overlayImage.tag | default $baseImage.tag)
    -}}
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
Keycloak service name should be a max of 20 char since the Keycloak Bitnami Chart is using Wildfly, the node identifier in WildFly is limited to 23 characters.
Furthermore, this allows changing the referenced Keycloak name inside the sub-charts.
Subcharts can't access values from other sub-charts or the parent, global only. This is the reason why we have a global value to specify the Keycloak full name.
*/}}

{{- define "camundaPlatform.issuerBackendUrl" -}}
    {{- $keycloakRealmPath := "/realms/camunda-platform" -}}
    {{- if .Values.global.identity.keycloak.url -}}
        {{- include "identity.keycloak.url" . -}}{{- $keycloakRealmPath -}}
    {{- else -}}
        http://{{ include "common.names.dependency.fullname" (dict "chartName" "keycloak" "chartValues" . "context" $) | trunc 20 | trimSuffix "-" }}:80{{- include "identity.keycloak.contextPath" . -}}{{ $keycloakRealmPath }}
    {{- end -}}
{{- end -}}
