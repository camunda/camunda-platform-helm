{{/* vim: set filetype=mustache: */}}
{{/*
Expand the name of the chart.
*/}}
{{- define "ccsm.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Common labels
*/}}
{{- define "ccsm.labels" -}}
{{- if .Values.global.labels -}}
{{ toYaml .Values.global.labels }}
{{- end }}
app.kubernetes.io/name: {{ template "ccsm.name" . }}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version | replace "+" "_" }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- if .Values.image }}
    {{- if .Values.image.tag }}
app.kubernetes.io/version: {{ .Values.image.tag | quote }}
    {{- else }}
app.kubernetes.io/version: {{ .Values.global.image.tag | quote }}
    {{- end }}
{{- else }}
app.kubernetes.io/version: {{ .Values.global.image.tag | quote }}
{{- end }}
app.kubernetes.io/part-of: camunda-cloud-self-managed
{{- end -}}

