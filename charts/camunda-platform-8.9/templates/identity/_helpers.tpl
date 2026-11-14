{{/* vim: set filetype=mustache: */}}

{{ define "identity.internalUrl" }}
  {{- if eq (include "camundaPlatform.identityEnabled" .) "true" -}}
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
            {{- $proto := ternary (include "camundaPlatform.ingressProtocol" .) (ternary "https" "http" .Values.global.gateway.tls.enabled) .Values.global.ingress.enabled -}}
            {{- $host := (tpl .Values.global.host $ | default (tpl .Values.global.ingress.host $)) -}}
            {{- $path := .Values.identity.contextPath | default "" -}}
            {{- printf "%s://%s%s" $proto $host $path -}}
        {{- else -}}
            {{- "http://localhost:8084" -}}
        {{- end -}}
    {{- end -}}
{{- end -}}

{{- define "identity.extraLabels" -}}
    {{- include "camundaPlatform.componentExtraLabels" (dict "componentName" "identity" "componentValuesKey" "identity" "context" $) -}}
{{- end -}}

{{- define "identity.labels" -}}
    {{- include "camundaPlatform.componentLabels" (dict "componentName" "identity" "componentValuesKey" "identity" "context" $) -}}
{{- end -}}

{{- define "identity.matchLabels" -}}
    {{- include "camundaPlatform.componentMatchLabels" (dict "componentName" "identity" "context" $) -}}
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
        {{- tpl .Values.global.identity.keycloak.url.host . -}}
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
    {{- .Values.global.identity.keycloak.contextPath -}}
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

{{/*
[identity] Whether to render the sidecar that assigns the Keycloak client scope
"kc-admin-api" to the Identity client. Keycloak 26.8.2 (CVE-2026-19688) rejects Admin REST API
tokens without this scope, and Identity calls the Admin API with its own client.
Only for the bundled Keycloak, whose image and admin credentials the chart knows.
*/}}
{{- define "identity.keycloakAdminApiScope.enabled" -}}
    {{- and
        .Values.global.identity.auth.enabled
        .Values.identityKeycloak.enabled
        (eq (include "camundaPlatform.authIssuerType" .) "KEYCLOAK")
    -}}
{{- end -}}

{{/*
[identity] Keycloak realm of Identity, as in the "keycloak.realm" setting of the configmap.
*/}}
{{- define "identity.keycloak.realm" -}}
    {{- if and .Values.identity.realm .Values.identity.realm.enabled -}}
        {{- .Values.identity.realm.id -}}
    {{- else -}}
        camunda-platform
    {{- end -}}
{{- end -}}

{{/*
[identity] Image pull secrets of the Identity pod. With the kc-admin-api sidecar, the
pull secrets of the Keycloak image are added, because the sidecar uses that image.
*/}}
{{- define "identity.podImagePullSecrets" -}}
    {{- $secrets := include "identity.imagePullSecrets" . | fromYamlArray -}}
    {{- if eq (include "identity.keycloakAdminApiScope.enabled" .) "true" -}}
        {{- range (.Values.identityKeycloak.image.pullSecrets | default .Values.global.image.pullSecrets | default list) -}}
            {{- $secret := kindIs "string" . | ternary (dict "name" .) . -}}
            {{- if not (has $secret $secrets) -}}
                {{- $secrets = append $secrets $secret -}}
            {{- end -}}
        {{- end -}}
    {{- end -}}
    {{- $secrets | toYaml -}}
{{- end -}}

{{/*
[identity] Sidecar that assigns the Keycloak client scope "kc-admin-api" to the Identity client
as a default client scope. Identity creates the realm and the client on its first start, and then
stops with HTTP 401 on Keycloak 26.8.2 until the client has the scope; the sidecar assigns it, and
the next start of Identity succeeds. The sidecar does nothing if Keycloak has no such scope (before
26.8.2) or if the scope is already assigned, and checks again every few minutes.
*/}}
{{- define "identity.keycloakAdminApiScope.container" -}}
- name: keycloak-admin-api-scope
  image: {{ include "camundaPlatform.imageByParams" (dict "base" .Values.global "overlay" .Values.identityKeycloak) }}
  imagePullPolicy: {{ .Values.global.image.pullPolicy }}
  {{- if .Values.identity.containerSecurityContext }}
  securityContext: {{- include "common.compatibility.renderSecurityContext" (dict "secContext" .Values.identity.containerSecurityContext "context" .) | nindent 4 }}
  {{- end }}
  command: ["/bin/sh", "-c"]
  args:
    - |
      set -u
      export HOME=/tmp
      cfg=/tmp/kcadm.config
      kc() { kcadm.sh "$@" --config "$cfg"; }
      ensure() {
        kc config credentials --server "$KEYCLOAK_URL" --realm master \
          --user "$KEYCLOAK_SETUP_USER" --password "$KEYCLOAK_SETUP_PASSWORD" >/dev/null 2>&1 || return 1
        kc get "realms/$KEYCLOAK_REALM" --fields id >/dev/null 2>&1 || return 1
        scope=""
        for row in $(kc get client-scopes -r "$KEYCLOAK_REALM" --fields id,name --format csv --noquotes); do
          [ "${row#*,}" = "kc-admin-api" ] && scope="${row%%,*}"
        done
        [ -n "$scope" ] || return 0
        client=$(kc get clients -r "$KEYCLOAK_REALM" -q "clientId=$IDENTITY_CLIENT_ID" --fields id --format csv --noquotes)
        [ -n "$client" ] || return 1
        for id in $(kc get "clients/$client/default-client-scopes" -r "$KEYCLOAK_REALM" --fields id --format csv --noquotes); do
          [ "$id" = "$scope" ] && return 0
        done
        kc update "clients/$client/default-client-scopes/$scope" -r "$KEYCLOAK_REALM" || return 1
        echo "kc-admin-api: assigned to client '$IDENTITY_CLIENT_ID' in realm '$KEYCLOAK_REALM'."
      }
      while true; do
        if ensure; then sleep 300; else sleep 5; fi
      done
  resources:
    requests:
      cpu: 10m
      memory: 64Mi
    limits:
      memory: 256Mi
  env:
    - name: KC_OPTS
      value: "-Xmx96m -XX:+UseSerialGC"
    - name: KEYCLOAK_URL
      value: {{ include "identity.keycloak.url" . | quote }}
    - name: KEYCLOAK_REALM
      value: {{ include "identity.keycloak.realm" . | quote }}
    - name: IDENTITY_CLIENT_ID
      value: {{ include "identity.authClientId" . | quote }}
    - name: KEYCLOAK_SETUP_USER
      value: {{ include "identity.keycloak.authAdminUser" . | quote }}
    {{- include "camundaPlatform.emitEnvVarFromSecretConfig" (dict
        "envName" "KEYCLOAK_SETUP_PASSWORD"
        "config" (include "identity.keycloak.authPasswordConfig" . | fromYaml)
    ) | nindent 4 }}
  volumeMounts:
    - mountPath: /tmp
      name: keycloak-admin-api-scope-tmp
{{- end -}}
