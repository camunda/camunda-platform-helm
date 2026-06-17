# GitHub Copilot — Repository Instructions

Follow the root `AGENTS.md` and the path-scoped guides in `.github/instructions/`.

## Comments

Write comments only to explain non-obvious **how**. Never add reasoning, rationale,
"why", or narration comments. Architectural rationale belongs in an ADR under `docs/adr/`,
authored by a human — never create or edit ADRs proactively. Tactical rationale (bug-fix
defaults, timeouts, label choices) goes in the PR body or commit message. Keep only required
structured comments: Apache license headers, `## @param`/`## @extra` values docs, the
`{{- /* NOTE */ -}}` helper convention, and lint/build pragmas (`//nolint`, `//go:build`,
`# yamllint disable`, `# yamllint disable-line`, `# shellcheck disable`).
