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
* Camunda 8.7 => 8.8

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
Core compatibility.
*/}}
{{- if .Values.global.compatibility.core.enabled -}}
    {{/*
    Zeebe => Core.
    Deep copy "zeebe" key as "core" key, then set/override the new changes.
    */}}
    {{- if and .Values.zeebe .Values.zeebe.enabled -}}
        {{/* Deep copy "core" as tmp var to set some keys later after the merge with "zeebe" key. */}}
        {{- $coreOrig := deepCopy .Values.core -}}
        {{/* Deep copy and merge "zeebe" with "core" */}}
        {{- $_ := set .Values "core" (deepCopy .Values.zeebe | mergeOverwrite .Values.core) -}}

        {{/*
            Override keys with different values.
        */}}

        {{/*
            zeebe.retention => core.history.retention
            # TODO: Update the retention values after review the correct path with the dev team.
            {{- if (.Values.zeebe.retention).enabled -}}
                {{- $_ := set .Values.core.history.retention "enabled" .Values.zeebe.retention.enabled -}}
                {{- if ((.Values.zeebe.retention).minimumAge) -}}
                    {{- $_ := set .Values.core.history.retention "minimumAge" .Values.zeebe.retention.minimumAge -}}
                {{- end -}}
                {{- if ((.Values.zeebe.retention).policyName) -}}
                    {{- $_ := set .Values.core.history.retention "policyName" .Values.zeebe.retention.policyName -}}
                {{- end -}}
            {{- end -}}
        */}}


        {{/*
            zeebe.resources => core.resources
            Set the default values for the core resources if they use the Zeebe default values,
            as the defaults have been changed in Camunda Helm chart v13.0.0 (Camunda 8.8).
            If the user has set custom values for the core resources, they will not be overridden.
        */}}
        {{- if eq (((.Values.zeebe).resources).requests).cpu "800m" -}}
            {{- $_ := set .Values.core.resources.requests "cpu" $coreOrig.resources.requests.cpu -}}
        {{- end -}}
        {{- if eq (((.Values.zeebe).resources).requests).memory "1200Mi" -}}
            {{- $_ := set .Values.core.resources.requests "memory" $coreOrig.resources.requests.memory -}}
        {{- end -}}
        {{- if eq (((.Values.zeebe).resources).limits).cpu "960m" -}}
            {{- $_ := set .Values.core.resources.limits "cpu" $coreOrig.resources.limits.cpu -}}
        {{- end -}}
        {{- if eq (((.Values.zeebe).resources).limits).memory "1920Mi" -}}
            {{- $_ := set .Values.core.resources.limits "memory" $coreOrig.resources.limits.memory -}}
        {{- end -}}

        {{/*
            zeebe.javaOpts => core.javaOpts
            NOTE: The key is already overwritten by Zeebe values, so we need to replace the value.
        */}}
        {{- $_ := set .Values.core "javaOpts" (.Values.core.javaOpts | replace "zeebe" "camunda") -}}
    {{- end -}}

    {{- if and .Values.zeebe -}}
        {{/*
            zeebe.enabled => core.profiles.broker
        */}}
        {{- $_ := set .Values.core.profiles "broker" .Values.zeebe.enabled -}}
    {{- end -}}

    {{/*
    Zeebe Gateway => Core.
    */}}
    {{- if and .Values.zeebeGateway .Values.zeebeGateway.enabled -}}
        {{- if ((.Values.zeebeGateway).ingress) -}}
            {{- $_ := set .Values.core "ingress" .Values.zeebeGateway.ingress -}}
        {{- end -}}
        {{- if ((.Values.zeebeGateway).contextPath) -}}
            {{- $_ := set .Values.core "contextPath" .Values.zeebeGateway.contextPath -}}
        {{- end -}}
        {{- if ((.Values.zeebeGateway).service) -}}
            {{- if ((.Values.zeebeGateway.service).restPort) -}}
                {{- $_ := set .Values.core.service "httpPort" .Values.zeebeGateway.service.restPort -}}
            {{- end -}}
            {{- if ((.Values.zeebeGateway.service).grpcPort) -}}
                {{- $_ := set .Values.core.service "grpcPort" .Values.zeebeGateway.service.grpcPort -}}
            {{- end -}}
            {{- if ((.Values.zeebeGateway.service).commandPort) -}}
                {{- $_ := set .Values.core.service "commandPort" .Values.zeebeGateway.service.commandPort -}}
            {{- end -}}
            {{- if ((.Values.zeebeGateway.service).internalPort) -}}
                {{- $_ := set .Values.core.service "internalPort" .Values.zeebeGateway.service.internalPort -}}
            {{- end -}}
        {{- end -}}
    {{- end -}}

    {{/*
    Operate => Core.
    */}}
    {{- if and .Values.operate .Values.operate.enabled -}}
        {{- $_ := set .Values.core.profiles "operate" .Values.operate.enabled -}}
    {{- end -}}

    {{/*
    Tasklist => Core.
    */}}
    {{- if and .Values.tasklist .Values.tasklist.enabled -}}
        {{- $_ := set .Values.core.profiles "tasklist" .Values.tasklist.enabled -}}
    {{- end -}}
{{- end -}}

{{/*
Core constraints.
Free-style inputs should be migrated manually by the user.
*/}}

{{/*
camundaPlatform.manualMigrationRequired
Fail with message when the old values file key is used and show the new key.
Usage:
{{ include "camundaPlatform.manualMigrationRequired" (dict
  "condition" (.Values.zeebe.configuration)
  "oldName" "zeebe.configuration"
  "newName" "core.configuration"
) }}
*/}}
{{- define "camundaPlatform.manualMigrationRequired" }}
  {{- if .condition }}
    {{- $errorMessage := printf
        "[core][compatibility][error] Please migrate the value of \"%s\" to the new syntax under \"%s\" %s %s"
        .oldName .newName
        "For more details, please check Camunda Helm chart documentation."
        "https://docs.camunda.io/docs/next/self-managed/installation-methods/helm/upgrade/upgrade-hc-870-880/"
    -}}
    {{ printf "\n%s" $errorMessage | trimSuffix "\n"| fail }}
  {{- end }}
{{- end -}}

{{- if .Values.global.compatibility.core.enabled -}}
    {{/*
    Zeebe => Core.
    */}}
    {{- if and .Values.zeebe .Values.zeebe.enabled -}}
        {{/*
        zeebe.configuration => core.configuration
        */}}
        {{ include "camundaPlatform.manualMigrationRequired" (dict
            "condition" (.Values.zeebe.configuration)
            "oldName" "zeebe.configuration"
            "newName" "core.configuration"
        ) }}
        {{/*
        zeebe.extraConfiguration => core.extraConfiguration
        */}}
        {{ include "camundaPlatform.manualMigrationRequired" (dict
            "condition" (.Values.zeebe.extraConfiguration)
            "oldName" "zeebe.extraConfiguration"
            "newName" "core.extraConfiguration"
        ) }}
        {{/*
        zeebe.env => core.env
        */}}
        {{ include "camundaPlatform.manualMigrationRequired" (dict
            "condition" (.Values.zeebe.env)
            "oldName" "zeebe.env"
            "newName" "core.env"
        ) }}
        {{/*
        zeebe.envFrom => core.envFrom
        */}}
        {{ include "camundaPlatform.manualMigrationRequired" (dict
            "condition" (.Values.zeebe.envFrom)
            "oldName" "zeebe.envFrom"
            "newName" "core.envFrom"
        ) }}
        {{/*
        zeebe.initContainers => core.initContainers
        */}}
        {{ include "camundaPlatform.manualMigrationRequired" (dict
            "condition" (.Values.zeebe.initContainers)
            "oldName" "zeebe.initContainers"
            "newName" "core.initContainers"
        ) }}
        {{/*
        zeebe.sidecars => core.sidecars
        */}}
        {{ include "camundaPlatform.manualMigrationRequired" (dict
            "condition" (.Values.zeebe.sidecars)
            "oldName" "zeebe.sidecars"
            "newName" "core.sidecars"
        ) }}
        {{/*
        zeebe.extraVolumes => core.extraVolumes
        */}}
        {{ include "camundaPlatform.manualMigrationRequired" (dict
            "condition" (.Values.zeebe.extraVolumes)
            "oldName" "zeebe.extraVolumes"
            "newName" "core.extraVolumes"
        ) }}
        {{/*
        zeebe.extraVolumeMounts => core.extraVolumeMounts
        */}}
        {{ include "camundaPlatform.manualMigrationRequired" (dict
            "condition" (.Values.zeebe.extraVolumeMounts)
            "oldName" "zeebe.extraVolumeMounts"
            "newName" "core.extraVolumeMounts"
        ) }}
    {{- end -}}

    {{/*
    Zeebe Gateway => Core.
    */}}
    {{- if and .Values.zeebeGateway .Values.zeebeGateway.enabled -}}
        {{/*
        zeebeGateway.configuration => core.configuration
        */}}
        {{ include "camundaPlatform.manualMigrationRequired" (dict
            "condition" (.Values.zeebeGateway.configuration)
            "oldName" "zeebeGateway.configuration"
            "newName" "core.configuration"
        ) }}
        {{/*
        zeebeGateway.extraConfiguration => core.extraConfiguration
        */}}
        {{ include "camundaPlatform.manualMigrationRequired" (dict
            "condition" (.Values.zeebeGateway.extraConfiguration)
            "oldName" "zeebeGateway.extraConfiguration"
            "newName" "core.extraConfiguration"
        ) }}
        {{/*
        zeebeGateway.env => core.env
        */}}
        {{ include "camundaPlatform.manualMigrationRequired" (dict
            "condition" (.Values.zeebeGateway.env)
            "oldName" "zeebeGateway.env"
            "newName" "core.env"
        ) }}
        {{/*
        zeebeGateway.envFrom => core.envFrom
        */}}
        {{ include "camundaPlatform.manualMigrationRequired" (dict
            "condition" (.Values.zeebeGateway.envFrom)
            "oldName" "zeebeGateway.envFrom"
            "newName" "core.envFrom"
        ) }}
        {{/*
        zeebeGateway.initContainers => core.initContainers
        */}}
        {{ include "camundaPlatform.manualMigrationRequired" (dict
            "condition" (.Values.zeebeGateway.initContainers)
            "oldName" "zeebeGateway.initContainers"
            "newName" "core.initContainers"
        ) }}
        {{/*
        zeebeGateway.sidecars => core.sidecars
        */}}
        {{ include "camundaPlatform.manualMigrationRequired" (dict
            "condition" (.Values.zeebeGateway.sidecars)
            "oldName" "zeebeGateway.sidecars"
            "newName" "core.sidecars"
        ) }}
        {{/*
        zeebeGateway.extraVolumes => core.extraVolumes
        */}}
        {{ include "camundaPlatform.manualMigrationRequired" (dict
            "condition" (.Values.zeebeGateway.extraVolumes)
            "oldName" "zeebeGateway.extraVolumes"
            "newName" "core.extraVolumes"
        ) }}
        {{/*
        zeebeGateway.extraVolumeMounts => core.extraVolumeMounts
        */}}
        {{ include "camundaPlatform.manualMigrationRequired" (dict
            "condition" (.Values.zeebeGateway.extraVolumeMounts)
            "oldName" "zeebeGateway.extraVolumeMounts"
            "newName" "core.extraVolumeMounts"
        ) }}
    {{- end -}}

    {{/*
    Operate => Core.
    */}}
    {{- if and .Values.operate .Values.operate.enabled -}}
        {{/*
        operate.configuration => core.configuration
        */}}
        {{ include "camundaPlatform.manualMigrationRequired" (dict
            "condition" (.Values.operate.configuration)
            "oldName" "operate.configuration"
            "newName" "core.configuration"
        ) }}
        {{/*
        operate.extraConfiguration => core.extraConfiguration
        */}}
        {{ include "camundaPlatform.manualMigrationRequired" (dict
            "condition" (.Values.operate.extraConfiguration)
            "oldName" "operate.extraConfiguration"
            "newName" "core.extraConfiguration"
        ) }}
        {{/*
        operate.env => core.env
        */}}
        {{ include "camundaPlatform.manualMigrationRequired" (dict
            "condition" (.Values.operate.env)
            "oldName" "operate.env"
            "newName" "core.env"
        ) }}
        {{/*
        operate.envFrom => core.envFrom
        */}}
        {{ include "camundaPlatform.manualMigrationRequired" (dict
            "condition" (.Values.operate.envFrom)
            "oldName" "operate.envFrom"
            "newName" "core.envFrom"
        ) }}
        {{/*
        operate.initContainers => core.initContainers
        */}}
        {{ include "camundaPlatform.manualMigrationRequired" (dict
            "condition" (.Values.operate.initContainers)
            "oldName" "operate.initContainers"
            "newName" "core.initContainers"
        ) }}
        {{/*
        operate.sidecars => core.sidecars
        */}}
        {{ include "camundaPlatform.manualMigrationRequired" (dict
            "condition" (.Values.operate.sidecars)
            "oldName" "operate.sidecars"
            "newName" "core.sidecars"
        ) }}
        {{/*
        operate.extraVolumes => core.extraVolumes
        */}}
        {{ include "camundaPlatform.manualMigrationRequired" (dict
            "condition" (.Values.operate.extraVolumes)
            "oldName" "operate.extraVolumes"
            "newName" "core.extraVolumes"
        ) }}
        {{/*
        operate.extraVolumeMounts => core.extraVolumeMounts
        */}}
        {{ include "camundaPlatform.manualMigrationRequired" (dict
            "condition" (.Values.operate.extraVolumeMounts)
            "oldName" "operate.extraVolumeMounts"
            "newName" "core.extraVolumeMounts"
        ) }}
    {{- end -}}

    {{/*
    Tasklist => Core.
    */}}
    {{- if and .Values.tasklist .Values.tasklist.enabled -}}
        {{/*
        tasklist.configuration => core.configuration
        */}}
        {{ include "camundaPlatform.manualMigrationRequired" (dict
            "condition" (.Values.tasklist.configuration)
            "oldName" "tasklist.configuration"
            "newName" "core.configuration"
        ) }}
        {{/*
        tasklist.extraConfiguration => core.extraConfiguration
        */}}
        {{ include "camundaPlatform.manualMigrationRequired" (dict
            "condition" (.Values.tasklist.extraConfiguration)
            "oldName" "tasklist.extraConfiguration"
            "newName" "core.extraConfiguration"
        ) }}
        {{/*
        tasklist.env => core.env
        */}}
        {{ include "camundaPlatform.manualMigrationRequired" (dict
            "condition" (.Values.tasklist.env)
            "oldName" "tasklist.env"
            "newName" "core.env"
        ) }}
        {{/*
        tasklist.envFrom => core.envFrom
        */}}
        {{ include "camundaPlatform.manualMigrationRequired" (dict
            "condition" (.Values.tasklist.envFrom)
            "oldName" "tasklist.envFrom"
            "newName" "core.envFrom"
        ) }}
        {{/*
        tasklist.initContainers => core.initContainers
        */}}
        {{ include "camundaPlatform.manualMigrationRequired" (dict
            "condition" (.Values.tasklist.initContainers)
            "oldName" "tasklist.initContainers"
            "newName" "core.initContainers"
        ) }}
        {{/*
        tasklist.sidecars => core.sidecars
        */}}
        {{ include "camundaPlatform.manualMigrationRequired" (dict
            "condition" (.Values.tasklist.sidecars)
            "oldName" "tasklist.sidecars"
            "newName" "core.sidecars"
        ) }}
        {{/*
        tasklist.extraVolumes => core.extraVolumes
        */}}
        {{ include "camundaPlatform.manualMigrationRequired" (dict
            "condition" (.Values.tasklist.extraVolumes)
            "oldName" "tasklist.extraVolumes"
            "newName" "core.extraVolumes"
        ) }}
        {{/*
        tasklist.extraVolumeMounts => core.extraVolumeMounts
        */}}
        {{ include "camundaPlatform.manualMigrationRequired" (dict
            "condition" (.Values.tasklist.extraVolumeMounts)
            "oldName" "tasklist.extraVolumeMounts"
            "newName" "core.extraVolumeMounts"
        ) }}
    {{- end -}}
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
