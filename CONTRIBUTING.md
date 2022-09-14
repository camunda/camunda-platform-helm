# Contributing

 * [Issues and PRs](#issues-and-prs)
 * [Submitting a pull request](#submitting-a-pull-request)
   + [Helm version](#helm-version)
   + [Best Practices](#best-practices)
   + [Commit Guidelines](#commit-guidelines)
   + [Tests](#tests)
     - [Unit Tests](#unit-tests)
       * [Golden Files](#golden-files)
       * [Properties Test](#properties-test)
     - [Test License Headers](#test-license-headers)
   + [Documentation](#documentation)
 * [CI](#ci)
 * [Resources](#resources)

[fork]: /fork
[pr]: /compare
[CODE_OF_CONDUCT]: CODE_OF_CONDUCT.md

We're thrilled that you'd like to contribute to this project. Your help is essential for keeping it great.

Please note that this project is released with a [Contributor Code of Conduct](https://github.com/camunda/camunda-platform-helm/blob/main/CODE_OF_CONDUCT.md). By participating in this project you agree to abide by its terms.

## Issues and PRs

If you have suggestions for how this project could be improved, or want to report a bug, open an issue! We'd love all and any contributions. If you have questions, too, we'd love to hear them.

We'd also love PRs. If you're thinking of a large PR, we advise opening up an issue first to talk about it, though! Look at the links below if you're not sure how to open a PR.

## Submitting a pull request

Please feel free to fork this repository and open a pull request to fix an issue or add a new feature.

Make sure that your provided PR's works via:

 * `helm lint` to run the linting
 * `make fmt` to run the gofmt
 * `make checkLicense` to run the license check
 * `make test` to run the go tests
 * `helm install <releasename> chartPath/` to install a Helm release in your K8s cluster (e.g. kind)

We have the following expectation on PR's:

 * They follow the [best practices](#best-practices)
 * They contain new [tests](#tests) on a bug fix or on adding a new feature
 * They follow the [commit guidelines](#commit-guidelines)
 * The [documentation](#documentation) has been updated, if necessary.

### Helm version

To have a smooth contribution experience, before working on a new PR make sure to use the exact Helm version
that's currently used in the repo.

Helm version is set in the [.tool-versions](./.tool-versions) file, so you can use [asdf version manager](https://github.com/asdf-vm/asdf) to install Helm locally or just install the same version manually.

To install the Helm version that's used in this repo using `asdf`, in the repo root, run:

```
asdf plugin-add helm
asdf install helm
```

### Best Practices

Make sure you're familiar with some Helm best practices like:

 * https://helm.sh/docs/chart_best_practices/
 * https://codersociety.com/blog/articles/helm-best-practices

### Commit Guidelines

Commit messages should follow the [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/#summary) format.

For example:

```
fix: set correct port

Previously no metrics have been exposed because the wrong port was used. This commit fixes the post and set it to 9600.
```

Available commit types:

* `feat` - enhancements, new features
* `fix` - bug fixes
* `refactor` - non-behavior changes
* `test` - only changes in tests
* `docs` - changes in the documentation, readme, etc.
* `style` - apply code styles
* `build` - changes to the build (e.g. to Maven's `Chart.yaml`)
* `ci` - changes to the CI (e.g. to GitHub related configs)

### Tests

> **Note**
>
> For more details about Helm chart testing read the following blog post:
> [Advanced Test Practices For Helm Charts](https://medium.com/@zelldon91/advanced-test-practices-for-helm-charts-587caeeb4cb).

In order to make sure that the Helm charts work properly and that further development doesn't break anything we introduced with [#125](https://github.com/camunda/camunda-platform-helm/issues/125) tests for the Helm charts. The tests are written in go, and we use the [terratest framework](https://terratest.gruntwork.io/) to write them.

We separate our tests in two parts, with different targets and goals.

 1. **Template tests** (unit tests), which verify the general structure. Is it yaml conform, has it the right value/structure if set, does the default values not change or are set at all.
 2. **Integration tests**, which verify whether I can install the charts and use them. This means, are the manifests accepted by the K8 API and does it work (it can be valid yaml but not accepted by K8). Can the services reach each other and are they working?

**For new contributions it is expected to write new unit tests, but no integration tests.** We keep the count of integration tests to a minimum, and the knowledge for writing them is not expected for contributors.

Tests can be found in the `charts/camunda-platform` directory under `test/`. For each sub-chart we have a sub-directory 
in the `test/` directory. For example [test/zeebe](charts/camunda-platform/test/zeebe).

In order to run the tests, execute `make test` on the root repository level.

#### Unit Tests

As mentioned earlier we expect unit tests on new contributions. The unit tests (template tests) are divided in two parts, golden file tests and explicit property tests. In this section we want to explain when which test type should be used.

##### Golden Files

We write new golden file tests, for default values, where we can compare a complete manifest with his properties.
Most of the golden file tests are part of the `goldenfiles_test.go` in the corresponding sub-chart testing directory.
For an example see [zeebe/goldenfiles_test.go](charts/camunda-platform/test/zeebe/goldenfiles_test.go).

If the complete manifest can be enabled by a toggle, we also write a golden file test. This test is part of `<manifestFileName>_test.go` file. The `<manifestFileName>` corresponds to the template filename we have in the sub-chart `templates` dir. For example, the prometheus [servicemonitor](charts/camunda-platform/templates/service-monitor.yaml) can be enabled by a toggle. This means we write a golden file test in [servicemonitor_test.go](charts/camunda-platform/test/servicemonitor_test.go).

In order to generate the golden files run `make golden` on the root level of the repository. This will add a new golden file in a `golden` sub-dir and run the corresponding test. The golden files should also be named related to the manifest.

##### Properties Test

For things which are not per default enabled or set we write a property test.

Here we directly set the specific property/variable and verify that the Helm chart can be rendered and the property is set correctly on the object. These kind of tests should be part of a `<manifestFileName>_test.go` file. The `<manifestFileName>` corresponds to the template filename we have in the sub-chart `templates` dir. For example, for the zeebe statefulset manifest we have the test [statefulset_test.go](charts/camunda-platform/test/zeebe/statefulset_test.go) under the `zeebe` sub-dir.

It is always helpful to check already existing tests to get a better understanding in how to write new tests, so do not hesitant to read and copy them.

#### Test License Headers

Make sure that new go tests contain the apache license headers, otherwise the CI license check will fail. For adding and checking the license we use [addlicense](https://github.com/google/addlicense). In order to install it locally, simply run `make installLicense`. Afterwards you can run `make addlicense` to add the missing license header to a new go file.

#### OpenShift

There is a suite of integration tests which directly targets OpenShift in 
[openshift_test.go](/charts/camunda-platform/test/integration/openshift_test.go). It's separate from the normal integration test suite
as we need a specific base set of values to deploy the chart on OpenShift. This suite should only contain the following:

- Smoke tests which guarantee that all services can start, are minimally usable, and can communicate with each other (where applicable)
- OpenShift specific behavior tests

Everything else should be in either the normal golden file tests, or the other integration tests.

### Documentation

The `values.yaml` file follows Helm best practices https://helm.sh/docs/chart_best_practices/values/

This means:
  * Variable names should begin with a lowercase letter, and words should be separated with camelcase.
  * Every defined property in values.yaml should be documented. The documentation string should begin with the name of the property that it describes, and then give at least a one-sentence description

Furthermore, we try to apply the following pattern: `# [VarName] [conjunction] [definition]`

_VarName:_

  * In the documentation the variable name is started with a big letter, similar to Kubernetes resource documentation.
  * If the variable is part of a subsection/object we use a json path expression (to make it more clear where the variable belongs to).
    The root (chart name) is omitted (e.g. zeebe). This is useful for using --set in Helm.

_Conjunction:_

  * [defines] for mandatory configuration
  * [can be used] for optional configuration
  * [if true] for toggles
  * [configuration] for section/group of variables


All variables and the corresponding documentation are reflected in the
[README](https://github.com/camunda/camunda-platform-helm/blob/main/charts/camunda-platform/README.md).
Please make sure to update the README as well, if changing or adding new variables. Their exist an
helper script ([convertValuesDoc.sh](https://github.com/camunda/camunda-platform-helm/blob/main/charts/camunda-platform/convertValuesDoc.sh)) to generate a markdown like structure based on the `.yaml` file documentation.


## CI

CI is performed via GitHub Actions [workflow](.github/workflows). Currently, CI consists of:

- [chart linting](/.github/workflows/charttesting.yml)
- [a suite of golden file/unit tests](/.github/workflows/go.yml)
- [a suite of integration tests](/.github/workflows/go-it.yaml)
- [a suite of OpenShift integration tests](/.github/workflows/go-it-os.yaml)

### OpenShift

One of the workflows, [Openshift Integration Tests](/.github/workflows/go-it-os.yaml), runs the integration tests
against OpenShift. To do this, it uses a pre-provisioned cluster and service account. For more
details find out [how to provision both](https://confluence.camunda.com/display/ZEEBE/OpenShift+Cluster).

## Resources

- [How to Contribute to Open Source](https://opensource.guide/how-to-contribute/)
- [Using Pull Requests](https://help.github.com/articles/about-pull-requests/)
- [GitHub Help](https://help.github.com)
- [CODE_OF_CONDUCT](https://github.com/camunda/camunda-platform-helm/blob/main/CODE_OF_CONDUCT.md)
