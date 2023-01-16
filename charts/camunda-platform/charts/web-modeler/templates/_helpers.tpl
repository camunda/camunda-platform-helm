{{/* vim: set filetype=mustache: */}}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "webModeler.fullname" -}}
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

{{/*
Define extra labels for Web Modeler.
*/}}
{{- define "webModeler.extraLabels" -}}
app.kubernetes.io/component: web-modeler
{{- end -}}

{{/*
Define extra labels for Web Modeler restapi.
*/}}
{{- define "webModeler.restapi.extraLabels" -}}
app.kubernetes.io/component: restapi
{{- end -}}

{{/*
Define extra labels for Web Modeler webapp.
*/}}
{{- define "webModeler.webapp.extraLabels" -}}
app.kubernetes.io/component: webapp
{{- end -}}

{{/*
Define extra labels for Web Modeler websockets.
*/}}
{{- define "webModeler.websockets.extraLabels" -}}
app.kubernetes.io/component: websockets
{{- end -}}

{{/*
Define common labels for Web Modeler, combining the match labels and transient labels, which might change on updating
(version depending). These labels shouldn't be used on matchLabels selector, since the selectors are immutable.
*/}}
{{- define "webModeler.labels" -}}
{{- template "camundaPlatform.labels" . }}
{{ template "webModeler.extraLabels" . }}
{{- end -}}

{{/*
Define common labels for Web Modeler restapi, combining the match labels and transient labels, which might change on updating
(version depending). These labels shouldn't be used on matchLabels selector, since the selectors are immutable.
*/}}
{{- define "webModeler.restapi.labels" -}}
{{- template "camundaPlatform.labels" . }}
{{ template "webModeler.restapi.extraLabels" . }}
{{- end -}}

{{/*
Define common labels for Web Modeler webapp, combining the match labels and transient labels, which might change on updating
(version depending). These labels shouldn't be used on matchLabels selector, since the selectors are immutable.
*/}}
{{- define "webModeler.webapp.labels" -}}
{{- template "camundaPlatform.labels" . }}
{{ template "webModeler.webapp.extraLabels" . }}
{{- end -}}

{{/*
Define common labels for Web Modeler websockets, combining the match labels and transient labels, which might change on updating
(version depending). These labels shouldn't be used on matchLabels selector, since the selectors are immutable.
*/}}
{{- define "webModeler.websockets.labels" -}}
{{- template "camundaPlatform.labels" . }}
{{ template "webModeler.websockets.extraLabels" . }}
{{- end -}}

{{/*
Define match labels for Web Modeler restapi, which are extended by sub-charts and should be used in matchLabels selectors.
*/}}
{{- define "webModeler.restapi.matchLabels" -}}
{{- template "camundaPlatform.matchLabels" . }}
{{ template "webModeler.restapi.extraLabels" . }}
{{- end -}}

{{/*
Define match labels for Web Modeler webapp, which are extended by sub-charts and should be used in matchLabels selectors.
*/}}
{{- define "webModeler.webapp.matchLabels" -}}
{{- template "camundaPlatform.matchLabels" . }}
{{ template "webModeler.webapp.extraLabels" . }}
{{- end -}}

{{/*
Define match labels for Web Modeler websockets, which are extended by sub-charts and should be used in matchLabels selectors.
*/}}
{{- define "webModeler.websockets.matchLabels" -}}
{{- template "camundaPlatform.matchLabels" . }}
{{ template "webModeler.websockets.extraLabels" . }}
{{- end -}}

{{/*
[web-modeler] Get the full name (<registry>/<repository>:<tag>) of the restapi Docker image
*/}}
{{- define "webModeler.restapi.image" -}}
{{ include "camundaPlatform.imageByParams" (dict "base" .Values.global "overlay" (dict "image" (deepCopy .Values.image | merge .Values.restapi.image))) }}
{{- end }}

{{/*
[web-modeler] Get the full name (<registry>/<repository>:<tag>) of the webapp Docker image
*/}}
{{- define "webModeler.webapp.image" -}}
{{ include "camundaPlatform.imageByParams" (dict "base" .Values.global "overlay" (dict "image" (deepCopy .Values.image | merge .Values.webapp.image))) }}
{{- end }}

{{/*
[web-modeler] Get the full name (<registry>/<repository>:<tag>) of the websockets Docker image
*/}}
{{- define "webModeler.websockets.image" -}}
{{ include "camundaPlatform.imageByParams" (dict "base" .Values.global "overlay" (dict "image" (deepCopy .Values.image | merge .Values.websockets.image))) }}
{{- end }}

{{/*
[web-modeler] Create the name of the service account to use
*/}}
{{- define "webModeler.serviceAccountName" -}}
{{- if .Values.serviceAccount.enabled }}
{{- default (include "webModeler.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
[web-modeler] Get the database host, depending on whether the postgresql dependency chart is enabled.
*/}}
{{- define "webModeler.restapi.databaseHost" -}}
{{- .Values.postgresql.enabled | ternary (include "webModeler.postgresql.fullname" .) .Values.restapi.externalDatabase.host -}}
{{- end -}}

{{/*
[web-modeler] Get the database port, depending on whether the postgresql dependency chart is enabled.
*/}}
{{- define "webModeler.restapi.databasePort" -}}
{{- .Values.postgresql.enabled | ternary 5432 .Values.restapi.externalDatabase.port -}}
{{- end -}}

{{/*
[web-modeler] Get the database name, depending on whether the postgresql dependency chart is enabled.
*/}}
{{- define "webModeler.restapi.databaseName" -}}
{{- .Values.postgresql.enabled | ternary .Values.postgresql.auth.database .Values.restapi.externalDatabase.database -}}
{{- end -}}

{{/*
[web-modeler] Get the database user, depending on whether the postgresql dependency chart is enabled.
*/}}
{{- define "webModeler.restapi.databaseUser" -}}
{{- .Values.postgresql.enabled | ternary .Values.postgresql.auth.username .Values.restapi.externalDatabase.user -}}
{{- end -}}

{{/*
[web-modeler] Get the name of the secret that contains the database password, depending on whether the postgresql dependency chart is enabled.
*/}}
{{- define "webModeler.restapi.databaseSecretName" -}}
{{- .Values.postgresql.enabled | ternary (include "webModeler.postgresql.fullname" .) (include "webModeler.restapi.fullname" .) -}}
{{- end -}}

{{/*
[web-modeler] Get the name of the database password key in the secret, depending on whether the postgresql dependency chart is enabled.
*/}}
{{- define "webModeler.restapi.databaseSecretKey" -}}
{{- .Values.postgresql.enabled | ternary "password" "database-password" -}}
{{- end -}}

{{/*
[web-modeler] Get the full name of the Kubernetes objects from the postgresql dependency chart
*/}}
{{- define "webModeler.postgresql.fullname" -}}
{{- include "common.names.dependency.fullname" (dict "chartName" "postgresql" "chartValues" .Values.postgresql "context" $) -}}
{{- end -}}

{{/*
[web-modeler] Create the base URL of the Identity API (using backchannel communication)
*/}}
{{- define "webModeler.identityBaseUrl" -}}
http://{{ include "identity.fullname" . }}:{{ .Values.global.identity.service.port }}
{{- end -}}
