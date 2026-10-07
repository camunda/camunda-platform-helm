# Support Helm CLI v3 and v4 for chart 15.x (8.10), and restrict version-gated Helm built-ins

- Status: accepted
- Date: 2026-08-18
- Decision-makers: Immanuel Monma

## Context and Problem Statement

Chart 15.x (Camunda 8.10) was made Helm CLI v4-only under the product-hub epic
[camunda/product-hub#3555](https://github.com/camunda/product-hub/issues/3555) ("Self-Managed:
Helm 4"), which set 8.9 (chart 14.x) as the last Helm-v3-supported minor and 8.10+ as Helm-v4-only,
with Helm v3 security fixes then due to end on 2026-11-11. A top-of-render guard in
`charts/camunda-platform-8.10/templates/common/constraints.tpl` failed every render on a Helm CLI
older than 4.0.0 ([#6156](https://github.com/camunda/camunda-platform-helm/pull/6156), issue
[#6137](https://github.com/camunda/camunda-platform-helm/issues/6137)), and an earlier revision of
this ADR ([#6887](https://github.com/camunda/camunda-platform-helm/pull/6887)) recorded that v4
baseline.

[camunda/self-managed-experience#29](https://github.com/camunda/self-managed-experience/issues/29)
reverses the baseline: chart 15.x supports Helm v3 and v4. Upstream Helm extended Helm v3 security
fixes to 2027-02-10 ([Helm v3 end of life](https://helm.sh/blog/helm-v3-end-of-life/)), so the
original rationale for dropping Helm v3 at 8.10 no longer holds, and users who have not moved to
Helm v4 need to install 8.10, and upgrade to it from 8.9, with Helm v3
([#7340](https://github.com/camunda/camunda-platform-helm/issues/7340)). Supporting Helm v3 again
raises two questions: which Helm v3 releases a chart line supports, and how template code avoids
depending on Helm features newer than that floor.

Charts 8.8 and 8.9 carry a helper, `camundaPlatform.toYamlPretty`
(`charts/camunda-platform-8.8/templates/common/_utilz.tpl:24-30` and
`charts/camunda-platform-8.9/templates/common/_utilz.tpl:24-30`), that exists only because
Helm's own built-in `toYamlPretty` function was introduced in Helm 3.17.0 — calling it directly
on an older 3.x CLI is a parse-time "function not defined" error, not a graceful runtime
fallback. The helper works around this with `tpl` (to defer evaluation past parse time) plus
`semverCompare ">=3.17.0" ... else toYaml`. Chart 8.10 dropped the helper when it adopted the v4
floor ([#6169](https://github.com/camunda/camunda-platform-helm/pull/6169), issue
[#6139](https://github.com/camunda/camunda-platform-helm/issues/6139)) and called `toYamlPretty`
directly in `templates/orchestration/_statefulset.tpl`, so Helm v3 releases older than 3.17.0 failed
at parse time, before any guard ran.

This is a distinct failure class from the CLI-major-version question: Helm's built-in function
surface is not stable even within v3, so any new template code that reaches for a built-in can
unknowingly introduce a floor higher than the chart's declared minimum — discovered only when a
user on an older-but-still-supported Helm CLI hits a parse error.

The same holds for Helm behavior. Helm releases before 3.10 are built with Go older than 1.18, whose
`text/template` evaluates every `and` argument instead of short-circuiting. Charts 12.x–15.x rely on
short-circuit `and` (for example `charts/camunda-platform-8.10/templates/common/_helpers.tpl:1814`),
so Helm 3.9 fails on them with nil-pointer errors, while Helm 3.10.0, the first release built with
Go 1.18, renders them ([#7340](https://github.com/camunda/camunda-platform-helm/issues/7340),
[#7341](https://github.com/camunda/camunda-platform-helm/issues/7341)). The 12.x, 13.x and 14.x READMEs
nevertheless listed Helm 3.9 as their minimum.

### Applicability by version

- Chart 15.x (8.10): Helm CLI v3 (3.10 or later) and Helm CLI v4. On Helm v3 the chart warns
  instead of failing (Decision Outcome item 1).
- Chart 14.x (8.9): officially supports Helm CLI v3.10+ and v4.
- Chart 13.x (8.8): officially supports Helm CLI v3.10+ only. Helm v4 CI is internal
  compatibility coverage and does not expand official support.
- Chart 12.x (8.7): existing Helm v3 support continues with the 3.10 floor.
- Charts 13.x–15.x support Helm v3 only until February 10, 2027. After that date, Camunda
  no longer supports Helm v3. Helm v4 is supported for the release cycles of 8.9 and 8.10.
- Chart lines after 15.x: this ADR does not set their Helm CLI baseline; that is left to a later
  decision, which [#7360](https://github.com/camunda/camunda-platform-helm/issues/7360) tracks for
  chart 16.x (8.11).
- The built-in-function restriction (Decision Outcome, item 3) applies to all currently
  maintained chart lines going forward, independent of the v3/v4 line.

## Decision Drivers

- **Helm v3 support window:** upstream Helm v3 security fixes end on 2027-02-10, extended from
  November 2026, and 3.22 is the final Helm v3 minor
  ([Helm v3 end of life](https://helm.sh/blog/helm-v3-end-of-life/),
  [#7346](https://github.com/camunda/camunda-platform-helm/issues/7346)). The v4-only baseline was
  set against the earlier date.
- **Users on Helm v3:** users who have not moved to Helm v4 need to install 8.10, and upgrade to it
  from 8.9, with Helm v3; the v4 guard blocked both
  ([#7340](https://github.com/camunda/camunda-platform-helm/issues/7340)).
- **Test/support matrix cost:** every supported Helm CLI adds CI coverage to maintain, splitting QA
  capacity across an aging and a current CLI ([#5921](https://github.com/camunda/camunda-platform-helm/issues/5921)).
- **Landmine prevention:** the `toYamlPretty` wrapper is a correct but fragile pattern (deferred
  `tpl` evaluation to dodge parse-time errors). It is non-obvious, easy to forget, and each new
  instance is normally discovered only when a user's CLI is older than a template author assumed.
- **Single, testable floor per chart line:** a chart-line-wide "requires Helm vX.Y+" statement is
  easier to state, document, and enforce than an implicit floor that varies function-by-function.

## Considered Options

- **Keep the Helm v4 floor for chart 15.x** — rejected: it rests on Helm v3 security fixes ending in
  November 2026, which upstream moved to 2027-02-10, and it blocks users who have not moved to
  Helm v4 from installing 8.10 or upgrading to it from 8.9.
- **Support Helm v3 implicitly, without a stated floor, notice, or targeted tests** — rejected: the
  implicit floor was already wrong (the 12.x, 13.x and 14.x READMEs listed Helm 3.9, which cannot render
  those charts), chart 15.x's direct `toYamlPretty` call raises it to 3.17.0, and it does nothing to
  stop new version-gated built-ins from accumulating.
- **Auto-detect and branch around every new built-in as it's introduced** — rejected: this is the
  status quo (`toYamlPretty`) and it does not scale; each occurrence is fragile, uses a
  non-obvious `tpl`-deferral trick, and is discovered reactively rather than prevented.
- **Stated, tested Helm CLI support per chart line, plus a policy against new version-gated
  built-ins (chosen)** — makes the floor explicit and testable (chart README plus the unit-test
  Helm matrix), warns Helm v3 users instead of failing, and shifts the function-availability problem
  from "wrap it" to "don't introduce it unless justified."

## Decision Outcome

Chart 15.x (8.10) supports Helm CLI v3 (3.10 or later) and Helm CLI v4, and new template code does
not depend on a Helm built-in or behavior newer than its chart line's floor. The following
constraints are normative:

1. Chart 15.x (8.10) MUST support Helm CLI v3 (3.10 or later) and Helm CLI v4. A render MUST NOT
   fail because of the Helm CLI major version. On Helm v3 the chart emits a non-blocking
   `[camunda][warning]` through `camunda.constraints.warnings`, shown in NOTES and in a ConfigMap
   whose name ends in `-warnings`, that Helm v3 security fixes end on 2027-02-10 and that users
   should upgrade to Helm v4 before then. The warning replaces the `fail` guard from
   [#6156](https://github.com/camunda/camunda-platform-helm/pull/6156)
   ([#7340](https://github.com/camunda/camunda-platform-helm/issues/7340)).
2. Charts 13.x–15.x support Helm v3 until its upstream end of life on February 10, 2027.
   After that date, Camunda no longer supports Helm v3; continued use is at the customer's own
   risk. Helm v4 is officially supported for the release cycles of 8.9 and 8.10, but not 8.8,
   consistent with the [published support announcement](https://camunda.com/blog/2026/06/camunda-8-helm-chart-update-helm-4/).
   Chart 13.x MAY test Helm v4 internally without extending official support. For future releases, its generated
   `camunda.io/helmCLIVersion`, release information, and public version-matrix entries MUST list
   only Helm v3; historical Helm v4 entries do not establish official support. Chart 12.x
   retains its documented Helm v3 support window.
   Charts 12.x (8.7), 13.x (8.8) and 14.x (8.9) document Helm 3.10 as their
   minimum, the oldest release that renders them
   ([#7341](https://github.com/camunda/camunda-platform-helm/issues/7341)), and chart 14.x emits the
   same Helm v3 warning as 15.x ([#7342](https://github.com/camunda/camunda-platform-helm/issues/7342)).
   Chart 13.x also warns on Helm v3, directing users to Camunda 8.9 or 8.10 and Helm v4 for
   an officially supported combination before the cutoff. Its Helm v4 tests do not require
   a render-time failure or warning on Helm v4.
3. New template code MUST NOT depend on a Helm built-in function or behavior introduced later
   than the oldest Helm version the chart line currently claims to support. If no alternative
   exists, the usage MUST be wrapped so the unsupported-version path either fails with a clear
   `fail` message (preferred, matching the `constraints.tpl` pattern) or falls back correctly, and
   the wrapper MUST have a unit test exercising both the supported and unsupported paths against a
   deterministic Helm-version matrix (one CLI at or above the floor, one below the relevant
   function/floor boundary) rather than whichever Helm binary happens to be on `PATH` in CI.
   `camundaPlatform.toYamlPretty` is the fallback precedent: on Helm 3.17.0 and later it calls
   `toYamlPretty` through `tpl`, and on older CLIs it falls back to `toYaml`. For charts 14.x and
   15.x, [#7344](https://github.com/camunda/camunda-platform-helm/issues/7344) adds the unit-test
   Helm matrix, which runs the Helm v3 warning test and the Orchestration StatefulSet scheduling
   tests that render through the wrapper on Helm 3.10.3 and 3.22.x, and fails when a test selector
   matches no passing test. The regular unit job runs the same tests on Helm v4.
4. When a chart line's CLI floor rises enough to make a version-gated wrapper's fallback branch
   unreachable, the wrapper MUST be removed rather than kept for symmetry. For
   `camundaPlatform.toYamlPretty` in charts 13.x–15.x, the 3.10 floor is below the wrapper's 3.17.0
   boundary, so the fallback is reachable and the wrapper stays:
   [#7340](https://github.com/camunda/camunda-platform-helm/issues/7340) reinstates it in 15.x,
   reverting its removal in [#6169](https://github.com/camunda/camunda-platform-helm/pull/6169)
   ([#6139](https://github.com/camunda/camunda-platform-helm/issues/6139)).

Applies to chart 15.x (8.10) through the sub-issues of
[camunda/self-managed-experience#29](https://github.com/camunda/self-managed-experience/issues/29):
[#7340](https://github.com/camunda/camunda-platform-helm/issues/7340) replaces the guard with the
warning and reinstates the wrapper;
[#7343](https://github.com/camunda/camunda-platform-helm/issues/7343) runs the `orchestration-tls`
install and `component-persistence-upgrade` upgrade-from-14.x integration scenarios on Helm v3, and
the rest of the 15.x CI runs Helm v4;
[#7344](https://github.com/camunda/camunda-platform-helm/issues/7344) adds the unit-test Helm matrix
for charts 14.x and 15.x; [#7345](https://github.com/camunda/camunda-platform-helm/issues/7345)
records Helm v3 and v4 in the `camunda.io/helmCLIVersion` annotation of 15.x releases; and
[#7346](https://github.com/camunda/camunda-platform-helm/issues/7346) keeps the Helm v3 CI pins on
the final v3 line, 3.22.x, through Renovate.
[#7341](https://github.com/camunda/camunda-platform-helm/issues/7341) and
[#7342](https://github.com/camunda/camunda-platform-helm/issues/7342) apply item 2 to charts 12.x, 13.x and
14.x. Item 3 is a forward rule for all chart lines from this ADR's acceptance date.

### Positive Consequences

- Users who have not moved to Helm v4 can install chart 15.x, and upgrade to it from 14.x, with
  Helm v3 ([#7340](https://github.com/camunda/camunda-platform-helm/issues/7340),
  [#7343](https://github.com/camunda/camunda-platform-helm/issues/7343)).
- Helm v3 users of charts 14.x and 15.x get a non-blocking notice that Helm v3 security fixes end on
  2027-02-10 ([#7340](https://github.com/camunda/camunda-platform-helm/issues/7340),
  [#7342](https://github.com/camunda/camunda-platform-helm/issues/7342)).
- A single, testable CLI-support statement per chart line replaces an implicit, function-by-
  function floor.
- Removes latent parse-time failure risk from template code that unknowingly assumes a newer
  Helm built-in than the chart's stated floor.
- Item 3's both-path test runs in CI: version-gated template code in charts 14.x and 15.x is tested
  on pinned Helm CLIs below and above the relevant boundary, not on whichever Helm is on `PATH`
  ([#7344](https://github.com/camunda/camunda-platform-helm/issues/7344)).

### Negative Consequences

- Helm v3 CI coverage for 15.x has to be maintained alongside Helm v4: two pinned integration scenarios and
  the unit-test Helm matrix ([#7343](https://github.com/camunda/camunda-platform-helm/issues/7343),
  [#7344](https://github.com/camunda/camunda-platform-helm/issues/7344)), with Helm v3 pins that
  Renovate keeps on the final v3 line ([#7346](https://github.com/camunda/camunda-platform-helm/issues/7346)).
- Helm v3 and v4 render the same 14.x and 15.x manifests except for blank lines, so golden files are
  byte-compared on Helm v4 only, and Helm v3 unit coverage is limited to selected tests that decode
  rendered objects ([#7344](https://github.com/camunda/camunda-platform-helm/issues/7344)).
- The warning does not block: charts 13.x–15.x still render on Helm v3 after 2027-02-10,
  but that CLI is no longer supported. This cutoff does not require removing rendering compatibility.
- Camunda 8.8 has no officially supported Helm CLI combination after 2027-02-10, even while
  its Camunda support window remains open. The supported migration path requires upgrading
  Camunda to 8.9 or 8.10 as well as moving to Helm v4. Internal Helm v4 compatibility coverage
  does not close this support gap.
- Below Helm 3.10, a render fails with a template nil-pointer error, not a message that names the
  floor ([#7340](https://github.com/camunda/camunda-platform-helm/issues/7340),
  [#7341](https://github.com/camunda/camunda-platform-helm/issues/7341)).
- The reversal contradicts published statements: the docs
  ([camunda-docs#8859](https://github.com/camunda/camunda-docs/pull/8859)), release notes, and blog
  posts said that 8.10 requires Helm v4. Their correction is tracked in
  [camunda/self-managed-experience#29](https://github.com/camunda/self-managed-experience/issues/29),
  outside this repository.
- Item 3 is a review-time policy, not a tooling-enforced one today — no lint step currently flags
  a newly introduced Helm built-in against the chart's declared floor. Reviewers must know the
  floor and check new template code against it manually until such tooling exists.

## Links

- [camunda/self-managed-experience#29](https://github.com/camunda/self-managed-experience/issues/29) — the decision to support Helm v3 and v4 in chart 15.x, and its implementation checklist across the chart, docs, and communications.
- Epic: [camunda/product-hub#3555](https://github.com/camunda/product-hub/issues/3555) — the original product decision for a Helm v4-only 8.10; camunda/self-managed-experience#29 tracks amending its release notes and validation criteria.
- [#7340](https://github.com/camunda/camunda-platform-helm/issues/7340) — chart 15.x: Helm v3 warning instead of the v4 guard, and the `toYamlPretty` wrapper reinstated.
- [#7341](https://github.com/camunda/camunda-platform-helm/issues/7341) — charts 12.x, 13.x and 14.x: documented Helm minimum corrected from 3.9 to 3.10.
- [#7342](https://github.com/camunda/camunda-platform-helm/issues/7342) — chart 14.x: the same Helm v3 warning.
- [#7343](https://github.com/camunda/camunda-platform-helm/issues/7343) — chart 15.x: Helm v3 integration scenarios.
- [#7344](https://github.com/camunda/camunda-platform-helm/issues/7344) — unit-test Helm matrix for charts 14.x and 15.x, the test harness for item 3.
- [#7345](https://github.com/camunda/camunda-platform-helm/issues/7345) — release tooling records Helm v3 and v4 for chart 15.x.
- [#7346](https://github.com/camunda/camunda-platform-helm/issues/7346) — Helm v3 CI pins on 3.22.x, updated by Renovate.
- [#6137](https://github.com/camunda/camunda-platform-helm/issues/6137) / [#6156](https://github.com/camunda/camunda-platform-helm/pull/6156) — the Helm v4 `constraints.tpl` fail guard that #7340 replaces.
- [#6139](https://github.com/camunda/camunda-platform-helm/issues/6139) / [#6169](https://github.com/camunda/camunda-platform-helm/pull/6169) — `toYamlPretty` compat wrapper removal from 8.10, the motivating example for item 3, which #7340 reverts.
- [#5921](https://github.com/camunda/camunda-platform-helm/issues/5921) — CI matrix scoping per chart line.
- [ADR-0081](0081-expose-helm-v4-compatibility-options-as-explicit-values.md) — related but distinct: opt-in Helm v4-compatible *rendering* flags for charts 8.6-8.9, not a CLI-version floor.

## Changelog

- 2026-10-07 — Clarify 8.8's Helm v3-only official support, internal Helm v4 compatibility coverage, public metadata, and the Helm v3 support cutoff for 8.8–8.10 (PR link added when opened).
