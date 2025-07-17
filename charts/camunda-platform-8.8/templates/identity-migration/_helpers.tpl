{{/* vim: set filetype=mustache: */}}

{{/*
Define common labels for identity migration app, combining the match labels and transient labels, which might change on updating
(version depending). These labels shouldn't be used on matchLabels selector, since the selectors are immutable.
*/}}
{{- define "identityMigration.labels" -}}
{{- template "camundaPlatform.labels" . }}
{{- end -}}

