# Camunda Platform Helm Chart Release Process

The charts are build, linted and tested on every push to the main branch. If the chart version (in `Chart.yaml`) changes a new github release with the corresponding packaged helm chart is created. The charts are hosted via github pages and use the release artifacts. We use the [chart-releaser-action](https://github.com/helm/chart-releaser-action) to release the charts.

### Process

In order, to make the release easier we have created a script [release.sh](charts/camunda-platform/release.sh),
which accepts as parameter the new version to release. This script will set in the parent and sub-charts
the corresponding version. Please make sure to adjust the changelog in [Chart.yaml](charts/camunda-platform/Chart.yaml).
This changelog will be shown on [artifacthub.io](https://artifacthub.io/packages/helm/camunda-platform-helm/camunda-platform).

After committing and pushing the changes on the `Chart.yaml` file, our [release github action](.github/workflows/release.yaml) will do the rest.
