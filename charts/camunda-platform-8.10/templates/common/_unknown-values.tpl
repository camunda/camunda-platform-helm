{{- define "camundaPlatform.walkUnknownValues" -}}
  {{- $schema := .schema -}}
  {{- if hasKey $schema "$ref" -}}
    {{- $ref := index $schema "$ref" -}}
    {{- if hasPrefix "#/" $ref -}}
      {{- $schema = .root -}}
      {{- range splitList "/" (trimPrefix "#/" $ref) -}}
        {{- $schema = index $schema (replace "~0" "~" (replace "~1" "/" .)) | default dict -}}
      {{- end -}}
    {{- else -}}
      {{- $schema = dict -}}
    {{- end -}}
  {{- end -}}
  {{- if kindIs "map" .value -}}
    {{- $properties := $schema.properties | default dict -}}
    {{- range $key, $value := .value -}}
      {{- $path := trimPrefix "." (printf "%s.%s" $.path $key) -}}
      {{- if or (has $path $.exempt) (and (hasKey $properties $key) (has $key (list "annotations" "podAnnotations" "labels" "podLabels" "commonLabels" "nodeSelector"))) -}}
      {{- else if hasKey $properties $key -}}
        {{- include "camundaPlatform.walkUnknownValues" (dict "schema" (index $properties $key) "root" $.root "value" $value "path" $path "found" $.found "exempt" $.exempt) -}}
      {{- else if kindIs "map" $schema.additionalProperties -}}
        {{- include "camundaPlatform.walkUnknownValues" (dict "schema" $schema.additionalProperties "root" $.root "value" $value "path" $path "found" $.found "exempt" $.exempt) -}}
      {{- else if and $properties (ne (toJson $schema.additionalProperties) "true") -}}
        {{- $_ := set $.found $path true -}}
      {{- end -}}
    {{- end -}}
  {{- else if and (kindIs "slice" .value) (kindIs "map" $schema.items) -}}
    {{- range $index, $value := .value -}}
      {{- include "camundaPlatform.walkUnknownValues" (dict "schema" $schema.items "root" $.root "value" $value "path" (printf "%s[%d]" $.path $index) "found" $.found "exempt" $.exempt) -}}
    {{- end -}}
  {{- end -}}
{{- end -}}

{{- define "camundaPlatform.unknownValuesPaths" -}}
  {{- $schema := .Files.Get "values.unknown-keys.schema.json" | mustFromJson -}}
  {{- $found := dict -}}
  {{- /* NOTE: These namespaces are shared, free-form, or handled by constraints.tpl migration guards. */ -}}
  {{- $exempt := list "global" "common" "console" "identityKeycloak" "identityPostgresql" "webModelerPostgresql" "elasticsearch" "camundaHub.webModeler" "camundaHub.console" "identity.keycloak" "orchestration.profiles.identity" "webModeler.restapi.externalDatabase.user" "orchestration.security.initialization.defaultRoles" -}}
  {{- include "camundaPlatform.walkUnknownValues" (dict "schema" $schema "root" $schema "value" .Values "path" "" "found" $found "exempt" $exempt) -}}
  {{- keys $found | sortAlpha | toJson -}}
{{- end -}}

{{- define "camundaPlatform.unknownValuesWarnings" -}}
  {{- range (include "camundaPlatform.unknownValuesPaths" . | fromJsonArray) -}}
    {{- printf "\n[camunda][warning] UNKNOWN VALUES KEY: %s. Remove or correct this key; chart-owned unknown keys will be rejected in Camunda 8.11 (chart v16)." . -}}
  {{- end -}}
{{- end -}}
