# GitHub Copilot — Repository Instructions

Follow the root `AGENTS.md` and the path-scoped guides in `.github/instructions/`.

## Comments

Write comments only to explain non-obvious **how**. Never add reasoning, rationale,
"why", or narration comments. Architectural rationale belongs in an ADR under `docs/adr/`,
written as a durable architectural record. Agents may draft or amend an ADR only when a human
explicitly requests it; every ADR change requires human review and approval before acceptance.
Tactical rationale (bug-fix defaults, timeouts, label choices) goes in the PR body or commit message. Keep only required
structured comments: Apache license headers, `## @param`/`## @extra` values docs, the
`{{- /* NOTE */ -}}` helper convention, and lint/build pragmas (`//nolint`, `//go:build`,
`# yamllint disable`, `# yamllint disable-line`, `# shellcheck disable`).
