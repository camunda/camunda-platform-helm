# Rename the Helm chart "core" values key to "orchestration" to align with upstream product terminology

- Status: accepted
- Date: 2025-08-13
- Decision-makers: Ahmed AbouZaid

## Context and Problem Statement

The Camunda 8.8 platform unified Zeebe, Operate, and Tasklist into a single component and officially renamed it from "Core" to "Orchestration," reflecting its role as the process orchestration engine. The Helm chart's top-level values key remained `core`, creating a terminology mismatch between the deployment configuration, product documentation, and application APIs. This divergence would compound over time as documentation, support channels, and user mental models converged on the new name.

## Decision Drivers

- **Terminology alignment with upstream product** — the chart should use the same names as the application it deploys to reduce cognitive overhead for operators and prevent documentation drift.
- **Pre-GA window for breaking changes** — the 8.8 chart is in alpha, making this the lowest-cost moment to introduce a breaking rename before users depend on the existing key in production.
- **Maintainability** — carrying dual naming indefinitely increases template complexity and test surface area without delivering long-term value.
- **Consistency across the chart ecosystem** — charts for 8.9+ will use `orchestration` natively; aligning 8.8 now prevents version-specific naming exceptions in tooling and CI.

## Considered Options

- **Clean atomic rename (chosen)** — rename `core` → `orchestration` everywhere in a single commit, marked as a BREAKING CHANGE. Provides a clear migration boundary.
- **Indefinite dual-key support via compatibility shim** — maintain both `core` and `orchestration` keys using template-level aliasing (`z_compatibility_helpers.tpl`). Rejected because it adds permanent complexity and defers the inevitable migration cost without eliminating it.
- **Keep `core` unchanged** — rejected because it would diverge from the product's official terminology, confusing users who reference Camunda documentation and APIs that use "Orchestration."

## Decision Outcome

The Helm chart's top-level values key was renamed from `core` to `orchestration` across all templates, helpers, ingress definitions, secrets, configmaps, CI workflows, and integration test values for the 8.8 chart. A compatibility helper template was included to ease detection of stale configurations, but the change is explicitly breaking — users must update their values files upon upgrade. This establishes `orchestration` as the canonical key going forward across all chart versions.

### Positive Consequences

- Chart configuration now mirrors product naming, eliminating the translation layer between documentation, application APIs, and deployment values.
- Future charts (8.9+) inherit the correct naming from the start, avoiding repeated migration efforts.
- Operators onboarding to Camunda 8.8+ encounter consistent terminology across all touchpoints, reducing support burden.

### Negative Consequences

- **Migration friction for existing users** — anyone with customized `core` values must rename them to `orchestration` during upgrade, which is a manual and error-prone step.
- **Large blast radius** — 112 files changed atomically increases merge conflict risk with concurrent work and elevates review burden, though this is a one-time cost.