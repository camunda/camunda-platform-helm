# Camunda 8 Helm Charts

## Release Cycles

Camunda provides a standard support policy of **18 months from the date it is released**. During this time, patches are regularly released containing security and bug fixes, some of which may come from dependency updates. Therefore, for the vast majority of dependencies Camunda only applies patch updates.

## Compatibility

Each Camunda version is only compatible with the corresponding chart. For example, Camunda `8.7.x` is only compatible with Helm chart `12.x.x` version.

For more details about the applications version included in the Helm chart, review the full [version matrix](https://helm.camunda.io/camunda-platform/version-matrix/).

## Versions

Check the [chart-versions.yaml](./chart-versions.yaml) to find all information about the chart versions, the following is an overview: 

- **Alpha:** The next Camunda versions under development. They are released for test purposes only.
- **Standard Support:** Stable Camunda versions reached general availability. They are stable and supported for 18 months from the release date.
- **Extended Support:** Stable Camunda versions but only get critical patches. They are supported for 48 months from the release date.
- **End Of Life:** Stable Camunda versions are not supported anymore.

In the file [chart-versions.yaml](./chart-versions.yaml), all numbers are in the format of "major.minor" (e.g. "8.8", "8.7", etc.).
The versions should be in the same order and be quoted in the YAML file to avoid issues with trailing zeros.

## References

- [Announcements and release notes](https://docs.camunda.io/docs/reference/announcements-release-notes/overview/) (find list of currently supported versions)
- [Release policy and cycles](https://docs.camunda.io/docs/reference/announcements-release-notes/release-policy/)
- [Release Policy](https://camunda.com/release-policy/)
