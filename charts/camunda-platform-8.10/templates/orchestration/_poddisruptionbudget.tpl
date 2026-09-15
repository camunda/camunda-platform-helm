{{- define "orchestration.podDisruptionBudgetManifest" -}}
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: {{ include "orchestration.zoneFullname" (dict "context" . "zone" (include "orchestration.scopedZone" .)) }}
  labels:
    {{- include "orchestration.labels" . | nindent 4 }}
spec:
  {{- if .Values.orchestration.podDisruptionBudget.minAvailable }}
  minAvailable: {{ .Values.orchestration.podDisruptionBudget.minAvailable | default 0 }}
  {{- else }}
  maxUnavailable: {{ .Values.orchestration.podDisruptionBudget.maxUnavailable }}
  {{- end }}
  selector:{{- if and .OrchestrationRender (eq .OrchestrationRender.scope "unzoned") }}
    matchExpressions:
      - key: camunda.io/broker-generation
        operator: In
        values: [numbered]
{{- end }}
    matchLabels:
      {{- include "orchestration.matchLabels" . | nindent 6 }}
{{- end -}}
