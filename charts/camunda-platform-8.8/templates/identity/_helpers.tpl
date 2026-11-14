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
        {{- if .Values.global.ingress.enabled -}}
            {{- $proto := include "camundaPlatform.ingressProtocol" . -}}
            {{- $host := .Values.global.ingress.host -}}
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
[identity] Fail in case Keycloak chart is disabled and existing Keycloak URL is not configured.
*/}}
{{- define "identity.keycloak.isConfigured" -}}
{{- $failMessageRaw := `
[identity] To configure Keycloak, you have 3 options:

  - Case 1: If you want to deploy Keycloak chart as it is, then set the following:
    - keycloak.enabled: true

  - Case 2: If you want to customize the Keycloak chart URL, then set the following:
    - keycloak.enabled: true
    - global.identity.keycloak.url.protocol
    - global.identity.keycloak.url.host
    - global.identity.keycloak.url.port

  - Case 3: If you want to use already existing Keycloak, then set the following:
    - keycloak.enabled: false
    - global.identity.keycloak.url.protocol
    - global.identity.keycloak.url.host
    - global.identity.keycloak.url.port
    - global.identity.keycloak.auth.adminUser
    - global.identity.keycloak.auth.existingSecret

For more details, please check Camunda Helm chart documentation.
` -}}
    {{- $failMessage := printf "\n%s" $failMessageRaw | trimSuffix "\n" -}}

    {{- if .Values.global.identity.keycloak.url -}}
        {{- $_ := required $failMessage .Values.global.identity.keycloak.url.protocol -}}
        {{- $_ := required $failMessage .Values.global.identity.keycloak.url.host -}}
        {{- $_ := required $failMessage .Values.global.identity.keycloak.url.port -}}
    {{- end -}}

    {{- if .Values.global.identity.keycloak.auth -}}
        {{- $_ := required $failMessage .Values.global.identity.keycloak.auth.adminUser -}}
        {{- $_ := required $failMessage .Values.global.identity.keycloak.auth.existingSecret -}}
    {{- end -}}
{{- end -}}

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
    {{- include "identity.keycloak.isConfigured" . -}}
    {{-
      printf "%s://%s%s%s"
        (include "identity.keycloak.protocol" .)
        (include "identity.keycloak.host" .)
        (include "identity.keycloak.portUrl" .)
        (include "identity.keycloak.contextPath" .)
    -}}
{{- end -}}

{{/*
[identity] Get Keycloak auth admin user. For more details:
*/}}
{{- define "identity.keycloak.authAdminUser" -}}
    {{- if .Values.global.identity.keycloak.auth }}
        {{- .Values.global.identity.keycloak.auth.adminUser -}}
    {{- else }}
        {{- .Values.identityKeycloak.auth.adminUser -}}
    {{- end }}
{{- end -}}

{{/*
[identity] Get name of Keycloak auth existing secret. For more details:
https://docs.bitnami.com/kubernetes/apps/keycloak/configuration/manage-passwords/
*/}}
{{- define "identity.keycloak.authExistingSecret" -}}
    {{- if .Values.global.identity.keycloak.auth }}
        {{- .Values.global.identity.keycloak.auth.existingSecret -}}
    {{- else if .Values.identityKeycloak.auth.existingSecret }}
        {{- .Values.identityKeycloak.auth.existingSecret }}
    {{- else -}}
        {{ .Release.Name }}-keycloak
    {{- end }}
{{- end -}}

{{/*
[identity] Get Keycloak auth existing secret key.
*/}}
{{- define "identity.keycloak.authExistingSecretKey" -}}
    {{- if .Values.global.identity.keycloak.auth }}
        {{- .Values.global.identity.keycloak.auth.existingSecretKey -}}
    {{- else if .Values.identityKeycloak.auth.passwordSecretKey }}
        {{- .Values.identityKeycloak.auth.passwordSecretKey }}
    {{- else -}}
        admin-password
    {{- end }}
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
    {{- $externalDatabaseExistingSecret := (.Values.identity.externalDatabase.existingSecret | default $defaultExistingSecret) -}}
    {{- .Values.identity.externalDatabase.enabled | ternary $externalDatabaseExistingSecret $autExistingSecret }}
{{- end -}}

{{- define "identity.postgresql.secretKey" -}}
    {{- $defaultSecretKey := "password" -}}
    {{- $authExistingSecretKey := (.Values.identityPostgresql.auth.secretKeys.userPasswordKey | default $defaultSecretKey) -}}
    {{- $externalDatabaseSecretKey := (.Values.identity.externalDatabase.existingSecretPasswordKey | default $defaultSecretKey) -}}
    {{- .Values.identity.externalDatabase.enabled | ternary $externalDatabaseSecretKey $authExistingSecretKey }}
{{- end -}}

{{- define "identity.postgresql.secretPassword" -}}
    {{- $authPassword := .Values.identityPostgresql.auth.password -}}
    {{- .Values.identity.externalDatabase.enabled | ternary .Values.identity.externalDatabase.password $authPassword }}
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
    - name: KEYCLOAK_SETUP_PASSWORD
      valueFrom:
        secretKeyRef:
          name: {{ include "identity.keycloak.authExistingSecret" . }}
          key: {{ include "identity.keycloak.authExistingSecretKey" . }}
  volumeMounts:
    - mountPath: /tmp
      name: keycloak-admin-api-scope-tmp
{{- end -}}
