# Agent Instructions — `docs/`

Applies on top of root `AGENTS.md`. Active when working inside `docs/`.

## Writing or amending an ADR

1. **Process**: `docs/maintainer-guide.md` → "Architecture Decision Records" is
   the single source for when an ADR is required, the required-table, and the
   announce/approve/implement sequence. Read it first.
2. **Structure**: copy `docs/adr/TEMPLATE.md`. Match the layout, do not invent.
3. **Style**: read at least two recent accepted ADRs before drafting (e.g.
   `0091`, `0092`) to match tone and density.
4. **Numbering / filename**: next free 4-digit ID; `NNNN-short-kebab-desc.md`.
5. **Index**: add a row to `docs/adr/index.md`; title matches the file's H1.
6. **Cross-link**: if amending or superseding, add `Amends:` / `Supersedes:`
   in the frontmatter AND link from `Context and Problem Statement`. Example
   pair: ADR 0043 + ADR 0092.
7. **Scope**: if the decision applies to a subset of chart versions or
   components, state it in `Context and Problem Statement` under an
   `Applicability by version` subsection.

If a user requests an ADR for a change that clearly does not warrant one (per
the maintainer-guide table), ask before drafting.

## PR title for ADR changes

Follow root `AGENTS.md` → "PR title type: CI-enforced constraint".

- ADR-only PR (no `charts/<version>/` files) → `chore:`.
- ADR + chart change in the same PR → use the type that fits the chart change.
