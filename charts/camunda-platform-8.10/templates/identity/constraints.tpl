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
Validate Keycloak configuration when external Keycloak URL or auth is configured.
*/}}
{{- $keycloakFailMessageRaw := `
[identity] To configure external Keycloak, set the following:
  - global.identity.keycloak.url.protocol
  - global.identity.keycloak.url.host
  - global.identity.keycloak.url.port
  - global.identity.keycloak.auth.adminUser
  - global.identity.keycloak.auth.secret.existingSecret
  - global.identity.keycloak.auth.secret.existingSecretKey

For more details, please check Camunda Helm chart documentation.
` -}}
{{- $keycloakFailMessage := printf "\n%s" $keycloakFailMessageRaw | trimSuffix "\n" -}}

{{- if or .Values.global.identity.keycloak.url.protocol .Values.global.identity.keycloak.url.host .Values.global.identity.keycloak.url.port -}}
    {{- $_ := required $keycloakFailMessage .Values.global.identity.keycloak.url.protocol -}}
    {{- $_ := required $keycloakFailMessage .Values.global.identity.keycloak.url.host -}}
    {{- $_ := required $keycloakFailMessage .Values.global.identity.keycloak.url.port -}}
{{- end -}}

{{- $hasSecret := and .Values.global.identity.keycloak.auth.secret (or .Values.global.identity.keycloak.auth.secret.existingSecret .Values.global.identity.keycloak.auth.secret.inlineSecret) -}}

{{/*
When Keycloak auth is active (auth enabled, KEYCLOAK type, URL host configured),
the admin user and secret are required so that Identity can perform realm setup.
*/}}
{{- if and .Values.global.identity.auth.enabled (eq .Values.global.identity.auth.type "KEYCLOAK") .Values.global.identity.keycloak.url.host -}}
    {{- $_ := required $keycloakFailMessage .Values.global.identity.keycloak.auth.adminUser -}}
    {{- if not $hasSecret -}}
        {{- fail $keycloakFailMessage -}}
    {{- end -}}
{{- end -}}

{{- if or .Values.global.identity.keycloak.auth.adminUser $hasSecret -}}
    {{- $_ := required $keycloakFailMessage .Values.global.identity.keycloak.auth.adminUser -}}
    {{- if not $hasSecret -}}
        {{- fail $keycloakFailMessage -}}
    {{- end -}}
{{- end -}}

{{- end }}
