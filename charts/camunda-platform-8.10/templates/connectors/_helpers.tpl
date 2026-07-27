{{/* vim: set filetype=mustache: */}}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}

{{ define "connectors.zeebeEndpoint" }}
  {{- include "orchestration.fullname" . | replace "\"" "" -}}:{{- .Values.orchestration.service.grpcPort -}}
{{- end -}}

{{- define "connectors.fullname" -}}
    {{- include "camundaPlatform.componentFullname" (dict
        "componentName" "connectors"
        "componentValues" .Values.connectors
        "context" $
    ) -}}
{{- end -}}

{{- define "connectors.extraLabels" -}}
    {{- include "camundaPlatform.componentExtraLabels" (dict "componentName" "connectors" "componentValuesKey" "connectors" "context" $) -}}
{{- end -}}

{{- define "connectors.labels" -}}
    {{- include "camundaPlatform.componentLabels" (dict "componentName" "connectors" "componentValuesKey" "connectors" "context" $) -}}
{{- end -}}

{{- define "connectors.matchLabels" -}}
    {{- include "camundaPlatform.componentMatchLabels" (dict "componentName" "connectors" "context" $) -}}
{{- end -}}

{{/*
[connectors] Create the name of the service account to use
*/}}
{{- define "connectors.serviceAccountName" -}}
    {{- include "camundaPlatform.serviceAccountName" (dict
        "component" "connectors"
        "context" $
    ) -}}
{{- end -}}

{{/*
[connectors] Get the image pull secrets.
*/}}
{{- define "connectors.imagePullSecrets" -}}
{{- include "camundaPlatform.componentImagePullSecrets" (dict "Values" (set (deepCopy .Values) "image" .Values.connectors.image)) }}
{{- end }}

{{/*
[connectors] Service name.
*/}}
{{- define "connectors.serviceName" -}}
  {{ include "connectors.fullname" . }}
{{- end }}

{{- define "connectors.serviceHeadlessName" -}}
  {{ include "connectors.fullname" . }}-headless
{{- end }}


{{/*
********************************************************************************
Authentication.
********************************************************************************
*/}}

{{/*
[connectors] Define variables related to authentication.
*/}}

{{- define "connectors.authMethod" -}}
    {{- if ne (include "camundaPlatform.connectorsEnabled" .) "true" -}}
        none
    {{- else -}}
        {{- .Values.connectors.security.authentication.method | default (
            .Values.global.security.authentication.method | default "none"
        ) -}}
    {{- end -}}
{{- end -}}

{{/*
[connectors] Defines the auth client
*/}}
{{- define "connectors.authClientId" -}}
    {{- $topologyCluster := .Values.global.topology.cluster | default dict -}}
    {{- $topologyConnectors := dig "components" "connectors" dict $topologyCluster -}}
    {{- if and (eq .Values.global.topology.mode "orchestration") $topologyConnectors.clientId -}}
      {{- $topologyConnectors.clientId -}}
    {{- else -}}
      {{- .Values.connectors.security.authentication.oidc.clientId -}}
    {{- end -}}
{{- end }}

{{- define "connectors.authSecretConfig" -}}
    {{- $topologyCluster := .Values.global.topology.cluster | default dict -}}
    {{- $topologyConnectors := dig "components" "connectors" dict $topologyCluster -}}
    {{- if and (eq .Values.global.topology.mode "orchestration") $topologyConnectors.secret -}}
      {{- toYaml $topologyConnectors -}}
    {{- else -}}
      {{- toYaml .Values.connectors.security.authentication.oidc -}}
    {{- end -}}
{{- end -}}

{{- define "connectors.authAudience" -}}
    {{- .Values.connectors.security.authentication.oidc.audience |
      default (include "orchestration.authAudience" .)
    -}}
{{- end -}}

{{- define "connectors.authTokenScope" -}}
    {{- .Values.connectors.security.authentication.oidc.tokenScope -}}
{{- end -}}
