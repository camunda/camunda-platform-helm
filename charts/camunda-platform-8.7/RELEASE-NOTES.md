The changelog is automatically generated and it follows [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) format.

## [camunda-platform-12.0.1](https://github.com/camunda/camunda-platform-helm/releases/tag/camunda-platform-12.0.1) (2025-04-10)

### Fixes

- Update existingSecret params for 8.6 8.7 and 8.8 (#3299)
- Add gateway enabled (#3307)
- Port of new style to camunda unit tests in 8.7 (#3324)
- Refactored console unit tests in 8.7 (#3326)
- Refactor of identity unit tests in 8.7 (#3327)
- Refactored optimize unit tests to new style (#3332)
- Add elasticsearch golden files (#3334)
- Refactored web-modeler unit tests to new style (#3330)
- Refactored tasklist unit tests to new style (#3331)
- Refactored zeebe unit tests to new style in 8.7 (#3329)
- Refactored zeebe gateway unit tests to new style (#3328)
- Refactor of connector unit tests to new style in 8.7 (#3325)

<!-- generated by git-cliff -->
### Release Info

Supported versions:

- Camunda applications: [8.7](https://github.com/camunda/camunda-platform/releases?q=tag%3A8.7&expanded=true)
- Helm values: [12.0.1](https://artifacthub.io/packages/helm/camunda/camunda-platform/12.0.1#parameters)
- Helm CLI: [3.17.2](https://github.com/helm/helm/releases/tag/v3.17.2)

Camunda images:

- docker.io/camunda/connectors-bundle:8.7.0
- docker.io/camunda/console:8.7.7
- docker.io/camunda/identity:8.7.0
- docker.io/camunda/keycloak:26.1.4
- docker.io/camunda/operate:8.7.1
- docker.io/camunda/optimize:8.7.0
- docker.io/camunda/tasklist:8.7.1
- docker.io/camunda/web-modeler-restapi:8.7.0
- docker.io/camunda/web-modeler-webapp:8.7.0
- docker.io/camunda/web-modeler-websockets:8.7.0
- docker.io/camunda/zeebe:8.7.1

Non-Camunda images:

- docker.io/bitnami/elasticsearch:8.17.4
- docker.io/bitnami/os-shell:12-debian-12-r40
- docker.io/bitnami/postgresql:14.17.0-debian-12-r14
- docker.io/bitnami/postgresql:15.10.0-debian-12-r2

### Verification

To verify the integrity of the Helm chart using [Cosign](https://docs.sigstore.dev/signing/quickstart/):

```shell
cosign verify-blob camunda-platform-12.0.1.tgz \
  --bundle camunda-platform-12.0.1.cosign.bundle \
  --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
  --certificate-identity "https://github.com/camunda/camunda-platform-helm/.github/workflows/chart-release-chores.yml@refs/pull/3343/merge"
```
