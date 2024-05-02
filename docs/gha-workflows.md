# Docs: GitHub Actions Workflows

In this repo, we have many [GitHub Actions workflows](../.github/workflows) for different aspects
of the CI pipelines.

- [Integration tests](#integration-tests)
  - [Single Namespace](#single-namespace)
  - [Multi Namespace](#multi-namespace)
  - [Persistent namespace deletion](#persistent-namespace-deletion)
  - [GitHub Actions: Integration Test Template Configuration](#github-actions-integration-test-template-configuration)
    - [Integration Test Workflow Inputs](#workflow-inputs)
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

> [!NOTE]
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
      persistent: true
      extra-values: |
        ${{ matrix.deployment.extra-values }}
```

### Persistent namespace deletion

If you have long-running workflows with multiple jobs, you can set `persistent: true` which will keep the deployment namespace and not delete it after the workflow is done. But you need to add the following workflow to delete the namespace once the main workflow is finished.

```yaml
name: Helm Deployment Cleanup

on: 
  workflow_run:
    workflows: 
      - "*"
    types:
      - completed

jobs:
  cleaup:
    name: Delete Namespace
    uses: camunda/camunda-platform-helm/.github/workflows/test-integration-cleanup-template.yaml@main
    secrets: inherit
    with:
      github-run-id: "${{ github.event.workflow_run.id }}"
```
# GitHub Actions: Integration Test Template Configuration

This section details the inputs for the GitHub Actions workflow defined in `.github/workflows/test-integration-template.yaml` within the Camunda Platform Helm repository. These inputs allow you to customize Helm chart deployments for integration testing.

## Workflow Inputs

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

      # Git reference for the Camunda Helm chart repository 
      # Default: 'main'
      # Required: false
      camunda-helm-git-ref: 'main'

      # Git reference of the caller's repository (branch, tag, or commit SHA) that initiated the workflow
      # Default: 'main'
      # Required: false
      caller-git-ref: 'feature-branch'

      # Whether to keep the test deployment after the workflow is completed
      # Default: false
      # Note: All persistent deployments will be deleted frequently to save costs
      # Required: false
      persistent: false

      # Specifies the cloud platform that is currently used
      # Default: 'gke'
      # Required: false
      platforms: 'gke'

      # Types of operations to perform with the Helm chart, like install, upgrade
      # Default: 'install'
      # Required: false
      flows: 'install'

      # Flag to enable or disable the execution of test scenarios after Helm chart deployment
      # Default: true
      # Required: false
      test-enabled: true

      # Pass extra values to the Helm chart during deployment
      # Required: false
      extra-values: |
        global:
          image:
            tag: 8.2.10
        console:
          image:
            tag: xyz

# Note:
# - Adjust 'identifier', 'caller-git-ref', 'flows', 'test-enabled', and 'extra-values' as needed for your specific testing scenario.
# - The 'identifier' is essential for distinguishing between different deployments, particularly useful in environments with multiple parallel deployments.
# - For 'extra-values', ensure the YAML format is correct and that the values specified meet the requirements for your environment.

```

## Additional Information

For more details on how to use these inputs within the workflow or to modify them for specific testing needs, refer to the official [GitHub Actions documentation](https://docs.github.com/en/actions).
