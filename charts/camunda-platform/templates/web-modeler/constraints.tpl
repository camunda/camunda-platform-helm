{{- if .Values.webModeler.enabled -}}

{{/*
A template to handel constraints.
*/}}

{{/*
Fail with a message if the old refactored keys are still used and the new keys are not used.
Chart Version: 10.0.0
*/}}

{{- if (.Values.postgresql) }}
    {{- $errorMessage := printf "[web-modeler][error] %s %s %s"
        "The PostgreSQL key changed from \"postgresql\" to \"webModelerPostgresql\"."
        "For more details, please check Camunda Helm chart documentation."
        "https://docs.camunda.io/docs/self-managed/platform-deployment/helm-kubernetes/upgrade/#version-update-instructions"
    -}}
    {{ printf "\n%s" $errorMessage | trimSuffix "\n"| fail }}
{{- end }}

{{- end }}
