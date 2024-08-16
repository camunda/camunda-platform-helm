{{/*
Expand the name of the chart.
*/}}
{{- define "execution-identity.name" -}}
    {{- default .Chart.Name .Values.execution-identity.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "execution-identity.fullname" -}}
    {{- include "camundaPlatform.componentFullname" (dict
        "componentName" "execution-identity"
        "componentValues" .Values.execution-identity
        "context" $
    ) -}}
{{- end -}}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "execution-identity.chart" -}}
    {{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Defines extra labels for execution-identity.
*/}}
{{- define "execution-identity.extraLabels" -}}
app.kubernetes.io/component: execution-identity
app.kubernetes.io/version: {{ include "camundaPlatform.imageTagByParams" (dict "base" .Values.global "overlay" .Values.execution-identity) | quote }}
{{- end -}}

{{/*
Common labels
*/}}
{{- define "execution-identity.labels" -}}
{{- template "camundaPlatform.labels" . }}
{{ template "execution-identity.extraLabels" . }}
{{- end -}}

{{/*
Selector labels
*/}}
{{- define "execution-identity.matchLabels" -}}
{{- template "camundaPlatform.matchLabels" . }}
app.kubernetes.io/component: execution-identity
{{- end -}}

{{/*
Create the name of the service account to use
*/}}
{{- define "execution-identity.serviceAccountName" -}}
    {{- include "camundaPlatform.serviceAccountName" (dict
        "component" "execution-identity"
        "context" $
    ) -}}
{{- end -}}

{{/*
Get the image pull secrets.
*/}}
{{- define "execution-identity.imagePullSecrets" -}}
    {{- include "camundaPlatform.imagePullSecrets" (dict
        "component" "execution-identity"
        "context" $
    ) -}}
{{- end }}
