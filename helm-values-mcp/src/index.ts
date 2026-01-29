#!/usr/bin/env node
/**
 * Helm Values MCP Server - Direct Entry Point (for local mode)
 * 
 * This is the original entry point that runs the server directly.
 * Use entrypoint.ts for the full experience (fetch mode support).
 * 
 * Environment variables:
 *   CAMUNDA_HELM_CHART_PATH - Path to the camunda-platform-helm repository (required)
 */

import * as path from 'path';
import { runServer } from './server.js';

/**
 * Main entry point
 */
async function main(): Promise<void> {
  const chartPath = process.env.CAMUNDA_HELM_CHART_PATH;
  
  if (!chartPath) {
    console.error('Error: CAMUNDA_HELM_CHART_PATH environment variable is required');
    console.error('Usage: CAMUNDA_HELM_CHART_PATH=/path/to/repo node dist/index.js');
    process.exit(1);
  }
  
  const resolvedPath = path.resolve(chartPath);
  
  console.error(`Helm Values MCP Server starting...`);
  console.error(`Chart path: ${resolvedPath}`);
  
  try {
    await runServer(resolvedPath);
  } catch (error) {
    console.error('Failed to start server:', error);
    process.exit(1);
  }
}

main();
