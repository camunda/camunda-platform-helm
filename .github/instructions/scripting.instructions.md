---
applyTo: "scripts/**"
---

# Scripting — Scoped Instructions

Bash stays acceptable for a sequence of commands (`kubectl`, `helm`, `cosign`, `gh`) and for labels, annotations and paths that a workflow reader must see. Do not grow a `.sh` script into orchestration code or copy API and `jq` patterns across scripts.

Do not rewrite bash in another language unless the issue you work on, or the person who gave you the task, asks for it and names the files. A size limit, a complexity limit or testability is not a reason. If you think bash causes a defect, write the failing run or the reproduction and the bash fix you tried in the issue, then stop.

All new Go under `scripts/`:

- goes into an existing module; a new module needs an issue that asks for it.
- adds no interface with fewer than two non-test implementations; tests use temporary directories, fixture files or function values.
- reuses the helpers in `scripts/camunda-core/pkg` (`ghactions.Writer`, `versionmatrix.CompareChartVersions`, `executil`); search there before you write a helper.
- adds no type, field or option that no caller in the change uses.

A rewrite also:

- changes only the files that the request names.
- keeps the behaviour of the bash, except the changes that the request names; another behaviour change goes in a separate PR.
- before you delete the bash, runs the bash and the Go on the same inputs and puts the commands and the diff in the PR body. If the bash needs a live system, say so, and test each decision of the bash in Go.

A non-test Go function under `scripts/` that a change touches must stay at or below cognitive complexity 30, cyclomatic complexity 30 and 250 lines. Split a function that is above a limit before you change it; there is no ignore directive. `make go.complexity` checks the change against `origin/main` (`lintBase=<ref>` to compare with another ref), and `test-release-tooling.yaml` runs the same check on every PR. `cd scripts/complexity-gate && go run . --repo-root ../.. --all` lists every function above the limits.
