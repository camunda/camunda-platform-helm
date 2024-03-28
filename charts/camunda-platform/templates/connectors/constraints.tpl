{{/*
A template to handle constraints.
*/}}
{{/*Pre-validate that inbound mode contains correct values*/}}
{{- $inboundMode := .Values.connectors.inbound.mode -}}
{{- if not (has $inboundMode (list "disabled" "credentials" "oauth" "enabled")) }}
  {{ fail "Not supported inbound mode" }}
{{- end -}}
