# Agent Instructions — `docs/`

Applies on top of root `AGENTS.md`. Active when working inside `docs/`.

## Reading ADRs

An ADR with the status `superseded by [ADR NNNN](...)` is not the current decision.
Read the linked ADR instead. To list all statuses, run `grep "^- Status:" docs/adr/0*.md`.

## Writing or editing an ADR

1. **Process**: `docs/maintainer-guide.md` → "Architecture Decision Records" is
   the single source for when an ADR is required, the required-table, and the
   announce/approve/implement sequence. Read it first.
2. **Structure**: copy `docs/adr/TEMPLATE.md`. Match the layout, do not invent.
3. **Numbering / filename**: next free 4-digit ID; `NNNN-short-kebab-desc.md`.
4. **Index**: add a row to `docs/adr/index.md`; title matches the file's H1.
5. **Content**: write the decision, not the implementation. Do not reference
   files by line number. Put PR references only in the `Changelog` section.
6. **Amend in place**: when constraints or conclusions change, edit the ADR.
   Do not create a new ADR to amend it. Write the body as the current decision,
   without history. Add one line to the `Changelog` section at the end of the
   ADR: `- YYYY-MM-DD — [#NNNN](PR URL) — What changed.` Create the section if
   it does not exist. Do not add a line for typo or format fixes.
7. **Supersede**: create a new ADR only when a new decision replaces the ADR
   completely. Add `Supersedes:` to the new ADR. Set the status of the earlier
   ADR to `superseded by [ADR NNNN](NNNN-slug.md)`.
8. **Scope**: if the decision applies to a subset of chart versions or
   components, state it in `Context and Problem Statement` under an
   `Applicability by version` subsection.

If a user requests an ADR for a change that clearly does not warrant one (per
the maintainer-guide table), ask before drafting.

Agents may draft or edit an ADR only when a human explicitly asks. Every ADR change
must be reviewed and approved by human maintainers and affected stakeholders before
acceptance; agent-generated text is never approval. Write ADRs for durable architectural
decisions and long-lived constraints, not transient implementation details or tactical fixes.

If you notice rationale that would otherwise become a "why" code comment, surface it
to the human as a candidate ADR. Don't create or edit an ADR without that explicit request.

## PR title for ADR changes

Follow root `AGENTS.md` → "PR title type: CI-enforced constraint".

- ADR-only PR (no `charts/<version>/` files) → `chore:`.
- ADR + chart change in the same PR → use the type that fits the chart change.
