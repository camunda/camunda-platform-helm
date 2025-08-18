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
{{- if or .Values.identity.multitenancy.enabled .Values.global.multitenancy.enabled }}
  {{- $identityDatabaseEnabled := (or .Values.identityPostgresql.enabled .Values.identity.externalDatabase.enabled) }}
  {{- if has false (list $identityAuthEnabled $identityDatabaseEnabled) }}
    {{- $errorMessage := printf "[camunda][error] %s %s %s %s"
        "The Multi-Tenancy feature \"identity.multitenancy\" requires Identity enabled and configured with database."
        "Ensure that \"identity.enabled: true\" and \"global.identity.auth.enabled: true\""
        "and Identity database is configured built-in PostgreSQL chart via \"identityPostgresql\""
        "or configure an external database via \"identity.externalDatabase\"."
    -}}
    {{ printf "\n%s" $errorMessage | trimSuffix "\n"| fail }}
  {{- end }}
{{- end }}

{{/*
Fail with a message if noSecondaryStorage is enabled but Elasticsearch or OpenSearch are still enabled.
*/}}
{{- if .Values.global.noSecondaryStorage }}
  {{- if or .Values.global.elasticsearch.enabled .Values.global.opensearch.enabled }}
    {{- $errorMessage := printf "[camunda][error] %s %s %s %s"
        "When \"global.noSecondaryStorage\" is enabled, both Elasticsearch and OpenSearch must be disabled."
        "Please ensure that \"global.elasticsearch.enabled: false\" and \"global.opensearch.enabled: false\""
        "are set when using \"global.noSecondaryStorage: true\"."
        "Secondary storage components cannot be enabled when noSecondaryStorage is true."
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
  {{- $errorMessage := printf "[camunda][error] %s %s"
      "Identity is disabled but identityKeycloak is enabled."
      "Please ensure that if identityKeycloak is enabled, Identity must also be enabled."
  -}}
  {{ printf "\n%s" $errorMessage | trimSuffix "\n"| fail }}
{{- end }}

{{/*
[opensearch] when existingSecret is provided for opensearch then password field should be empty
*/}}
{{- if and .Values.global.opensearch.auth.existingSecret .Values.global.opensearch.auth.password }}
  {{- $errorMessage := printf "[camunda][error] %s"
      " global.opensearch.auth.existingSecret and global.opensearch.auth.password cannot both be set."
  -}}
  {{ printf "\n%s" $errorMessage | trimSuffix "\n"| fail }}
{{- end }}

{{/*
Fail with a message if Console is enabled but managed Identity is not enabled.
*/}}
{{- if and .Values.console.enabled (not .Values.identity.enabled) }}
  {{- $errorMessage := printf "[camunda][error] %s %s"
      "Console is enabled but managed Identity is not enabled."
      "Please ensure that if Console is enabled, managed Identity must also be enabled."
  -}}
  {{ printf "\n%s" $errorMessage | trimSuffix "\n"| fail }}
{{- end }}

{{- define "camunda.constraints.warnings" }}
  {{- if .Values.global.testDeprecationFlags.existingSecretsMustBeSet }}
    {{/* TODO: Check if there are more existingSecrets to check */}}

    {{- $existingSecretsNotConfigured := list }}

    {{ if .Values.global.identity.auth.enabled }}    {{ if and (.Values.connectors.enabled)
              (not .Values.global.identity.auth.connectors.existingSecret)
              (not .Values.global.secrets.autoGenerated) }}
      {{- $existingSecretsNotConfigured = append
          $existingSecretsNotConfigured "global.identity.auth.connectors.existingSecret.name" }}
    {{- end }}

    {{ if and (ne (upper .Values.global.identity.auth.type) "KEYCLOAK")
              (.Values.identity.enabled) (not  .Values.global.identity.auth.identity.existingSecret)
              (not .Values.global.secrets.autoGenerated) }}
      {{- $existingSecretsNotConfigured = append
          $existingSecretsNotConfigured "global.identity.auth.identity.existingSecret.name" }}
    {{- end }}

    {{ if and (.Values.console.enabled)
          (not .Values.global.identity.auth.console.existingSecret)
          (not .Values.global.secrets.autoGenerated) }}
      {{- $existingSecretsNotConfigured = append
          $existingSecretsNotConfigured "global.identity.auth.console.existingSecret.name" }}
    {{- end }}

    {{ if and (.Values.orchestration.enabled)
              (not .Values.global.identity.auth.orchestration.existingSecret)
              (not .Values.global.secrets.autoGenerated) }}
      {{- $existingSecretsNotConfigured = append
          $existingSecretsNotConfigured "global.identity.auth.orchestration.existingSecret.name" }}
    {{- end }}
    {{- end }}

  {{ if and (.Values.identityKeycloak.enabled)
            (not .Values.identityKeycloak.auth.existingSecret)
            (not .Values.global.secrets.autoGenerated) }}
    {{- $existingSecretsNotConfigured = append
        $existingSecretsNotConfigured "identityKeycloak.auth.existingSecret"
    }}
  {{- end }}

  {{ if and (.Values.identityKeycloak.postgresql.enabled)
            (not .Values.identityKeycloak.postgresql.auth.existingSecret)
            (not .Values.global.secrets.autoGenerated) }}
    {{- $existingSecretsNotConfigured = append
        $existingSecretsNotConfigured "identityKeycloak.postgresql.auth.existingSecret"
    }}
  {{- end }}

  {{ if and (.Values.webModelerPostgresql.enabled)
            (not .Values.webModelerPostgresql.auth.existingSecret)
            (not .Values.global.secrets.autoGenerated) }}
    {{- $existingSecretsNotConfigured = append
        $existingSecretsNotConfigured "webModelerPostgresql.auth.existingSecret"
    }}
  {{- end }}

  {{ if and (.Values.identityPostgresql.enabled)
            (not .Values.identityPostgresql.auth.existingSecret)
            (not .Values.global.secrets.autoGenerated) }}
    {{- $existingSecretsNotConfigured = append
        $existingSecretsNotConfigured "identityPostgresql.auth.existingSecret"
    }}
  {{- end }}

    {{- if $existingSecretsNotConfigured }}
      {{- if eq .Values.global.testDeprecationFlags.existingSecretsMustBeSet "warning" }}
        {{- $errorMessage := (printf "%s"
      `
[camunda][warning]
DEPRECATION NOTICE: Starting from appVersion 8.7, the Camunda Helm chart will no longer automatically generate passwords for the Identity component.
Users must provide passwords as Kubernetes secrets. 
In appVersion 8.6, this warning will appear if all necessary existingSecrets are not set.

The following values inside your values.yaml need to be set but were not:
      `
          )
        -}}
        {{- range $existingSecretsNotConfigured }}
          {{- $errorMessage = (cat "  " $errorMessage "\n" .) }}
        {{- end }}
        {{- $errorMessage = (cat $errorMessage "\n\n" "Please be aware that each of the above parameters expect a string name of a Kubernetes Secret object.\n") }}
        {{- printf "\n%s" $errorMessage | trimSuffix "\n" }}
      {{- else if eq .Values.global.testDeprecationFlags.existingSecretsMustBeSet "error" }}
        {{- $errorMessage := (printf "%s"
      `
[camunda][error]
DEPRECATION NOTICE: Starting from appVersion 8.7, the Camunda Helm chart will no longer automatically generate passwords for the Identity component.
Users must provide passwords as Kubernetes secrets. 

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
{{- end }}

{{/*
**************************************************************
Deprecation helpers.
**************************************************************
*/}}

{{/*
camundaPlatform.keyRenamed
Fail with message when the old values file key is used and show the new key.
Usage:
{{ include "camundaPlatform.keyRenamed" (dict
  "condition" (.Values.identity.keycloak)
  "oldName" "identity.keycloak"
  "newName" "identityKeycloak"
) }}
*/}}
{{- define "camundaPlatform.keyRenamed" }}
  {{- if .condition }}
    {{- $errorMessage := printf
        "[camunda][error] The Helm values file key changed from \"%s\" to \"%s\". %s %s"
        .oldName .newName
        "For more details, please check Camunda Helm chart documentation."
        "https://docs.camunda.io/docs/self-managed/setup/upgrade/#version-update-instructions"
    -}}
    {{ printf "\n%s" $errorMessage | trimSuffix "\n"| fail }}
  {{- end }}
{{- end -}}


{{/*
camundaPlatform.keyRemoved
Fail with message when the old values file key is used.
Usage:
{{ include "camundaPlatform.keyRemoved" (dict
  "condition" (.Values.identity.keycloak)
  "oldName" "identity.keycloak"
) }}
*/}}
{{- define "camundaPlatform.keyRemoved" }}
  {{- if .condition }}
    {{- $errorMessage := printf
        "[camunda][error] The Helm values file key \"%s\" has been removed. %s %s"
        .oldName
        "For more details, please check Camunda Helm chart documentation."
        "https://docs.camunda.io/docs/self-managed/setup/upgrade/#version-update-instructions"
    -}}
    {{ printf "\n%s" $errorMessage | trimSuffix "\n"| fail }}
  {{- end }}
{{- end -}}


{{/*
*******************************************************************************
Camunda 8.7 cycle deprecated keys.
*******************************************************************************
Fail with a message when old values syntax is used.
Chart Version: 12.0.0
*******************************************************************************
*/}}

{{/*
*******************************************************************************
Global
*******************************************************************************
*/}}

{{/*
- removed: global.multiregion.installationType
*/}}
{{- if hasKey .Values.global.multiregion "installationType" }}
  {{- $errorMessage := printf "[camunda][error] %s %s %s"
      "The option \"global.multiregion.installationType\" has been removed."
      "Use application API's for multi-region failover/failback operations."
      "More details: https://docs.camunda.io/docs/self-managed/operational-guides/multi-region/dual-region-operational-procedure/"
  -}}
  {{ printf "\n%s" $errorMessage | trimSuffix "\n"| fail }}
{{- end }}

{{/*
- changed: global.elasticsearch.url => from string to dict.
- renamed: global.elasticsearch.protocol => global.elasticsearch.url.protocol
- renamed: global.elasticsearch.host => global.elasticsearch.url.host
- renamed: global.elasticsearch.port => global.elasticsearch.url.port
*/}}

{{ include "camundaPlatform.keyRenamed" (dict
  "condition" (eq (kindOf .Values.global.elasticsearch.url) "string")
  "oldName" "global.elasticsearch.url: \"\" (string)"
  "newName" "global.elasticsearch.url: {} (dict)"
) }}

{{ include "camundaPlatform.keyRenamed" (dict
  "condition" (.Values.global.elasticsearch.protocol)
  "oldName" "global.elasticsearch.protocol"
  "newName" "global.elasticsearch.url.protocol"
) }}

{{ include "camundaPlatform.keyRenamed" (dict
  "condition" (.Values.global.elasticsearch.host)
  "oldName" "global.elasticsearch.host"
  "newName" "global.elasticsearch.url.host"
) }}

{{ include "camundaPlatform.keyRenamed" (dict
  "condition" (.Values.global.elasticsearch.port)
  "oldName" "global.elasticsearch.port"
  "newName" "global.elasticsearch.url.port"
) }}

{{/*
*******************************************************************************
Identity.
*******************************************************************************
*/}}

{{- if .Values.identity.enabled -}}
{{/*
- renamed: identity.keycloak => identityKeycloak
*/}}

{{ include "camundaPlatform.keyRenamed" (dict
  "condition" (.Values.identity.keycloak)
  "oldName" "identity.keycloak"
  "newName" "identityKeycloak"
) }}

{{/*
- renamed: identity.postgresql => identityPostgresql
*/}}

{{ include "camundaPlatform.keyRenamed" (dict
  "condition" (.Values.identity.postgresql)
  "oldName" "identity.postgresql"
  "newName" "identityPostgresql"
) }}
{{- end }}

{{/*
*******************************************************************************
Web Modeler
The old key was deprecated in 8.5 and renamed in the 8.8 release.
*******************************************************************************
*/}}

{{- if .Values.webModeler.enabled -}}
  {{/*
  - renamed: postgresql => webModelerPostgresql
  */}}
  {{ include "camundaPlatform.keyRenamed" (dict
    "condition" (.Values.postgresql)
    "oldName" "postgresql"
    "newName" "webModelerPostgresql"
  ) }}
{{- end }}

{{/*
*******************************************************************************
Zeebe Gateway
The old key was deprecated and renamed (with backward compatibility) in 8.5
then removed in the 8.8 release.
*******************************************************************************
*/}}

{{- if (index .Values "zeebe-gateway").enabled -}}
  {{/*
  - renamed: zeebe-gateway => zeebeGateway
  */}}
  {{ include "camundaPlatform.keyRenamed" (dict
    "condition" (index .Values "zeebe-gateway")
    "oldName" "zeebe-gateway"
    "newName" "zeebeGateway"
  ) }}
{{- end }}

{{/*
*******************************************************************************
Separated Ingress.
The old key was deprecated in 8.6 and removed in the 8.8 release.
*******************************************************************************
*/}}

{{ include "camundaPlatform.keyRemoved" (dict
  "condition" ((.Values.identity.ingress).enabled)
  "oldName" "identity.ingress"
) }}

{{ include "camundaPlatform.keyRemoved" (dict
  "condition" ((.Values.console.ingress).enabled)
  "oldName" "console.ingress"
) }}

{{ include "camundaPlatform.keyRemoved" (dict
  "condition" ((.Values.webModeler.ingress).enabled)
  "oldName" "webModeler.ingress"
) }}

{{ include "camundaPlatform.keyRemoved" (dict
  "condition" ((.Values.connectors.ingress).enabled)
  "oldName" "connectors.ingress"
) }}

{{ include "camundaPlatform.keyRemoved" (dict
  "condition" (((.Values.orchestration.ingress).rest).enabled)
  "oldName" "orchestration.ingress.rest"
) }}
