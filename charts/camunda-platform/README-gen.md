## Parameters

### Global parameters

| Name                       | Description                                                                                                                             | Value              |
| -------------------------- | --------------------------------------------------------------------------------------------------------------------------------------- | ------------------ |
| `global`                   |                                                                                                                                         |                    |
| `global.annotations`       | Annotations can be used to define common annotations, which should be applied to all deployments                                        | `{}`               |
| `global.labels.app`        | Name of the application                                                                                                                 | `camunda-platform` |
| `global.image.registry`    | Can be used to set container image registry.                                                                                            | `""`               |
| `global.image.tag`         | defines the tag / version which should be used in the most of the apps.                                                                 | `8.2.12`           |
| `global.image.pullPolicy`  | defines the image pull policy which should be used https://kubernetes.io/docs/concepts/containers/images/#image-pull-policy             | `IfNotPresent`     |
| `global.image.pullSecrets` | can be used to configure image pull secrets https://kubernetes.io/docs/concepts/containers/images/#specifying-imagepullsecrets-on-a-pod | `[]`               |
