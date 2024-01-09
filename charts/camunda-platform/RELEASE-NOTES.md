The changelog is automatically generated using [git-chglog](https://github.com/git-chglog/git-chglog)
and it follows [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) format.


<a name="camunda-platform-9.0.0"></a>
## [camunda-platform-9.0.0](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.3.5...camunda-platform-9.0.0) (2024-01-09)

> [!IMPORTANT]
> Starting from the 8.4 release (January 2024), the Camunda Helm chart version is decoupled from the version of the application (e.g., the chart version is 9.0.0 and the application version is 8.4.x).
>
> For more details about the applications' version included in the Helm chart, check out the [full version matrix](https://helm.camunda.io/camunda-platform/version-matrix/).

### Ci

* enhance version matrix and release notes generation ([#1204](https://github.com/camunda/camunda-platform-helm/issues/1204))
* removing dev comments before the release ([#1127](https://github.com/camunda/camunda-platform-helm/issues/1127))

### Docs

* rename example deployment name ([#1159](https://github.com/camunda/camunda-platform-helm/issues/1159))

### Feat

* use new Identity variables for auth configuration ([#1155](https://github.com/camunda/camunda-platform-helm/issues/1155))

### Refactor

* update helm chart version schema ([#1171](https://github.com/camunda/camunda-platform-helm/issues/1171))
* remove deprecated tasklist graphql playground ([#1172](https://github.com/camunda/camunda-platform-helm/issues/1172))
* upgrade elasticsearch image from 8.8.2 to 8.9.2 ([#1130](https://github.com/camunda/camunda-platform-helm/issues/1130))
* fail if Multi-Tenancy requirements are not met ([#1160](https://github.com/camunda/camunda-platform-helm/issues/1160))
* upgrade keycloak chart from 16.1.7 to 17.3.5 ([#1143](https://github.com/camunda/camunda-platform-helm/issues/1143))
* resolve issues with web modeler deployment when using OIDC ([#1189](https://github.com/camunda/camunda-platform-helm/issues/1189))
* use correct web modeler audiences ([#1187](https://github.com/camunda/camunda-platform-helm/issues/1187))

### BREAKING CHANGE

The new Identity variables for auth configuration may require an update to values.yaml, refer to https://docs.camunda.io/docs/next/self-managed/platform-deployment/helm-kubernetes/guides/connect-to-an-oidc-provider/ and https://docs.camunda.io/docs/next/self-managed/platform-deployment/helm-kubernetes/upgrade/#version-update-instructions for more instructions.
