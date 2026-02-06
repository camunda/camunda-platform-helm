/**
 * Filesystem loader for chart data
 * Discovers chart versions and loads schema/scenario files
 */

import * as fs from 'fs';
import * as path from 'path';
import YAML from 'yaml';
import type { ChartData, ScenarioFile, DependencyInfo } from '../types/index.js';
import { parseSchema } from './parser.js';

const CHART_PREFIX = 'camunda-platform-';

/**
 * Discover all chart versions in the charts directory
 */
export function discoverChartVersions(chartPath: string): string[] {
  const chartsDir = path.join(chartPath, 'charts');
  
  if (!fs.existsSync(chartsDir)) {
    throw new Error(`Charts directory not found: ${chartsDir}`);
  }
  
  const entries = fs.readdirSync(chartsDir, { withFileTypes: true });
  const versions: string[] = [];
  
  for (const entry of entries) {
    if (entry.isDirectory() && entry.name.startsWith(CHART_PREFIX)) {
      // Extract version from directory name (e.g., "camunda-platform-8.9" -> "8.9")
      const version = entry.name.substring(CHART_PREFIX.length);
      // Only include versioned directories (version filtering is handled by fetcher)
      if (version && /^\d+\.\d+$/.test(version)) {
        versions.push(version);
      }
    }
  }
  
  // Sort versions in descending order (latest first)
  return versions.sort((a, b) => {
    const [aMajor, aMinor] = a.split('.').map(Number);
    const [bMajor, bMinor] = b.split('.').map(Number);
    if (bMajor !== aMajor) return bMajor - aMajor;
    return bMinor - aMinor;
  });
}

/**
 * Get the chart directory path for a specific version
 */
export function getChartDir(chartPath: string, version: string): string {
  return path.join(chartPath, 'charts', `${CHART_PREFIX}${version}`);
}

/**
 * Load chart metadata from Chart.yaml
 */
export function loadChartMetadata(chartDir: string): { 
  chartVersion: string; 
  appVersion: string;
  chartDescription: string;
  dependencies: Map<string, DependencyInfo>;
} {
  const chartYamlPath = path.join(chartDir, 'Chart.yaml');
  
  if (!fs.existsSync(chartYamlPath)) {
    return { 
      chartVersion: 'unknown', 
      appVersion: 'unknown',
      chartDescription: '',
      dependencies: new Map(),
    };
  }
  
  const content = fs.readFileSync(chartYamlPath, 'utf-8');
  const chartYaml = YAML.parse(content);
  
  // Parse dependencies
  const dependencies = new Map<string, DependencyInfo>();
  if (chartYaml.dependencies && Array.isArray(chartYaml.dependencies)) {
    for (const dep of chartYaml.dependencies) {
      const alias = dep.alias || dep.name;
      const isLocal = dep.repository?.startsWith('file://') || false;
      dependencies.set(alias, {
        chart: dep.name,
        version: dep.version || 'unknown',
        source: isLocal ? 'local' : 'external',
        repository: dep.repository || '',
        enableCondition: dep.condition,
      });
    }
  }
  
  // Get description, truncate if too long
  let chartDescription = chartYaml.description || '';
  if (chartDescription.length > 200) {
    chartDescription = chartDescription.substring(0, 197) + '...';
  }
  
  return {
    chartVersion: chartYaml.version || 'unknown',
    appVersion: chartYaml.appVersion || 'unknown',
    chartDescription,
    dependencies,
  };
}

/**
 * Load and parse the values.schema.json file
 */
export function loadSchema(chartDir: string): ReturnType<typeof parseSchema> {
  const schemaPath = path.join(chartDir, 'values.schema.json');
  
  if (!fs.existsSync(schemaPath)) {
    throw new Error(`Schema file not found: ${schemaPath}`);
  }
  
  const content = fs.readFileSync(schemaPath, 'utf-8');
  const schema = JSON.parse(content);
  
  return parseSchema(schema);
}

/**
 * Discover scenario files (values-*.yaml) in a chart directory
 */
export function discoverScenarios(chartDir: string): ScenarioFile[] {
  const scenarios: ScenarioFile[] = [];
  const entries = fs.readdirSync(chartDir);
  
  for (const entry of entries) {
    const filePath = path.join(chartDir, entry);
    
    if (entry === 'values.yaml') {
      scenarios.push({
        name: 'default',
        file: entry,
        path: filePath,
        description: deriveScenarioDescription(filePath, 'default'),
      });
    } else if (entry.startsWith('values-') && entry.endsWith('.yaml')) {
      // Extract scenario name from filename (e.g., "values-local.yaml" -> "local")
      const name = entry.replace('values-', '').replace('.yaml', '');
      scenarios.push({
        name,
        file: entry,
        path: filePath,
        description: deriveScenarioDescription(filePath, name),
      });
    }
  }
  
  // Sort by name
  return scenarios.sort((a, b) => a.name.localeCompare(b.name));
}

/**
 * Derive scenario description from file header comments
 * Falls back to a generated description if no comments found
 */
function deriveScenarioDescription(filePath: string, name: string): string {
  try {
    const content = fs.readFileSync(filePath, 'utf-8');
    const lines = content.split('\n');
    
    // Collect meaningful comment lines at the start
    const commentLines: string[] = [];
    for (const line of lines) {
      if (line.startsWith('#')) {
        const text = line.replace(/^#\s*/, '').trim();
        // Skip empty comments and generic headers
        if (text && !text.match(/^-+$/) && !text.match(/^=+$/)) {
          commentLines.push(text);
        }
      } else if (line.trim() && !line.startsWith('#')) {
        // Stop at first non-comment, non-empty line
        break;
      }
    }
    
    // Use first meaningful comment line as description
    if (commentLines.length > 0) {
      // Clean up the description - remove trailing periods, limit length
      let desc = commentLines[0];
      if (desc.length > 100) {
        desc = desc.substring(0, 97) + '...';
      }
      return desc;
    }
  } catch {
    // Ignore read errors, fall through to default
  }
  
  // Fallback based on scenario name
  if (name === 'default') {
    return 'Standard default values';
  }
  
  // Convert name to readable format: "bitnami-legacy" -> "Bitnami legacy configuration"
  const readable = name
    .split('-')
    .map((word, i) => i === 0 ? word.charAt(0).toUpperCase() + word.slice(1) : word)
    .join(' ');
  
  return `${readable} configuration`;
}

/**
 * Load a scenario file and return its parsed content
 */
export function loadScenarioContent(scenarioPath: string): Record<string, unknown> {
  if (!fs.existsSync(scenarioPath)) {
    throw new Error(`Scenario file not found: ${scenarioPath}`);
  }
  
  const content = fs.readFileSync(scenarioPath, 'utf-8');
  return YAML.parse(content) || {};
}

/**
 * Load all data for a single chart version
 */
export function loadChartData(chartPath: string, version: string): ChartData {
  const chartDir = getChartDir(chartPath, version);
  
  if (!fs.existsSync(chartDir)) {
    throw new Error(`Chart directory not found: ${chartDir}`);
  }
  
  const metadata = loadChartMetadata(chartDir);
  const configs = loadSchema(chartDir);
  const scenarios = discoverScenarios(chartDir);
  
  return {
    version,
    chartVersion: metadata.chartVersion,
    appVersion: metadata.appVersion,
    chartDescription: metadata.chartDescription,
    dependencies: metadata.dependencies,
    configs,
    scenarios,
  };
}
