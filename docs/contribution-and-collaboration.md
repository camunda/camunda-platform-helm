---
id: contribution-and-collaboration
title: Contribution & Collaboration
---

The Camunda Helm chart is the integration point between the Camunda application components and Kubernetes. To maintain consistency, reliability, and ownership boundaries, contributions from application teams should follow a structured collaboration process. This ensures all configuration, feature, and bug-fix changes align with established review, testing, and documentation standards.

## When App Teams Should Contribute Independently

App teams are expected to contribute directly when changes primarily affect their application configuration, such as:

- Adjustments to the ConfigMap or application-specific configuration fields.
- Additions of new configuration properties to `values.yaml` or related templates.
  - **Note:** With 8.9+, applications should be configured per default via the [`<component>.extraConfiguration`](https://docs.camunda.io/docs/next/self-managed/deployment/helm/configure/application-configs/#componentnameextraconfiguration) key.
- Adding or updating secret references (for example, credential or endpoint configurations).

Larger architectural or user-facing changes, such as new components or features that affect multiple components, should always be designed and implemented in collaboration with the Distribution team.

:::note
Read the [contribution guide](https://github.com/camunda/camunda-platform-helm/blob/main/CONTRIBUTING.md) to learn how to contribute.
:::

## General Collaboration Workflow

### 1. ADR-first: Align before you implement

For any architectural change, new component, cross-team impact, or feature
with architectural footprint, an ADR is required *before* writing code or
opening a PR.

The authoritative process — required-table, MADR format, announce/approve/
implement sequence, and PDP/kickoff guidance — lives in the
**[Maintainer Guide → Architecture Decision Records](./maintainer-guide.md#architecture-decision-records-adrs)**.
Read it before drafting an ADR. App-team contributors follow the same process
as maintainers.

### 2. Determine ownership

Depending on the nature of the change, either:

- The Distro team implements the change directly.
- The app team member creates the implementation, while the Distro team acts as reviewer and owner of Helm-related concerns.

The Distro team remains the **final reviewer and approval authority** for all Helm chart modifications.

### 3. Propose an issue

Each contribution begins with a GitHub issue describing:

- Context and motivation.
- The configuration or feature change.
- Expected impact on existing values or manifests.
- Linked documentation or references (if available).
- Link to the relevant ADR (if applicable).

### 4. AI-assisted review with `crev`, then PR

Once the ADR is approved and the implementation is ready, use the following PR workflow:

1. **Open your PR as a draft.**
2. **Run [`crev`](https://github.com/camunda/crev) against your PR:**
   ```bash
   crev https://github.com/camunda/camunda-platform-helm/pull/<your-pr-number>
   ```
   `crev` automatically reads the ADR corpus in `docs/adr/`, this governance doc, and any escalation policy defined in `.github/escalation-policy.md`.
3. **Read and respond to the review findings.** Address blockers before requesting human review.
4. **Fix findings** — iterate until the AI review is clean or findings are intentionally accepted with explanation.
5. **If the review output seems off or misses context**, add clarifying context directly in the PR description for the agents. Reference the [helm-repo-specific agent context](https://github.com/camunda/crev/blob/main/cmd/crev/review.go#L733) to understand what documents `crev` uses.
6. **Mark the PR ready for review** once the crev report is satisfactory.
7. The Distro team participates during design review and functional validation stages to ensure chart consistency.

> ℹ️ `crev` detects ADRs (`docs/adr/**/*.md`), governance documents (`docs/contribution-and-collaboration.md`, `CONTRIBUTING.md`, etc.), and escalation policies. Keeping these files accurate directly improves AI review quality and reduces back-and-forth with human reviewers.

---

## Contribution Policy

### Configuration documentation

Before adding or modifying any configuration in the Helm chart, contributors must ensure the change is properly documented.

Ideally, the configuration property should already be reflected in the user documentation.
At minimum, a GitHub issue must describe the property clearly — its purpose, effect, default behavior, and any important constraints.

This requirement serves as a **hard contribution criterion**.

Self-Managed engineers review documentation clarity as part of the code review process. Through this step, reviewers also deepen understanding of how the application integrates with its Helm configuration.

:::note
Configuration values represent the main handover point between the application and the Helm chart. Clear, accurate documentation ensures maintainability and shared understanding across teams.
:::

### Helm documentation

The `values.yaml` file follows [Helm's best practices](https://helm.sh/docs/chart_best_practices/values/). This means:

- Variable names should begin with a lowercase letter, and words should be separated using camelCase.
- Every defined property in `values.yaml` should be documented.
- The documentation string should begin with the name of the property that it describes, followed by at least a one-sentence description.
- We use [bitnami/readme-generator-for-helm](https://github.com/bitnami/readme-generator-for-helm) to generate the Helm chart values docs from the values file. Ensure to follow the same convention as the tool.

### New applications: minimal requirements

To ensure consistency and operational reliability across all components shipped within the Camunda Platform Helm chart, any new application introduced into the chart must meet a set of minimal, mandatory requirements:

- **Enable/Disable flag (`enabled`):** Allows users to explicitly opt-in to running the component, avoids accidental deployments, and ensures backward compatibility in chart upgrades.
- **Environment Variable Configuration:** Allows users to configure runtime behavior without modifying template files.
  ```yaml
  <appName>:
    env: {}
  ```
- **TLS / Java Keystore (JKS) Integration:** Provides consistent TLS behavior across all chart components and ensures secure communication patterns are applied uniformly.
  ```yaml
  <appName>:
    tls:
      enabled: false
      existingSecret: ""
      jks:
        secret:
          existingSecret: ""
          existingSecretKey: ""
          inlineSecret: ""
  ```

---

## Pull Request Requirements

Every pull request (PR) related to Helm chart changes must adhere to the following checklist:

- **ADR or team alignment:** For new features, architectural changes, or direction shifts — an ADR must exist and have team approval before the PR is opened.
- **Linked issue:** Every PR must reference a clearly described GitHub issue.
- **`crev` review completed:** Run `crev` against the PR before marking it ready for review. Address or acknowledge all findings.
- **Unit tests:** Changes should include or update corresponding unit tests where applicable.
- **Documentation updates:** User or technical documentation must reflect configuration or behavior changes.
- **Passing CI:** All CI checks must pass successfully before merge.
- **Code review:** At least one formal human review must be completed and approved. **Exception:** PRs authored by a maintainer listed in [`.github/auto-approve-allowlist.txt`](https://github.com/camunda/camunda-platform-helm/blob/main/.github/auto-approve-allowlist.txt) may be approved by `distro-ci[bot]` under the team-sanctioned [trusted-author auto-approval](#trusted-author-auto-approval) control. This exception never applies to PRs that modify CI-privileged paths (`.github/workflows/`, `.github/actions/`, `CODEOWNERS`, the allowlist), a chart's public API (values.yaml, values.schema.json, constraints.tpl), or agent instruction files (`AGENTS.md`/`CLAUDE.md`/`SKILLS.md`, `.claude/`, `.github/instructions/`, `.github/copilot-instructions.md`, `.github/escalation-policy.md`) — those always require a human review.
- **Atomic changes:** Aim for small, focused PRs that address a single issue or configuration change to simplify review and reduce merge complexity.

### Automated AI review and escalation

PRs are automatically reviewed by [`crev`](https://github.com/camunda/crev), an AI code-review tool with domain-specific knowledge of the Camunda Helm charts. After review, `crev` posts:

- A **commit status** (`crev/escalation`) indicating whether human review is required.
- A **label** (`human-review-required` or `ai-review-sufficient`) based on the escalation score.

The escalation decision is governed by the [escalation policy](https://github.com/camunda/camunda-platform-helm/blob/main/.github/escalation-policy.md). PRs that touch security surfaces, span multiple chart versions, or violate hard rules always require human review. Routine additive changes by familiar authors may be approved with AI review alone.

Contributors should treat the `human-review-required` label as a signal that the Distro team must approve before merge.

### Trusted-author auto-approval

The [`Repo - Auto Approve`](https://github.com/camunda/camunda-platform-helm/blob/main/.github/workflows/repo-auto-approve.yaml) workflow lets `distro-ci[bot]` post an approving review on PRs whose author is listed in `.github/auto-approve-allowlist.txt`. This is a Distro-team-sanctioned control that satisfies the human-review requirement above for those authors, on the basis that the allowlist is a small, high-trust set curated by the team.

Guardrails:

- **Approve-only** — the bot never merges; branch protection (required status checks, merge queue) still gates merge.
- **Privileged, public-API, and agent-instruction paths always need a human** — any PR whose net diff (including renamed-from paths) touches a CI-privileged path (`.github/workflows/`, `.github/actions/`, `CODEOWNERS`, the allowlist), a chart's public API (`values.yaml`, `values.schema.json`, `constraints.tpl`; chart `test/` fixtures excluded), or an agent instruction file (`AGENTS.md`/`CLAUDE.md`/`SKILLS.md`, `.claude/`, `.github/instructions/`, `.github/copilot-instructions.md`, `.github/escalation-policy.md`) is never auto-approved. The exact set lives in [`.github/auto-approve-protected-paths.txt`](https://github.com/camunda/camunda-platform-helm/blob/main/.github/auto-approve-protected-paths.txt).
- **Renovate PRs use a narrower protected set** — `renovate[bot]`-authored PRs are auto-approved on every push but may never touch the approval trust machinery (the allowlist, the protected-path lists, the auto-approve workflow, `CODEOWNERS`), per [`.github/auto-approve-protected-paths-renovate.txt`](https://github.com/camunda/camunda-platform-helm/blob/main/.github/auto-approve-protected-paths-renovate.txt). Renovate's routine changes (workflow action pins, chart image tags) remain governed by the `renovate.json5` automerge policy.
- **Fail-closed** — if the workflow cannot determine what a PR changed, it does not approve.

Adding or removing allowlist entries requires a normal human-reviewed PR.

**Ruleset constraint:** the `main` ruleset must keep `require_last_push_approval` and `dismiss_stale_reviews_on_push` **off**. Both break the release-please and Renovate bot-merge flows (a bot approval is dismissed or disqualified by the follow-up commit those flows push), and neither adds real protection to this control: the allowlist is a subset of the ruleset's bypass actors, so an allowlisted author can already merge without review regardless.

### Helm version

To have a smooth contribution experience, before working on a new PR make sure to use the exact Helm version that's currently used in the repo.

The Helm version is set in the [`.tool-versions`](https://github.com/camunda/camunda-platform-helm/blob/main/.tool-versions) file, so you can use the [asdf version manager](https://github.com/asdf-vm/asdf) to install Helm locally or just install the same version manually.

To install the Helm version that's used in this repo using asdf, in the repo root, run:

```bash
make tools.asdf-install
```

---

## Tests

New contributions are expected to include unit tests (golden file tests for defaults/toggles, property tests for non-default values), but no integration tests. Run them with `make go.test` at the repository root; new Go test files need Apache license headers (`make go.addlicense-run`).

The full testing reference — test types, golden file vs property test guidance, examples, and license tooling — is [Testing](./reference/testing.md).

---

## Backporting Policy

Backports exist to deliver critical fixes and stability improvements to actively supported Camunda Helm chart releases, while minimizing regression risk and avoiding surprising changes for users. **Golden rule:** if upgrading to a patch release changes a user's deployment in an unexpected way, the backport failed.

The canonical policy — supported versions, the Tier 1/Tier 2 three-way logic, decision diagram, what is and is not backported, the backport-PR process, and the `format-patch` workflow for subdirectory versioning — is [Backporting Policy](./policies/backporting.md).
