{{- if .Values.identity.enabled -}}

{{/*
A template to handel constraints.
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

{{- end }}
