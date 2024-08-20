{{/*
Expand the name of the chart.
*/}}
{{- define "executionIdentity.name" -}}
    {{- default .Chart.Name .Values.executionIdentity.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "executionIdentity.fullname" -}}
    {{- include "camundaPlatform.componentFullname" (dict
        "componentName" "executionIdentity"
        "componentValues" .Values.executionIdentity
        "context" $
    ) -}}
{{- end -}}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "executionIdentity.chart" -}}
    {{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Defines extra labels for executionIdentity.
*/}}
{{- define "executionIdentity.extraLabels" -}}
app.kubernetes.io/component: executionIdentity
app.kubernetes.io/version: {{ include "camundaPlatform.imageTagByParams" (dict "base" .Values.global "overlay" .Values.executionIdentity) | quote }}
{{- end -}}

{{/*
Common labels
*/}}
{{- define "executionIdentity.labels" -}}
{{- template "camundaPlatform.labels" . }}
{{ template "executionIdentity.extraLabels" . }}
{{- end -}}

{{/*
Selector labels
*/}}
{{- define "executionIdentity.matchLabels" -}}
{{- template "camundaPlatform.matchLabels" . }}
app.kubernetes.io/component: executionIdentity
{{- end -}}

{{/*
Create the name of the service account to use
*/}}
{{- define "executionIdentity.serviceAccountName" -}}
    {{- include "camundaPlatform.serviceAccountName" (dict
        "component" "executionIdentity"
        "context" $
    ) -}}
{{- end -}}

{{/*
Get the image pull secrets.
*/}}
{{- define "executionIdentity.imagePullSecrets" -}}
    {{- include "camundaPlatform.imagePullSecrets" (dict
        "component" "executionIdentity"
        "context" $
    ) -}}
{{- end }}
