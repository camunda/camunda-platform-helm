#!/usr/bin/env node
/**
 * Helm Values MCP Server - Entrypoint
 * 
 * Modes:
 *   - Local mode: CAMUNDA_HELM_CHART_PATH set → use local charts, skip fetch
 *   - Fetch mode: CAMUNDA_HELM_CHART_PATH unset → download charts to ~/.helm-values-mcp
 */

import * as path from 'path';
import { runServer } from './server.js';
import { fetchAllCharts, getDefaultChartPath } from './data/fetcher.js';

/**
 * Main entry point
 */
async function main(): Promise<void> {
  const chartPath = process.env.CAMUNDA_HELM_CHART_PATH;

  if (chartPath) {
    // Local mode - use provided path directly
    const resolvedPath = path.resolve(chartPath);
    console.error(`Helm Values MCP Server - Local mode`);
    console.error(`Chart path: ${resolvedPath}`);
    
    await runServer(resolvedPath);
  } else {
    // Fetch mode - download charts first
    const defaultPath = getDefaultChartPath();
    console.error(`Helm Values MCP Server - Fetch mode`);
    console.error(`Chart path: ${defaultPath}`);

    try {
      await fetchAllCharts(defaultPath);
    } catch (error) {
      console.error(`\nFailed to fetch charts: ${error}`);
      console.error(`\nEnsure 'helm' CLI is installed and you have network access.`);
      process.exit(1);
    }

    await runServer(defaultPath);
  }
}

main().catch((error) => {
  console.error('Fatal error:', error);
  process.exit(1);
});
