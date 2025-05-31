{{- define "currentdeployment.fullname" -}}
    {{- printf "%s-currentdeployment" .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- end -}}