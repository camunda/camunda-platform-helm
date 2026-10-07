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
      {{- if has $path $.exempt -}}
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
  {{- if not (hasKey . "_camundaUnknownValuesPaths") -}}
  {{- $schema := .Files.Get "values.unknown-keys.schema.json" | mustFromJson -}}
  {{- $found := dict -}}
  {{- $exempt := list "global" "common" "console" "identityKeycloak" "identityPostgresql" "webModelerPostgresql" "elasticsearch" "camundaHub.webModeler" "camundaHub.console" "identity.keycloak" "orchestration.profiles.identity" "webModeler.restapi.externalDatabase.user" "orchestration.security.initialization.defaultRoles" -}}
  {{- include "camundaPlatform.walkUnknownValues" (dict "schema" $schema "root" $schema "value" .Values "path" "" "found" $found "exempt" $exempt) -}}
  {{- $_ := set . "_camundaUnknownValuesPaths" (keys $found | sortAlpha | toJson) -}}
  {{- end -}}
  {{- index . "_camundaUnknownValuesPaths" -}}
{{- end -}}

{{- define "camundaPlatform.unknownValuesWarnings" -}}
  {{- $paths := include "camundaPlatform.unknownValuesPaths" . | fromJsonArray -}}
  {{- if and .Values.global.strictValues $paths -}}
    {{- fail (printf "[camunda][error] Unknown values keys (global.strictValues=true): %s" (join ", " $paths)) -}}
  {{- end -}}
  {{- range $paths -}}
    {{- printf "\n[camunda][warning] UNKNOWN VALUES KEY: %s. Helm ignores this key. Remove or correct it. A future chart version can reject unknown keys." . -}}
  {{- end -}}
{{- end -}}
