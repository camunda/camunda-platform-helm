{{/*
A template to handel constraints.
*/}}

{{/*
Keycloak chart and external Keycloak URL cannot be enabled at the same time.
*/}}
{{- $keycloakFailMessage := `
[identity] Keycloak chart and external Keycloak URL are mutually exclusive.
Either enable the Keycloak chart (identity.keycloak.enabled) OR set the external Keycloak URL (global.identity.keycloak.url).
` -}}
{{- if and .Values.keycloak.enabled .Values.global.identity.keycloak.url }}
    {{ printf "\n%s" $keycloakFailMessage | trimSuffix "\n" | fail }}
{{- end }}
