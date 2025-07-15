{{/* vim: set filetype=mustache: */}}

{{- define "identity.authAudience" -}}
  {{- .Values.global.identity.auth.identity.audience | default "camunda-identity-resource-server" -}}
{{- end -}}


{{- define "identity.authClientId" -}}
  {{- .Values.global.identity.auth.identity.clientId | default "camunda-identity" -}}
{{- end -}}
