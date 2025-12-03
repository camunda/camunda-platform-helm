---
name: Issue report
about: Create a issue report
title: "[ISSUE] <issue title>"
labels: "kind/issue"
body:
  - type: dropdown
    id: severity
    attributes:
      label: <!-- Severity -->
      description: <a href="https://github.com/camunda/camunda/blob/main/CONTRIBUTING.md#severity-and-likelihood-bugs"> What is the severity of the issue?
      options:
        - <!-- Low- -->
        - <!-- Medium- -->
        - <!-- High- -->
        - <!-- Critical- -->
        - <!-- Unknown- -->
      default: null
    validations:
      required: true
  - type: dropdown
    id: likelihood
    attributes:
      label: <!-- Likelihood -->
      description: <a href="https://github.com/camunda/camunda/blob/main/CONTRIBUTING.md#severity-and-likelihood-bugs"> How likely is it to occur?
      options:
        - <!-- Unknown_ -->
        - <!-- Low_ -->
        - <!-- Medium_ -->
        - <!-- High_ -->
      default: null
    validations:
      required: true
---

**Describe the issue:**

<!-- A clear and concise description of what the issue is. -->

**Actual behavior:**

<!-- A clear and concise description of what actually happens. -->

**Expected behavior:**

<!-- A clear and concise description of what you expected to happen. -->

**How to reproduce:**

<!--
Steps to reproduce the issue.

If possible add a minimal reproducer code sample in a new repo/branch.
-->

**Logs:**

<!-- If possible add the full logs related to the issue. -->

**Environment:**

**Please note: Without the following info, it's hard to resolve the issue and probably it will be closed.**

- Platform: <!-- [e.g. GCP, AWS, etc] -->
- Helm CLI version: <!-- [e.g. 3.10.0] -->
- Chart version: <!-- [e.g. 8.x.x] -->
- Values file: <!-- [e.g. include or link to your values file] -->
