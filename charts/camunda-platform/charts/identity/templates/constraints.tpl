{{/*
A template to handel constraints.
*/}}

{{/*
Show deprecation messages for using ".global.identity.keycloak.fullname".
*/}}

{{- if (.Values.global.identity.keycloak.fullname) }}
{{- $errorMessage := `
[identity][deprecation] The var ".global.identity.keycloak.fullname" is deprecated in favour of
".global.identity.keycloak.url".
For more details, please check Camunda Platform Helm chart documentation.
` -}}
    {{ printf "\n%s" $errorMessage | trimSuffix "\n"| fail }}
{{- end }}

{{/*
Show error message if both "identity.postgresql.enabled" and "identity.externalDatabase.enabled" are true.
*/}}

{{- if and .Values.postgresql.enabled .Values.externalDatabase.enabled }}
{{- $errorMessage := `
[identity][error] The values "identity.postgresql.enabled" and "identity.externalDatabase.enabled"
are mutually exclusive and cannot be enabled together. Only use one of either.
` -}}
    {{ printf "\n%s" $errorMessage | trimSuffix "\n"| fail }}
{{- end }}
