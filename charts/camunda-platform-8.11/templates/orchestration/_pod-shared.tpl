{{- /* NOTE: shared by the broker StatefulSet and the standalone gateway Deployment; each block keeps the container indentation of both pod specs. */ -}}

{{- define "orchestration.apiTlsEnv" -}}
            {{- if .Values.global.tls.orchestration.rest.enabled }}
            - name: SERVER_SSL_ENABLED
              value: "true"
            {{- end }}
            {{- if eq (include "camundaPlatform.orchestrationRESTTLSEnabled" .) "true" }}
            {{- $restCertRef := include "camundaPlatform.orchestrationTLSCertRef" (dict "context" . "proto" "rest") | fromYaml }}
            {{- $restKeyRef := include "camundaPlatform.orchestrationTLSPrivateKeyRef" (dict "context" . "proto" "rest") | fromYaml }}
            {{- $restKsPwRef := include "camundaPlatform.orchestrationTLSKeystorePasswordRef" (dict "context" .) | fromYaml }}
            {{- if $restCertRef.name }}
            {{- if eq (.Values.global.tls.orchestration.rest.type | default "pkcs12") "pem" }}
            - name: SERVER_SSL_CERTIFICATE
              value: /usr/local/camunda/certificates/orchestration/rest/{{ $restCertRef.key }}
            - name: SERVER_SSL_CERTIFICATE_PRIVATE_KEY
            {{- if eq $restKeyRef.name $restCertRef.name }}
              value: /usr/local/camunda/certificates/orchestration/rest/{{ $restKeyRef.key }}
            {{- else }}
              value: /usr/local/camunda/certificates/orchestration/rest/key/{{ $restKeyRef.key }}
            {{- end }}
            {{- else }}
            - name: SERVER_SSL_KEY_STORE
              value: file:/usr/local/camunda/certificates/orchestration/rest/{{ $restCertRef.key }}
            - name: SERVER_SSL_KEY_STORE_TYPE
              value: PKCS12
            - name: SERVER_SSL_KEY_STORE_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: {{ $restKsPwRef.name }}
                  key: {{ $restKsPwRef.key }}
            {{- with .Values.global.tls.orchestration.rest.keyAlias }}
            - name: SERVER_SSL_KEY_ALIAS
              value: {{ . | quote }}
            {{- end }}
            {{- end }}
            {{- end }}
            {{- end }}
            {{- if .Values.global.tls.orchestration.grpc.enabled }}
            - name: CAMUNDA_API_GRPC_SSL_ENABLED
              value: "true"
            {{- end }}
            {{- if eq (include "camundaPlatform.orchestrationGRPCTLSEnabled" .) "true" }}
            {{- $grpcCertRef := include "camundaPlatform.orchestrationTLSCertRef" (dict "context" . "proto" "grpc") | fromYaml }}
            {{- $grpcKeyRef := include "camundaPlatform.orchestrationTLSPrivateKeyRef" (dict "context" . "proto" "grpc") | fromYaml }}
            {{- if $grpcCertRef.name }}
            - name: CAMUNDA_API_GRPC_SSL_CERTIFICATE
              value: /usr/local/camunda/certificates/orchestration/grpc/{{ $grpcCertRef.key }}
            - name: CAMUNDA_API_GRPC_SSL_CERTIFICATEPRIVATEKEY
            {{- if eq $grpcKeyRef.name $grpcCertRef.name }}
              value: /usr/local/camunda/certificates/orchestration/grpc/{{ $grpcKeyRef.key }}
            {{- else }}
              value: /usr/local/camunda/certificates/orchestration/grpc/key/{{ $grpcKeyRef.key }}
            {{- end }}
            {{- end }}
            {{- end }}
{{- end -}}

{{- define "orchestration.documentStoreEnv" -}}
            {{- if and .Values.global.documentStore.type.aws.enabled (not .Values.global.documentStore.type.aws.irsa.enabled) }}
            - name: AWS_ACCESS_KEY_ID
              {{- include "camundaPlatform.emitAwsDocumentStoreSecret" (dict
                  "secretType" "accessKeyId"
                  "context" .
              ) | nindent 14 }}
            - name: AWS_SECRET_ACCESS_KEY
              {{- include "camundaPlatform.emitAwsDocumentStoreSecret" (dict
                  "secretType" "secretAccessKey"
                  "context" .
              ) | nindent 14 }}
            {{- end }}
            {{- if eq (lower .Values.global.documentStore.activeStoreId) "gcp" }}
            - name: GOOGLE_APPLICATION_CREDENTIALS
              value: {{ printf "%s/%s" .Values.global.documentStore.type.gcp.mountPath .Values.global.documentStore.type.gcp.fileName | quote }}
            {{- end }}
            {{- include "camundaPlatform.emitEnvVarFromSecretConfig" (dict
                "envName" "VALUES_DOCUMENT_STORE_AZURE_CONNECTION_STRING"
                "config" .Values.global.documentStore.type.azure.connectionString
            ) | nindent 12 }}
{{- end -}}

{{- define "orchestration.secondaryStorageCredentialsEnv" -}}
            {{- if (eq (include "camundaPlatform.hasSecretConfig" (dict "config" .Values.orchestration.data.secondaryStorage.elasticsearch.auth)) "true") }}
            {{- include "camundaPlatform.emitEnvVarFromSecretConfig" (dict
                "envName" "VALUES_ELASTICSEARCH_PASSWORD"
                "config" .Values.orchestration.data.secondaryStorage.elasticsearch.auth
            ) | nindent 12 }}
            {{- end }}
            {{- $hasOpenSearchConsumer := or
              (eq (include "orchestration.secondaryStorage" .) "opensearch")
              (and
                (ne (include "orchestration.hasElasticsearchExporter" .) "true")
                (eq (include "orchestration.hasOpenSearchExporter" .) "true")
              )
            -}}
            {{- if and $hasOpenSearchConsumer (eq (include "camundaPlatform.hasSecretConfig" (dict "config" .Values.orchestration.data.secondaryStorage.opensearch.auth)) "true") }}
            {{- include "camundaPlatform.emitEnvVarFromSecretConfig" (dict
                "envName" "VALUES_OPENSEARCH_PASSWORD"
                "config" .Values.orchestration.data.secondaryStorage.opensearch.auth
            ) | nindent 12 }}
            {{- end }}
{{- end -}}

{{- define "orchestration.rdbmsCredentialsEnv" -}}
            {{- if and .Values.orchestration.exporters.rdbms.enabled (eq (include "camundaPlatform.hasSecretConfig" (dict "config" .Values.orchestration.data.secondaryStorage.rdbms)) "true") }}
            {{- include "camundaPlatform.emitEnvVarFromSecretConfig" (dict
                "envName" "VALUES_ORCHESTRATION_DATA_SECONDARYSTORAGE_RDBMS_PASSWORD"
                "config" .Values.orchestration.data.secondaryStorage.rdbms
            ) | nindent 12 }}
            {{- end }}
{{- end -}}

{{- define "orchestration.sharedVolumeMounts" -}}
            {{- if .Values.global.documentStore.type.gcp.enabled }}
            - name: gcp-credentials-volume
              mountPath: {{ .Values.global.documentStore.type.gcp.mountPath | quote }}
              readOnly: true
            {{- end }}
            {{- range $volume := (fromYaml (include "camundaPlatform.secretStore.fileVolumes" .)).volumes }}
            - name: {{ $volume.name }}
              mountPath: {{ $volume.path | quote }}
              readOnly: true
            {{- end }}
            {{- if eq (include "camundaPlatform.hasCaBundle" .) "true" }}
            {{- include "camundaPlatform.caBundleVolumeMount" . | nindent 12 }}
            {{- include "camundaPlatform.caBundleTruststoreVolumeMount" . | nindent 12 }}
            {{- end }}
{{- end -}}

{{- define "orchestration.apiTlsVolumeMounts" -}}
            {{- if eq (include "camundaPlatform.orchestrationRESTTLSEnabled" .) "true" }}
            {{- $restCertRef := include "camundaPlatform.orchestrationTLSCertRef" (dict "context" . "proto" "rest") | fromYaml }}
            {{- $restKeyRef := include "camundaPlatform.orchestrationTLSPrivateKeyRef" (dict "context" . "proto" "rest") | fromYaml }}
            {{- if $restCertRef.name }}
            - name: orchestration-tls-rest
              mountPath: /usr/local/camunda/certificates/orchestration/rest
              readOnly: true
            {{- end }}
            {{- if and $restKeyRef.name (ne $restKeyRef.name $restCertRef.name) }}
            - name: orchestration-tls-rest-key
              mountPath: /usr/local/camunda/certificates/orchestration/rest/key
              readOnly: true
            {{- end }}
            {{- end }}
            {{- if eq (include "camundaPlatform.orchestrationGRPCTLSEnabled" .) "true" }}
            {{- $grpcCertRef := include "camundaPlatform.orchestrationTLSCertRef" (dict "context" . "proto" "grpc") | fromYaml }}
            {{- $grpcKeyRef := include "camundaPlatform.orchestrationTLSPrivateKeyRef" (dict "context" . "proto" "grpc") | fromYaml }}
            {{- if $grpcCertRef.name }}
            - name: orchestration-tls-grpc
              mountPath: /usr/local/camunda/certificates/orchestration/grpc
              readOnly: true
            {{- end }}
            {{- if and $grpcKeyRef.name (ne $grpcKeyRef.name $grpcCertRef.name) }}
            - name: orchestration-tls-grpc-key
              mountPath: /usr/local/camunda/certificates/orchestration/grpc/key
              readOnly: true
            {{- end }}
            {{- end }}
{{- end -}}

{{- define "orchestration.sharedVolumes" -}}
        {{- if .Values.global.documentStore.type.gcp.enabled }}
        {{- include "camundaPlatform.emitVolumeFromSecretConfig" (dict
            "volumeName" "gcp-credentials-volume"
            "config" .Values.global.documentStore.type.gcp
            "fileName" .Values.global.documentStore.type.gcp.fileName
        ) | nindent 8 }}
        {{- end }}
        {{- range $volume := (fromYaml (include "camundaPlatform.secretStore.fileVolumes" .)).volumes }}
        - name: {{ $volume.name }}
          secret:
            secretName: {{ $volume.secret | quote }}
            defaultMode: 0400
        {{- end }}
        {{- if eq (include "camundaPlatform.hasCaBundle" .) "true" }}
        {{- include "camundaPlatform.caBundleVolume" . | nindent 8 }}
        {{- include "camundaPlatform.caBundleTruststoreVolume" . | nindent 8 }}
        {{- end }}
{{- end -}}

{{- define "orchestration.apiTlsVolumes" -}}
        {{- if eq (include "camundaPlatform.orchestrationRESTTLSEnabled" .) "true" }}
        {{- $restCertRef := include "camundaPlatform.orchestrationTLSCertRef" (dict "context" . "proto" "rest") | fromYaml }}
        {{- $restKeyRef := include "camundaPlatform.orchestrationTLSPrivateKeyRef" (dict "context" . "proto" "rest") | fromYaml }}
        {{- if $restCertRef.name }}
        - name: orchestration-tls-rest
          secret:
            secretName: {{ $restCertRef.name }}
            defaultMode: 0440
        {{- end }}
        {{- if and $restKeyRef.name (ne $restKeyRef.name $restCertRef.name) }}
        - name: orchestration-tls-rest-key
          secret:
            secretName: {{ $restKeyRef.name }}
            defaultMode: 0440
        {{- end }}
        {{- end }}
        {{- if eq (include "camundaPlatform.orchestrationGRPCTLSEnabled" .) "true" }}
        {{- $grpcCertRef := include "camundaPlatform.orchestrationTLSCertRef" (dict "context" . "proto" "grpc") | fromYaml }}
        {{- $grpcKeyRef := include "camundaPlatform.orchestrationTLSPrivateKeyRef" (dict "context" . "proto" "grpc") | fromYaml }}
        {{- if $grpcCertRef.name }}
        - name: orchestration-tls-grpc
          secret:
            secretName: {{ $grpcCertRef.name }}
            defaultMode: 0440
        {{- end }}
        {{- if and $grpcKeyRef.name (ne $grpcKeyRef.name $grpcCertRef.name) }}
        - name: orchestration-tls-grpc-key
          secret:
            secretName: {{ $grpcKeyRef.name }}
            defaultMode: 0440
        {{- end }}
        {{- end }}
{{- end -}}
