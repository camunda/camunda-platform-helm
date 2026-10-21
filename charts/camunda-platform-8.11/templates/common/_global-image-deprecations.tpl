{{- define "camundaPlatform.globalImageDeprecations" -}}
  {{- $components := list "orchestration.image" "connectors.image" "identity.image" "optimize.image" "webModeler.restapi.image" "webModeler.websockets.image" "camundaHub.restapi.image" "camundaHub.websockets.image" -}}
  {{- range $field := list "tag" "repository" -}}
    {{- $paths := $components -}}
    {{- if eq $field "tag" -}}
      {{- $paths = concat $paths (list "webModeler.image" "camundaHub.image") -}}
    {{- end -}}
    {{- $keys := list -}}
    {{- range $path := $paths -}}
      {{- $keys = append $keys (printf "%s.%s" $path $field) -}}
    {{- end -}}
    {{- $url := "https://github.com/camunda/camunda-platform-helm/blob/main/charts/camunda-platform-8.11/README.md#global-image-deprecations" -}}
    {{- if eq $field "tag" -}}
      {{- $url = printf "Deprecated per ADR 0042. Migration: %s" $url -}}
    {{- end -}}
    {{ include "camundaPlatform.keyDeprecated" (dict
      "condition" (hasKey $.Values.global.image $field)
      "oldName" (printf "global.image.%s" $field)
      "migration" (join "\", \"" $keys)
      "url" $url) }}
  {{- end -}}
{{- end -}}
