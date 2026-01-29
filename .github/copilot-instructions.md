# Camunda Platform Helm Charts - AI Coding Assistant Instructions

## Project Overview

This repository contains Helm charts for deploying Camunda 8 Self-Managed to Kubernetes. It's an **umbrella chart** managing both internal Camunda components (Zeebe, Operate, Tasklist, Optimize, Identity, Connectors, Console, Web Modeler) and external dependencies (Elasticsearch/OpenSearch, Keycloak, PostgreSQL).

**Architecture**: Multiple versioned chart directories (`charts/camunda-platform-8.X/`) allow parallel maintenance of different Camunda releases. Each chart is a complete Helm package with templates, tests, and values files.

## Chart Versioning & Structure

- **Chart versions follow application versions** (since July 2023): Chart 8.9.x deploys Camunda apps 8.9.x
- **Multi-version maintenance**: Charts exist for versions 8.2 through 8.9+ (check `charts/` directory)
- **Version matrix**: See `version-matrix/` for compatibility between Helm chart versions, Camunda components, and dependencies
- **Each chart directory contains**:
  - `values.yaml` - Default configuration (3000+ lines with inline documentation)
  - `values.schema.json` - JSON schema for validation
  - `values-*.yaml` - Scenario files (ingress, enterprise, local, digest, bitnami-legacy)
  - `templates/` - Helm templates organized by component
  - `test/unit/` - Go-based unit tests
  - `test/integration/` - Integration test scenarios

## Development Workflows

### Running Tests

```bash
# Run unit tests (checks against golden files)
make go.test

# Update golden files (run after template changes)
make go.test-golden-updated

# Run tests for specific chart version
make chartPath=charts/camunda-platform-8.9 go.test

# Update Helm dependencies (required before tests)
make helm.dependency-update
```

**Golden files pattern**: Unit tests use [terratest](https://github.com/gruntwork-io/terratest) to render Helm templates and compare against golden YAML files in `test/unit/*/golden/`. When templates change, regenerate golden files with `-args -update-golden`.

### Helm Operations

```bash
# Lint chart
make helm.lint chartPath=charts/camunda-platform-8.9

# Render templates locally
make helm.template chartPath=charts/camunda-platform-8.9

# Install to cluster
make helm.install chartPath=charts/camunda-platform-8.9 releaseName=my-release

# Update documentation from values.yaml
make helm.readme-update chartPath=charts/camunda-platform-8.9

# Regenerate values.schema.json
make helm.schema-update chartPath=charts/camunda-platform-8.9
```

### Go Tooling

Several Go CLI tools in `scripts/` automate complex tasks:

```bash
# Build all tools
make build.dx-tooling

# Install to $GOPATH/bin
make install.dx-tooling
```

**Key tools**:
- `deploy-camunda` - CLI for deploying charts to Kubernetes with scenario management (see `CONTRIBUTING.md`)
- `prepare-helm-values` - Merges and prepares values files for testing
- `vault-secret-mapper` - Manages secrets from Vault
- `camunda-deployer` - Advanced deployment orchestration

## Code Conventions

### Commit Messages

**CRITICAL**: Follow [Conventional Commits](https://www.conventionalcommits.org/):
```
<type>: <description>

Types: feat, fix, refactor, test, docs, style, build, ci, deps, chore
```

Examples from `.github/AGENTS.md`:
- `feat: add support for external Elasticsearch`
- `fix(zeebe): correct memory limit configuration`
- `docs: update installation guide for 8.9`

**CI checks commit messages** - PRs will fail without proper format.

### PR Requirements

1. **Open an issue first** - Don't create PRs without discussion
2. Follow title format: `<type>[optional scope]: <description>`
3. Valid types: `feat`, `fix`, `refactor`, `revert`, `test`, `docs`, `style`, `build`, `ci`, `cd`, `chore`, `deps`

### Go Code Standards

- **Complex CI logic (>20 lines) MUST be Go scripts**, not bash
- **All Go scripts MUST have unit tests**
- Place in `scripts/<tool-name>/` with proper module structure
- Add Apache 2.0 license header (use `make go.addlicense-run`)

## Testing Patterns

### Unit Tests (Go + Terratest)

Located in `charts/<version>/test/unit/<component>/`. Pattern:
```go
func TestSomething(t *testing.T) {
    chartPath, _ := filepath.Abs("../../../")
    options := &helm.Options{
        ValuesFiles: []string{},
        SetValues: map[string]string{
            "component.setting": "value",
        },
    }
    output := helm.RenderTemplate(t, options, chartPath, releaseName, templates)
    // Assert against output or golden file
}
```

### Integration Tests

**For Camunda team only** - require K8s infrastructure. Community contributors rely on CI.

**Scenarios** are in `charts/<version>/test/integration/scenarios/chart-full-setup/`. File naming: `values-integration-test-ingress-<scenario-name>.yaml`

Deploy with: `deploy-camunda deploy --scenario <name>` (see `CONTRIBUTING.md` for full CLI usage)

## Configuration Patterns

### Values File Structure

- **Global section first** (`global.*`) - shared by all components
- **Component sections** - `orchestration`, `identity`, `optimize`, `connectors`, etc.
- **External dependencies** - `elasticsearch`, `identityKeycloak`, `identityPostgresql`, `webModelerPostgresql`
- **Each component has**:
  - `enabled: true/false` - toggle deployment
  - `image.*` - container image config
  - `resources.*` - CPU/memory limits
  - `env.*` - environment variables
  - `podLabels`, `podAnnotations` - Kubernetes metadata

### Template Helpers

Common patterns in `templates/common/_*.tpl`:
- Image pull secrets: `{{ include "camundaPlatform.imagePullSecrets" . }}`
- Full names: `{{ include "camundaPlatform.fullname" . }}`
- Component-specific labels: `{{ include "camundaPlatform.labels" . }}`

## Important Files

- `Makefile` - Primary automation entry point (test, lint, install, etc.)
- `CONTRIBUTING.md` - Contributor guide with issue guidelines, testing details, deploy-camunda CLI usage
- `.github/AGENTS.md` - PR conventions and development tips
- `version-matrix/` - Version compatibility documentation
- `charts/chart-versions.yaml` - Chart version metadata
- `helm-values-mcp/` - MCP server for AI-assisted values exploration (TypeScript/Node.js)

## External Dependencies

- **Bitnami charts**: Elasticsearch, PostgreSQL, Keycloak (referenced as OCI or file dependencies)
- **Terratest**: Go testing library for Helm chart validation
- **helm-docs** (readme-generator): Auto-generates README from values.yaml comments

## Pre-commit Workflow

```bash
# Run before committing changes to charts
make precommit.chores
```

This runs: lint, readme-update, schema-update, and golden file update for quick tests.

## Key Considerations

1. **Never bypass the issue-first requirement** - Discuss changes before coding
2. **Chart version compatibility** - Changes may need backporting to multiple chart versions
3. **Golden files are source of truth** - Update them when templates change, don't manually edit
4. **Values comments are documentation** - They generate README.md via `helm.readme-update`
5. **Integration tests run in CI** - Community contributors don't need K8s access
6. **Multi-tenancy support** - Many scenarios test multi-tenant configurations

## MCP Server (New Component)

The `helm-values-mcp/` directory contains a TypeScript Model Context Protocol server that provides AI tools for exploring Helm values:
- Lists chart versions, components, and scenarios
- Searches configuration paths
- Generates example YAML from scenario files
- Built with: Node.js, TypeScript, `@modelcontextprotocol/sdk`
