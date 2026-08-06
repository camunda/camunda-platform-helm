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

{{/*
Defines extra labels for connectors.
*/}}
{{- define "connectors.extraLabels" -}}
app.kubernetes.io/component: connectors
app.kubernetes.io/version: {{ include "camundaPlatform.versionLabel" (dict "base" .Values.global "overlay" .Values.connectors "chart" .Chart) | quote }}
{{- end -}}

{{/*
Define common labels for connectors, combining the match labels and transient labels, which might change on updating
(version depending). These labels shouldn't be used on matchLabels selector, since the selectors are immutable.
*/}}
{{- define "connectors.labels" -}}
{{- template "camundaPlatform.labels" . }}
{{ template "connectors.extraLabels" . }}
{{- end -}}
{{/*
Defines match labels for connectors, which are extended by sub-charts and should be used in matchLabels selectors.
*/}}
{{- define "connectors.matchLabels" -}}
{{- template "camundaPlatform.matchLabels" . }}
app.kubernetes.io/component: connectors
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
{{- include "camundaPlatform.subChartImagePullSecrets" (dict "Values" (set (deepCopy .Values) "image" .Values.connectors.image)) }}
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
    {{- if .Values.connectors.enabled -}}
        {{- .Values.connectors.security.authentication.method | default (
            .Values.global.security.authentication.method | default "none"
        ) -}}
    {{- else -}}
        {{- "none" -}}
    {{- end -}}
{{- end -}}

{{/*
[connectors] Defines the auth client
*/}}
{{- define "connectors.authClientId" -}}
    {{- .Values.connectors.security.authentication.oidc.clientId -}}
{{- end }}

{{- define "connectors.authAudience" -}}
    {{- .Values.connectors.security.authentication.oidc.audience |
      default (include "orchestration.authAudience" .)
    -}}
{{- end -}}

{{- define "connectors.authTokenScope" -}}
    {{- .Values.connectors.security.authentication.oidc.tokenScope -}}
{{- end -}}

{{/*
********************************************************************************
AWS document-store credentials file.
********************************************************************************
*/}}

{{/*
[connectors] Whether the AWS document-store credentials are injected into
this deployment. Mirrors the gate used by camundaPlatform.emitAwsDocumentStoreSecret's
callers, so the init container only ever runs where credentials would
otherwise have rendered as env vars.
*/}}
{{- define "connectors.hasAwsDocumentStoreCredentials" -}}
{{- if and .Values.global.documentStore.type.aws.enabled (not .Values.global.documentStore.type.aws.irsa.enabled) -}}
true
{{- else -}}
false
{{- end -}}
{{- end -}}

{{/*
[connectors] Init container that writes the AWS document-store credentials
to an AWS shared-credentials file, consumed via awsCredentialsFileEnv.

Usage (inside .spec.initContainers):
  {{- if eq (include "connectors.hasAwsDocumentStoreCredentials" .) "true" }}
  {{- include "connectors.awsCredentialsFileInitContainer" (dict "context" $ "image" (include "camundaPlatform.imageByParams" (dict "base" $.Values.global "overlay" $.Values.connectors))) | nindent 8 }}
  {{- end }}
*/}}
{{- define "connectors.awsCredentialsFileInitContainer" -}}
{{- $ctx := .context -}}
- name: aws-credentials-file-init
  image: {{ .image | quote }}
  imagePullPolicy: {{ $ctx.Values.global.image.pullPolicy | quote }}
  {{- if $ctx.Values.connectors.containerSecurityContext }}
  securityContext: {{- include "common.compatibility.renderSecurityContext" (dict "secContext" $ctx.Values.connectors.containerSecurityContext "context" $ctx) | nindent 4 }}
  {{- end }}
  env:
    - name: AWS_ACCESS_KEY_ID
      {{- include "camundaPlatform.emitAwsDocumentStoreSecret" (dict "secretType" "accessKeyId" "context" $ctx) | nindent 6 }}
    - name: AWS_SECRET_ACCESS_KEY
      {{- include "camundaPlatform.emitAwsDocumentStoreSecret" (dict "secretType" "secretAccessKey" "context" $ctx) | nindent 6 }}
  command: ["sh", "-c"]
  args:
    - |
      set -eu
      umask 077
      if [ -z "${AWS_ACCESS_KEY_ID:-}" ] || [ -z "${AWS_SECRET_ACCESS_KEY:-}" ]; then
        echo "[aws-credentials-file-init] ERROR: AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY resolved empty; check global.documentStore.type.aws.accessKeyId / secretAccessKey configuration." >&2
        exit 1
      fi
      printf '[default]\naws_access_key_id=%s\naws_secret_access_key=%s\n' "$AWS_ACCESS_KEY_ID" "$AWS_SECRET_ACCESS_KEY" > /var/camunda/aws-credentials/credentials
      chmod 0600 /var/camunda/aws-credentials/credentials
  volumeMounts:
    - name: aws-credentials-file
      mountPath: /var/camunda/aws-credentials
{{- end -}}

{{/*
[connectors] emptyDir backing the AWS credentials file written by
awsCredentialsFileInitContainer.

Usage (inside .spec.volumes):
  {{- if eq (include "connectors.hasAwsDocumentStoreCredentials" .) "true" }}
  {{- include "connectors.awsCredentialsFileVolume" . | nindent 8 }}
  {{- end }}
*/}}
{{- define "connectors.awsCredentialsFileVolume" -}}
- name: aws-credentials-file
  emptyDir:
    medium: Memory
{{- end -}}

{{/*
[connectors] Main-container volumeMount for the AWS credentials file,
read-only since the init container is the sole writer.

Usage (inside container.volumeMounts):
  {{- if eq (include "connectors.hasAwsDocumentStoreCredentials" .) "true" }}
  {{- include "connectors.awsCredentialsFileVolumeMount" . | nindent 12 }}
  {{- end }}
*/}}
{{- define "connectors.awsCredentialsFileVolumeMount" -}}
- name: aws-credentials-file
  mountPath: /var/camunda/aws-credentials
  readOnly: true
{{- end -}}

{{/*
[connectors] Sets AWS_SHARED_CREDENTIALS_FILE and AWS_CREDENTIAL_PROFILES_FILE
to the credentials file written by awsCredentialsFileInitContainer.

Usage (inside an env: list):
  {{- if eq (include "connectors.hasAwsDocumentStoreCredentials" .) "true" }}
  {{- include "connectors.awsCredentialsFileEnv" . | nindent 12 }}
  {{- end }}
*/}}
{{- define "connectors.awsCredentialsFileEnv" -}}
- name: AWS_SHARED_CREDENTIALS_FILE
  value: /var/camunda/aws-credentials/credentials
- name: AWS_CREDENTIAL_PROFILES_FILE
  value: /var/camunda/aws-credentials/credentials
{{- end -}}
