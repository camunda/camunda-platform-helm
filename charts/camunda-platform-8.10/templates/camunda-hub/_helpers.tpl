{{/* vim: set filetype=mustache: */}}

{{/*
Camunda Hub helpers.

Enablement gates live in templates/common/_helpers.tpl ("camundaHub.webModelerEnabled",
"camundaHub.consoleEnabled") and are driven by global.topology.mode.
*/}}

{{- define "camundaHub.values" -}}
    {{- $merged := deepCopy (.Values.webModeler | default dict) -}}
    {{- include "camundaHub.mergeInto" (dict "dst" $merged "src" (.Values.camundaHub | default dict)) -}}
    {{- toYaml $merged -}}
{{- end -}}

{{- define "camundaHub.mergeInto" -}}
    {{- /* NOTE: Mutates .dst in place and emits nothing; callers read the mutated map, not the return value. */ -}}
    {{- $dst := .dst -}}
    {{- $path := .path | default "camundaHub" -}}
    {{- range $key, $srcValue := .src -}}
        {{- $dstValue := index $dst $key -}}
        {{- $dstIsMap := and (hasKey $dst $key) (kindIs "map" $dstValue) -}}
        {{- $dstIsSet := and (hasKey $dst $key) (not (kindIs "invalid" $dstValue)) -}}
        {{- /* NOTE: A nil value means "not specified" so chart-declared placeholders fall through to webModeler.*; false, 0 and "" are real overrides. */ -}}
        {{- if kindIs "invalid" $srcValue -}}
        {{- else if and (kindIs "map" $srcValue) $dstIsMap -}}
            {{- include "camundaHub.mergeInto" (dict "dst" $dstValue "src" $srcValue "path" (printf "%s.%s" $path $key)) -}}
        {{- else if and $dstIsSet (ne (kindIs "map" $srcValue) $dstIsMap) -}}
            {{- fail (printf "[camunda][error] %s.%s is a %s but the corresponding webModeler value is a %s; these types cannot be merged. Set %s.%s to a %s, or set its nested keys individually."
                $path $key (kindOf $srcValue) (kindOf $dstValue) $path $key (kindOf $dstValue)) -}}
        {{- else -}}
            {{- $_ := set $dst $key $srcValue -}}
        {{- end -}}
    {{- end -}}
{{- end -}}
