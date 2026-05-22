<!-- THIS FILE IS AUTO-GENERATED, DO NOT EDIT IT MANUALLY! -->
# Camunda 8 Helm Chart Version Matrix

## Overview

Camunda 8 Self-Managed is deployed via Helm charts.

For the best experience, please remember:

- The Camunda `application version` is different from the Helm `chart version`. The Camunda application version is presented by `appVersion` in the chart. The Camunda Helm chart version is presented by `version` in the chart.

- You can view application versions and chart versions via Helm CLI.

  ```helm search repo camunda/camunda-platform --versions```

- Always use the supported `Helm CLI` versions used with the Helm chart. They're mentioned in the matrix for all charts or under chart annotation `camunda.io/helmCLIVersion` for newer charts.

- Camunda 8.9 (chart 14.x) is the last minor that supports Helm v3. Camunda 8.10 (chart 15.x) and later require Helm v4.

- Camunda 8.9 (chart 14.x) is the last minor that supports Helm v3. Camunda 8.10 (chart 15.x) and later require Helm v4.

- Camunda 8.9 (chart 14.x) is the last minor that supports Helm v3. Camunda 8.10 (chart 15.x) and later require Helm v4.

- During the upgrade from the non-patch versions, ensure to review [version update instructions](https://docs.camunda.io/docs/self-managed/deployment/helm/upgrade/).

{{ range (ds "versions") }}
{{- $appVersion := .app }}
## [Camunda {{ $appVersion }}](./camunda-{{ $appVersion }})

{{ range .charts -}}
### [Helm chart {{ . }}](./camunda-{{ $appVersion }}/#helm-chart-{{ . | regexp.Replace "\\."  "" }})
{{ end -}}

{{ end -}}
