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
Fail if there is no secondary storage type specified and if noSecondaryStorage is not enabled.
*/}}
{{- if and .Values.orchestration.enabled (eq (include "orchestration.secondaryStorage" .) "unset") }}
  {{- fail "Please configure an expected secondary storage type under `orchestration.data.secondaryStorage.type`, available values are [elasticsearch, opensearch, rdbms]. For more details, see our documentation here: https://docs.camunda.io/docs/next/self-managed/concepts/secondary-storage/configuring-secondary-storage/" -}}
{{- end }}

{{/*
Fail with a message if noSecondaryStorage is enabled but Elasticsearch or OpenSearch are still enabled.
*/}}
{{- if .Values.global.noSecondaryStorage }}
  {{- if or .Values.global.elasticsearch.enabled .Values.global.opensearch.enabled .Values.elasticsearch.enabled }}
    {{- $errorMessage := printf "[camunda][error] %s %s %s %s"
        "When \"global.noSecondaryStorage\" is enabled, both Elasticsearch and OpenSearch must be disabled."
        "Please ensure that \"global.elasticsearch.enabled: false\", \"global.opensearch.enabled: false\", and \"elasticsearch.enabled: false\""
        "are set when using \"global.noSecondaryStorage: true\"."
        "Secondary storage components cannot be enabled when noSecondaryStorage is true."
    -}}
    {{ printf "\n%s" $errorMessage | trimSuffix "\n"| fail }}
  {{- end }}
{{- end }}

{{/*
Fail with a message if the auth type is not in the enums (KEYCLOAK, MICROSOFT, or GENERIC).
*/}}
{{- if not (has (include "camundaPlatform.authIssuerType" .) (list "KEYCLOAK" "MICROSOFT" "GENERIC")) }}
  {{- $errorMessage := printf "[camunda][error] %s"
      "The Identity auth type should be one of the following values: KEYCLOAK, MICROSOFT, or GENERIC."
  -}}
  {{ printf "\n%s" $errorMessage | trimSuffix "\n"| fail }}
{{- end }}

{{/*
Fail with a message if the auth type is set to non-Keycloak and its requirements are not met.
*/}}
{{- if has (include "camundaPlatform.authIssuerType" .) (list "MICROSOFT" "GENERIC") }}
  {{/*
  TODO: Once refactor the auth issuers, we need to add more constraints here to validate the new auth types. 
        More details: https://github.com/camunda/camunda-platform-helm/issues/4419
  */}}
{{- end }}

{{/*
Fail with a message if global.identity.auth.identity secret configuration is set and global.identity.auth.type is set to KEYCLOAK
*/}}

{{- if eq (include "camundaPlatform.hasSecretConfig" (dict "config" .Values.global.identity.auth.identity)) "true" }}
  {{- if eq (include "camundaPlatform.authIssuerType" .) "KEYCLOAK" }}
    {{- $errorMessage := "[camunda][error] global.identity.auth.identity secret configuration does not need to be set when using Keycloak."
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
Fail with a message if Console is enabled but management Identity is not enabled.
*/}}
{{- if and .Values.console.enabled (not .Values.identity.enabled) }}
  {{- $errorMessage := printf "[camunda][error] %s %s"
      "Console is enabled but management Identity is not enabled."
      "Please ensure that if Console is enabled, management Identity must also be enabled."
  -}}
  {{ printf "\n%s" $errorMessage | trimSuffix "\n"| fail }}
{{- end }}

{{/*
Fail with a message if Web Modeler is enabled but management Identity is not enabled.
*/}}
{{- if and .Values.webModeler.enabled (not .Values.identity.enabled) }}
  {{- $errorMessage := printf "[camunda][error] %s %s"
      "Web Modeler is enabled but management Identity is not enabled."
      "Please ensure that if Web Modeler is enabled, management Identity must also be enabled."
  -}}
  {{ printf "\n%s" $errorMessage | trimSuffix "\n"| fail }}
{{- end }}

{{- define "camunda.constraints.warnings" }}
  {{- if .Values.global.testDeprecationFlags.existingSecretsMustBeSet }}
    {{/* TODO: Check if there are more existingSecrets to check */}}

    {{- $existingSecretsNotConfigured := list }}

    {{ if .Values.global.identity.auth.enabled }}
      {{ if and (.Values.connectors.enabled)
                (eq (include "connectors.authMethod" .) "oidc")
                (not .Values.connectors.security.authentication.oidc.secret.existingSecret) }}
        {{- $existingSecretsNotConfigured = append
            $existingSecretsNotConfigured "connectors.security.authentication.oidc.secret.existingSecret" }}
      {{- end }}

      {{ if and (ne (include "camundaPlatform.authIssuerType" .) "KEYCLOAK")
                (.Values.identity.enabled)
                (not .Values.global.identity.auth.identity.secret.existingSecret) }}
        {{- $existingSecretsNotConfigured = append
            $existingSecretsNotConfigured "global.identity.auth.identity.secret.existingSecret" }}
      {{- end }}

      {{- /* Console is a public client and does not require a secret */ -}}

      {{ if and (.Values.orchestration.enabled)
                (eq (include "orchestration.authMethod" .) "oidc")
                (not .Values.orchestration.security.authentication.oidc.secret.existingSecret) }}
        {{- $existingSecretsNotConfigured = append
            $existingSecretsNotConfigured "orchestration.security.authentication.oidc.secret.existingSecret" }}
      {{- end }}
    {{- end }}

  {{/* External Keycloak auth secret must be explicitly configured when using external Keycloak */}}
  {{ if and (.Values.identity.enabled)
            (not .Values.identityKeycloak.enabled)
            (.Values.global.identity.keycloak.auth.adminUser)
            (not .Values.global.identity.keycloak.auth.secret.existingSecret) }}
    {{- $existingSecretsNotConfigured = append
        $existingSecretsNotConfigured "global.identity.keycloak.auth.secret.existingSecret"
    }}
  {{- end }}

  {{ if and (.Values.identityKeycloak.enabled)
            (not .Values.identityKeycloak.auth.existingSecret) }}
    {{- $existingSecretsNotConfigured = append
        $existingSecretsNotConfigured "identityKeycloak.auth.existingSecret"
    }}
  {{- end }}

  {{ if and (.Values.identityKeycloak.postgresql.enabled)
            (not .Values.identityKeycloak.postgresql.auth.existingSecret) }}
    {{- $existingSecretsNotConfigured = append
        $existingSecretsNotConfigured "identityKeycloak.postgresql.auth.existingSecret"
    }}
  {{- end }}

  {{ if and (.Values.webModelerPostgresql.enabled)
            (not .Values.webModelerPostgresql.auth.existingSecret) }}
    {{- $existingSecretsNotConfigured = append
        $existingSecretsNotConfigured "webModelerPostgresql.auth.existingSecret"
    }}
  {{- end }}

  {{ if and (.Values.identityPostgresql.enabled)
            (not .Values.identityPostgresql.auth.existingSecret) }}
    {{- $existingSecretsNotConfigured = append
        $existingSecretsNotConfigured "identityPostgresql.auth.existingSecret"
    }}
  {{- end }}

  {{ if and (.Values.webModeler.enabled)
            (not .Values.webModeler.restapi.pusher.secret.existingSecret) }}
    {{- $existingSecretsNotConfigured = append
        $existingSecretsNotConfigured "webModeler.restapi.pusher.secret.existingSecret"
    }}
  {{- end }}

  {{ if and (.Values.webModeler.enabled)
            (not .Values.webModeler.restapi.pusher.client.secret.existingSecret) }}
    {{- $existingSecretsNotConfigured = append
        $existingSecretsNotConfigured "webModeler.restapi.pusher.client.secret.existingSecret"
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
        {{- $errorMessage = (cat $errorMessage "\n\n" "Please set each parameter above to your Kubernetes Secret name, along with the corresponding .secret.existingSecretKey parameter.\n") }}
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
        {{- $errorMessage = (cat $errorMessage "\n\n" "Please set each parameter above to your Kubernetes Secret name, along with the corresponding .secret.existingSecretKey parameter.\n") }}
        {{ printf "\n%s" $errorMessage | trimSuffix "\n"| fail }}
      {{- end }}
    {{- end }}
  {{- end }}

  {{/* Warn about insecure inlineSecret usage */}}
  {{- $inlineSecretSections := list -}}
  {{- range $k, $v := .Values -}}
    {{- if kindIs "map" $v -}}
      {{- if eq $k "global" -}}
        {{/* Drill into global.* children for finer-grained reporting */}}
        {{- range $gk, $gv := $v -}}
          {{- if and (kindIs "map" $gv) (mustToJson $gv | regexMatch "\"inlineSecret\":\"[^\"]+\"") -}}
            {{- $inlineSecretSections = append $inlineSecretSections (printf "global.%s" $gk) -}}
          {{- end -}}
        {{- end -}}
      {{- else -}}
        {{- if (mustToJson $v | regexMatch "\"inlineSecret\":\"[^\"]+\"") -}}
          {{- $inlineSecretSections = append $inlineSecretSections $k -}}
        {{- end -}}
      {{- end -}}
    {{- end -}}
  {{- end -}}
  {{- if $inlineSecretSections -}}
    {{- $warningMessage := printf "%s %s %s %s"
        "[camunda][warning]"
        (printf "SECURITY: inlineSecret is set in: [%s]." (join ", " $inlineSecretSections))
        "This stores secrets as plain-text in the Helm values and is NOT suitable for production use."
        "For production environments, please use Kubernetes Secrets with 'secret.existingSecret' instead."
    -}}
    {{ printf "\n%s" $warningMessage | trimSuffix "\n" }}
  {{- end }}

  {{/* Bitnami subchart deprecation warnings */}}
  {{- $bitnamiSubchartsEnabled := list -}}
  {{- if .Values.identityPostgresql.enabled -}}
    {{- $bitnamiSubchartsEnabled = append $bitnamiSubchartsEnabled "identityPostgresql" -}}
  {{- end -}}
  {{- if .Values.identityKeycloak.enabled -}}
    {{- $bitnamiSubchartsEnabled = append $bitnamiSubchartsEnabled "identityKeycloak" -}}
  {{- end -}}
  {{- if .Values.webModelerPostgresql.enabled -}}
    {{- $bitnamiSubchartsEnabled = append $bitnamiSubchartsEnabled "webModelerPostgresql" -}}
  {{- end -}}
  {{- if .Values.elasticsearch.enabled -}}
    {{- $bitnamiSubchartsEnabled = append $bitnamiSubchartsEnabled "elasticsearch" -}}
  {{- end -}}
  {{- if $bitnamiSubchartsEnabled }}
    {{- $warningMessage := printf "%s %s %s %s %s"
        "[camunda][warning]"
        "DEPRECATION: The following Bitnami-based subcharts are deprecated and will be removed in Camunda 8.10:"
        (join ", " $bitnamiSubchartsEnabled | printf "[%s].")
        "Please migrate to externally managed services before upgrading to 8.10."
        "For more details: https://docs.camunda.io/self-managed/deployment/helm/operational-tasks/migration-from-bitnami/"
    -}}
    {{ printf "\n%s" $warningMessage | trimSuffix "\n" }}
  {{- end }}

  {{/* global.elasticsearch and global.opensearch config warnings */}}
  {{- $deprecatedDatabaseTlsOptions := list
  (dict "path" "global.elasticsearch.tls.secret" "config" .Values.global.elasticsearch.tls.secret)
  (dict "path" "global.opensearch.tls.secret" "config" .Values.global.opensearch.tls.secret)
  }}
  {{- range $deprecatedDatabaseTlsOptions }}
    {{- if (eq (include "camundaPlatform.hasSecretConfig" (dict "config" .config)) "true") }}
        {{- $warningMessage := printf "%s %s %s %s %s"
            "[camunda][warning]"
            (printf "DEPRECATION: values.yaml is using legacy option '%s'." .path)
            "This option is deprecated and will be removed in a future version."
            (printf "Please migrate to the new option: 'orchestration.data.secondaryStorage.(elasticsearch|opensearch).tls.secret.existingSecret'")
            (printf "or for optimize: 'optimize.database.(elasticsearch|opensearch).tls.secret.existingSecret'")
        -}}
        {{ printf "\n%s" $warningMessage | trimSuffix "\n" }}
    {{- end }}
  {{- end }}

  {{- $deprecatedDatabaseOptions := list
  (dict "path" "global.elasticsearch.enabled" "config" .Values.global.elasticsearch.enabled)
  (dict "path" "global.opensearch.enabled" "config" .Values.global.opensearch.enabled)
  }}
  {{- range $deprecatedDatabaseOptions }}
    {{- if .config }}
        {{- $warningMessage := printf "%s %s %s %s %s"
            "[camunda][warning]"
            (printf "DEPRECATION: values.yaml is using legacy option '%s'." .path)
            "This option is deprecated and will be removed in a future version."
            (printf "Please migrate to the new option: 'orchestration.data.secondaryStorage.(elasticsearch|opensearch).enabled'.")
            (printf "or for optimize: 'optimize.database.(elasticsearch|opensearch).enabled'.")
        -}}
        {{ printf "\n%s" $warningMessage | trimSuffix "\n" }}
    {{- end }}
  {{- end }}

  {{/* global.opensearch.aws.enabled deprecation warning */}}
  {{- if .Values.global.opensearch.aws.enabled }}
    {{- $warningMessage := printf "%s %s %s %s"
        "[camunda][warning]"
        "DEPRECATION: values.yaml is using legacy option 'global.opensearch.aws.enabled'."
        "This option is deprecated, ignored by the chart, and will be removed in a future version."
        "Please configure AWS IRSA via 'orchestration.env' or 'orchestration.extraConfiguration' instead."
    -}}
    {{ printf "\n%s" $warningMessage | trimSuffix "\n" }}
  {{- end }}

  {{/* optimize.database.opensearch.aws.enabled deprecation warning */}}
  {{- if .Values.optimize.database.opensearch.aws.enabled }}
    {{- $warningMessage := printf "%s %s %s %s"
        "[camunda][warning]"
        "DEPRECATION: values.yaml is using legacy option 'optimize.database.opensearch.aws.enabled'."
        "This option is deprecated, ignored by the chart, and will be removed in a future version."
        "Please configure AWS IRSA via 'optimize.env' instead."
    -}}
    {{ printf "\n%s" $warningMessage | trimSuffix "\n" }}
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
        "https://docs.camunda.io/docs/self-managed/deployment/helm/upgrade/"
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
        "https://docs.camunda.io/docs/self-managed/deployment/helm/upgrade/"
    -}}
    {{ printf "\n%s" $errorMessage | trimSuffix "\n"| fail }}
  {{- end }}
{{- end -}}


{{/*
*******************************************************************************
Ingress and Gateway API should not be enabled at the same time.
*******************************************************************************
*/}}
{{- if and .Values.global.gateway.enabled .Values.global.ingress.enabled }}
  {{- $errorMessage := printf "[camunda][error] %s %s"
      "Gateway API and Ingress cannot both be enabled at the same time."
      "Please ensure that either \"global.gateway.enabled: true\" or \"global.ingress.enabled: true\" is set, but not both."
  -}}
  {{ printf "\n%s" $errorMessage | trimSuffix "\n"| fail }}
{{- end }}

{{/*
*******************************************************************************
Camunda 8.9 cycle deprecated keys.
*******************************************************************************
Fail with a message when old values syntax is used.
Chart Version: 14.0.0
*******************************************************************************
*/}}

{{/*
*******************************************************************************
Global
*******************************************************************************
*/}}

{{/*
- removed: global.secrets.autoGenerated
- removed: global.secrets.name
- removed: global.secrets.annotations
*/}}
{{ include "camundaPlatform.keyRemoved" (dict
  "condition" (and .Values.global.secrets .Values.global.secrets.autoGenerated)
  "oldName" "global.secrets.autoGenerated"
) }}

{{ include "camundaPlatform.keyRemoved" (dict
  "condition" (and .Values.global.secrets (hasKey .Values.global.secrets "name"))
  "oldName" "global.secrets.name"
) }}

{{ include "camundaPlatform.keyRemoved" (dict
  "condition" (and .Values.global.secrets (hasKey .Values.global.secrets "annotations"))
  "oldName" "global.secrets.annotations"
) }}

{{/*
*******************************************************************************
Camunda 8.10 cycle deprecated keys.
*******************************************************************************
Keys deprecated during the 8.9 cycle, removed in the 8.10 chart.
Chart Version: 15.0.0
*******************************************************************************
*/}}

{{/*
*******************************************************************************
Global - Ingress
*******************************************************************************
*/}}

{{ include "camundaPlatform.keyRemoved" (dict
  "condition" (hasKey .Values.global.ingress "host")
  "oldName" "global.ingress.host"
) }}

{{/*
*******************************************************************************
Global - Identity Keycloak Auth
*******************************************************************************
*/}}

{{ include "camundaPlatform.keyRemoved" (dict
  "condition" (hasKey .Values.global.identity.keycloak.auth "existingSecret")
  "oldName" "global.identity.keycloak.auth.existingSecret"
) }}

{{ include "camundaPlatform.keyRemoved" (dict
  "condition" (hasKey .Values.global.identity.keycloak.auth "existingSecretKey")
  "oldName" "global.identity.keycloak.auth.existingSecretKey"
) }}

{{/*
*******************************************************************************
Global - Elasticsearch TLS
*******************************************************************************
*/}}

{{ include "camundaPlatform.keyRemoved" (dict
  "condition" (hasKey .Values.global.elasticsearch.tls "existingSecret")
  "oldName" "global.elasticsearch.tls.existingSecret"
) }}

{{/*
*******************************************************************************
Global - OpenSearch TLS
*******************************************************************************
*/}}

{{ include "camundaPlatform.keyRemoved" (dict
  "condition" (hasKey .Values.global.opensearch.tls "existingSecret")
  "oldName" "global.opensearch.tls.existingSecret"
) }}

{{/*
*******************************************************************************
Console
*******************************************************************************
*/}}

{{ include "camundaPlatform.keyRemoved" (dict
  "condition" (hasKey .Values.console "overrideConfiguration")
  "oldName" "console.overrideConfiguration"
) }}

{{ include "camundaPlatform.keyRemoved" (dict
  "condition" (hasKey .Values.console.tls "existingSecret")
  "oldName" "console.tls.existingSecret"
) }}

{{/*
*******************************************************************************
Orchestration
*******************************************************************************
*/}}

{{- if .Values.orchestration.enabled -}}

{{ include "camundaPlatform.keyRemoved" (dict
  "condition" (hasKey .Values.orchestration.profiles "identity")
  "oldName" "orchestration.profiles.identity"
) }}

{{- end }}

{{/*
*******************************************************************************
Web Modeler
*******************************************************************************
*/}}

{{- if .Values.webModeler.enabled -}}

{{ include "camundaPlatform.keyRemoved" (dict
  "condition" (hasKey .Values.webModeler.restapi.externalDatabase "user")
  "oldName" "webModeler.restapi.externalDatabase.user"
) }}

{{- end }}
