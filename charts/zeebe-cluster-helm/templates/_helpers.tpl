{{/* vim: set filetype=mustache: */}}
{{/*
Expand the name of the chart.
*/}}
{{- define "zeebe-cluster.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "zeebe-cluster.fullname" -}}
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
Create chart name and version as used by the chart label.
*/}}
{{- define "zeebe-cluster.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Common labels
*/}}
{{- define "zeebe-cluster.labels" -}}
app.kubernetes.io/name: {{ include "zeebe-cluster.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{- define "zeebe-cluster.version" -}}
{{- printf "%s:%s" .Values.image.repository .Values.image.tag -}}
{{- end -}}

{{- define "zeebe-cluster.labels.broker" -}}
{{- template "zeebe-cluster.labels" . }}
app.kubernetes.io/component: broker
{{- end -}}

{{- define "zeebe-cluster.labels.gateway" -}}
{{- template "zeebe-cluster.labels" . }}
app.kubernetes.io/component: gateway
{{- end -}}

{{/*
Common names
*/}}
{{- define "zeebe-cluster.names.broker" -}}
{{- if .Values.global.zeebe -}}
{{- tpl .Values.global.zeebe . | trunc 63 | trimSuffix "-" | quote -}}
{{- else -}}
{{- printf "%s-broker" .Release.Name | trunc 63 | trimSuffix "-" | quote -}}
{{- end -}}
{{- end -}}

{{/*
Creates a valid DNS name for the gateway
*/}}
{{- define "zeebe-cluster.names.gateway" -}}
{{- $name := default .Release.Name (tpl .Values.global.zeebe .) -}}
{{- printf "%s-gateway" $name | trunc 63 | trimSuffix "-" | quote -}}
{{- end -}}
