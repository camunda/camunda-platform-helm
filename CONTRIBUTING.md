# Contributing to the Camunda Helm chart

We welcome new contributions. We take pride in maintaining and encouraging a friendly, welcoming, and collaborative community.

Anyone is welcome to contribute to Camunda! The best way to get started is to choose an existing [issue](#starting-on-an-issue).

For community-maintained Camunda projects, please visit the [Camunda Community Hub](https://github.com/camunda-community-hub). For connectors and process blueprints, please visit [Camunda Marketplace](https://marketplace.camunda.com/en-US/home) instead.

## Table of Contents

- [Prerequisites](#prerequisites)
  - [Code of Conduct](#code-of-conduct)
- [GitHub Issue Guidelines](#github-issue-guidelines)
  - [Severity and Likelihood (bugs)](#severity-and-likelihood-bugs)
  - [Determining the severity of an issue](#determining-the-severity-of-an-issue)
  - [Starting on an issue](#starting-on-an-issue)
- [Commit Message Guidelines](#commit-message-guidelines)
  - [Commit message header](#commit-message-header)
- [CI](#ci)
- [Integration Testing](#integration-testing)

## Prerequisites

<!-- TODO: uncomment this section when we have a CLA in place -->
<!-- ### Contributor License Agreement -->
<!---->
<!-- You will be asked to sign our [Contributor License Agreement](https://cla-assistant.io/camunda-community-hub/community) when you open a Pull Request. We are not asking you to assign copyright to us but to give us the right to distribute your code without restriction. We ask this of all contributors to assure our users of the origin and continuing existence of the code. -->
<!---->
<!-- > [!NOTE] -->
<!-- > In most cases, you will only need to sign the CLA once. -->

### Code of Conduct

This project adheres to the [Camunda Code of Conduct](https://camunda.com/events/code-conduct/). By participating, you are expected to uphold this code. Please [report](https://camunda.com/events/code-conduct/reporting-violations/) unacceptable behavior as soon as possible.

## GitHub issue guidelines

If you want to report a bug or request a new feature, feel free to open a new issue on [GitHub][issues].

If you report a bug, please help speed up problem diagnosis by providing as much information as possible.

> [!NOTE]
> If you have a general usage question, please ask on the [forum][forum].

Every issue should have a meaningful name and a description that either describes:

- A new feature with details about the use case the feature would solve or
  improve
- A problem, how we can reproduce it, and what the expected behavior would be
- A change and the intention of how this would improve the system

### Severity and Likelihood (bugs)

To help us prioritize, please also determine the severity and likelihood of the bug. To help you with this, here are the definitions for the options:

Severity:

- _Low:_ Having little to no noticeable impact on usage for the user (e.g. log noise)
- _Mid:_ Having a noticeable impact on production usage, which does not lead to data loss, or for which there is a known configuration workaround.
- _High:_ Having a noticeable impact on production usage, which does not lead to data loss, but for which there is no known workaround, or the workaround is very complex. Examples include issues which lead to regular crashes and break the availability SLA.
- _Critical:_ Stop-the-world issue with a high impact that can lead to data loss (e.g. corruption, deletion, inconsistency, etc.), unauthorized privileged actions (e.g. remote code execution, data exposure, etc.), and for which there is no existing configuration workaround.
- _Unknown:_ If it's not possible to determine the severity of a bug without in-depth investigation, you can select unknown. This should be treated as high until we have enough information to triage it properly.

Likelihood:

- _Low:_ rarely observed issue/ rather unlikely edge-case
- _Mid:_ occasionally observed
- _High:_ recurring issue

#### Determining the severity of an issue

Whenever possible, please try to determine the severity of an issue to the best of your knowledge.
Only select `Unknown` if it's really difficult to tell without spending a non-negligible amount of time (e.g. >1h) to
figure it out.

### Starting on an issue

The `main` branch contains the current in-development state of the project. To work on an issue, follow these steps:

1. Check that a [GitHub issue][issues] exists for the task you want to work on.
   If one does not, create one. Refer to the [issue guidelines](#github-issue-guidelines).
2. Check that no one is already working on the issue, and make sure the team would accept a pull request for this topic. Some topics are complex and may touch multiple of [Camunda's Components](https://docs.camunda.io/docs/components/), requiring internal coordination.
3. Checkout the `main` branch and pull the latest changes.

   ```
   git checkout main
   git pull
   ```

4. Create a new branch with the naming scheme `issueId-description`.

   ```
   git checkout -b 123-adding-bpel-support
   ```

5. Implement the required changes on your branch and regularly push your
   changes to the origin so that the CI can run. Run `make go.fmt` to format
   Go code and `make go.addlicense-run chartPath=...` to add missing license
   headers before committing.

   ```
   git commit -am 'feat: add BPEL execution support'
   git push -u origin 123-adding-bpel-support
   ```

6. If you think you finished the issue, please prepare the branch for review. Please consider our [pull requests and code reviews](https://github.com/camunda/camunda/wiki/Pull-Requests-and-Code-Reviews) guide, before requesting a review. In general, the commits should be squashed into meaningful commits with a helpful message. This means cleanup/fix etc. commits should be squashed into the related commit. If you made refactorings it would be best if they are split up into another commit. Think about how a reviewer can best understand your changes. Please follow the [commit message guidelines](#commit-message-guidelines).

## Commit message guidelines

Commit messages use [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/#summary) format.

```
<header>
<BLANK LINE> (optional - mandatory with body)
<body> (optional)
<BLANK LINE> (optional - mandatory with footer)
<footer> (optional)
```

Camunda uses a GitHub Actions workflow to check your commit messages when a pull request is submitted. Please make sure to address any hints from the bot, otherwise the PR cannot be merged.

**Exception:** In some situations it is not possible to avoid having commits that violate above guidelines, e.g. when merging another PR into the branch of your PR via merge commit or when merging back a release branch. Only in those cases you should explain the motivation and add the `ci:ignore-commitlint` label to your PR to disable the commit message checks.

### Commit message header

Examples:

- `docs: add guide for external Elasticsearch`
- `perf: increase memory limit of Orchestration Cluster`
- `feat: add sidecar for Optimize`

The commit header should match the following pattern:

```
%{type}: %{description}
```

The commit header must be under 120 characters.

- `type` should be one of:
  - `build`: Changes that affect the build system (e.g. Go toolchain, Docker, Makefile)
  - `cd`: Changes to continuous delivery configuration
  - `ci`: Changes to CI configuration files and scripts (e.g. GitHub Actions)
  - `chore`: Maintenance, housekeeping, or non-functional changes
  - `deps`: A change to external dependencies
  - `docs`: A documentation change — **only valid when chart files under `charts/<version>/` are also changed**; use `chore:` for docs-only PRs
  - `feat`: A new chart feature — requires changes to user-facing chart files under `charts/<version>/`
  - `fix`: A bug fix — requires changes to user-facing chart files under `charts/<version>/`
  - `perf`: A code change that improves performance
  - `refactor`: A code change that does not change behavior — requires changes to user-facing chart files under `charts/<version>/`
  - `revert`: Reverts a previous commit — requires changes to user-facing chart files under `charts/<version>/`
  - `style`: A change to align the code with our style guide
  - `test`: Adding or correcting tests
- `description`: short description of the change in present tense

> **CI enforcement:** `feat:`, `fix:`, `refactor:`, `docs:`, and `revert:` are rejected by CI if no user-facing chart files are changed (files under `charts/<version>/` excluding `test/`, `go.mod`, `go.sum`). These types feed into release notes. For changes that only touch `docs/`, `AGENTS.md`, scripts, or CI config, use `chore:` or `ci:` instead.

## CI

CI is performed via GitHub Actions [workflow](.github/workflows).

## Integration Testing

Integration tests verify that Helm charts can be deployed to Kubernetes and that services work correctly together. **Integration tests are primarily maintained by the Camunda team** and require access to Kubernetes infrastructure.

> [!NOTE]
>
> **For community contributors:** You are not expected to run integration tests. The CI pipeline handles this automatically.

For full details on scenarios, the `deploy-camunda` CLI, and E2E test execution, see:
- [`SKILLS.md`](SKILLS.md) — deploy-camunda CLI usage, kubectl patterns, E2E test workflow
- [`docs/skills/reproducing-ci-e2e-failures.md`](docs/skills/reproducing-ci-e2e-failures.md) — reproducing CI failures locally
- [`.github/AGENTS.md`](.github/AGENTS.md) — CI/CD architecture and workflow structure

