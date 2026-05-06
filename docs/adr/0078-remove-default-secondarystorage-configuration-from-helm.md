# Remove default secondaryStorage configuration from Helm chart to require explicit opt-in

- Status: accepted
- Date: 2026-01-05
- Decision-makers: Jesse Simpson

## Context and Problem Statement

The Camunda Platform 8.9 Helm chart shipped with a default `secondaryStorage` configuration that was automatically applied to all deployments, regardless of whether users intended to use a secondary storage backend. This violated the principle of least surprise — users who never requested secondary storage were receiving one implicitly, creating potential for misconfiguration, unexpected resource consumption, and operational confusion in production environments.

## Decision Drivers

- **Principle of least surprise** — production Helm charts should not silently configure infrastructure backends that users did not explicitly request
- **Correctness over convenience** — reducing misconfiguration risk in production deployments outweighs the convenience of a pre-wired default
- **Explicit configuration as policy** — aligning with Helm community best practices where optional subsystems require deliberate opt-in
- **Maintainability** — removing implicit coupling between the chart's baseline behavior and an optional storage backend simplifies reasoning about deployed state

## Considered Options

- **Keep the default but make it a no-op** — rejected because it still leaves configuration surface area that implies secondary storage is expected, confusing operators inspecting rendered manifests
- **Deprecation warnings before removal** — rejected because the implicit default was considered actively harmful (silently provisioning unneeded infrastructure), warranting immediate correction over a gradual cycle
- **Immediate removal as a breaking change** — chosen, signaled via conventional commit `fix!:` to ensure upgrade tooling and changelogs surface the change prominently

## Decision Outcome

The default `secondaryStorage` block was removed from the chart's base values, making secondary storage a purely opt-in feature. Users who need secondary storage must now explicitly declare it in their values overrides. This structurally decouples the chart's minimal deployment footprint from optional storage backends.

### Positive Consequences

- Deployments are now explicit about their storage topology — what you configure is what you get, with no hidden backends
- Reduced blast radius for upgrades — users who never used secondary storage are unaffected by future changes to that subsystem
- Clearer contract between chart maintainers and consumers — optional features are visibly optional in the values schema

### Negative Consequences

- **Breaking change for existing users** — anyone relying on the implicit default must add explicit `secondaryStorage` configuration during upgrade, creating a one-time migration burden
- **High test churn** — 56 files required updates due to tight coupling between default values and test fixtures, revealing (and partially addressing) over-reliance on rendered defaults in the test harness