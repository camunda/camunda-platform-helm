---
id: ticket-and-label-policy
title: Ticket & Label Policy
---

## 1. Ticket Creation Policy

- Every Pull Request (PR) **must** be linked to a ticket.
  - Preferred: use GitHub keywords in the PR description (e.g. "Closes #123", "Relates to #123").
  - Exception (automation): for PRs created by automation (e.g. Renovate, `release-please`), the PR itself can serve as the tracking item if it carries the appropriate `automation/*` label.
- Tickets are managed in one of the following systems:
  - **Camunda Product Hub**
  - **Internal Team Board**

## 2. Scope Definition: Product Hub vs. Internal Team Board

### Use **Camunda Product Hub** when the work:

- Delivers significant customer-facing value.
- Requires customer discovery and/or customer feedback inputs.
- Requires coordination across multiple teams or stakeholders.
- Introduces or changes public APIs or core platform components.
- Has cross-team dependencies.

> Product Hub epics must be created by PMs.

### Use the **Internal Team Board** when the work:

- Is limited in scope and owned by our team.
- Can be delivered without broader coordination.
- Covers internal improvements such as bug fixes, refactoring, or technical debt.
- Represents implementation tasks or subtasks linked to a Product Hub epic.

---

## 3. Camunda Platform Helm Label Policy (Internal Team Board)

This label policy applies to tickets tracked on the **Internal Team Board**. Product Hub epics follow ProductBoard/PDP conventions.

### 3.1. `kind/`

Use exactly **one** `kind/*` label per ticket indicating the kind of work.

- **`kind/bug`:** A defect or regression.
  - Requires:
    - **`affects/x.y`:** Affected version(s) (e.g. `affects/8.9`).
    - **`severity/low|mid|high|critical`:** User impact.
    - **`likelihood/low|mid|high`:** How likely it is to occur.
- **`kind/feature`:** New or improved behavior and enhancements.
  - Requires:
    - **`P0|P1|P2|P3`:** Priority.
    - **`size/xs|s|m|l`:** Effort estimate.
- **`kind/chore`:** Maintenance work (toil, cleanup, refactoring, tech debt, upgrades, housekeeping).
- **`kind/research`:** Investigation work (research/spike/PoC/proposal/design to reduce uncertainty before implementation).
- **`kind/internal`:** General internal work and improvements (not customer-facing).

### 3.2. Domain

**A) `area/*`**

Use `area/*` to indicate the component or process the ticket relates to.

Examples:

- `area/ci`
- `area/release`
- `area/security`
- `area/docs`
- `area/test`
- `area/ux`

### 3.3. Ticket Status & Workflow

These labels make it easy to triage, route, and track work consistently (what it is, how it relates to other work, and whether it needs attention).

#### 3.3.1. Ticket category

- **`support`:** Support-related tickets — used for report automation.
  - Requires:
    - **`version/x.y`:** Fixed version in question.
    - **support ticket link** in the body of the ticket.
- **`medic`:** Ticket for the medic to look into.
- **`epic`:** Epic-level tracking ticket on the Internal Team Board.
  - Use GitHub issue linking to relate work (child issues should link back to the epic).
  - If it's not an epic, it's a task.
- **`target/x.y[-suffix]`:** Intended release/milestone for completion (use when needed), e.g. `target/8.9-alpha4`.

#### 3.3.2. Status & flags

- **`needs-info`:** Needs more information, e.g. a question/discussion (not actionable work yet).
- **`good first issue`:** Good for new contributors.
- **`deprecated/8.x`:** Track work that gets deprecated in version `8.x`.
- **`breaking change`:** Marks a breaking change that needs special attention and communication.

### 3.4. Epic/Task Structure

Use GitHub's issue linking to model epics and their work items.

- Label the parent tracking ticket as **`epic`**.
- Issues with no epic are tasks or subtasks by default.

### 3.5. Best Practices

- Always apply exactly one `kind/*` label.
- Add `area/*`, ticket status, and workflow labels when they help automation and filtering.
- Keep label meanings stable; if you introduce a new label, document it here.
