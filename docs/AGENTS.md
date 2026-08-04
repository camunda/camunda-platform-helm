# Agent Instructions — `docs/`

Applies on top of root `AGENTS.md`. Active when working inside `docs/`.

## Writing or editing an ADR

1. **Process**: `docs/maintainer-guide.md` → "Architecture Decision Records" is
   the single source for when an ADR is required, the required-table, and the
   announce/approve/implement sequence. Read it first.
2. **Structure**: copy `docs/adr/TEMPLATE.md`. Match the layout, do not invent.
3. **Style**: read at least two recent accepted ADRs before drafting (e.g.
   `0091`, `0092`) to match tone and density.
4. **Numbering / filename**: next free 4-digit ID; `NNNN-short-kebab-desc.md`.
5. **Index**: add a row to `docs/adr/index.md`; title matches the file's H1.
6. **Cross-link**: when a new ADR changes constraints or conclusions, add
   `Amends:` in the frontmatter. When it replaces a decision, add `Supersedes:`.
   In both cases, link the earlier ADR from `Context and Problem Statement`.
   Example pair: ADR 0043 + ADR 0092.
7. **Scope**: if the decision applies to a subset of chart versions or
   components, state it in `Context and Problem Statement` under an
   `Applicability by version` subsection.

If a user requests an ADR for a change that clearly does not warrant one (per
the maintainer-guide table), ask before drafting.

Agents may draft or edit an ADR only when a human explicitly asks. Every ADR change
must be reviewed and approved by human maintainers and affected stakeholders before
acceptance; agent-generated text is never approval. Write ADRs for durable architectural
decisions and long-lived constraints, not transient implementation details or tactical fixes.
Edit an accepted ADR in place only for non-semantic corrections. Create a new, linked ADR
when constraints, conclusions, or the decision materially change.

If you notice rationale that would otherwise become a "why" code comment, surface it
to the human as a candidate ADR. Don't create or edit an ADR without that explicit request.

## PR title for ADR changes

Follow root `AGENTS.md` → "PR title type: CI-enforced constraint".

- ADR-only PR (no `charts/<version>/` files) → `chore:`.
- ADR + chart change in the same PR → use the type that fits the chart change.
