{{/*
A template to handle constraints.
*/}}

{{- $identityEnabled := (or .Values.identity.enabled .Values.global.identity.service.url) }}
{{- $identityAuthEnabled := (or $identityEnabled .Values.global.identity.auth.enabled) }}

{{/*
Fail with a message if Multi-Tenancy is enabled and its requirements are not met which are:
- Identity chart/service.
- Identity PostgreSQL chart/service.
- Identity Auth enabled for other apps.
Multi-Tenancy requirements: https://docs.camunda.io/docs/self-managed/concepts/multi-tenancy/
*/}}
{{- if .Values.global.multitenancy.enabled }}
  {{- $identityDatabaseEnabled := (or .Values.identityPostgresql.enabled .Values.identity.externalDatabase.enabled) }}
  {{- if has false (list $identityAuthEnabled $identityDatabaseEnabled) }}
    {{- $errorMessage := printf "[camunda][error] %s %s %s %s"
        "The Multi-Tenancy feature \"global.multitenancy\" requires Identity enabled and configured with database."
        "Ensure that \"identity.enabled: true\" and \"global.identity.auth.enabled: true\""
        "and Identity database is configured built-in PostgreSQL chart via \"identityPostgresql\""
        "or configure an external database via \"identity.externalDatabase\"."
    -}}
    {{ printf "\n%s" $errorMessage | trimSuffix "\n"| fail }}
  {{- end }}
{{- end }}

{{/*
Fail with a message if the auth type is set to non-Keycloak and its requirements are not met which are:
- Global Identity issuerBackendUrl.
*/}}
{{- if not (eq (upper .Values.global.identity.auth.type) "KEYCLOAK") }}
  {{- if not .Values.global.identity.auth.issuerBackendUrl }}
    {{- $errorMessage := printf "[camunda][error] %s %s"
        "The Identity auth type is set to non-Keycloak but the issuerBackendUrl is not configured."
        "Ensure that \"global.identity.auth.issuerBackendUrl\" is set."
    -}}
    {{ printf "\n%s" $errorMessage | trimSuffix "\n"| fail }}
  {{- end }}
{{- end }}

{{/*
Fail with a message if Identity is disabled and identityKeycloak is enabled.
*/}}
{{- if and (not .Values.identity.enabled) .Values.identityKeycloak.enabled }}
  {{- $errorMessage := "[camunda][error] Identity is disabled but identityKeycloak is enabled. Please ensure that if identityKeycloak is enabled, Identity must also be enabled."
  -}}
  {{ printf "\n%s" $errorMessage | trimSuffix "\n"| fail }}
{{- end }}


{{- define "camunda.constraints.warnings" }}
  {{- if .Values.global.testDeprecationFlags.existingSecretsMustBeSet }}
    {{/* TODO: Check if there are more existingSecrets to check */}}

    {{- $existingSecretsNotConfigured := list }}

    {{ if and (.Values.global.identity.auth.enabled) (.Values.connectors.enabled) (not .Values.global.identity.auth.connectors.existingSecret) }}
      {{- $existingSecretsNotConfigured = append $existingSecretsNotConfigured "global.identity.auth.connectors.existingSecret" }}
    {{- end }}

    {{ if and (.Values.global.identity.auth.enabled) (.Values.identity.enabled) (not  .Values.global.identity.auth.identity.existingSecret) }}
      {{- $existingSecretsNotConfigured = append $existingSecretsNotConfigured "global.identity.auth.identity.existingSecret" }}
    {{- end }}

    {{ if and (.Values.global.identity.auth.enabled) (.Values.operate.enabled) (not .Values.global.identity.auth.operate.existingSecret) }}
      {{- $existingSecretsNotConfigured = append $existingSecretsNotConfigured "global.identity.auth.operate.existingSecret" }}
    {{- end }}

    {{ if and (.Values.global.identity.auth.enabled) (.Values.tasklist.enabled) (not .Values.global.identity.auth.tasklist.existingSecret) }}
      {{- $existingSecretsNotConfigured = append $existingSecretsNotConfigured "global.identity.auth.tasklist.existingSecret" }}
    {{- end }}

    {{ if and (.Values.global.identity.auth.enabled) (.Values.tasklist.enabled) (not .Values.global.identity.auth.optimize.existingSecret) }}
      {{- $existingSecretsNotConfigured = append $existingSecretsNotConfigured "global.identity.auth.optimize.existingSecret" }}
    {{- end }}

    {{ if and (.Values.global.identity.auth.enabled) (.Values.console.enabled) (not .Values.global.identity.auth.console.existingSecret) }}
      {{- $existingSecretsNotConfigured = append $existingSecretsNotConfigured "global.identity.auth.console.existingSecret" }}
    {{- end }}

    {{ if and (.Values.global.identity.auth.enabled) (.Values.zeebe.enabled) (not .Values.global.identity.auth.zeebe.existingSecret) }}
      {{- $existingSecretsNotConfigured = append $existingSecretsNotConfigured "global.identity.auth.zeebe.existingSecret" }}
    {{- end }}

    {{ if and (.Values.identityKeycloak.enabled) (not .Values.identityKeycloak.auth.existingSecret) }}
      {{- $existingSecretsNotConfigured = append $existingSecretsNotConfigured "identityKeycloak.auth.existingSecret" }}
    {{- end }}

    {{ if and (.Values.postgresql.enabled) (not .Values.postgresql.auth.existingSecret) }}
      {{- $existingSecretsNotConfigured = append $existingSecretsNotConfigured "postgresql.auth.existingSecret" }}
    {{- end }}

    {{ if and (.Values.webModeler.enabled) (not .Values.webModeler.restapi.externalDatabase.existingSecret) }}
      {{- $existingSecretsNotConfigured = append $existingSecretsNotConfigured "webModeler.restapi.externalDatabase.existingSecret" }}
    {{- end }}

    {{ if and (.Values.webModeler.enabled) (not .Values.webModeler.mail.existingSecret) }}
      {{- $existingSecretsNotConfigured = append $existingSecretsNotConfigured "webModeler.mail.existingSecret" }}
    {{- end }}

    {{- if eq .Values.global.testDeprecationFlags.existingSecretsMustBeSet "warning" }}
      {{- $errorMessage := (printf "%s"
      `
[camunda][warning]
As of appVersion 8.7, the camunda helm chart will NOT perform automatic passwords generation.
This means that secrets must be provided in the form of kubernetes secrets.
In appVersion 8.6, this warning will display if not all of the necessary existingSecrets are set.:

An example of one of these secrets is:

apiVersion: v1
kind: Secret
metadata:
  name: identity-secret-for-components
type: Opaque
data:
  operate-secret: <base64-encoded-secret>
  tasklist-secret: <base64-encoded-secret>
  optimize-secret: <base64-encoded-secret>
  connectors-secret: <base64-encoded-secret>
  console-secret: <base64-encoded-secret>
  keycloak-secret: <base64-encoded-secret>
  zeebe-secret: <base64-encoded-secret>

The following values inside your values.yaml need to be set but were not:
      `
        )
      -}}
      {{- range $existingSecretsNotConfigured }}
        {{- $errorMessage = (cat "  " $errorMessage "\n" .) }}
      {{- end }}
      {{- printf "\n%s" $errorMessage | trimSuffix "\n" }}
    {{- else if eq .Values.global.testDeprecationFlags.existingSecretsMustBeSet "error" }}
      {{- $errorMessage := (printf "%s"
      `
[camunda][error]
As of appVersion 8.7, the camunda helm chart will NOT perform automatic passwords generation.
This means that secrets must be provided in the form of kubernetes secrets.

An example of one of these secrets is:

apiVersion: v1
kind: Secret
metadata:
  name: identity-secret-for-components
type: Opaque
data:
  operate-secret: <base64-encoded-secret>
  tasklist-secret: <base64-encoded-secret>
  optimize-secret: <base64-encoded-secret>
  connectors-secret: <base64-encoded-secret>
  console-secret: <base64-encoded-secret>
  keycloak-secret: <base64-encoded-secret>
  zeebe-secret: <base64-encoded-secret>

The following values inside your values.yaml need to be set but were not:
      `
        )
      -}}
      {{- range $existingSecretsNotConfigured }}
        {{- $errorMessage = (cat "  " $errorMessage "\n" .) }}
      {{- end }}
      {{ printf "\n%s" $errorMessage | trimSuffix "\n"| fail }}
    {{- end }}
  {{- end }}
{{- end }}


{{/*
TODO: Enable for 8.7 cycle.

Fail with a message if global.zeebePort is set since now it's used from Zeebe Gateway values:
"zeebeGateway.service.grpcPort".
Chart Version: 10.0.0
{{- if (.Values.global.zeebePort) }}
  {{- $errorMessage := printf "[camunda][error] %s %s"
      "The global Zeebe Gateway port \"global.zeebePort\" is deprecated. Please remove it."
      "It is now used directly via \"zeebeGateway.service.grpcPort\"."
  -}}
  {{ printf "\n%s" $errorMessage | trimSuffix "\n"| fail }}
{{- end }}
*/}}

{{/*
TODO: Enable for 8.7 cycle.

********************************************************************************
elasticsearch and opensearch constraints
********************************************************************************
*/}}

{{/*
ensuring external elasticsearch and external opensearch to be mutually exclusive
{{- if and .Values.global.elasticsearch.enabled .Values.global.opensearch.enabled }}
  {{- $errorMessage := "[camunda][error] global.elasticsearch.enabled and global.opensearch.enabled cannot both be true." -}}
  {{ printf "\n%s" $errorMessage | trimSuffix "\n"| fail }}
{{- end }}
*/}}


{{/*
when external elasticsearch is enabled then global elasticsearch should be enabled
{{- if and .Values.global.elasticsearch.external ( not .Values.global.elasticsearch.enabled ) }}
  {{- $errorMessage := "[camunda][error] global.elasticsearch should be enabled with global.elasticsearch.external" -}}
  {{ printf "\n%s" $errorMessage | trimSuffix "\n"| fail }}
{{- end }}
*/}}


{{/*
ensuring internal and external elasticsearch to be mutually exclusive
{{- if and .Values.global.elasticsearch.external .Values.elasticsearch.enabled }}
  {{- $errorMessage := "[camunda][error] global.elasticsearch.external and elasticsearch.enabled cannot both be true." -}}
  {{ printf "\n%s" $errorMessage | trimSuffix "\n"| fail }}
{{- end }}
*/}}

{{/*
ensuring internal and external opensearch to be mutually exclusive
{{- if and .Values.global.opensearch.enabled .Values.elasticsearch.enabled }}
  {{- $errorMessage := "[camunda][error] global.opensearch.enabled and elasticsearch.enabled cannot both be true." -}}
  {{ printf "\n%s" $errorMessage | trimSuffix "\n"| fail }}
{{- end }}
*/}}

{{/*
when global elasticsearch is enabled then either external elasticsearch should be enabled or internal elasticsearch should be enabled
{{- if .Values.global.elasticsearch.enabled -}}
  {{- if and (not .Values.global.elasticsearch.external) (not .Values.elasticsearch.enabled) -}}
  {{- $errorMessage := "[camunda][error] global.elasticsearch.enabled is true, but neither global.elasticsearch.external.enabled nor elasticsearch.enabled is true" -}}
  {{ printf "\n%s" $errorMessage | trimSuffix "\n"| fail }}
  {{- end -}}
{{- end -}}
*/}}

{{/*
[elasticsearch] when existingSecret is provided for elasticsearch then password field should be empty
{{- if and .Values.global.elasticsearch.auth.existingSecret .Values.global.elasticsearch.auth.password }}
  {{- $errorMessage := "[camunda][error] global.elasticsearch.auth.existingSecret and global.elasticsearch.auth.password cannot both be set." -}}
  {{ printf "\n%s" $errorMessage | trimSuffix "\n"| fail }}
{{- end }}
*/}}

{{/*
[opensearch] when existingSecret is provided for opensearch then password field should be empty
{{- if and .Values.global.opensearch.auth.existingSecret .Values.global.opensearch.auth.password }}
  {{- $errorMessage := "[camunda][error] global.opensearch.auth.existingSecret and global.opensearch.auth.password cannot both be set." -}}
  {{ printf "\n%s" $errorMessage | trimSuffix "\n"| fail }}
{{- end }}
*/}}
