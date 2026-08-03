---
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

Most of the golden file tests are part of `goldenfiles_test.go` in the corresponding chart version testing directory. For an example see `charts/camunda-platform-8.10/test/unit/orchestration/goldenfiles_test.go`.

If the complete manifest can be enabled by a toggle, we also write a golden file test. This test is part of a `<manifestFileName>_test.go` file. The `<manifestFileName>` corresponds to the template filename in the sub-chart templates dir.

For example, the Prometheus `templates/service-monitor.yaml` can be enabled by a toggle, so we write a golden file test in `charts/camunda-platform-8.10/test/unit/orchestration/servicemonitor_test.go`.

To generate the golden files, run `make go.update-golden-only chartPath=charts/camunda-platform-8.10` at the repository root. This will add a new golden file in a `golden` sub-dir and run the corresponding test. The golden files should be named related to the manifest.

### Properties tests

For things that are not enabled or set by default, we write a property test. Here we directly set the specific property/variable and verify that the Helm chart can be rendered and the property is set correctly on the object.

This kind of test should be part of a `<manifestFileName>_test.go` file. The `<manifestFileName>` corresponds to the template filename in the sub-chart templates dir.

For example, for the Orchestration StatefulSet manifest we have the test `charts/camunda-platform-8.10/test/unit/orchestration/statefulset_test.go`.

It is always helpful to check existing tests to get a better understanding of how to write new tests, so do not hesitate to read and copy from them.

### Source-owned branch contracts

When a template or helper branches on a feature flag, backend, authentication mode, or TLS state, keep the semantic contract beside the test for the template that owns that predicate. Add a valid positive endpoint and the relevant negative or alternate endpoint. These rows describe intended behavior; generated combinations cannot infer that semantic oracle.

Set competing values explicitly, including `false` values needed to neutralize another backend or a version-local fixture default. Use the current values for the chart version under test. Do not use deprecated global datastore values as ordinary test scaffolding.

For Kubernetes object paths, the 8.8+ `RunTestCasesE` implementations apply deterministic semantics. Charts 8.7 and older do not execute ordinary `Expected` paths and do not provide `Unexpected`; use an explicit typed `Verifier` for those versions.

- `Expected` paths must resolve to exactly one scalar value in exactly one rendered object.
- `Unexpected` paths must resolve to zero values and therefore assert structural absence.
- `Expected: ""` means a present empty string, not an absent field.
- `Expected: "null"` matches the scalar representation `null`; it does not distinguish a YAML null from the literal string `"null"`. Use a typed verifier when the YAML type matters.
- Empty maps and lists are present non-scalar values. Use a typed verifier when their exact shape matters.

Anchor an absent leaf to a positive parent or sibling when deleting the whole structure would otherwise make the test pass. For example:

```go
testCases := []testhelpers.TestCase{
	{
		Name: "OpenSearch password is rendered for OpenSearch storage",
		Values: map[string]string{
			"orchestration.data.secondaryStorage.type":                                "opensearch",
			"orchestration.data.secondaryStorage.opensearch.auth.secret.inlineSecret": "test-password",
		},
		Expected: map[string]string{
			"spec.template.spec.containers[0].name": "orchestration",
			"spec.template.spec.containers[0].env[?(@.name=='VALUES_OPENSEARCH_PASSWORD')].name": "VALUES_OPENSEARCH_PASSWORD",
		},
	},
	{
		Name: "OpenSearch password is absent for Elasticsearch storage",
		Values: map[string]string{
			"orchestration.data.secondaryStorage.type":                                "elasticsearch",
			"orchestration.data.secondaryStorage.opensearch.auth.secret.inlineSecret": "test-password",
		},
		Expected: map[string]string{
			"spec.template.spec.containers[0].name": "orchestration",
		},
		Unexpected: []string{
			"spec.template.spec.containers[0].env[?(@.name=='VALUES_OPENSEARCH_PASSWORD')]",
		},
	},
}
```

Use an explicit typed `Verifier` instead of Kubernetes paths for embedded payloads such as `ConfigMap.data["application.yaml"]`. Unmarshal the rendered Kubernetes object, then unmarshal the embedded YAML into a small struct containing the fields under test. A verifier must not be combined with `Expected`, `Unexpected`, or object assertions in the same test case.

Current version-owned examples include:

- [`TestOpenSearchPasswordEnv` in the 8.10 Orchestration StatefulSet tests](https://github.com/camunda/camunda-platform-helm/blob/main/charts/camunda-platform-8.10/test/unit/orchestration/statefulset_test.go)
- [`TestCaBundleInitContainerSecurityContext` in the 8.9 TLS tests](https://github.com/camunda/camunda-platform-helm/blob/main/charts/camunda-platform-8.9/test/unit/common/tls_secrets_test.go)
- [`TestGlobalIngressHostTemplating` in the 8.10 Web Modeler ConfigMap tests](https://github.com/camunda/camunda-platform-helm/blob/main/charts/camunda-platform-8.10/test/unit/web-modeler/configmap_restapi_test.go)
- [8.7 document-store IRSA tests using explicit verifiers](https://github.com/camunda/camunda-platform-helm/blob/main/charts/camunda-platform-8.7/test/unit/common/documentstore_irsa_test.go)

Exhaustively enumerate a branch domain only when it is genuinely tiny and local. Source predicates and the values schema can suggest factors and boundaries, but they cannot generate the expected behavior. Pairwise coverage, MC/DC percentages, mutation scores, metamorphic relations, and solver-selected rows are optional maintainer audit tools, not contributor or merge gates.

## Test license headers

Make sure that new Go tests contain the Apache license headers, otherwise the CI license check will fail.

For adding and checking the license we use [addlicense](https://github.com/google/addlicense). To install it locally, run `make go.addlicense-install`. Afterward, you can run `make go.addlicense-run` to add the missing license header to a new Go file.
