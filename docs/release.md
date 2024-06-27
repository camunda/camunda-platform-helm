# Camunda Helm Chart Release Process

The charts are built, linted, and tested on every push to the main branch. If the chart version
(in `Chart.yaml`) changes a new GitHub release with the corresponding packaged helm chart is
created. The charts are hosted via GitHub pages and use the release artifacts. We use the
[chart-releaser-action](https://github.com/helm/chart-releaser-action) to release the charts.

## Snapshot

For debugging and advanced testing purposes, we provide a snapshot chart from the unreleased changes on the `main` branch (never use it in production or stable environments).

It will be always updated with the fixed version name like `0.0.0-snapshot-latest` (the tag needed to start with SemVer because Helm requires it).

### Latest

The latest unreleased changes version has rolling version `0.0.0-snapshot-latest`:

```shell
helm template my-camunda-devel \
  oci://ghcr.io/camunda/helm/camunda-platform \
  --version 0.0.0-snapshot-latest
```

### Alpha

The Alpha version is available in two flavors:

Rolling version `0.0.0-snapshot-alpha` (always pointing to the Camunda latest alpha release):

```shell
helm template my-camunda-devel \
  oci://ghcr.io/camunda/helm/camunda-platform \
  --version 0.0.0-snapshot-alpha
```

Fixed version like `0.0.0-8.6.0-alpha2` (it will be updated according to the Camunda latest alpha release):

```shell
helm template my-camunda-devel \
  oci://ghcr.io/camunda/helm/camunda-platform \
  --version 0.0.0-8.6.0-alpha2
```

## Process

We are trying to automate as much as possible of the release process yet without sacrificing
transparency so we are using PR release flow with minimal manual interactions.

When it's time to release, just do the following steps.

Locally, run:

```shell
make release.chores
```

This action will:

- Locally pull the latest updates to the `main` branch.
- Locally create a new branch called `release` from `main` branch.
- Bump the chart version and make a commit.
- Generate release notes and make a commit.
- Push the updated `release` branch to the repo.
- Generate a link to open a PR with prefilled title and template.

Next, all you need to open the PR using the generated link and follow the checklist there.

> [!NOTE]
>
> The release notes depend on git commit log, only the commits that follow
[Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) format) will be added to
the release notes.

## Artifact Hub

[Camunda repo](https://artifacthub.io/packages/search?repo=camunda) is already configured on
Artifact Hub. Once the release workflow is done, Artifact Hub automatically scans the Camunda Helm repo
and the new release will show on Artifact Hub.

> [!NOTE]
>
> The charts could take some time till shown on Artifact Hub (up to 30 min).
> But we sharing our Helm chart on Artifact Hub just for visibility. After successful release,
> our Helm charts are immediately available via [Camunda Helm Repo](https://helm.camunda.io).
