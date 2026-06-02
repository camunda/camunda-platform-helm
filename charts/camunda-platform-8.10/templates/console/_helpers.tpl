{{- define "console.fullname" -}}
    {{/* mustMergeOverwrite is used instead of or because camundaHub.console has intermediate
         sub-maps that make it truthy even when no overrides are set. Deep-merging empty maps
         is a no-op, preserving all legacy values. */}}
    {{- include "camundaPlatform.componentFullname" (dict
        "componentName" "console"
        "componentValues" (mustMergeOverwrite (deepCopy .Values.console) .Values.camundaHub.console)
        "context" $
    ) -}}
{{- end -}}

{{- define "console.extraLabels" -}}
app.kubernetes.io/component: console
app.kubernetes.io/version: {{ include "camundaPlatform.versionLabel" (dict "base" .Values.global "overlay" (mustMergeOverwrite (deepCopy .Values.console) .Values.camundaHub.console) "chart" .Chart) | quote }}
{{- end -}}

{{- define "console.labels" -}}
    {{- include "camundaPlatform.labels" . }}
    {{- "\n" }}
    {{- include "console.extraLabels" . }}
{{- end -}}

{{- define "console.matchLabels" -}}
    {{- include "camundaPlatform.componentMatchLabels" (dict "componentName" "console" "context" $) -}}
{{- end -}}

{{/*
Create the name of the service account to use
*/}}
{{- define "console.serviceAccountName" -}}
    {{- $saName := (or .Values.camundaHub.console.serviceAccount.name .Values.console.serviceAccount.name) -}}
    {{- if .Values.camundaHub.console.serviceAccount.enabled -}}
        {{- $saName | default (include "console.fullname" .) -}}
    {{- else -}}
        {{- $saName | default "default" -}}
    {{- end -}}
{{- end -}}

{{/*
Get the image pull secrets.
*/}}
{{- define "console.imagePullSecrets" -}}
    {{- $componentValue := (or .Values.camundaHub.console.image.pullSecrets .Values.console.image.pullSecrets) -}}
    {{- $globalValue := .Values.global.image.pullSecrets -}}
    {{- $componentValue | default $globalValue | default list | toYaml -}}
{{- end }}

{{/*
[console] Define variables related to authentication.
*/}}
{{- define "console.authAudience" -}}
  {{- (or .Values.global.identity.auth.camundaHub.console.audience .Values.global.identity.auth.console.audience) | default "console-api" -}}
{{- end -}}
