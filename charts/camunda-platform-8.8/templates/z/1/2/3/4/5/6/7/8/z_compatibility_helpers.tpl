{{/*
NOTE: We need to load this file first thing before all other resources to support backward compatibility.

      Helm prioritizes files that are deeply nested in subdirectories when it is determining the render order.
      see the sort function in Helm:
      https://github.com/helm/helm/blob/d58d7b376265338e059ff11c71267b5a6cf504c3/pkg/engine/engine.go#L347-L356

      Because of this sort order, and that we have nested subcharts such that
      one of the rendered templates is:
      charts/keycloak/charts/postgresql/charts/common/templates/validations/_validations.tpl,
      we need this z_compatibility_helpers.tpl to be nested in at least 8 folders.

      In addition to the subdirectory ordering, Helm also orders the templates
      alphabetically descending within the same folder level, which is why it
      is named with a "z_" inside the zeebe directory. so Helm will process
      this file first, and all migration steps will be applied to all templates
      in the chart implicitly:
      https://github.com/helm/helm/blob/d58d7b376265338e059ff11c71267b5a6cf504c3/pkg/engine/engine.go#L362-L369
*/}}


{{/*
********************************************************************************
* Camunda 8.7 => 8.8 backward compatibility.

Overview:
    Backward compatibility to make the values of Camunda Helm chart v12.0.0 (Camunda 8.7)
    works with Camunda Helm chart v13.0.0 (Camunda 8.8).

Approach:
    Using deep copy and deep merge functions to override new keys using the old key.
    https://helm.sh/docs/chart_template_guide/function_list/#mergeoverwrite-mustmergeoverwrite

Note:
    Some values are not compatible with the new values syntax. Thus, the user needs to manually adjust their configurations.
********************************************************************************
*/}}

{{/*
Zeebe => Core.
*/}}

{{/*
Set the default values for the core resources if they use the Zeebe default values,
as the defaults have been changed in Camunda Helm chart v13.0.0 (Camunda 8.8).
If the user has set custom values for the core resources, they will not be overridden.
*/}}
{{- define "compatibility.coreResources" -}}
  {{- if eq .values.core.resources.requests.cpu "800m" -}}
    {{- $_ := set .values.core.resources.requests "cpu" .override.resources.requests.cpu -}}
  {{- end -}}
  {{- if eq .values.zeebe.resources.requests.memory "1200Mi" -}}
    {{- $_ := set .values.core.resources.requests "memory" .override.resources.requests.memory -}}
  {{- end -}}
  {{- if eq .values.zeebe.resources.limits.cpu "960m" -}}
    {{- $_ := set .values.core.resources.limits "cpu" .override.resources.limits.cpu -}}
  {{- end -}}
  {{- if eq .values.zeebe.resources.limits.memory "1920Mi" -}}
    {{- $_ := set .values.core.resources.limits "memory" .override.resources.limits.memory -}}
  {{- end -}}
{{- end -}}

{{/*
Zeebe to Core main.
*/}}
{{- if and .Values.zeebe .Values.zeebe.enabled -}}
    {{/* Deep copy core as tmp var to set some keys later after the merge with zeebe key. */}}
    {{- $coreOrig := deepCopy .Values.core -}}
    {{/* Deep copy zeebe as core */}}
    {{- $_ := set .Values "core" (deepCopy .Values.zeebe | mergeOverwrite .Values.core) -}}
    {{/*
        Override keys with different values
    */}}
    {{/* zeebe.retention => core.history.retention */}}
    {{- $_ := set .Values.core.history "retention" .Values.zeebe.retention -}}
    {{/* zeebe.resources => core.resources */}}
    {{- include "compatibility.coreResources" (dict "values" $.Values "override" $coreOrig) -}}
{{- end -}}

{{/*
ZeebeGateway => Core.
*/}}
{{- if and .Values.zeebeGateway .Values.zeebeGateway.enabled -}}
    {{- $_ := set .Values.core "ingress" .Values.zeebeGateway.ingress -}}
    {{- $_ := set .Values.core "contextPath" .Values.zeebeGateway.contextPath -}}
    {{- $_ := set .Values.core "httpPort" .Values.zeebeGateway.service.restPort -}}
    {{- $_ := set .Values.core "grpcPort" .Values.zeebeGateway.service.grpcPort -}}
    {{- $_ := set .Values.core "commandPort" .Values.zeebeGateway.service.commandPort -}}
    {{- $_ := set .Values.core "internalPort" .Values.zeebeGateway.service.internalPort -}}
{{- end -}}

{{/*
Operate => Core.
*/}}
{{- if .Values.operate -}}
    {{- $_ := set .Values.core.profiles "operate" .Values.operate.enabled -}}
{{- end -}}

{{/*
Tasklist => Core.
*/}}
{{- if .Values.tasklist -}}
    {{- $_ := set .Values.core.profiles "tasklist" .Values.tasklist.enabled -}}
{{- end -}}

{{/*
********************************************************************************
* Camunda 8.5 backward compatibility.

Overview:
    Backward compatibility with values syntax before Camunda Helm chart v10.0.0 (Camunda 8.5 cycle).

Approach:
    Using deep copy and deep merge functions to override new keys using the old key.
    https://helm.sh/docs/chart_template_guide/function_list/#mergeoverwrite-mustmergeoverwrite
********************************************************************************
*/}}

{{/*
OpenShift.
The `elasticsearch.sysctlImage` container adjusts the virtual memory and file descriptors of the machine needed for Elasticsearch.
By default, the `sysctlImage` container will fail on OpenShift because it requires privileged mode.
Also, recent OpenShift versions (> 4.10) have adjusted the virtual memory of the machine by default.
*/}}
{{- if eq .Values.global.compatibility.openshift.adaptSecurityContext "force" -}}
    {{- $_ := set .Values.elasticsearch.sysctlImage "enabled" false -}}
{{- end -}}


{{/*
OpenShift.
The label `tuned.openshift.io/elasticsearch` is added to ensure compatibility with the previous Camunda Helm charts.
Without this label, the Helm upgrade will fail for OpenShift because it is already set for the volumeClaimTemplate.
*/}}

{{- if eq .Values.global.compatibility.openshift.adaptSecurityContext "force" -}}
    {{- if not (hasKey .Values.elasticsearch.commonLabels "tuned.openshift.io/elasticsearch") -}}
        {{- $_ := set .Values.elasticsearch.commonLabels "tuned.openshift.io/elasticsearch" "" -}}
    {{- end -}}
{{- end -}}
