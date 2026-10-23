---
applyTo: "scripts/**"
---

# Scripting — Scoped Instructions

Automation that calls external APIs, parses JSON/YAML and branches on the values, coordinates several systems (GitHub, Slack, Vault, OpsGenie, etc.), or needs retries, error classification or structured output is written in Go under `scripts/` with tests. Bash stays acceptable for a sequence of commands (`kubectl`, `helm`, `cosign`, `gh`) and for labels, annotations and paths that a workflow reader must see, while each `run:` block stays inside the limits in `github-actions.instructions.md`. Do not grow a `.sh` script into orchestration code or copy API and `jq` patterns across scripts.

When bash outgrows those limits, take the first step that is enough:

1. Shrink it: delete dead branches, merge duplicated blocks, split the step.
2. Move the logic into a Go tool that the job already runs, such as a `deploy-camunda` flag or subcommand, so the job needs no new setup step.
3. Port the rest to Go.

A port to Go:

- adds at most about twice as many production Go lines as the non-blank, non-comment bash lines it removes; the PR body states both numbers.
- adds no new Go module and no interface with only one real implementation.
- reuses `camunda-core/pkg/ghactions.Writer` for `GITHUB_OUTPUT` and `GITHUB_ENV`, `versionmatrix.CompareChartVersions` for chart versions and `camunda-core/pkg/executil` for external commands, instead of new copies.
- keeps the behaviour of the bash; a new check or input goes in a separate PR.
- has tests that pin each decision of the bash and one test that compares the Go output with the bash output for the same inputs, not tests that copy struct fields.

A non-test Go function under `scripts/` that a change touches must stay at or below cognitive complexity 30, cyclomatic complexity 30 and 250 lines. Split a function that is above a limit before you change it; there is no ignore directive. `make go.complexity` checks the change against `origin/main` (`lintBase=<ref>` to compare with another ref), and `test-release-tooling.yaml` runs the same check on every PR. `cd scripts/complexity-gate && go run . --repo-root ../.. --all` lists every function above the limits.
