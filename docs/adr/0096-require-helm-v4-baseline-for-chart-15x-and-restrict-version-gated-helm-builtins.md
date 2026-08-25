# Require Helm v4 baseline for chart 15.x (8.10) and later, and restrict version-gated Helm built-ins

- Status: proposed
- Date: 2026-08-18
- Decision-makers: Immanuel Monma

## Context and Problem Statement

Chart 15.x (Camunda 8.10) already ships a top-of-render guard
(`charts/camunda-platform-8.10/templates/common/constraints.tpl:9-10`) that fails
`helm template`/`helm install` when `.Capabilities.HelmVersion.Version` is below `4.0.0`.
This was implemented in [#6156](https://github.com/camunda/camunda-platform-helm/pull/6156)
(issue [#6137](https://github.com/camunda/camunda-platform-helm/issues/6137)) as part of the
product-hub epic [camunda/product-hub#3555](https://github.com/camunda/product-hub/issues/3555)
("Self-Managed: Helm 4"), which sets 8.9 (chart 14.x) as the last Helm-v3-supported minor and
8.10+ as Helm-v4-only. That epic records the product/business decision and customer-facing
commitments; no repo-level ADR captures the chart-engineering decision or the precedent it sets.

Separately, charts 8.8 and 8.9 carry a helper, `camundaPlatform.toYamlPretty`
(`charts/camunda-platform-8.8/templates/common/_utilz.tpl:24-30` and
`charts/camunda-platform-8.9/templates/common/_utilz.tpl:24-30`), that exists only because
Helm's own built-in `toYamlPretty` function was introduced in Helm 3.17.0 — calling it directly
on an older 3.x CLI is a parse-time "function not defined" error, not a graceful runtime
fallback. The helper works around this with `tpl` (to defer evaluation past parse time) plus
`semverCompare ">=3.17.0" ... else toYaml`. Chart 8.10 dropped the helper entirely once the v4
floor made the fallback branch unreachable ([#6139](https://github.com/camunda/camunda-platform-helm/issues/6139)).

This is a distinct failure class from the CLI-major-version question: Helm's built-in function
surface is not stable even within v3, so any new template code that reaches for a built-in can
unknowingly introduce a floor higher than the chart's declared minimum — discovered only when a
user on an older-but-still-supported Helm CLI hits a parse error.

### Applicability by version

- Chart 15.x (8.10) and later: Helm CLI **must** be v4.0.0 or later. Enforced today via the
  `constraints.tpl` guard.
- Chart 14.x (8.9) and earlier: existing Helm v3 support continues per the epic's stated
  deprecation window (v3 bug fixes end 2026-07-08, security fixes end 2026-11-11). Per
  [#5921](https://github.com/camunda/camunda-platform-helm/issues/5921), 8.8 and earlier test
  Helm v3 only, 8.9 tests both v3 and v4, 8.10+ tests v4 only.
- The built-in-function restriction (Decision Outcome, item 3) applies to all currently
  maintained chart lines going forward, independent of the v3/v4 line.

## Decision Drivers

- **Helm v3 EOL:** upstream bug fixes stop 2026-07-08, security fixes stop 2026-11-11 — shipping
  new chart lines that depend on a soon-unpatched CLI conflicts with the "enterprise-grade"
  positioning stated in the epic.
- **Test/support matrix cost:** maintaining parallel v3/v4 CI coverage indefinitely splits QA
  capacity across an aging and a current CLI ([#5921](https://github.com/camunda/camunda-platform-helm/issues/5921)).
- **Landmine prevention:** the `toYamlPretty` wrapper is a correct but fragile pattern (deferred
  `tpl` evaluation to dodge parse-time errors). It is non-obvious, easy to forget, and each new
  instance is normally discovered only when a user's CLI is older than a template author assumed.
- **Single, testable floor per chart line:** a chart-line-wide "requires Helm vX.Y+" statement is
  easier to state, document, and enforce than an implicit floor that varies function-by-function.

## Considered Options

- **Continue implicit dual v3/v4 support indefinitely** — rejected: rides past upstream EOL,
  keeps the CI matrix doubled, and does nothing to stop new version-gated built-ins from
  accumulating.
- **Auto-detect and branch around every new built-in as it's introduced** — rejected: this is the
  status quo (`toYamlPretty`) and it does not scale; each occurrence is fragile, uses a
  non-obvious `tpl`-deferral trick, and is discovered reactively rather than prevented.
- **Hard CLI-version floor per chart line, plus a policy against new version-gated built-ins
  (chosen)** — makes the floor explicit and testable (`constraints.tpl` + unit test), and shifts
  the function-availability problem from "wrap it" to "don't introduce it unless justified."

## Decision Outcome

1. Chart 15.x (8.10) and later MUST require Helm CLI >= 4.0.0. A top-of-render `fail` guard for
   this is already implemented in `constraints.tpl` and covered by
   `charts/camunda-platform-8.10/test/unit/common/constraints_test.go`, verified against the
   supported predecessor CLI, Helm 3.20.2. Chart 8.10 also calls the built-in `toYamlPretty`
   directly (`templates/orchestration/statefulset.yaml`) with no version guard; on a Helm CLI
   older than 3.17.0 this fails at template-parse time with `function "toYamlPretty" not defined`
   before the guard ever executes, not with the guard's intended v4 message. This is a known gap
   in the current implementation, not a claim that the guard covers the full `<4.0.0` range.
2. Chart 14.x (8.9) and earlier MAY continue to support Helm v3 for the remainder of its
   documented support window; no retroactive floor change.
3. New template code MUST NOT depend on a Helm built-in function or behavior introduced later
   than the oldest Helm version the chart line currently claims to support. If no alternative
   exists, the usage MUST be wrapped so the unsupported-version path either fails with a clear
   `fail` message (preferred, matching the `constraints.tpl` pattern) or falls back correctly, and
   the wrapper MUST have a unit test exercising both the supported and unsupported paths against a
   deterministic Helm-version matrix (one CLI at or above the floor, one below the relevant
   function/floor boundary) rather than whichever Helm binary happens to be on `PATH` in CI.
   `constraints_test.go` does not yet demonstrate this end-to-end — CI currently runs it only
   against the repository's pinned Helm 4 CLI — so it is cited here as the guard-pattern precedent
   to follow, not as an existing both-path test.
4. When a chart line's CLI floor rises enough to make a version-gated wrapper's fallback branch
   unreachable, the wrapper MUST be removed rather than kept for symmetry (as done for
   `toYamlPretty` in 8.10 — [#6139](https://github.com/camunda/camunda-platform-helm/issues/6139)).

Applies first to chart 15.x (8.10), landed via [#6156](https://github.com/camunda/camunda-platform-helm/pull/6156)
and [#6139](https://github.com/camunda/camunda-platform-helm/issues/6139). Item 3 is a forward
rule for all chart lines from this ADR's acceptance date.

### Positive Consequences

- A single, testable CLI-support statement per chart line replaces an implicit, function-by-
  function floor.
- Removes latent parse-time failure risk from template code that unknowingly assumes a newer
  Helm built-in than the chart's stated floor.
- Shrinks the CI matrix for 8.10+ to a single Helm CLI version ([#5921](https://github.com/camunda/camunda-platform-helm/issues/5921)).

### Negative Consequences

- Real breaking change for 8.10+ adopters still on Helm v3; requires the migration guide and
  comms tracked in [camunda-docs#8846](https://github.com/camunda/camunda-docs/issues/8846) and
  the epic's release-notes/blog-post commitments.
- Item 3 is a review-time policy, not a tooling-enforced one today — no lint step currently flags
  a newly introduced Helm built-in against the chart's declared floor. Reviewers must know the
  floor and check new template code against it manually until such tooling exists.

## Links

- Epic: [camunda/product-hub#3555](https://github.com/camunda/product-hub/issues/3555) — product/business decision and customer-facing commitments this ADR's item 1-2 records.
- [#6137](https://github.com/camunda/camunda-platform-helm/issues/6137) / [#6156](https://github.com/camunda/camunda-platform-helm/pull/6156) — the `constraints.tpl` fail guard.
- [#6139](https://github.com/camunda/camunda-platform-helm/issues/6139) — `toYamlPretty` compat wrapper removal, the motivating example for item 3.
- [#5921](https://github.com/camunda/camunda-platform-helm/issues/5921) — CI matrix scoping per chart line.
- [ADR-0081](0081-expose-helm-v4-compatibility-options-as-explicit-values.md) — related but distinct: opt-in Helm v4-compatible *rendering* flags for charts 8.6-8.9, not a CLI-version floor.
