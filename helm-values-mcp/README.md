# Helm Values MCP Server

An MCP (Model Context Protocol) server that understands Camunda Helm chart's `values.yaml` and `values.schema.json`, allowing users to ask natural language questions about configuration options.

## Features

- **List chart versions** - See all available Camunda Helm chart versions
- **List components** - Discover top-level components (orchestration, identity, etc.)
- **List scenarios** - Find available example configurations (values-*.yaml files)
- **Get config info** - Get detailed information about any configuration path
- **Search configs** - Search for configurations by keyword
- **Generate examples** - Generate example YAML from scenario files

## Installation

```bash
cd helm-values-mcp
npm install
npm run build
```

## Usage

### With Claude Desktop

Add to your Claude Desktop config (`~/.config/claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "helm-values": {
      "command": "node",
      "args": ["/path/to/helm-values-mcp/dist/index.js"],
      "env": {
        "CAMUNDA_HELM_CHART_PATH": "/path/to/camunda-platform-helm"
      }
    }
  }
}
```

### With VS Code

Configure in your VS Code MCP settings to use the server.

### Command Line Options

```bash
# Using environment variable
CAMUNDA_HELM_CHART_PATH=/path/to/repo node dist/index.js

# Using command line argument
node dist/index.js --chart-path=/path/to/repo

# Using current directory (if inside the repo)
node dist/index.js
```

## Available Tools

### `list_versions`
List all available Camunda Helm chart versions.

```json
// Output
{
  "versions": ["8.9", "8.8", "8.4", "8.3", "8.2"],
  "latest": "8.9"
}
```

### `list_components`
List top-level components in a chart version.

```json
// Input
{ "version": "8.9" }

// Output
{
  "version": "8.9",
  "components": [
    { "name": "global", "description": "Global configuration...", "configCount": 150 },
    { "name": "orchestration", "description": "Orchestration layer...", "configCount": 45 }
  ]
}
```

### `list_scenarios`
List available scenario files (values-*.yaml).

```json
// Input
{ "version": "8.9" }

// Output
{
  "version": "8.9",
  "scenarios": [
    { "name": "default", "file": "values.yaml", "description": "Standard defaults" },
    { "name": "local", "file": "values-local.yaml", "description": "Local development setup" }
  ]
}
```

### `get_config_info`
Get detailed information about a specific configuration path.

```json
// Input
{ "path": "global.ingress.enabled", "version": "8.9" }

// Output
{
  "path": "global.ingress.enabled",
  "type": "boolean",
  "description": "if true, an ingress resource is deployed...",
  "default": false,
  "deprecated": false,
  "example": "global:\n  ingress:\n    enabled: true"
}
```

### `search_configs`
Search for configuration options by keyword.

```json
// Input
{ "query": "ingress enabled", "version": "8.9", "limit": 5 }

// Output
{
  "version": "8.9",
  "query": "ingress enabled",
  "results": [
    {
      "path": "global.ingress.enabled",
      "type": "boolean",
      "description": "if true, an ingress resource is deployed...",
      "default": false,
      "relevance": 0.95
    }
  ]
}
```

### `generate_values_example`
Generate example values.yaml content from scenario files.

```json
// Input
{ "scenario": "local", "component": "global", "version": "8.9" }

// Output
{
  "version": "8.9",
  "yaml": "global:\n  ingress:\n    enabled: false\n  ...",
  "description": "global configuration from values-local.yaml",
  "sourceFile": "values-local.yaml"
}
```

## Example Conversations

**User:** "How do I enable ingress?"

The LLM can use `search_configs({ query: "enable ingress" })` to find relevant configs, then `get_config_info({ path: "global.ingress.enabled" })` to get detailed information.

**User:** "Give me example values for local development"

The LLM can use `list_scenarios({})` to see available scenarios, then `generate_values_example({ scenario: "local" })` to generate the example.

## Development

```bash
# Install dependencies
npm install

# Build
npm run build

# Run tests
npm test

# Watch mode for tests
npm run test:watch

# Development mode (auto-reload)
npm run dev
```

## Architecture

```
src/
├── index.ts           # Entry point
├── server.ts          # MCP server implementation
├── tools/             # MCP tool implementations
│   ├── listVersions.ts
│   ├── listComponents.ts
│   ├── listScenarios.ts
│   ├── getConfigInfo.ts
│   ├── searchConfigs.ts
│   └── generateExample.ts
├── data/              # Data layer
│   ├── loader.ts      # Filesystem operations
│   ├── parser.ts      # Schema parsing
│   └── store.ts       # In-memory data store
└── types/             # TypeScript types
    └── index.ts
```

## Docker Usage

### Build
```bash
docker build -t helm-values-mcp .
```

### Run
```bash
docker run -i helm-values-mcp
```

The container fetches the latest Camunda Helm charts on startup.

## License

Apache-2.0
