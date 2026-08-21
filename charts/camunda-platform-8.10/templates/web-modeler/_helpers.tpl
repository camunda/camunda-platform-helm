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
    {{- $hub := include "camundaHub.values" . | fromYaml -}}
  {{- include "camundaPlatform.componentFullname" (dict
      "componentName" "web-modeler"
      "componentValues" $hub
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
    {{- $hub := include "camundaHub.values" . | fromYaml -}}
app.kubernetes.io/component: web-modeler
app.kubernetes.io/version: {{ include "camundaPlatform.versionLabel" (dict "base" .Values.global "overlay" $hub "chart" .Chart) | quote }}
{{- end -}}

{{/*
Define common labels for all Web Modeler components.
*/}}
{{- define "webModeler.commonLabels" -}}
    {{- $hub := include "camundaHub.values" . | fromYaml -}}
{{- $values := merge (deepCopy .Values) (dict "nameOverride" (include "webModeler.name" .) "image" $hub.image) }}
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
    {{- $hub := include "camundaHub.values" . | fromYaml -}}
  {{- $image := $hub.image }}
  {{- include "camundaPlatform.componentImagePullSecrets" (dict "Values" (set (deepCopy .Values) "image" $image)) }}
{{- end }}

{{/*
[web-modeler] Get the full name (<registry>/<repository>:<tag>) of the restapi Docker image
*/}}
{{- define "webModeler.restapi.image" -}}
    {{- $hub := include "camundaHub.values" . | fromYaml -}}
  {{- $image := $hub.image }}
  {{- $image = mustMergeOverwrite $image ($hub.restapi.image) }}
  {{- include "camundaPlatform.imageByParams" (dict "base" .Values.global "overlay" (dict "image" $image)) }}
{{- end }}

{{/*
[web-modeler] Get the full name (<registry>/<repository>:<tag>) of the websockets Docker image
*/}}
{{- define "webModeler.websockets.image" -}}
    {{- $hub := include "camundaHub.values" . | fromYaml -}}
  {{- $image := $hub.image }}
  {{- $image = mustMergeOverwrite $image ($hub.websockets.image) }}
  {{- include "camundaPlatform.imageByParams" (dict "base" .Values.global "overlay" (dict "image" $image)) }}
{{- end }}

{{/*
[web-modeler] Create the name of the service account to use
*/}}
{{- define "webModeler.serviceAccountName" -}}
    {{- $hub := include "camundaHub.values" . | fromYaml -}}
    {{- $saName := $hub.serviceAccount.name -}}
    {{- if $hub.serviceAccount.enabled -}}
        {{- $saName | default (include "webModeler.fullname" .) -}}
    {{- else -}}
        {{- $saName | default "default" -}}
    {{- end -}}
{{- end -}}

{{/*
[web-modeler] Get the database JDBC url for the external PostgreSQL.
*/}}
{{- define "webModeler.restapi.databaseUrl" -}}
    {{- $hub := include "camundaHub.values" . | fromYaml -}}
  {{- if $hub.restapi.externalDatabase.url -}}
    {{- $hub.restapi.externalDatabase.url -}}
  {{- else if $hub.restapi.externalDatabase.host -}}
    {{- printf "jdbc:postgresql://%s:%s/%s"
        $hub.restapi.externalDatabase.host
        (toString $hub.restapi.externalDatabase.port)
        $hub.restapi.externalDatabase.database
      -}}
  {{- end -}}
{{- end -}}

{{/*
[web-modeler] Get the database user.
*/}}
{{- define "webModeler.restapi.databaseUser" -}}
    {{- $hub := include "camundaHub.values" . | fromYaml -}}
  {{- $hub.restapi.externalDatabase.username -}}
{{- end -}}

{{/*
[web-modeler] Check if username and password is provided for the SMTP server
*/}}
{{- define "webModeler.restapi.mail.authEnabled" -}}
    {{- $hub := include "camundaHub.values" . | fromYaml -}}
  {{- $authEnabled := false -}}
  {{- if and (typeIs "string" $hub.restapi.mail.smtpUser) (ne $hub.restapi.mail.smtpUser "") }}
    {{- if or (and (typeIs "string" $hub.restapi.mail.smtpPassword) (ne $hub.restapi.mail.smtpPassword "")) $hub.restapi.mail.existingSecret }}
      {{- $authEnabled = true }}
    {{- end }}
  {{- end }}
  {{- $authEnabled -}}
{{- end -}}

{{/*
[web-modeler] Create the context path for the WebSocket app (= configured context path + suffix "-ws").
*/}}
{{- define "webModeler.websocketContextPath" -}}
    {{- $hub := include "camundaHub.values" . | fromYaml -}}
  {{- $hub.contextPath }}-ws
{{- end -}}

{{/*
[web-modeler] Get the host name on which the WebSocket server is reachable from the client.
*/}}
{{- define "webModeler.publicWebsocketHost" -}}
    {{- $hub := include "camundaHub.values" . | fromYaml -}}
  {{- if and .Values.global.ingress.enabled $hub.contextPath }}
    {{- tpl .Values.global.host $ }}
  {{- else -}}
    {{- $hub.websockets.publicHost }}
  {{- end }}
{{- end -}}

{{/*
[web-modeler] Get the port number on which the WebSocket server is reachable from the client.
*/}}
{{- define "webModeler.publicWebsocketPort" -}}
    {{- $hub := include "camundaHub.values" . | fromYaml -}}
  {{- if and .Values.global.ingress.enabled $hub.contextPath }}
    {{- .Values.global.ingress.tls.enabled | ternary "443" "80" }}
  {{- else }}
    {{- $hub.websockets.publicPort }}
  {{- end }}
{{- end -}}

{{/*
[web-modeler] Check if TLS must be enabled for WebSocket connections from the client.
*/}}
{{- define "webModeler.websocketTlsEnabled" -}}
    {{- $hub := include "camundaHub.values" . | fromYaml -}}
  {{- if and .Values.global.ingress.enabled $hub.contextPath }}
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
    {{- $hub := include "camundaHub.values" . | fromYaml -}}
    {{- $hub.security.authentication.method | default (
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
