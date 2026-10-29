# Complexity checks

The default command checks Go functions and workflow run blocks.
The Helm report inspects named templates and unnamed/root bodies without rendering
templates or executing Helm functions.

```sh
make helm.template-complexity-report chartPath=charts/camunda-platform-8.11
```

The report counts `if`, `range` and `with` actions and their maximum control depth.
Chained alternatives share a level. Definitions and blocks start independent
bodies; called templates are not expanded. Comments and quoted template-like
text are not control actions.

Chart paths are relative to the repository root, or absolute paths inside it.
Only the selected Camunda charts' own templates are included. Dependency charts
are excluded, and parsing errors fail the report. No Git baseline is required.
