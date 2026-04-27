---
id: breaking-changes
title: Breaking Changes & Deprecation Policy
---

## Definition of a breaking change

A breaking change is any change that breaks a previously stable interface/contract and requires user action to upgrade.

### Helm chart "Public API"

For the Camunda Helm chart, _the stable interface is `values.yaml`_: user-provided values are rendered by Helm templates into Kubernetes manifests. A change is breaking if it changes how existing `values.yaml` inputs map to rendered manifests in a way that is not backward compatible.

### Classifying changes (examples)

- **Feature:** Add a new optional values.yaml field that does not change existing behavior.
  - _Example_: add an optional metrics section or a new env-var toggle.
- **Bug:** A defect that forces users to apply mitigations (overrides/workarounds) to get expected behavior.
  - _Example_: users must set a specific env var or template override to avoid incorrect behavior.
- **Breaking change:** Remove/rename an existing field, or change its meaning/defaults such that upgrades require user updates.
  - _Example_: deprecate a values.yaml key without backward compatibility.
- **Unstructured override interaction:** Changes involving unstructured fields (e.g., `<component>.env` or `<component>.extraConfig`) are typically not breaking API changes to the chart, but can cause upgrade issues due to user-specific overrides; treat resulting incompatibilities as bugs (see [Unstructured fields](#unstructured-fields)).
  - _Example_: a user-set env var workaround in `operate.env` conflicts after an upgrade because the underlying behavior changed.

## Breaking Change Policy

**Default rule:** Breaking changes are **not allowed in released (non-alpha) patch updates** (e.g., app versions releases `8.9.1 → 8.9.2`). Changes to the `values.yaml` schema, configuration structure, or rendering logic must be backward compatible.

- _Exception:_ A breaking change in a released patch update is allowed only for a critical fix (major bug or security issue) where not changing would leave users severely broken or insecure.

### Allowed cases

- **Alpha charts:** Breaking changes are allowed in alpha Helm charts (e.g., `8.9-alpha3 → 8.9-alpha4`). Alpha releases are for development/testing and do not guarantee stability.
- **Minor app versions:** Breaking changes are allowed between minor app versions (e.g., `8.8 → 8.9`) only when justifiable, and must follow the **Breaking change checklist**.

## Breaking change checklist

### Evaluate and justify breaking change

A breaking change between minor app versions is justifiable only if all of the following are true:

1. **No reasonable compatible approach:** A backward-compatible design exists only with disproportionate complexity, risk, or long-term maintenance cost (e.g., supporting two schemas/behaviors would be fragile, delay delivery significantly, or create ongoing upgrade/support burden).
2. **Necessary outcome:** The change is required to deliver at least one of:
   - a critical fix (security / severe malfunction), or
   - a correctness fix that prevents wrong/unsafe deployments, or
   - an essential architectural/maintainability improvement that otherwise blocks future work.
3. **Customer impact is acceptable and understood:** Impacted users can be identified/estimated, and the migration is practical (clear steps; no "guessing" required).
4. **Migration is documented:** Exact migration steps exist and can be included in docs/release notes/upgrade guide.

If a breaking change is not justifiable, follow the [Deprecation Policy](#deprecation-policy).

#### Approval / alignment

1. Document and align with EM and PM justification of the breaking change (rationale, scope, customer impact, target version).
2. Mark the change as breaking in the issue and PR (label `breaking-change` + clear description).
3. Notify and align with affected stakeholder teams (InfraEx, QA, infra, Reliability Team).
4. Proceed only once EM/PM approved and stakeholder teams explicitly acknowledged.

#### Documentation

5. Prepare and publish:
   - Add docs about what changed (breaking behavior) and required customer actions.
   - Add release notes (breaking change callout).
   - Add release announcements (link to docs/migration guidance).
   - Add upgrade guide (steps + links to docs).

#### Merge and final internal communication

- Merge PR(s).
- Broadcast internally to [#engineering](https://camunda.slack.com/archives/C01H4NG9XDY) and, after alignment with PM, in [#team-tech-gtm](https://camunda.slack.com/archives/C03N6LM5QVD).

## Deprecation Policy

If a breaking change is not justifiable, follow this policy: deprecate first, remove in the next major chart release.

_Default rule:_ Within the same major chart version, customers must not be forced to change values immediately. Deprecations may be introduced in minor releases; removals must only happen in the next major chart release (or via an explicitly announced exception; see [Breaking Change Policy](#breaking-change-policy)).

Major chart releases follow Camunda's release cycle.

### Deprecation Checklist

1. **Deprecate (keep compatibility):**
   - Add a `values.yaml` comment: `DEPRECATED since vX.Y.Z; remove in vNextMajor; use instead: ...`.
   - On install/upgrade, warn if the deprecated key is set (old key, replacement, removal version).
   - If both old and new are set: new wins + warning.

2. **Document:**
   - Release notes: "Deprecations" entry (old → new, removal version).
   - Upgrade guide: migration steps (before/after) + removal version.
   - Examples: use new format only.

3. **Communicate:**
   - Notify relevant channels/teams (InfraEx, QA, infra, Reliability Team).

4. **Remove (next major only):**
   - Follow the [Breaking change checklist](#breaking-change-checklist) (after the justification step).

## Reference

### Unstructured fields

A more nuanced discussion arises with unstructured fields, such as `<component>.env`. These allow users to inject arbitrary environment variables into the Camunda platform components (e.g., Operate, Tasklist, or Zeebe). Because these overrides fall outside Helm's awareness or validation scope, they introduce a level of variability that can affect upgrade stability. For example, if a user overrides an environment variable to work around a previous limitation and a subsequent Helm chart update or upstream Camunda release resolves that limitation internally, the existing override may conflict with the new behavior.

From a versioning and stability standpoint, this scenario does not constitute a breaking change in the Helm chart's API. The core capability — the ability to define environment variable overrides — remains stable. However, the functional outcome in the application layer may differ, and if that behavior deviates from expectations, it should be treated as a bug, not a breaking API change.

Framing stability in this way helps maintain consistency in the Camunda Helm chart release process, ensuring that users can rely on `values.yaml` as the stable interface while understanding the inherent risks of unstructured overrides and Helm's limitations in validating them.
