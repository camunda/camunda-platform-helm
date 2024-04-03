
{{- define "operate.configmap.serverConfiguration" }}
    {{- if .Values.operate.contextPath }}
    server:
      servlet:
        context-path: {{ .Values.operate.contextPath | quote }}
    {{- end }}
{{- end }}

{{- define "operate.configmap.identityConfiguration" }}
    {{- if .Values.global.identity.auth.enabled }}
    spring:
      profiles:
        active: "identity-auth"
      security:
        oauth2:
          resourceserver:
            jwt:
              issuer-uri: {{ include "camundaPlatform.authIssuerBackendUrl" . | quote }}
              jwk-set-uri: {{ include "camundaPlatform.authIssuerBackendUrlCertsEndpoint" . | quote }}

    camunda:
      identity:
        clientId: {{ include "operate.authClientId" . | quote }}
        audience: {{ include "operate.authAudience" . | quote }}
    {{- else }}
    spring:
      profiles:
        active: "auth"
    {{- end }}
{{- end }}

{{- define "operate.configmap.common" }}
      {{- if .Values.global.opensearch.enabled }}
      database: opensearch
      {{- end }}
      {{- if .Values.global.multitenancy.enabled }}
      multiTenancy:
        enabled: true
      {{- end }}
      {{- if .Values.global.identity.auth.enabled }}
      identity:
        redirectRootUrl: {{ tpl .Values.global.identity.auth.operate.redirectUrl $ | trimSuffix .Values.operate.contextPath | quote }}
      {{- end }}
    
      # ELS instance to store Operate data
      {{- if .Values.global.elasticsearch.enabled }}
      elasticsearch:
        # Cluster name
        clusterName: {{ .Values.global.elasticsearch.clusterName }}
        {{- if .Values.global.elasticsearch.external }}
        username: {{ .Values.global.elasticsearch.auth.username | quote }}
        {{- end }}
        # Host
        host: {{ include "camundaPlatform.elasticsearchHost" . }}
        # Transport port
        port: {{ .Values.global.elasticsearch.url.port }}
        {{- if .Values.global.elasticsearch.url.host }}
        # Elasticsearch full url
        url: {{ include "camundaPlatform.elasticsearchURL" . | quote }}
        {{- end }}
      # ELS instance to export Zeebe data to
      zeebeElasticsearch:
        # Cluster name
        clusterName: {{ .Values.global.elasticsearch.clusterName }}
        # Host
        host: {{ include "camundaPlatform.elasticsearchHost" . }}
        # Transport port
        port: {{ .Values.global.elasticsearch.url.port }}
        # Index prefix, configured in Zeebe Elasticsearch exporter
        prefix: {{ .Values.global.elasticsearch.prefix }}
        {{- if .Values.global.elasticsearch.url.host }}
        # Elasticsearch full url
        url: {{ include "camundaPlatform.elasticsearchURL" . | quote }}
        {{- end }}
        {{- if .Values.global.elasticsearch.external }}
        # Elasticsearch username
        username: {{ .Values.global.elasticsearch.auth.username | quote }}
        {{- end }}
      {{- end }}
      {{- if .Values.global.opensearch.enabled }}
      opensearch:
        url: {{ include "camundaPlatform.opensearchURL" . | quote }}
        username: {{ .Values.global.opensearch.auth.username | quote }}
      zeebeOpensearch:
        url: {{ include "camundaPlatform.opensearchURL" . | quote }}
        username: {{ .Values.global.opensearch.auth.username | quote }}
      {{- end }}
      # Zeebe instance
      zeebe:
        # Broker contact point
        brokerContactPoint: "{{ tpl .Values.global.zeebeClusterName . }}-gateway:{{ .Values.zeebeGateway.service.grpcPort }}"
      {{- if .Values.operate.retention.enabled }}
      archiver:
        ilmEnabled: true
        ilmMinAgeForDeleteArchivedIndices: {{ .Values.operate.retention.minimumAge }}
      {{- end }}
    logging:
{{- with .Values.operate.logging }}
{{ . | toYaml | indent 6 }}
{{- end }}
    #Spring Boot Actuator endpoints to be exposed
    management.endpoints.web.exposure.include: health,info,conditions,configprops,prometheus,loggers,usage-metrics,backups
{{- end }}
