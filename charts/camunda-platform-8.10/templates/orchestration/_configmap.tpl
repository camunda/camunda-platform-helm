{{- define "orchestration.configmapManifest" -}}
{{- $mr := include "camundaPlatform.multiregion" $ | fromYaml -}}
kind: ConfigMap
metadata:
  name: {{ include "orchestration.zoneFullname" . }}-configuration
  labels:
    {{- include "orchestration.labels" . | nindent 4 }}
apiVersion: v1
data:
  startup.sh: |
    # The Node ID depends on the StatefulSet Pod's name so it cannot be templated in the StatefulSet level.
    # The value of "node-id" is calculated in the "startup.sh" file and exported as "VALUES_ORCHESTRATION_NODE_ID" env var.
    {{- if eq (include "orchestration.zoned" .) "true" }}
    # Zone-aware brokers are identified by the composite member ID "<zone>_<node-id>",
    # so the node ID is the index of the broker inside its own zone, 0 to
    # numberOfBrokers-1. The zone name is what keeps it unique across the cluster.
    export VALUES_ORCHESTRATION_NODE_ID="${VALUES_ORCHESTRATION_NODE_ID:-${K8S_NAME##*-}}"
    {{- else }}
    export VALUES_ORCHESTRATION_NODE_ID="${VALUES_ORCHESTRATION_NODE_ID:-$[${K8S_NAME##*-} * {{ $mr.regions }} + {{ $mr.regionId }}]}"
    {{- end }}
    echo "export VALUES_ORCHESTRATION_NODE_ID=${VALUES_ORCHESTRATION_NODE_ID}"

    if [ "$ZEEBE_RESTORE" = "true" ]; then
      if [ "${ZEEBE_RESTORE_FROM_BACKUP_ID:-}" ]; then
        exec restore --backupId="${ZEEBE_RESTORE_FROM_BACKUP_ID}"
      elif [ "${ZEEBE_RESTORE_FROM_TIMESTAMP:-}" ] && [ "${ZEEBE_RESTORE_TO_TIMESTAMP:-}" ]; then
        exec restore --from="${ZEEBE_RESTORE_FROM_TIMESTAMP}" --to="${ZEEBE_RESTORE_TO_TIMESTAMP}"
      elif [ "${ZEEBE_RESTORE_FROM_TIMESTAMP:-}" ]; then
        exec restore --from="${ZEEBE_RESTORE_FROM_TIMESTAMP}"
      elif [ "${ZEEBE_RESTORE_TO_TIMESTAMP:-}" ]; then
        exec restore --to="${ZEEBE_RESTORE_TO_TIMESTAMP}"
      else
        exec restore
      fi
    else
      exec camunda
    fi

  {{- if .Values.orchestration.configuration }}
  application.yaml: |
    {{ .Values.orchestration.configuration | indent 4 | trim }}
  {{- else }}
  application.yaml: |
    {{- (include (print $.Template.BasePath "/orchestration/files/_application.yaml") $) | indent 4 }}
  {{- end }}
  {{- if and .Values.orchestration.log4j2 (ne (include "camundaPlatform.extraConfigHasFile" (dict "extraConfiguration" .Values.orchestration.extraConfiguration "file" "log4j2.xml")) "true") }}
  log4j2.xml: |
    {{ .Values.orchestration.log4j2 | indent 4 | trim }}
  {{- end }}

  {{- include "camundaPlatform.renderExtraConfiguration" (dict "extraConfig" .Values.orchestration.extraConfiguration) }}
{{- end -}}
