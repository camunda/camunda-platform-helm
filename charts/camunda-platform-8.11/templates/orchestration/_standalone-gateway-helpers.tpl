{{/*
********************************************************************************
Standalone gateway.
********************************************************************************
*/}}

{{- define "orchestration.standaloneGatewayEnabled" -}}
{{- if and (eq (include "camundaPlatform.orchestrationEnabled" .) "true") .Values.orchestration.standaloneGateway.enabled -}}
true
{{- else -}}
false
{{- end -}}
{{- end -}}

{{- define "orchestration.standaloneGatewayName" -}}
zeebe-standalone-gateway
{{- end -}}

{{- define "orchestration.standaloneGatewayFullname" -}}
{{- printf "%s-standalone-gateway" (include "orchestration.fullname" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "orchestration.standaloneGatewayServiceName" -}}
{{- include "orchestration.standaloneGatewayFullname" . -}}
{{- end -}}

{{- define "orchestration.standaloneGatewayMatchLabels" -}}
{{ include "camundaPlatform.matchLabels" . }}
app.kubernetes.io/component: {{ include "orchestration.standaloneGatewayName" . }}
{{- end -}}

{{- define "orchestration.standaloneGatewayLabels" -}}
{{ include "camundaPlatform.labels" . }}
app.kubernetes.io/component: {{ include "orchestration.standaloneGatewayName" . }}
{{ include "orchestration.versionLabel" . }}
{{- end -}}

{{- define "orchestration.standaloneGatewayServiceAccountName" -}}
{{- $sa := .Values.orchestration.standaloneGateway.serviceAccount -}}
{{- if $sa.enabled -}}
{{- default (include "orchestration.standaloneGatewayFullname" .) $sa.name -}}
{{- else -}}
{{- default (include "orchestration.serviceAccountName" .) $sa.name -}}
{{- end -}}
{{- end -}}

{{/*
[orchestration] Service that API clients (gRPC and REST) should target: the standalone gateway
Service when it is enabled, the Orchestration Cluster Service otherwise.
*/}}
{{- define "orchestration.clientServiceName" -}}
{{- if eq (include "orchestration.standaloneGatewayEnabled" .) "true" -}}
{{- include "orchestration.standaloneGatewayServiceName" . -}}
{{- else -}}
{{- include "orchestration.serviceName" . -}}
{{- end -}}
{{- end -}}

{{- define "orchestration.standaloneGatewayProfiles" -}}
{{- if eq (include "orchestration.authEnabled" .) "true" -}}
gateway,standalone,consolidated-auth
{{- else -}}
gateway,standalone
{{- end -}}
{{- end -}}

{{- define "orchestration.standaloneGatewayRESTIngressEnabled" -}}
{{- $ingress := .Values.orchestration.standaloneGateway.ingress.rest -}}
{{- if and (eq (include "orchestration.standaloneGatewayEnabled" .) "true") $ingress.enabled (not $ingress.external) -}}
true
{{- else -}}
false
{{- end -}}
{{- end -}}

{{- define "orchestration.standaloneGatewayRESTRouteEnabled" -}}
{{- if and (eq (include "orchestration.standaloneGatewayEnabled" .) "true") .Values.global.gateway.enabled (not .Values.global.gateway.external) .Values.orchestration.standaloneGateway.gateway.rest.enabled -}}
true
{{- else -}}
false
{{- end -}}
{{- end -}}

{{/*
[orchestration] External URL of the standalone gateway REST API entry point, empty unless one is configured.
*/}}
{{- define "orchestration.standaloneGatewayRESTExternalURL" -}}
{{- $gw := .Values.orchestration.standaloneGateway -}}
{{- if and (eq (include "orchestration.standaloneGatewayEnabled" .) "true") .Values.global.gateway.enabled $gw.gateway.rest.enabled (tpl $gw.gateway.rest.host .) -}}
{{- printf "%s%s" (include "camundaPlatform.gatewayExternalURL" (dict "context" . "host" $gw.gateway.rest.host)) (include "camundaPlatform.joinpath" (list .Values.orchestration.contextPath)) -}}
{{- else if and (eq (include "orchestration.standaloneGatewayEnabled" .) "true") $gw.ingress.rest.enabled (tpl $gw.ingress.rest.host .) -}}
{{- printf "%s%s" (include "camundaPlatform.ingressExternalURL" (dict "context" . "host" $gw.ingress.rest.host "tlsEnabled" $gw.ingress.rest.tls.enabled)) (include "camundaPlatform.joinpath" (list .Values.orchestration.contextPath)) -}}
{{- end -}}
{{- end -}}

{{/*
[orchestration] Renders a standalone gateway manifest with a context whose orchestration env,
envFrom and extraConfiguration are extended with the standalone gateway entries, and whose
configuration is the standalone gateway one. Shared helpers then resolve TLS, contact points and
secret references the same way for the gateway as for the brokers.

Usage:
{{ include "orchestration.renderStandaloneGateway" (dict "manifest" "orchestration.standaloneGatewayDeploymentManifest" "context" .) }}
*/}}
{{- define "orchestration.renderStandaloneGateway" -}}
{{- $root := .context -}}
{{- $ctx := dict
    "Values" (deepCopy $root.Values)
    "Release" $root.Release
    "Chart" $root.Chart
    "Capabilities" $root.Capabilities
    "Template" $root.Template
    "Files" $root.Files
    "Subcharts" $root.Subcharts
-}}
{{- $orchestration := $ctx.Values.orchestration -}}
{{- $gw := $orchestration.standaloneGateway -}}
{{- $_ := set $orchestration "env" (concat ($orchestration.env | default list) ($gw.env | default list)) -}}
{{- $_ := set $orchestration "envFrom" (concat ($orchestration.envFrom | default list) ($gw.envFrom | default list)) -}}
{{- $_ := set $orchestration "envFromProvides" (concat ($orchestration.envFromProvides | default list) ($gw.envFromProvides | default list)) -}}
{{- $baseExtra := $orchestration.extraConfiguration -}}
{{- $gwExtra := $gw.extraConfiguration -}}
{{- if not $gwExtra -}}
{{- else if not $baseExtra -}}
  {{- $_ := set $orchestration "extraConfiguration" $gwExtra -}}
{{- else if and (kindIs "slice" $baseExtra) (kindIs "slice" $gwExtra) -}}
  {{- $_ := set $orchestration "extraConfiguration" (concat $baseExtra $gwExtra) -}}
{{- else if and (kindIs "map" $baseExtra) (kindIs "map" $gwExtra) -}}
  {{- $_ := set $orchestration "extraConfiguration" (mergeOverwrite (deepCopy $baseExtra) $gwExtra) -}}
{{- else -}}
  {{- fail "[camunda][error] orchestration.extraConfiguration and orchestration.standaloneGateway.extraConfiguration must use the same format (both lists or both maps)" -}}
{{- end -}}
{{- $_ := set $orchestration "configuration" ($gw.configuration | default "") -}}
{{- $_ := set $orchestration "javaOpts" $gw.javaOpts -}}
{{- $_ := set $ctx "OrchestrationRender" (dict "scope" "current" "zone" "") -}}
{{- $_ := set $ctx "StandaloneGatewayRender" true -}}
{{- include .manifest $ctx -}}
{{- end -}}

{{- define "orchestration.standaloneGatewayConfigmapManifest" -}}
kind: ConfigMap
metadata:
  name: {{ include "orchestration.standaloneGatewayFullname" . }}-configuration
  labels:
    {{- include "orchestration.standaloneGatewayLabels" . | nindent 4 }}
apiVersion: v1
data:
  {{- if .Values.orchestration.configuration }}
  application.yaml: |
    {{ .Values.orchestration.configuration | indent 4 | trim }}
  {{- else }}
  application.yaml: |
    {{- (include (print $.Template.BasePath "/orchestration/files/_application.yaml") $) | indent 4 }}
  {{- end }}
  {{- if and .Values.orchestration.log4j2 (ne (include "camundaPlatform.extraConfigHasFile" (dict "extraConfiguration" .Values.orchestration.extraConfiguration "file" "log4j2.xml")) "true") }}
  log4j2.xml: |
    {{ .Values.orchestration.log4j2 | indent 4 | trim }}
  {{- end }}

  {{- include "camundaPlatform.renderExtraConfiguration" (dict "extraConfig" .Values.orchestration.extraConfiguration) }}
{{- end -}}

{{- define "orchestration.standaloneGatewayConfigChecksum" -}}
{{- include "orchestration.standaloneGatewayConfigmapManifest" . | sha256sum -}}
{{- end -}}
