{{/*
A template to handle constraints.
*/}}

{{/*
Fail with a message if the Helm CLI version is less than v4.
Chart 15.x (Camunda 8.10) requires Helm v4 or later.
*/}}
{{- if not (semverCompare ">=4.0.0-0" .Capabilities.HelmVersion.Version) -}}
{{- fail (printf "[camunda][error] Camunda chart 15.x (8.10) requires Helm CLI v4 or later. Detected Helm CLI version: %s. Please upgrade to Helm v4: https://helm.sh/docs/topics/v4_migration/" .Capabilities.HelmVersion.Version) -}}
{{- end -}}

{{- $values := .Values | toYaml | fromYaml }}

{{ include "camundaPlatform.keyRenamed" (dict
  "condition" (ne nil (dig "camundaHub" "webModeler" nil $values))
  "oldName" "camundaHub.webModeler.*"
  "newName" "camundaHub.*"
) }}

{{ include "camundaPlatform.keyRenamed" (dict
  "condition" (ne nil (dig "camundaHub" "console" nil $values))
  "oldName" "camundaHub.console.*"
  "newName" "console.*"
) }}

{{ include "camundaPlatform.keyRenamed" (dict
  "condition" (ne nil (dig "identity" "auth" "camundaHub" "webModeler" nil .Values.global))
  "oldName" "global.identity.auth.camundaHub.webModeler.*"
  "newName" "global.identity.auth.camundaHub.*"
) }}

{{- $identityEnabled := (or .Values.identity.enabled .Values.global.identity.service.url) }}
{{- $identityAuthEnabled := (or $identityEnabled .Values.global.identity.auth.enabled) }}

{{/*
Fail with a message if Multi-Tenancy is enabled and its requirements are not met which are:
- Identity chart/service.
- Identity PostgreSQL chart/service.
- Identity Auth enabled for other apps.
Multi-Tenancy requirements: https://docs.camunda.io/docs/self-managed/concepts/multi-tenancy/
*/}}
{{- if or .Values.identity.multitenancy.enabled .Values.global.multitenancy.enabled }}
  {{- $identityDatabaseEnabled := .Values.identity.externalDatabase.enabled }}
  {{- if has false (list $identityAuthEnabled $identityDatabaseEnabled) }}
    {{- $errorMessage := printf "[camunda][error] %s %s %s"
        "The Multi-Tenancy feature \"identity.multitenancy\" requires Identity enabled and configured with database."
        "Ensure that \"identity.enabled: true\" and \"global.identity.auth.enabled: true\""
        "and configure an external database via \"identity.externalDatabase\"."
    -}}
    {{ printf "\n%s" $errorMessage | trimSuffix "\n"| fail }}
  {{- end }}
{{- end }}

{{/*
Fail if there is no secondary storage type specified and if noSecondaryStorage is not enabled.
*/}}
{{- if and .Values.orchestration.enabled (eq (include "orchestration.secondaryStorage" .) "unset") }}
  {{- fail "Please configure an expected secondary storage type under `orchestration.data.secondaryStorage.type`, available values are [elasticsearch, opensearch, rdbms]. For more details, see our documentation here: https://docs.camunda.io/docs/next/self-managed/concepts/secondary-storage/configuring-secondary-storage/" -}}
{{- end }}

{{/*
Fail with a message if noSecondaryStorage is enabled but Elasticsearch or OpenSearch are still enabled.
*/}}
{{- if .Values.global.noSecondaryStorage }}
  {{- if or .Values.global.elasticsearch.enabled .Values.global.opensearch.enabled }}
    {{- $errorMessage := printf "[camunda][error] %s %s %s"
        "When \"global.noSecondaryStorage\" is enabled, both Elasticsearch and OpenSearch must be disabled."
        "Please ensure that \"global.elasticsearch.enabled: false\" and \"global.opensearch.enabled: false\""
        "are set when using \"global.noSecondaryStorage: true\"."
    -}}
    {{ printf "\n%s" $errorMessage | trimSuffix "\n"| fail }}
  {{- end }}
{{- end }}

{{/*
Fail with a message if the auth type is not in the enums (KEYCLOAK, MICROSOFT, or GENERIC).
*/}}
{{- if not (has (include "camundaPlatform.authIssuerType" .) (list "KEYCLOAK" "MICROSOFT" "GENERIC")) }}
  {{- $errorMessage := printf "[camunda][error] %s"
      "The Identity auth type should be one of the following values: KEYCLOAK, MICROSOFT, or GENERIC."
  -}}
  {{ printf "\n%s" $errorMessage | trimSuffix "\n"| fail }}
{{- end }}

{{/*
Fail with a message if the auth type is set to non-Keycloak and its requirements are not met.
*/}}
{{- if has (include "camundaPlatform.authIssuerType" .) (list "MICROSOFT" "GENERIC") }}
  {{/*
  TODO: Once refactor the auth issuers, we need to add more constraints here to validate the new auth types. 
        More details: https://github.com/camunda/camunda-platform-helm/issues/4419
  */}}
{{- end }}

{{/*
Fail with a message if global.identity.auth.identity secret configuration is set and global.identity.auth.type is set to KEYCLOAK
*/}}

{{- if eq (include "camundaPlatform.hasSecretConfig" (dict "config" .Values.global.identity.auth.identity)) "true" }}
  {{- if eq (include "camundaPlatform.authIssuerType" .) "KEYCLOAK" }}
    {{- $errorMessage := "[camunda][error] global.identity.auth.identity secret configuration does not need to be set when using Keycloak."
    -}}
    {{ printf "\n%s" $errorMessage | trimSuffix "\n"| fail }}
  {{- end }}
{{- end }}

{{/*
Fail with a message if adaptSecurityContext has any value other than "force" or "disabled".
*/}}
{{- if not (has .Values.global.compatibility.openshift.adaptSecurityContext (list "force" "disabled")) }}
  {{- $errorMessage := "[camunda][error] Invalid value for adaptSecurityContext. The value must be either 'force' or 'disabled'." -}}
  {{ printf "\n%s" $errorMessage | trimSuffix "\n" | fail }}
{{- end }}

{{/*
Fail with a message if Web Modeler is enabled but management Identity is not enabled.
*/}}
{{- if and (eq (include "camundaHub.webModelerEnabled" .) "true") (not $identityEnabled) }}
  {{- $errorMessage := printf "[camunda][error] %s %s"
      "Web Modeler is enabled but management Identity is not configured."
      "Enable local management Identity with \"identity.enabled: true\", or point to an external management Identity via \"global.identity.service.url\"."
  -}}
  {{ printf "\n%s" $errorMessage | trimSuffix "\n"| fail }}
{{- end }}

{{/*
camunda.constraints.warnings
Non-fatal deprecation/config warnings. Consumed by NOTES.txt (helm install/upgrade) and by
configmap-warnings.yaml, which renders the "<release>-warnings" ConfigMap on the GitOps path
(helm template / Argo CD / Flux). Feed new deprecations here so they reach both channels.
*/}}
{{- define "camunda.constraints.warnings" }}
  {{- if .Values.global.testDeprecationFlags.existingSecretsMustBeSet }}
    {{/* TODO: Check if there are more existingSecrets to check */}}

    {{- $existingSecretsNotConfigured := list }}

    {{ if .Values.global.identity.auth.enabled }}
      {{ if and (.Values.connectors.enabled)
                (eq (include "connectors.authMethod" .) "oidc")
                (not .Values.connectors.security.authentication.oidc.secret.existingSecret) }}
        {{- $existingSecretsNotConfigured = append
            $existingSecretsNotConfigured "connectors.security.authentication.oidc.secret.existingSecret" }}
      {{- end }}

      {{ if and (ne (include "camundaPlatform.authIssuerType" .) "KEYCLOAK")
                (.Values.identity.enabled)
                (not .Values.global.identity.auth.identity.secret.existingSecret) }}
        {{- $existingSecretsNotConfigured = append
            $existingSecretsNotConfigured "global.identity.auth.identity.secret.existingSecret" }}
      {{- end }}

      {{ if and (.Values.orchestration.enabled)
                (eq (include "orchestration.authMethod" .) "oidc")
                (not .Values.orchestration.security.authentication.oidc.secret.existingSecret) }}
        {{- $existingSecretsNotConfigured = append
            $existingSecretsNotConfigured "orchestration.security.authentication.oidc.secret.existingSecret" }}
      {{- end }}
    {{- end }}

  {{/* External Keycloak auth secret must be explicitly configured when using external Keycloak */}}
  {{ if and (.Values.identity.enabled)
            (.Values.global.identity.keycloak.auth.adminUser)
            (not .Values.global.identity.keycloak.auth.secret.existingSecret) }}
    {{- $existingSecretsNotConfigured = append
        $existingSecretsNotConfigured "global.identity.keycloak.auth.secret.existingSecret"
    }}
  {{- end }}

  {{- $wmPusher := mustMergeOverwrite (deepCopy .Values.webModeler.restapi.pusher) (.Values.camundaHub.restapi.pusher | default dict) }}
  {{ if and (eq (include "camundaHub.webModelerEnabled" .) "true")
            (not $wmPusher.secret.existingSecret) }}
    {{- $existingSecretsNotConfigured = append
        $existingSecretsNotConfigured "webModeler.restapi.pusher.secret.existingSecret"
    }}
  {{- end }}

  {{ if and (eq (include "camundaHub.webModelerEnabled" .) "true")
            (not $wmPusher.client.secret.existingSecret) }}
    {{- $existingSecretsNotConfigured = append
        $existingSecretsNotConfigured "webModeler.restapi.pusher.client.secret.existingSecret"
    }}
  {{- end }}

    {{- if $existingSecretsNotConfigured }}
      {{- if eq .Values.global.testDeprecationFlags.existingSecretsMustBeSet "warning" }}
        {{- $errorMessage := (printf "%s"
      `
[camunda][warning]
DEPRECATION NOTICE: Starting from appVersion 8.7, the Camunda Helm chart will no longer automatically generate passwords for the Identity component.
Users must provide passwords as Kubernetes secrets. 
In appVersion 8.6, this warning will appear if all necessary existingSecrets are not set.

The following values inside your values.yaml need to be set but were not:
      `
          )
        -}}
        {{- range $existingSecretsNotConfigured }}
          {{- $errorMessage = (cat "  " $errorMessage "\n" .) }}
        {{- end }}
        {{- $errorMessage = (cat $errorMessage "\n\n" "Please set each parameter above to your Kubernetes Secret name, along with the corresponding .secret.existingSecretKey parameter.\n") }}
        {{- printf "\n%s" $errorMessage | trimSuffix "\n" }}
      {{- else if eq .Values.global.testDeprecationFlags.existingSecretsMustBeSet "error" }}
        {{- $errorMessage := (printf "%s"
      `
[camunda][error]
DEPRECATION NOTICE: Starting from appVersion 8.7, the Camunda Helm chart will no longer automatically generate passwords for the Identity component.
Users must provide passwords as Kubernetes secrets. 

The following values inside your values.yaml need to be set but were not:
      `
          )
        -}}
        {{- range $existingSecretsNotConfigured }}
          {{- $errorMessage = (cat "  " $errorMessage "\n" .) }}
        {{- end }}
        {{- $errorMessage = (cat $errorMessage "\n\n" "Please set each parameter above to your Kubernetes Secret name, along with the corresponding .secret.existingSecretKey parameter.\n") }}
        {{ printf "\n%s" $errorMessage | trimSuffix "\n"| fail }}
      {{- end }}
    {{- end }}
  {{- end }}

  {{/* Warn about insecure inlineSecret usage */}}
  {{- $inlineSecretSections := list -}}
  {{- range $k, $v := .Values -}}
    {{- if kindIs "map" $v -}}
      {{- if eq $k "global" -}}
        {{/* Drill into global.* children for finer-grained reporting */}}
        {{- range $gk, $gv := $v -}}
          {{- if and (kindIs "map" $gv) (mustToJson $gv | regexMatch "\"inlineSecret\":\"[^\"]+\"") -}}
            {{- $inlineSecretSections = append $inlineSecretSections (printf "global.%s" $gk) -}}
          {{- end -}}
        {{- end -}}
      {{- else -}}
        {{- if (mustToJson $v | regexMatch "\"inlineSecret\":\"[^\"]+\"") -}}
          {{- $inlineSecretSections = append $inlineSecretSections $k -}}
        {{- end -}}
      {{- end -}}
    {{- end -}}
  {{- end -}}
  {{- if $inlineSecretSections -}}
    {{- $warningMessage := printf "%s %s %s %s"
        "[camunda][warning]"
        (printf "SECURITY: inlineSecret is set in: [%s]." (join ", " $inlineSecretSections))
        "This stores secrets as plain-text in the Helm values and is NOT suitable for production use."
        "For production environments, please use Kubernetes Secrets with 'secret.existingSecret' instead."
    -}}
    {{ printf "\n%s" $warningMessage | trimSuffix "\n" }}
  {{- end }}

  {{/* Legacy per-component JKS truststore deprecation
       (in favour of `global.tls.caBundle.secret.*` PEM bundle, which the
       chart converts to a PKCS12 truststore at pod start via the caBundle
       init container — see helm#3498). */}}
  {{- /* hasSecretConfig (via normalizeSecretConfiguration) checks
         $config.secret.existingSecret / .inlineSecret — so each "config"
         binding is the PARENT of the .secret block, not the .secret leaf
         itself. The pre-existing pair on this list (the two
         `global.<engine>.tls.secret` paths) had the same bug and never
         fired in production; this fix enables them as well. */ -}}
  {{- $deprecatedDatabaseTlsOptions := list
  (dict "path" "global.elasticsearch.tls.secret" "config" .Values.global.elasticsearch.tls)
  (dict "path" "global.opensearch.tls.secret" "config" .Values.global.opensearch.tls)
  (dict "path" "global.elasticsearch.tls.jks.secret" "config" .Values.global.elasticsearch.tls.jks)
  (dict "path" "global.opensearch.tls.jks.secret" "config" .Values.global.opensearch.tls.jks)
  (dict "path" "orchestration.data.secondaryStorage.elasticsearch.tls.secret" "config" .Values.orchestration.data.secondaryStorage.elasticsearch.tls)
  (dict "path" "orchestration.data.secondaryStorage.opensearch.tls.secret" "config" .Values.orchestration.data.secondaryStorage.opensearch.tls)
  (dict "path" "optimize.database.elasticsearch.tls.secret" "config" .Values.optimize.database.elasticsearch.tls)
  (dict "path" "optimize.database.opensearch.tls.secret" "config" .Values.optimize.database.opensearch.tls)
  }}
  {{- /* Direct existingSecret / inlineSecret check rather than going via
         camundaPlatform.hasSecretConfig — that helper requires BOTH
         existingSecret AND existingSecretKey to be truthy (because it
         normalizes for actual secret-ref injection). For a deprecation
         warning we want to fire when the user has opted into the legacy
         path AT ALL, including the natural minimal config of setting only
         existingSecret (existingSecretKey defaults to "" on the
         secondaryStorage / database paths). */ -}}
  {{- range $deprecatedDatabaseTlsOptions }}
    {{- $secret := (.config).secret -}}
    {{- if and $secret (or $secret.existingSecret $secret.inlineSecret) }}
        {{- $warningMessage := printf "%s %s %s %s %s"
            "[camunda][warning]"
            (printf "DEPRECATION: values.yaml is using legacy JKS truststore option '%s'." .path)
            "This option is deprecated as of chart 15.x and will be removed in a future major release."
            "Please migrate to 'global.tls.caBundle.secret.{existingSecret,existingSecretKey}', supplying a PEM-encoded CA bundle."
            "The chart will build the JVM truststore at pod start (no offline keytool needed). Migration: supply a PEM CA bundle to global.tls.caBundle.secret.existingSecret and remove the legacy tls.secret.existingSecret entries plus any -Djavax.net.ssl.trustStore* flags from javaOpts."
        -}}
        {{ printf "\n%s" $warningMessage | trimSuffix "\n" }}
    {{- end }}
  {{- end }}

  {{- $deprecatedDatabaseOptions := list
  (dict "path" "global.elasticsearch.enabled" "config" .Values.global.elasticsearch.enabled)
  (dict "path" "global.opensearch.enabled" "config" .Values.global.opensearch.enabled)
  }}
  {{- range $deprecatedDatabaseOptions }}
    {{- if .config }}
        {{- $warningMessage := printf "%s %s %s %s %s"
            "[camunda][warning]"
            (printf "DEPRECATION: values.yaml is using legacy option '%s'." .path)
            "This option is deprecated and will be removed in a future version."
            (printf "Please migrate to the new option: 'orchestration.data.secondaryStorage.(elasticsearch|opensearch).enabled'.")
            (printf "or for optimize: 'optimize.database.(elasticsearch|opensearch).enabled'.")
        -}}
        {{ printf "\n%s" $warningMessage | trimSuffix "\n" }}
    {{- end }}
  {{- end }}

  {{/* global.tls.caBundle guardrails: surface the three silent failure modes
       a caBundle user can hit (JKS precedence, trust!=encryption, env override). */}}
  {{- if eq (include "camundaPlatform.hasCaBundle" .) "true" }}

    {{/* (1) A per-component JKS truststore silently wins over caBundle for that
           component — the init container still builds a truststore the JVM never uses. */}}
    {{- $jksOverrides := list
        (dict "comp" "orchestration secondaryStorage.elasticsearch" "config" .Values.orchestration.data.secondaryStorage.elasticsearch.tls)
        (dict "comp" "orchestration secondaryStorage.opensearch" "config" .Values.orchestration.data.secondaryStorage.opensearch.tls)
        (dict "comp" "optimize database.elasticsearch" "config" .Values.optimize.database.elasticsearch.tls)
        (dict "comp" "optimize database.opensearch" "config" .Values.optimize.database.opensearch.tls)
        (dict "comp" "global.elasticsearch" "config" .Values.global.elasticsearch.tls)
        (dict "comp" "global.opensearch" "config" .Values.global.opensearch.tls)
    }}
    {{- range $jksOverrides }}
      {{- if eq (include "camundaPlatform.hasSecretConfig" (dict "config" .config)) "true" }}
        {{- $warningMessage := printf "%s %s %s"
            "[camunda][warning]"
            (printf "global.tls.caBundle is set, but %s also configures a per-component JKS truststore (tls.secret)." .comp)
            "The JKS takes precedence for that component, so the caBundle is NOT used there (its init container still builds an unused truststore). Remove the per-component tls.secret to switch to the caBundle, or ignore this if the JKS is intentional."
        -}}
        {{ printf "\n%s" $warningMessage | trimSuffix "\n" }}
      {{- end }}
    {{- end }}

    {{/* (2) caBundle provides CA trust, not encryption. A plaintext datastore URL
           means traffic is still unencrypted despite the bundle being set.
           Orchestration secondaryStorage URLs are full scheme strings;
           Optimize database URLs are split into a separate .protocol field. */}}
    {{- range $url := (list .Values.orchestration.data.secondaryStorage.opensearch.url .Values.orchestration.data.secondaryStorage.elasticsearch.url) }}
      {{- if and $url (hasPrefix "http://" (lower $url)) }}
        {{- $warningMessage := printf "%s %s %s"
            "[camunda][warning]"
            (printf "global.tls.caBundle is set, but the secondary-storage URL '%s' is plaintext http://." $url)
            "caBundle provides CA TRUST, not encryption — it does not enable TLS by itself. Set the URL to https:// to actually encrypt the connection."
        -}}
        {{ printf "\n%s" $warningMessage | trimSuffix "\n" }}
      {{- end }}
    {{- end }}
    {{- range $db := (list "opensearch" "elasticsearch") }}
      {{- $u := index $.Values.optimize.database $db "url" }}
      {{- if and $u $u.protocol (eq (lower $u.protocol) "http") }}
        {{- $warningMessage := printf "%s %s %s"
            "[camunda][warning]"
            (printf "global.tls.caBundle is set, but optimize.database.%s.url.protocol is plaintext 'http'." $db)
            "caBundle provides CA TRUST, not encryption — it does not enable TLS by itself. Set the protocol to https to actually encrypt the connection."
        -}}
        {{ printf "\n%s" $warningMessage | trimSuffix "\n" }}
      {{- end }}
    {{- end }}

    {{/* (3) A component-level JAVA_TOOL_OPTIONS env entry overrides (last-wins) the
           chart's truststore flags, silently breaking JVM trust. */}}
    {{/* webModeler.restapi env uses `or` to mirror deployment-restapi.yaml's own
         env coalescing (camundaHub takes precedence; only that one list is
         applied). We check exactly the list the deployment uses, so we never warn
         about a JAVA_TOOL_OPTIONS in the ignored list — that would be a false
         alarm since it is not applied. */}}
    {{- $envComponents := list
        (dict "comp" "orchestration" "env" .Values.orchestration.env)
        (dict "comp" "optimize" "env" .Values.optimize.env)
        (dict "comp" "connectors" "env" .Values.connectors.env)
        (dict "comp" "identity" "env" .Values.identity.env)
        (dict "comp" "webModeler.restapi" "env" (or .Values.camundaHub.restapi.env .Values.webModeler.restapi.env))
    }}
    {{- range $c := $envComponents }}
      {{- range $e := $c.env }}
        {{- if eq $e.name "JAVA_TOOL_OPTIONS" }}
          {{- $warningMessage := printf "%s %s %s"
              "[camunda][warning]"
              (printf "global.tls.caBundle is set, but %s.env sets JAVA_TOOL_OPTIONS directly." $c.comp)
              "Kubernetes keeps the last duplicate env var, so this overrides the chart's truststore flags and JVM TLS trust will break (PKIX errors). Include the chart's flags in your value: '-Djavax.net.ssl.trustStore=/var/camunda/tls-truststore/cacerts -Djavax.net.ssl.trustStorePassword=changeit'. Components that expose a 'javaOpts' value (orchestration, optimize, web-modeler restapi) can set that instead — the chart appends its truststore flags to it."
          -}}
          {{ printf "\n%s" $warningMessage | trimSuffix "\n" }}
        {{- end }}
      {{- end }}
    {{- end }}

  {{- end }}

  {{/* Warn when webModeler pusher secret is auto-generated */}}
  {{- if eq (include "camundaHub.webModelerEnabled" .) "true" }}
    {{- $pusher := mustMergeOverwrite (deepCopy .Values.webModeler.restapi.pusher) (.Values.camundaHub.restapi.pusher | default dict) }}
    {{- $pusherSecret := $pusher.secret }}
    {{- if not (or $pusherSecret.existingSecret $pusherSecret.inlineSecret) }}
      {{- $warningMessage := printf "%s %s %s %s"
          "[camunda][warning]"
          "Web Modeler is using an auto-generated Pusher secret. This will produce a new random secret on every 'helm upgrade', causing WebSocket authentication failures."
          "Please set 'webModeler.restapi.pusher.secret.existingSecret' (recommended) or 'webModeler.restapi.pusher.secret.inlineSecret'."
          "Auto-generation will be removed in a future release."
      -}}
      {{ printf "\n%s" $warningMessage | trimSuffix "\n" }}
    {{- end }}
    {{- $pusherClientSecret := $pusher.client.secret }}
    {{- if not (or $pusherClientSecret.existingSecret $pusherClientSecret.inlineSecret) }}
      {{- $warningMessage := printf "%s %s %s %s"
          "[camunda][warning]"
          "Web Modeler is using an auto-generated Pusher app key. This will produce a new random key on every 'helm upgrade', causing WebSocket authentication failures."
          "Please set 'webModeler.restapi.pusher.client.secret.existingSecret' (recommended) or 'webModeler.restapi.pusher.client.secret.inlineSecret'."
          "Auto-generation will be removed in a future release."
      -}}
      {{ printf "\n%s" $warningMessage | trimSuffix "\n" }}
    {{- end }}
  {{- end }}

  {{/* Camunda Hub consolidation deprecation warnings */}}
  {{- if .Values.webModeler.enabled }}
    {{- $warningMessage := printf "%s %s %s %s"
        "[camunda][warning]"
        "DEPRECATION: \"webModeler.enabled\" is deprecated and will be removed in a future version."
        "Web Modeler has been consolidated into Camunda Hub. Please use \"camundaHub.enabled: true\" instead."
        "Any web-modeler-specific overrides can be placed under \"camundaHub.*\"."
    -}}
    {{ printf "\n%s" $warningMessage | trimSuffix "\n" }}
  {{- end }}
  {{- $console := default (dict) .Values.console }}
  {{- if $console.enabled }}
    {{- $warningMessage := printf "%s %s %s %s"
        "[camunda][warning]"
        "DEPRECATION: \"console.enabled\" is deprecated and will be removed in a future version."
        "Console has been consolidated into Camunda Hub as an in-Modeler feature. Please use \"camundaHub.enabled: true\" instead."
        "Any console-specific overrides should use the top-level \"console.*\" keys."
    -}}
    {{ printf "\n%s" $warningMessage | trimSuffix "\n" }}
  {{- end }}
  {{- if hasKey .Values.global.identity.auth "console" }}
    {{- $warningMessage := printf "%s %s %s"
        "[camunda][warning]"
        "DEPRECATION: \"global.identity.auth.console.*\" is no longer used in Camunda 8.10."
        "Console has been consolidated into Camunda Hub and this key has no replacement; it can be safely removed from values.yaml."
    -}}
    {{ printf "\n%s" $warningMessage | trimSuffix "\n" }}
  {{- end }}

  {{/*
  *****************************************************************************
  8.10 deprecated app-config-proxy keys (epic #6051): warn (never fail) when set
  to a non-default value; removed in chart v16 (8.11) -> extraConfiguration.
  *****************************************************************************
  */}}
  {{- if .Values.orchestration.enabled }}
    {{- $orchestrationExtra := "orchestration.extraConfiguration" }}
    {{ include "camundaPlatform.keyDeprecated" (dict
      "condition" (ne (.Values.orchestration.logLevel | toString) "info")
      "oldName" "orchestration.logLevel" "migration" $orchestrationExtra) }}
    {{ include "camundaPlatform.keyDeprecated" (dict
      "condition" (.Values.orchestration.security.authentication.unprotectedApi)
      "oldName" "orchestration.security.authentication.unprotectedApi" "migration" $orchestrationExtra) }}
    {{ include "camundaPlatform.keyDeprecated" (dict
      "condition" (ne (.Values.orchestration.security.authentication.oidc.usernameClaim | toString) "preferred_username")
      "oldName" "orchestration.security.authentication.oidc.usernameClaim" "migration" $orchestrationExtra) }}
    {{ include "camundaPlatform.keyDeprecated" (dict
      "condition" (ne (.Values.orchestration.security.authentication.oidc.clientIdClaim | toString) "client_id")
      "oldName" "orchestration.security.authentication.oidc.clientIdClaim" "migration" $orchestrationExtra) }}
    {{ include "camundaPlatform.keyDeprecated" (dict
      "condition" (not (empty .Values.orchestration.security.authentication.oidc.groupsClaim))
      "oldName" "orchestration.security.authentication.oidc.groupsClaim" "migration" $orchestrationExtra) }}
    {{ include "camundaPlatform.keyDeprecated" (dict
      "condition" (not (empty .Values.orchestration.index.prefix))
      "oldName" "orchestration.index.prefix" "migration" $orchestrationExtra) }}
    {{ include "camundaPlatform.keyDeprecated" (dict
      "condition" (ne (.Values.orchestration.index.replicas | toString) "1")
      "oldName" "orchestration.index.replicas" "migration" $orchestrationExtra) }}
    {{ include "camundaPlatform.keyDeprecated" (dict
      "condition" (ne (.Values.orchestration.data.snapshotPeriod | toString) "5m")
      "oldName" "orchestration.data.snapshotPeriod" "migration" $orchestrationExtra) }}
    {{ include "camundaPlatform.keyDeprecated" (dict
      "condition" (.Values.orchestration.history.retention.enabled)
      "oldName" "orchestration.history.retention.enabled" "migration" $orchestrationExtra) }}
    {{ include "camundaPlatform.keyDeprecated" (dict
      "condition" (ne (.Values.orchestration.history.retention.minimumAge | toString) "30d")
      "oldName" "orchestration.history.retention.minimumAge" "migration" $orchestrationExtra) }}
    {{ include "camundaPlatform.keyDeprecated" (dict
      "condition" (ne (.Values.orchestration.history.retention.policyName | toString) "camunda-history-retention-policy")
      "oldName" "orchestration.history.retention.policyName" "migration" $orchestrationExtra) }}
    {{ include "camundaPlatform.keyDeprecated" (dict
      "condition" (.Values.orchestration.retention.enabled)
      "oldName" "orchestration.retention.enabled" "migration" $orchestrationExtra) }}
    {{ include "camundaPlatform.keyDeprecated" (dict
      "condition" (ne (.Values.orchestration.retention.minimumAge | toString) "30d")
      "oldName" "orchestration.retention.minimumAge" "migration" $orchestrationExtra) }}
    {{ include "camundaPlatform.keyDeprecated" (dict
      "condition" (ne (.Values.orchestration.retention.policyName | toString) "zeebe-record-retention-policy")
      "oldName" "orchestration.retention.policyName" "migration" $orchestrationExtra) }}
    {{ include "camundaPlatform.keyDeprecated" (dict
      "condition" (.Values.orchestration.security.authentication.oidc.preferUsernameClaim)
      "oldName" "orchestration.security.authentication.oidc.preferUsernameClaim" "migration" $orchestrationExtra) }}
    {{ include "camundaPlatform.keyDeprecated" (dict
      "condition" (ne (.Values.orchestration.security.authentication.authenticationRefreshInterval | toString) "PT30S")
      "oldName" "orchestration.security.authentication.authenticationRefreshInterval" "migration" $orchestrationExtra) }}
    {{ include "camundaPlatform.keyDeprecated" (dict
      "condition" (not .Values.orchestration.security.authorizations.enabled)
      "oldName" "orchestration.security.authorizations.enabled" "migration" $orchestrationExtra) }}
    {{ include "camundaPlatform.keyDeprecated" (dict
      "condition" (ne (.Values.orchestration.cpuThreadCount | toString) "3")
      "oldName" "orchestration.cpuThreadCount" "migration" $orchestrationExtra) }}
    {{ include "camundaPlatform.keyDeprecated" (dict
      "condition" (ne (.Values.orchestration.ioThreadCount | toString) "3")
      "oldName" "orchestration.ioThreadCount" "migration" $orchestrationExtra) }}
    {{ include "camundaPlatform.keyDeprecated" (dict
      "condition" (ne (.Values.orchestration.partitionCount | toString) "3")
      "oldName" "orchestration.partitionCount" "migration" $orchestrationExtra) }}
    {{ include "camundaPlatform.keyDeprecated" (dict
      "condition" (ne (.Values.orchestration.replicationFactor | toString) "3")
      "oldName" "orchestration.replicationFactor" "migration" $orchestrationExtra) }}
    {{ include "camundaPlatform.keyDeprecated" (dict
      "condition" (ne (.Values.orchestration.history.delayBetweenRuns | toString) "2000")
      "oldName" "orchestration.history.delayBetweenRuns" "migration" $orchestrationExtra) }}
    {{ include "camundaPlatform.keyDeprecated" (dict
      "condition" (ne (.Values.orchestration.history.maxDelayBetweenRuns | toString) "60000")
      "oldName" "orchestration.history.maxDelayBetweenRuns" "migration" $orchestrationExtra) }}
    {{ include "camundaPlatform.keyDeprecated" (dict
      "condition" (ne (.Values.orchestration.history.rolloverBatchSize | toString) "100")
      "oldName" "orchestration.history.rolloverBatchSize" "migration" $orchestrationExtra) }}
    {{ include "camundaPlatform.keyDeprecated" (dict
      "condition" (ne (.Values.orchestration.history.rolloverInterval | toString) "1d")
      "oldName" "orchestration.history.rolloverInterval" "migration" $orchestrationExtra) }}
    {{ include "camundaPlatform.keyDeprecated" (dict
      "condition" (ne (.Values.orchestration.history.waitPeriodBeforeArchiving | toString) "1h")
      "oldName" "orchestration.history.waitPeriodBeforeArchiving" "migration" $orchestrationExtra) }}
    {{ include "camundaPlatform.keyDeprecated" (dict
      "condition" (ne (.Values.orchestration.history.elsRolloverDateFormat | toString) "date")
      "oldName" "orchestration.history.elsRolloverDateFormat" "migration" $orchestrationExtra) }}
    {{ include "camundaPlatform.keyDeprecated" (dict
      "condition" (ne (.Values.orchestration.history.retention.usageMetricsMinimumAge | toString) "730d")
      "oldName" "orchestration.history.retention.usageMetricsMinimumAge" "migration" $orchestrationExtra) }}
    {{ include "camundaPlatform.keyDeprecated" (dict
      "condition" (ne (.Values.orchestration.history.retention.usageMetricsPolicyName | toString) "camunda-usage-metrics-retention-policy")
      "oldName" "orchestration.history.retention.usageMetricsPolicyName" "migration" $orchestrationExtra) }}
    {{ include "camundaPlatform.keyDeprecated" (dict
      "condition" (.Values.orchestration.debug)
      "oldName" "orchestration.debug" "migration" $orchestrationExtra) }}
    {{ include "camundaPlatform.keyDeprecated" (dict
      "condition" (not (empty .Values.orchestration.profilesOverride))
      "oldName" "orchestration.profilesOverride" "migration" $orchestrationExtra) }}
    {{ include "camundaPlatform.keyDeprecated" (dict
      "condition" (not (empty .Values.orchestration.log4j2))
      "oldName" "orchestration.log4j2" "migration" "orchestration.extraConfiguration (file-content kind)") }}
    {{ include "camundaPlatform.keyDeprecated" (dict
      "condition" (ne (.Values.orchestration.data.disk.freeSpace.processing | toString) "2GB")
      "oldName" "orchestration.data.disk.freeSpace.processing" "migration" $orchestrationExtra) }}
    {{ include "camundaPlatform.keyDeprecated" (dict
      "condition" (ne (.Values.orchestration.data.disk.freeSpace.replication | toString) "1GB")
      "oldName" "orchestration.data.disk.freeSpace.replication" "migration" $orchestrationExtra) }}
    {{ include "camundaPlatform.keyDeprecated" (dict
      "condition" (not (empty .Values.orchestration.security.authentication.oidc.scope))
      "oldName" "orchestration.security.authentication.oidc.scope" "migration" $orchestrationExtra) }}
    {{ include "camundaPlatform.keyDeprecated" (dict
      "condition" (not (empty .Values.orchestration.security.authentication.oidc.backwardsCompatibleAudiences))
      "oldName" "orchestration.security.authentication.oidc.backwardsCompatibleAudiences" "migration" $orchestrationExtra) }}
    {{ include "camundaPlatform.keyDeprecated" (dict
      "condition" (not (empty .Values.orchestration.security.initialization.mappingRules))
      "oldName" "orchestration.security.initialization.mappingRules" "migration" $orchestrationExtra) }}
    {{ include "camundaPlatform.keyDeprecated" (dict
      "condition" (not (empty .Values.orchestration.security.initialization.authorizations))
      "oldName" "orchestration.security.initialization.authorizations" "migration" $orchestrationExtra) }}
    {{ include "camundaPlatform.keyDeprecated" (dict
      "condition" (not .Values.orchestration.exporters.camunda.enabled)
      "oldName" "orchestration.exporters.camunda.enabled" "migration" $orchestrationExtra) }}
    {{ include "camundaPlatform.keyDeprecated" (dict
      "condition" (not (empty .Values.orchestration.exporters.zeebe.replicas))
      "oldName" "orchestration.exporters.zeebe.replicas" "migration" $orchestrationExtra) }}
    {{ include "camundaPlatform.keyDeprecated" (dict
      "condition" (ne (.Values.orchestration.security.authentication.oidc.redirectUrl | toString) "http://localhost:8080")
      "oldName" "orchestration.security.authentication.oidc.redirectUrl" "migration" $orchestrationExtra) }}
    {{ include "camundaPlatform.keyDeprecated" (dict
      "condition" (.Values.orchestration.upgrade.allowPreReleaseImages)
      "oldName" "orchestration.upgrade.allowPreReleaseImages" "migration" $orchestrationExtra) }}
    {{ include "camundaPlatform.keyDeprecated" (dict
      "condition" (.Values.orchestration.multitenancy.checks.enabled)
      "oldName" "orchestration.multitenancy.checks.enabled" "migration" $orchestrationExtra) }}
    {{ include "camundaPlatform.keyDeprecated" (dict
      "condition" (not .Values.orchestration.multitenancy.api.enabled)
      "oldName" "orchestration.multitenancy.api.enabled" "migration" $orchestrationExtra) }}
  {{- end }}

  {{- if .Values.connectors.enabled }}
    {{ include "camundaPlatform.keyDeprecated" (dict
      "condition" (ne (index .Values.connectors.logging.level "io.camunda.connector" | toString) "INFO")
      "oldName" "connectors.logging.level.io.camunda.connector" "migration" "connectors.extraConfiguration") }}
  {{- end }}

  {{- if .Values.optimize.enabled }}
    {{- $optimizeExtra := "optimize.extraConfiguration" }}
    {{ include "camundaPlatform.keyDeprecated" (dict
      "condition" (ne (.Values.optimize.logLevel | toString) "info")
      "oldName" "optimize.logLevel" "migration" $optimizeExtra) }}
    {{ include "camundaPlatform.keyDeprecated" (dict
      "condition" (ne (.Values.optimize.upgradeLogLevel | toString) "info")
      "oldName" "optimize.upgradeLogLevel" "migration" $optimizeExtra) }}
    {{ include "camundaPlatform.keyDeprecated" (dict
      "condition" (ne (.Values.optimize.esLogLevel | toString) "warn")
      "oldName" "optimize.esLogLevel" "migration" $optimizeExtra) }}
    {{ include "camundaPlatform.keyDeprecated" (dict
      "condition" (ne (.Values.optimize.profiles | toString) "ccsm")
      "oldName" "optimize.profiles" "migration" $optimizeExtra) }}
    {{ include "camundaPlatform.keyDeprecated" (dict
      "condition" (ne (.Values.optimize.caches.cloudTenantAuthorizations.maxSize | toString) "10000")
      "oldName" "optimize.caches.cloudTenantAuthorizations.maxSize" "migration" $optimizeExtra) }}
    {{ include "camundaPlatform.keyDeprecated" (dict
      "condition" (ne (.Values.optimize.caches.cloudTenantAuthorizations.minFetchIntervalSeconds | toString) "600000")
      "oldName" "optimize.caches.cloudTenantAuthorizations.minFetchIntervalSeconds" "migration" $optimizeExtra) }}
    {{ include "camundaPlatform.keyDeprecated" (dict
      "condition" (ne (.Values.optimize.partitionCount | toString) "3")
      "oldName" "optimize.partitionCount" "migration" $optimizeExtra) }}
    {{ include "camundaPlatform.keyDeprecated" (dict
      "condition" (ne (.Values.optimize.database.elasticsearch.prefix | toString) "zeebe-record")
      "oldName" "optimize.database.elasticsearch.prefix" "migration" $optimizeExtra) }}
    {{ include "camundaPlatform.keyDeprecated" (dict
      "condition" (ne (.Values.optimize.database.opensearch.prefix | toString) "zeebe-record")
      "oldName" "optimize.database.opensearch.prefix" "migration" $optimizeExtra) }}
    {{ include "camundaPlatform.keyDeprecated" (dict
      "condition" (.Values.optimize.multitenancy.enabled)
      "oldName" "optimize.multitenancy.enabled" "migration" "global.multitenancy.enabled") }}
  {{- end }}

  {{- if eq (include "camundaHub.webModelerEnabled" .) "true" }}
    {{- $wm := deepCopy .Values.webModeler }}
    {{- $wmExtra := "webModeler.restapi.extraConfiguration" }}
    {{ include "camundaPlatform.keyDeprecated" (dict
      "condition" (not (empty $wm.restapi.mail.fromAddress))
      "oldName" "webModeler.restapi.mail.fromAddress" "migration" $wmExtra) }}
    {{ include "camundaPlatform.keyDeprecated" (dict
      "condition" (ne ($wm.restapi.mail.fromName | toString) "Camunda 8")
      "oldName" "webModeler.restapi.mail.fromName" "migration" $wmExtra) }}
    {{ include "camundaPlatform.keyDeprecated" (dict
      "condition" (not (empty $wm.restapi.mail.smtpHost))
      "oldName" "webModeler.restapi.mail.smtpHost" "migration" $wmExtra) }}
    {{ include "camundaPlatform.keyDeprecated" (dict
      "condition" (not (empty $wm.restapi.mail.smtpUser))
      "oldName" "webModeler.restapi.mail.smtpUser" "migration" $wmExtra) }}
    {{ include "camundaPlatform.keyDeprecated" (dict
      "condition" (not $wm.restapi.mail.smtpTlsEnabled)
      "oldName" "webModeler.restapi.mail.smtpTlsEnabled" "migration" $wmExtra) }}
    {{ include "camundaPlatform.keyDeprecated" (dict
      "condition" (ne ($wm.restapi.mail.smtpPort | toString) "587")
      "oldName" "webModeler.restapi.mail.smtpPort" "migration" $wmExtra) }}
    {{ include "camundaPlatform.keyDeprecated" (dict
      "condition" (ne (index $wm.restapi.logging.level "io.camunda.modeler" | toString) "INFO")
      "oldName" "webModeler.restapi.logging.level.io.camunda.modeler" "migration" $wmExtra) }}
    {{ include "camundaPlatform.keyDeprecated" (dict
      "condition" (ne (index $wm.restapi.logging.level "io.grpc" | toString) "INFO")
      "oldName" "webModeler.restapi.logging.level.io.grpc" "migration" $wmExtra) }}
  {{- end }}

  {{- $componentExtra := "the consuming component's extraConfiguration" }}
  {{ include "camundaPlatform.keyDeprecated" (dict
    "condition" (ne (.Values.global.config.requestBodySize | toString) "10MB")
    "oldName" "global.config.requestBodySize" "migration" $componentExtra) }}
  {{ include "camundaPlatform.keyDeprecated" (dict
    "condition" (ne (.Values.global.zeebeClusterName | toString) (printf "%s .Release.Name %s-zeebe" "{{" "}}"))
    "oldName" "global.zeebeClusterName" "migration" $componentExtra) }}
  {{ include "camundaPlatform.keyDeprecated" (dict
    "condition" (ne (.Values.global.documentStore.type.aws.storeId | toString) "AWS")
    "oldName" "global.documentStore.type.aws.storeId" "migration" $componentExtra) }}
  {{ include "camundaPlatform.keyDeprecated" (dict
    "condition" (ne (.Values.global.documentStore.type.gcp.storeId | toString) "GCP")
    "oldName" "global.documentStore.type.gcp.storeId" "migration" $componentExtra) }}
  {{ include "camundaPlatform.keyDeprecated" (dict
    "condition" (ne (.Values.global.documentStore.type.inmemory.storeId | toString) "INMEMORY")
    "oldName" "global.documentStore.type.inmemory.storeId" "migration" $componentExtra) }}
{{- end }}

{{/*
**************************************************************
Deprecation helpers.
**************************************************************
*/}}

{{/*
camundaPlatform.keyRenamed
Fail with message when the old values file key is used and show the new key.
Usage:
{{ include "camundaPlatform.keyRenamed" (dict
  "condition" (.Values.identity.keycloak)
  "oldName" "identity.keycloak"
  "newName" "identityKeycloak"
) }}
*/}}
{{- define "camundaPlatform.keyRenamed" }}
  {{- if .condition }}
    {{- $errorMessage := printf
        "[camunda][error] The Helm values file key changed from \"%s\" to \"%s\". %s %s"
        .oldName .newName
        "For more details, please check Camunda Helm chart documentation."
        "https://docs.camunda.io/docs/self-managed/deployment/helm/upgrade/"
    -}}
    {{ printf "\n%s" $errorMessage | trimSuffix "\n"| fail }}
  {{- end }}
{{- end -}}


{{/*
camundaPlatform.keyRemoved
Fail with message when the old values file key is used.
Usage:
{{ include "camundaPlatform.keyRemoved" (dict
  "condition" (.Values.identity.keycloak)
  "oldName" "identity.keycloak"
) }}
*/}}
{{- define "camundaPlatform.keyRemoved" }}
  {{- if .condition }}
    {{- $errorMessage := printf
        "[camunda][error] The Helm values file key \"%s\" has been removed. %s %s"
        .oldName
        "For more details, please check Camunda Helm chart documentation."
        "https://docs.camunda.io/docs/self-managed/deployment/helm/upgrade/"
    -}}
    {{ printf "\n%s" $errorMessage | trimSuffix "\n"| fail }}
  {{- end }}
{{- end -}}


{{/*
camundaPlatform.keyDeprecated
Emit a non-fatal DEPRECATION warning when a deprecated values file key is set.
Unlike camundaPlatform.keyRemoved/keyRenamed this does NOT fail; it returns a
warning string, so it must be called from within "camunda.constraints.warnings",
which surfaces on install/upgrade (NOTES.txt) and on the GitOps render path via the
"<release>-warnings" ConfigMap (templates/common/configmap-warnings.yaml). Per the Breaking Changes
& Deprecation Policy (docs/policies/breaking-changes.md): warn when the key is
set, name the replacement and the removal version.
Usage:
{{ include "camundaPlatform.keyDeprecated" (dict
  "condition" (ne (toString .Values.orchestration.logLevel) "info")
  "oldName" "orchestration.logLevel"
  "migration" "orchestration.extraConfiguration"
) }}
*/}}
{{- define "camundaPlatform.keyDeprecated" }}
  {{- if .condition }}
    {{- $warningMessage := printf
        "[camunda][warning] DEPRECATION: The Helm values file key \"%s\" is deprecated and will be removed in chart v16 (Camunda 8.11). %s %s"
        .oldName
        (printf "Configure this via \"%s\" instead." (.migration | default "the component's extraConfiguration"))
        (.url | default "https://docs.camunda.io/docs/self-managed/deployment/helm/upgrade/")
    -}}
    {{ printf "\n%s" $warningMessage | trimSuffix "\n" }}
  {{- end }}
{{- end -}}


{{/*
*******************************************************************************
Gateway namespace and createGatewayResource are mutually exclusive.
*******************************************************************************
*/}}
{{- if and .Values.global.gateway.enabled .Values.global.gateway.namespace .Values.global.gateway.createGatewayResource }}
  {{- $errorMessage := printf "[camunda][error] %s %s"
      "global.gateway.namespace and global.gateway.createGatewayResource=true cannot be set together."
      "When using a shared Gateway in another namespace, set \"global.gateway.createGatewayResource: false\"."
  -}}
  {{ printf "\n%s" $errorMessage | trimSuffix "\n"| fail }}
{{- end }}

{{/*
*******************************************************************************
Ingress and Gateway API should not be enabled at the same time.
*******************************************************************************
*/}}
{{- if and .Values.global.gateway.enabled .Values.global.ingress.enabled }}
  {{- $errorMessage := printf "[camunda][error] %s %s"
      "Gateway API and Ingress cannot both be enabled at the same time."
      "Please ensure that either \"global.gateway.enabled: true\" or \"global.ingress.enabled: true\" is set, but not both."
  -}}
  {{ printf "\n%s" $errorMessage | trimSuffix "\n"| fail }}
{{- end }}

{{/*
*******************************************************************************
Camunda 8.8 cycle deprecated keys (removed in 8.9).
*******************************************************************************
Fail with a message when old values syntax is used.
Chart Version: 14.0.0
*******************************************************************************
*/}}

{{/*
*******************************************************************************
Global - License
*******************************************************************************
*/}}

{{ include "camundaPlatform.keyRemoved" (dict
  "condition" (hasKey .Values.global.license "key")
  "oldName" "global.license.key"
) }}

{{ include "camundaPlatform.keyRemoved" (dict
  "condition" (hasKey .Values.global.license "existingSecret")
  "oldName" "global.license.existingSecret"
) }}

{{ include "camundaPlatform.keyRemoved" (dict
  "condition" (hasKey .Values.global.license "existingSecretKey")
  "oldName" "global.license.existingSecretKey"
) }}

{{/*
*******************************************************************************
Global - Elasticsearch Auth
*******************************************************************************
*/}}

{{ include "camundaPlatform.keyRemoved" (dict
  "condition" (hasKey .Values.global.elasticsearch.auth "password")
  "oldName" "global.elasticsearch.auth.password"
) }}

{{ include "camundaPlatform.keyRemoved" (dict
  "condition" (hasKey .Values.global.elasticsearch.auth "existingSecret")
  "oldName" "global.elasticsearch.auth.existingSecret"
) }}

{{ include "camundaPlatform.keyRemoved" (dict
  "condition" (hasKey .Values.global.elasticsearch.auth "existingSecretKey")
  "oldName" "global.elasticsearch.auth.existingSecretKey"
) }}

{{/*
*******************************************************************************
Global - OpenSearch Auth
*******************************************************************************
*/}}

{{ include "camundaPlatform.keyRemoved" (dict
  "condition" (hasKey .Values.global.opensearch.auth "password")
  "oldName" "global.opensearch.auth.password"
) }}

{{ include "camundaPlatform.keyRemoved" (dict
  "condition" (hasKey .Values.global.opensearch.auth "existingSecret")
  "oldName" "global.opensearch.auth.existingSecret"
) }}

{{ include "camundaPlatform.keyRemoved" (dict
  "condition" (hasKey .Values.global.opensearch.auth "existingSecretKey")
  "oldName" "global.opensearch.auth.existingSecretKey"
) }}

{{/*
*******************************************************************************
Global - Identity Auth
*******************************************************************************
*/}}

{{ include "camundaPlatform.keyRemoved" (dict
  "condition" (hasKey .Values.global.identity.auth.admin "existingSecret")
  "oldName" "global.identity.auth.admin.existingSecret"
) }}

{{ include "camundaPlatform.keyRemoved" (dict
  "condition" (hasKey .Values.global.identity.auth.admin "existingSecretKey")
  "oldName" "global.identity.auth.admin.existingSecretKey"
) }}

{{ include "camundaPlatform.keyRemoved" (dict
  "condition" (hasKey .Values.global.identity.auth.identity "existingSecret")
  "oldName" "global.identity.auth.identity.existingSecret"
) }}

{{ include "camundaPlatform.keyRemoved" (dict
  "condition" (hasKey .Values.global.identity.auth.identity "existingSecretKey")
  "oldName" "global.identity.auth.identity.existingSecretKey"
) }}

{{ include "camundaPlatform.keyRemoved" (dict
  "condition" (hasKey .Values.global.identity.auth.optimize "existingSecret")
  "oldName" "global.identity.auth.optimize.existingSecret"
) }}

{{ include "camundaPlatform.keyRemoved" (dict
  "condition" (hasKey .Values.global.identity.auth.optimize "existingSecretKey")
  "oldName" "global.identity.auth.optimize.existingSecretKey"
) }}

{{/*
*******************************************************************************
Global - Document Store
*******************************************************************************
*/}}

{{ include "camundaPlatform.keyRemoved" (dict
  "condition" (hasKey .Values.global.documentStore.type.aws "existingSecret")
  "oldName" "global.documentStore.type.aws.existingSecret"
) }}

{{ include "camundaPlatform.keyRemoved" (dict
  "condition" (hasKey .Values.global.documentStore.type.aws "accessKeyIdKey")
  "oldName" "global.documentStore.type.aws.accessKeyIdKey"
) }}

{{ include "camundaPlatform.keyRemoved" (dict
  "condition" (hasKey .Values.global.documentStore.type.aws "secretAccessKeyKey")
  "oldName" "global.documentStore.type.aws.secretAccessKeyKey"
) }}

{{ include "camundaPlatform.keyRemoved" (dict
  "condition" (hasKey .Values.global.documentStore.type.gcp "existingSecret")
  "oldName" "global.documentStore.type.gcp.existingSecret"
) }}

{{ include "camundaPlatform.keyRemoved" (dict
  "condition" (hasKey .Values.global.documentStore.type.gcp "credentialsKey")
  "oldName" "global.documentStore.type.gcp.credentialsKey"
) }}

{{/*
*******************************************************************************
Identity
*******************************************************************************
*/}}

{{- if .Values.identity.enabled -}}

{{ include "camundaPlatform.keyRemoved" (dict
  "condition" (hasKey .Values.identity.firstUser "password")
  "oldName" "identity.firstUser.password"
) }}

{{ include "camundaPlatform.keyRemoved" (dict
  "condition" (hasKey .Values.identity.firstUser "existingSecret")
  "oldName" "identity.firstUser.existingSecret"
) }}

{{ include "camundaPlatform.keyRemoved" (dict
  "condition" (hasKey .Values.identity.firstUser "existingSecretKey")
  "oldName" "identity.firstUser.existingSecretKey"
) }}

{{ include "camundaPlatform.keyRemoved" (dict
  "condition" (hasKey .Values.identity.externalDatabase "password")
  "oldName" "identity.externalDatabase.password"
) }}

{{ include "camundaPlatform.keyRemoved" (dict
  "condition" (hasKey .Values.identity.externalDatabase "existingSecret")
  "oldName" "identity.externalDatabase.existingSecret"
) }}

{{ include "camundaPlatform.keyRemoved" (dict
  "condition" (hasKey .Values.identity.externalDatabase "existingSecretPasswordKey")
  "oldName" "identity.externalDatabase.existingSecretPasswordKey"
) }}

{{- end }}

{{/*
*******************************************************************************
Connectors
*******************************************************************************
*/}}

{{- if .Values.connectors.enabled -}}

{{ include "camundaPlatform.keyRemoved" (dict
  "condition" (hasKey .Values.connectors.security.authentication.oidc "existingSecret")
  "oldName" "connectors.security.authentication.oidc.existingSecret"
) }}

{{ include "camundaPlatform.keyRemoved" (dict
  "condition" (hasKey .Values.connectors.security.authentication.oidc "existingSecretKey")
  "oldName" "connectors.security.authentication.oidc.existingSecretKey"
) }}

{{- end }}

{{/*
*******************************************************************************
Orchestration
*******************************************************************************
*/}}

{{- if .Values.orchestration.enabled -}}

{{ include "camundaPlatform.keyRemoved" (dict
  "condition" (hasKey .Values.orchestration.security.authentication.oidc "existingSecret")
  "oldName" "orchestration.security.authentication.oidc.existingSecret"
) }}

{{ include "camundaPlatform.keyRemoved" (dict
  "condition" (hasKey .Values.orchestration.security.authentication.oidc "existingSecretKey")
  "oldName" "orchestration.security.authentication.oidc.existingSecretKey"
) }}

{{- end }}

{{/*
*******************************************************************************
Web Modeler
*******************************************************************************
*/}}

{{- if eq (include "camundaHub.webModelerEnabled" .) "true" -}}

{{ include "camundaPlatform.keyRemoved" (dict
  "condition" (hasKey .Values.webModeler.restapi.externalDatabase "password")
  "oldName" "webModeler.restapi.externalDatabase.password"
) }}

{{ include "camundaPlatform.keyRemoved" (dict
  "condition" (hasKey .Values.webModeler.restapi.externalDatabase "existingSecret")
  "oldName" "webModeler.restapi.externalDatabase.existingSecret"
) }}

{{ include "camundaPlatform.keyRemoved" (dict
  "condition" (hasKey .Values.webModeler.restapi.externalDatabase "existingSecretPasswordKey")
  "oldName" "webModeler.restapi.externalDatabase.existingSecretPasswordKey"
) }}

{{ include "camundaPlatform.keyRemoved" (dict
  "condition" (hasKey .Values.webModeler.restapi.mail "smtpPassword")
  "oldName" "webModeler.restapi.mail.smtpPassword"
) }}

{{ include "camundaPlatform.keyRemoved" (dict
  "condition" (hasKey .Values.webModeler.restapi.mail "existingSecret")
  "oldName" "webModeler.restapi.mail.existingSecret"
) }}

{{ include "camundaPlatform.keyRemoved" (dict
  "condition" (hasKey .Values.webModeler.restapi.mail "existingSecretPasswordKey")
  "oldName" "webModeler.restapi.mail.existingSecretPasswordKey"
) }}

{{- end }}

{{/*
*******************************************************************************
Camunda 8.9 cycle deprecated keys.
*******************************************************************************
Fail with a message when old values syntax is used.
Chart Version: 14.0.0
*******************************************************************************
*/}}

{{/*
*******************************************************************************
Global
*******************************************************************************
*/}}

{{/*
- removed: global.secrets.autoGenerated
- removed: global.secrets.name
- removed: global.secrets.annotations
*/}}
{{ include "camundaPlatform.keyRemoved" (dict
  "condition" (and .Values.global.secrets .Values.global.secrets.autoGenerated)
  "oldName" "global.secrets.autoGenerated"
) }}

{{ include "camundaPlatform.keyRemoved" (dict
  "condition" (and .Values.global.secrets (hasKey .Values.global.secrets "name"))
  "oldName" "global.secrets.name"
) }}

{{ include "camundaPlatform.keyRemoved" (dict
  "condition" (and .Values.global.secrets (hasKey .Values.global.secrets "annotations"))
  "oldName" "global.secrets.annotations"
) }}

{{/*
*******************************************************************************
Camunda 8.10 cycle deprecated keys.
*******************************************************************************
Keys deprecated during the 8.9 cycle, removed in the 8.10 chart.
Chart Version: 15.0.0
*******************************************************************************
*/}}

{{/*
*******************************************************************************
Global - Ingress
*******************************************************************************
*/}}

{{ include "camundaPlatform.keyRemoved" (dict
  "condition" (hasKey .Values.global.ingress "host")
  "oldName" "global.ingress.host"
) }}

{{/*
*******************************************************************************
Global - Identity Keycloak Auth
*******************************************************************************
*/}}

{{ include "camundaPlatform.keyRemoved" (dict
  "condition" (hasKey .Values.global.identity.keycloak.auth "existingSecret")
  "oldName" "global.identity.keycloak.auth.existingSecret"
) }}

{{ include "camundaPlatform.keyRemoved" (dict
  "condition" (hasKey .Values.global.identity.keycloak.auth "existingSecretKey")
  "oldName" "global.identity.keycloak.auth.existingSecretKey"
) }}

{{/*
*******************************************************************************
Global - Elasticsearch TLS
*******************************************************************************
*/}}

{{ include "camundaPlatform.keyRemoved" (dict
  "condition" (hasKey .Values.global.elasticsearch.tls "existingSecret")
  "oldName" "global.elasticsearch.tls.existingSecret"
) }}

{{/*
*******************************************************************************
Global - OpenSearch TLS
*******************************************************************************
*/}}

{{ include "camundaPlatform.keyRemoved" (dict
  "condition" (hasKey .Values.global.opensearch.tls "existingSecret")
  "oldName" "global.opensearch.tls.existingSecret"
) }}

{{/*
*******************************************************************************
Orchestration
*******************************************************************************
*/}}

{{- if .Values.orchestration.enabled -}}

{{ include "camundaPlatform.keyRemoved" (dict
  "condition" (hasKey .Values.orchestration.profiles "identity")
  "oldName" "orchestration.profiles.identity"
) }}

{{- end }}

{{/*
*******************************************************************************
Web Modeler
*******************************************************************************
*/}}

{{- if eq (include "camundaHub.webModelerEnabled" .) "true" -}}

{{ include "camundaPlatform.keyRemoved" (dict
  "condition" (hasKey .Values.webModeler.restapi.externalDatabase "user")
  "oldName" "webModeler.restapi.externalDatabase.user"
) }}

{{- end }}

{{/*
*******************************************************************************
Bundled Bitnami subcharts (removed in 8.10)
*******************************************************************************
*/}}

{{ include "camundaPlatform.keyRemoved" (dict
  "condition" (hasKey .Values "identityKeycloak")
  "oldName" "identityKeycloak"
) }}

{{ include "camundaPlatform.keyRemoved" (dict
  "condition" (hasKey .Values "identityPostgresql")
  "oldName" "identityPostgresql"
) }}

{{ include "camundaPlatform.keyRemoved" (dict
  "condition" (hasKey .Values "webModelerPostgresql")
  "oldName" "webModelerPostgresql"
) }}

{{ include "camundaPlatform.keyRemoved" (dict
  "condition" (hasKey .Values "elasticsearch")
  "oldName" "elasticsearch"
) }}
