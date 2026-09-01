{{/* vim: set filetype=mustache: */}}

{{/*
[orchestration] Create a default fully qualified app name.
*/}}
{{- define "orchestration.fullname" -}}
    {{- /* NOTE: The value is set to "zeebe" for backward compatibility between 8.7 and 8.8. */ -}}
    {{- include "camundaPlatform.componentFullname" (dict
        "componentName" "zeebe"
        "componentValues" .Values.orchestration
        "context" $
    ) -}}
{{- end -}}

{{/*
[orchestration] Defines extra labels for orchestration.
*/}}

{{ define "orchestration.componentName" -}}
orchestration
{{- end }}

{{ define "orchestration.brokerName" -}}
{{- /*
    NOTE: The value is set to "zeebe-broker" for backward compatibility between 8.7 and 8.8,
*/ -}}
zeebe-broker
{{- end }}

{{ define "orchestration.gatewayName" -}}
{{- /*
    NOTE: The value is set to "zeebe-gateway" for backward compatibility between 8.7 and 8.8
*/ -}}
zeebe-gateway
{{- end }}

{{- /*
    NOTE: The gateway and broker labels are for backward compatibility between 8.7 and 8.8.
*/ -}}
{{ define "orchestration.gatewayLabel" -}}
app.kubernetes.io/component: {{ include "orchestration.gatewayName" . }}
{{- end }}

{{ define "orchestration.brokerLabel" -}}
app.kubernetes.io/component: {{ include "orchestration.brokerName" . }}
{{- end }}

{{ define "orchestration.versionLabel" -}}
app.kubernetes.io/version: {{ include "camundaPlatform.versionLabel" (dict
    "base" .Values.global
    "overlay" .Values.orchestration
    "chart" .Chart
) | quote }}
{{- end }}

{{ define "orchestration.extraLabelsGatewayService" -}}
    {{- include "orchestration.gatewayLabel" . }}
    {{- "\n" }}
    {{- include "orchestration.versionLabel" . }}
{{- end }}

{{ define "orchestration.extraLabelsBrokerServiceHeadless" -}}
    {{- include "orchestration.brokerLabel" . }}
    {{- "\n" }}
    {{- include "orchestration.versionLabel" . }}
{{- end }}

{{/*
[orchestration] Defines extra labels for orchestration.
*/}}
{{ define "orchestration.extraLabelsMigration" -}}
app.kubernetes.io/component: {{ printf "%s-migration" (include "orchestration.componentName" .) }}
app.kubernetes.io/version: {{ include "camundaPlatform.versionLabel" (dict
    "base" .Values.global
    "overlay" .Values.orchestration
    "chart" .Chart
) | quote }}
{{- end }}


{{/*
[orchestration] Define common labels for orchestration, combining the match labels and transient labels, which might change on updating
(version depending). These labels shouldn't be used on matchLabels selector, since the selectors are immutable.
*/}}
{{- define "orchestration.labels" -}}
    {{- include "camundaPlatform.labels" . }}
    {{- "\n" }}
    {{- include "orchestration.brokerLabel" . }}
    {{- "\n" }}
    {{- include "orchestration.versionLabel" . }}
{{- end -}}

{{/*
[orchestration] Define common labels for orchestration cluster migrations, combining the match labels and transient labels, which might change on updating
(version depending). These labels shouldn't be used on matchLabels selector, since the selectors are immutable.
*/}}
{{- define "orchestration.labelsMigration" -}}
    {{- include "camundaPlatform.labels" . }}
    {{- "\n" }}
    {{- include "orchestration.extraLabelsMigration" . }}
{{- end -}}

{{/*
[orchestration] Defines match labels for orchestration, which are extended by sub-charts and should be used in matchLabels selectors.
*/}}
{{- define "orchestration.matchLabels" -}}
    {{- include "camundaPlatform.matchLabels" . }}
    {{- "\n" -}}
    {{/*    For backward compatibility, the component label is set to "zeebe-broker".*/}}
    {{- include "orchestration.brokerLabel" . }}
{{- end -}}

{{/*
[orchestration] Define variables related to multitenancy checks
*/}}
{{- define "orchestration.multitenancyChecksEnabled" -}}
  {{- if .Values.orchestration.multitenancy.checks.enabled -}}
    {{ .Values.orchestration.multitenancy.checks.enabled }}
  {{- else if .Values.global.multitenancy.enabled -}}
    {{ .Values.global.multitenancy.enabled }}
  {{- else -}}
    false
  {{- end -}}
{{- end -}}

{{/*
[orchestration] Define variables related to multitenancy api
*/}}
{{- define "orchestration.multitenancyApiEnabled" -}}
  {{- if .Values.orchestration.multitenancy.api.enabled -}}
    {{ .Values.orchestration.multitenancy.api.enabled }}
  {{- else if .Values.global.multitenancy.enabled -}}
    {{ .Values.global.multitenancy.enabled }}
  {{- else -}}
    false
  {{- end -}}
{{- end -}}

{{/*
[orchestration] Create the name of the service account to use.
*/}}
{{- define "orchestration.serviceAccountName" -}}
    {{- if .Values.orchestration.serviceAccount.enabled -}}
        {{- default (include "orchestration.fullname" .) .Values.orchestration.serviceAccount.name -}}
    {{- else -}}
        {{- default "default" .Values.orchestration.serviceAccount.name -}}
    {{- end -}}
{{- end -}}


{{/*
********************************************************************************
Authentication.
********************************************************************************
*/}}

{{/*
[orchestration] Define variables related to authentication.
*/}}

{{- define "orchestration.authMethod" -}}
    {{- if ne (include "camundaPlatform.orchestrationEnabled" .) "true" -}}
        none
    {{- else -}}
        {{- .Values.orchestration.security.authentication.method | default (
            .Values.global.security.authentication.method | default "none"
        ) -}}
    {{- end -}}
{{- end -}}

{{- define "orchestration.authEnabled" -}}
    {{- if has (include "orchestration.authMethod" .) (list "oidc" "basic") -}}
        true
    {{- else -}}
        false
    {{- end -}}
{{- end -}}

{{- define "orchestration.authIssuerType" -}}
    {{- .Values.orchestration.security.authentication.oidc.type | default (
        include "camundaPlatform.authIssuerType" .
    ) -}}
{{- end -}}

{{- define "orchestration.authIssuerUrl" -}}
  {{- if .Values.orchestration.security.authentication.oidc.issuer -}}
    {{- .Values.orchestration.security.authentication.oidc.issuer -}}
  {{- else -}}
    {{- include "camundaPlatform.authIssuerUrl" . -}}
  {{- end -}}
{{- end -}}

{{- define "orchestration.authIssuerUrlEndpointAuth" -}}
  {{- if .Values.orchestration.security.authentication.oidc.authUrl -}}
    {{- tpl .Values.orchestration.security.authentication.oidc.authUrl . -}}
  {{- else -}}
    {{- include "camundaPlatform.authIssuerUrlEndpointAuth" . -}}
  {{- end -}}
{{- end -}}

{{- define "orchestration.authIssuerBackendUrlEndpointCerts" -}}
  {{- if .Values.orchestration.security.authentication.oidc.jwksUrl -}}
    {{- tpl .Values.orchestration.security.authentication.oidc.jwksUrl . -}}
  {{- else -}}
    {{- include "camundaPlatform.authIssuerBackendUrlEndpointCerts" . -}}
  {{- end -}}
{{- end -}}

{{- define "orchestration.authIssuerBackendUrlEndpointToken" -}}
  {{- if .Values.orchestration.security.authentication.oidc.tokenUrl -}}
    {{- tpl .Values.orchestration.security.authentication.oidc.tokenUrl . -}}
  {{- else -}}
    {{- include "camundaPlatform.authIssuerBackendUrlEndpointToken" . -}}
  {{- end -}}
{{- end -}}

{{- define "orchestration.authClientId" -}}
    {{- .Values.orchestration.security.authentication.oidc.clientId | default "orchestration" -}}
{{- end -}}

{{- define "orchestration.hubPingClientId" -}}
    {{- .Values.orchestration.hub.ping.credentials.clientId | default (include "orchestration.authClientId" .) -}}
{{- end -}}

{{- define "orchestration.hubPingTokenEndpoint" -}}
    {{- if .Values.orchestration.hub.ping.credentials.tokenEndpoint -}}
        {{- tpl .Values.orchestration.hub.ping.credentials.tokenEndpoint . -}}
    {{- else if .Values.orchestration.security.authentication.oidc.tokenUrl -}}
        {{- tpl .Values.orchestration.security.authentication.oidc.tokenUrl . -}}
    {{- else if .Values.global.identity.auth.tokenUrl -}}
        {{- tpl .Values.global.identity.auth.tokenUrl . -}}
    {{- else if eq (include "orchestration.authIssuerType" .) "KEYCLOAK" -}}
        {{- include "orchestration.authIssuerBackendUrlEndpointToken" . -}}
    {{- end -}}
{{- end -}}

{{- define "orchestration.hubPingClientSecretConfig" -}}
    {{- $cs := .Values.orchestration.hub.ping.credentials.clientSecret.secret -}}
    {{- if or $cs.inlineSecret (and $cs.existingSecret $cs.existingSecretKey) -}}
        {{- .Values.orchestration.hub.ping.credentials.clientSecret | toYaml -}}
    {{- else -}}
        {{- .Values.orchestration.security.authentication.oidc | toYaml -}}
    {{- end -}}
{{- end -}}

{{- define "orchestration.authAudience" -}}
    {{- .Values.orchestration.security.authentication.oidc.audience | default "orchestration-api" -}}
{{- end -}}

{{- define "orchestration.authSecretConfig" -}}
    {{- toYaml .Values.orchestration.security.authentication.oidc -}}
{{- end -}}

{{- define "orchestration.enabledProfiles" -}}
    {{- $enabledProfiles := list -}}
    {{- range $key, $value := .Values.orchestration.profiles }}
        {{- $isNoStorageProfile := and
            (or
                (eq $key "operate")
                (eq $key "tasklist")
                (eq $key "consolidated-auth")
            )
            $.Values.global.noSecondaryStorage
        }}
        {{- if and (not $isNoStorageProfile) (eq $value true) }}
            {{- $enabledProfiles = append $enabledProfiles $key }}
        {{- end }}
    {{- end }}
    {{- join "," $enabledProfiles }}
{{- end -}}

{{- define "orchestration.enabledProfilesWithAuth" -}}
    {{- if or
        (eq (include "orchestration.authMethod" .) "oidc")
        (eq (include "orchestration.authMethod" .) "basic")
    }}
        {{- printf "%s,%s" (include "orchestration.enabledProfiles" .) "consolidated-auth" -}}
    {{- else }}
        {{- include "orchestration.enabledProfiles" . | replace "admin" "auth" -}}
    {{- end }}
{{- end -}}

{{- define "orchestration.secondaryStorage" -}}
    {{- if .Values.orchestration.data.secondaryStorage.type -}}
        {{- .Values.orchestration.data.secondaryStorage.type -}}
    {{- else -}}
        {{- if .Values.global.noSecondaryStorage -}}
            none
        {{- else if .Values.orchestration.exporters.rdbms.enabled -}}
            rdbms
        {{- else if .Values.optimize.database.elasticsearch.enabled -}}
            elasticsearch
        {{- else if .Values.optimize.database.opensearch.enabled -}}
            opensearch
        {{- else -}}
            unset
        {{- end -}}
    {{- end -}}
{{- end -}}

{{- define "orchestration.sharedTlsConfig" -}}
{{- $config := dict -}}
{{- if eq (include "camundaPlatform.hasSecretConfig" (dict "config" .Values.orchestration.data.secondaryStorage.elasticsearch.tls)) "true" -}}
    {{- $config = .Values.orchestration.data.secondaryStorage.elasticsearch.tls -}}
{{- else if eq (include "camundaPlatform.hasSecretConfig" (dict "config" .Values.orchestration.data.secondaryStorage.opensearch.tls)) "true" -}}
    {{- $config = .Values.orchestration.data.secondaryStorage.opensearch.tls -}}
{{- end -}}
{{- toYaml $config -}}
{{- end -}}

{{- define "orchestration.legacyExporterTlsConfig" -}}
{{- $config := dict -}}
{{- if and
      (eq (include "orchestration.legacyElasticsearchExporterUsesOptimizeSource" .) "true")
      (eq (include "camundaPlatform.hasSecretConfig" (dict "config" .Values.optimize.database.elasticsearch.tls)) "true")
-}}
    {{- $config = .Values.optimize.database.elasticsearch.tls -}}
{{- else if and
      (eq (include "orchestration.legacyOpenSearchExporterUsesOptimizeSource" .) "true")
      (eq (include "camundaPlatform.hasSecretConfig" (dict "config" .Values.optimize.database.opensearch.tls)) "true")
-}}
    {{- $config = .Values.optimize.database.opensearch.tls -}}
{{- end -}}
{{- toYaml $config -}}
{{- end -}}

{{- /*
NOTE: the Orchestration JVM mounts a single truststore, so the first matching source wins for the
whole pod. An Optimize-owned legacy exporter contributes its component TLS secret first; the chain
falls through to the shared secondary-storage sources otherwise.
*/ -}}
{{- define "orchestration.effectiveTlsConfig" -}}
{{- $exporterTls := include "orchestration.legacyExporterTlsConfig" . | fromYaml -}}
{{- if eq (include "camundaPlatform.hasSecretConfig" (dict "config" $exporterTls)) "true" -}}
{{- toYaml $exporterTls -}}
{{- else -}}
{{- include "orchestration.sharedTlsConfig" . -}}
{{- end -}}
{{- end -}}

{{- define "orchestration.persistentSessionsEnabled" -}}
    {{ not .Values.global.noSecondaryStorage -}}
{{- end -}}


{{- /*
NOTE: resolves the effective camunda-exporter toggle. Starts from the deprecated
orchestration.exporters.camunda.enabled key and lets its migration target,
camunda.data.secondary-storage.autoconfigure-camunda-exporter supplied through a
spring-imported orchestration.extraConfiguration file, override it.
*/ -}}
{{- define "orchestration.camundaExporterEnabled" -}}
{{- include "camundaPlatform.effectiveExtraConfigValue" (dict
  "default" .Values.orchestration.exporters.camunda.enabled
  "extraConfiguration" .Values.orchestration.extraConfiguration
  "path" (list "camunda" "data" "secondary-storage" "autoconfigure-camunda-exporter")
) -}}
{{- end -}}

{{/*
[orchestration] The backend the legacy exporters key off is the resolved one, via
orchestration.secondaryStorage: an unset orchestration.data.secondaryStorage.type still resolves to
elasticsearch or opensearch through optimize.database.<backend>.enabled,
and reading the raw key instead would silently drop an explicit orchestration.exporters.zeebe opt-in.
*/}}
{{- define "orchestration.hasElasticsearchExporter" -}}
{{- and
      (or
        (and (eq (include "orchestration.secondaryStorage" .) "elasticsearch") .Values.orchestration.exporters.zeebe.enabled)
        (and .Values.optimize.database.elasticsearch.enabled (eq (include "camundaPlatform.optimizeEnabled" .) "true"))
      )
      (or
        .Values.orchestration.exporters.zeebe.enabled
        (lt (int (default 0 .Values.global.multiregion.regions)) 2)
      )
-}}
{{- end -}}

{{- define "orchestration.hasOpenSearchExporter" -}}
{{- and
      (or
        (and (eq (include "orchestration.secondaryStorage" .) "opensearch") .Values.orchestration.exporters.zeebe.enabled)
        (and .Values.optimize.database.opensearch.enabled (eq (include "camundaPlatform.optimizeEnabled" .) "true"))
      )
      (or
        .Values.orchestration.exporters.zeebe.enabled
        (lt (int (default 0 .Values.global.multiregion.regions)) 2)
      )
-}}
{{- end -}}

{{/*
NOTE: the legacy exporters connect to the datastore Optimize reads, so their endpoint, credentials,
AWS mode, and TLS all resolve from the Optimize component chain whenever these predicates hold.
The host term mirrors the gate the Optimize deployment uses to render its own connection env vars;
when no host resolves, the exporter keeps the secondary-storage/global compatibility source.
*/}}
{{- define "orchestration.legacyElasticsearchExporterUsesOptimizeSource" -}}
{{- and
      (eq (include "orchestration.hasElasticsearchExporter" .) "true")
      (eq (include "camundaPlatform.optimizeEnabled" .) "true")
      (ne (include "camundaPlatform.elasticsearchHost" .) "")
-}}
{{- end -}}

{{- define "orchestration.legacyOpenSearchExporterUsesOptimizeSource" -}}
{{- and
      (ne (include "orchestration.hasElasticsearchExporter" .) "true")
      (eq (include "orchestration.hasOpenSearchExporter" .) "true")
      (eq (include "camundaPlatform.optimizeEnabled" .) "true")
      (ne (include "camundaPlatform.opensearchHost" .) "")
-}}
{{- end -}}

{{/*
NOTE: Optimize-owned exporters substitute the Optimize-scoped password variable and resolve their
username from the Optimize component chain; otherwise both keep the generic engine-wide sources.
The emission in statefulset.yaml keys off the same predicates as the substitution here.
*/}}
{{- define "orchestration.legacyElasticsearchExporterPasswordRef" -}}
{{- if eq (include "orchestration.legacyElasticsearchExporterUsesOptimizeSource" .) "true" -}}
${VALUES_OPTIMIZE_DATABASE_ELASTICSEARCH_PASSWORD:}
{{- else -}}
${VALUES_ELASTICSEARCH_PASSWORD:}
{{- end -}}
{{- end -}}

{{- define "orchestration.legacyOpenSearchExporterPasswordRef" -}}
{{- if eq (include "orchestration.legacyOpenSearchExporterUsesOptimizeSource" .) "true" -}}
${VALUES_OPTIMIZE_DATABASE_OPENSEARCH_PASSWORD:}
{{- else -}}
${VALUES_OPENSEARCH_PASSWORD:}
{{- end -}}
{{- end -}}

{{- define "orchestration.legacyElasticsearchExporterUsername" -}}
{{- if eq (include "orchestration.legacyElasticsearchExporterUsesOptimizeSource" .) "true" -}}
{{- .Values.optimize.database.elasticsearch.auth.username -}}
{{- else -}}
{{- .Values.optimize.database.elasticsearch.auth.username | default .Values.orchestration.data.secondaryStorage.elasticsearch.auth.username -}}
{{- end -}}
{{- end -}}

{{- define "orchestration.legacyOpenSearchExporterUsername" -}}
{{- if eq (include "orchestration.legacyOpenSearchExporterUsesOptimizeSource" .) "true" -}}
{{- .Values.optimize.database.opensearch.auth.username -}}
{{- else -}}
{{- .Values.optimize.database.opensearch.auth.username | default .Values.orchestration.data.secondaryStorage.opensearch.auth.username -}}
{{- end -}}
{{- end -}}

{{- define "orchestration.legacyOpenSearchExporterAuthenticationEnabled" -}}
{{- ne (include "orchestration.legacyOpenSearchExporterAwsEnabled" .) "true" -}}
{{- end -}}

{{/*
NOTE: the legacy OpenSearch exporter resolves AWS mode from the same source as its endpoint and
credentials: the Optimize component chain when Optimize-owned, the secondary-storage/global pair
otherwise.
*/}}
{{- define "orchestration.legacyOpenSearchExporterAwsEnabled" -}}
{{- if eq (include "orchestration.legacyOpenSearchExporterUsesOptimizeSource" .) "true" -}}
{{- .Values.optimize.database.opensearch.aws.enabled -}}
{{- else -}}
{{- .Values.orchestration.data.secondaryStorage.opensearch.aws.enabled -}}
{{- end -}}
{{- end -}}

{{/* Effective writer prefixes exposed to topology contract validation. */}}
{{- define "orchestration.legacyExporterElasticsearchPrefix" -}}
{{- dig "index" "prefix" "" .Values.orchestration.exporters.zeebe | default .Values.optimize.database.elasticsearch.prefix | default "zeebe-record" -}}
{{- end -}}

{{- define "orchestration.legacyExporterOpenSearchPrefix" -}}
{{- dig "index" "prefix" "" .Values.orchestration.exporters.zeebe | default .Values.optimize.database.opensearch.prefix | default "zeebe-record" -}}
{{- end -}}

{{/*
[orchestration] Exporter id of the release's legacy exporter, empty when it runs none.
*/}}
{{- /*
NOTE: matches the nested, dotted and raw-properties forms, as the camunda.secrets check in
constraints.tpl does: one form alone lets a different YAML style slip past the check.
*/ -}}
{{- define "orchestration.physicalTenantsDeclared" -}}
{{- $args := dict "extraConfiguration" .Values.orchestration.extraConfiguration "path" (list "camunda" "physical-tenants") -}}
{{- if or (eq (include "camundaPlatform.extraConfigHasPathInAnyYamlDocument" $args) "true") (eq (include "camundaPlatform.extraConfigHasRawKeyPrefix" $args) "true") -}}
true
{{- end -}}
{{- end -}}

{{- define "orchestration.hasAppIntegrations" -}}
{{- include "camundaPlatform.hasSecretConfig" (dict "config" .Values.orchestration.exporters.appIntegrations.apiKey) -}}
{{- end -}}

{{- define "orchestration.hasAzureDocumentStore" -}}
{{- $storeId := lower .Values.global.documentStore.activeStoreId -}}
{{- and
    (eq (include "camundaPlatform.extraConfigHasPath" (dict
        "extraConfiguration" .Values.orchestration.extraConfiguration
        "path" (list "camunda" "document" "azure" $storeId)
    )) "true")
  (eq (include "camundaPlatform.hasSecretConfig" (dict "config" .Values.global.documentStore.type.azure.connectionString)) "true")
-}}
{{- end -}}


{{/*
********************************************************************************
Service names.
********************************************************************************
*/}}

{{/*
[orchestration] Define Orchestration Cluster service name - Main.
*/}}
{{- define "orchestration.serviceName" }}
    {{- include "orchestration.fullname" . -}}-gateway
{{- end -}}

{{/*
[orchestration] Define Orchestration Cluster service name - Main - gRPC.
*/}}
{{- define "orchestration.serviceNameGRPC" }}
    {{- include "orchestration.serviceName" . -}}:{{ .Values.orchestration.service.grpcPort }}
{{- end -}}

{{/*
[orchestration] Define Orchestration Cluster service name - Main - HTTP.
*/}}
{{- define "orchestration.serviceNameHTTP" }}
    {{- include "orchestration.serviceName" . -}}:{{ .Values.orchestration.service.httpPort }}
{{- end -}}

{{/*
[orchestration] Define Orchestration Cluster service name - Headless.
*/}}
{{- define "orchestration.serviceNameHeadless" }}
    {{- include "orchestration.fullname" . -}}
{{- end -}}

{{/*
[orchestration] Define Orchestration Cluster service name - Headless - gRPC.
*/}}
{{- define "orchestration.serviceNameHeadlessGRPC" }}
    {{- include "orchestration.serviceNameHeadless" . -}}:{{ .Values.orchestration.service.grpcPort }}
{{- end -}}

{{/*
********************************************************************************
Service labels.
********************************************************************************
*/}}

{{/*
[orchestration] Define Orchestration Cluster service labels - Main.
*/}}
{{- define "orchestration.serviceLabels" }}
    {{- include "camundaPlatform.labels" . }}
    {{- "\n" }}
    {{- include "orchestration.extraLabelsGatewayService" . }}
{{- end -}}

{{/*
[orchestration] Define Orchestration Cluster service labels - Headless.
*/}}
{{- define "orchestration.serviceLabelsHeadless" }}
    {{- include "camundaPlatform.labels" . }}
    {{- "\n" }}
    {{- include "orchestration.extraLabelsBrokerServiceHeadless" . }}
{{- end -}}

{{/*
********************************************************************************
URIs.
********************************************************************************
*/}}

{{/*
[orchestration] Orchestration Cluster Redirect URI.
*/}}
{{- define "orchestration.RedirectURI" -}}
    {{- $redirectURIDefault := include "orchestration.serviceNameHTTP" . -}}
    {{- tpl .Values.orchestration.security.authentication.oidc.redirectUrl . | default $redirectURIDefault -}}
{{- end -}}
