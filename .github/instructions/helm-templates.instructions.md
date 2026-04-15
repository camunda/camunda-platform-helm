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

### Chart Design Principles (from `docs/index.md`)

Every template change must respect these principles:

- **Minimal & common** — expose configuration that is common, minimal, and useful. Every new
  field or behaviour must result from a conscious, user-driven decision.
- **1:1 mapping** — maintain a 1:1 mapping between application configuration and Helm values
  wherever possible. Do not invent extra abstraction layers.
- **Generic extensibility** — provide generic, composable mechanisms (`extraConfiguration`,
  `extraEnv`, `extraVolumes`) rather than embedding opinionated solutions for monitoring,
  security, or identity management.
- **Reasonable defaults** — establish sensible defaults that cover common deployment scenarios
  without exposing unnecessary complexity.

### What the Chart is NOT

Do not introduce templates that:

- Expose arbitrary or exhaustive Camunda application configuration.
- Implement opinionated solutions from individual engineers (e.g., bundled monitoring stacks,
  hard-coded security policies).
- Bundle or depend on external components not part of Camunda's core product.
- Abstract away or bypass the intrinsic complexity of Camunda's architecture.
- Patch or work around application-level issues or technical debt.
- Override core application defaults unless the change aligns with Camunda's default behaviour.

---

## Critical Rules

### NEVER
- **NEVER** use unguarded `{{ }}` (without `{{-`/`-}}`) where it would emit spurious blank lines.
- **NEVER** hardcode component names; use `include "<component>.fullname" .` helpers.
- **NEVER** skip the top-level `{{- if .Values.<component>.enabled -}}` guard on every resource file.
- **NEVER** change a helper name that is already exported without updating all callers across all chart versions.
- **NEVER** use `range` over `extraConfiguration` without the `kindIs "slice"` branch — it supports both map and slice forms.
- **NEVER** inline multi-line YAML strings without `indent N | trim` to prevent YAML parse errors.

### ALWAYS
- **ALWAYS** use `{{-` / `-}}` to strip whitespace around block-level directives.
- **ALWAYS** use `nindent N` (not `indent N`) when piping `include` results into YAML blocks.
- **ALWAYS** place `{{- include "<component>.labels" . | nindent 4 }}` under every `metadata.labels`.
- **ALWAYS** add a `checksum/config` annotation when a resource depends on a ConfigMap.
- **ALWAYS** use `tpl` when rendering user-supplied label/annotation maps to support template expressions.
- **ALWAYS** use `camundaPlatform.imageByParams` to resolve images — never concatenate `image:tag` manually.
- **ALWAYS** keep 2-space YAML indentation throughout.

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

---

## Common Mistakes

1. **Missing `-` on block directives** — `{{- if ... }}` without the leading `-` emits an empty line before the block,
   producing invalid YAML in certain contexts.

2. **`indent` without `trim` on multi-line strings** — always pair: `{{ .Values.foo.configuration | indent 4 | trim }}`.
   Without `trim`, a leading newline causes YAML parse errors.

3. **`include` without `nindent`** — `{{- include "foo.labels" . }}` without `| nindent N` produces un-indented output
   that breaks YAML structure.

4. **Hardcoding component names** — writing `name: zeebe` instead of `name: {{ include "orchestration.fullname" . }}`
   breaks `nameOverride` / `fullnameOverride` support.

5. **Skipping backward-compat comments** — when a helper uses a legacy name (e.g., `"zeebe"` for orchestration),
   always add a `{{- /* NOTE: ... */ -}}` comment explaining why.

6. **Checking only map form of `extraConfiguration`** — callers that do `range $k, $v := .extraConfig` without first
   checking `kindIs "slice"` will panic when the user passes a list.

7. **Using `$.Template.BasePath` outside `include`** — `print $.Template.BasePath "/foo/bar.yaml"` only works inside
   `include`; don't use it to construct file paths for other purposes.

8. **Missing `enabled` guard** — resources emitted when a component is disabled cause Helm install failures.

---

## Resources

- Helm best practices: <https://helm.sh/docs/chart_best_practices/>
- Chart design principles: `docs/index.md`
- `common/_helpers.tpl`: `charts/<version>/templates/common/_helpers.tpl`
- `common/_utilz.tpl`: `charts/<version>/templates/common/_utilz.tpl`
- Version differences overview: `AGENTS.md` (Version-Aware Rules section)
- Render templates locally: `make helm.template chartPath=charts/camunda-platform-8.10`
- Lint: `make helm.lint chartPath=charts/camunda-platform-8.10`
