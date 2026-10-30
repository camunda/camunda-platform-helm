# Complexity checks

The default command checks changed Go functions and workflow run blocks.
Helm modes inspect the source of named templates and unnamed/root bodies.

```sh
make helm.template-complexity-report chartPath=charts/camunda-platform-8.11
make helm.template-complexity templateCheckArgs=-help
make helm.lint chartPath=charts/camunda-platform-8.11
```

The executable owns the metric definitions and budgets. Its help and failure
diagnostics expose those values; agent instructions do not duplicate them.
Numeric compliance does not replace review of cohesion, duplication or behavior.

Helm checks compare parsed bodies against the merge base of `lintBase` and `HEAD`.
`HELM_TEMPLATE_BASE_REF` supplies an exact CI base. Edited bodies must meet both
budgets even if their counts stay the same or decrease. Untouched legacy bodies
remain exempt. Comments and expression formatting that preserve the parsed body
do not make it eligible.

Named helpers retain identity when moved within a chart. Renamed helpers, new root
paths and new chart versions are new units.

Chart paths are relative to the repository root, or absolute paths inside it.
Explicit unsupported chart paths fail rather than producing a partial report.
Bundled dependency templates are excluded. The report needs no Git baseline.

`helm.lint` and global CI use the same check. The Make targets locate the installed
binary through Go's settings; no PATH change or asdf reshim is required.
Parsing never renders a template or executes Helm functions.

Manual runs of `Repo - Lint Global` accept a `base-ref` input, defaulting to
`origin/main`. The checkout fetches full history for the merge-base comparison.
PR, merge-queue and push runs use their event's base SHA.
