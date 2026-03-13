{{- define "console.fullname" -}}
    {{- $consoleVals := include "camundaHub.consoleValues" . | fromYaml -}}
    {{- include "camundaPlatform.componentFullname" (dict
        "componentName" "console"
        "componentValues" $consoleVals
        "context" $
    ) -}}
{{- end -}}

{{- define "console.extraLabels" -}}
    {{- include "camundaPlatform.componentExtraLabels" (dict "componentName" "console" "componentValuesKey" "console" "context" $) -}}
{{- end -}}

{{- define "console.labels" -}}
    {{- include "camundaPlatform.componentLabels" (dict "componentName" "console" "componentValuesKey" "console" "context" $) -}}
{{- end -}}

{{- define "console.matchLabels" -}}
    {{- include "camundaPlatform.componentMatchLabels" (dict "componentName" "console" "context" $) -}}
{{- end -}}

{{/*
Create the name of the service account to use
*/}}
{{- define "console.serviceAccountName" -}}
    {{- include "camundaPlatform.serviceAccountName" (dict
        "component" "console"
        "context" $
    ) -}}
{{- end -}}

{{/*
Get the image pull secrets.
*/}}
{{- define "console.imagePullSecrets" -}}
    {{- include "camundaPlatform.imagePullSecrets" (dict
        "component" "console"
        "context" $
    ) -}}
{{- end }}

{{/*
[console] Define variables related to authentication.
*/}}
{{- define "console.authAudience" -}}
  {{- (include "camundaHub.consoleAuthValues" . | fromYaml).audience | default "console-api" -}}
{{- end -}}
