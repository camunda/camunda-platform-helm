# Docs: GitHub Actions Workflows

In this repo, we have many [GitHub Actions workflows](../.github/workflows) for different aspects
of the CI pipelines.

## Integration tests

The most important workflow is the [main integration workflow](../.github/workflows/test-integration-main.yaml),
where each Camunda Platform component can reuse that workflow in their CI pipelines to ensure that
each component works as expected within the Camunda Platform as a whole.

To embed the Camunda Platform Helm chart integration tests in your GHA workflow, you need to use
the following:

```yaml
deploy:
  name: Helm chart Integration Tests
  uses: camunda/camunda-platform-helm/.github/workflows/test-integration-main.yaml@main
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

Adding that will run Camunda Platform Helm chart integration tests and add the deployment URL
in your repo (the URL will show in the PR or in the GH deployment section).

> **Note**
> The default behavior in the integration tests workflow is to deleted the test resources 
> after the test is finished. To keep the deployment you need to set `persistent: true`.
> **However**, even it's persistent, the resources will be deleted frequently to save costs
> and you need to rerun the workflow again when you need it.
