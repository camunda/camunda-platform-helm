---
id: maintainer-guide
title: Maintainer Guide
---

This guide is for **Distro team members and HC owners** — those who review PRs, author ADRs, and own chart releases. All general contribution requirements in [Contribution & Collaboration](./contribution-and-collaboration.md) apply equally to maintainers.

---

## Roles

**HC owners** are the designated decision-makers for chart architecture and direction. **Distro team members** are the reviewers and release owners for all chart modifications. The authoritative list of both is in [`.github/CODEOWNERS`](https://github.com/camunda/camunda-platform-helm/blob/main/.github/CODEOWNERS).

---

## Architecture Decision Records (ADRs)

ADRs are required for architectural decisions. Features and config additions do not require an ADR — a feature request issue is sufficient.

| Change type | ADR required? |
|---|---|
| Architectural or structural change | Yes |
| Directional / cross-team impact | Yes |
| New component added to the chart | Yes |
| New feature or config addition | No — feature request issue instead |
| Bug fix / documentation update | No |

### ADR process

1. Open an ADR in [`docs/adr/`](https://github.com/camunda/camunda-platform-helm/tree/main/docs/adr) using the [MADR format](https://adr.github.io/madr/).
2. Announce in the relevant Slack channel so the team can review async.
3. Wait for majority approval from the Distribution team and any affected stakeholders.
4. Implement, then open a PR referencing the ADR.

:::note
ADRs in `docs/adr/` are automatically detected by [`crev`](https://github.com/camunda/crev) during AI-assisted review. A well-written ADR directly improves the quality of automated review feedback and reduces back-and-forth with human reviewers.
:::

### PDP / Kickoff

Major changes with cross-team impact require a design discussion (PDP or kickoff meeting) to define scope, clarify responsibilities, and ensure alignment with existing Helm and Kubernetes design. This complements the ADR — it is not a substitute.

:::note
We are currently exploring how to adapt this process as part of Camunda's AI-first operating model. Until a new process is defined, the intention of the PDP/kickoff step should be followed.
:::

---

## PR Review

### Ownership

The Distro team is the **final reviewer and approval authority** for all Helm chart modifications.

For app team PRs:
- **Tier 2, well-scoped changes**: the app team implements; the Distro team reviews and approves. See [Values YAML Policy](./policies/values-yaml-policy.md) for Tier 2 definition.
- **Larger or architectural changes**: Distro may take over implementation after ADR approval, or co-author with the app team.

### crev — automated AI review

PRs should be reviewed by [`crev`](https://github.com/camunda/crev), an AI code-review tool with domain-specific knowledge of the Camunda Helm charts. After reviewing, `crev` posts:

- A **commit status** (`crev/escalation`) indicating whether human review is required.
- A **label** (`human-review-required` or `ai-review-sufficient`) based on the escalation score.

Run `crev` against the PR before marking it ready for review:

```bash
crev https://github.com/camunda/camunda-platform-helm/pull/<pr-number>
```

Maintainers use the `crev/escalation` status and label to triage incoming PRs. The escalation decision is governed by the [escalation policy](https://github.com/camunda/camunda-platform-helm/blob/main/.github/escalation-policy.md). PRs that touch security surfaces, span multiple chart versions, or violate hard rules always require human review. Routine additive changes by familiar authors may be approved with AI review alone.

Treat the `human-review-required` label as a signal that the Distro team must approve before merge.

:::note
Keeping `docs/adr/`, this guide, `docs/contribution-and-collaboration.md`, and `CONTRIBUTING.md` accurate directly improves `crev` review quality — it uses this corpus as context when reviewing PRs.
:::

---

## Shared Requirements

The following policies apply equally to maintainer-authored changes:

| Topic | Reference |
|---|---|
| PR requirements | [Contribution & Collaboration — PR Requirements](./contribution-and-collaboration.md#pull-request-requirements) |
| Tests | [Testing Guide](./reference/testing.md) |
| Backporting | [Backporting Policy](./policies/backporting.md) |
| Values key classification | [Values YAML Policy](./policies/values-yaml-policy.md) |
| Breaking changes | [Breaking Changes Policy](./policies/breaking-changes.md) |
