{{- if .Values.identity.enabled -}}

{{/*
A template to handle constraints.
*/}}

{{/*
Show a deprecation messages for using ".global.identity.keycloak.fullname".
*/}}

{{- if (.Values.global.identity.keycloak.fullname) }}
    {{- $errorMessage := printf "[identity][deprecation] %s %s"
        "The var \"global.identity.keycloak.fullname\" is deprecated in favour of \".global.identity.keycloak.url\"."
        "For more details, please check Camunda Helm chart documentation."
    -}}
    {{ printf "\n%s" $errorMessage | trimSuffix "\n"| fail }}
{{- end }}


{{/*
Show an error message if both internal and external databases are enabled at the same time.
*/}}

{{- if and .Values.identityPostgresql.enabled .Values.identity.externalDatabase.enabled }}
    {{- $errorMessage := printf "[identity][error] %s %s"
        "The values \"identityPostgresql.enabled\" and \"identity.externalDatabase.enabled\""
        "are mutually exclusive and cannot be enabled together. Only use one of either."
    -}}
    {{ printf "\n%s" $errorMessage | trimSuffix "\n"| fail }}
{{- end }}

{{/*
Validate Keycloak configuration when external Keycloak URL or auth is configured.
*/}}
{{- $keycloakFailMessageRaw := `
[identity] To configure Keycloak, you have 3 options:

  - Case 1: If you want to deploy Keycloak chart as it is, then set the following:
    - keycloak.enabled: true

  - Case 2: If you want to customize the Keycloak chart URL, then set the following:
    - keycloak.enabled: true
    - global.identity.keycloak.url.protocol
    - global.identity.keycloak.url.host
    - global.identity.keycloak.url.port

  - Case 3: If you want to use already existing Keycloak, then set the following:
    - keycloak.enabled: false
    - global.identity.keycloak.url.protocol
    - global.identity.keycloak.url.host
    - global.identity.keycloak.url.port
    - global.identity.keycloak.auth.adminUser
    - global.identity.keycloak.auth.secret.existingSecret
    - global.identity.keycloak.auth.secret.existingSecretKey

For more details, please check Camunda Helm chart documentation.
` -}}
{{- $keycloakFailMessage := printf "\n%s" $keycloakFailMessageRaw | trimSuffix "\n" -}}

{{- if .Values.global.identity.keycloak.url -}}
    {{- $_ := required $keycloakFailMessage .Values.global.identity.keycloak.url.protocol -}}
    {{- $_ := required $keycloakFailMessage .Values.global.identity.keycloak.url.host -}}
    {{- $_ := required $keycloakFailMessage .Values.global.identity.keycloak.url.port -}}
{{- end -}}

{{- $hasNewSecret := and .Values.global.identity.keycloak.auth.secret (or .Values.global.identity.keycloak.auth.secret.existingSecret .Values.global.identity.keycloak.auth.secret.inlineSecret) -}}
{{- if or .Values.global.identity.keycloak.auth.adminUser .Values.global.identity.keycloak.auth.existingSecret $hasNewSecret -}}
    {{- $_ := required $keycloakFailMessage .Values.global.identity.keycloak.auth.adminUser -}}
    {{- if not (or .Values.global.identity.keycloak.auth.existingSecret $hasNewSecret) -}}
        {{- fail $keycloakFailMessage -}}
    {{- end -}}
{{- end -}}

{{- end }}
