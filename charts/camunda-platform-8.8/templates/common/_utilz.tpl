{{/* vim: set filetype=mustache: */}}

{{/*
********************************************************************************
Utilities.
********************************************************************************
*/}}

{{/*
camundaPlatform.toYamlPretty
A version-safe wrapper for toYamlPretty that falls back to toYaml on older Helm versions.
toYamlPretty was introduced in Helm 3.17.0, so older versions will fail if it's used directly.

This uses tpl to dynamically evaluate the template string at runtime, avoiding parse-time
errors on older Helm versions that don't recognize toYamlPretty as a function.

Usage:
{{ include "camundaPlatform.toYamlPretty" (dict "value" . "context" $) }}

Parameters:
- value: The value to convert to YAML
- context: The root context ($) needed for accessing Capabilities
*/}}
{{- define "camundaPlatform.toYamlPretty" -}}
  {{- if semverCompare ">=3.17.0" .context.Capabilities.HelmVersion.Version -}}
    {{- tpl "{{ toYamlPretty .value }}" (dict "value" .value) -}}
  {{- else -}}
    {{- toYaml .value -}}
  {{- end -}}
{{- end -}}

{{/*
camundaPlatform.manualMigrationRequired
Fail with message when the old values file key is used and show the new key.

Usage:
{{ include "camundaPlatform.manualMigrationRequired" (dict
  "condition" (.Values.zeebe.configuration)
  "oldName" "zeebe.configuration"
  "newName" "orchestration.configuration"
) }}
*/}}
{{- define "camundaPlatform.manualMigrationRequired" }}
  {{- if .condition }}
    {{- $errorMessage := printf
        "[orchestration][compatibility][error] Please migrate the value of \"%s\" to the new syntax under \"%s\" %s %s"
        .oldName .newName
        "For more details, please check Camunda Helm chart documentation."
        "https://docs.camunda.io/docs/self-managed/deployment/helm/upgrade/upgrade-hc-870-880/"
    -}}
    {{ printf "\n%s" $errorMessage | trimSuffix "\n"| fail }}
  {{- end }}
{{- end -}}

{{/*
camundaPlatform.failWithMessageOnCondition
Fail with message when the condition is met.

Usage:
{{ include "camundaPlatform.failWithMessageOnCondition" (dict
  "condition" (eq .Values.foo .Values.bar)
  "errorType" "[WARNING]"
  "errorMessage" "Foo is deprecated, use Bar instead"
) }}
*/}}
{{- define "camundaPlatform.failWithMessageOnCondition" }}
  {{- if .condition }}
    {{- $errorType := .errorType | default "[camunda-platform][warning]" -}}
    {{ printf "\n%s %s" $errorType .errorMessage | trimSuffix "\n"| fail }}
  {{- end }}
{{- end -}}

{{/*
[camunda-platform] Joins an arbirtary number of subpaths (e.g., contextPath+probePath) for HTTP paths.
Slashes are trimmed from the beginning and end of each part, and a single slash is inserted between parts,
leading slash added at the beginning.

Usage:
{{ include "camundaPlatform.joinpath" (list
  .Values.orchestration.contextPath
  .Values.orchestration.readinessProbe.probePath
) }}
*/}}
{{- define "camundaPlatform.joinpath" -}}
  {{- $paths := join "/" . -}}
  {{- $pathsSanitized := regexReplaceAll "/+" $paths "/" | trimAll "/" }}
  {{- printf "/%s" $pathsSanitized -}}
{{- end -}}
