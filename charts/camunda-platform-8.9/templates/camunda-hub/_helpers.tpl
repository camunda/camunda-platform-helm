{{/*
Camunda Hub Helpers - Experimental Component
This component is experimental in Camunda 8.9 and will replace Console and Web Modeler in 8.10.
*/}}

{{/*
Expand the name of the chart.
*/}}
{{- define "camundaHub.name" -}}
    {{- default .Chart.Name .Values.camundaHub.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "camundaHub.fullname" -}}
    {{- include "camundaPlatform.componentFullname" (dict
        "componentName" "camunda-hub"
        "componentValues" .Values.camundaHub
        "context" $
    ) -}}
{{- end -}}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "camundaHub.chart" -}}
    {{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Defines extra labels for Camunda Hub.
*/}}
{{- define "camundaHub.extraLabels" -}}
app.kubernetes.io/component: camunda-hub
app.kubernetes.io/version: {{ include "camundaPlatform.versionLabel" (dict "base" .Values.global "overlay" .Values.camundaHub "chart" .Chart) | quote }}
{{- end -}}

{{/*
Common labels
*/}}
{{- define "camundaHub.labels" -}}
{{- template "camundaPlatform.labels" . }}
{{ template "camundaHub.extraLabels" . }}
{{- end -}}

{{/*
Selector labels
*/}}
{{- define "camundaHub.matchLabels" -}}
{{- template "camundaPlatform.matchLabels" . }}
app.kubernetes.io/component: camunda-hub
{{- end -}}

{{/*
Create the name of the service account to use
*/}}
{{- define "camundaHub.serviceAccountName" -}}
    {{- include "camundaPlatform.serviceAccountName" (dict
        "component" "camundaHub"
        "context" $
    ) -}}
{{- end -}}

{{/*
Get the image pull secrets.
*/}}
{{- define "camundaHub.imagePullSecrets" -}}
    {{- include "camundaPlatform.imagePullSecrets" (dict
        "component" "camundaHub"
        "context" $
    ) -}}
{{- end }}

{{/*
[camundaHub] Define variables related to authentication.
*/}}
{{- define "camundaHub.authAudience" -}}
  {{- .Values.global.identity.auth.camundaHub.audience | default "camunda-hub-api" -}}
{{- end -}}

{{/*
[camundaHub] Validate experimental acknowledgement.
This helper is called in deployment.yaml to ensure users explicitly acknowledge the experimental nature.
*/}}
{{- define "camundaHub.validateExperimental" -}}
{{- if not .Values.camundaHub.experimental.acknowledged -}}
{{- fail `
[camunda][error] Camunda Hub is an EXPERIMENTAL feature in Camunda 8.9.

To enable this experimental feature, you must explicitly acknowledge this by setting:

    camundaHub:
      enabled: true
      experimental:
        acknowledged: true

WARNING:
- This feature is under active development and may change without notice
- Not recommended for production use
- Configuration options may change in future releases
- Camunda Hub will replace Console and Web Modeler in Camunda 8.10

For more information, see: https://docs.camunda.io/docs/self-managed/
` -}}
{{- end -}}
{{- end -}}
