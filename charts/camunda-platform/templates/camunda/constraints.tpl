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
