{{- define "orchestration.podDisruptionBudgetManifest" -}}
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: {{ include "orchestration.zoneFullname" . }}
  labels:
    {{- include "orchestration.labels" . | nindent 4 }}
spec:
  {{- if .Values.orchestration.podDisruptionBudget.minAvailable }}
  minAvailable: {{ .Values.orchestration.podDisruptionBudget.minAvailable | default 0 }}
  {{- else }}
  maxUnavailable: {{ .Values.orchestration.podDisruptionBudget.maxUnavailable }}
  {{- end }}
  selector:
    matchLabels:
      {{- include "orchestration.matchLabels" . | nindent 6 }}
{{- end -}}
