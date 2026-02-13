{{/* vim: set filetype=mustache: */}}

{{/*
Get the default app name.
*/}}
{{- define "webModeler.name" -}}
web-modeler
{{- end -}}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "webModeler.fullname" -}}
  {{- include "camundaPlatform.componentFullname" (dict
      "componentName" "web-modeler"
      "componentValues" .Values.webModeler
      "context" $
  ) -}}
{{- end -}}

{{/*
Create a fully qualified name for the restapi objects.
*/}}
{{- define "webModeler.restapi.fullname" -}}
  {{- (include "webModeler.fullname" .) | trunc 55 | trimSuffix "-" -}}-restapi
{{- end -}}

{{/*
Create a fully qualified name for the webapp objects.
*/}}
{{- define "webModeler.webapp.fullname" -}}
  {{- (include "webModeler.fullname" .) | trunc 56 | trimSuffix "-" -}}-webapp
{{- end -}}

{{/*
Create a fully qualified name for the websockets objects.
*/}}
{{- define "webModeler.websockets.fullname" -}}
  {{- (include "webModeler.fullname" .) | trunc 52 | trimSuffix "-" -}}-websockets
{{- end -}}

{{- define "webModeler.extraLabels" -}}
    {{- include "camundaPlatform.componentExtraLabels" (dict "componentName" "web-modeler" "componentValuesKey" "webModeler" "context" $) -}}
{{- end -}}

{{/*
Define common labels for all Web Modeler components.
*/}}
{{- define "webModeler.commonLabels" -}}
{{- $values := merge (deepCopy .Values) (dict "nameOverride" (include "webModeler.name" .) "image" .Values.webModeler.image) }}
{{- template "camundaPlatform.labels" (dict "Chart" .Chart "Release" .Release "Values" $values) }}
{{- end -}}

{{/*
Define common match labels for all Web Modeler components.
*/}}
{{- define "webModeler.commonMatchLabels" -}}
{{- $values := set (deepCopy .Values) "nameOverride" (include "webModeler.name" .) }}
{{- template "camundaPlatform.matchLabels" (dict "Chart" .Chart "Release" .Release "Values" $values) }}
{{- end -}}

{{- define "webModeler.labels" -}}
{{ template "webModeler.commonLabels" . }}
{{ template "webModeler.extraLabels" . }}
{{- end -}}

{{/*
[web-modeler] Defines labels for a sub-component, combining common labels and the sub-component name.
*/}}
{{- define "webModeler.subComponentLabels" -}}
{{ template "webModeler.commonLabels" .context }}
app.kubernetes.io/component: {{ .componentName }}
{{- end -}}

{{- define "webModeler.restapi.labels" -}}
    {{- include "webModeler.subComponentLabels" (dict "componentName" "restapi" "context" $) -}}
{{- end -}}

{{- define "webModeler.webapp.labels" -}}
    {{- include "webModeler.subComponentLabels" (dict "componentName" "webapp" "context" $) -}}
{{- end -}}

{{- define "webModeler.websockets.labels" -}}
    {{- include "webModeler.subComponentLabels" (dict "componentName" "websockets" "context" $) -}}
{{- end -}}

{{/*
[web-modeler] Defines match labels for a sub-component, combining common match labels and the sub-component name.
*/}}
{{- define "webModeler.subComponentMatchLabels" -}}
{{ template "webModeler.commonMatchLabels" .context }}
app.kubernetes.io/component: {{ .componentName }}
{{- end -}}

{{- define "webModeler.restapi.matchLabels" -}}
    {{- include "webModeler.subComponentMatchLabels" (dict "componentName" "restapi" "context" $) -}}
{{- end -}}

{{- define "webModeler.webapp.matchLabels" -}}
    {{- include "webModeler.subComponentMatchLabels" (dict "componentName" "webapp" "context" $) -}}
{{- end -}}

{{- define "webModeler.websockets.matchLabels" -}}
    {{- include "webModeler.subComponentMatchLabels" (dict "componentName" "websockets" "context" $) -}}
{{- end -}}

{{/*
[web-modeler] Get the image pull secrets.
*/}}
{{- define "webModeler.imagePullSecrets" -}}
  {{- include "camundaPlatform.subChartImagePullSecrets" (dict "Values" (set (deepCopy .Values) "image" .Values.webModeler.image)) }}
{{- end }}

{{/*
[web-modeler] Get the full name (<registry>/<repository>:<tag>) of the restapi Docker image
*/}}
{{- define "webModeler.restapi.image" -}}
  {{- include "camundaPlatform.imageByParams" (dict "base" .Values.global "overlay" (dict "image" (deepCopy .Values.webModeler.image | merge .Values.webModeler.restapi.image))) }}
{{- end }}

{{/*
[web-modeler] Get the full name (<registry>/<repository>:<tag>) of the webapp Docker image
*/}}
{{- define "webModeler.webapp.image" -}}
  {{- include "camundaPlatform.imageByParams" (dict "base" .Values.global "overlay" (dict "image" (deepCopy .Values.webModeler.image | merge .Values.webModeler.webapp.image))) }}
{{- end }}

{{/*
[web-modeler] Get the full name (<registry>/<repository>:<tag>) of the websockets Docker image
*/}}
{{- define "webModeler.websockets.image" -}}
  {{- include "camundaPlatform.imageByParams" (dict "base" .Values.global "overlay" (dict "image" (deepCopy .Values.webModeler.image | merge .Values.webModeler.websockets.image))) }}
{{- end }}

{{/*
[web-modeler] Create the name of the service account to use
*/}}
{{- define "webModeler.serviceAccountName" -}}
    {{- include "camundaPlatform.serviceAccountName" (dict
        "component" "webModeler"
        "context" $
    ) -}}
{{- end -}}

{{/*
[web-modeler] Get the database JDBC url, depending on whether the postgresql dependency chart is enabled.
*/}}
{{- define "webModeler.restapi.databaseUrl" -}}
  {{- if .Values.webModelerPostgresql.enabled -}}
    {{- printf "jdbc:postgresql://%s:5432/%s"
        (include "webModeler.postgresql.fullname" .)
        (.Values.webModelerPostgresql.auth.database)
      -}}
  {{- else if .Values.webModeler.restapi.externalDatabase.url -}}
    {{- .Values.webModeler.restapi.externalDatabase.url -}}
  {{- else if .Values.webModeler.restapi.externalDatabase.host -}}
    {{- printf "jdbc:postgresql://%s:%s/%s"
        .Values.webModeler.restapi.externalDatabase.host
        (toString (.Values.webModeler.restapi.externalDatabase.port))
        (.Values.webModeler.restapi.externalDatabase.database)
      -}}
  {{- end -}}
{{- end -}}

{{/*
[web-modeler] Get the database user, depending on whether the postgresql dependency chart is enabled.
*/}}
{{- define "webModeler.restapi.databaseUser" -}}
  {{- if .Values.webModelerPostgresql.enabled -}}
    {{- .Values.webModelerPostgresql.auth.username -}}
  {{- else -}}
    {{- .Values.webModeler.restapi.externalDatabase.username | default .Values.webModeler.restapi.externalDatabase.user -}}
  {{- end -}}
{{- end -}}

{{/*
[web-modeler] Check if username and password is provided for the SMTP server
*/}}
{{- define "webModeler.restapi.mail.authEnabled" -}}
  {{- $authEnabled := false -}}
  {{- if and (typeIs "string" .Values.webModeler.restapi.mail.smtpUser) (ne .Values.webModeler.restapi.mail.smtpUser "") }}
    {{- if or (and (typeIs "string" .Values.webModeler.restapi.mail.smtpPassword) (ne .Values.webModeler.restapi.mail.smtpPassword "")) .Values.webModeler.restapi.mail.existingSecret }}
      {{- $authEnabled = true }}
    {{- end }}
  {{- end }}
  {{- $authEnabled -}}
{{- end -}}

{{/*
[web-modeler] Get the full name of the Kubernetes objects from the postgresql dependency chart
*/}}
{{- define "webModeler.postgresql.fullname" -}}
  {{- include "common.names.dependency.fullname" (dict "chartName" "webModelerPostgresql" "chartValues" .Values.webModelerPostgresql "context" $) -}}
{{- end -}}

{{/*
[web-modeler] Create the context path for the WebSocket app (= configured context path for the webapp + suffix "-ws").
*/}}
{{- define "webModeler.websocketContextPath" -}}
  {{- .Values.webModeler.contextPath }}-ws
{{- end -}}

{{/*
[web-modeler] Get the host name on which the WebSocket server is reachable from the client.
*/}}
{{- define "webModeler.publicWebsocketHost" -}}
  {{- if and .Values.global.ingress.enabled .Values.webModeler.contextPath }}
    {{- tpl .Values.global.host $ | default (tpl .Values.global.ingress.host $) }}
  {{- else -}}
    {{- .Values.webModeler.websockets.publicHost }}
  {{- end }}
{{- end -}}

{{/*
[web-modeler] Get the port number on which the WebSocket server is reachable from the client.
*/}}
{{- define "webModeler.publicWebsocketPort" -}}
  {{- if and .Values.global.ingress.enabled .Values.webModeler.contextPath }}
    {{- .Values.global.ingress.tls.enabled | ternary "443" "80" }}
  {{- else }}
    {{- .Values.webModeler.websockets.publicPort }}
  {{- end }}
{{- end -}}

{{/*
[web-modeler] Check if TLS must be enabled for WebSocket connections from the client.
*/}}
{{- define "webModeler.websocketTlsEnabled" -}}
  {{- if and .Values.global.ingress.enabled .Values.webModeler.contextPath }}
    {{- .Values.global.ingress.tls.enabled }}
  {{- else -}}
    false
  {{- end }}
{{- end -}}

{{/*
[web-modeler] Define variables related to authentication.
*/}}
{{- define "webModeler.authClientId" -}}
  {{- .Values.global.identity.auth.webModeler.clientId | default "web-modeler" -}}
{{- end -}}

{{- define "webModeler.authClientApiAudience" -}}
  {{- .Values.global.identity.auth.webModeler.clientApiAudience | default "web-modeler-api" -}}
{{- end -}}

{{- define "webModeler.authPublicApiAudience" -}}
  {{- .Values.global.identity.auth.webModeler.publicApiAudience | default "web-modeler-public-api" -}}
{{- end -}}

{{- define "webModeler.authMethod" -}}
    {{- .Values.webModeler.security.authentication.method | default (
        .Values.global.security.authentication.method | default "none"
    ) -}}
{{- end -}}

{{- define "webModeler.authConfigValue" -}}
  {{- if eq (include "webModeler.authMethod" .) "oidc" -}}
    BEARER_TOKEN
  {{- else if eq (include "webModeler.authMethod" .) "basic" -}}
    BASIC
  {{- else -}}
    NONE
  {{- end -}}
{{- end -}}
