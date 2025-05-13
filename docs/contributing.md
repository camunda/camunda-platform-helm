# Contributing

- [Issues and PRs](#issues-and-prs)
- [Backporting](#backporting)
- [Submitting a pull request](#submitting-a-pull-request)
  - [Helm version](#helm-version)
  - [Best Practices](#best-practices)
  - [Commit Guidelines](#commit-guidelines)
  - [Tests](#tests)
  - [Documentation](#documentation)
- [CI](#ci)
- [Resources](#resources)

[fork]: /fork
[pr]: /compare
[CODE_OF_CONDUCT]: CODE_OF_CONDUCT.md

We're thrilled that you'd like to contribute to this project. Your help is essential for keeping it great.

Please note that this project is released with a [Contributor Code of Conduct](https://github.com/camunda/camunda-platform-helm/blob/main/CODE_OF_CONDUCT.md).
By participating in this project you agree to abide by its terms.

## Issues and PRs

If you have suggestions for how this project could be improved, or want to report a bug, open an issue! We'd love all and any contributions.
If you have questions, too, we'd love to hear them.

We'd also love PRs. If you're thinking of a large PR, we advise opening up an issue first to talk about it, though!
Look at the links below if you're not sure how to open a PR.

## Backporting

Camunda enterprise covers the last three supported minor versions. Hence all fixes should be also backported to the supported versions.
We are using a directory-based structure which means all supported will be under the [charts directory](./../charts/).

In the charts directory, the latest supported chart doesn't have a version in its directory name like `camunda-platform`.
The previous releases have the Camunda version in their directory name e.g. `camunda-platform-8.4`.

Please note that the version mentioned here is the Camunda app version (in the Chart.yaml file represented by the key `version`),
not the chart version (in the Chart.yaml file represented by the key `version`).

## Submitting a pull request

Please feel free to fork this repository and open a pull request to fix an issue or add a new feature.

We have the following expectations on the PR's:

- They follow the [best practices](#best-practices)
- They contain new [tests](#tests) on a bug fix or on adding a new feature
- They follow the [commit guidelines](#commit-guidelines)
- The [documentation](#documentation) has been updated, if necessary.


### Helm version

To have a smooth contribution experience, before working on a new PR make sure to use the exact Helm version
that's currently used in the repo.

Helm version is set in the [.tool-versions](./.tool-versions) file, so you can use the [asdf version manager](https://github.com/asdf-vm/asdf)
to install Helm locally or just install the same version manually.

To install the Helm version that's used in this repo using `asdf`, in the repo root, run:

```
make tools.asdf-install
```

### Best Practices

Make sure you're familiar with some Helm best practices like:

- https://helm.sh/docs/chart_best_practices/
- https://codersociety.com/blog/articles/helm-best-practices

### Commit Guidelines

Commit messages should follow the [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/#summary) format.

For example:

```
fix: set correct port

Previously no metrics have been exposed because the wrong port was used. This commit fixes the post and set it to 9600.
```

Available commit types:

- `feat` - enhancements, new features
- `fix` - bug fixes
- `refactor` - non-behavior changes
- `test` - only changes in tests
- `docs` - changes in the documentation, readme, etc.
- `style` - apply code styles
- `build` - changes to the build (e.g. to Maven's `Chart.yaml`)
- `ci` - changes to the CI (e.g. to GitHub-related configs)

### Tests

> [!NOTE]
>
> For more details about Helm chart testing read the following blog post:
> [Advanced Test Practices For Helm Charts](https://medium.com/@zelldon91/advanced-test-practices-for-helm-charts-587caeeb4cb).

In order to make sure that the Helm charts work properly and that further development doesn't break anything we introduced tests for the Helm charts.
The tests are written in Golang, and we use the [Terratest framework](https://terratest.gruntwork.io/) to write them.

We separate our tests into two parts, with different targets and goals.

1. **Template tests** (unit tests), verify the general structure. Is it YAML conform, has it the right value/structure if set, do the default values not change or are set at all?
2. **Integration tests**, verify whether I can install the charts and use them. This means, are the manifests accepted by the K8 API and does it work? (it can be valid YAML but not accepted by K8s). Can the services reach each other and are they working?

**For new contributions it is expected to write new unit tests, but no integration tests.** We keep the count of integration tests to a minimum, and the knowledge for writing them is not expected for contributors.

Tests can be found in the chart directory under `test/`. For each component, we have a sub-directory
in the `test/` directory.

To run the tests, execute `make go.test` on the root repository level.

#### Unit Tests

As mentioned earlier, we expect unit tests on new contributions. The unit tests (template tests) are divided into two parts,
golden file tests and explicit property tests. In this section, we want to explain when which test type should be used.

##### Golden Files

We write new golden file tests, for default values, where we can compare a complete manifest with his properties.
Most of the golden file tests are part of the `goldenfiles_test.go` to the corresponding sub-chart testing directory.
For an example see `/test/zeebe/goldenfiles_test.go`.

If the complete manifest can be enabled by a toggle, we also write a golden file test. This test is part of `<manifestFileName>_test.go` file. The `<manifestFileName>` corresponds to the template filename we have in the sub-chart `templates` dir. For example, the Prometheus `templates/service-monitor.yaml` can be enabled by a toggle. This means we write a golden file test in `test/servicemonitor_test.go`.

To generate the golden files run `go.test-golden-updated` on the root level of the repository.
This will add a new golden file in a `golden` sub-dir and run the corresponding test. The golden files should also be named related to the manifest.

##### Properties Test

For things that are not per default enabled or set we write a property test.

Here we directly set the specific property/variable and verify that the Helm chart can be rendered and the property is set correctly on the object. This kind of test should be part of a `<manifestFileName>_test.go` file. The `<manifestFileName>` corresponds to the template filename we have in the sub-chart `templates` dir. For example, for the Zeebe statefulset manifest we have the test `test/zeebe/statefulset_test.go` under the `zeebe` sub-dir.

It is always helpful to check already existing tests to get a better understanding of how to write new tests, so do not hesitant to read and copy them.

#### Test License Headers

Make sure that new go tests contain the Apache license headers, otherwise, the CI license check will fail. For adding and checking the license we use [addlicense](https://github.com/google/addlicense). In order to install it locally, simply run `make go.addlicense-install`. Afterward, you can run `make go.addlicense-run` to add the missing license header to a new go file.

### Documentation

The `values.yaml` file follows Helm's best practices https://helm.sh/docs/chart_best_practices/values/

This means:

- Variable names should begin with a lowercase letter, and words should be separated with a camelcase.
- Every defined property in values.yaml should be documented. The documentation string should begin with the name of the property that it describes,
  and then give at least a one-sentence description

We are using [bitnami/readme-generator-for-helm](https://github.com/bitnami/readme-generator-for-helm)
to generate the Helm chart values docs from the values file. Ensure to follow the same convention of the tool.

## CI

CI is performed via GitHub Actions [workflow](.github/workflows).

## Resources

- [How to Contribute to Open Source](https://opensource.guide/how-to-contribute/)
- [Using Pull Requests](https://help.github.com/articles/about-pull-requests/)
- [GitHub Help](https://help.github.com)
- [CODE_OF_CONDUCT](https://github.com/camunda/camunda-platform-helm/blob/main/CODE_OF_CONDUCT.md)
