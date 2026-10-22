{{- define "orchestration.standaloneGatewayDeploymentManifest" -}}
{{- $gw := .Values.orchestration.standaloneGateway -}}
{{- $tlsConfig := include "orchestration.effectiveTlsConfig" . | fromYaml -}}
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ include "orchestration.standaloneGatewayFullname" . }}
  labels:
    {{- include "orchestration.standaloneGatewayLabels" . | nindent 4 }}
  annotations:
    {{- range $key, $value := .Values.global.annotations }}
    {{ $key }}: {{ $value | quote }}
    {{- end }}
spec:
  {{- if not $gw.autoscaling.enabled }}
  replicas: {{ $gw.replicas }}
  {{- end }}
  selector:
    matchLabels:
      {{- include "orchestration.standaloneGatewayMatchLabels" . | nindent 6 }}
  strategy:
    type: RollingUpdate
  template:
    metadata:
      labels:
        {{- include "orchestration.standaloneGatewayLabels" . | nindent 8 }}
        {{- if $gw.podLabels }}
        {{- tpl (toYaml $gw.podLabels) $ | nindent 8 }}
        {{- end }}
      annotations:
        checksum/config: {{ include "orchestration.standaloneGatewayConfigChecksum" . }}
        {{- if (include "camundaPlatform.secretStore.serviceAccountAnnotations" .) }}
        checksum/secret-store-identity: {{ include "camundaPlatform.secretStore.serviceAccountAnnotations" . | sha256sum }}
        {{- end }}
        {{- with (include "camundaPlatform.caBundleChecksumAnnotation" .) }}{{ . | nindent 8 }}{{- end }}
        {{- with (include "camundaPlatform.orchestrationTLSChecksumAnnotations" .) }}{{ . | nindent 8 }}{{- end }}
        {{- if $gw.podAnnotations }}
        {{- tpl (toYaml $gw.podAnnotations) $ | nindent 8 }}
        {{- end }}
    spec:
      {{- if $gw.priorityClassName }}
      priorityClassName: {{ $gw.priorityClassName | quote }}
      {{- end }}
      imagePullSecrets:
        {{- include "orchestration.imagePullSecrets" . | nindent 8 }}
      {{- if or (eq (include "camundaPlatform.hasCaBundle" .) "true") $gw.initContainers }}
      initContainers:
        {{- if eq (include "camundaPlatform.hasCaBundle" .) "true" }}
        {{- include "camundaPlatform.caBundleInitContainer" (dict "context" $ "image" (include "camundaPlatform.imageByParams" (dict "base" $.Values.global "overlay" $.Values.orchestration))) | nindent 8 }}
        {{- end }}
        {{- with $gw.initContainers }}
        {{- tpl (toYaml . | nindent 8) $ }}
        {{- end }}
      {{- end }}
      containers:
        - name: standalone-gateway
          image: {{ include "camundaPlatform.imageByParams" (dict "base" .Values.global "overlay" .Values.orchestration) }}
          imagePullPolicy: {{ .Values.global.image.pullPolicy }}
          {{- if $gw.containerSecurityContext }}
          securityContext: {{- include "common.compatibility.renderSecurityContext" (dict "secContext" $gw.containerSecurityContext "context" $) | nindent 12 }}
          {{- end }}
          {{- with $gw.command }}
          command: {{ toYaml . | nindent 10 }}
          {{- end }}
          env:
            - name: LC_ALL
              value: C.UTF-8
            - name: K8S_NAME
              valueFrom:
                fieldRef:
                  fieldPath: metadata.name
            - name: K8S_POD_IP
              valueFrom:
                fieldRef:
                  fieldPath: status.podIP
            - name: K8S_SERVICE_NAME
              value: {{ include "orchestration.serviceNameHeadless" . | quote }}
            - name: K8S_NAMESPACE
              valueFrom:
                fieldRef:
                  fieldPath: metadata.namespace
            {{- include "camundaPlatform.emitEnvVarFromSecretConfig" (dict
                "envName" "VALUES_ORCHESTRATION_LICENSE_KEY"
                "config" .Values.global.license
            ) | nindent 12 }}
            {{- if eq (include "orchestration.authMethod" .) "oidc" }}
            {{- $orchestrationAuthSecret := include "orchestration.authSecretConfig" . | fromYaml }}
            {{- include "camundaPlatform.emitEnvVarFromSecretConfig" (dict
                "envName" "VALUES_ORCHESTRATION_CLIENT_SECRET"
                "config" $orchestrationAuthSecret
            ) | nindent 12 }}
            {{- end }}
            {{- if eq (include "camundaPlatform.hasSecretConfig" (dict "config" $tlsConfig)) "true" }}
            {{- include "common.java_tool_options_tls_env" (dict "Values" .Values "component" "orchestration" "tlsConfig" $tlsConfig) | nindent 12 }}
            {{- else if eq (include "camundaPlatform.hasCaBundle" .) "true" }}
            - name: JAVA_TOOL_OPTIONS
              value: {{ printf "%s %s" .Values.orchestration.javaOpts (include "camundaPlatform.caBundleJavaOpts" .) | trim | quote }}
            {{- else }}
            - name: JAVA_TOOL_OPTIONS
              value: {{ .Values.orchestration.javaOpts | quote }}
            {{- end }}
            {{- if eq (include "camundaPlatform.hasCaBundle" .) "true" }}
            {{- include "camundaPlatform.caBundleEnv" . | nindent 12 }}
            {{- end }}
            {{- include "orchestration.apiTlsEnv" . }}
            {{- with .Values.orchestration.env }}
              {{- tpl (toYaml .) $ | nindent 12 }}
            {{- end }}
            {{- include "orchestration.documentStoreEnv" . }}
            {{- include "orchestration.secondaryStorageCredentialsEnv" . }}
            {{- include "orchestration.rdbmsCredentialsEnv" . }}
          {{- $documentStoreEnvFrom := ne (include "camundaPlatform.extraConfigHasPath" (dict "extraConfiguration" .Values.orchestration.extraConfiguration "path" (list "camunda" "document"))) "true" }}
          {{- if or $documentStoreEnvFrom .Values.orchestration.envFrom }}
          envFrom:
          {{- if $documentStoreEnvFrom }}
            - configMapRef:
                name: {{ include "camundaPlatform.fullname" . }}-documentstore-env-vars
          {{- end }}
          {{- if .Values.orchestration.envFrom }}
            {{- .Values.orchestration.envFrom | toYaml | nindent 12 }}
          {{- end }}
          {{- end }}
          ports:
            - containerPort: {{ .Values.orchestration.service.httpPort }}
              name: {{ default "http" .Values.orchestration.service.httpName }}
            - containerPort: {{ .Values.orchestration.service.internalPort }}
              name: {{ default "internal" .Values.orchestration.service.internalName }}
            - containerPort: {{ .Values.orchestration.service.managementPort }}
              name: {{ default "management" .Values.orchestration.service.managementName }}
            - containerPort: {{ .Values.orchestration.service.grpcPort }}
              name: {{ default "grpc" .Values.orchestration.service.grpcName }}
          {{- range $probeName := (list "startupProbe" "readinessProbe" "livenessProbe") }}
          {{- $probe := index $gw $probeName }}
          {{- if $probe.enabled }}
          {{ $probeName }}:
            httpGet:
              path: {{ include "camundaPlatform.joinpath" (list $.Values.orchestration.contextPath $probe.probePath) }}
              scheme: {{ $probe.scheme }}
              port: {{ $.Values.orchestration.service.managementPort }}
            initialDelaySeconds: {{ $probe.initialDelaySeconds }}
            periodSeconds: {{ $probe.periodSeconds }}
            successThreshold: {{ $probe.successThreshold }}
            failureThreshold: {{ $probe.failureThreshold }}
            timeoutSeconds: {{ $probe.timeoutSeconds }}
          {{- end }}
          {{- end }}
          resources:
            {{- toYaml $gw.resources | nindent 12 }}
          volumeMounts:
            {{- if eq (include "camundaPlatform.hasSecretConfig" (dict "config" $tlsConfig)) "true" }}
            - name: keystore
              mountPath: /usr/local/camunda/certificates/{{ include "camundaPlatform.getTlsSecretKey" (dict "config" $tlsConfig) }}
              subPath: {{ include "camundaPlatform.getTlsSecretKey" (dict "config" $tlsConfig) }}
            {{- end }}
            - mountPath: /tmp
              name: tmp
            {{- if and .Values.orchestration.log4j2 (ne (include "camundaPlatform.extraConfigHasFile" (dict "extraConfiguration" .Values.orchestration.extraConfiguration "file" "log4j2.xml")) "true") }}
            - name: config
              mountPath: /usr/local/camunda/config/log4j2.xml
              subPath: log4j2.xml
            {{- end }}
            - name: config
              mountPath: /usr/local/camunda/config/application.yaml
              subPath: application.yaml
            {{- with .Values.orchestration.extraConfiguration }}
            {{- include "camundaPlatform.extraConfigurationVolumeMounts" (dict "extraConfig" . "volumeName" "config" "basePath" "/usr/local/camunda/config") | trim | nindent 12 }}
            {{- end }}
            {{- include "orchestration.sharedVolumeMounts" . }}
            {{- include "orchestration.apiTlsVolumeMounts" . }}
            {{- with $gw.extraVolumeMounts }}
            {{- toYaml . | nindent 12 }}
            {{- end }}
      {{- with $gw.sidecars }}
      {{- toYaml . | nindent 8 }}
      {{- end }}
      volumes:
        {{- if eq (include "camundaPlatform.hasSecretConfig" (dict "config" $tlsConfig)) "true" }}
        {{- include "camundaPlatform.emitTlsVolumeFromSecretConfig" (dict
            "volumeName" "keystore"
            "config" $tlsConfig
        ) | nindent 8 }}
        {{- end }}
        - name: config
          configMap:
            name: {{ include "orchestration.standaloneGatewayFullname" . }}-configuration
            defaultMode: {{ .Values.orchestration.configMap.defaultMode }}
        - name: tmp
          emptyDir: {}
        {{- include "orchestration.sharedVolumes" . }}
        {{- include "orchestration.apiTlsVolumes" . }}
        {{- with $gw.extraVolumes }}
        {{- toYaml . | nindent 8 }}
        {{- end }}
      serviceAccountName: {{ include "orchestration.standaloneGatewayServiceAccountName" . }}
      {{- if kindIs "bool" $gw.automountServiceAccountToken }}
      automountServiceAccountToken: {{ $gw.automountServiceAccountToken }}
      {{- end }}
      {{- with $gw.dnsConfig }}
      dnsConfig: {{- toYaml . | nindent 8 }}
      {{- end }}
      {{- with $gw.dnsPolicy }}
      dnsPolicy: {{ . | quote }}
      {{- end }}
      {{- if $gw.podSecurityContext }}
      securityContext: {{- include "common.compatibility.renderSecurityContext" (dict "secContext" $gw.podSecurityContext "context" $) | nindent 8 }}
      {{- end }}
      {{- with $gw.nodeSelector | default .Values.global.nodeSelector }}
      nodeSelector:
        {{- include "camundaPlatform.toYamlPretty" (dict "value" . "context" $) | nindent 8 }}
      {{- end }}
      {{- with $gw.affinity }}
      affinity:
        {{- include "camundaPlatform.toYamlPretty" (dict "value" (tpl (toYaml .) $ | fromYaml) "context" $) | nindent 8 }}
      {{- end }}
      {{- with $gw.tolerations }}
      tolerations:
        {{- include "camundaPlatform.toYamlPretty" (dict "value" . "context" $) | nindent 8 }}
      {{- end }}
      {{- with $gw.topologySpreadConstraints }}
      topologySpreadConstraints:
        {{- include "camundaPlatform.toYamlPretty" (dict "value" . "context" $) | nindent 8 }}
      {{- end }}
{{- end -}}
