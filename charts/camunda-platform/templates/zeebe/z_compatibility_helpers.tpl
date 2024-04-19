{{/*
NOTE: According to how Helm order the templates, this helper file should be the last file in the Zeebe dir,
      so Helm will process it first, and all migration steps will be applied to all templates in the chart implicitly.
*/}}

{{/*
TODO: Remove after 8.7 cycle.
********************************************************************************
* Camunda 8.5 backward compatibility.

Overview:
    Backward compatibility with values syntax before Camunda Helm chart v10.0.0 (Camunda 8.5 cycle).

Approach:
    Using deep copy and deep merge functions to override new keys using the old key.
    https://helm.sh/docs/chart_template_guide/function_list/#mergeoverwrite-mustmergeoverwrite
********************************************************************************
*/}}


{{/*
Identity.
*/}}
{{- if .Values.identity.keycloak -}}
    {{- $_ := set .Values "identityKeycloak" (deepCopy .Values.identity.keycloak | mergeOverwrite .Values.identityKeycloak) -}}
{{- end -}}

{{- if .Values.identity.postgresql -}}
    {{- $_ := set .Values "identityPostgresql" (deepCopy .Values.identity.postgresql | mergeOverwrite .Values.identityPostgresql) -}}
{{- end -}}


{{/*
Zeebe Gateway.
*/}}

{{- if (index .Values "zeebe-gateway") -}}
    {{- $_ := set .Values "zeebeGateway" (deepCopy (index .Values "zeebe-gateway") | mergeOverwrite .Values.zeebeGateway) -}}
{{- end -}}

{{- if .Values.zeebeGateway.service.gatewayName -}}
    {{- $_ := set .Values.zeebeGateway.service "grpcName" .Values.zeebeGateway.service.gatewayName -}}
{{- end -}}

{{- if .Values.zeebeGateway.service.gatewayPort -}}
    {{- $_ := set .Values.zeebeGateway.service "grpcPort" .Values.zeebeGateway.service.gatewayPort -}}
{{- end -}}

{{- if .Values.zeebeGateway.ingress.enabled -}}
    {{- $zgIngress := omit .Values.zeebeGateway.ingress "rest" -}}
    {{- $_ := set .Values.zeebeGateway.ingress "grpc" (deepCopy $zgIngress | mergeOverwrite .Values.zeebeGateway.ingress.grpc) -}}
{{- end -}}


{{/*
Elasticsearch.

Old:
- "global.elasticsearch.url" is a string (had priority over global.elasticsearch.{protocol, host, port})
- "global.elasticsearch.protocol", "global.elasticsearch.host, "global.elasticsearch.port".

New:
- "global.elasticsearch.url.protocol", "global.elasticsearch.url.host, "global.url.elasticsearch.port".

Notes:
- Helm CLI will show a warning like "cannot overwrite table with non table for", but the old syntax will still work.
*/}}
{{- if or (not .Values.global.elasticsearch.url) (empty .Values.global.elasticsearch.url) -}}
    {{- $esProtocol := .Values.global.elasticsearch.protocol | default "http" -}}
    {{- $esHost := .Values.global.elasticsearch.host | default (print .Release.Name "-elasticsearch") -}}
    {{- $esPort := .Values.global.elasticsearch.port | default "9200" -}}
    {{- $_ := set .Values.global.elasticsearch "url" (dict "protocol" $esProtocol "host" $esHost "port" $esPort) -}}
{{- else if eq (kindOf .Values.global.elasticsearch.url) "string" -}}
    {{- $esURL := urlParse .Values.global.elasticsearch.url -}}
    {{- $esProtocol := $esURL.scheme | default .Values.global.elasticsearch.protocol | default "http" -}}
    {{- $esHost := ($esURL.host | splitList ":" | first) | default .Values.global.elasticsearch.host | default (print .Release.Name "-elasticsearch") -}}
    {{- $esPort := ($esURL.host | splitList ":" | last) | default .Values.global.elasticsearch.port | default "9200" -}}
    {{- $_ := set .Values.global.elasticsearch "url" (dict "protocol" $esProtocol "host" $esHost "port" $esPort) -}}
{{- else }}
    {{- /* Handle unexpected type with a default or error message */}}
    {{- $_ := set .Values.global.elasticsearch "url" (dict "protocol" "http" "host" (print .Release.Name "-elasticsearch") "port" "9200") -}}
    {{- /* Optionally, log a warning or error */}}
    {{/* WARNING: .Values.global.elasticsearch.url is not a string as expected. Using default settings. */}}
{{- end -}}
