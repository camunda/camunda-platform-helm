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
  {{/* mustMergeOverwrite is used instead of or because camundaHub has intermediate
       sub-maps that make it truthy even when no overrides are set. Deep-merging empty maps
       is a no-op, preserving all legacy values. */}}
  {{- include "camundaPlatform.componentFullname" (dict
      "componentName" "web-modeler"
      "componentValues" (mustMergeOverwrite (deepCopy .Values.webModeler) .Values.camundaHub)
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
app.kubernetes.io/component: web-modeler
app.kubernetes.io/version: {{ include "camundaPlatform.versionLabel" (dict "base" .Values.global "overlay" (mustMergeOverwrite (deepCopy .Values.webModeler) .Values.camundaHub) "chart" .Chart) | quote }}
{{- end -}}

{{/*
Define common labels for all Web Modeler components.
*/}}
{{- define "webModeler.commonLabels" -}}
{{- $values := merge (deepCopy .Values) (dict "nameOverride" (include "webModeler.name" .) "image" (or .Values.camundaHub.image .Values.webModeler.image)) }}
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
  {{- include "camundaPlatform.componentImagePullSecrets" (dict "Values" (set (deepCopy .Values) "image" (or .Values.camundaHub.image .Values.webModeler.image))) }}
{{- end }}

{{/*
[web-modeler] Get the full name (<registry>/<repository>:<tag>) of the restapi Docker image
*/}}
{{- define "webModeler.restapi.image" -}}
  {{- include "camundaPlatform.imageByParams" (dict "base" .Values.global "overlay" (dict "image" (deepCopy (or .Values.camundaHub.image .Values.webModeler.image) | merge (or .Values.camundaHub.restapi.image .Values.webModeler.restapi.image)))) }}
{{- end }}

{{/*
[web-modeler] Get the full name (<registry>/<repository>:<tag>) of the websockets Docker image
*/}}
{{- define "webModeler.websockets.image" -}}
  {{- include "camundaPlatform.imageByParams" (dict "base" .Values.global "overlay" (dict "image" (deepCopy (or .Values.camundaHub.image .Values.webModeler.image) | merge (or .Values.camundaHub.websockets.image .Values.webModeler.websockets.image)))) }}
{{- end }}

{{/*
[web-modeler] Create the name of the service account to use
*/}}
{{- define "webModeler.serviceAccountName" -}}
    {{- $saName := (or .Values.camundaHub.serviceAccount.name .Values.webModeler.serviceAccount.name) -}}
    {{- if (or .Values.camundaHub.serviceAccount.enabled .Values.webModeler.serviceAccount.enabled) -}}
        {{- $saName | default (include "webModeler.fullname" .) -}}
    {{- else -}}
        {{- $saName | default "default" -}}
    {{- end -}}
{{- end -}}

{{/*
[web-modeler] Get the database JDBC url for the external PostgreSQL.
*/}}
{{- define "webModeler.restapi.databaseUrl" -}}
  {{- if (or .Values.camundaHub.restapi.externalDatabase.url .Values.webModeler.restapi.externalDatabase.url) -}}
    {{- (or .Values.camundaHub.restapi.externalDatabase.url .Values.webModeler.restapi.externalDatabase.url) -}}
  {{- else if (or .Values.camundaHub.restapi.externalDatabase.host .Values.webModeler.restapi.externalDatabase.host) -}}
    {{- printf "jdbc:postgresql://%s:%s/%s"
        (or .Values.camundaHub.restapi.externalDatabase.host .Values.webModeler.restapi.externalDatabase.host)
        (toString (or .Values.camundaHub.restapi.externalDatabase.port .Values.webModeler.restapi.externalDatabase.port))
        (or .Values.camundaHub.restapi.externalDatabase.database .Values.webModeler.restapi.externalDatabase.database)
      -}}
  {{- end -}}
{{- end -}}

{{/*
[web-modeler] Get the database user.
*/}}
{{- define "webModeler.restapi.databaseUser" -}}
  {{- (or .Values.camundaHub.restapi.externalDatabase.username .Values.webModeler.restapi.externalDatabase.username) -}}
{{- end -}}

{{/*
[web-modeler] Check if username and password is provided for the SMTP server
*/}}
{{- define "webModeler.restapi.mail.authEnabled" -}}
  {{- $authEnabled := false -}}
  {{- if and (typeIs "string" (or .Values.camundaHub.restapi.mail.smtpUser .Values.webModeler.restapi.mail.smtpUser)) (ne (or .Values.camundaHub.restapi.mail.smtpUser .Values.webModeler.restapi.mail.smtpUser) "") }}
    {{- if or (and (typeIs "string" (or .Values.camundaHub.restapi.mail.smtpPassword .Values.webModeler.restapi.mail.smtpPassword)) (ne (or .Values.camundaHub.restapi.mail.smtpPassword .Values.webModeler.restapi.mail.smtpPassword) "")) (or .Values.camundaHub.restapi.mail.existingSecret .Values.webModeler.restapi.mail.existingSecret) }}
      {{- $authEnabled = true }}
    {{- end }}
  {{- end }}
  {{- $authEnabled -}}
{{- end -}}

{{/*
[web-modeler] Create the context path for the WebSocket app (= configured context path + suffix "-ws").
*/}}
{{- define "webModeler.websocketContextPath" -}}
  {{- (or .Values.camundaHub.contextPath .Values.webModeler.contextPath) }}-ws
{{- end -}}

{{/*
[web-modeler] Get the host name on which the WebSocket server is reachable from the client.
*/}}
{{- define "webModeler.publicWebsocketHost" -}}
  {{- if and .Values.global.ingress.enabled (or .Values.camundaHub.contextPath .Values.webModeler.contextPath) }}
    {{- tpl .Values.global.host $ }}
  {{- else -}}
    {{- (or .Values.camundaHub.websockets.publicHost .Values.webModeler.websockets.publicHost) }}
  {{- end }}
{{- end -}}

{{/*
[web-modeler] Get the port number on which the WebSocket server is reachable from the client.
*/}}
{{- define "webModeler.publicWebsocketPort" -}}
  {{- if and .Values.global.ingress.enabled (or .Values.camundaHub.contextPath .Values.webModeler.contextPath) }}
    {{- .Values.global.ingress.tls.enabled | ternary "443" "80" }}
  {{- else }}
    {{- (or .Values.camundaHub.websockets.publicPort .Values.webModeler.websockets.publicPort) }}
  {{- end }}
{{- end -}}

{{/*
[web-modeler] Check if TLS must be enabled for WebSocket connections from the client.
*/}}
{{- define "webModeler.websocketTlsEnabled" -}}
  {{- if and .Values.global.ingress.enabled (or .Values.camundaHub.contextPath .Values.webModeler.contextPath) }}
    {{- .Values.global.ingress.tls.enabled }}
  {{- else -}}
    false
  {{- end }}
{{- end -}}

{{/*
[web-modeler] Define variables related to authentication.
*/}}
{{- define "webModeler.authClientId" -}}
  {{- (or .Values.global.identity.auth.camundaHub.clientId .Values.global.identity.auth.webModeler.clientId) | default "web-modeler" -}}
{{- end -}}

{{- define "webModeler.authClientApiAudience" -}}
  {{- (or .Values.global.identity.auth.camundaHub.clientApiAudience .Values.global.identity.auth.webModeler.clientApiAudience) | default "web-modeler-api" -}}
{{- end -}}

{{- define "webModeler.authPublicApiAudience" -}}
  {{- (or .Values.global.identity.auth.camundaHub.publicApiAudience .Values.global.identity.auth.webModeler.publicApiAudience) | default "web-modeler-public-api" -}}
{{- end -}}

{{- define "webModeler.authMethod" -}}
    {{- (or .Values.camundaHub.security.authentication.method .Values.webModeler.security.authentication.method) | default (
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
