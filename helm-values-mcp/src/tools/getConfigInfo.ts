/**
 * Tool: get_config_info
 * Get detailed info about a specific config path
 */

import YAML from 'yaml';
import type { GetConfigInfoInput, ConfigEntry } from '../types/index.js';
import { getChartData, getConfigByPath } from '../data/store.js';

export interface GetConfigInfoOutput {
  path: string;
  type: string;
  description: string;
  default: unknown;
  deprecated: boolean;
  required: boolean;
  enum?: string[];
  children?: string[];
  parent?: string;
  example: string;
}

/**
 * Generate a YAML example for a config path
 */
function generateExample(path: string, defaultValue: unknown): string {
  const parts = path.split('.');
  let result: Record<string, unknown> = {};
  let current = result;
  
  for (let i = 0; i < parts.length - 1; i++) {
    current[parts[i]] = {};
    current = current[parts[i]] as Record<string, unknown>;
  }
  
  // Set the value
  const lastKey = parts[parts.length - 1];
  if (defaultValue !== undefined) {
    current[lastKey] = defaultValue;
  } else {
    // Provide a sensible placeholder
    current[lastKey] = '<value>';
  }
  
  return YAML.stringify(result).trim();
}

export function getConfigInfo(input: GetConfigInfoInput): GetConfigInfoOutput {
  const chartData = getChartData(input.version);
  const entry = getConfigByPath(input.path, input.version);
  
  if (!entry) {
    // Try to find partial matches
    const partialMatches: string[] = [];
    for (const path of chartData.configs.keys()) {
      if (path.includes(input.path) || input.path.includes(path)) {
        partialMatches.push(path);
      }
    }
    
    let errorMsg = `Config path not found: ${input.path}`;
    if (partialMatches.length > 0) {
      errorMsg += `\nDid you mean: ${partialMatches.slice(0, 5).join(', ')}`;
    }
    throw new Error(errorMsg);
  }
  
  return {
    path: entry.path,
    type: entry.type,
    description: entry.description,
    default: entry.default,
    deprecated: entry.deprecated || false,
    required: entry.required || false,
    enum: entry.enum,
    children: entry.children,
    parent: entry.parent,
    example: generateExample(entry.path, entry.default),
  };
}
