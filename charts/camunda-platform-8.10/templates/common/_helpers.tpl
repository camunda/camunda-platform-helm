{{/* vim: set filetype=mustache: */}}

{{/*
********************************************************************************
General.
********************************************************************************
*/}}

{{/*
Expand the name of the chart.
*/}}
{{- define "camundaPlatform.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (for example,
by the DNS naming spec). If release name contains chart name it will be used as a full name.
*/}}
{{- define "camundaPlatform.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{/*
[camunda-platform] Create a default fully qualified app name for component.

Example:
{{ include "camundaPlatform.componentFullname" (dict "componentName" "foo" "componentValues" .Values.foo "context" $) }}
*/}}
{{- define "camundaPlatform.componentFullname" -}}
    {{- if (.componentValues).fullnameOverride -}}
        {{- .componentValues.fullnameOverride | trunc 63 | trimSuffix "-" -}}
    {{- else -}}
        {{- $name := default .componentName (.componentValues).nameOverride -}}
        {{- if contains $name .context.Release.Name -}}
            {{- .context.Release.Name | trunc 63 | trimSuffix "-" -}}
        {{- else -}}
            {{- printf "%s-%s" .context.Release.Name $name | trunc 63 | trimSuffix "-" -}}
        {{- end -}}
    {{- end -}}
{{- end -}}

{{/*
Define common labels, combining the match labels and transient labels, which might change on updating
(version depending). These labels should not be used on matchLabels selector, since the selectors are immutable.
*/}}
{{- define "camundaPlatform.labels" -}}
{{- template "camundaPlatform.matchLabels" . }}
{{- if .Values.global.commonLabels }}
{{ tpl (toYaml .Values.global.commonLabels) $ }}
{{- end }}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version | replace "+" "_" }}
{{- end }}

{{/*
Common match labels, which are extended by sub-charts and should be used in matchLabels selectors.
*/}}
{{- define "camundaPlatform.matchLabels" -}}
{{- if .Values.global.labels -}}
{{ toYaml .Values.global.labels }}
{{- end }}
app.kubernetes.io/name: {{ template "camundaPlatform.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: camunda-platform
{{- end -}}

{{/*
[camunda-platform] Defines extra labels for a component (component name + version).

Usage:
{{ include "camundaPlatform.componentExtraLabels" (dict "componentName" "connectors" "componentValuesKey" "connectors" "context" $) }}
*/}}
{{- define "camundaPlatform.componentExtraLabels" -}}
app.kubernetes.io/component: {{ .componentName }}
app.kubernetes.io/version: {{ include "camundaPlatform.versionLabel" (dict "base" .context.Values.global "overlay" (index .context.Values .componentValuesKey) "chart" .context.Chart) | quote }}
{{- end -}}

{{/*
[camunda-platform] Define common labels for a component, combining the platform labels and component extra labels.
These labels shouldn't be used on matchLabels selector, since the selectors are immutable.

Usage:
{{ include "camundaPlatform.componentLabels" (dict "componentName" "connectors" "componentValuesKey" "connectors" "context" $) }}
*/}}
{{- define "camundaPlatform.componentLabels" -}}
    {{- include "camundaPlatform.labels" .context }}
    {{- "\n" }}
    {{- include "camundaPlatform.componentExtraLabels" . }}
{{- end -}}

{{/*
[camunda-platform] Defines match labels for a component, which should be used in matchLabels selectors.

Usage:
{{ include "camundaPlatform.componentMatchLabels" (dict "componentName" "connectors" "context" $) }}
*/}}
{{- define "camundaPlatform.componentMatchLabels" -}}
    {{- include "camundaPlatform.matchLabels" .context }}
    {{- "\n" -}}
app.kubernetes.io/component: {{ .componentName }}
{{- end -}}

{{/*
Get image tag according the values of "base" or "overlay" values.
If the "overlay" values exist, they will override the "base" values, otherwise the "base" values will be used.
Usage: {{ include "camundaPlatform.imageTagByParams" (dict "base" .Values.global "overlay" .Values.console) }}
*/}}
{{- define "camundaPlatform.imageTagByParams" -}}
    {{- .overlay.image.tag | default .base.image.tag -}}
{{- end -}}

{{/*
Get image according the values of "base" or "overlay" values.
If the "overlay" values exist, they will override the "base" values, otherwise the "base" values will be used.
Usage: {{ include "camundaPlatform.imageByParams" (dict "base" .Values.global "overlay" .Values.console) }}
*/}}
{{- define "camundaPlatform.imageByParams" -}}
  {{- $imageRegistry    := .overlay.image.registry | default .base.image.registry -}}
  {{- $imageRepository   := .overlay.image.repository | default .base.image.repository -}}
  {{- $imageDigest := .overlay.image.digest | default .base.image.digest | default "" -}}

  {{- if $imageDigest }}
    {{- /* digest‐override path */ -}}
    {{- printf "%s%s%s@%s"
        $imageRegistry
        (empty $imageRegistry | ternary "" "/")
        $imageRepository
        $imageDigest
    -}}
  {{- else }}
    {{- /* original tag path */ -}}
    {{- printf "%s%s%s:%s"
        $imageRegistry
        (empty $imageRegistry | ternary "" "/")
        $imageRepository
        (include "camundaPlatform.imageTagByParams" (dict "base" .base "overlay" .overlay))
    -}}
  {{- end }}
{{- end -}}

{{/*
Return the version label for resources.
If an image digest is specified without a tag, fall back to .Chart.AppVersion (e.g., "8.8.x"); otherwise use the resolved image tag.
*/}}
{{- define "camundaPlatform.versionLabel" -}}
  {{- $imageTag := include "camundaPlatform.imageTagByParams" (dict "base" .base "overlay" .overlay) -}}
  {{- $imageDigest := .overlay.image.digest | default .base.image.digest -}}
  {{- if $imageDigest }}
    {{- /* Using digest: fall back to application version for label */ -}}
    {{- .chart.AppVersion -}}
  {{- else if $imageTag }}
    {{- /* Using tag: use the tag for the label */ -}}
    {{- $imageTag -}}
  {{- else }}
    {{- /* Neither tag nor digest provided: use appVersion as default */ -}}
    {{- .chart.AppVersion -}}
  {{- end -}}
{{- end -}}

{{/*
Get imagePullSecrets according the values of global, component, or empty.
*/}}
{{- define "camundaPlatform.componentImagePullSecrets" -}}
    {{- if (.Values.image.pullSecrets) -}}
        {{- .Values.image.pullSecrets | toYaml -}}
    {{- else if (.Values.global.image.pullSecrets) -}}
        {{- .Values.global.image.pullSecrets | toYaml -}}
    {{- else -}}
        {{- "[]" -}}
    {{- end -}}
{{- end -}}

{{/*
Get imagePullSecrets for top-level components.
Usage:
{{ include "camundaPlatform.imagePullSecrets" (dict "component" "zeebe" "context" $) }}
*/}}
{{- define "camundaPlatform.imagePullSecrets" -}}
    {{- $componentValue := (index $.context.Values .component "image" "pullSecrets") -}}
    {{- $globalValue := (index $.context.Values.global "image" "pullSecrets") -}}
    {{- $componentValue | default $globalValue | default list | toYaml -}}
{{- end -}}


{{/*
[camunda-platform] Create labels for secrets shared between Identity and other components.
*/}}
{{- define "camundaPlatform.identityLabels" -}}
{{- if .Values.global.labels -}}
{{ toYaml .Values.global.labels }}
{{- end }}
app.kubernetes.io/name: identity
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: camunda-platform
helm.sh/chart: identity-{{ .Chart.Version | replace "+" "_" }}
app.kubernetes.io/component: identity
{{- end }}

{{/*
[camunda-platform] Create the name of the service account to use
Usage: {{ include "camundaPlatform.serviceAccountName" (dict "component" "operate" "context" $) }}
*/}}
{{- define "camundaPlatform.serviceAccountName" -}}
    {{- $values := (index .context.Values .component) -}}
    {{- if $values.serviceAccount.enabled -}}
        {{- $values.serviceAccount.name | default (include (printf "%s.fullname" .component) .context) -}}
    {{- else -}}
        {{- $values.serviceAccount.name | default "default" -}}
    {{- end -}}
{{- end -}}

{{/*
********************************************************************************
Authentication.
********************************************************************************
*/}}

{{/*
[camunda-platform] Auth issuer public URL which used externally for Camunda apps (with a fallback to publicIssuerUrl).
*/}}
{{- define "camundaPlatform.authIssuerUrlWithFallback" -}}
  {{- if .Values.global.identity.auth.issuer -}}
    {{- tpl .Values.global.identity.auth.issuer . -}}
  {{- else -}}
    {{- tpl .Values.global.identity.auth.publicIssuerUrl . -}}
  {{- end -}}
{{- end -}}

{{/*
[camunda-platform] Auth issuer public URL which used externally for Camunda apps.
*/}}
{{- define "camundaPlatform.authIssuerUrl" -}}
  {{- tpl .Values.global.identity.auth.issuer . -}}
{{- end -}}

{{/*
[camunda-platform] Auth issuer backend URL which used internally for Camunda apps.
*/}}
{{- define "camundaPlatform.authIssuerBackendUrl" -}}
  {{- if .Values.global.identity.auth.issuerBackendUrl -}}
    {{- tpl .Values.global.identity.auth.issuerBackendUrl . -}}
  {{- else if and (eq (include "camundaPlatform.authIssuerType" .) "KEYCLOAK") (.Values.global.identity.keycloak.url).host -}}
    {{-
      printf "%s://%s:%v%s"
        .Values.global.identity.keycloak.url.protocol
        (include "identity.keycloak.host" .)
        .Values.global.identity.keycloak.url.port
        (include "camundaPlatform.joinpath" (list .Values.global.identity.keycloak.contextPath .Values.global.identity.keycloak.realm))
    -}}
  {{- end -}}
{{- end -}}

{{/*
[camunda-platform] Auth type which used internally for Camunda apps.
NOTE: This is for Management Identity config, all new types will be supported via OIDC.
*/}}
{{- define "camundaPlatform.authIssuerType" -}}
  {{- upper .Values.global.identity.auth.type -}}
{{- end -}}

{{/*
[camunda-platform] Auth URL which used externally by the user.
*/}}
{{- define "camundaPlatform.authIssuerUrlEndpointAuth" -}}
  {{- if or .Values.global.identity.auth.authUrl -}}
    {{- tpl .Values.global.identity.auth.authUrl . -}}
  {{- else if eq (include "camundaPlatform.authIssuerType" .) "KEYCLOAK" -}}
    {{- include "camundaPlatform.authIssuerUrlWithFallback" . -}}/protocol/openid-connect/auth
  {{- end -}}
{{- end -}}

{{/*
[camunda-platform] Auth token URL which used internally for Camunda apps.
*/}}
{{- define "camundaPlatform.authIssuerBackendUrlEndpointToken" -}}
  {{- if .Values.global.identity.auth.tokenUrl -}}
    {{- tpl .Values.global.identity.auth.tokenUrl . -}}
  {{- else if eq (include "camundaPlatform.authIssuerType" .) "KEYCLOAK" -}}
    {{- include "camundaPlatform.authIssuerBackendUrl" . -}}/protocol/openid-connect/token
  {{- end -}}
{{- end -}}

{{/*
[camunda-platform] Auth certs URL which used internally for Camunda apps.
*/}}
{{- define "camundaPlatform.authIssuerBackendUrlEndpointCerts" -}}
  {{- if .Values.global.identity.auth.jwksUrl -}}
    {{- tpl .Values.global.identity.auth.jwksUrl . -}}
  {{- else if eq (include "camundaPlatform.authIssuerType" .) "KEYCLOAK" -}}
    {{- include "camundaPlatform.authIssuerBackendUrl" . -}}/protocol/openid-connect/certs
  {{- end -}}
{{- end -}}

{{/*
Get the externally-reachable Keycloak URL for display in Console and NOTES.txt.
When the chart proxies Keycloak through its combined Ingress (internal: true), use
the chart host + contextPath. Otherwise return the configured external Keycloak URL.
*/}}
{{- define "camundaPlatform.keycloakExternalURL" -}}
  {{- if .Values.global.identity.keycloak.internal -}}
    {{- $proto := ternary "https" "http" .Values.global.ingress.tls.enabled -}}
    {{- printf "%s://%s%s" $proto ((tpl .Values.global.host $) | default "localhost:18080") .Values.global.identity.keycloak.contextPath -}}
  {{- else if (.Values.global.identity.keycloak.url).host -}}
    {{- include "identity.keycloak.url" . -}}
  {{- end -}}
{{- end -}}


{{/*
********************************************************************************
Elasticsearch and Opensearch templates.
********************************************************************************
*/}}

{{/*
[camunda-platform] Elasticsearch URL which could be external.
*/}}

{{- define "camundaPlatform.elasticsearchHost" -}}
  {{- tpl .Values.optimize.database.elasticsearch.url.host $ | default (tpl .Values.global.elasticsearch.url.host $) -}}
{{- end -}}

{{/*
[camunda-platform] Elasticsearch port
*/}}
{{- define "camundaPlatform.elasticsearchPort" -}}
{{- if ne (int .Values.optimize.database.elasticsearch.url.port) 0 -}}
  {{ .Values.optimize.database.elasticsearch.url.port }}
{{- else -}}
  {{ .Values.global.elasticsearch.url.port }}
{{- end -}}
{{- end -}}

{{- define "camundaPlatform.elasticsearchURL" -}}
{{- if .Values.orchestration.data.secondaryStorage.elasticsearch.url -}}
  {{ .Values.orchestration.data.secondaryStorage.elasticsearch.url }}
{{- else -}}
  {{ .Values.optimize.database.elasticsearch.url.protocol | default .Values.global.elasticsearch.url.protocol }}://{{ include "camundaPlatform.elasticsearchHost" . }}:{{ include "camundaPlatform.elasticsearchPort" . }}
{{- end -}}
{{- end -}}

{{- define "camundaPlatform.opensearchHost" -}}
  {{- tpl .Values.optimize.database.opensearch.url.host $ | default (tpl .Values.global.opensearch.url.host $) -}}
{{- end -}}

{{/*
[camunda-platform] Opensearch port
*/}}
{{- define "camundaPlatform.opensearchPort" -}}
{{- if ne (int .Values.optimize.database.opensearch.url.port) 0 -}}
  {{ .Values.optimize.database.opensearch.url.port }}
{{- else -}}
  {{ .Values.global.opensearch.url.port }}
{{- end -}}
{{- end -}}

{{- define "camundaPlatform.opensearchURL" -}}
{{- if .Values.orchestration.data.secondaryStorage.opensearch.url -}}
  {{ .Values.orchestration.data.secondaryStorage.opensearch.url }}
{{- else -}}
  {{ .Values.optimize.database.opensearch.url.protocol | default .Values.global.opensearch.url.protocol }}://{{ include "camundaPlatform.opensearchHost" . }}:{{ include "camundaPlatform.opensearchPort" . }}
{{- end -}}
{{- end -}}




{{/*
********************************************************************************
Operate templates.
********************************************************************************
*/}}

{{/*
Get the external url for a given component.
If the "overlay" values exist, they will override the "base" values, otherwise the "base" values will be used.
Usage: {{ include "camundaPlatform.getExternalURL" (dict "component" "identity" "context" .) }}
*/}}
{{- define "camundaPlatform.getExternalURL" -}}
  {{- if (index .context.Values .component "enabled") -}}
    {{- if $.context.Values.global.ingress.enabled -}}
      {{ $proto := ternary "https" "http" .context.Values.global.ingress.tls.enabled -}}
      {{- printf "%s://%s%s" $proto (tpl .context.Values.global.host .context) (index .context.Values .component "contextPath") -}}
    {{- else -}}
      {{- $portMapping := (dict
      "identity" "8080"
      "optimize" "8083"
      "connectors" "8086"
      ) -}}
      {{- printf "http://localhost:%s" (get $portMapping .component) -}}
    {{- end -}}
  {{- end -}}
{{- end -}}

{{/*
[camunda-platform] Operate external URL.
*/}}
{{- define "camundaPlatform.operateExternalURL" }}
  {{- printf "%s/operate" (include "camundaPlatform.orchestrationExternalURL" . | trimSuffix "/") -}}
{{- end -}}


{{/*
********************************************************************************
Optimize templates.
********************************************************************************
*/}}
{{/*
[camunda-platform] Optimize external URL.
*/}}
{{- define "camundaPlatform.optimizeExternalURL" }}
  {{- printf "%s" (include "camundaPlatform.getExternalURL" (dict "component" "optimize" "context" .)) -}}
{{- end -}}

{{/*
********************************************************************************
Connectors templates.
********************************************************************************
*/}}
{{/*
[camunda-platform] Connectors external URL.
*/}}
{{- define "camundaPlatform.connectorsExternalURL" }}
  {{- $proto := (lower .Values.connectors.readinessProbe.scheme) -}}
  {{- $baseURLInternal := printf "%s://%s.%s" $proto (include "connectors.serviceName" .) .Release.Namespace -}}
  {{- printf "%s:%v%s" $baseURLInternal .Values.connectors.service.serverPort (include "camundaPlatform.joinpath" (list .Values.connectors.contextPath "")) | trimSuffix "/" -}}
{{- end -}}

{{/*
********************************************************************************
Tasklist templates.
********************************************************************************
*/}}

{{/*
[camunda-platform] Tasklist external URL.
*/}}
{{- define "camundaPlatform.tasklistExternalURL" }}
  {{- printf "%s/tasklist" (include "camundaPlatform.orchestrationExternalURL" . | trimSuffix "/") -}}
{{- end -}}


{{/*
********************************************************************************
Orchestration Identity templates.
********************************************************************************
*/}}

{{/*
[camunda-platform] Orchestration Admin external URL.
*/}}
{{- define "camundaPlatform.orchestrationIdentityExternalURL" }}
  {{- printf "%s/admin" (include "camundaPlatform.orchestrationExternalURL" . | trimSuffix "/") -}}
{{- end -}}


{{/*
********************************************************************************
Web Modeler templates.
********************************************************************************
*/}}
{{/*
[camunda-platform] Web Modeler external URL.
*/}}

{{- define "camundaPlatform.getExternalURLModeler" -}}
  {{- if eq (include "camundaHub.webModelerEnabled" .context) "true" -}}
    {{- if $.context.Values.global.ingress.enabled -}}
      {{ $proto := ternary "https" "http" .context.Values.global.ingress.tls.enabled -}}
      {{- if eq .component "websockets" }}
        {{- printf "%s://%s%s" $proto (tpl .context.Values.global.host .context) (include "webModeler.websocketContextPath" .context) -}}
      {{- else -}}
        {{- printf "%s://%s%s" $proto (tpl .context.Values.global.host .context) (or .context.Values.camundaHub.webModeler.contextPath .context.Values.webModeler.contextPath) -}}
      {{- end -}}
    {{- else -}}
      {{- if eq .component "websockets" -}}
        {{- printf "http://localhost:8085" -}}
      {{- else -}}
        {{- printf "http://localhost:8070" -}}
      {{- end -}}
    {{- end -}}
  {{- end -}}
{{- end -}}

{{- define "camundaPlatform.webModelerWebSocketsExternalURL" }}
  {{- printf "%s" (include "camundaPlatform.getExternalURLModeler" (dict "component" "websockets" "context" .)) -}}
{{- end -}}

{{- define "camundaPlatform.webModelerExternalURL" }}
  {{- printf "%s" (include "camundaPlatform.getExternalURLModeler" (dict "component" "" "context" .)) -}}
{{- end -}}


{{/*
********************************************************************************
Identity templates.
********************************************************************************
*/}}

{{- define "identity.authAudience" -}}
  {{- .Values.global.identity.auth.identity.audience | default "camunda-identity-resource-server" -}}
{{- end -}}

{{- define "identity.authClientId" -}}
  {{- .Values.global.identity.auth.identity.clientId | default "camunda-identity" -}}
{{- end -}}


{{/*
Create a default fully qualified app name.
*/}}

{{- define "identity.fullname" -}}
    {{- include "camundaPlatform.componentFullname" (dict
        "componentName" "identity"
        "componentValues" .Values.identity
        "context" $
    ) -}}
{{- end -}}

{{/*
[camunda-platform] Identity internal URL.
*/}}
{{ define "camundaPlatform.identityURL" }}
  {{- if .Values.global.identity.service.url -}}
    {{- .Values.global.identity.service.url -}}
  {{- else -}}
    {{-
      printf "http://%s:%v%s"
        (include "identity.fullname" .)
        .Values.identity.service.port
        (.Values.identity.contextPath | default "")
    -}}
  {{- end -}}
{{- end -}}

{{/*
[camunda-platform] Create the name of the Identity secret for components.
Usage: {{ include "camundaPlatform.identitySecretName" (dict "context" . "component" "zeebe") }}
*/}}
{{- define "camundaPlatform.identitySecretName" -}}
  {{- $releaseName := .context.Release.Name | trunc 63 | trimSuffix "-" -}}
  {{- printf "%s-%s-identity-secret" $releaseName .component -}}
{{- end }}

{{/*
[camunda-platform] Identity external URL.
*/}}
{{- define "camundaPlatform.identityExternalURL" }}
  {{- printf "%s" (include "camundaPlatform.getExternalURL" (dict "component" "identity" "context" .)) -}}
{{- end -}}


{{/*
********************************************************************************
Identity Auth.
********************************************************************************
*/}}

{{- define "camundaPlatform.authAudienceOptimize" -}}
  {{- .Values.global.identity.auth.optimize.audience | default "optimize-api" -}}
{{- end -}}



{{/*
********************************************************************************
Camunda Hub backward-compatibility shim helpers.
These helpers support the consolidation of Console and WebModeler into a single
"camundaHub" component while maintaining full backward compatibility with
the legacy console.* and webModeler.* top-level keys.

When camundaHub.enabled=true, both sub-components are enabled regardless of
their individual .enabled flags. The camundaHub.console.* and
camundaHub.webModeler.* overrides take precedence over the legacy keys.
********************************************************************************
*/}}

{{/*
[camunda-hub] Check if the WebModeler sub-component should be enabled.
Returns "true" if camundaHub.enabled OR webModeler.enabled.
Usage: {{- if eq (include "camundaHub.webModelerEnabled" .) "true" }}
*/}}
{{- define "camundaHub.webModelerEnabled" -}}
  {{- if or .Values.camundaHub.enabled .Values.webModeler.enabled -}}
    true
  {{- else -}}
    false
  {{- end -}}
{{- end -}}


{{/*
********************************************************************************
Orchestration templates.
********************************************************************************
*/}}

{{/*
[orchestration] Get the image pull secrets.
*/}}
{{- define "orchestration.imagePullSecrets" -}}
    {{- include "camundaPlatform.imagePullSecrets" (dict
        "component" "orchestration"
        "context" $
    ) -}}
{{- end }}

{{/*
********************************************************************************
Zeebe templates.
********************************************************************************
*/}}
{{/*
[camunda-platform] Zeebe Gateway external URL.
*/}}
{{- define "camundaPlatform.orchestrationExternalURL" }}
  {{- if .Values.global.ingress.enabled -}}
    {{ $proto := ternary "https" "http" .Values.global.ingress.tls.enabled -}}
    {{- printf "%s://%s%s" $proto (tpl .Values.global.host $) (include "camundaPlatform.joinpath" (list .Values.orchestration.contextPath)) -}}
  {{- else -}}
    {{- printf "http://localhost:8080" -}}
  {{- end -}}
{{- end -}}

{{/*
[camunda-platform] Zeebe Gateway GRPC external URL.
*/}}
{{- define "camundaPlatform.orchestrationGRPCExternalURL" -}}
  {{ $proto := ternary "https" "http" .Values.orchestration.ingress.grpc.tls.enabled -}}
  {{- printf "%s://%s" $proto (tpl .Values.orchestration.ingress.grpc.host . | default "localhost:26500") -}}
{{- end -}}

{{/*
[camunda-platform] Zeebe Gateway REST internal URL.
*/}}
{{ define "camundaPlatform.orchestrationHTTPInternalURL" }}
  {{- if .Values.orchestration.enabled -}}
    {{-
      printf "%s://%s%s"
        (ternary "https" "http" (eq (include "camundaPlatform.orchestrationEnvIsTrue" (dict "context" . "name" "SERVER_SSL_ENABLED")) "true"))
        (include "orchestration.serviceNameHTTP" .)
        (.Values.orchestration.contextPath | default "")
    -}}
  {{- end -}}
{{- end -}}

{{/*
[camunda-platform] Zeebe Gateway GRPC internal URL.
*/}}
{{ define "camundaPlatform.orchestrationGRPCInternalURL" }}
  {{- if .Values.orchestration.enabled -}}
    {{-
      printf "%s://%s"
        (ternary "grpcs" "grpc" (eq (include "camundaPlatform.orchestrationEnvIsTrue" (dict "context" . "name" "CAMUNDA_API_GRPC_SSL_ENABLED")) "true"))
        (include "orchestration.serviceNameGRPC" .)
    -}}
  {{- end -}}
{{- end -}}

{{/*
[camunda-platform] Returns true when the last orchestration.env entry for name has value true.
*/}}
{{- define "camundaPlatform.orchestrationEnvIsTrue" -}}
  {{- $ctx := .context -}}
  {{- $name := .name -}}
  {{- $enabled := false -}}
  {{- range $env := $ctx.Values.orchestration.env -}}
    {{- if eq ($env.name | default "") $name -}}
      {{- $enabled = (eq (lower (tpl (toString ($env.value | default "")) $ctx)) "true") -}}
    {{- end -}}
  {{- end -}}
  {{- $enabled -}}
{{- end -}}


{{/*
********************************************************************************
Release templates.
********************************************************************************
*/}}

{{ define "camundaPlatform.releaseInfo" -}}
- name: {{ .Release.Name }}
  namespace: {{ .Release.Namespace }}
  version: {{ .Chart.Version }}
  tags:
  - dev
  custom-properties: []
  components:
  {{- $proto := ternary "https" "http" .Values.global.ingress.tls.enabled -}}
  {{- $baseURL := printf "%s://%s" $proto (tpl .Values.global.host $) }}

  {{- if .Values.identity.enabled }}
  {{-  $proto := (lower .Values.identity.readinessProbe.scheme) -}}
  {{- $baseURLInternal := printf "%s://%s.%s:%v" $proto (include "identity.fullname" .) .Release.Namespace .Values.identity.service.metricsPort -}}
  {{- if include "camundaPlatform.keycloakExternalURL" . }}
  - name: Keycloak
    id: keycloak
    url: {{ include "camundaPlatform.keycloakExternalURL" . }}
  {{- end }}
  - name: Identity
    id: identity
    version: {{ include "camundaPlatform.imageTagByParams" (dict "base" .Values.global "overlay" .Values.identity) }}
    url: {{ include "camundaPlatform.identityExternalURL" . }}
    readiness: {{ printf "%s%s" $baseURLInternal .Values.identity.readinessProbe.probePath }}
    metrics: {{ printf "%s%s" $baseURLInternal .Values.identity.metrics.prometheus }}
  {{- end }}

  {{- if eq (include "camundaHub.webModelerEnabled" .) "true" }}
  {{-  $proto := (lower (or .Values.camundaHub.webModeler.restapi.readinessProbe.scheme .Values.webModeler.restapi.readinessProbe.scheme)) -}}
  {{- $baseURLInternal := printf "%s://%s.%s:%v" $proto (include "webModeler.restapi.fullname" .) .Release.Namespace (or .Values.camundaHub.webModeler.restapi.service.managementPort .Values.webModeler.restapi.service.managementPort) }}
  - name: WebModeler
    id: webModelerWebApp
    version: {{ include "camundaPlatform.imageTagByParams" (dict "base" .Values.global "overlay" (mustMergeOverwrite (deepCopy .Values.webModeler) (.Values.camundaHub.webModeler | default dict))) }}
    url: {{ include "camundaPlatform.webModelerExternalURL" . }}
    readiness: {{ printf "%s%s" $baseURLInternal (include "camundaPlatform.joinpath" (list (or .Values.camundaHub.webModeler.contextPath .Values.webModeler.contextPath) (or .Values.camundaHub.webModeler.restapi.readinessProbe.probePath .Values.webModeler.restapi.readinessProbe.probePath))) }}
    metrics: {{ printf "%s%s" $baseURLInternal (include "camundaPlatform.joinpath" (list (or .Values.camundaHub.webModeler.contextPath .Values.webModeler.contextPath) (or .Values.camundaHub.webModeler.restapi.metrics.prometheus .Values.webModeler.restapi.metrics.prometheus))) }}
  {{- end }}

  {{- if .Values.optimize.enabled }}
  {{-  $proto := (lower .Values.optimize.readinessProbe.scheme) -}}
  {{- $baseURLInternal := printf "%s://%s.%s" $proto (include "optimize.fullname" .) .Release.Namespace }}
  - name: Optimize
    id: optimize
    version: {{ include "camundaPlatform.imageTagByParams" (dict "base" .Values.global "overlay" .Values.optimize) }}
    url: {{ include "camundaPlatform.optimizeExternalURL" . }}
    readiness: {{ printf "%s:%v%s" $baseURLInternal .Values.optimize.service.port (include "camundaPlatform.joinpath" (list .Values.optimize.contextPath .Values.optimize.readinessProbe.probePath)) }}
    metrics: {{ printf "%s:%v%s" $baseURLInternal .Values.optimize.service.managementPort .Values.optimize.metrics.prometheus }}
  {{- end }}

  {{- if .Values.connectors.enabled }}
  {{-  $proto := (lower .Values.connectors.readinessProbe.scheme) -}}
  {{- $baseURLInternal := printf "%s://%s.%s" $proto (include "connectors.serviceName" .) .Release.Namespace }}
  - name: Connectors
    id: connectors
    version: {{ include "camundaPlatform.imageTagByParams" (dict "base" .Values.global "overlay" .Values.connectors) }}
    url: {{ include "camundaPlatform.connectorsExternalURL" . }}
    readiness: {{ printf "%s:%v%s" $baseURLInternal .Values.connectors.service.serverPort (include "camundaPlatform.joinpath" (list .Values.connectors.contextPath .Values.connectors.readinessProbe.probePath)) }}
    metrics: {{ printf "%s:%v%s" $baseURLInternal .Values.connectors.service.serverPort (include "camundaPlatform.joinpath" (list .Values.connectors.contextPath .Values.connectors.metrics.prometheus)) }}
  {{- end }}

  {{- if .Values.orchestration.enabled }}
  {{-  $proto := (lower .Values.orchestration.readinessProbe.scheme) -}}
  {{- $baseURLInternal := printf "%s://%s.%s:%v" $proto (include "orchestration.fullname" . | trimAll "\"") .Release.Namespace .Values.orchestration.service.managementPort }}
  - name: Operate
    id: operate
    version: {{ include "camundaPlatform.imageTagByParams" (dict "base" .Values.global "overlay" .Values.orchestration) }}
    url: {{ include "camundaPlatform.operateExternalURL" . }}
    readiness: {{ printf "%s%s" $baseURLInternal (include "camundaPlatform.joinpath" (list .Values.orchestration.contextPath .Values.orchestration.readinessProbe.probePath)) }}
    metrics: {{ printf "%s%s" $baseURLInternal (include "camundaPlatform.joinpath" (list .Values.orchestration.contextPath .Values.orchestration.metrics.prometheus)) }}
  - name: Tasklist
    id: tasklist
    version: {{ include "camundaPlatform.imageTagByParams" (dict "base" .Values.global "overlay" .Values.orchestration) }}
    url: {{ include "camundaPlatform.tasklistExternalURL" . }}
    readiness: {{ printf "%s%s" $baseURLInternal (include "camundaPlatform.joinpath" (list .Values.orchestration.contextPath .Values.orchestration.readinessProbe.probePath)) }}
    metrics: {{ printf "%s%s" $baseURLInternal (include "camundaPlatform.joinpath" (list .Values.orchestration.contextPath .Values.orchestration.metrics.prometheus)) }}
  - name: Orchestration Admin
    id: orchestrationIdentity
    version: {{ include "camundaPlatform.imageTagByParams" (dict "base" .Values.global "overlay" .Values.orchestration) }}
    url: {{ include "camundaPlatform.orchestrationIdentityExternalURL" . }}
    readiness: {{ printf "%s%s" $baseURLInternal (include "camundaPlatform.joinpath" (list .Values.orchestration.contextPath .Values.orchestration.readinessProbe.probePath)) }}
    metrics: {{ printf "%s%s" $baseURLInternal (include "camundaPlatform.joinpath" (list .Values.orchestration.contextPath .Values.orchestration.metrics.prometheus)) }}

  - name: Orchestration Cluster
    id: orchestration
    version: {{ include "camundaPlatform.imageTagByParams" (dict "base" .Values.global "overlay" .Values.orchestration) }}
    urls:
      grpc: {{ include "camundaPlatform.orchestrationGRPCExternalURL" . }}
      http: {{ include "camundaPlatform.orchestrationExternalURL" . }}
    readiness: {{ printf "%s%s" $baseURLInternal (include "camundaPlatform.joinpath" (list .Values.orchestration.contextPath .Values.orchestration.readinessProbe.probePath)) }}
    metrics: {{ printf "%s%s" $baseURLInternal (include "camundaPlatform.joinpath" (list .Values.orchestration.contextPath .Values.orchestration.metrics.prometheus)) }}
  {{- end }}
{{- end -}}

{{/*
********************************************************************************
Generate the default WebModeler cluster config using the new components[] schema.
Mirrors camundaPlatform.releaseInfo component gating but outputs the new format
required by camunda.modeler.clusters (introduced in 8.10 Hub/WebModeler).
********************************************************************************
*/}}
{{- define "camundaPlatform.defaultWebModelerCluster" -}}
{{- if or .Values.identity.enabled (eq (include "camundaHub.webModelerEnabled" .) "true") }}
- id: "management-cluster"
  name: "management"
  version: {{ include "camundaPlatform.imageTagByParams" (dict "base" .Values.global "overlay" (mustMergeOverwrite (deepCopy .Values.webModeler) (.Values.camundaHub.webModeler | default dict))) | quote }}
  authentication: {{ include "webModeler.authConfigValue" . | quote }}
  authorizations:
    enabled: false
  components:
  {{- if .Values.identity.enabled }}
  {{- $proto := (lower .Values.identity.readinessProbe.scheme) }}
  {{- $baseURLInternal := printf "%s://%s.%s:%v" $proto (include "identity.fullname" .) .Release.Namespace .Values.identity.service.metricsPort }}
  - name: Identity
    type: identity
    version: {{ include "camundaPlatform.imageTagByParams" (dict "base" .Values.global "overlay" .Values.identity) | quote }}
    urls:
      webapp: {{ include "camundaPlatform.identityExternalURL" . | quote }}
      readiness: {{ printf "%s%s" $baseURLInternal .Values.identity.readinessProbe.probePath | quote }}
  {{- end }}
  {{- if eq (include "camundaHub.webModelerEnabled" .) "true" }}
  {{- $proto := (lower (or .Values.camundaHub.webModeler.restapi.readinessProbe.scheme .Values.webModeler.restapi.readinessProbe.scheme)) }}
  {{- $baseURLInternal := printf "%s://%s.%s:%v" $proto (include "webModeler.restapi.fullname" .) .Release.Namespace (or .Values.camundaHub.webModeler.restapi.service.managementPort .Values.webModeler.restapi.service.managementPort) }}
  - name: WebModeler
    type: webModelerWebApp
    version: {{ include "camundaPlatform.imageTagByParams" (dict "base" .Values.global "overlay" (mustMergeOverwrite (deepCopy .Values.webModeler) (.Values.camundaHub.webModeler | default dict))) | quote }}
    urls:
      webapp: {{ include "camundaPlatform.webModelerExternalURL" . | quote }}
      readiness: {{ printf "%s%s" $baseURLInternal (include "camundaPlatform.joinpath" (list (or .Values.camundaHub.webModeler.contextPath .Values.webModeler.contextPath) (or .Values.camundaHub.webModeler.restapi.readinessProbe.probePath .Values.webModeler.restapi.readinessProbe.probePath))) | quote }}
  {{- end }}
{{- end }}
{{- if or .Values.orchestration.enabled .Values.optimize.enabled .Values.connectors.enabled }}
- id: "default-cluster"
  name: {{ tpl .Values.global.zeebeClusterName . | quote }}
  version: {{ include "camundaPlatform.imageTagByParams" (dict "base" .Values.global "overlay" .Values.orchestration) | quote }}
  authentication: {{ include "webModeler.authConfigValue" . | quote }}
  authorizations:
    enabled: {{ .Values.orchestration.security.authorizations.enabled }}
  components:
  {{- if .Values.optimize.enabled }}
  {{- $proto := (lower .Values.optimize.readinessProbe.scheme) }}
  {{- $baseURLInternal := printf "%s://%s.%s" $proto (include "optimize.fullname" .) .Release.Namespace }}
  - name: Optimize
    type: optimize
    version: {{ include "camundaPlatform.imageTagByParams" (dict "base" .Values.global "overlay" .Values.optimize) | quote }}
    urls:
      webapp: {{ include "camundaPlatform.optimizeExternalURL" . | quote }}
      readiness: {{ printf "%s:%v%s" $baseURLInternal .Values.optimize.service.port (include "camundaPlatform.joinpath" (list .Values.optimize.contextPath .Values.optimize.readinessProbe.probePath)) | quote }}
  {{- end }}
  {{- if .Values.connectors.enabled }}
  {{- $proto := (lower .Values.connectors.readinessProbe.scheme) }}
  {{- $baseURLInternal := printf "%s://%s.%s" $proto (include "connectors.serviceName" .) .Release.Namespace }}
  - name: Connectors
    type: connectors
    version: {{ include "camundaPlatform.imageTagByParams" (dict "base" .Values.global "overlay" .Values.connectors) | quote }}
    urls:
      rest: {{ include "camundaPlatform.connectorsExternalURL" . | quote }}
      readiness: {{ printf "%s:%v%s" $baseURLInternal .Values.connectors.service.serverPort (include "camundaPlatform.joinpath" (list .Values.connectors.contextPath .Values.connectors.readinessProbe.probePath)) | quote }}
  {{- end }}
  {{- if .Values.orchestration.enabled }}
  {{- $proto := (lower .Values.orchestration.readinessProbe.scheme) }}
  {{- $baseURLInternal := printf "%s://%s.%s:%v" $proto (include "orchestration.fullname" . | trimAll "\"") .Release.Namespace .Values.orchestration.service.managementPort }}
  - name: Operate
    type: operate
    version: {{ include "camundaPlatform.imageTagByParams" (dict "base" .Values.global "overlay" .Values.orchestration) | quote }}
    urls:
      webapp: {{ include "camundaPlatform.operateExternalURL" . | quote }}
      readiness: {{ printf "%s%s" $baseURLInternal (include "camundaPlatform.joinpath" (list .Values.orchestration.contextPath .Values.orchestration.readinessProbe.probePath)) | quote }}
  - name: Tasklist
    type: tasklist
    version: {{ include "camundaPlatform.imageTagByParams" (dict "base" .Values.global "overlay" .Values.orchestration) | quote }}
    urls:
      webapp: {{ include "camundaPlatform.tasklistExternalURL" . | quote }}
      readiness: {{ printf "%s%s" $baseURLInternal (include "camundaPlatform.joinpath" (list .Values.orchestration.contextPath .Values.orchestration.readinessProbe.probePath)) | quote }}
  - name: Orchestration Admin
    type: orchestrationIdentity
    version: {{ include "camundaPlatform.imageTagByParams" (dict "base" .Values.global "overlay" .Values.orchestration) | quote }}
    urls:
      webapp: {{ include "camundaPlatform.orchestrationIdentityExternalURL" . | quote }}
      readiness: {{ printf "%s%s" $baseURLInternal (include "camundaPlatform.joinpath" (list .Values.orchestration.contextPath .Values.orchestration.readinessProbe.probePath)) | quote }}
  - name: Orchestration Cluster
    type: orchestration
    version: {{ include "camundaPlatform.imageTagByParams" (dict "base" .Values.global "overlay" .Values.orchestration) | quote }}
    urls:
      grpc: {{ include "camundaPlatform.orchestrationGRPCInternalURL" . | quote }}
      rest: {{ include "camundaPlatform.orchestrationHTTPInternalURL" . | quote }}
      readiness: {{ printf "%s%s" $baseURLInternal (include "camundaPlatform.joinpath" (list .Values.orchestration.contextPath .Values.orchestration.readinessProbe.probePath)) | quote }}
  {{- end }}
{{- end }}
{{- end -}}

{{/*
normalizeSecretConfiguration
Resolves secret configuration to a standardized output format.
Returns a dict with "ref" and "plaintext" keys.
- "ref": dict with "name" and "key" fields for Kubernetes secret reference, or nil if not using secret
- "plaintext": string value for inline plaintext, or empty string if using secret reference
Usage:
  {{ include "camundaPlatform.normalizeSecretConfiguration" (dict
      "config" .Values.identity.firstUser
      "defaultSecretName" "my-default-secret"
      "defaultSecretKey" "password"
  ) }}
*/}}
{{- define "camundaPlatform.normalizeSecretConfiguration" -}}
{{- $config := .config | default dict -}}
{{- $defName := .defaultSecretName | default "" -}}
{{- $defKey := .defaultSecretKey | default "password" -}}

{{- $result := dict "ref" nil "plaintext" "" -}}

{{- if and $config.secret $config.secret.existingSecret $config.secret.existingSecretKey -}}
  {{- $_ := set $result "ref" (dict "name" $config.secret.existingSecret "key" $config.secret.existingSecretKey) -}}
{{- else if and $config.secret $config.secret.inlineSecret -}}
  {{- $_ := set $result "plaintext" $config.secret.inlineSecret -}}
{{- end }}

{{/* Fallback to the caller‑supplied default */}}
{{- if and (not $result.ref) (not $result.plaintext) $defName -}}
  {{- $_ := set $result "ref" (dict "name" $defName "key" $defKey) -}}
{{- end }}

{{- toYaml $result -}}
{{- end -}}

{{/*
emitEnvVarFromSecretConfig
Usage:
  {{ include "camundaPlatform.emitEnvVarFromSecretConfig" (dict
      "envName" "VALUES_IDENTITY_FIRSTUSER_PASSWORD"
      "config"  .Values.identity.firstUser
  ) }}
*/}}
{{- define "camundaPlatform.emitEnvVarFromSecretConfig" -}}
{{- $norm := include "camundaPlatform.normalizeSecretConfiguration" . | fromYaml -}}
{{- if or $norm.ref $norm.plaintext -}}
- name: {{ .envName }}
{{- if $norm.ref }}
  valueFrom:
    secretKeyRef:
      name: {{ $norm.ref.name }}
      key: {{ $norm.ref.key }}
{{- else }}
  value: {{ $norm.plaintext | quote }}
{{- end }}
{{- end -}}
{{- end -}}

{{/*
hasSecretConfig
Returns a string indicating whether there is a valid secret configuration.
Named templates don't return bools, only strings [1].
Usage:
  {{ if eq (include "camundaPlatform.hasSecretConfig" (dict
      "config"  .Values.identity.firstUser
  )) "true" }}

[1] https://github.com/helm/helm/issues/11231
*/}}
{{- define "camundaPlatform.hasSecretConfig" -}}
{{- $norm := include "camundaPlatform.normalizeSecretConfiguration" . | fromYaml -}}
{{- if or $norm.ref $norm.plaintext -}}
true
{{- else -}}
false
{{- end -}}
{{- end -}}

{{/*
emitAwsDocumentStoreSecret
Emits AWS Document Store environment variable secret configuration.
Usage:
  - name: AWS_ACCESS_KEY_ID
    {{ include "camundaPlatform.emitAwsDocumentStoreSecret" (dict "secretType" "accessKeyId" "context" .) }}
  - name: AWS_SECRET_ACCESS_KEY
    {{ include "camundaPlatform.emitAwsDocumentStoreSecret" (dict "secretType" "secretAccessKey" "context" .) }}
*/}}
{{- define "camundaPlatform.emitAwsDocumentStoreSecret" -}}
{{- $root := .context -}}
{{- if $root.Values.global.documentStore.type.aws.enabled -}}
{{- $awsConfig := $root.Values.global.documentStore.type.aws -}}
{{- $secretType := .secretType -}}
{{- $secretConfig := (index $awsConfig $secretType) | default dict -}}
{{- if and $secretConfig.secret (or $secretConfig.secret.existingSecret $secretConfig.secret.inlineSecret) -}}
{{- if and $secretConfig.secret.existingSecret $secretConfig.secret.existingSecretKey -}}
valueFrom:
  secretKeyRef:
    name: {{ $secretConfig.secret.existingSecret }}
    key: {{ $secretConfig.secret.existingSecretKey }}
{{- else if $secretConfig.secret.inlineSecret -}}
value: {{ $secretConfig.secret.inlineSecret | quote }}
{{- end -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{/*
emitVolumeFromSecretConfig
Emits volume definition using normalized secret configuration.
Usage:
  {{ include "camundaPlatform.emitVolumeFromSecretConfig" (dict
      "volumeName" "gcp-credentials-volume"
      "config" .Values.global.documentStore.type.gcp
      "fileName" (.Values.global.documentStore.type.gcp.fileName | default "service-account.json")
  ) }}
*/}}
{{- define "camundaPlatform.emitVolumeFromSecretConfig" -}}
{{- $norm := include "camundaPlatform.normalizeSecretConfiguration" . | fromYaml -}}
{{- if $norm.ref }}
- name: {{ .volumeName }}
  secret:
    secretName: {{ $norm.ref.name | quote }}
    items:
      - key: {{ $norm.ref.key | quote }}
        path: {{ .fileName | quote }}
{{- end }}
{{- end -}}

{{/*
emitTlsVolumeFromSecretConfig
Emits volume definition for TLS secrets.
Usage:
  {{ include "camundaPlatform.emitTlsVolumeFromSecretConfig" (dict
      "volumeName" "keystore"
      "config" .Values.global.elasticsearch.tls
  ) }}
*/}}
{{- define "camundaPlatform.emitTlsVolumeFromSecretConfig" -}}
{{- $config := .config | default dict -}}
{{- if and $config.secret $config.secret.existingSecret }}
- name: {{ .volumeName }}
  secret:
    secretName: {{ $config.secret.existingSecret | quote }}
    optional: false
{{- end }}
{{- end -}}

{{/*
getTlsSecretKey
Returns the secret key name from TLS config.
Uses config.secret.existingSecretKey.
Accepts root context (.) and uses the enabled database type (ES or OS).
Usage:
  {{ include "camundaPlatform.getTlsSecretKey" . }}
  {{ include "camundaPlatform.getTlsSecretKey" (dict "config" .Values.global.elasticsearch.tls) }}
*/}}
{{- define "camundaPlatform.getTlsSecretKey" -}}
{{- $config := dict -}}

{{- if .config -}}
  {{- $config = .config -}}
{{- else if .Values -}}
  {{- if .Values.global.opensearch.enabled -}}
    {{- $config = .Values.global.opensearch.tls -}}
  {{- else -}}
    {{- $config = .Values.global.elasticsearch.tls -}}
  {{- end -}}
{{- end -}}

{{- if and $config.secret $config.secret.existingSecretKey -}}
  {{- $config.secret.existingSecretKey -}}
{{- end -}}
{{- end -}}

{{/*
common.java_tool_options_tls_env

Emits JAVA_TOOL_OPTIONS with truststore flags and emits TRUSTSTORE_PASSWORD using the normalized secret helper.

Usage in a Deployment/StatefulSet env: block:
  {{ include "common.java_tool_options_tls_env" (dict
    "Values" .Values
    "component" "orchestration"            # REQUIRED: values key to read javaOpts from (e.g., orchestration, optimize)
  ) | nindent 12 }}

Prerequisites when TLS is enabled for Elasticsearch/OpenSearch:
- 8.10 removed the legacy global.<engine>.tls.existingSecret string field; use
  global.<engine>.tls.secret.existingSecret / existingSecretKey instead. To opt into the
  password injection, ALSO set the corresponding global.<engine>.tls.jks.secret block:

Example (existing secret, recommended):
  global:
    elasticsearch:
      tls:
        enabled: true
        secret:
          existingSecret: my-es-tls
          existingSecretKey: externaldb.jks
        jks:
          secret:
            existingSecret: my-truststore-secret
            existingSecretKey: truststore-password

Example (inline plaintext, testing only):
  global:
    opensearch:
      tls:
        enabled: true
        secret:
          existingSecret: my-os-tls
          existingSecretKey: externaldb.jks
        jks:
          secret:
            inlineSecret: "changeit"

Behavior:
- Requires "component" parameter; fails if omitted.
- Renders JAVA_TOOL_OPTIONS composed of:
  <component>.javaOpts (or provided "javaOpts") plus:
    -Djavax.net.ssl.trustStore=<truststoreDir>/<dynamic-filename>
    -Djavax.net.ssl.trustStorePassword=$(TRUSTSTORE_PASSWORD)        # only when global.<engine>.tls.jks.secret is configured
- Truststore filename is resolved via "camundaPlatform.getTlsSecretKey".
- Emits TRUSTSTORE_PASSWORD via "camundaPlatform.emitEnvVarFromSecretConfig" when one of:
  - global.elasticsearch.tls.jks (preferred when ES TLS secret is set), or
  - global.opensearch.tls.jks (preferred when OS TLS secret is set)
  is configured (existingSecret/existingSecretKey or inlineSecret). Component-level TLS configs
  (e.g. orchestration.data.secondaryStorage.*.tls, optimize.database.*.tls) do NOT yet carry a jks
  password block; for those, callers must continue to set the password via JAVA_OPTS manually.

Note: the $(TRUSTSTORE_PASSWORD) placeholder is not resolved during Helm template evaluation.
It is expanded by Kubernetes env-var substitution when constructing the container environment,
because TRUSTSTORE_PASSWORD is emitted as an earlier env entry. Java then receives the
already-expanded JAVA_TOOL_OPTIONS value at runtime.

Reserved env var name: when this helper fires it owns the env var name TRUSTSTORE_PASSWORD.
Do not set TRUSTSTORE_PASSWORD via <component>.env / extraEnv when using this feature —
duplicate env names are rejected by some Kubernetes API servers (e.g. AKS) and cause undefined
last-wins behaviour elsewhere.

Migration note: customers who previously worked around the missing password support by setting
"-Djavax.net.ssl.trustStorePassword=..." via <component>.javaOpts should remove that flag once
they adopt global.<engine>.tls.jks.secret.* — otherwise both flags are present and the JVM uses
the last one, which becomes confusing to debug.
*/}}

{{- /* Internal: resolve the correct TLS JKS config for the currently enabled engine */ -}}
{{- define "camundaPlatform._resolve_tls_jks_config" -}}
{{- $cfg := dict -}}
{{- if (eq (include "camundaPlatform.hasSecretConfig" (dict "config" .Values.global.elasticsearch.tls)) "true") -}}
{{-   $cfg = (.Values.global.elasticsearch.tls.jks | default dict) -}}
{{- else if (eq (include "camundaPlatform.hasSecretConfig" (dict "config" .Values.global.opensearch.tls)) "true") -}}
{{-   $cfg = (.Values.global.opensearch.tls.jks | default dict) -}}
{{- end -}}
{{- toYaml $cfg -}}
{{- end -}}

{{- /* Internal: unified JAVA_TOOL_OPTIONS + TRUSTSTORE_PASSWORD emitter */ -}}
{{- define "camundaPlatform._java_tool_options_tls_env" -}}
{{- $vals := .Values -}}
{{- $comp := required "common.java_tool_options_tls_env: parameter 'component' is required" .component -}}
{{- $compVals := (get $vals $comp) | default dict -}}
{{- $javaOpts := (.javaOpts | default ((get $compVals "javaOpts") | default "")) | trim -}}
{{- $truststoreDir := required "camundaPlatform._java_tool_options_tls_env: parameter 'truststoreDir' is required" .truststoreDir -}}
{{- $secretKey := include "camundaPlatform.getTlsSecretKey" (dict "Values" $vals) -}}
{{- $truststorePath := printf "%s/%s" $truststoreDir $secretKey -}}
{{- $jks := ((include "camundaPlatform._resolve_tls_jks_config" .) | fromYaml) | default dict -}}
{{- if (eq (include "camundaPlatform.hasSecretConfig" (dict "config" $jks)) "true") -}}
{{- include "camundaPlatform.emitEnvVarFromSecretConfig" (dict
    "envName" "TRUSTSTORE_PASSWORD"
    "config" $jks
 ) | nindent 0 }}
{{- end }}
- name: JAVA_TOOL_OPTIONS
  value: >-
    {{- if $javaOpts -}}
    {{- if (eq (include "camundaPlatform.hasSecretConfig" (dict "config" $jks)) "true") -}}
    {{- printf "%s\n-Djavax.net.ssl.trustStore=%s\n-Djavax.net.ssl.trustStorePassword=$(TRUSTSTORE_PASSWORD)" $javaOpts $truststorePath | nindent 4 }}
    {{- else -}}
    {{- printf "%s\n-Djavax.net.ssl.trustStore=%s" $javaOpts $truststorePath | nindent 4 }}
    {{- end -}}
    {{- else -}}
    {{- if (eq (include "camundaPlatform.hasSecretConfig" (dict "config" $jks)) "true") -}}
    {{- printf "-Djavax.net.ssl.trustStore=%s\n-Djavax.net.ssl.trustStorePassword=$(TRUSTSTORE_PASSWORD)" $truststorePath | nindent 4 }}
    {{- else -}}
    {{- printf "-Djavax.net.ssl.trustStore=%s" $truststorePath | nindent 4 }}
    {{- end -}}
    {{- end -}}
{{- end }}

{{- define "common.java_tool_options_tls_env" -}}
{{ include "camundaPlatform._java_tool_options_tls_env" (dict
  "Values" .Values
  "component" .component
  "javaOpts" .javaOpts
  "truststoreDir" "/usr/local/camunda/certificates"
) }}
{{- end }}

{{/* optimize.java_tool_options_tls_env
Delegates to camundaPlatform._java_tool_options_tls_env.
See common.java_tool_options_tls_env for full documentation.
*/}}
{{- define "optimize.java_tool_options_tls_env" -}}
{{ include "camundaPlatform._java_tool_options_tls_env" (dict
  "Values" .Values
  "component" .component
  "javaOpts" .javaOpts
  "truststoreDir" "/optimize/certificates"
) }}
{{- end }}

{{/*
hasCaBundle
Returns "true" when global.tls.caBundle.secret.existingSecret is set,
"false" otherwise. Mirrors hasSecretConfig but specific to the OS-level
CA bundle.
Usage:
  {{ if eq (include "camundaPlatform.hasCaBundle" .) "true" }}
*/}}
{{- define "camundaPlatform.hasCaBundle" -}}
{{- if and .Values.global.tls .Values.global.tls.caBundle .Values.global.tls.caBundle.secret .Values.global.tls.caBundle.secret.existingSecret -}}
true
{{- else -}}
false
{{- end -}}
{{- end -}}

{{/*
caBundleChecksumAnnotation
Emits a `checksum/ca-bundle` pod annotation derived from the live CA-bundle
Secret so that, on `helm upgrade`, pods roll automatically when the CA is
rotated — the init container then rebuilds the truststore with the new CA.
Without this, rotating the secret has no effect until the operator manually
runs `kubectl rollout restart`.

`lookup` returns an empty value during `helm template` / `--dry-run` (no API
access), so the checksum is stable there and only carries real content on a
live `helm upgrade`. It therefore does NOT react to a raw `kubectl edit secret`
— that path still needs a manual rollout restart (documented in
docs/tls-values-quickstart.md). Emits nothing when caBundle is unset.

Usage (inside a pod template's metadata.annotations):
  {{- include "camundaPlatform.caBundleChecksumAnnotation" . | nindent 8 }}
*/}}
{{- define "camundaPlatform.caBundleChecksumAnnotation" -}}
{{- /* Gated on autoRollout (default off): the lookup below requires `get` on
       Secrets for the upgrading identity — a Forbidden error there is NOT
       catchable in templates and fails `helm upgrade` — and is inert under
       GitOps `helm template` rendering. Opt in only where the upgrader has
       Secret-read RBAC. See global.tls.caBundle.autoRollout. */ -}}
{{- if and (eq (include "camundaPlatform.hasCaBundle" .) "true") .Values.global.tls.caBundle.autoRollout -}}
{{- $secret := lookup "v1" "Secret" .Release.Namespace .Values.global.tls.caBundle.secret.existingSecret -}}
{{- $key := .Values.global.tls.caBundle.secret.existingSecretKey | default "ca.crt" -}}
{{- /* Hash ONLY the CA-bundle key's bytes, not the whole Secret. The full
       object carries server-managed metadata (resourceVersion, managedFields,
       uid) that controllers (cert-manager, ArgoCD, server-side apply) churn
       without touching the CA; and a Secret may hold co-located keys whose
       rotation is unrelated to the CA. Scoping to existingSecretKey avoids
       spurious rollouts from either. `get` returns "" under helm template /
       --dry-run (no API access), giving a stable sentinel. */ -}}
checksum/ca-bundle: {{ get (($secret | default dict).data | default dict) $key | toYaml | sha256sum }}
{{- end -}}
{{- end -}}

{{/*
caBundleVolume
Emits the volume entry that exposes global.tls.caBundle as a single file
under /etc/camunda/tls/ca.crt inside the container. Always called from a
.spec.volumes list; the caller is responsible for the conditional. Use
hasCaBundle to gate.

The secret may carry the bundle under any key; we re-project it to the
fixed filename "ca.crt" so SSL_CERT_FILE points at a stable path.

Usage:
  {{- if eq (include "camundaPlatform.hasCaBundle" .) "true" }}
  {{- include "camundaPlatform.caBundleVolume" . | nindent 8 }}
  {{- end }}
*/}}
{{- define "camundaPlatform.caBundleVolume" -}}
- name: ca-bundle
  secret:
    secretName: {{ .Values.global.tls.caBundle.secret.existingSecret | quote }}
    items:
      - key: {{ .Values.global.tls.caBundle.secret.existingSecretKey | default "ca.crt" | quote }}
        path: ca.crt
    optional: false
{{- end -}}

{{/*
caBundleVolumeMount
Emits the volumeMount entry pointing at the ca-bundle volume. Mounts
read-only at /etc/camunda/tls so SSL_CERT_FILE=/etc/camunda/tls/ca.crt
resolves.

Usage:
  {{- if eq (include "camundaPlatform.hasCaBundle" .) "true" }}
  {{- include "camundaPlatform.caBundleVolumeMount" . | nindent 12 }}
  {{- end }}
*/}}
{{- define "camundaPlatform.caBundleVolumeMount" -}}
- name: ca-bundle
  mountPath: /etc/camunda/tls
  readOnly: true
{{- end -}}

{{/*
caBundleEnv
Emits env vars pointing at the mounted CA bundle so both native and
Node.js TLS stacks can resolve trust:
  - SSL_CERT_FILE: honoured by the OpenSearch client (post-8.6.7) and many
    native HTTP/TLS libraries that resolve trust through the OS.
  - NODE_EXTRA_CA_CERTS: honoured by Node.js (Web Modeler websockets,
    Console, etc.)

Note: Console gates its own NODE_EXTRA_CA_CERTS behind
`ne (include "camundaPlatform.hasCaBundle" .) "true"` to avoid
duplication when this helper is also included.

Usage (inside an env: list):
  {{- if eq (include "camundaPlatform.hasCaBundle" .) "true" }}
  {{- include "camundaPlatform.caBundleEnv" . | nindent 12 }}
  {{- end }}
*/}}
{{- define "camundaPlatform.caBundleEnv" -}}
- name: SSL_CERT_FILE
  value: /etc/camunda/tls/ca.crt
- name: NODE_EXTRA_CA_CERTS
  value: /etc/camunda/tls/ca.crt
{{- end -}}

{{/*
caBundleInitContainer
Emits an init container that builds a Java truststore (PKCS12-format JKS)
combining the system $JAVA_HOME/lib/security/cacerts with the user CA
mounted by caBundleVolume. Output is written to /var/camunda/tls-truststore/cacerts
in an emptyDir shared with the main container. The main container then
points -Djavax.net.ssl.trustStore at this file.

This is the JVM-side counterpart to caBundleEnv (SSL_CERT_FILE): the
JVM does not honour SSL_CERT_FILE, so we have to give it a real
truststore. Runs `keytool` as a non-root user with readOnlyRootFilesystem
and dropped capabilities.

Usage (inside .spec.initContainers) — pass the calling component's own
resolved image so the init container reuses an image the customer has
already mirrored and inherits that image's cacerts:
  {{- if eq (include "camundaPlatform.hasCaBundle" .) "true" }}
  {{- include "camundaPlatform.caBundleInitContainer" (dict "context" $ "image" (include "camundaPlatform.imageByParams" (dict "base" $.Values.global "overlay" $.Values.orchestration))) | nindent 8 }}
  {{- end }}

Image: defaults to the calling component's own image (the `image` key of
the dict). That image already ships a JRE with keytool, it is one the
customer has already mirrored (so this adds NO new registry dependency in
air-gapped installs), and copying its cacerts preserves any CAs baked into
a custom build. `global.tls.caBundle.image` is an explicit override for the
rare image that lacks keytool; it is used VERBATIM (NOT prefixed with
`global.image.registry`), so it must already be a routable reference.

SecurityContext: comes from global.tls.caBundle.containerSecurityContext,
rendered through common.compatibility.renderSecurityContext so OpenShift's
adaptSecurityContext drops the fixed runAsUser/runAsGroup and lets the
restricted SCC assign a namespace-range UID.
*/}}
{{- define "camundaPlatform.caBundleInitContainer" -}}
{{- $ctx := .context -}}
{{- $img := .image -}}
{{- /* An explicit global.tls.caBundle.image override wins over the
       component image and is used VERBATIM. We deliberately do NOT prepend
       global.image.registry: the override is frequently already fully
       qualified (e.g. myregistry.io/eclipse-temurin:21-jre), and prefixing
       it would double it (myregistry.io/myregistry.io/...) and break the
       pull. The user supplying an override owns its routability. The
       component-image default (.image) is already fully resolved. */ -}}
{{- if $ctx.Values.global.tls.caBundle.image -}}
{{-   $img = $ctx.Values.global.tls.caBundle.image -}}
{{- end -}}
- name: ca-bundle-truststore-init
  image: {{ $img | quote }}
  imagePullPolicy: {{ $ctx.Values.global.tls.caBundle.imagePullPolicy | default $ctx.Values.global.image.pullPolicy | quote }}
  securityContext: {{- include "common.compatibility.renderSecurityContext" (dict "secContext" $ctx.Values.global.tls.caBundle.containerSecurityContext "context" $ctx) | nindent 4 }}
  command: ["sh", "-c"]
  args:
    - |
      set -eu
      umask 022
      # Locate the JRE. Prefer JAVA_HOME; otherwise derive it from keytool
      # on PATH. Reusing the component's own image means we cannot assume a
      # single layout, so resolve it at runtime and call keytool by full
      # path (it is not always on PATH even when JAVA_HOME is set).
      # Fail with a clear message if neither is available, rather than letting
      # the fallback resolve to "." and surface a confusing "cacerts: No such
      # file or directory" later (e.g. a global.tls.caBundle.image override
      # that lacks keytool).
      if [ -z "${JAVA_HOME:-}" ] && ! command -v keytool >/dev/null 2>&1; then
        echo "[ca-bundle-truststore-init] ERROR: no JAVA_HOME set and no keytool on PATH in this image. Set global.tls.caBundle.image to a JRE image that ships keytool." >&2
        exit 1
      fi
      JH="${JAVA_HOME:-$(dirname "$(dirname "$(readlink -f "$(command -v keytool)")")")}"
      KEYTOOL="$JH/bin/keytool"
      # Validate the derivation before using it. The keytool-on-PATH heuristic
      # mis-resolves when keytool is a non-symlink binary (e.g. /usr/bin/keytool
      # gives JH=/usr), so confirm cacerts and keytool actually exist at $JH and
      # fail with an actionable message rather than a confusing later error.
      if [ ! -f "$JH/lib/security/cacerts" ] || [ ! -x "$KEYTOOL" ]; then
        echo "[ca-bundle-truststore-init] ERROR: could not locate a JRE at JH='$JH' (expected \$JH/lib/security/cacerts and \$JH/bin/keytool). Set JAVA_HOME in the image, or set global.tls.caBundle.image to a JRE image with a standard layout." >&2
        exit 1
      fi
      # Java 21 default cacerts is PKCS12; copy it as-is so we keep all
      # public CAs (including any baked into this image) and add the user CA.
      cp -L "$JH/lib/security/cacerts" /var/camunda/tls-truststore/cacerts
      chmod 0644 /var/camunda/tls-truststore/cacerts
      # Split a multi-cert PEM bundle into single-cert files and import
      # each under its own alias. keytool -importcert with a single
      # -alias only ever stores the first cert under that alias and
      # silently discards the rest, so customers who concatenate
      # root + intermediate CAs into one ca.crt would otherwise lose
      # everything but the first. Use a writable workdir under
      # /var/camunda — readOnlyRootFilesystem is set on this container.
      WORK=/var/camunda/tls-truststore/work
      mkdir -p "$WORK"
      # Strip trailing CR first so a CRLF (Windows-authored) PEM still matches
      # the BEGIN/END markers and the split cert files stay LF-clean for keytool.
      awk 'BEGIN{n=0; out=""}
           { sub(/\r$/, "") }
           /-----BEGIN CERTIFICATE-----/ {n++; out=sprintf("'"$WORK"'/cert-%02d.pem", n)}
           out!="" {print > out}
           /-----END CERTIFICATE-----/ {close(out); out=""}' \
           /etc/camunda/tls/ca.crt
      i=0
      for cert in "$WORK"/cert-*.pem; do
        # POSIX sh has no nullglob — when awk produced zero output files,
        # the literal pattern is iterated once. Skip non-files.
        [ -f "$cert" ] || continue
        i=$((i+1))
        "$KEYTOOL" -importcert \
          -noprompt \
          -trustcacerts \
          -keystore /var/camunda/tls-truststore/cacerts \
          -storepass changeit \
          -alias "camunda-user-ca-$i" \
          -file "$cert"
      done
      rm -rf "$WORK"
      if [ "$i" = "0" ]; then
        echo "[ca-bundle-truststore-init] ERROR: no certificates found in /etc/camunda/tls/ca.crt — check that global.tls.caBundle.secret references a non-empty PEM bundle." >&2
        exit 1
      fi
      echo "[ca-bundle-truststore-init] imported $i user CA cert(s) into /var/camunda/tls-truststore/cacerts"
  volumeMounts:
    - name: ca-bundle
      mountPath: /etc/camunda/tls
      readOnly: true
    - name: ca-bundle-truststore
      mountPath: /var/camunda/tls-truststore
{{- end -}}

{{/*
caBundleTruststoreVolume
Emits the emptyDir volume that the init container writes the combined
truststore into. Always paired with caBundleInitContainer in
.spec.initContainers and caBundleTruststoreVolumeMount on the main
container.

Usage (inside .spec.volumes):
  {{- if eq (include "camundaPlatform.hasCaBundle" .) "true" }}
  {{- include "camundaPlatform.caBundleTruststoreVolume" . | nindent 8 }}
  {{- end }}
*/}}
{{- define "camundaPlatform.caBundleTruststoreVolume" -}}
- name: ca-bundle-truststore
  emptyDir: {}
{{- end -}}

{{/*
caBundleTruststoreVolumeMount
Emits the volumeMount for the chart-built truststore on the main
container. Mounted read-only — the init container writes once.

Usage (inside container.volumeMounts):
  {{- if eq (include "camundaPlatform.hasCaBundle" .) "true" }}
  {{- include "camundaPlatform.caBundleTruststoreVolumeMount" . | nindent 12 }}
  {{- end }}
*/}}
{{- define "camundaPlatform.caBundleTruststoreVolumeMount" -}}
- name: ca-bundle-truststore
  mountPath: /var/camunda/tls-truststore
  readOnly: true
{{- end -}}

{{/*
caBundleJavaOpts
Returns the -D arguments needed to point a JVM at the chart-built
combined truststore. Use this when injecting JAVA_TOOL_OPTIONS for a
component that has caBundle set but no component-specific JKS path.

Usage (inside an env: list, after computing the trustStore branch):
  - name: JAVA_TOOL_OPTIONS
    value: {{ printf "%s %s" .Values.<component>.javaOpts (include "camundaPlatform.caBundleJavaOpts" .) | trim | quote }}
*/}}
{{- define "camundaPlatform.caBundleJavaOpts" -}}
-Djavax.net.ssl.trustStore=/var/camunda/tls-truststore/cacerts -Djavax.net.ssl.trustStorePassword=changeit
{{- end -}}

{{/*
caBundleJavaToolOptionsEnv
Emits a JAVA_TOOL_OPTIONS env var pre-populated with caBundleJavaOpts.
Use on components that do NOT already render their own JAVA_TOOL_OPTIONS
(connectors, identity, console, web-modeler websockets) so the chart-built
truststore takes effect with no per-component branching.

Components that already render JAVA_TOOL_OPTIONS conditionally
(orchestration, optimize) should compose caBundleJavaOpts into their
existing branch instead, so user-supplied javaOpts are preserved.

Usage (inside an env: list):
  {{- if eq (include "camundaPlatform.hasCaBundle" .) "true" }}
  {{- include "camundaPlatform.caBundleJavaToolOptionsEnv" . | nindent 12 }}
  {{- end }}
*/}}
{{- define "camundaPlatform.caBundleJavaToolOptionsEnv" -}}
- name: JAVA_TOOL_OPTIONS
  value: {{ include "camundaPlatform.caBundleJavaOpts" . | quote }}
{{- end -}}

{{/*
********************************************************************************
Release highlights.
********************************************************************************
*/}}

{{- define "camundaPlatform.ReleaseHighlights" }}
## [info] Helm chart release highlights
- Some values have been renamed or moved in the new chart structure.
- When upgraded from 8.9 to 8.10, manual adjustments may be required for some cases like custom configurations.
- Please refer to the official docs for more details.
https://docs.camunda.io/docs/self-managed/deployment/helm/upgrade/upgrade-hc-890-8100/
{{- end -}}
