{{/*
A template to handle constraints.
*/}}

{{/*
Show a message if Optimize is enabled but identity is disabled
*/}}

{{- if (and (.Values.optimize.enabled) (not .Values.identity.enabled))}}
    {{- $errorMessage := printf "Error %s %s"
        "Optimize may only be used if Identity is enabled."
        "Please set 'identity.enabled' to true or set 'optimize.enabled' to false"
    -}}
    {{ printf "\n%s" $errorMessage | trimSuffix "\n"| fail }}
{{- end }}

