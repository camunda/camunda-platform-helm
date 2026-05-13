{{/* vim: set filetype=mustache: */}}

{{ define "identity.internalUrl" }}
  {{- if .Values.identity.enabled -}}
    {{-
      printf "http://%s:%v%s"
        (include "identity.fullname" .)
        .Values.identity.service.port
        (.Values.identity.contextPath | default "")
    -}}
  {{- end -}}
{{- end -}}

{{- define "identity.externalUrl" -}}
    {{- if .Values.identity.fullURL -}}
        {{ tpl .Values.identity.fullURL $ }}
    {{- else -}}
        {{- if or .Values.global.ingress.enabled .Values.global.gateway.enabled -}}
            {{- $proto := ternary "https" "http" (or .Values.global.ingress.tls.enabled .Values.global.gateway.tls.enabled) -}}
            {{- $host := (tpl .Values.global.host $) -}}
            {{- $path := .Values.identity.contextPath | default "" -}}
            {{- printf "%s://%s%s" $proto $host $path -}}
        {{- else -}}
            {{- "http://localhost:8084" -}}
        {{- end -}}
    {{- end -}}
{{- end -}}

{{- define "identity.extraLabels" -}}
    {{- include "camundaPlatform.componentExtraLabels" (dict "componentName" "identity" "componentValuesKey" "identity" "context" $) -}}
{{- end -}}

{{- define "identity.labels" -}}
    {{- include "camundaPlatform.componentLabels" (dict "componentName" "identity" "componentValuesKey" "identity" "context" $) -}}
{{- end -}}

{{- define "identity.matchLabels" -}}
    {{- include "camundaPlatform.componentMatchLabels" (dict "componentName" "identity" "context" $) -}}
{{- end -}}

{{/*
[identity] Create the name of the service account to use
*/}}
{{- define "identity.serviceAccountName" -}}
    {{- include "camundaPlatform.serviceAccountName" (dict
        "component" "identity"
        "context" $
    ) -}}
{{- end -}}

{{/*
Keycloak helpers
*/}}

{{/*
[identity] Get Keycloak URL protocol from global value.
*/}}
{{- define "identity.keycloak.protocol" -}}
    {{- (.Values.global.identity.keycloak.url).protocol | default "" -}}
{{- end -}}

{{/*
[identity] Get Keycloak URL service name when the chart proxies an external Keycloak via an in-cluster ExternalName service.
*/}}
{{- define "identity.keycloak.service" -}}
    {{- if and (.Values.global.identity.keycloak.url).host .Values.global.identity.keycloak.internal -}}
        {{- printf "%s-keycloak-custom" .Release.Name | trunc 63 -}}
    {{- end -}}
{{- end -}}

{{/*
[identity] Get Keycloak URL host from global value.
*/}}
{{- define "identity.keycloak.host" -}}
    {{- with (.Values.global.identity.keycloak.url).host -}}
        {{- tpl . $ -}}
    {{- end -}}
{{- end -}}


{{/*
[identity] Get Keycloak URL port from global value.
*/}}
{{- define "identity.keycloak.port" -}}
    {{- (.Values.global.identity.keycloak.url).port | default "" -}}
{{- end -}}

{{/*
[identity] Get Keycloak contextPath based on global value.
*/}}
{{- define "identity.keycloak.contextPath" -}}
    {{ .Values.global.identity.keycloak.contextPath | default "/auth/" }}
{{- end -}}


{{/*
[identity] Get port part of a url, return empty string if port is 80 or 443.
*/}}
{{- define "identity.keycloak.portUrl" -}}
  {{- if or (eq (include "identity.keycloak.port" .) "80") (eq (include "identity.keycloak.port" .) "443") -}}
      {{- "" -}}
  {{- else -}}
      {{- printf ":%s" (include "identity.keycloak.port" .) -}}
  {{- end -}}
{{- end -}}

{{/*
[identity] Get multitenancy setting
*/}}
{{- define "identity.multitenancyEnabled" -}}
    {{- if .Values.identity.externalDatabase.enabled }}
        {{- if .Values.identity.multitenancy.enabled -}}
            {{ .Values.identity.multitenancy.enabled }}
        {{- else if .Values.global.multitenancy.enabled -}}
            {{ .Values.global.multitenancy.enabled }}
        {{- else -}}
          false
        {{- end -}}
    {{- else -}}
      false
    {{- end -}}
{{- end -}}

{{/*
[identity] Get Keycloak full URL (protocol, host, port, and contextPath).
*/}}
{{- define "identity.keycloak.url" -}}
    {{-
      printf "%s://%s%s%s"
        (include "identity.keycloak.protocol" .)
        (include "identity.keycloak.host" .)
        (include "identity.keycloak.portUrl" .)
        (include "identity.keycloak.contextPath" .)
    -}}
{{- end -}}

{{/*
[identity] Get Keycloak auth admin user.
*/}}
{{- define "identity.keycloak.authAdminUser" -}}
    {{- .Values.global.identity.keycloak.auth.adminUser | default "" -}}
{{- end -}}

{{/*
[identity] External PostgreSQL helpers.
*/}}

{{- define "identity.postgresql.secretName" -}}
    {{- .Values.identity.externalDatabase.secret.existingSecret | default "" -}}
{{- end -}}

{{- define "identity.postgresql.secretKey" -}}
    {{- .Values.identity.externalDatabase.secret.existingSecretKey | default "password" -}}
{{- end -}}

{{- define "identity.postgresql.host" -}}
    {{- .Values.identity.externalDatabase.host | default "" -}}
{{- end -}}

{{- define "identity.postgresql.port" -}}
    {{- .Values.identity.externalDatabase.port | default "5432" -}}
{{- end -}}

{{- define "identity.postgresql.username" -}}
    {{- .Values.identity.externalDatabase.username | default "" -}}
{{- end -}}

{{- define "identity.postgresql.database" -}}
    {{- .Values.identity.externalDatabase.database | default "" -}}
{{- end -}}

{{/*
[identity] Get the image pull secrets.
*/}}
{{- define "identity.imagePullSecrets" -}}
    {{- include "camundaPlatform.subChartImagePullSecrets" (dict "Values" (set (deepCopy .Values) "image" .Values.identity.image)) }}
{{- end }}
