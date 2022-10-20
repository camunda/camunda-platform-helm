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

{{/*
Keycloak helpers
*/}}

{{/*
[identity] Fail in case Keycloak chart is disabled and external Keycloak URL is not configured.
*/}}
{{- define "identity.keycloakIsConfigured" -}}
{{- $failMessage := `
[identity] Keycloak chart is not enabled, and the external Keycloak URL is not configured.
If you want to use your own Keycloak, please make sure to configure the following var:
  - global.identity.keycloak.url
` -}}

{{- if not .Values.keycloak.enabled }}
{{- if not .Values.global.identity.keycloak.url }}
    {{ printf "\n%s" $failMessage | trimSuffix "\n" | fail }}
{{- end }}
{{- end }}
{{- end -}}

{{/*
[identity] Get Keycloak name based on the Keycloak subchart.
*/}}
{{- define "identity.keycloakName" -}}
{{- if .Values.keycloak.enabled -}}
{{ include "keycloak.fullname" .Subcharts.keycloak }}
{{- end -}}
{{- end -}}

{{/*
[identity] Get Keycloak URL based on global value or Keycloak subchart.
*/}}
{{- define "identity.keycloakURL" -}}
{{ include "identity.keycloakIsConfigured" . }}
{{- if (.Values.global.identity.keycloak.url) -}}
    {{- .Values.global.identity.keycloak.url -}}
{{- else -}}
    {{- $keycloakChart := index $ "Subcharts" "keycloak" -}}
    {{- $keycloakProtocol := ternary "https" "http" ($keycloakChart.Values.auth.tls.enabled) -}}
    {{- $keycloakPort := get $keycloakChart.Values.service.ports $keycloakProtocol -}}
    {{- $keycloakProtocol -}}://{{- include "identity.keycloakName" . -}}:{{- $keycloakPort -}}
{{- end -}}
{{- end -}}
