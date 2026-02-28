/**
 * MCP Server implementation for Helm Values
 */

import { Server } from '@modelcontextprotocol/sdk/server/index.js';
import { StdioServerTransport } from '@modelcontextprotocol/sdk/server/stdio.js';
import {
  CallToolRequestSchema,
  ListToolsRequestSchema,
  type Tool,
} from '@modelcontextprotocol/sdk/types.js';

import { initializeStore } from './data/store.js';
import {
  listVersions,
  listComponents,
  listScenarios,
  getConfigInfo,
  searchConfigs,
  generateValuesExample,
} from './tools/index.js';

/**
 * Tool definitions for MCP
 */
const TOOLS: Tool[] = [
  {
    name: 'list_versions',
    description: 'List all available Camunda Helm chart versions',
    inputSchema: {
      type: 'object',
      properties: {},
      required: [],
    },
  },
  {
    name: 'list_components',
    description: 'List top-level components (zeebe, operate, tasklist, etc.) in a chart version',
    inputSchema: {
      type: 'object',
      properties: {
        version: {
          type: 'string',
          description: 'Chart version (e.g., "8.9"). Defaults to latest.',
        },
      },
      required: [],
    },
  },
  {
    name: 'list_scenarios',
    description: 'List available scenario files (values-*.yaml) that provide example configurations',
    inputSchema: {
      type: 'object',
      properties: {
        version: {
          type: 'string',
          description: 'Chart version (e.g., "8.9"). Defaults to latest.',
        },
      },
      required: [],
    },
  },
  {
    name: 'get_config_info',
    description: 'Get detailed information about a specific configuration path, including type, description, default value, and example YAML',
    inputSchema: {
      type: 'object',
      properties: {
        path: {
          type: 'string',
          description: 'Configuration path (e.g., "global.ingress.enabled", "zeebe.replicas")',
        },
        version: {
          type: 'string',
          description: 'Chart version (e.g., "8.9"). Defaults to latest.',
        },
      },
      required: ['path'],
    },
  },
  {
    name: 'search_configs',
    description: 'Search for configuration options by keyword. Searches both paths and descriptions.',
    inputSchema: {
      type: 'object',
      properties: {
        query: {
          type: 'string',
          description: 'Search query (e.g., "ingress", "enable", "multitenancy")',
        },
        version: {
          type: 'string',
          description: 'Chart version (e.g., "8.9"). Defaults to latest.',
        },
        limit: {
          type: 'number',
          description: 'Maximum number of results to return. Defaults to 10.',
        },
        requiredOnly: {
          type: 'boolean',
          description: 'Filter to only return required fields. Defaults to false.',
        },
      },
      required: ['query'],
    },
  },
  {
    name: 'generate_values_example',
    description: 'Generate example values.yaml content from scenario files. Can filter to a specific component.',
    inputSchema: {
      type: 'object',
      properties: {
        scenario: {
          type: 'string',
          description: 'Scenario name (e.g., "default", "local", "enterprise"). Use list_scenarios to see available options.',
        },
        component: {
          type: 'string',
          description: 'Filter to a specific component (e.g., "zeebe", "global.ingress")',
        },
        version: {
          type: 'string',
          description: 'Chart version (e.g., "8.9"). Defaults to latest.',
        },
      },
      required: [],
    },
  },
];

/**
 * Create and configure the MCP server
 */
export function createServer(chartPath: string): Server {
  // Initialize the data store
  initializeStore(chartPath);
  
  const server = new Server(
    {
      name: 'helm-values-mcp',
      version: '1.0.0',
    },
    {
      capabilities: {
        tools: {},
      },
    }
  );
  
  // Handle list tools request
  server.setRequestHandler(ListToolsRequestSchema, async () => {
    return { tools: TOOLS };
  });
  
  // Handle tool calls
  server.setRequestHandler(CallToolRequestSchema, async (request) => {
    const { name, arguments: args } = request.params;
    
    try {
      let result: unknown;
      
      switch (name) {
        case 'list_versions':
          result = listVersions();
          break;
          
        case 'list_components':
          result = listComponents({
            version: args?.version as string | undefined,
          });
          break;
          
        case 'list_scenarios':
          result = listScenarios({
            version: args?.version as string | undefined,
          });
          break;
          
        case 'get_config_info':
          result = getConfigInfo({
            path: args?.path as string,
            version: args?.version as string | undefined,
          });
          break;
          
        case 'search_configs':
          result = searchConfigs({
            query: args?.query as string,
            version: args?.version as string | undefined,
            limit: args?.limit as number | undefined,
          });
          break;
          
        case 'generate_values_example':
          result = generateValuesExample({
            scenario: args?.scenario as string | undefined,
            component: args?.component as string | undefined,
            version: args?.version as string | undefined,
          });
          break;
          
        default:
          throw new Error(`Unknown tool: ${name}`);
      }
      
      return {
        content: [
          {
            type: 'text',
            text: JSON.stringify(result, null, 2),
          },
        ],
      };
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      return {
        content: [
          {
            type: 'text',
            text: `Error: ${message}`,
          },
        ],
        isError: true,
      };
    }
  });
  
  return server;
}

/**
 * Run the MCP server with stdio transport
 */
export async function runServer(chartPath: string): Promise<void> {
  const server = createServer(chartPath);
  const transport = new StdioServerTransport();
  
  await server.connect(transport);
  
  // Keep the process running
  process.on('SIGINT', async () => {
    await server.close();
    process.exit(0);
  });
}
