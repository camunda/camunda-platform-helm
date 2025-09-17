{{/* vim: set filetype=mustache: */}}

{{/*
[orchestration] Create a default fully qualified app name.
*/}}
{{- define "orchestration.fullname" -}}
    {{- /* NOTE: The value is set to "zeebe" for backward compatibility between 8.7 and 8.8. */ -}}
    {{- include "camundaPlatform.componentFullname" (dict
        "componentName" "zeebe"
        "componentValues" .Values.orchestration
        "context" $
    ) -}}
{{- end -}}

{{/*
[orchestration] Defines extra labels for orchestration.
*/}}

{{ define "orchestration.extraLabelsMain" -}}
{{- /* NOTE: The value is set to "zeebe-broker" for backward compatibility between 8.7 and 8.8. */ -}}
app.kubernetes.io/component: zeebe-broker
app.kubernetes.io/version: {{ include "camundaPlatform.versionLabel" (dict
    "base" .Values.global
    "overlay" .Values.orchestration
    "chart" .Chart
) | quote }}
{{- end }}

{{ define "orchestration.extraLabelsHeadless" -}}
{{- /* NOTE: The value is set to "zeebe-broker" for backward compatibility between 8.7 and 8.8. */ -}}
app.kubernetes.io/component: zeebe-broker-headless
app.kubernetes.io/version: {{ include "camundaPlatform.versionLabel" (dict
    "base" .Values.global
    "overlay" .Values.orchestration
    "chart" .Chart
) | quote }}
{{- end }}

{{/*
[orchestration] Defines extra labels for orchestration.
*/}}
{{ define "orchestration.extraLabelsMigration" -}}
{{- /* NOTE: The value is set to "zeebe-broker" for backward compatibility between 8.7 and 8.8. */ -}}
app.kubernetes.io/component: zeebe-broker-migration
app.kubernetes.io/version: {{ include "camundaPlatform.versionLabel" (dict
    "base" .Values.global
    "overlay" .Values.orchestration
    "chart" .Chart
) | quote }}
{{- end }}

{{/*
[orchestration] Defines extra labels for orchestration importer.
*/}}
{{ define "orchestration.extraLabelsImporter" -}}
app.kubernetes.io/component: zeebe-broker-importer
app.kubernetes.io/version: {{ include "camundaPlatform.versionLabel" (dict
    "base" .Values.global
    "overlay" .Values.orchestration
    "chart" .Chart
) | quote }}
{{- end }}

{{/*
[orchestration] Define common labels for orchestration, combining the match labels and transient labels, which might change on updating
(version depending). These labels shouldn't be used on matchLabels selector, since the selectors are immutable.
*/}}
{{- define "orchestration.labels" -}}
    {{- include "camundaPlatform.labels" . }}
    {{- "\n" }}
    {{- include "orchestration.extraLabelsMain" . }}
{{- end -}}

{{/*
[orchestration] Define common labels for orchestration cluster migrations, combining the match labels and transient labels, which might change on updating
(version depending). These labels shouldn't be used on matchLabels selector, since the selectors are immutable.
*/}}
{{- define "orchestration.labelsMigration" -}}
    {{- include "camundaPlatform.labels" . }}
    {{- "\n" }}
    {{- include "orchestration.extraLabelsMigration" . }}
{{- end -}}

{{/*
[orchestration] Define common labels for orchestration importer, combining the match labels and transient labels, which might change on updating
(version depending). These labels shouldn't be used on matchLabels selector, since the selectors are immutable.
*/}}
{{- define "orchestration.labelsImporter" -}}
    {{- include "camundaPlatform.labels" . }}
    {{- "\n" }}
    {{- include "orchestration.extraLabelsImporter" . }}
{{- end -}}

{{/*
[orchestration] Defines match labels for orchestration, which are extended by sub-charts and should be used in matchLabels selectors.
*/}}
{{- define "orchestration.matchLabels" -}}
    {{- include "camundaPlatform.matchLabels" . }}
    {{- "\n" -}}
    {{- /* NOTE: The value is set to "zeebe-broker" for backward compatibility between 8.7 and 8.8. */ -}}
    app.kubernetes.io/component: zeebe-broker
{{- end -}}


{{/*
[orchestration] Defines match labels for orchestration importer, which are extended by sub-charts and should be used in matchLabels selectors.
*/}}
{{- define "orchestration.matchLabelsImporter" -}}
    {{- include "camundaPlatform.matchLabels" . }}
    {{- "\n" -}}
    app.kubernetes.io/component: zeebe-broker-importer
{{- end -}}

{{/*
[orchestration] Define variables related to multitenancy checks
*/}}
{{- define "orchestration.multitenancyChecksEnabled" -}}
  {{- if .Values.orchestration.multitenancy.checks.enabled -}}
    {{ .Values.orchestration.multitenancy.checks.enabled }}
  {{- else if .Values.global.multitenancy.enabled -}}
    {{ .Values.global.multitenancy.enabled }}
  {{- else -}}
    false
  {{- end -}}
{{- end -}}

{{/*
[orchestration] Define variables related to multitenancy api
*/}}
{{- define "orchestration.multitenancyApiEnabled" -}}
  {{- if .Values.orchestration.multitenancy.api.enabled -}}
    {{ .Values.orchestration.multitenancy.api.enabled }}
  {{- else if .Values.global.multitenancy.enabled -}}
    {{ .Values.global.multitenancy.enabled }}
  {{- else -}}
    false
  {{- end -}}
{{- end -}}

{{/*
[orchestration] Create the name of the service account to use.
*/}}
{{- define "orchestration.serviceAccountName" -}}
    {{- if .Values.orchestration.serviceAccount.enabled -}}
        {{- default (include "orchestration.fullname" .) .Values.orchestration.serviceAccount.name -}}
    {{- else -}}
        {{- default "default" .Values.orchestration.serviceAccount.name -}}
    {{- end -}}
{{- end -}}


{{/*
[orchestration] Define variables related to authentication.
*/}}
{{- define "orchestration.authClientId" -}}
    {{- .Values.global.identity.auth.orchestration.clientId | default "orchestration" -}}
{{- end -}}

{{- define "orchestration.authAudience" -}}
    {{- .Values.global.identity.auth.orchestration.audience | default "orchestration-api" -}}
{{- end -}}

{{- define "orchestration.authTokenScope" -}}
    {{- .Values.global.identity.auth.orchestration.tokenScope -}}
{{- end -}}

{{- define "orchestration.enabledProfiles" -}}
    {{- $enabledProfiles := list -}}
    {{- range $key, $value := .Values.orchestration.profiles }}
        {{- if eq $value true }}
            {{- $enabledProfiles = append $enabledProfiles $key }}
        {{- end }}
    {{- end }}
    {{- join "," $enabledProfiles }}
{{- end -}}

{{- define "orchestration.enabledProfilesWithIdentity" -}}
    {{- if or (eq .Values.orchestration.security.authentication.method "oidc") (eq .Values.orchestration.security.authentication.method "basic") }}
        {{- printf "%s,%s" (include "orchestration.enabledProfiles" .) "consolidated-auth" -}}
    {{- else }}
        {{- include "orchestration.enabledProfiles" . | replace "identity" "auth" -}}
    {{- end }}
{{- end -}}

{{- define "orchestration.secondaryStorage" -}}
    {{- if .Values.global.noSecondaryStorage -}}
        none
    {{- else if .Values.global.elasticsearch.enabled -}}
        elasticsearch
    {{- else if .Values.global.opensearch.enabled -}}
        opensearch
    {{- else -}}
        {{- fail "Please enable a secondary storage type. Either Elasticsearch or OpenSearch" -}}
    {{- end -}}
{{- end -}}

{{- define "orchestration.persistentSessionsEnabled" -}}
    {{- not .Values.global.noSecondaryStorage -}}
{{- end -}}


{{/*
********************************************************************************
Service names.
********************************************************************************
*/}}

{{/*
[orchestration] Define Orchestration Cluster service name - Main.
*/}}
{{- define "orchestration.serviceNameMain" }}
    {{- include "orchestration.fullname" . -}}
{{- end -}}

{{/*
[orchestration] Define Orchestration Cluster service name - Main - gRPC.
*/}}
{{- define "orchestration.serviceNameMainGRPC" }}
    {{- include "orchestration.serviceNameMain" . -}}:{{ .Values.orchestration.service.grpcPort }}
{{- end -}}

{{/*
[orchestration] Define Orchestration Cluster service name - Main - HTTP.
*/}}
{{- define "orchestration.serviceNameMainHTTP" }}
    {{- include "orchestration.serviceNameMain" . -}}:{{ .Values.orchestration.service.httpPort }}
{{- end -}}

{{/*
[orchestration] Define Orchestration Cluster service name - Headless.
*/}}
{{- define "orchestration.serviceNameHeadless" }}
    {{- include "orchestration.fullname" . -}}-headless
{{- end -}}

{{/*
[orchestration] Define Orchestration Cluster service name - Headless - gRPC.
*/}}
{{- define "orchestration.serviceNameHeadlessGRPC" }}
    {{- include "orchestration.serviceNameHeadless" . -}}:{{ .Values.orchestration.service.grpcPort }}
{{- end -}}

{{/*
********************************************************************************
Service labels.
********************************************************************************
*/}}

{{/*
[orchestration] Define Orchestration Cluster service labels - Main.
*/}}
{{- define "orchestration.serviceLabelsMain" }}
    {{- include "camundaPlatform.labels" . }}
    {{- "\n" }}
    {{- include "orchestration.extraLabelsMain" . }}
{{- end -}}

{{/*
[orchestration] Define Orchestration Cluster service labels - Headless.
*/}}
{{- define "orchestration.serviceLabelsHeadless" }}
    {{- include "camundaPlatform.labels" . }}
    {{- "\n" }}
    {{- include "orchestration.extraLabelsHeadless" . }}
{{- end -}}

{{/*
********************************************************************************
Service Ports.
********************************************************************************
*/}}

{{- define "orchestration.servicePortsCommon" -}}
- port: {{ .Values.orchestration.service.internalPort }}
  protocol: TCP
  name: {{ default "internal" .Values.orchestration.service.internalName  }}
- port: {{ .Values.orchestration.service.commandPort }}
  protocol: TCP
  name: {{ default "command" .Values.orchestration.service.commandName }}
- port: {{ .Values.orchestration.service.managementPort }}
  protocol: TCP
  name: {{ default "management" .Values.orchestration.service.managementName }}
- port: {{ .Values.orchestration.service.httpPort }}
  protocol: TCP
  name: {{ default "http" .Values.orchestration.service.httpName  }}
- port: {{ .Values.orchestration.service.grpcPort }}
  protocol: TCP
  name: {{ default "grpc" .Values.orchestration.service.grpcName }}
{{- end -}}
