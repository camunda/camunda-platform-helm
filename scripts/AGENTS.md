<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-07-01 | Updated: 2026-07-01 -->

# scripts

## Purpose

This directory contains Go CLI tools and shell scripts for the camunda-platform-helm repository. The Go CLIs handle complex orchestration logic (deployment, values merging, secret mapping, and analysis), while shell scripts coordinate deployment workflows, CI matrix generation, and e2e test execution.

All Go tools must be built and tested per the parent `AGENTS.md` rules. Complex CI logic (>20 lines) belongs in Go with unit tests, not bash.

## Key Files and Subdirectories

### Go CLI Tools (in subdirectories, each with `go.mod`)

| Tool | Purpose | Build | Install |
|------|---------|-------|---------|
| **camunda-core** | Shared Go library (core types, helpers, scenarios, logging) | N/A | Part of `install.dx-tooling` |
| **camunda-deployer** | Deploy orchestration CLI | `make build.deployer` | `make install.deployer` |
| **deploy-camunda** | Helm deploy wrapper CLI (values prep, Helm install/upgrade) | `make build.deploy-camunda` | `make install.deploy-camunda` |
| **helm_unused_values** | Finds values.yaml keys not referenced in templates | `make build.dx-tooling` (standalone) | Built separately; run via `./helm-unused-values <chart_templates_dir>` |
| **notify-pr-activity** | GitHub PR notification tool (Slack block formatting) | Built separately | Built separately |
| **prepare-helm-values** | Values preparation/merging CLI | `make build.prepare-helm-values` | `make install.prepare-helm-values` |
| **templates** | Go text/template helpers (no main binary) | N/A | N/A |
| **values-injector** | Injects computed image tags into values.yaml per chart version | Built separately | Built separately |
| **vault-secret-mapper** | Maps Vault secrets to Kubernetes Secret YAML | `make build.vault-secret-mapper` | `make install.vault-secret-mapper` |

### Key Shell Scripts

| Script | Purpose |
|--------|---------|
| `generate-chart-matrix.sh` | Generates CI test matrix from active chart versions and scenarios |
| `generate-version-matrix.sh` | Generates component version matrix (orchestrates image tag discovery) |
| `deploy-camunda.sh` | Deployment orchestration wrapper (sources deploy-camunda CLI) |
| `render-e2e-env.sh` | Renders e2e environment configs from templates |
| `run-e2e-tests.sh` | Executes e2e test suite (playwright-based) |
| `bump-chart-version.sh` | Bumps Helm chart version numbers |
| `check-values-latest.sh` | Validates values-latest.yaml exists and is synchronized |
| `build-ci-runner-local.sh` | Local CI runner for testing matrix generation locally |
| `test-ci-runner-local.sh` | Test harness for local CI runner |
| `apply-ttl-to-elasticsearch-indexes.sh` | Elasticsearch TTL management for test cleanup |
| `get-credientials-from-cluster.sh` | Extracts credentials from running Kubernetes cluster |
| `harbor-retry.sh` | Retry wrapper for Harbor registry operations |
| `list-chart-images.sh` | Lists Docker images referenced by a chart |
| `list-chart-image-commits.sh` | Maps chart images to Git commits |
| `verify-components-version.sh` | Verifies component versions match expectations |
| `download-chart-docker-images.sh` | Pre-pulls chart images for offline deployments |
| `set-prs-version-label.sh` | Labels PRs with version information |
| `sort-version-matrix.go` | Standalone Go utility to sort version matrices |

## For AI Agents

### Working In This Directory

1. **Build vs. Install**: Use `make build.<tool>` for development/testing (outputs to local dir). Use `make install.<tool>` to place binaries in `$GOPATH/bin`. All Go tools use `go mod tidy` before building.

2. **Testing Go Tools**:
   - Go tools with unit tests live alongside their code (e.g., `deploy-camunda/cmd/root_test.go`).
   - Run tests from the tool's directory: `cd scripts/<tool> && go test ./...`
   - No integration tests for individual tools here; chart-level integration tests live in `charts/<version>/test/`.

3. **Chart Version Awareness**: Tools like `values-injector` and `prepare-helm-values` are version-aware. Always check supported versions in code:
   - `values-injector/main.go` validates against `validVersions` map (currently 8.6–8.10).
   - `prepare-helm-values` delegates to chart-specific test helpers.

4. **Modifying Shell Scripts**:
   - Use proper logging: prefer the patterns from `deploy-camunda.sh` (color-coded timestamp logs with level prefixes).
   - Always set `set -euo pipefail` at the top.
   - Use `trap` for cleanup and error handling (see `deploy-camunda.sh` for examples).
   - Resolve paths relative to script location via `SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"` to enable calling from anywhere.

5. **Go Module Patterns**:
   - Imports use full module paths (e.g., `github.com/camunda/camunda-platform-helm/scripts/...`).
   - The shared `camunda-core` library provides logging, scenarios, and common types.
   - Go 1.26.1 (pinned in `.tool-versions`); check pinned version before assuming language features.

### Testing Requirements

**Go CLI Testing**:
- Unit tests required for any CLI logic (error handling, value merging, secret mapping).
- Use `testify/suite` and table-driven patterns (see parent `AGENTS.md`).
- Run tests before committing: `cd scripts/<tool> && go test ./...`
- License headers required: `addlicense -c 'Camunda Services GmbH' -l apache <go-files>`

**Shell Script Testing**:
- Bash unit tests use `bats` (Bash Automated Testing System).
- Test file location: `test/scripts/<script_name>.bats`
- Run tests: `bats test/scripts/` or `bats test/scripts/<test_file>.bats`
- No license headers required for shell scripts.

**Integration Testing**:
- Shell scripts are integration-tested via CI matrix execution (`charts/*/test/ci-test-config.yaml`).
- `generate-chart-matrix.sh` output is verified by CI pipelines.
- Local testing: `scripts/build-ci-runner-local.sh` simulates matrix generation.

### Common Patterns

**Adding a New Go Tool**:
1. Create `scripts/<tool_name>/` directory.
2. Run `go mod init github.com/camunda/camunda-platform-helm/scripts/<tool_name>`.
3. Add main.go and supporting cmd/ and pkg/ packages.
4. Add unit tests.
5. Update Makefile: add `build.<tool_name>` and `install.<tool_name>` targets.
6. Reference in `make build.dx-tooling` and `make install.dx-tooling` if it's a core tool.
7. Document in this file.

**Adding a New Shell Script**:
1. Create `scripts/<name>.sh`.
2. Start with `#!/usr/bin/env bash` and `set -euo pipefail`.
3. Include logging helpers and trap handlers (see `deploy-camunda.sh`).
4. Resolve paths relative to script directory.
5. Add corresponding `test/scripts/<name>.bats` test file.
6. Document in this file.

**Version-Specific Logic**:
- Check chart versions at runtime (don't assume 8.10 exists).
- Use maps or case statements to define supported versions.
- Example: `values-injector/main.go` validates against `validVersions`.

**Error Handling**:
- Go: Use `log.Fatal` for unrecoverable initialization errors; return errors otherwise.
- Bash: Trap `ERR` and call error handler with `$LINENO`. Use `set -e` to fail on first error.

## Dependencies

### External Commands (pinned in `.tool-versions`)

- **Go**: 1.26.1
- **Helm**: 3.20.1
- **kubectl**: 1.27.16
- **yq**: 4.52.5 (YAML parsing, used in scripts)
- **jq**: 1.8.1 (JSON parsing)
- **bats**: 1.11.0 (Bash testing)
- **rg** (ripgrep): Optional; used by `helm-unused-values` for faster search if installed

### Go Module Dependencies

View with `go mod graph` in each tool directory. Key shared dependencies:
- `github.com/spf13/cobra`: CLI framework (used by deployer, deploy-camunda, prepare-helm-values)
- `github.com/stretchr/testify`: Testing utilities (suite, assert, require)
- `gopkg.in/yaml.v3`: YAML parsing
- Camunda-internal: `github.com/camunda/camunda-platform-helm/scripts/camunda-core`

### Internal Dependencies

- `camunda-core`: Imported by most CLI tools (logging, scenarios, types).
- `templates/`: Go template helpers; no external import (used at chart test time).

## Build and Install

```bash
# Install all DX tooling (Go CLIs) into $GOPATH/bin
make install.dx-tooling

# Build all without installing
make build.dx-tooling

# Build individual tools
make build.deployer
make build.deploy-camunda
make build.prepare-helm-values
make build.vault-secret-mapper

# After build, binaries are in scripts/<tool>/ (relative to repo root)
# After install, binaries are in $GOPATH/bin
```

## Common Agent Tasks

### "Fix a CLI tool that's failing tests"
1. Read the failing test in `scripts/<tool>/` directory.
2. Run `cd scripts/<tool> && go test ./... -v` to see exact failure.
3. Fix the code.
4. Run tests again to confirm.
5. If error handling changed, update CLI help text if needed.

### "Add support for a new chart version"
1. Check if version is already in tool's supported versions (e.g., `values-injector/main.go`).
2. Add to the tool's version map/validation.
3. Add version-specific logic if needed (e.g., `buildOverridesXY()` functions in `values-injector`).
4. Add unit test cases for the new version.
5. Update this file with new version range.

### "Debug a shell script failure in CI"
1. Reproduce locally: `scripts/build-ci-runner-local.sh --manual-trigger scenario_name`.
2. Check log output (timestamps + colored log levels).
3. Verify path resolution (script should work from anywhere).
4. Add `DEBUG=1` flag if available, or add `set -x` temporarily.
5. Check for missing environment variables.

### "Extend the scenario system"
1. Read `scripts/camunda-core/pkg/scenarios/scenarios.go` (defines `Scenario` struct).
2. Add new persistence layer: create YAML in `charts/<version>/test/integration/scenarios/chart-full-setup/values/persistence/`, add name to `validPersistence` in scenarios.go.
3. Add new scenario to `charts/<version>/test/ci-test-config.yaml` with identity, persistence, platforms, flows, features, shortname.
4. Add corresponding pre-install or pre-upgrade script if needed (in `charts/<version>/test/integration/scenarios/pre-setup-scripts/`).
5. Update help text in `deploy-camunda/cmd/prepare_values.go` and `deploy-camunda/cmd/root.go`.

<!-- MANUAL: -->
