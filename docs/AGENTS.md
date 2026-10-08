# Agent Instructions — `docs/`

Applies on top of root `AGENTS.md`. Active when working inside `docs/`.

## Reading ADRs

An ADR with the status `superseded by [ADR NNNN](...)` is not the current decision.
Read the linked ADR instead. To list all statuses, run `grep "^- Status:" docs/adr/0*.md`.

## Writing or editing an ADR

1. **Rules**: `docs/maintainer-guide.md` → "Architecture Decision Records" is
   the single source for when an ADR is required, what it contains, when to
   amend it in place or supersede it, and the review and approval sequence.
   Read it first.
2. **Structure**: copy `docs/adr/TEMPLATE.md`. Match the layout, do not invent.
   The template shows the `Changelog` line format.
3. **Numbering / filename**: next free 4-digit ID; `NNNN-short-kebab-desc.md`.
4. **Index**: add a row to `docs/adr/index.md`; title matches the file's H1.
5. **Scope**: if the decision applies to a subset of chart versions or
   components, state it in `Context and Problem Statement` under an
   `Applicability by version` subsection.

If a user requests an ADR for a change that clearly does not warrant one (per
the maintainer-guide table), ask before drafting.

If you notice rationale that would otherwise become a "why" code comment, surface it
to the human as a candidate ADR.

## PR title for ADR changes

Follow root `AGENTS.md` → "PR title type: CI-enforced constraint".

- ADR-only PR (no `charts/<version>/` files) → `chore:`.
- ADR + chart change in the same PR → use the type that fits the chart change.
