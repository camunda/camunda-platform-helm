# Camunda Platform Helm Chart Release Process

The charts are build, linted and tested on every push to the main branch. If the chart version
(in `Chart.yaml`) changes a new github release with the corresponding packaged helm chart is
created. The charts are hosted via github pages and use the release artifacts. We use the
[chart-releaser-action](https://github.com/helm/chart-releaser-action) to release the charts.

## Update Camunda Platform image tag

Before the release make sure to update the Docker image tag for all components to latest release
available in [camunda-platform](https://github.com/camunda/camunda-platform).

This could be done manually or via the workflow
[Update Image Tag](https://github.com/camunda/camunda-platform-helm/actions/workflows/chart-update-image-tag.yaml).

## Process

We are trying to automate as much as possible of the release process yet without sacrificing
transparency so we are using PR release flow with minimal manual interactions.

When it's time to release, just do the following steps.

Locally, run:

```
make release.chores
```

This action will:

- Locally pull latest updates to the `main` branch.
- Locally create a new branch called `release` from `main` branch.
- Bump chart version and make a commit.
- Generate release notes and make a commit.
- Push updated `release` branch to the repo.
- Generate a link to open a PR with prefilled title and template.

Next, all that you need to open the PR using the generated link and follow th checklist there.

> **Note**
>
> The release notes depend on git commit log, only the commits that follow
[Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) format) will be added to
the release notes.

## Artifact Hub

[Camunda repo](https://artifacthub.io/packages/search?repo=camunda) is already configured on
Artifact Hub. Once the release workflow is done, Artifact Hub automatically scans Camunda Helm repo
and the new release will show on Artifact Hub.

> **Note**
>
> The charts could take some time till shown on Artifact Hub (up to 30 min).
> But we sharing our Helm chart on Artifact Hub just for visibility. After successful release,
> our Helm charts are immediately available via [Camunda Helm Repo](https://helm.camunda.io).
