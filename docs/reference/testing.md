---
id: testing
title: Testing
---

This is the full testing reference for the Camunda Platform Helm chart. New contributions are expected to include unit tests — see [Contribution & Collaboration](../contribution-and-collaboration.md) for the contribution requirements.

:::note
For more details about Helm chart testing, read the blog post: [Advanced Test Practices For Helm Charts](https://medium.com/@zelldon91/advanced-test-practices-for-helm-charts-587caeeb4cb).
:::

In order to make sure that the Helm charts work properly and that further development doesn't break anything, we have introduced tests for the Helm charts.

The tests are written in Go, using the [Terratest framework](https://terratest.gruntwork.io/).

We separate our tests into two parts, with different targets and goals.

- **Template tests (unit tests)** verify the general structure. Is it YAML-conformant, does it have the right value/structure if set, do the default values not change or are they set at all?
- **Integration tests** verify whether the charts can be installed and used. This means: are the manifests accepted by the K8s API, and do they work? Can the services reach each other and are they working?

For new contributions it is expected to write new unit tests, but no integration tests. We keep the count of integration tests to a minimum, and the knowledge for writing them is not expected for contributors.

Tests can be found in the chart directory under `test/`. For each component we have a sub-directory in the `test/` directory.

To run the tests, execute `make go.test` at the repository root.

## Unit tests

As mentioned earlier, we expect unit tests on new contributions. The unit tests (template tests) are divided into two parts: golden file tests and explicit property tests. In this section we explain when which test type should be used.

### Golden files

We write new golden file tests for default values, where we can compare a complete manifest with its properties.

Most of the golden file tests are part of `goldenfiles_test.go` in the corresponding sub-chart testing directory. For an example see `/test/zeebe/goldenfiles_test.go`.

If the complete manifest can be enabled by a toggle, we also write a golden file test. This test is part of a `<manifestFileName>_test.go` file. The `<manifestFileName>` corresponds to the template filename in the sub-chart templates dir.

For example, the Prometheus `templates/service-monitor.yaml` can be enabled by a toggle, so we write a golden file test in `test/servicemonitor_test.go`.

To generate the golden files, run `go.test-golden-updated` at the repository root. This will add a new golden file in a `golden` sub-dir and run the corresponding test. The golden files should be named related to the manifest.

### Properties tests

For things that are not enabled or set by default, we write a property test. Here we directly set the specific property/variable and verify that the Helm chart can be rendered and the property is set correctly on the object.

This kind of test should be part of a `<manifestFileName>_test.go` file. The `<manifestFileName>` corresponds to the template filename in the sub-chart templates dir.

For example, for the Zeebe StatefulSet manifest we have the test `test/zeebe/statefulset_test.go` under the zeebe sub-dir.

It is always helpful to check existing tests to get a better understanding of how to write new tests, so do not hesitate to read and copy from them.

## Test license headers

Make sure that new Go tests contain the Apache license headers, otherwise the CI license check will fail.

For adding and checking the license we use [addlicense](https://github.com/google/addlicense). To install it locally, run `make go.addlicense-install`. Afterward, you can run `make go.addlicense-run` to add the missing license header to a new Go file.
