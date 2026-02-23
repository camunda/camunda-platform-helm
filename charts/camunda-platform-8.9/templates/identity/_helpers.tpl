{{/* vim: set filetype=mustache: */}}

{{ define "identity.internalUrl" }}
  {{- if .Values.identity.enabled -}}
    {{-
      printf "http://%s:%v%s"
        (include "identity.fullname" .)
        .Values.identity.service.port
        (.Values.identity.contextPath | default "")
    -}}
  {{- end -}}
{{- end -}}

{{- define "identity.externalUrl" -}}
    {{- if .Values.identity.fullURL -}}
        {{ tpl .Values.identity.fullURL $ }}
    {{- else -}}
        {{- if or .Values.global.ingress.enabled .Values.global.gateway.enabled -}}
            {{- $proto := ternary "https" "http" (or .Values.global.ingress.tls.enabled .Values.global.gateway.tls.enabled) -}}
            {{- $host := (tpl .Values.global.host $ | default (tpl .Values.global.ingress.host $)) -}}
            {{- $path := .Values.identity.contextPath | default "" -}}
            {{- printf "%s://%s%s" $proto $host $path -}}
        {{- else -}}
            {{- "http://localhost:8084" -}}
        {{- end -}}
    {{- end -}}
{{- end -}}

{{/*
Defines extra labels for identity.
*/}}
{{- define "identity.extraLabels" -}}
app.kubernetes.io/component: identity
app.kubernetes.io/version: {{ include "camundaPlatform.versionLabel" (dict "base" .Values.global "overlay" .Values.identity "chart" .Chart) | quote }}
{{- end -}}

{{/*
Define common labels for identity, combining the match labels and transient labels, which might change on updating
(version depending). These labels shouldn't be used on matchLabels selector, since the selectors are immutable.
*/}}
{{- define "identity.labels" -}}
{{- template "camundaPlatform.labels" . }}
{{ template "identity.extraLabels" . }}
{{- end -}}

{{/*
Defines match labels for identity, which are extended by sub-charts and should be used in matchLabels selectors.
*/}}
{{- define "identity.matchLabels" -}}
{{- template "camundaPlatform.matchLabels" . }}
app.kubernetes.io/component: identity
{{- end -}}

{{/*
[identity] Create the name of the service account to use
*/}}
{{- define "identity.serviceAccountName" -}}
    {{- include "camundaPlatform.serviceAccountName" (dict
        "component" "identity"
        "context" $
    ) -}}
{{- end -}}

{{/*
Keycloak helpers
*/}}

{{/*
[identity] Keycloak default URL.
*/}}
{{- define "identity.keycloak.hostDefault" -}}
    {{- if .Values.identityKeycloak.enabled -}}
        {{- template "common.names.fullname" .Subcharts.identityKeycloak -}}
    {{- end -}}
{{- end -}}

{{/*
[identity] Get Keycloak URL protocol based on global value or Keycloak subchart.
*/}}
{{- define "identity.keycloak.protocol" -}}
    {{- if and .Values.global.identity.keycloak.url .Values.global.identity.keycloak.url.protocol -}}
        {{- .Values.global.identity.keycloak.url.protocol -}}
    {{- else -}}
            {{- ternary "https" "http" (.Values.identityKeycloak.tls.enabled) -}}
    {{- end -}}
{{- end -}}

{{/*
[identity] Get Keycloak URL service name based on global value or Keycloak subchart.
This is mainly used to access the external Keycloak service in the global Ingress.
*/}}
{{- define "identity.keycloak.service" -}}
    {{- if and (.Values.global.identity.keycloak.url).host .Values.global.identity.keycloak.internal -}}
        {{- printf "%s-keycloak-custom" .Release.Name | trunc 63 -}}
    {{- else -}}
        {{- include "identity.keycloak.hostDefault" . -}}
    {{- end -}}
{{- end -}}

{{/*
[identity] Get Keycloak URL host based on global value or Keycloak subchart.
*/}}
{{- define "identity.keycloak.host" -}}
    {{- if and .Values.global.identity.keycloak.url .Values.global.identity.keycloak.url.host -}}
        {{- .Values.global.identity.keycloak.url.host -}}
    {{- else -}}
        {{- include "identity.keycloak.hostDefault" . -}}
    {{- end -}}
{{- end -}}


{{/*
[identity] Get Keycloak URL port based on global value or Keycloak subchart.
*/}}
{{- define "identity.keycloak.port" -}}
    {{- if and .Values.global.identity.keycloak.url .Values.global.identity.keycloak.url.port -}}
        {{- .Values.global.identity.keycloak.url.port -}}
    {{- else -}}
        {{- if .Values.identityKeycloak.enabled -}}
            {{- $keycloakProtocol := (include "identity.keycloak.protocol" .) -}}
            {{- get .Values.identityKeycloak.service.ports $keycloakProtocol -}}
        {{- end -}}
    {{- end -}}
{{- end -}}

{{/*
[identity] Get Keycloak contextPath based on global value.
*/}}
{{- define "identity.keycloak.contextPath" -}}
    {{ .Values.global.identity.keycloak.contextPath | default "/auth/" }}
{{- end -}}


{{/*
[identity] Get port part of a url, return empty string if port is 80 or 443.
*/}}
{{- define "identity.keycloak.portUrl" -}}
  {{- if or (eq (include "identity.keycloak.port" .) "80") (eq (include "identity.keycloak.port" .) "443") -}}
      {{- "" -}}
  {{- else -}}
      {{- printf ":%s" (include "identity.keycloak.port" .) -}}
  {{- end -}}
{{- end -}}

{{/*
[identity] Get multitenancy setting
*/}}
{{- define "identity.multitenancyEnabled" -}}
    {{- if or .Values.identityPostgresql.enabled .Values.identity.externalDatabase.enabled }}
        {{- if .Values.identity.multitenancy.enabled -}}
            {{ .Values.identity.multitenancy.enabled }}
        {{- else if .Values.global.multitenancy.enabled -}}
            {{ .Values.global.multitenancy.enabled }}
        {{- else -}}
          false
        {{- end -}}
    {{- else -}}
      false
    {{- end -}}
{{- end -}}

{{/*
[identity] Get Keycloak full URL (protocol, host, port, and contextPath).
*/}}
{{- define "identity.keycloak.url" -}}
    {{-
      printf "%s://%s%s%s"
        (include "identity.keycloak.protocol" .)
        (include "identity.keycloak.host" .)
        (include "identity.keycloak.portUrl" .)
        (include "identity.keycloak.contextPath" .)
    -}}
{{- end -}}

{{/*
[identity] Get Keycloak auth admin user.
Checks the individual key rather than map truthiness to avoid the bug where
setting any key in the auth map causes all helpers to switch source.
*/}}
{{- define "identity.keycloak.authAdminUser" -}}
    {{- if .Values.global.identity.keycloak.auth.adminUser -}}
        {{- .Values.global.identity.keycloak.auth.adminUser -}}
    {{- else -}}
        {{- .Values.identityKeycloak.auth.adminUser -}}
    {{- end -}}
{{- end -}}

{{/*
[identity] Resolve the default Kubernetes Secret name for keycloak admin password
from identityKeycloak subchart values.
*/}}
{{- define "identity.keycloak.defaultAuthSecretName" -}}
    {{- .Values.identityKeycloak.auth.existingSecret | default (printf "%s-keycloak" .Release.Name) -}}
{{- end -}}

{{/*
[identity] Resolve the default key within the Kubernetes Secret for keycloak admin password
from identityKeycloak subchart values.
*/}}
{{- define "identity.keycloak.defaultAuthSecretKey" -}}
    {{- .Values.identityKeycloak.auth.passwordSecretKey | default "admin-password" -}}
{{- end -}}

{{/*
[identity] Normalize keycloak auth password configuration into the standard secret config format
expected by camundaPlatform.normalizeSecretConfiguration.
Priority: new .secret.* keys > legacy existingSecret/existingSecretKey > identityKeycloak subchart defaults
*/}}
{{- define "identity.keycloak.authPasswordConfig" -}}
{{- $auth := .Values.global.identity.keycloak.auth -}}
{{- $config := dict -}}
{{/* New standard keys take priority */}}
{{- if and $auth.secret (or $auth.secret.existingSecret $auth.secret.inlineSecret) -}}
  {{- $_ := set $config "secret" $auth.secret -}}
{{- else if $auth.existingSecret -}}
  {{- $_ := set $config "secret" (dict
      "existingSecret" $auth.existingSecret
      "existingSecretKey" ($auth.existingSecretKey | default "admin-password")
  ) -}}
{{- else -}}
  {{- $_ := set $config "secret" (dict
      "existingSecret" (include "identity.keycloak.defaultAuthSecretName" .)
      "existingSecretKey" (include "identity.keycloak.defaultAuthSecretKey" .)
  ) -}}
{{- end -}}
{{- toYaml $config -}}
{{- end -}}

{{/*
[identity] PostgreSQL helpers.
*/}}

{{- define "identity.postgresql.id" -}}
    {{- (printf "%s-%s" .Release.Name .Values.identityPostgresql.nameOverride) | trunc 63 | trimSuffix "-" }}
{{- end -}}

{{- define "identity.postgresql.secretName" -}}
    {{- $defaultExistingSecret := (include "identity.postgresql.id" .) -}}
    {{- $autExistingSecret := (.Values.identityPostgresql.auth.existingSecret | default $defaultExistingSecret) -}}
    {{- $externalDatabaseExistingSecret := (.Values.identity.externalDatabase.secret.existingSecret | default $defaultExistingSecret) -}}
    {{- .Values.identity.externalDatabase.enabled | ternary $externalDatabaseExistingSecret $autExistingSecret }}
{{- end -}}

{{- define "identity.postgresql.secretKey" -}}
    {{- $defaultSecretKey := "password" -}}
    {{- $authExistingSecretKey := (.Values.identityPostgresql.auth.secretKeys.userPasswordKey | default $defaultSecretKey) -}}
    {{- $externalDatabaseSecretKey := (.Values.identity.externalDatabase.secret.existingSecretKey | default $defaultSecretKey) -}}
    {{- .Values.identity.externalDatabase.enabled | ternary $externalDatabaseSecretKey $authExistingSecretKey }}
{{- end -}}

{{- define "identity.postgresql.host" -}}
    {{- .Values.identity.externalDatabase.enabled | ternary .Values.identity.externalDatabase.host (include "identity.postgresql.id" .) }}
{{- end -}}

{{- define "identity.postgresql.port" -}}
    {{- .Values.identity.externalDatabase.enabled | ternary .Values.identity.externalDatabase.port "5432" }}
{{- end -}}

{{- define "identity.postgresql.username" -}}
    {{- .Values.identity.externalDatabase.enabled | ternary .Values.identity.externalDatabase.username .Values.identityPostgresql.auth.username }}
{{- end -}}

{{- define "identity.postgresql.database" -}}
    {{- .Values.identity.externalDatabase.enabled | ternary .Values.identity.externalDatabase.database .Values.identityPostgresql.auth.database }}
{{- end -}}

{{/*
[identity] Get the image pull secrets.
*/}}
{{- define "identity.imagePullSecrets" -}}
    {{- include "camundaPlatform.subChartImagePullSecrets" (dict "Values" (set (deepCopy .Values) "image" .Values.identity.image)) }}
{{- end }}
