# Rewrite CI shell scripts as structured Go CLIs with shared core library

- Status: accepted
- Date: 2025-11-18
- Decision-makers: Eamonn Moloney

## Context and Problem Statement

The CI/CD deployment pipeline for Camunda Helm charts relied on shell scripts (`deploy-camunda.sh`) to orchestrate Helm, kubectl, and Docker operations. As the number of deployment scenarios grew across chart versions, the shell scripts became difficult to test, extend, and run locally. The team needed a structured, type-safe toolchain that could serve both CI automation and developer workflows with the same code paths.

## Decision Drivers

- **Local reproducibility**: Engineers needed to deploy and debug scenarios on their own clusters using the same logic CI uses, without reverse-engineering shell scripts.
- **Testability**: The deployment shell scripts lacked unit testing; Go enables isolated testing of value preparation, kube operations, and deployment orchestration.
- **Shared abstractions**: Common operations (Docker auth, Helm invocation, kubectl wrappers, logging) were duplicated or inconsistently implemented across scripts.
- **Extensibility**: Adding new deployment flows (upgrade, multi-tenancy, OpenSearch) required a composable architecture rather than growing monolithic scripts.

## Considered Options

- **Keep the deployment shell scripts and add ShellCheck/BATS tests** — rejected because the deployment orchestration needed structured error handling and libraries shared across tools, which shell does not give.
- **Single monolithic Go binary** — rejected in favour of a multi-module approach (`camunda-core`, `camunda-deployer`, `prepare-helm-values`) to maintain separation of concerns and independent versioning.
- **Python or Node.js CLI** — rejected because the Helm chart repository is Go-native (template tests, CI tooling) and Go produces static binaries ideal for CI environments.

## Decision Outcome

The deployment pipeline was restructured into three Go modules: a shared `camunda-core` library providing Docker, Helm, Kube, and utility packages; a `camunda-deployer` CLI implementing the full deployment orchestration via Cobra; and a `prepare-helm-values` CLI for value file processing. All modules are installable locally and invoked identically in CI and on developer machines.

This decision covers the deployment pipeline only. It is not a reason to rewrite other CI shell in Go: a size limit, a complexity limit or testability alone does not justify a rewrite, and a rewrite happens only when the issue or the person who gave the task asks for it (see `.github/instructions/scripting.instructions.md`).

### Positive Consequences

- Deployment logic is unit-testable in isolation (value merging, namespace preparation, Helm argument construction).
- Shared `camunda-core` library eliminates duplication and ensures consistent behaviour across all CLI tools.
- Engineers can `go install` the deployer locally and reproduce any CI scenario without Docker-in-Docker or CI-specific environment assumptions.

### Negative Consequences

- Higher barrier to entry for contributors unfamiliar with Go module structure and Cobra CLI patterns.
- Multi-module `replace` directives required during development add friction to the local build workflow until modules are published.

## Changelog

- 2026-10-24 — [#7697](https://github.com/camunda/camunda-platform-helm/pull/7697) — Limited the decision to the deployment pipeline; testability alone no longer justifies rewriting other CI shell in Go.
