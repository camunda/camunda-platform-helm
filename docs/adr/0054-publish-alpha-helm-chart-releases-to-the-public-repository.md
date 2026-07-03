# Publish alpha Helm chart releases to the public repository

- Status: accepted
- Date: 2025-03-14
- Decision-makers: Ahmed AbouZaid

## Context and Problem Statement

Alpha versions of the Camunda Platform Helm charts were previously distributed through a private/internal channel, requiring external users and partners to request separate repository access for early testing. This created friction for community adoption of pre-release features and added operational overhead in maintaining a distinct distribution path for alpha artifacts.

## Decision Drivers

- **Reduced user friction** — external users and partners need access to alpha charts without requesting private repo credentials or configuring additional Helm repositories
- **Operational simplicity** — maintaining separate release pipelines and repositories for different chart maturity levels increases CI/CD complexity and maintenance burden
- **Community testing velocity** — earlier and broader access to alpha charts accelerates feedback loops before stable releases
- **Consistent distribution model** — a single public repository as the canonical source for all chart lifecycle stages simplifies documentation and tooling

## Considered Options

- **Keep alphas in a private registry (status quo)** — rejected because it gates early access behind manual provisioning and fragments the user experience across repositories
- **Publish to a separate public "pre-release" repository** — rejected because it still requires users to add a second Helm repo and doubles the release infrastructure to maintain
- **Unified public repository with naming conventions** — selected as the approach that minimizes both user and operator complexity

## Decision Outcome

Alpha charts are now released through the same public Helm repository and CI/CD pipeline as stable charts, distinguished by explicit naming (`camunda-platform-alpha`, `camunda-platform-alpha-8.8`). The chart release workflow, matrix generation, and release-please configurations were unified to handle both stable and alpha lifecycle stages in a single pipeline.

### Positive Consequences

- Single Helm repository for all consumers eliminates the need for separate access provisioning and reduces onboarding friction for alpha testers
- Unified release pipeline reduces duplication in CI/CD configuration and ensures alpha charts receive the same release quality gates as stable charts
- Clear naming convention (`-alpha` suffix) establishes a scalable pattern for future pre-release channels without repository proliferation

### Negative Consequences

- Risk of accidental production consumption of unstable alpha charts if users do not observe the naming convention — requires ongoing documentation and potentially admission control guidance
- Increased complexity in the release pipeline, which must now handle multiple chart lifecycle stages, version matrices, and release-please configurations within a single workflow