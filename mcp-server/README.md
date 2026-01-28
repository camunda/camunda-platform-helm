# Camunda Helm Chart MCP Server

An MCP (Model Context Protocol) server that provides AI-powered assistance for Camunda Platform Helm chart installation and upgrades.

## Project Structure

```
mcp-server/
├── src/
│   ├── index.ts                   # Main entry point with MCP server setup
│   ├── types.ts                   # TypeScript type definitions
│   ├── services/
│   │   ├── cache.ts               # Caching layer (memory + disk)
│   │   ├── github-data.ts         # GitHub API data fetcher
│   │   ├── migration.ts           # Values.yaml migration service
│   │   ├── schema-validation.ts   # JSON Schema-based validation and filtering
│   │   └── validation.ts          # Values.yaml validation service
│   ├── tools/
│   │   └── index.ts               # 7 MCP tools registered
│   ├── resources/
│   │   └── index.ts               # 9 MCP resources registered
│   └── prompts/
│       └── index.ts               # 5 MCP prompts registered
├── camunda-8.7/
│   ├── values.schema.json         # JSON Schema for 8.7 chart
│   └── values.yaml                # Default values for 8.7
├── camunda-8.8/
│   └── values.yaml                # Default values for 8.8
├── .vscode/mcp.json               # VS Code MCP configuration
├── package.json
├── tsconfig.json
└── README.md
```

## Features

- **Schema-based Validation**: Validates values.yaml against JSON Schema fetched from GitHub
- **Smart Migration**: Automatically migrates values between chart versions, handling the 8.8+ orchestration structure
- **Schema Filtering**: Removes unknown/invalid keys from values.yaml during migration using target schema
- **Upgrade Planning**: Provides step-by-step upgrade paths with breaking changes and estimated downtime
- **Values Generation**: Generates scenario-based values.yaml (minimal, development, production, production-ha)
- **GitHub Integration**: Fetches latest chart data, schemas, and documentation from GitHub
- **Caching**: Memory and disk caching for improved performance

## Capabilities

| Type | Name | Purpose |
|------|------|---------|
| **Tool** | `validate-values` | Validate values.yaml against chart version schema |
| **Tool** | `migrate-values` | Migrate values between versions with schema-based filtering (handles 8.7→8.8 orchestration) |
| **Tool** | `get-upgrade-path` | Get upgrade steps with breaking changes and estimated downtime |
| **Tool** | `generate-values` | Generate values.yaml for scenarios (minimal/development/production/production-ha) |
| **Tool** | `get-version-matrix` | Get component version compatibility matrix |
| **Tool** | `detect-values-version` | Detect values structure (pre-8.8 component-based vs 8.8+ orchestration) |
| **Tool** | `explain-value` | Explain configuration paths with examples |
| **Resource** | `camunda://chart/values/{version}` | Default values.yaml for a version |
| **Resource** | `camunda://chart/values-enterprise/{version}` | Enterprise values overrides |
| **Resource** | `camunda://version-matrix` | Version compatibility matrix (JSON) |
| **Resource** | `camunda://docs/upgrade/{version}` | Upgrade documentation |
| **Resource** | `camunda://docs/installation/{type}` | Installation guides (quick/production) |
| **Resource** | `camunda://chart/readme/{version}` | Chart README documentation |
| **Resource** | `camunda://chart/versions` | Available chart versions with metadata |
| **Resource** | `camunda://chart/releases` | Recent GitHub releases |
| **Resource** | `camunda://chart/components/{version}` | Component reference and structure for a version |
| **Prompt** | `plan-installation` | Interactive guide for planning new installations |
| **Prompt** | `plan-upgrade` | Interactive upgrade planning with migration analysis |
| **Prompt** | `troubleshoot-deployment` | Troubleshooting guide for deployment issues |
| **Prompt** | `configure-component` | Detailed configuration guide for specific components |
| **Prompt** | `compare-versions` | Compare features and configuration between versions |

## Installation

```bash

# Clone the repository

cd camunda-platform-helm/mcp-server

# Install dependencies

npm install

# Build

npm run build
```

## Usage with Claude Desktop

Add to your Claude Desktop configuration (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "camunda-helm": {
      "command": "node",
      "args": ["/path/to/camunda-helm-chart-mcp/dist/index.js"],
      "env": {
        "GITHUB_TOKEN": "your-github-token-optional"
      }
    }
  }
}
```

## Usage with VS Code

The `.vscode/mcp.json` is already configured. Just open the workspace in VS Code with the MCP extension enabled.

```json
{
  "servers": {
    "camunda-helm": {
      "command": "node",
      "args": ["${workspaceFolder}/dist/index.js"],
      "env": {
        "GITHUB_TOKEN": "${env:GITHUB_TOKEN}"
      }
    }
  }
}
```

## Environment Variables

| Variable | Description | Required |
|----------|-------------|----------|
| `GITHUB_TOKEN` | GitHub personal access token for higher API rate limits | No |

## Supported Versions

This MCP server supports Camunda chart versions: **8.2, 8.3, 8.4, 8.5, 8.6, 8.7, 8.8, 8.9**

## Key Knowledge

### Chart Version Structure

- **8.2 - 8.7**: Component-based structure with separate `zeebe`, `operate`, `tasklist` keys
- **8.8+**: Unified `orchestration` structure containing zeebe, operate, tasklist

### Version Mapping

| Camunda Version | Helm Chart Version | Key Changes |
|-----------------|-------------------|-------------|
| 8.2 - 8.5 | 9.x - 10.x | Component-based structure |
| 8.6 | 11.x | Component-based |
| 8.7 | 12.x | Component-based |
| 8.8 | 13.x | Orchestration unification |
| 8.9 | 14.x | Orchestration structure |

### Upgrade Requirements

- Sequential upgrades required (patch → patch, then minor → minor)
- Crossing 8.7 → 8.8 requires values.yaml migration
- Always consult version matrix before upgrading

## Data Sources

This MCP server fetches data from:

- **Helm Chart Repository**: [camunda/camunda-platform-helm](https://github.com/camunda/camunda-platform-helm)
  - `values.yaml` for each chart version
  - `values.schema.json` schemas for validation
  - Version matrix
  - Chart READMEs and release notes

- **Documentation Repository**: [camunda/camunda-docs](https://github.com/camunda/camunda-docs)
  - Installation guides
  - Upgrade documentation

- **Caching**: Data is cached in memory and on disk to reduce API calls and improve performance

## Development

```bash

# Watch mode for development

npm run dev

# Run linting

npm run lint

# Clean build artifacts

npm run clean
```

## Next Steps

1. **Add GitHub token** for higher API rate limits (optional but recommended)
2. **Run in VS Code** — the `.vscode/mcp.json` is already configured
3. __For Claude Desktop__, add configuration to `~/Library/Application Support/Claude/claude_desktop_config.json`


## License

MIT
