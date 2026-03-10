/**
 * Tool: generate_values_example
 * Generate example values.yaml from existing scenario files
 */

import YAML from 'yaml';
import type { GenerateValuesExampleInput } from '../types/index.js';
import { getChartData, getScenarioByName } from '../data/store.js';
import { loadScenarioContent } from '../data/loader.js';

export interface GenerateValuesExampleOutput {
  version: string;
  yaml: string;
  description: string;
  sourceFile: string;
  component?: string;
}

/**
 * Extract a specific component from a values object
 */
function extractComponent(values: Record<string, unknown>, component: string): Record<string, unknown> {
  const parts = component.split('.');
  let current: unknown = values;
  
  for (const part of parts) {
    if (current && typeof current === 'object' && part in (current as Record<string, unknown>)) {
      current = (current as Record<string, unknown>)[part];
    } else {
      return {}; // Component not found in values
    }
  }
  
  // Rebuild the nested structure
  let result: Record<string, unknown> = {};
  let target = result;
  
  for (let i = 0; i < parts.length - 1; i++) {
    target[parts[i]] = {};
    target = target[parts[i]] as Record<string, unknown>;
  }
  target[parts[parts.length - 1]] = current;
  
  return result;
}

export function generateValuesExample(input: GenerateValuesExampleInput): GenerateValuesExampleOutput {
  const chartData = getChartData(input.version);
  const scenarioName = input.scenario || 'default';
  
  const scenario = getScenarioByName(scenarioName, input.version);
  
  if (!scenario) {
    const available = chartData.scenarios.map(s => s.name).join(', ');
    throw new Error(`Scenario '${scenarioName}' not found. Available scenarios: ${available}`);
  }
  
  // Load the scenario content
  let values = loadScenarioContent(scenario.path);
  
  // Filter to specific component if requested
  if (input.component) {
    values = extractComponent(values, input.component);
    
    if (Object.keys(values).length === 0) {
      throw new Error(`Component '${input.component}' not found in scenario '${scenarioName}'`);
    }
  }
  
  // Convert to YAML string
  const yamlStr = YAML.stringify(values, {
    indent: 2,
    lineWidth: 0, // Don't wrap lines
  });
  
  const description = input.component
    ? `${input.component} configuration from ${scenario.file}`
    : `Configuration from ${scenario.file}`;
  
  return {
    version: chartData.version,
    yaml: yamlStr.trim(),
    description,
    sourceFile: scenario.file,
    component: input.component,
  };
}
