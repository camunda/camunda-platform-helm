The changelog is automatically generated using [git-chglog](https://github.com/git-chglog/git-chglog)
and it follows [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) format.


<a name="camunda-platform-8.3.14"></a>
## [camunda-platform-8.3.14](https://github.com/camunda/camunda-platform-helm/releases/tag/camunda-platform-8.3.14) (2024-06-28)

### Ci

* automate release chores ([#2013](https://github.com/camunda/camunda-platform-helm/issues/2013))

### Refactor

* remove the global image tag value and use it from the components - 8.2, 8.3, and 8.4 ([#2080](https://github.com/camunda/camunda-platform-helm/issues/2080))

### Release Info

Supported versions:

- Camunda applications: [8.3](https://github.com/camunda/camunda-platform/releases?q=tag%3A8.3&expanded=true)
- Helm values: [8.3.14](https://artifacthub.io/packages/helm/camunda/camunda-platform/8.3.14#parameters)
- Helm CLI: [3.15.2](https://github.com/helm/helm/releases/tag/v3.15.2)

Camunda images:

- docker.io/camunda/connectors-bundle:8.3.13
- docker.io/camunda/identity:8.3.13
- docker.io/camunda/operate:8.3.13
- docker.io/camunda/optimize:8.3.11
- docker.io/camunda/tasklist:8.3.14
- docker.io/camunda/zeebe:8.3.13
- registry.camunda.cloud/console/console-sm:latest
- registry.camunda.cloud/web-modeler-ee/modeler-restapi:8.3.9
- registry.camunda.cloud/web-modeler-ee/modeler-webapp:8.3.9
- registry.camunda.cloud/web-modeler-ee/modeler-websockets:8.3.9

Non-Camunda images:

- docker.io/bitnami/elasticsearch:8.8.2
- docker.io/bitnami/keycloak:22.0.5
- docker.io/bitnami/os-shell:11-debian-11-r92
- docker.io/bitnami/postgresql:14.5.0-debian-11-r35
- docker.io/bitnami/postgresql:15.5.0

### Verification

To verify the integrity of the Helm chart using [Cosign](https://docs.sigstore.dev/signing/quickstart/):

```shell
cosign verify-blob camunda-platform-8.3.14.tgz \
  --bundle camunda-platform-8.3.14.cosign.bundle \
  --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
  --certificate-identity "https://github.com/camunda/camunda-platform-helm/.github/workflows/chart-release-chores.yml@refs/pull/2014/merge"
```
