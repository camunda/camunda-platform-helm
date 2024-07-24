The changelog is automatically generated using [git-chglog](https://github.com/git-chglog/git-chglog)
and it follows [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) format.


<a name="camunda-platform-10.2.1"></a>
## [camunda-platform-10.2.1](https://github.com/camunda/camunda-platform-helm/releases/tag/camunda-platform-10.2.1) (2024-07-24)

### Fix

* added recreate strategy to all Operate deployments (#2143)

### Release Info

Supported versions:

- Camunda applications: [8.5](https://github.com/camunda/camunda-platform/releases?q=tag%3A8.5&expanded=true)
- Helm values: [10.2.1](https://artifacthub.io/packages/helm/camunda/camunda-platform/10.2.1#parameters)
- Helm CLI: [3.15.3](https://github.com/helm/helm/releases/tag/v3.15.3)

Camunda images:

- docker.io/camunda/connectors-bundle:8.5.4
- docker.io/camunda/identity:8.5.4
- docker.io/camunda/identity:latest
- docker.io/camunda/operate:8.5.4
- docker.io/camunda/optimize:8.5.3
- docker.io/camunda/tasklist:8.5.3
- docker.io/camunda/zeebe:8.5.5
- registry.camunda.cloud/console/console-sm:8.5.83
- registry.camunda.cloud/web-modeler-ee/modeler-restapi:8.5.5
- registry.camunda.cloud/web-modeler-ee/modeler-webapp:8.5.5
- registry.camunda.cloud/web-modeler-ee/modeler-websockets:8.5.5

Non-Camunda images:

- docker.io/bitnami/elasticsearch:8.12.2
- docker.io/bitnami/keycloak:23.0.7
- docker.io/bitnami/os-shell:12-debian-12-r18
- docker.io/bitnami/postgresql:14.12.0
- docker.io/bitnami/postgresql:15.7.0

### Verification

To verify the integrity of the Helm chart using [Cosign](https://docs.sigstore.dev/signing/quickstart/):

```shell
cosign verify-blob camunda-platform-10.2.1.tgz \
  --bundle camunda-platform-10.2.1.cosign.bundle \
  --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
  --certificate-identity "https://github.com/camunda/camunda-platform-helm/.github/workflows/chart-release-chores.yml@refs/pull/2127/merge"
```
