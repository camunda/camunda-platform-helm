#!/usr/bin/env node
/**
 * Camunda Helm Chart MCP Server
 * 
 * Provides AI-powered assistance for Camunda Platform Helm chart
 * installation and upgrades via the Model Context Protocol.
 */

import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import { registerTools } from "./tools/index.js";
import { registerResources } from "./resources/index.js";
import { registerPrompts } from "./prompts/index.js";
import { GitHubDataService } from "./services/github-data.js";
import { CacheService } from "./services/cache.js";

const SERVER_NAME = "camunda-helm-mcp";
const SERVER_VERSION = "1.0.0";

const SERVER_INSTRUCTIONS = `
You are a specialized assistant for Camunda Platform Helm chart installation and upgrades.

You help users:
- Configure Helm values for fresh installations
- Plan and execute upgrades between chart versions
- Understand component configurations and dependencies
- Troubleshoot deployment issues
- Migrate values.yaml between breaking version changes

Key Knowledge:
- Chart versions 8.6-8.7 use component-based structure (zeebe, operate, tasklist, etc.)
- Chart versions 8.8+ unify core components into "orchestration" 
- Sequential upgrades are required (patch to patch, then minor to minor)
- Version matrix at helm.camunda.io defines component compatibility

Always validate configurations against the correct chart version schema.
`;

async function main() {
  // Initialize services
  const cacheService = new CacheService();
  const githubService = new GitHubDataService(cacheService);

  // Create MCP server
  const server = new McpServer(
    {
      name: SERVER_NAME,
      version: SERVER_VERSION,
    },
    {
      capabilities: {
        tools: {},
        resources: { subscribe: true },
        prompts: {},
        logging: {},
      },
      instructions: SERVER_INSTRUCTIONS,
    }
  );

  // Register capabilities
  registerTools(server, githubService);
  registerResources(server, githubService);
  registerPrompts(server);

  // Connect via stdio transport
  const transport = new StdioServerTransport();
  await server.connect(transport);

  console.error(`${SERVER_NAME} v${SERVER_VERSION} started`);
}

main().catch((error) => {
  console.error("Fatal error:", error);
  process.exit(1);
});
