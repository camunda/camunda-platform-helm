The changelog is automatically generated and it follows [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) format.

## [camunda-platform-12.0.0-alpha4](https://github.com/camunda/camunda-platform-helm/releases/tag/camunda-platform-12.0.0-alpha4) (2025-02-13)

### Fixes

- Add zeebe role (#2889)
- Add empty commit to alpha (#2901)

<!-- generated by git-cliff -->
### Release Info

Supported versions:

- Camunda applications: [8.7](https://github.com/camunda/camunda-platform/releases?q=tag%3A8.7&expanded=true)
- Helm values: [12.0.0-alpha4](https://artifacthub.io/packages/helm/camunda/camunda-platform/12.0.0-alpha4#parameters)
- Helm CLI: [3.17.0](https://github.com/helm/helm/releases/tag/v3.17.0)

Camunda images:

- docker.io/camunda/connectors-bundle:8.7.0-alpha4
- docker.io/camunda/console:8.7.0-alpha3
- docker.io/camunda/identity:8.7.0-alpha4
- docker.io/camunda/keycloak:25.0.6
- docker.io/camunda/operate:8.7.0-alpha4
- docker.io/camunda/optimize:8.7.0-alpha4
- docker.io/camunda/tasklist:8.7.0-alpha4
- docker.io/camunda/web-modeler-restapi:8.7.0-alpha4
- docker.io/camunda/web-modeler-webapp:8.7.0-alpha4
- docker.io/camunda/web-modeler-websockets:8.7.0-alpha4
- docker.io/camunda/zeebe:8.7.0-alpha4

Non-Camunda images:

- docker.io/bitnami/elasticsearch:8.15.4
- docker.io/bitnami/os-shell:12-debian-12-r34
- docker.io/bitnami/postgresql:14.15.0-debian-12-r10
- docker.io/bitnami/postgresql:15.10.0-debian-12-r2

### Verification

To verify the integrity of the Helm chart using [Cosign](https://docs.sigstore.dev/signing/quickstart/):

```shell
cosign verify-blob camunda-platform-12.0.0-alpha4.tgz \
  --bundle camunda-platform-12.0.0-alpha4.cosign.bundle \
  --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
  --certificate-identity "https://github.com/camunda/camunda-platform-helm/.github/workflows/chart-release-chores.yml@refs/pull/2902/merge"
```
