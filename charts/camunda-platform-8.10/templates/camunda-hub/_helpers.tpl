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
    {{- range $key, $srcValue := .src -}}
        {{- /* NOTE: A nil value means "not specified" so chart-declared placeholders fall through to webModeler.*; false, 0 and "" are real overrides. */ -}}
        {{- if kindIs "invalid" $srcValue -}}
        {{- else if and (kindIs "map" $srcValue) (hasKey $dst $key) (kindIs "map" (index $dst $key)) -}}
            {{- include "camundaHub.mergeInto" (dict "dst" (index $dst $key) "src" $srcValue) -}}
        {{- else -}}
            {{- $_ := set $dst $key $srcValue -}}
        {{- end -}}
    {{- end -}}
{{- end -}}
