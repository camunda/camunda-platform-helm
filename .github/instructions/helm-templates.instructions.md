---
applyTo: "charts/**/templates/**"
---

# Helm Templates — Scoped Instructions

## Overview

This repository contains Helm charts for Camunda 8 Self-Managed. Template files live under
`charts/<version>/templates/<component>/`. Charts from 8.8 onward use a unified
`templates/orchestration/` directory; 8.7 and older use separate per-component directories
(`zeebe/`, `operate/`, `tasklist/`, etc.). All templates follow the same whitespace, naming,
and helper conventions described here. Always identify the target chart version before editing
templates — never assume paths or component names are identical across versions. Common helpers
live in `templates/common/_helpers.tpl` and `templates/common/_utilz.tpl`; component-specific
helpers live in `templates/<component>/_helpers.tpl`.

### Chart Design Principles

Every template change must respect the chart design principles and "What the Chart is NOT"
boundaries in `docs/index.md` (canonical): minimal & common, user-driven, generic extensibility
(`extraConfiguration`/`extraEnv`/`extraVolumes` over opinionated integrations), 1:1 mapping to
application config, reasonable defaults; no external-component bundling, no workarounds for
application-level issues.

---

## Critical Rules

### NEVER
- **NEVER** use unguarded `{{ }}` (without `{{-`/`-}}`) where it would emit spurious blank lines.
- **NEVER** hardcode component names; use `include "<component>.fullname" .` helpers.
- **NEVER** skip the top-level `{{- if .Values.<component>.enabled -}}` guard on every resource file.
- **NEVER** change a helper name that is already exported without updating all callers across all chart versions.
- **NEVER** use `range` over `extraConfiguration` without the `kindIs "slice"` branch — it supports both map and slice forms.
- **NEVER** inline multi-line YAML strings without `indent N | trim` to prevent YAML parse errors.
- **NEVER** write a render-time warning directly into `NOTES.txt`. Declare it inside the `camunda.constraints.warnings` define in the constraints template (`templates/common/constraints.tpl` on 8.8+; `templates/camunda/constraints.tpl` on 8.7 and older); `NOTES.txt` surfaces warnings only via `{{ include "camunda.constraints.warnings" . | trim }}`.

### ALWAYS
- **ALWAYS** use `{{-` / `-}}` to strip whitespace around block-level directives.
- **ALWAYS** use `nindent N` (not `indent N`) when piping `include` results into YAML blocks.
- **ALWAYS** place `{{- include "<component>.labels" . | nindent 4 }}` under every `metadata.labels`.
- **ALWAYS** add a `checksum/config` annotation when a resource depends on a ConfigMap.
- **ALWAYS** use `tpl` when rendering user-supplied label/annotation maps to support template expressions.
- **ALWAYS** use `camundaPlatform.imageByParams` to resolve images — never concatenate `image:tag` manually.
- **ALWAYS** keep 2-space YAML indentation throughout.
- **ALWAYS** raise hard validation failures at the top level of the constraints template and abort with `fail` (piped `... | fail` or a direct `fail (...)` call); raise non-fatal warnings inside the `camunda.constraints.warnings` define (no `fail`). Prefer the `[camunda][error]` / `[camunda][warning]` message prefixes — they are the house convention used by nearly all existing entries.

---

## Core Patterns with Code Examples

### 1. Resource Guard and Labels

Every resource file must start and end with an enabled guard:

```yaml
{{- if .Values.connectors.enabled -}}
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ include "connectors.fullname" . }}
  labels:
    {{- include "connectors.labels" . | nindent 4 }}
  annotations:
    {{- range $key, $value := .Values.global.annotations }}
    {{ $key }}: {{ $value | quote }}
    {{- end }}
{{- end }}
```

### 2. Pod Labels and Annotations (with tpl)

```yaml
metadata:
  labels:
    {{- include "orchestration.labels" . | nindent 8 }}
    {{- if .Values.orchestration.podLabels }}
    {{- tpl (toYaml .Values.orchestration.podLabels) $ | nindent 8 }}
    {{- end }}
  annotations:
    checksum/config: {{ include (print $.Template.BasePath "/orchestration/configmap.yaml") . | sha256sum }}
    {{- if .Values.orchestration.podAnnotations }}
    {{- tpl (toYaml .Values.orchestration.podAnnotations) $ | nindent 8 }}
    {{- end }}
```

### 3. Helper Naming Convention

Global helpers in `templates/common/_helpers.tpl`:

```
camundaPlatform.<functionName>   e.g. camundaPlatform.fullname
camundaPlatform.imageByParams
camundaPlatform.renderExtraConfiguration
camundaPlatform.extraConfigurationVolumeMounts
```

Component helpers in `templates/<component>/_helpers.tpl`:

```
<component>.<functionName>       e.g. orchestration.fullname, connectors.labels
```

### 4. Fullname Helper Pattern

```yaml
{{- define "orchestration.fullname" -}}
    {{- /* NOTE: Set to "zeebe" for backward compatibility between 8.7 and 8.8. */ -}}
    {{- include "camundaPlatform.componentFullname" (dict
        "componentName" "zeebe"
        "componentValues" .Values.orchestration
        "context" $
    ) -}}
{{- end -}}
```

### 5. extraConfiguration Pattern

The `extraConfiguration` field supports **both** a map (`{}`) and a list (`[]`) form.
Always use the shared helper:

```yaml
# In configmap.yaml
data:
  {{- if .Values.connectors.configuration }}
  application.yaml: |
    {{ .Values.connectors.configuration | indent 4 | trim }}
  {{- else }}
  application.yaml: |
    {{- (include (print $.Template.BasePath "/connectors/files/_application.yaml") $) | indent 4 }}
  {{- end }}
  {{- include "camundaPlatform.renderExtraConfiguration" (dict "extraConfig" .Values.connectors.extraConfiguration) }}
```

The `camundaPlatform.renderExtraConfiguration` helper handles both forms:

```yaml
{{- define "camundaPlatform.renderExtraConfiguration" -}}
  {{- if kindIs "slice" .extraConfig }}
  {{- range .extraConfig }}
  {{ .file }}: |
    {{ .content | indent 4 | trim }}
  {{- end }}
  {{- else }}
  {{- range $key, $val := .extraConfig }}
  {{ $key }}: |
    {{ $val | indent 4 | trim }}
  {{- end }}
  {{- end }}
{{- end -}}
```

### 6. Image Resolution

```yaml
image: {{ include "camundaPlatform.imageByParams" (dict "base" .Values.global "overlay" .Values.orchestration) }}
imagePullPolicy: {{ .Values.global.image.pullPolicy }}
```

### 7. Container Security Context

```yaml
{{- if .Values.orchestration.containerSecurityContext }}
securityContext:
  {{- include "common.compatibility.renderSecurityContext" (dict "secContext" $.Values.orchestration.containerSecurityContext "context" $) | nindent 12 }}
{{- end }}
```

### 8. Init Containers and Extra Containers

```yaml
initContainers:
  {{- tpl ((coalesce .Values.orchestration.initContainers .Values.orchestration.extraInitContainers) | default list | toYaml | nindent 8) $ }}
```

### 9. ExtraConfiguration Volume Mounts

```yaml
volumeMounts:
  {{- include "camundaPlatform.extraConfigurationVolumeMounts" (dict
      "extraConfig" .Values.connectors.extraConfiguration
      "volumeName" "connector-configuration"
      "basePath" "/usr/local/etc/connectors"
  ) | nindent 10 }}
```

### 10. Render-Time Constraints and Warnings

All render-time validation lives in the constraints template — `templates/common/constraints.tpl`
on 8.8+, `templates/camunda/constraints.tpl` on 8.7 and older — in two distinct shapes.
`NOTES.txt` does NOT author warnings — it only includes them: `{{ include "camunda.constraints.warnings" . | trim }}`.

**Hard failure** — top-level (outside any `define`), aborts the render with `fail` (the piped form
below, or an equivalent direct `fail (printf "[camunda][error] ...")` call):

```yaml
{{- if and .Values.foo.enabled (not .Values.foo.secret.existingSecret) }}
  {{- $errorMessage := printf "[camunda][error] %s"
      "foo.enabled requires foo.secret.existingSecret to be set."
  -}}
  {{ printf "\n%s" $errorMessage | trimSuffix "\n" | fail }}
{{- end }}
```

**Soft warning** — inside the `camunda.constraints.warnings` define, returns a string, **no `fail`**:

```yaml
{{- define "camunda.constraints.warnings" }}
  {{- if .Values.console.enabled }}
    {{- $warningMessage := printf "%s %s"
        "[camunda][warning]"
        "DEPRECATION: \"console.enabled\" is deprecated; use \"camundaHub.enabled: true\" instead."
    -}}
    {{ printf "\n%s" $warningMessage | trimSuffix "\n" }}
  {{- end }}
{{- end }}
```

For deprecated values keys, prefer the `camundaPlatform.keyDeprecated` helper (non-fatal) over
hand-rolling the warning; it must be called from within `camunda.constraints.warnings`.

---

## Common Mistakes

For anything restating a Critical Rule above (whitespace trimming, `nindent`, `enabled` guards,
hardcoded names, `kindIs "slice"`, NOTES.txt warnings), see Critical Rules — the failure modes
are: spurious blank lines / un-indented output breaking YAML, install failures for disabled
components, template panics on list-form `extraConfiguration`, and leading-newline YAML parse
errors from `indent` without `trim`. Additional pitfalls:

1. **Skipping backward-compat comments** — when a helper uses a legacy name (e.g., `"zeebe"` for orchestration),
   always add a `{{- /* NOTE: ... */ -}}` comment explaining why.

2. **Using `$.Template.BasePath` outside `include`** — `print $.Template.BasePath "/foo/bar.yaml"` only works inside
   `include`; don't use it to construct file paths for other purposes.

---

## Resources

- Helm best practices: <https://helm.sh/docs/chart_best_practices/>
- Chart design principles: `docs/index.md`
- `common/_helpers.tpl`: `charts/<version>/templates/common/_helpers.tpl`
- `common/_utilz.tpl`: `charts/<version>/templates/common/_utilz.tpl`
- Constraints & warnings: `charts/<version>/templates/common/constraints.tpl` (8.8+) or `charts/<version>/templates/camunda/constraints.tpl` (8.7 and older)
- Version differences overview: `AGENTS.md` (Version-Aware Rules section)
- Render templates locally: `make helm.template chartPath=charts/camunda-platform-8.10`
- Lint: `make helm.lint chartPath=charts/camunda-platform-8.10`
