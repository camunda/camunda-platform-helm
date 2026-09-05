{{- define "capacityManager.fullname" -}}
    {{- include "camundaPlatform.componentFullname" (dict
        "componentName" "capacity-manager"
        "componentValues" .Values.capacityManager
        "context" $
    ) -}}
{{- end -}}

{{- define "capacityManager.labels" -}}
    {{- include "camundaPlatform.componentLabels" (dict "componentName" "capacity-manager" "componentValuesKey" "capacityManager" "context" $) -}}
{{- end -}}

{{- define "capacityManager.matchLabels" -}}
    {{- include "camundaPlatform.componentMatchLabels" (dict "componentName" "capacity-manager" "context" $) -}}
{{- end -}}

{{- define "capacityManager.serviceAccountName" -}}
    {{- include "camundaPlatform.serviceAccountName" (dict "component" "capacityManager" "context" $) -}}
{{- end -}}

{{- define "capacityManager.imagePullSecrets" -}}
{{- include "camundaPlatform.componentImagePullSecrets" (dict "Values" (set (deepCopy .Values) "image" .Values.capacityManager.image)) }}
{{- end -}}
