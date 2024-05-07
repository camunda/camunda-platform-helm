{{- $release := ds "release" -}}
{{- $releaseHeader := conv.ToBool (getenv "VERSION_MATRIX_RELEASE_HEADER" "true") -}}
{{- if $releaseHeader -}}
<!-- THIS FILE IS AUTO-GENERATED, DO NOT EDIT IT MANUALLY! -->
# Camunda {{ $release.app }} Helm Chart Version Matrix
{{- end }}

{{- range $release.charts }}
{{- /* TODO: Unify charts image once gomplate v4 is released using coll.JQ */ -}}
{{- $vars := dict
  "app_version" $release.app
  "chart_version" .
  "chart_images_camunda" (versionMatrix "--chart-images-camunda" . | strings.Trim "\n")
  "chart_images_non_camunda" (versionMatrix "--chart-images-non-camunda" . | strings.Trim "\n")
  "helm_cli_version" (versionMatrix "--helm-cli-version" (printf "camunda-platform-%s" .) | strings.Trim " ")
}}

{{- $helmCLIVersion := ternary
  "N/A"
  (printf "[%s](https://github.com/helm/helm/releases/tag/v%s)" $vars.helm_cli_version $vars.helm_cli_version)
  (eq $vars.helm_cli_version "")
}}

{{- if $releaseHeader -}}
{{ "\n" }}
{{ printf "## Helm chart %s" $vars.chart_version }}
{{ "\n" }}
{{- end }}

{{- with $vars -}}
Supported versions:

- Camunda applications: [{{ .app_version }}](https://github.com/camunda/camunda-platform/releases?q=tag%3A{{ .app_version }}&expanded=true)
- Helm values: [{{ .chart_version }}](https://artifacthub.io/packages/helm/camunda/camunda-platform/{{ .chart_version }}#parameters)
- Helm CLI: {{ $helmCLIVersion }}

Camunda images:

{{ .chart_images_camunda }}

Non-Camunda images:

{{ .chart_images_non_camunda }}
{{ end }}

{{- end -}}
