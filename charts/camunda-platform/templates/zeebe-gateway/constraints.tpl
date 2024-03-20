{{- if .Values.zeebe.enabled -}}

{{/*
A template to handel constraints.
*/}}

{{/*
Fail if "zeebeGateway.ingress" is used and "zeebeGateway.ingress.grpc" is not defined.
*/}}

{{- if and .Values.zeebeGateway.ingress.enabled (not (.Values.zeebeGateway.ingress.grpc.enabled)) }}
    {{- $errorMessage := printf "[zeebe-gateway] %s %s %s"
        "The gRPC Ingress key changed from \"zeebeGateway.ingress\" to \"zeebeGateway.ingress.grpc\"."
        "For more details, please check Camunda Helm chart documentation."
        "https://docs.camunda.io/docs/self-managed/platform-deployment/helm-kubernetes/upgrade/#version-update-instructions"
    -}}
    {{ printf "\n%s" $errorMessage | trimSuffix "\n"| fail }}
{{- end }}

{{- end }}
