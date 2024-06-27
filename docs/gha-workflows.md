# Docs: GitHub Actions Workflows

In this repo, we have many [GitHub Actions workflows](../.github/workflows) for different aspects
of the CI pipelines.

- [Camunda Helm chart deployment](#camunda-helm-chart-deployment)
  - [Workflow inputs](#workflow-inputs)
  - [Workflow patterns](#workflow-patterns)

## Camunda Helm chart deployment

The Distro team provides a GitHub Actions Workflow to deploy the Camunda Helm chart via GitHub Actions. This workflow is customizable and supports different patterns. For example: disabling integration tests, single namespace, multi namespace, persistent setup with a defined ttl (not deleted after the workflow is done), and more.

The GitHub Actions workflow is defined in the [test-integration-template.yaml](../.github/workflows/test-integration-template.yaml) within the Camunda Platform Helm repository and could be used by other repos within Camunda organizations.

### Workflow inputs

These inputs allow you to customize Helm chart deployments for integration testing.

```yaml
jobs:
  ...
  helm-deploy:
    name: Helm chart Integration Tests
    uses: camunda/camunda-platform-helm/.github/workflows/test-integration-template.yaml@main
    secrets: inherit
    with:
      # Unique identifier used in the deployment hostname
      # Required: true
      identifier: 'dev-console-sm'

      # A reference for the Camunda Helm chart directory which allows to test unreleased chagnes from Git repo.
      # The latest supported chart doesn't have a version in its directory name like `camunda-platform`.
      # The previous releases have the Camunda version in their directory name e.g. `camunda-platform-8.4`.
      # Default: 'camunda-platform-latest'
      # Required: false
      camunda-helm-dir: 'camunda-platform-latest'

      # Git reference for the Camunda Helm chart repository 
      # Default: 'main'
      # Required: false
      camunda-helm-git-ref: 'main'

      # Git reference of the caller's repository (branch, tag, or commit SHA) that initiated the workflow
      # Default: 'main'
      # Required: false
      caller-git-ref: ''

      # Define a ttl for the deployment after the workflow is completed
      # Note: All persistent deployments will be deleted frequently to save costs
      # Default: ""
      # Required: false
      deployment-ttl:

      # Specifies the cloud platform that is currently used
      # Default: 'gke'
      # Required: false
      platforms: ''

      # Types of operations to perform with the Helm chart, like install, upgrade
      # Default: 'install'
      # Required: false
      flows: ''

      # Flag to enable or disable the execution of test scenarios after Helm chart deployment
      # Default: true
      # Required: false
      test-enabled:

      # Pass extra values to the Helm chart during deployment
      # Required: false
      extra-values: |
        global:
          image:
            tag: 8.2.10
        console:
          image:
            tag: xyz
```

> [!NOTE]
> - Adjust `identifier`, `caller-git-ref`, `flows`, `test-enabled`, and `extra-values` as needed for your specific testing scenario.
> - The `identifier` is essential for distinguishing between different deployments, particularly useful in environments with multiple parallel deployments.
> - For `extra-values`, ensure the YAML format is correct and that the values specified meet the requirements for your environment.
> - For more details on how to use these inputs within the workflow or to modify them for specific testing needs, refer to the official [GitHub Actions documentation](https://docs.github.com/en/actions).

> [!NOTE]
> The default behavior in the integration tests workflow is to delete the test resources 
> after the test is finished. To keep the deployment for at least one day, you need to set `deployment-ttl: 1d`.
> and you need to rerun the workflow again when you need the deployment to be persistent with a defined deployment-ttl.
Example deployment-ttl values:
360s: 360 seconds
10m: 10 minutes
24h: 24 hours
7d: 7 days
2w: 2 weeks


### Workflow patterns

The most important workflow is the [main integration workflow](../.github/workflows/test-integration-template.yaml),
where each Camunda component can reuse that workflow in their CI pipelines to ensure that
each component works as expected within the Camunda as a whole.

#### Single Namespace

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
in your repo (the URL will show in the PR or the GH deployment section).

#### Multi Namespace

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
              zeebeGateway:
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
      deployment-ttl: 1d
      extra-values: |
        ${{ matrix.deployment.extra-values }}
```

#### Persistent deployment

If you have long-running workflows with multiple jobs, you can set `deployment-ttl: 1d` which will keep the deployment namespace for 1 day and not delete it after the workflow is done.
