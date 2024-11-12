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
Fail with a message if global.identity.auth.identity.existingSecret is set and global.identity.auth.type is set to KEYCLOAK
*/}}

{{- if (.Values.global.identity.auth.identity.existingSecret) }}
  {{- if eq (upper .Values.global.identity.auth.type) "KEYCLOAK" }}
    {{- $errorMessage := "[camunda][error] global.identity.auth.identity.existingSecret does not need to be set when using Keycloak."
    -}}
    {{ printf "\n%s" $errorMessage | trimSuffix "\n"| fail }}
  {{- end }}
{{- end }}

{{/*
Fail with a message if adaptSecurityContext has any value other than "force" or "disabled".
*/}}
{{- if not (has .Values.global.compatibility.openshift.adaptSecurityContext (list "force" "disabled")) }}
  {{- $errorMessage := "[camunda][error] Invalid value for adaptSecurityContext. The value must be either 'force' or 'disabled'." -}}
  {{ printf "\n%s" $errorMessage | trimSuffix "\n" | fail }}
{{- end }}

{{/*
Fail with a message if Identity is disabled and identityKeycloak is enabled.
*/}}
{{- if and (not .Values.identity.enabled) .Values.identityKeycloak.enabled }}
  {{- $errorMessage := "[camunda][error] Identity is disabled but identityKeycloak is enabled. Please ensure that if identityKeycloak is enabled, Identity must also be enabled."
  -}}
  {{ printf "\n%s" $errorMessage | trimSuffix "\n"| fail }}
{{- end }}

{{/*
[opensearch] when existingSecret is provided for opensearch then password field should be empty
*/}}
{{- if and .Values.global.opensearch.auth.existingSecret .Values.global.opensearch.auth.password }}
  {{- $errorMessage := "[camunda][error] global.opensearch.auth.existingSecret and global.opensearch.auth.password cannot both be set." -}}
  {{ printf "\n%s" $errorMessage | trimSuffix "\n"| fail }}
{{- end }}

{{- define "camunda.constraints.warnings" }}
  {{- if .Values.global.testDeprecationFlags.existingSecretsMustBeSet }}
    {{/* TODO: Check if there are more existingSecrets to check */}}

    {{- $existingSecretsNotConfigured := list }}

    {{ if and (.Values.global.identity.auth.enabled) (.Values.connectors.enabled) (not .Values.global.identity.auth.connectors.existingSecret) }}
      {{- $existingSecretsNotConfigured = append $existingSecretsNotConfigured "global.identity.auth.connectors.existingSecret.name" }}
    {{- end }}

    {{ if and (.Values.global.identity.auth.enabled) (ne (upper .Values.global.identity.auth.type) "KEYCLOAK") (.Values.identity.enabled) (not  .Values.global.identity.auth.identity.existingSecret) }}
      {{- $existingSecretsNotConfigured = append $existingSecretsNotConfigured "global.identity.auth.identity.existingSecret.name" }}
    {{- end }}

    {{ if and (.Values.global.identity.auth.enabled) (.Values.console.enabled) (not .Values.global.identity.auth.console.existingSecret) }}
      {{- $existingSecretsNotConfigured = append $existingSecretsNotConfigured "global.identity.auth.console.existingSecret.name" }}
    {{- end }}

    {{ if and (.Values.global.identity.auth.enabled) (.Values.core.enabled) (not .Values.global.identity.auth.core.existingSecret) }}
      {{- $existingSecretsNotConfigured = append $existingSecretsNotConfigured "global.identity.auth.core.existingSecret.name" }}
    {{- end }}

    {{ if and (.Values.identityKeycloak.enabled) (not .Values.identityKeycloak.auth.existingSecret) }}
      {{- $existingSecretsNotConfigured = append $existingSecretsNotConfigured "identityKeycloak.auth.existingSecret" }}
    {{- end }}

    {{ if and (.Values.identityKeycloak.postgresql.enabled) (not .Values.identityKeycloak.postgresql.auth.existingSecret) }}
      {{- $existingSecretsNotConfigured = append $existingSecretsNotConfigured "identityKeycloak.postgresql.auth.existingSecret" }}
    {{- end }}

    {{ if and (.Values.postgresql.enabled) (not .Values.postgresql.auth.existingSecret) }}
      {{- $existingSecretsNotConfigured = append $existingSecretsNotConfigured "postgresql.auth.existingSecret" }}
    {{- end }}

    {{ if and (.Values.identityPostgresql.enabled) (not .Values.identityPostgresql.auth.existingSecret) }}
      {{- $existingSecretsNotConfigured = append $existingSecretsNotConfigured "identityPostgresql.auth.existingSecret" }}
    {{- end }}

    {{ if and (.Values.webModeler.enabled) (not .Values.webModeler.restapi.mail.existingSecret) }}
      {{- $existingSecretsNotConfigured = append $existingSecretsNotConfigured "webModeler.restapi.mail.existingSecret.name" }}
    {{- end }}

    {{- if $existingSecretsNotConfigured }}
      {{- if eq .Values.global.testDeprecationFlags.existingSecretsMustBeSet "warning" }}
        {{- $errorMessage := (printf "%s"
      `
[camunda][warning]
DEPRECATION NOTICE: Starting from appVersion 8.7, the Camunda Helm chart will no longer automatically generate passwords for the Identity component.
Users must provide passwords as Kubernetes secrets. 
In appVersion 8.6, this warning will appear if all necessary existingSecrets are not set.

Example of a required secret:

apiVersion: v1
kind: Secret
metadata:
  name: identity-secret-for-components
type: Opaque
data:
  # Identity apps auth.
  connectors-secret: <base64-encoded-secret>
  console-secret: <base64-encoded-secret>
  optimize-secret: <base64-encoded-secret>
  core-secret: <base64-encoded-secret>
  # Identity Keycloak.
  admin-password: <base64-encoded-secret>.
  # Identity Keycloak PostgreSQL.
  postgres-password: <base64-encoded-secret> # used for postgresql admin password
  password: <base64-encoded-secret> # used for postgresql user password
  # Web Modeler.
  smtp-password: <base64-encoded-secret> # used for web modeler mail

The following values inside your values.yaml need to be set but were not:
      `
          )
        -}}
        {{- range $existingSecretsNotConfigured }}
          {{- $errorMessage = (cat "  " $errorMessage "\n" .) }}
        {{- end }}
        {{- $errorMessage = (cat $errorMessage "\n\n" "Please be aware that each of the above parameters expect a string name of a kubernetes Secret.\n") }}
        {{- printf "\n%s" $errorMessage | trimSuffix "\n" }}
      {{- else if eq .Values.global.testDeprecationFlags.existingSecretsMustBeSet "error" }}
        {{- $errorMessage := (printf "%s"
      `
[camunda][error]
DEPRECATION NOTICE: Starting from appVersion 8.7, the Camunda Helm chart will no longer automatically generate passwords for the Identity component.
Users must provide passwords as Kubernetes secrets. 

Example of a required secret:

apiVersion: v1
kind: Secret
metadata:
  name: identity-secret-for-components
type: Opaque
data:
  # Identity apps auth.
  connectors-secret: <base64-encoded-secret>
  console-secret: <base64-encoded-secret>
  optimize-secret: <base64-encoded-secret>
  core-secret: <base64-encoded-secret>
  # Identity Keycloak.
  admin-password: <base64-encoded-secret>.
  # Identity Keycloak PostgreSQL.
  postgres-password: <base64-encoded-secret> # used for postgresql admin password
  password: <base64-encoded-secret> # used for postgresql user password
  # Web Modeler.
  smtp-password: <base64-encoded-secret> # used for web modeler mail

The following values inside your values.yaml need to be set but were not:
      `
          )
        -}}
        {{- range $existingSecretsNotConfigured }}
          {{- $errorMessage = (cat "  " $errorMessage "\n" .) }}
        {{- end }}
        {{- $errorMessage = (cat $errorMessage "\n\n" "Please be aware that each of the above parameters expect a string name of a kubernetes Secret.\n") }}
        {{ printf "\n%s" $errorMessage | trimSuffix "\n"| fail }}
      {{- end }}
    {{- end }}
  {{- end }}
  {{- if .Values.global.multiregion.installationType }}
    {{- $installationTypeMessage := "[camunda][warning]\nDEPRECATION NOTICE: Starting from appVersion 8.7, the Camunda Helm chart will no longer support the global.multiregion.installationType option. This is replaced with a new procedure for managing multi-region installations documented here:\nhttps://docs.camunda.io/docs/self-managed/operational-guides/multi-region/dual-region-operational-procedure/\nPlease unset this option to remove the warning.\n" }}
    {{ printf "\n%s" $installationTypeMessage }}
  {{- end }}
{{- end }}


{{/*
TODO: Enable for 8.7 cycle.

Fail with a message when old values syntax is used.
Chart Version: 12.0.0

{{- if (TBA) }}
  {{- $errorMessage := printf "[camunda][error] %s %s"
      "TBA"
      "TBA"
  -}}
  {{ printf "\n%s" $errorMessage | trimSuffix "\n"| fail }}
{{- end }}
*/}}
