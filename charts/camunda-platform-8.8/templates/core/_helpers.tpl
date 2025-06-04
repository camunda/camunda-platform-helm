{{/* vim: set filetype=mustache: */}}

{{/*
[core] Create a default fully qualified app name.
*/}}
{{- define "core.fullname" -}}
    {{- include "camundaPlatform.componentFullname" (dict
        "componentName" "core"
        "componentValues" .Values.core
        "context" $
    ) -}}
{{- end -}}

{{/*
[core] The old name used in PVC which is used to avoid upgrade downtime.
*/}}
{{- define "core.legacyName" -}}
    {{- printf "%s-zeebe" .Release.Name -}}
{{- end -}}

{{/*
[core] Defines extra labels for core.
*/}}
{{ define "core.extraLabels" -}}
app.kubernetes.io/component: core
app.kubernetes.io/version: {{ include "camundaPlatform.versionLabel" (dict "base" .Values.global "overlay" .Values.core "chart" .Chart) | quote }}
{{- end }}

{{/*
[core] Define common labels for core, combining the match labels and transient labels, which might change on updating
(version depending). These labels shouldn't be used on matchLabels selector, since the selectors are immutable.
*/}}
{{- define "core.labels" -}}
    {{- include "camundaPlatform.labels" . }}
    {{- "\n" }}
    {{- include "core.extraLabels" . }}
{{- end -}}

{{/*
[core] Defines match labels for core, which are extended by sub-charts and should be used in matchLabels selectors.
*/}}
{{- define "core.matchLabels" -}}
    {{- include "camundaPlatform.matchLabels" . }}
app.kubernetes.io/component: core
{{- end -}}

{{/*
[core] Create the name of the service account to use.
*/}}
{{- define "core.serviceAccountName" -}}
    {{- if .Values.core.serviceAccount.enabled -}}
        {{- default (include "core.fullname" .) .Values.core.serviceAccount.name -}}
    {{- else -}}
        {{- default "default" .Values.core.serviceAccount.name -}}
    {{- end -}}
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

{{/*
[web-modeler] Define variables related to authentication.
*/}}
{{- define "core.authClientId" -}}
    {{- .Values.global.identity.auth.core.clientId | default "core" -}}
{{- end -}}

{{- define "core.authClientSecretName" -}}
    {{- if and .Values.global.identity.auth.core.existingSecret (not (typeIs "string" .Values.global.identity.auth.core.existingSecret)) -}}
        {{- include "common.secrets.name" (dict "existingSecret" .Values.global.identity.auth.core.existingSecret "context" $) -}}
    {{- else -}}
        {{- include "camundaPlatform.identitySecretName" (dict "context" . "component" "core") -}}
    {{- end -}}
{{- end -}}

{{- define "core.authClientSecretKey" -}}
    {{ .Values.global.identity.auth.core.existingSecretKey }}
{{- end -}}

{{- define "core.authAudience" -}}
    {{- .Values.global.identity.auth.core.audience | default "core-api" -}}
{{- end -}}

{{- define "core.authTokenScope" -}}
    {{- .Values.global.identity.auth.core.tokenScope -}}
{{- end -}}
