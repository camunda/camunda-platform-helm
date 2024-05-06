<!-- THIS FILE IS AUTO-GENERATED, DO NOT EDIT IT MANUALLY! -->
# Camunda 8 Helm Chart Version Matrix
{{ range (ds "versions") }}
{{- $appVersion := .app }}
## Camunda {{ $appVersion }}

{{ range .charts -}}
### [Helm chart {{ . }}](./camunda-{{ $appVersion }}/#helm-chart-{{ . | regexp.Replace "\\."  "" }})
{{ end -}}

{{ end -}}
