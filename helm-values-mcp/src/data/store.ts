/**
 * In-memory data store for chart data
 * Singleton pattern - loaded once at startup
 */

import type { ChartData, DataStore, ConfigEntry, ScenarioFile } from '../types/index.js';
import { discoverChartVersions, loadChartData } from './loader.js';

let store: DataStore | null = null;

/**
 * Initialize the data store by loading all chart versions
 */
export function initializeStore(chartPath: string): DataStore {
  const versions = discoverChartVersions(chartPath);
  
  if (versions.length === 0) {
    throw new Error(`No chart versions found in ${chartPath}`);
  }
  
  const charts = new Map<string, ChartData>();
  
  for (const version of versions) {
    try {
      const chartData = loadChartData(chartPath, version);
      charts.set(version, chartData);
    } catch (error) {
      console.error(`Warning: Failed to load chart version ${version}:`, error);
    }
  }
  
  if (charts.size === 0) {
    throw new Error('Failed to load any chart versions');
  }
  
  store = {
    chartPath,
    charts,
    latestVersion: versions[0], // First version is latest (sorted descending)
  };
  
  return store;
}

/**
 * Get the current store instance
 */
export function getStore(): DataStore {
  if (!store) {
    throw new Error('Store not initialized. Call initializeStore() first.');
  }
  return store;
}

/**
 * Check if store is initialized
 */
export function isStoreInitialized(): boolean {
  return store !== null;
}

/**
 * Get chart data for a specific version (or latest if not specified)
 */
export function getChartData(version?: string): ChartData {
  const s = getStore();
  const v = version || s.latestVersion;
  
  const chartData = s.charts.get(v);
  if (!chartData) {
    throw new Error(`Chart version ${v} not found. Available: ${Array.from(s.charts.keys()).join(', ')}`);
  }
  
  return chartData;
}

/**
 * Get all available versions
 */
export function getVersions(): { versions: string[]; latest: string } {
  const s = getStore();
  return {
    versions: Array.from(s.charts.keys()),
    latest: s.latestVersion,
  };
}

/**
 * Get config entry by path
 */
export function getConfigByPath(path: string, version?: string): ConfigEntry | undefined {
  const chartData = getChartData(version);
  return chartData.configs.get(path);
}

/**
 * Search configs by query string
 */
export function searchConfigs(query: string, version?: string, limit = 10): ConfigEntry[] {
  const chartData = getChartData(version);
  const results: Array<{ entry: ConfigEntry; score: number }> = [];
  const queryLower = query.toLowerCase();
  const queryTerms = queryLower.split(/\s+/).filter(t => t.length > 0);
  
  for (const entry of chartData.configs.values()) {
    let score = 0;
    const pathLower = entry.path.toLowerCase();
    const descLower = entry.description.toLowerCase();
    
    for (const term of queryTerms) {
      // Path exact match (highest score)
      if (pathLower === term) {
        score += 100;
      }
      // Path contains term
      else if (pathLower.includes(term)) {
        score += 50;
      }
      // Path segment match
      else if (pathLower.split('.').some(seg => seg.includes(term))) {
        score += 30;
      }
      // Description contains term
      if (descLower.includes(term)) {
        score += 20;
      }
    }
    
    if (score > 0) {
      results.push({ entry, score });
    }
  }
  
  // Sort by score descending and return top results
  return results
    .sort((a, b) => b.score - a.score)
    .slice(0, limit)
    .map(r => r.entry);
}

/**
 * Get scenarios for a version
 */
export function getScenarios(version?: string): ScenarioFile[] {
  const chartData = getChartData(version);
  return chartData.scenarios;
}

/**
 * Get scenario by name
 */
export function getScenarioByName(name: string, version?: string): ScenarioFile | undefined {
  const scenarios = getScenarios(version);
  return scenarios.find(s => s.name === name);
}
