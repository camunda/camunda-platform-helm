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
      "componentValues" (or .Values.camundaHub.webModeler .Values.webModeler)
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
{{- $values := merge (deepCopy .Values) (dict "nameOverride" (include "webModeler.name" .) "image" (or .Values.camundaHub.webModeler.image .Values.webModeler.image)) }}
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

{{- define "webModeler.websockets.matchLabels" -}}
    {{- include "webModeler.subComponentMatchLabels" (dict "componentName" "websockets" "context" $) -}}
{{- end -}}

{{/*
[web-modeler] Get the image pull secrets.
*/}}
{{- define "webModeler.imagePullSecrets" -}}
  {{- include "camundaPlatform.subChartImagePullSecrets" (dict "Values" (set (deepCopy .Values) "image" (or .Values.camundaHub.webModeler.image .Values.webModeler.image))) }}
{{- end }}

{{/*
[web-modeler] Get the full name (<registry>/<repository>:<tag>) of the restapi Docker image
*/}}
{{- define "webModeler.restapi.image" -}}
  {{- include "camundaPlatform.imageByParams" (dict "base" .Values.global "overlay" (dict "image" (deepCopy (or .Values.camundaHub.webModeler.image .Values.webModeler.image) | merge (dig "webModeler" "restapi" "image" .Values.webModeler.restapi.image .Values.camundaHub)))) }}
{{- end }}

{{/*
[web-modeler] Get the full name (<registry>/<repository>:<tag>) of the websockets Docker image
*/}}
{{- define "webModeler.websockets.image" -}}
  {{- include "camundaPlatform.imageByParams" (dict "base" .Values.global "overlay" (dict "image" (deepCopy (or .Values.camundaHub.webModeler.image .Values.webModeler.image) | merge (dig "webModeler" "websockets" "image" .Values.webModeler.websockets.image .Values.camundaHub)))) }}
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
  {{- else if (dig "webModeler" "restapi" "externalDatabase" "url" .Values.webModeler.restapi.externalDatabase.url .Values.camundaHub) -}}
    {{- (dig "webModeler" "restapi" "externalDatabase" "url" .Values.webModeler.restapi.externalDatabase.url .Values.camundaHub) -}}
  {{- else if (dig "webModeler" "restapi" "externalDatabase" "host" .Values.webModeler.restapi.externalDatabase.host .Values.camundaHub) -}}
    {{- printf "jdbc:postgresql://%s:%s/%s"
        (dig "webModeler" "restapi" "externalDatabase" "host" .Values.webModeler.restapi.externalDatabase.host .Values.camundaHub)
        (toString ((dig "webModeler" "restapi" "externalDatabase" "port" .Values.webModeler.restapi.externalDatabase.port .Values.camundaHub)))
        ((dig "webModeler" "restapi" "externalDatabase" "database" .Values.webModeler.restapi.externalDatabase.database .Values.camundaHub))
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
    {{- (dig "webModeler" "restapi" "externalDatabase" "username" .Values.webModeler.restapi.externalDatabase.username .Values.camundaHub) -}}
  {{- end -}}
{{- end -}}

{{/*
[web-modeler] Check if username and password is provided for the SMTP server
*/}}
{{- define "webModeler.restapi.mail.authEnabled" -}}
  {{- $authEnabled := false -}}
  {{- if and (typeIs "string" (dig "webModeler" "restapi" "mail" "smtpUser" .Values.webModeler.restapi.mail.smtpUser .Values.camundaHub)) (ne (dig "webModeler" "restapi" "mail" "smtpUser" .Values.webModeler.restapi.mail.smtpUser .Values.camundaHub) "") }}
    {{- if or (and (typeIs "string" (dig "webModeler" "restapi" "mail" "smtpPassword" .Values.webModeler.restapi.mail.smtpPassword .Values.camundaHub)) (ne (dig "webModeler" "restapi" "mail" "smtpPassword" .Values.webModeler.restapi.mail.smtpPassword .Values.camundaHub) "")) (dig "webModeler" "restapi" "mail" "existingSecret" .Values.webModeler.restapi.mail.existingSecret .Values.camundaHub) }}
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
[web-modeler] Create the context path for the WebSocket app (= configured context path + suffix "-ws").
*/}}
{{- define "webModeler.websocketContextPath" -}}
  {{- (or .Values.camundaHub.webModeler.contextPath .Values.webModeler.contextPath) }}-ws
{{- end -}}

{{/*
[web-modeler] Get the host name on which the WebSocket server is reachable from the client.
*/}}
{{- define "webModeler.publicWebsocketHost" -}}
  {{- if and .Values.global.ingress.enabled (or .Values.camundaHub.webModeler.contextPath .Values.webModeler.contextPath) }}
    {{- tpl .Values.global.host $ }}
  {{- else -}}
    {{- (dig "webModeler" "websockets" "publicHost" .Values.webModeler.websockets.publicHost .Values.camundaHub) }}
  {{- end }}
{{- end -}}

{{/*
[web-modeler] Get the port number on which the WebSocket server is reachable from the client.
*/}}
{{- define "webModeler.publicWebsocketPort" -}}
  {{- if and .Values.global.ingress.enabled (or .Values.camundaHub.webModeler.contextPath .Values.webModeler.contextPath) }}
    {{- .Values.global.ingress.tls.enabled | ternary "443" "80" }}
  {{- else }}
    {{- (dig "webModeler" "websockets" "publicPort" .Values.webModeler.websockets.publicPort .Values.camundaHub) }}
  {{- end }}
{{- end -}}

{{/*
[web-modeler] Check if TLS must be enabled for WebSocket connections from the client.
*/}}
{{- define "webModeler.websocketTlsEnabled" -}}
  {{- if and .Values.global.ingress.enabled (or .Values.camundaHub.webModeler.contextPath .Values.webModeler.contextPath) }}
    {{- .Values.global.ingress.tls.enabled }}
  {{- else -}}
    false
  {{- end }}
{{- end -}}

{{/*
[web-modeler] Define variables related to authentication.
*/}}
{{- define "webModeler.authClientId" -}}
  {{- (or .Values.global.identity.auth.camundaHub.webModeler.clientId .Values.global.identity.auth.webModeler.clientId) | default "web-modeler" -}}
{{- end -}}

{{- define "webModeler.authClientApiAudience" -}}
  {{- (or .Values.global.identity.auth.camundaHub.webModeler.clientApiAudience .Values.global.identity.auth.webModeler.clientApiAudience) | default "web-modeler-api" -}}
{{- end -}}

{{- define "webModeler.authPublicApiAudience" -}}
  {{- (or .Values.global.identity.auth.camundaHub.webModeler.publicApiAudience .Values.global.identity.auth.webModeler.publicApiAudience) | default "web-modeler-public-api" -}}
{{- end -}}

{{- define "webModeler.authMethod" -}}
    {{- (dig "webModeler" "security" "authentication" "method" .Values.webModeler.security.authentication.method .Values.camundaHub) | default (
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
