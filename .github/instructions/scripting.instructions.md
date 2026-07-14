---
applyTo: "scripts/**"
---

# Scripting — Scoped Instructions

Non-trivial automation is implemented in Go under `scripts/<feature>/` with tests: anything that calls external APIs, parses JSON/YAML and branches on values, coordinates multiple systems (GitHub, Slack, Vault, OpsGenie, etc.), needs retries/error classification/structured output, or exceeds ~20 lines.

Bash is acceptable only for thin glue: argument/env validation and delegation, minimal branching, roughly <=20 lines, no complex parsing or orchestration. Do not grow one-off Bash scripts into orchestration code, copy API + `jq` patterns across scripts, or embed business rules in workflow `run:` blocks — move that logic to Go tooling and call it.
