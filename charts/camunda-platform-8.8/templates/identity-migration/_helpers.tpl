{{/* vim: set filetype=mustache: */}}

{{- define "identity.authAudience" -}}
  {{- .Values.global.identity.auth.identity.audience | default "camunda-identity-resource-server" -}}
{{- end -}}


{{- define "identity.authClientId" -}}
  {{- .Values.global.identity.auth.identity.clientId | default "camunda-identity" -}}
{{- end -}}

{{- define "identityMigration.fullname" -}}
    {{- include "camundaPlatform.componentFullname" (dict
        "componentName" "identity-migration"
        "componentValues" .Values.identity.migration
        "context" $
    ) -}}
{{- end -}}

{{/*
Define common labels for identity migration app, combining the match labels and transient labels, which might change on updating
(version depending). These labels shouldn't be used on matchLabels selector, since the selectors are immutable.
*/}}
{{- define "identityMigration.labels" -}}
{{- template "camundaPlatform.labels" . }}
{{ template "identityMigration.extraLabels" . }}
{{- end -}}

{{/*
[core] Get the image pull secrets.
*/}}
{{- define "core.imagePullSecrets" -}}
    {{- include "camundaPlatform.imagePullSecrets" (dict
        "component" "core"
        "context" $
    ) -}}
{{- end }}
