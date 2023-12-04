# Docs: GitHub Actions Workflows

In this repo, we have many [GitHub Actions workflows](../.github/workflows) for different aspects
of the CI pipelines.

## Integration tests

The most important workflow is the [main integration workflow](../.github/workflows/test-integration-template.yaml),
where each Camunda component can reuse that workflow in their CI pipelines to ensure that
each component works as expected within the Camunda as a whole.

### Single Namespace

To embed the Camunda Helm chart integration tests in your GHA workflow, you need to use
the following:

```yaml
jobs:
  ...
  helm-deploy:
    name: Helm chart Integration Tests
    uses: camunda/camunda-platform-helm/.github/workflows/test-integration-template.yaml@main
    secrets: inherit
    with:
      identifier: dev-console-sm
      extra-values: |
        global:
          image:
            tag: 8.2.10
        console:
          image:
            tag: xyz
```

Adding that will run Camunda Helm chart integration tests and add the deployment URL
in your repo (the URL will show in the PR or in the GH deployment section).

> **Note**
> The default behavior in the integration tests workflow is to delete the test resources 
> after the test is finished. To keep the deployment you need to set `persistent: true`.
> **However**, even it's persistent, the resources will be deleted frequently to save costs
> and you need to rerun the workflow again when you need it.

### Multi Namespace

> **Warning**
>
> This is a pre-alpha feature and not for external use.

```yaml
jobs:
  ...
  helm-deploy:
    strategy:
      matrix:
        deployment:
          - id: management
            extra-values: |
              zeebe:
                enabled: false
              zeebe-gateway:
                enabled: false
              operate:
                enabled: false
              tasklist:
                enabled: false
              optimize:
                enabled: false
              connectors:
                enabled: false
              elasticsearch:
                enabled: false
          - id: team01
            extra-values: |
              global:
                identity:
                  service:
                    url: "http://integration-identity.camunda-platform-id-dev-console-sm-main.svc.cluster.local:80/identity"
                  keycloak:
                    url:
                      protocol: "http"
                      host: "integration-keycloak.camunda-platform-id-dev-console-sm-main.svc.cluster.local"
                      port: "80"
              identity:
                enabled: false
              webModeler:
                enabled: false
              postgresql:
                enabled: false
              console:
                enabled: false
          - id: team02
            extra-values: |
              global:
                identity:
                  service:
                    url: "http://integration-identity.camunda-platform-id-dev-console-sm-main.svc.cluster.local:80/identity"
                  keycloak:
                    url:
                      protocol: "http"
                      host: "integration-keycloak.camunda-platform-id-dev-console-sm-main.svc.cluster.local"
                      port: "80"
              identity:
                enabled: false
              webModeler:
                enabled: false
              postgresql:
                enabled: false
              console:
                enabled: false
    name: Helm integration tests - ${{ matrix.deployment.id }}
    uses: camunda/camunda-platform-helm/.github/workflows/test-integration-template.yaml@main
    secrets: inherit
    with:
      identifier: dev-console-sm-${{ matrix.deployment.id }}
      persistent: true
      extra-values: |
        ${{ matrix.deployment.extra-values }}
```
