/**
 * Tool: list_components
 * List top-level components in a chart version
 */

import type { ComponentInfo, ListComponentsInput } from '../types/index.js';
import { getChartData } from '../data/store.js';
import { getTopLevelComponents, getConfigsByPrefix } from '../data/parser.js';

export interface ListComponentsOutput {
  version: string;
  components: ComponentInfo[];
}

/**
 * Derive a human-readable description for a component
 * First tries to find description from schema, then falls back to name-based generation
 */
function deriveComponentDescription(
  componentName: string,
  configs: { path: string; description?: string }[]
): string {
  // Try to find a root-level description for this component
  const rootConfig = configs.find(c => c.path === componentName);
  if (rootConfig?.description && rootConfig.description !== '(no description)') {
    return rootConfig.description;
  }
  
  // Try to infer from the component's configs
  // Look for common patterns in nested config descriptions
  const enabledConfig = configs.find(c => c.path === `${componentName}.enabled`);
  if (enabledConfig?.description && enabledConfig.description !== '(no description)') {
    // Extract meaningful part from "Enable X component" type descriptions
    const desc = enabledConfig.description;
    if (desc.toLowerCase().includes('enable')) {
      // Convert "Enable X" to "X component"
      const cleaned = desc.replace(/^enable\s+/i, '').replace(/\s*component$/i, '');
      if (cleaned && cleaned.length > 3) {
        return `${cleaned.charAt(0).toUpperCase()}${cleaned.slice(1)} component`;
      }
    }
  }
  
  // Fallback: generate description from component name
  return generateDescriptionFromName(componentName);
}

/**
 * Generate a readable description from a component name
 */
function generateDescriptionFromName(name: string): string {
  // Handle common component names with better descriptions
  const knownComponents: Record<string, string> = {
    global: 'Global configuration shared across all components',
    elasticsearch: 'Elasticsearch for data storage and search',
    prometheusServiceMonitor: 'Prometheus monitoring configuration',
  };
  
  if (knownComponents[name]) {
    return knownComponents[name];
  }
  
  // Convert camelCase to readable format
  const readable = name
    .replace(/([A-Z])/g, ' $1')
    .replace(/^./, str => str.toUpperCase())
    .trim();
  
  return `${readable} configuration`;
}

export function listComponents(input: ListComponentsInput): ListComponentsOutput {
  const chartData = getChartData(input.version);
  const topLevel = getTopLevelComponents(chartData.configs);
  
  const components: ComponentInfo[] = topLevel.map(name => {
    const configs = getConfigsByPrefix(chartData.configs, name);
    const dependency = chartData.dependencies.get(name);
    
    return {
      name,
      description: deriveComponentDescription(name, configs),
      configCount: configs.length,
      ...(dependency && { dependency }),
    };
  });
  
  return {
    version: chartData.version,
    components,
  };
}
