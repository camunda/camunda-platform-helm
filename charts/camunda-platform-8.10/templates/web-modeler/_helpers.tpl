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
  {{- $wmVals := include "camundaHub.webModelerValues" . | fromYaml -}}
  {{- include "camundaPlatform.componentFullname" (dict
      "componentName" "web-modeler"
      "componentValues" $wmVals
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
{{- $wmVals := include "camundaHub.webModelerValues" . | fromYaml -}}
{{- $values := merge (deepCopy .Values) (dict "nameOverride" (include "webModeler.name" .) "image" $wmVals.image) }}
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
  {{- $wmVals := include "camundaHub.webModelerValues" . | fromYaml -}}
  {{- include "camundaPlatform.subChartImagePullSecrets" (dict "Values" (set (deepCopy .Values) "image" $wmVals.image)) }}
{{- end }}

{{/*
[web-modeler] Get the full name (<registry>/<repository>:<tag>) of the restapi Docker image
*/}}
{{- define "webModeler.restapi.image" -}}
  {{- $wmVals := include "camundaHub.webModelerValues" . | fromYaml -}}
  {{- include "camundaPlatform.imageByParams" (dict "base" .Values.global "overlay" (dict "image" (deepCopy $wmVals.image | merge $wmVals.restapi.image))) }}
{{- end }}

{{/*
[web-modeler] Get the full name (<registry>/<repository>:<tag>) of the websockets Docker image
*/}}
{{- define "webModeler.websockets.image" -}}
  {{- $wmVals := include "camundaHub.webModelerValues" . | fromYaml -}}
  {{- include "camundaPlatform.imageByParams" (dict "base" .Values.global "overlay" (dict "image" (deepCopy $wmVals.image | merge $wmVals.websockets.image))) }}
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
  {{- $wmVals := include "camundaHub.webModelerValues" . | fromYaml -}}
  {{- if .Values.webModelerPostgresql.enabled -}}
    {{- printf "jdbc:postgresql://%s:5432/%s"
        (include "webModeler.postgresql.fullname" .)
        (.Values.webModelerPostgresql.auth.database)
      -}}
  {{- else if $wmVals.restapi.externalDatabase.url -}}
    {{- $wmVals.restapi.externalDatabase.url -}}
  {{- else if $wmVals.restapi.externalDatabase.host -}}
    {{- printf "jdbc:postgresql://%s:%s/%s"
        $wmVals.restapi.externalDatabase.host
        (toString ($wmVals.restapi.externalDatabase.port))
        ($wmVals.restapi.externalDatabase.database)
      -}}
  {{- end -}}
{{- end -}}

{{/*
[web-modeler] Get the database user, depending on whether the postgresql dependency chart is enabled.
*/}}
{{- define "webModeler.restapi.databaseUser" -}}
  {{- $wmVals := include "camundaHub.webModelerValues" . | fromYaml -}}
  {{- if .Values.webModelerPostgresql.enabled -}}
    {{- .Values.webModelerPostgresql.auth.username -}}
  {{- else -}}
    {{- .Values.webModeler.restapi.externalDatabase.username -}}
  {{- end -}}
{{- end -}}

{{/*
[web-modeler] Check if username and password is provided for the SMTP server
*/}}
{{- define "webModeler.restapi.mail.authEnabled" -}}
  {{- $wmVals := include "camundaHub.webModelerValues" . | fromYaml -}}
  {{- $authEnabled := false -}}
  {{- if and (typeIs "string" $wmVals.restapi.mail.smtpUser) (ne $wmVals.restapi.mail.smtpUser "") }}
    {{- if or (and (typeIs "string" $wmVals.restapi.mail.smtpPassword) (ne $wmVals.restapi.mail.smtpPassword "")) $wmVals.restapi.mail.existingSecret }}
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
  {{- $wmVals := include "camundaHub.webModelerValues" . | fromYaml -}}
  {{- $wmVals.contextPath }}-ws
{{- end -}}

{{/*
[web-modeler] Get the host name on which the WebSocket server is reachable from the client.
*/}}
{{- define "webModeler.publicWebsocketHost" -}}
  {{- $wmVals := include "camundaHub.webModelerValues" . | fromYaml -}}
  {{- if and .Values.global.ingress.enabled $wmVals.contextPath }}
    {{- tpl .Values.global.host $ }}
  {{- else -}}
    {{- $wmVals.websockets.publicHost }}
  {{- end }}
{{- end -}}

{{/*
[web-modeler] Get the port number on which the WebSocket server is reachable from the client.
*/}}
{{- define "webModeler.publicWebsocketPort" -}}
  {{- $wmVals := include "camundaHub.webModelerValues" . | fromYaml -}}
  {{- if and .Values.global.ingress.enabled $wmVals.contextPath }}
    {{- .Values.global.ingress.tls.enabled | ternary "443" "80" }}
  {{- else }}
    {{- $wmVals.websockets.publicPort }}
  {{- end }}
{{- end -}}

{{/*
[web-modeler] Check if TLS must be enabled for WebSocket connections from the client.
*/}}
{{- define "webModeler.websocketTlsEnabled" -}}
  {{- $wmVals := include "camundaHub.webModelerValues" . | fromYaml -}}
  {{- if and .Values.global.ingress.enabled $wmVals.contextPath }}
    {{- .Values.global.ingress.tls.enabled }}
  {{- else -}}
    false
  {{- end }}
{{- end -}}

{{/*
[web-modeler] Define variables related to authentication.
*/}}
{{- define "webModeler.authClientId" -}}
  {{- (include "camundaHub.webModelerAuthValues" . | fromYaml).clientId | default "web-modeler" -}}
{{- end -}}

{{- define "webModeler.authClientApiAudience" -}}
  {{- (include "camundaHub.webModelerAuthValues" . | fromYaml).clientApiAudience | default "web-modeler-api" -}}
{{- end -}}

{{- define "webModeler.authPublicApiAudience" -}}
  {{- (include "camundaHub.webModelerAuthValues" . | fromYaml).publicApiAudience | default "web-modeler-public-api" -}}
{{- end -}}

{{- define "webModeler.authMethod" -}}
    {{- $wmVals := include "camundaHub.webModelerValues" . | fromYaml -}}
    {{- $wmVals.security.authentication.method | default (
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
