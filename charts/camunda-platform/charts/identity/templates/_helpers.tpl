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
[identity] Fail in case Keycloak chart is disabled and existing Keycloak URL is not configured.
*/}}
{{- define "identity.keycloak.isConfigured" -}}
{{- $failMessage := `
[identity] Keycloak chart is not enabled, and the existing Keycloak URL is not configured.
  - If you want to deploy Keycloak chart then set "keycloak.enabled: true".
  - If you want to existing Keycloak, then set the vars under "global.identity.keycloak".
For more details please check the documentation.
` -}}

{{- if not .Values.keycloak.enabled }}
{{- if not .Values.global.identity.keycloak.url}}
    {{ printf "\n%s" $failMessage | trimSuffix "\n" | fail }}
{{- end }}
{{- end }}
{{- end -}}

{{/*
[identity] Get Keycloak URL based on global value or Keycloak subchart.
*/}}
{{- define "identity.keycloak.url" -}}
{{ include "identity.keycloak.isConfigured" . }}
{{- if (.Values.global.identity.keycloak.url) -}}
    {{- .Values.global.identity.keycloak.url -}}
{{- else -}}
    {{- $keycloakChart := index $ "Subcharts" "keycloak" -}}
    {{- $keycloakProtocol := ternary "https" "http" ($keycloakChart.Values.auth.tls.enabled) -}}
    {{- $keycloakPort := get $keycloakChart.Values.service.ports $keycloakProtocol -}}
    {{- $keycloakProtocol -}}://{{- include "keycloak.fullname" .Subcharts.keycloak -}}:{{- $keycloakPort -}}
{{- end -}}
{{- end -}}

{{/*
[identity] Get Keycloak auth admin user. For more details:
*/}}
{{- define "identity.keycloak.authAdminUser" -}}
{{- if .Values.global.identity.keycloak.auth.adminUser }}
    {{- .Values.global.identity.keycloak.auth.adminUser -}}
{{- else }}
    {{- .Values.keycloak.auth.adminUser -}}
{{- end }}
{{- end -}}

{{/*
[identity] Get name of Keycloak auth existing secret. For more details:
https://docs.bitnami.com/kubernetes/apps/keycloak/configuration/manage-passwords/
*/}}
{{- define "identity.keycloak.authExistingSecret" -}}
{{- if .Values.global.identity.keycloak.auth.existingSecret }}
    {{- /*
        Unlike the upstream Keycloak chart, in the global vars, we only support the "string" format,
        not the "dict" format. i.e., it should refer to the actual existing secret name.
    */ -}}
    {{- .Values.global.identity.keycloak.auth.existingSecret -}}
{{- else if and .Values.keycloak.auth.existingSecret (not (typeIs "string" .Values.keycloak.auth.existingSecret)) }}
    {{- /*
        Helper: https://github.com/bitnami/charts/blob/master/bitnami/common/templates/_secrets.tpl
        Usage in keycloak secrets https://github.com/bitnami/charts/blob/master/bitnami/keycloak/templates/secrets.yaml
        and in statefulset https://github.com/bitnami/charts/blob/master/bitnami/keycloak/templates/statefulset.yaml
    */ -}}
    {{ include "common.secrets.name" (dict "existingSecret" .Values.keycloak.auth.existingSecret "context" $) }}
{{- else }}
    {{- /*
      https://github.com/bitnami/charts/blob/master/bitnami/common/templates/_names.tpl
    */ -}}
    {{- include "common.names.dependency.fullname" (dict "chartName" "keycloak" "chartValues" .Values.keycloak "context" $) }}
{{- end }}
{{- end -}}
