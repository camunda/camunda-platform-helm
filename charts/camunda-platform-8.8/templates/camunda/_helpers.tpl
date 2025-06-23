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
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version | replace "+" "_" }}
{{- end }}

{{/*
Common match labels, which are extended by sub-charts and should be used in matchLabels selectors.
*/}}
{{- define "camundaPlatform.matchLabels" -}}
app.kubernetes.io/name: {{ template "camundaPlatform.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: camunda-platform
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
Get image according the values of "global" or "subchart" values.
Usage: {{ include "camundaPlatform.image" . }}
*/}}
{{- define "camundaPlatform.image" -}}
    {{ include "camundaPlatform.imageByParams" (dict "base" .Values.global "overlay" .Values) }}
{{- end -}}

{{/*
Return the version label for resources.
If an image digest is specified without a tag, fall back to .Chart.AppVersion (e.g., “8.8.x”); otherwise use the resolved image tag.
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
Get imagePullSecrets according the values of global, subchart, or empty.
*/}}
{{- define "camundaPlatform.subChartImagePullSecrets" -}}
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
[camunda-platform] Get Camunda license secret name.
*/}}
{{- define "camundaPlatform.licenseSecretName" -}}
  {{- $defaultSecretName := printf "%s-license" (include "camundaPlatform.fullname" .) -}}
  {{- .Values.global.license.existingSecret | default $defaultSecretName -}}
{{- end -}}

{{/*
[camunda-platform] Get Camunda license secret key.
*/}}
{{- define "camundaPlatform.licenseSecretKey" -}}
  {{- $defaultSecretKey := "CAMUNDA_LICENSE_KEY" -}}
  {{- .Values.global.license.existingSecretKey | default $defaultSecretKey -}}
{{- end -}}

{{/*
********************************************************************************
Keycloak templates.
********************************************************************************
*/}}

{{/*
[camunda-platform] Keycloak issuer public URL which used externally for Camunda apps.
*/}}
{{- define "camundaPlatform.authIssuerUrl" -}}
  {{- if .Values.global.identity.auth.issuer -}}
    {{- .Values.global.identity.auth.issuer -}}
  {{- else -}}
    {{- tpl .Values.global.identity.auth.publicIssuerUrl . -}}
  {{- end -}}
{{- end -}}

{{/*
[camunda-platform] Keycloak issuer backend URL which used internally for Camunda apps.
TODO: Most of the Keycloak config is handeled in Identity sub-chart, but it should be in the main chart.
*/}}
{{- define "camundaPlatform.authIssuerBackendUrl" -}}
  {{- if .Values.global.identity.auth.issuerBackendUrl -}}
    {{- .Values.global.identity.auth.issuerBackendUrl -}}
  {{- else -}}
    {{- if .Values.global.identity.keycloak.url -}}
      {{-
        printf "%s://%s:%v%s%s"
          .Values.global.identity.keycloak.url.protocol
          .Values.global.identity.keycloak.url.host
          .Values.global.identity.keycloak.url.port
          .Values.global.identity.keycloak.contextPath
          .Values.global.identity.keycloak.realm
      -}}
    {{- else -}}
      {{- include "identity.keycloak.url" . -}}{{- .Values.global.identity.keycloak.realm -}}
    {{- end -}}
  {{- end -}}
{{- end -}}

{{/*
[camunda-platform] Identity auth type which used internally for Camunda apps.
*/}}
{{- define "camundaPlatform.authType" -}}
  {{- .Values.global.identity.auth.type -}}
{{- end -}}

{{/*
[camunda-platform] Keycloak auth token URL which used internally for Camunda apps.
*/}}
{{- define "camundaPlatform.authIssuerBackendUrlTokenEndpoint" -}}
  {{- if .Values.global.identity.auth.tokenUrl -}}
    {{- .Values.global.identity.auth.tokenUrl -}}
  {{- else -}}
    {{- include "camundaPlatform.authIssuerBackendUrl" . -}}/protocol/openid-connect/token
  {{- end -}}
{{- end -}}


{{/*
[camunda-platform] Keycloak auth certs URL which used internally for Camunda apps.
*/}}
{{- define "camundaPlatform.authIssuerBackendUrlCertsEndpoint" -}}
  {{- if .Values.global.identity.auth.jwksUrl -}}
    {{- .Values.global.identity.auth.jwksUrl -}}
  {{- else -}}
    {{- include "camundaPlatform.authIssuerBackendUrl" . -}}/protocol/openid-connect/certs
  {{- end -}}
{{- end -}}

{{/*
Get the external url for keycloak
*/}}
{{- define "camundaPlatform.keycloakExternalURL" -}}
  {{ if .Values.identityKeycloak.ingress.enabled -}}
    {{- $proto := ternary "https" "http" .Values.identityKeycloak.ingress.tls -}}
    {{- printf "%s://%s%s" $proto .Values.identityKeycloak.ingress.hostname .Values.identityKeycloak.httpRelativePath -}}
  {{ else if .Values.identityKeycloak.enabled -}}
    {{- $proto := ternary "https" "http" .Values.global.ingress.tls.enabled -}}
    {{- printf "%s://%s%s" $proto (.Values.global.ingress.host | default "localhost:18080") .Values.global.identity.keycloak.contextPath -}}
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
  {{- tpl .Values.global.elasticsearch.url.host $ -}}
{{- end -}}

{{- define "camundaPlatform.elasticsearchURL" -}}
    {{ .Values.global.elasticsearch.url.protocol }}://{{ include "camundaPlatform.elasticsearchHost" . }}:{{ .Values.global.elasticsearch.url.port }}
{{- end -}}

{{- define "camundaPlatform.opensearchHost" -}}
  {{- tpl .Values.global.opensearch.url.host $ -}}
{{- end -}}

{{- define "camundaPlatform.opensearchURL" -}}
    {{ .Values.global.opensearch.url.protocol }}://{{ include "camundaPlatform.opensearchHost" . }}:{{ .Values.global.opensearch.url.port }}
{{- end -}}

{{/*
[elasticsearch] Get name of elasticsearch auth existing secret. For more details:
https://docs.bitnami.com/kubernetes/apps/keycloak/configuration/manage-passwords/
*/}}
{{- define "elasticsearch.authExistingSecret" -}}
    {{- if .Values.global.elasticsearch.auth.existingSecret }}
        {{- .Values.global.elasticsearch.auth.existingSecret -}}
    {{- else -}}
        {{ include "camundaPlatform.fullname" . }}-elasticsearch
    {{- end }}
{{- end -}}

{{/*
[elasticsearch] Get elasticsearch auth existing secret key.
*/}}
{{- define "elasticsearch.authExistingSecretKey" -}}
    {{- if .Values.global.elasticsearch.auth.existingSecretKey }}
        {{- .Values.global.elasticsearch.auth.existingSecretKey -}}
    {{- else -}}
        password
    {{- end }}
{{- end -}}

{{/*
[elasticsearch] Used as a boolean to determine whether any password is defined.
do not use this for its string value.
*/}}
{{- define "elasticsearch.passwordIsDefined" -}}
{{- (cat .Values.global.elasticsearch.auth.existingSecret .Values.global.elasticsearch.auth.password) -}}
{{- end -}}


{{/*
[opensearch] Get name of elasticsearch auth existing secret. For more details:
https://docs.bitnami.com/kubernetes/apps/keycloak/configuration/manage-passwords/
*/}}
{{- define "opensearch.authExistingSecret" -}}
    {{- if .Values.global.opensearch.auth.existingSecret }}
        {{- .Values.global.opensearch.auth.existingSecret -}}
    {{- else -}}
        {{ include "camundaPlatform.fullname" . }}-opensearch
    {{- end }}
{{- end -}}

{{/*
[opensearch] Get opensearch auth existing secret key.
*/}}
{{- define "opensearch.authExistingSecretKey" -}}
    {{- if .Values.global.opensearch.auth.existingSecretKey }}
        {{- .Values.global.opensearch.auth.existingSecretKey -}}
    {{- else -}}
        password
    {{- end }}
{{- end -}}

{{/*
********************************************************************************
Operate templates.
********************************************************************************
*/}}

{{/*
Get the external url for a given component.
If the "overlay" values exist, they will override the "base" values, otherwise the "base" values will be used.
Usage: {{ include "camundaPlatform.getExternalURL" (dict "component" "operate" "context" .) }}
*/}}
{{- define "camundaPlatform.getExternalURL" -}}
  {{- if (index .context.Values .component "enabled") -}}
    {{- if $.context.Values.global.ingress.enabled -}}
      {{ $proto := ternary "https" "http" .context.Values.global.ingress.tls.enabled -}}
      {{- printf "%s://%s%s" $proto .context.Values.global.ingress.host (index .context.Values .component "contextPath") -}}
    {{- else -}}
      {{- $portMapping := (dict
      "operate" "8081"
      "identity" "8080"
      "tasklist" "8082"
      "optimize" "8083"
      "webapp" "8084"
      "websockets" "8085"
      "console" "8087"
      "connectors" "8086"
      "zeebeGateway" "26500"
      ) -}}
      {{- printf "http://localhost:%s" (get $portMapping .component) -}}
    {{- end -}}
  {{- end -}}
{{- end -}}

{{/*
[camunda-platform] Operate external URL.
*/}}
{{- define "camundaPlatform.operateExternalURL" }}
  {{- printf "%s/operate" (include "camundaPlatform.coreExternalURL" .) -}}
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
  {{- printf "%s" (include "camundaPlatform.getExternalURL" (dict "component" "connectors" "context" .)) -}}
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
  {{- printf "%s/tasklist" (include "camundaPlatform.coreExternalURL" .) -}}
{{- end -}}


{{/*
********************************************************************************
Core Identity templates.
********************************************************************************
*/}}

{{/*
[camunda-platform] Core Identity external URL.
*/}}
{{- define "camundaPlatform.coreIdentityExternalURL" }}
  {{- printf "%s/identity" (include "camundaPlatform.coreExternalURL" .) -}}
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
  {{- if .context.Values.webModeler.enabled -}}
    {{- if $.context.Values.global.ingress.enabled -}}
      {{ $proto := ternary "https" "http" .context.Values.global.ingress.tls.enabled -}}
      {{- if eq .component "websockets" }}
        {{- printf "%s://%s%s" $proto .context.Values.global.ingress.host (include "webModeler.websocketContextPath" .context) -}}
      {{- else -}}
        {{- printf "%s://%s%s" $proto .context.Values.global.ingress.host (index .context.Values.webModeler "contextPath") -}}
      {{- end -}}
    {{- else -}}
      {{- if eq .component "websockets" -}}
        {{- printf "http://localhost:8085" -}}
      {{- else -}}
        {{- printf "http://localhost:8084" -}}
      {{- end -}}
    {{- end -}}
  {{- end -}}
{{- end -}}

{{- define "camundaPlatform.webModelerWebSocketsExternalURL" }}
  {{- printf "%s" (include "camundaPlatform.getExternalURLModeler" (dict "component" "websockets" "context" .)) -}}
{{- end -}}

{{- define "camundaPlatform.webModelerWebAppExternalURL" }}
  {{- printf "%s" (include "camundaPlatform.getExternalURLModeler" (dict "component" "webapp" "context" .)) -}}
{{- end -}}


{{/*
********************************************************************************
Identity templates.
********************************************************************************
*/}}

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
Console templates.
********************************************************************************
*/}}
{{/*
[camunda-platform] Console external URL.
*/}}
{{- define "camundaPlatform.consoleExternalURL" }}
  {{- printf "%s" (include "camundaPlatform.getExternalURL" (dict "component" "console" "context" .)) -}}
{{- end -}}


{{/*
********************************************************************************
Core templates.
********************************************************************************
*/}}

{{/*
[camunda-platform] Core internal URL.
*/}}
{{ define "camundaPlatform.CoreURL" }}
  {{- if .Values.core.enabled -}}
    {{-
      printf "http://%s:%v%s"
        (include "core.fullname" .)
        .Values.core.service.httpPort
        .Values.core.contextPath
    -}}
  {{- end -}}
{{- end -}}

{{/*
********************************************************************************
Zeebe templates.
********************************************************************************
*/}}
{{/*
[camunda-platform] Zeebe Gateway external URL.
*/}}
{{- define "camundaPlatform.coreExternalURL" }}
  {{- if .Values.global.ingress.enabled -}}
    {{ $proto := ternary "https" "http" .Values.global.ingress.tls.enabled -}}
    {{- printf "%s://%s%s" $proto .Values.global.ingress.host .Values.core.contextPath -}}
  {{- else -}}
    {{- printf "http://localhost:8088" -}}
  {{- end -}}
{{- end -}}

{{/*
[camunda-platform] Zeebe Gateway GRPC external URL.
*/}}
{{- define "camundaPlatform.coreGRPCExternalURL" -}}
  {{ $proto := ternary "https" "http" .Values.core.ingress.grpc.tls.enabled -}}
  {{- printf "%s://%s" $proto (tpl .Values.core.ingress.grpc.host . | default "localhost:26500") -}}
{{- end -}}

{{/*
[camunda-platform] Zeebe Gateway REST internal URL.
*/}}
{{ define "camundaPlatform.zeebeGatewayRESTURL" }}
  {{- if .Values.core.enabled -}}
    {{-
      printf "http://%s:%v%s"
        (include "core.fullname" .)
        .Values.core.service.httpPort
        (.Values.core.contextPath | default "")
    -}}
  {{- end -}}
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
  {{- $baseURL := printf "%s://%s" $proto .Values.global.ingress.host }}

  {{- if .Values.console.enabled }}
  {{-  $proto := (lower .Values.console.readinessProbe.scheme) -}}
  {{- $baseURLInternal := printf "%s://%s.%s:%v" $proto (include "console.fullname" .) .Release.Namespace .Values.console.service.managementPort }}
  - name: Console
    id: console
    version: {{ include "camundaPlatform.imageTagByParams" (dict "base" .Values.global "overlay" .Values.console) }}
    url: {{ include "camundaPlatform.consoleExternalURL" . }}
    readiness: {{ printf "%s%s" $baseURLInternal .Values.console.readinessProbe.probePath }}
    metrics: {{ printf "%s%s" $baseURLInternal .Values.console.metrics.prometheus }}
  {{- end }}
  {{ if .Values.identity.enabled -}}
  {{-  $proto := (lower .Values.identity.readinessProbe.scheme) -}}
  {{- $baseURLInternal := printf "%s://%s.%s:%v" $proto (include "identity.fullname" .) .Release.Namespace .Values.identity.service.metricsPort -}}
  - name: Keycloak
    id: keycloak
    version: {{ .Values.identityKeycloak.image.tag }}
    url: {{ include "camundaPlatform.keycloakExternalURL" . }}
  - name: Identity
    id: identity
    version: {{ include "camundaPlatform.imageTagByParams" (dict "base" .Values.global "overlay" .Values.identity) }}
    url: {{ include "camundaPlatform.identityExternalURL" . }}
    readiness: {{ printf "%s%s" $baseURLInternal .Values.identity.readinessProbe.probePath }}
    metrics: {{ printf "%s%s" $baseURLInternal .Values.identity.metrics.prometheus }}
  {{- end }}

  {{- if .Values.webModeler.enabled }}
  {{-  $proto := (lower .Values.webModeler.webapp.readinessProbe.scheme) -}}
  {{- $baseURLInternal := printf "%s://%s.%s:%v" $proto (include "webModeler.webapp.fullname" .) .Release.Namespace .Values.webModeler.webapp.service.managementPort }}
  - name: WebModeler WebApp
    id: webModelerWebApp
    version: {{ include "camundaPlatform.imageTagByParams" (dict "base" .Values.global "overlay" .Values.webModeler) }}
    url: {{ include "camundaPlatform.webModelerWebAppExternalURL" . }}
    readiness: {{ printf "%s%s" $baseURLInternal .Values.webModeler.webapp.readinessProbe.probePath }}
    metrics: {{ printf "%s%s" $baseURLInternal .Values.webModeler.webapp.metrics.prometheus }}
  {{- end }}

  {{- if .Values.optimize.enabled }}
  {{-  $proto := (lower .Values.optimize.readinessProbe.scheme) -}}
  {{- $baseURLInternal := printf "%s://%s.%s" $proto (include "optimize.fullname" .) .Release.Namespace }}
  - name: Optimize
    id: optimize
    version: {{ include "camundaPlatform.imageTagByParams" (dict "base" .Values.global "overlay" .Values.optimize) }}
    url: {{ include "camundaPlatform.optimizeExternalURL" . }}
    readiness: {{ printf "%s:%v%s%s" $baseURLInternal .Values.optimize.service.port .Values.optimize.contextPath .Values.optimize.readinessProbe.probePath }}
    metrics: {{ printf "%s:%v%s" $baseURLInternal .Values.optimize.service.managementPort .Values.optimize.metrics.prometheus }}
  {{- end }}

  {{- if .Values.connectors.enabled }}
  {{-  $proto := (lower .Values.connectors.readinessProbe.scheme) -}}
  {{- $baseURLInternal := printf "%s://%s.%s" $proto (include "connectors.serviceName" .) .Release.Namespace }}
  - name: Connectors
    id: connectors
    version: {{ include "camundaPlatform.imageTagByParams" (dict "base" .Values.global "overlay" .Values.connectors) }}
    url: {{ include "camundaPlatform.connectorsExternalURL" . }}
    readiness: {{ printf "%s:%v%s%s" $baseURLInternal .Values.connectors.service.serverPort .Values.connectors.contextPath .Values.connectors.readinessProbe.probePath }}
    metrics: {{ printf "%s:%v%s" $baseURLInternal .Values.connectors.service.serverPort .Values.connectors.metrics.prometheus }}
  {{- end }}

  {{- if .Values.core.enabled }}
  {{-  $proto := (lower .Values.core.readinessProbe.scheme) -}}
  {{- $baseURLInternal := printf "%s://%s.%s:%v" $proto (include "core.fullname" . | trimAll "\"") .Release.Namespace .Values.core.service.managementPort }}
  - name: Operate
    id: operate
    version: {{ include "camundaPlatform.imageTagByParams" (dict "base" .Values.global "overlay" .Values.core) }}
    url: {{ include "camundaPlatform.operateExternalURL" . }}
    readiness: {{ printf "%s%s%s" $baseURLInternal .Values.core.contextPath .Values.core.readinessProbe.probePath }}
    metrics: {{ printf "%s%s%s" $baseURLInternal .Values.core.contextPath .Values.core.metrics.prometheus }}
  - name: Tasklist
    id: tasklist
    version: {{ include "camundaPlatform.imageTagByParams" (dict "base" .Values.global "overlay" .Values.core) }}
    url: {{ include "camundaPlatform.tasklistExternalURL" . }}
    readiness: {{ printf "%s%s%s" $baseURLInternal .Values.core.contextPath .Values.core.readinessProbe.probePath }}
    metrics: {{ printf "%s%s%s" $baseURLInternal .Values.core.contextPath .Values.core.metrics.prometheus }}
  - name: Core Identity
    id: coreIdentity
    version: {{ include "camundaPlatform.imageTagByParams" (dict "base" .Values.global "overlay" .Values.core) }}
    url: {{ include "camundaPlatform.coreIdentityExternalURL" . }}
    readiness: {{ printf "%s%s%s" $baseURLInternal .Values.core.contextPath .Values.core.readinessProbe.probePath }}
    metrics: {{ printf "%s%s%s" $baseURLInternal .Values.core.contextPath .Values.core.metrics.prometheus }}

  - name: Orchestration Core
    id: core
    version: {{ include "camundaPlatform.imageTagByParams" (dict "base" .Values.global "overlay" .Values.core) }}
    urls:
      grpc: {{ include "camundaPlatform.coreGRPCExternalURL" . }}
      http: {{ include "camundaPlatform.coreExternalURL" . }}
    readiness: {{ printf "%s%s%s" $baseURLInternal .Values.core.contextPath .Values.core.readinessProbe.probePath }}
    metrics: {{ printf "%s%s%s" $baseURLInternal .Values.core.contextPath .Values.core.metrics.prometheus }}
  {{- end }}
{{- end -}}
