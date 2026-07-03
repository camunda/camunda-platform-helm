---
applyTo: "scripts/**"
---

# Scripting — Scoped Instructions

## Overview

This repository prefers Go for non-trivial automation. Scripts under `scripts/` are often used
from CI workflows and should be testable, maintainable, and explicit about error handling.
Use Bash only for thin wrappers and command glue.

---

## Critical Rules

### NEVER
- **NEVER** introduce new Bash business logic in `scripts/` when the script exceeds ~20 lines.
- **NEVER** implement API orchestration (HTTP calls + response handling) in new Bash scripts.
- **NEVER** implement JSON parsing and branching workflows in new Bash scripts.

### ALWAYS
- **ALWAYS** implement new non-trivial automation in Go under `scripts/<feature>/`.
- **ALWAYS** add or update tests for new Go automation logic where practical.
- **ALWAYS** keep Bash scripts as thin wrappers only (argument/env validation and delegation).

---

## Decision Heuristics

Use Go if one or more of the following is true:
- Script calls external APIs.
- Script parses JSON/YAML and branches on values.
- Script coordinates multiple systems (GitHub, Slack, Vault, OpsGenie, etc.).
- Script needs retries, error classification, or structured output.

Bash is acceptable when all are true:
- Glue-only command composition.
- Minimal branching.
- Roughly <=20 lines.
- No complex parsing or orchestration.

---

## Common Mistakes

1. **Growing one-off Bash scripts into orchestration code** — difficult to test and reason about.
2. **Copying API + jq patterns across scripts** — leads to duplicated, fragile behavior.
3. **Embedding business rules in workflow `run:` blocks** — move to Go tooling and call it.
