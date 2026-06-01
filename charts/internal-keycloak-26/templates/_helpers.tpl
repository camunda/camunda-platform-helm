{{/*
Expand the name of the chart.
*/}}
{{- define "keycloak.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "keycloak.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Common labels.
*/}}
{{- define "keycloak.labels" -}}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{ include "keycloak.selectorLabels" . }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels.
*/}}
{{- define "keycloak.selectorLabels" -}}
app.kubernetes.io/name: {{ include "keycloak.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: keycloak
{{- end }}

{{/*
PostgreSQL fullname.
*/}}
{{- define "keycloak.postgresql.fullname" -}}
{{- printf "%s-postgresql" (include "keycloak.fullname" .) | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
PostgreSQL labels.
*/}}
{{- define "keycloak.postgresql.labels" -}}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{ include "keycloak.postgresql.selectorLabels" . }}
app.kubernetes.io/version: {{ .Values.postgresql.image.tag | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
PostgreSQL selector labels.
*/}}
{{- define "keycloak.postgresql.selectorLabels" -}}
app.kubernetes.io/name: {{ include "keycloak.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: postgresql
{{- end }}

{{/*
Keycloak admin password secret name.
*/}}
{{- define "keycloak.secretName" -}}
{{- if .Values.auth.existingSecret }}
{{- .Values.auth.existingSecret }}
{{- else }}
{{- include "keycloak.fullname" . }}
{{- end }}
{{- end }}

{{/*
Keycloak admin password secret key.
*/}}
{{- define "keycloak.secretKey" -}}
{{- .Values.auth.passwordSecretKey | default "keycloak-admin-password" }}
{{- end }}

{{/*
PostgreSQL password secret name.
*/}}
{{- define "keycloak.postgresql.secretName" -}}
{{- if .Values.postgresql.auth.existingSecret }}
{{- .Values.postgresql.auth.existingSecret }}
{{- else }}
{{- include "keycloak.postgresql.fullname" . }}
{{- end }}
{{- end }}

{{/*
PostgreSQL password secret key.
*/}}
{{- define "keycloak.postgresql.secretKey" -}}
{{- .Values.postgresql.auth.passwordSecretKey | default "postgresql-password" }}
{{- end }}

{{/*
PostgreSQL JDBC URL.
*/}}
{{- define "keycloak.postgresql.jdbcUrl" -}}
{{- printf "jdbc:postgresql://%s:5432/%s" (include "keycloak.postgresql.fullname" .) .Values.postgresql.auth.database }}
{{- end }}

{{/*
Image reference helper.
*/}}
{{- define "keycloak.image" -}}
{{- printf "%s/%s:%s" .Values.image.registry .Values.image.repository .Values.image.tag }}
{{- end }}

{{/*
PostgreSQL image reference helper.
*/}}
{{- define "keycloak.postgresql.image" -}}
{{- printf "%s/%s:%s" .Values.postgresql.image.registry .Values.postgresql.image.repository .Values.postgresql.image.tag }}
{{- end }}
