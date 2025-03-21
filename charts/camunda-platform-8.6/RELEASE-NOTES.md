The changelog is automatically generated and it follows [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) format.

## [camunda-platform-11.3.0](https://github.com/camunda/camunda-platform-helm/releases/tag/camunda-platform-11.3.0) (2025-03-20)

### Features

- Add global.extraManifests support for injecting arbitrary YAML (#3050)

### Fixes

- Changing helper function for identityURL (#3084)

<!-- generated by git-cliff -->
### Release Info

Supported versions:

- Camunda applications: [8.6](https://github.com/camunda/camunda-platform/releases?q=tag%3A8.6&expanded=true)
- Helm values: [11.3.0](https://artifacthub.io/packages/helm/camunda/camunda-platform/11.3.0#parameters)
- Helm CLI: [3.17.2](https://github.com/helm/helm/releases/tag/v3.17.2)

Camunda images:

- docker.io/camunda/connectors-bundle:8.6.9
- docker.io/camunda/console:8.6.74
- docker.io/camunda/identity:8.6.9
- docker.io/camunda/keycloak:25.0.6
- docker.io/camunda/operate:8.6.12
- docker.io/camunda/optimize:8.6.6
- docker.io/camunda/tasklist:8.6.12
- docker.io/camunda/web-modeler-restapi:8.6.8
- docker.io/camunda/web-modeler-webapp:8.6.8
- docker.io/camunda/web-modeler-websockets:8.6.8
- docker.io/camunda/zeebe:8.6.12

Non-Camunda images:

- docker.io/bitnami/elasticsearch:8.15.4
- docker.io/bitnami/os-shell:12-debian-12-r39
- docker.io/bitnami/postgresql:14.17.0-debian-12-r6
- docker.io/bitnami/postgresql:15.10.0-debian-12-r2

### Verification

To verify the integrity of the Helm chart using [Cosign](https://docs.sigstore.dev/signing/quickstart/):

```shell
cosign verify-blob camunda-platform-11.3.0.tgz \
  --bundle camunda-platform-11.3.0.cosign.bundle \
  --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
  --certificate-identity "https://github.com/camunda/camunda-platform-helm/.github/workflows/chart-release-chores.yml@refs/pull/3193/merge"
```
