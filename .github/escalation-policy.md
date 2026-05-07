# Escalation Policy — camunda-platform-helm

This document defines when a pull request requires human review beyond the AI review
performed by `crev`. The escalation-assessor agent reads this file and computes a
composite score to determine whether human sign-off is necessary.

## Threshold

```yaml
threshold: 0.5
```

Score >= 0.5 triggers mandatory human review. Below that, AI review is sufficient
for merge approval (subject to CI passing).

## Hard Escalation Rules (NEVER violations = always escalate)

Any violation of these rules triggers immediate human-review-required regardless of
the composite score:

### Breaking Changes
- **NEVER** remove or rename an existing `values.yaml` field without a deprecation
  notice and migration path — this is a breaking API change for users.
- **NEVER** change the default value of an existing field in a way that alters runtime
  behaviour for users who do not override it.

### Security
- **NEVER** add secrets or credentials as default values in `values.yaml`.
- **NEVER** expose API keys, tokens, or passwords in ConfigMaps (must be Secrets).
- **NEVER** use mutable action tags (e.g., `@v4`) in GitHub Actions workflows — must
  be pinned to full commit SHAs.

### Chart Integrity
- **NEVER** hardcode image names or tags instead of using `camundaPlatform.imageByParams`.
- **NEVER** manually edit `values-digest.yaml`.
- **NEVER** approve changes that only apply to one chart version when the same fix is
  clearly needed across multiple versions (8.8, 8.9, 8.10).

## Deterministic Escalation Signals

These are checked automatically and contribute to the composite score:

### Golden File Mismatch (weight: 0.5)
Template files changed without corresponding golden file updates in
`test/unit/<component>/golden/`. This is a CI-blocking issue and indicates
the author may not have run the test suite.

### Cross-Version Scope (weight: 0.6)
File paths span multiple chart versions (e.g., `charts/camunda-platform-8.9/` AND
`charts/camunda-platform-8.10/`) without the PR description explicitly mentioning
cross-version changes or backporting.

### Security Surface (weight: 0.7)
Changes touch any of:
- `**/identity/**`
- `**/auth/**`
- Files containing `secret`, `token`, `password`, `credential`, `tls`, `certificate`
- `**/security*/**`
- Vault/secrets-related workflow steps

### Breaking Change Indicators (weight: 0.6)
- `values.yaml` field removals or renames (not additions)
- Helper function signature changes (`define "foo.bar"`)
- Changes to `values.schema.json` that tighten constraints

## Statistical Escalation Signals

### Novelty (weight: 0.4)
How different is this change from patterns previously reviewed in this repo?
High novelty (new subsystem, new integration pattern) warrants human attention.

### Author Familiarity (weight: 0.3)
First-time contributors or contributors who have never touched the changed
subsystem warrant additional human oversight.

### Model Uncertainty (weight: 0.5)
When the AI review itself shows signs of uncertainty (hedging language, many
specialist findings dropped by verifier, contradictory assessments), human
review provides the needed ground truth.

## AI-Only Review Acceptable When

Human review may be skipped (score < threshold) when ALL of the following hold:
- Change is purely additive (new files, new values fields with documentation)
- No NEVER rules violated
- Author has significant history in the changed subsystem
- Change pattern matches previously-reviewed similar PRs
- No security surfaces touched
- Golden files are updated
- Change is scoped to a single chart version OR explicitly describes cross-version intent
- CI passes (helm lint, unit tests, golden file check)

## Chart Design Principle Violations (always escalate at P1+)

PRs that violate the chart design principles from `docs/index.md` always require
human review because they represent architectural decisions:
- Adding opinionated integrations (bundled monitoring, hard-coded security policies)
- Exposing arbitrary/exhaustive application configuration
- Bundling external component dependencies
- Breaking the 1:1 mapping between application config and Helm values
- Working around application-level bugs in the chart
