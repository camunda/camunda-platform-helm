---
applyTo: "**/*_test.go"
---

# Go Tests — Scoped Instructions

## Overview

Unit tests for Helm charts live in `charts/<version>/test/unit/<component>/`. They use
[Terratest](https://github.com/gruntwork-io/terratest) to render templates and assert on
Kubernetes objects, and [testify](https://github.com/stretchr/testify) for assertions. All
tests follow the `testify/suite` pattern with a suite struct, a `Test<Resource>Template` entry
function, and table-driven helpers via `testhelpers.RunTestCasesE`. Golden file tests capture
full rendered output for default values and are stored in `test/unit/<component>/golden/`. The
`-update-golden` flag regenerates snapshots. Apache 2.0 license headers are required on every
file. Run `make go.test chartPath=charts/camunda-platform-8.10` to execute tests for a chart.

---

## Critical Rules

### NEVER
- **NEVER** write a test that calls `helm.RenderTemplate` directly — use `testhelpers.RunTestCasesE`
  or the `utils.TemplateGoldenTest` suite for consistency.
- **NEVER** use `assert.NoError` for setup/chart-path steps that must abort the test on failure —
  use `require.NoError` so the test stops immediately.
- **NEVER** commit golden files generated from an outdated chart dependency state. Always run
  `make helm.dependency-update` before regenerating golden files.
- **NEVER** name a suite entry function anything other than `Test<Resource>Template` — CI relies
  on this naming to discover and run tests.
- **NEVER** skip `t.Parallel()` in the entry function — tests must run in parallel.
- **NEVER** write tests that share mutable state across `TestCase` entries.

### ALWAYS
- **ALWAYS** add an Apache 2.0 license header to every new `_test.go` file.
- **ALWAYS** use `require.NoError(t, err)` immediately after `filepath.Abs` and any setup call
  that could fail.
- **ALWAYS** unmarshal rendered YAML with `helm.UnmarshalK8SYaml(s.T(), output, &obj)` before
  asserting on fields.
- **ALWAYS** run `make go.fmt` and `make go.addlicense-check` before committing.
- **ALWAYS** add a corresponding golden file test whenever adding a new template resource file.
- **ALWAYS** use `s.Require()` (suite-scoped require) inside suite test methods, not standalone `require`.

---

## Core Patterns with Code Examples

### 1. Suite Declaration

```go
// Copyright 2022 Camunda Services GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// ...

package orchestration

import (
    "camunda-platform/test/unit/testhelpers"
    "path/filepath"
    "strings"
    "testing"

    "github.com/gruntwork-io/terratest/modules/helm"
    "github.com/gruntwork-io/terratest/modules/random"
    "github.com/stretchr/testify/require"
    "github.com/stretchr/testify/suite"
    appsv1 "k8s.io/api/apps/v1"
    corev1 "k8s.io/api/core/v1"
)

type StatefulSetTest struct {
    suite.Suite
    chartPath string
    release   string
    namespace string
    templates []string
}

func TestStatefulSetTemplate(t *testing.T) {
    t.Parallel()

    chartPath, err := filepath.Abs("../../../")
    require.NoError(t, err)

    suite.Run(t, &StatefulSetTest{
        chartPath: chartPath,
        release:   "camunda-platform-test",
        namespace: "camunda-platform-" + strings.ToLower(random.UniqueId()),
        templates: []string{"templates/orchestration/statefulset.yaml"},
    })
}
```

### 2. Table-Driven Tests with RunTestCasesE

```go
func (s *StatefulSetTest) TestDifferentValuesInputs() {
    testCases := []testhelpers.TestCase{
        {
            Name: "TestContainerSetPodLabels",
            Values: map[string]string{
                "orchestration.podLabels.foo": "bar",
            },
            Verifier: func(t *testing.T, output string, err error) {
                var statefulSet appsv1.StatefulSet
                helm.UnmarshalK8SYaml(t, output, &statefulSet)

                s.Require().Equal("bar", statefulSet.Spec.Template.Labels["foo"])
            },
        },
        {
            Name: "TestContainerDisabled",
            Values: map[string]string{
                "orchestration.enabled": "false",
            },
            Verifier: func(t *testing.T, output string, err error) {
                s.Require().Error(err)
                s.Require().Contains(err.Error(), "Error: could not find template")
            },
        },
    }

    testhelpers.RunTestCasesE(s.T(), s.chartPath, s.release, s.namespace, s.templates, testCases)
}
```

### 3. TestCase Fields Reference

```go
type TestCase struct {
    Skip                    bool
    Name                    string
    HelmOptionsExtraArgs    map[string][]string
    RenderTemplateExtraArgs []string
    CaseTemplates           *CaseTemplate  // override templates per case
    Template                string
    Values                  map[string]string
    ValuesFiles             []string
    Expected                map[string]string
    Verifier                func(t *testing.T, output string, err error)
    ExpectedObject          any
    ObjectAsserter          func(t *testing.T, obj any)
}
```

### 4. Golden File Test

```go
func TestGoldenDefaultsTemplateOrchestration(t *testing.T) {
    t.Parallel()

    chartPath, err := filepath.Abs("../../../")
    require.NoError(t, err)

    templateNames := []string{
        "service",
        "service-headless",
        "serviceaccount",
        "statefulset",
        "configmap",
    }

    for _, name := range templateNames {
        suite.Run(t, &utils.TemplateGoldenTest{
            ChartPath:      chartPath,
            Release:        "camunda-platform-test",
            Namespace:      "camunda-platform-" + strings.ToLower(random.UniqueId()),
            GoldenFileName: name,
            Templates:      []string{"templates/orchestration/" + name + ".yaml"},
            SetValues: map[string]string{
                "global.elasticsearch.enabled": "true",
                "elasticsearch.enabled":        "true",
            },
            IgnoredLines: []string{
                `\s+checksum/.+?:\s+.*`, // ignore configmap checksums
            },
        })
    }
}
```

### 5. Updating Golden Files

```bash
# During iteration (fast — no cleanup):
make go.update-golden-only-lite chartPath=charts/camunda-platform-8.10

# Full update (with cleanup):
make go.update-golden-only chartPath=charts/camunda-platform-8.10

# Or directly via go test flag:
cd charts/camunda-platform-8.10/test/unit
go test ./orchestration/... -update-golden
```

### 6. Running a Single Test

```bash
cd charts/camunda-platform-8.10/test/unit
go test ./orchestration/... -run TestStatefulSetTemplate
go test ./orchestration/... -run TestStatefulSetTemplate/TestContainerSetPodLabels
```

### 7. Import Grouping (gofmt order)

```go
import (
    // stdlib
    "path/filepath"
    "strings"
    "testing"

    // third-party
    "github.com/gruntwork-io/terratest/modules/helm"
    "github.com/gruntwork-io/terratest/modules/random"
    "github.com/stretchr/testify/require"
    "github.com/stretchr/testify/suite"
    appsv1 "k8s.io/api/apps/v1"
    corev1 "k8s.io/api/core/v1"

    // local
    "camunda-platform/test/unit/testhelpers"
    "camunda-platform/test/unit/utils"
)
```

### 8. Asserting on Nested Kubernetes Fields

```go
Verifier: func(t *testing.T, output string, err error) {
    var statefulSet appsv1.StatefulSet
    helm.UnmarshalK8SYaml(t, output, &statefulSet)

    containers := statefulSet.Spec.Template.Spec.Containers
    s.Require().Len(containers, 1)

    env := containers[0].Env
    envMap := make(map[string]string)
    for _, e := range env {
        envMap[e.Name] = e.Value
    }
    s.Require().Equal("C.UTF-8", envMap["LC_ALL"])
},
```

---

## Common Mistakes

1. **Using `assert` instead of `require` for fatal setup** — `assert.NoError` on `filepath.Abs` lets
   the test continue with a bad path; use `require.NoError`.

2. **Forgetting `t.Parallel()`** — omitting it serializes tests and dramatically slows CI.

3. **Committing golden files without updating dependencies** — stale sub-chart `charts/` directories
   cause checksum mismatches. Always run `make helm.dependency-update` first.

4. **Asserting on raw string output** — parsing with `helm.UnmarshalK8SYaml` and asserting on typed
   fields is more resilient than `strings.Contains(output, "foo: bar")`.

5. **Missing license header** — `make go.addlicense-check` will fail in CI. Add headers before commit.

6. **Not using `IgnoredLines` for volatile content** — checksums, chart versions, and random namespaces
   must be excluded from golden files via regex patterns in `IgnoredLines`.

7. **Suite method not starting with `Test`** — testify/suite only discovers methods prefixed with `Test`.
   Non-prefixed helpers are fine but won't run as tests.

8. **Mutating shared `Values` map** — each `TestCase` gets its own rendering, but if `Values` is a
   reference shared across cases it can cause flaky tests. Declare values inline per case.

---

## Resources

- testify/suite docs: <https://pkg.go.dev/github.com/stretchr/testify/suite>
- Terratest Helm module: <https://pkg.go.dev/github.com/gruntwork-io/terratest/modules/helm>
- Test helpers: `charts/<version>/test/unit/testhelpers/testhelpers.go`
- Golden file utility: `charts/<version>/test/unit/utils/goldenfiles.go`
- Run all tests: `make go.test chartPath=charts/camunda-platform-8.10`
- License tool: `make go.addlicense-run chartPath=charts/camunda-platform-8.10`
