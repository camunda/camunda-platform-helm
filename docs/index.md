---
id: index
title: Overview
slug: /
---

# Camunda Helm Charts — Developer Docs

This site contains internal developer documentation for the [`camunda/camunda-platform-helm`](https://github.com/camunda/camunda-platform-helm) repository: the primary Helm chart for deploying Camunda 8 on Kubernetes.

:::note
Looking for end-user installation docs? See the [official Camunda docs](https://docs.camunda.io/docs/self-managed/about-self-managed/).
:::

## Topics

| Page | Description |
|------|-------------|
| [Release Process](./release-process) | How to cut a release, manage release candidates, and publish charts |
| [GitHub Actions Workflows](./github-actions-workflows) | Overview of every CI/CD workflow in `.github/workflows/` |
| [Contribution & Collaboration](./contribution-and-collaboration) | Branching model, PR conventions, review process |
| [Code Style](./code-style) | Helm, Go, and YAML style conventions enforced in this repo |
| [Breaking Changes & Deprecation Policy](./breaking-changes) | How breaking changes and deprecations are communicated |
| [Ticket & Label Policy](./ticket-and-label-policy) | GitHub issue/label taxonomy and triage process |

## Purpose of the Camunda Helm Chart

The Camunda Helm chart is the primary deployment mechanism for running Camunda 8 on Kubernetes. Its purpose is to abstract the complexity of Kubernetes manifests and Camunda's internal components, enabling users to perform installations and deployments with minimal Kubernetes expertise.

The chart provides a simple, reliable, and consistent way to configure and deploy Camunda while exposing only the most essential and commonly used configuration parameters.

## Guiding Principles

The Helm charts are designed with the following principles:

- Expose configuration options that are common, minimal, and useful — maintaining a balance between simplicity and flexibility.
- Ensure each exposed configuration or feature results from a conscious, user-driven decision, validated by product management.
- Allow users to define specific or advanced configuration in a clear, extensible, and generic manner outside of the chart defaults.
- Maintain a 1:1 mapping between application configuration and Helm values, wherever possible.
- Establish reasonable defaults and facilitate configuration of common deployment scenarios, avoiding unnecessary complexity.

## What the Camunda Helm Chart is NOT

The chart has defined boundaries to maintain clarity, consistency, and long-term maintainability. It is not intended to:

- Expose arbitrary or exhaustive Camunda application configuration.
- Implement or promote opinionated solutions from individual engineers. Instead, it should provide generic and composable mechanisms that enable flexible and pluggable architectures.
  - For example, it should not embed opinionated choices for monitoring, security, or identity management solutions. The chart should remain extensible through community and external tooling such as the Vertical Pod Autoscaler (VPA), security plugins, or GitOps integrations.
- Bundle or depend on external components that are not part of Camunda's core product (e.g. OpenSearch, Bitnami-based dependencies).
- Abstract away or bypass the intrinsic complexity of Camunda's architecture or its operational model.
- Serve as a mechanism to patch, override, or work around application-level issues or technical debt.
- Modify or override core application defaults unless a change aligns with Camunda's default behavior or best practices.

## Release Lifecycle

Please refer to the [Camunda Release Policy](https://camunda.com/release-policy/) to learn about the release cadence, maintenance periods, and support lifecycle.

## Contributing to These Docs

Documentation source files live in [`docs/`](https://github.com/camunda/camunda-platform-helm/tree/main/docs) at the repo root. The Docusaurus project is in [`helm-docs-site/`](https://github.com/camunda/camunda-platform-helm/tree/main/helm-docs-site).

To run the docs site locally:

```bash
cd helm-docs-site
npm install
npm start
```

See [helm-docs-site/README.md](https://github.com/camunda/camunda-platform-helm/blob/main/helm-docs-site/README.md) for full instructions.
