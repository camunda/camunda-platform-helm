/**
 * Schema parser for values.schema.json files
 * Extracts configuration entries from JSON Schema format
 */

import type { ConfigEntry } from '../types/index.js';

interface SchemaProperty {
  type?: string;
  description?: string;
  default?: unknown;
  enum?: string[];
  properties?: Record<string, SchemaProperty>;
  items?: SchemaProperty;
  required?: string[];
}

interface Schema {
  type?: string;
  properties?: Record<string, SchemaProperty>;
  required?: string[];
}

/**
 * Parse a JSON Schema and extract all config entries with their paths
 */
export function parseSchema(schema: Schema): Map<string, ConfigEntry> {
  const configs = new Map<string, ConfigEntry>();
  
  if (schema.properties) {
    parseProperties(schema.properties, '', configs, schema.required || []);
  }
  
  return configs;
}

/**
 * Recursively parse properties and build flat config entries
 */
function parseProperties(
  properties: Record<string, SchemaProperty>,
  parentPath: string,
  configs: Map<string, ConfigEntry>,
  requiredFields: string[]
): void {
  for (const [key, prop] of Object.entries(properties)) {
    const path = parentPath ? `${parentPath}.${key}` : key;
    
    const entry: ConfigEntry = {
      path,
      type: prop.type || 'unknown',
      description: prop.description || '',
      default: prop.default,
      deprecated: prop.description?.includes('DEPRECATED') || false,
      required: requiredFields.includes(key),
      parent: parentPath || undefined,
    };
    
    if (prop.enum) {
      entry.enum = prop.enum;
    }
    
    // Track children for object types
    if (prop.properties) {
      entry.children = Object.keys(prop.properties).map(k => `${path}.${k}`);
    }
    
    configs.set(path, entry);
    
    // Recursively parse nested properties
    if (prop.properties) {
      parseProperties(
        prop.properties,
        path,
        configs,
        prop.required || []
      );
    }
    
    // Handle array items with properties
    if (prop.items?.properties) {
      parseProperties(
        prop.items.properties,
        `${path}[]`,
        configs,
        prop.items.required || []
      );
    }
  }
}

/**
 * Get all top-level components from config entries
 */
export function getTopLevelComponents(configs: Map<string, ConfigEntry>): string[] {
  const components = new Set<string>();
  
  for (const path of configs.keys()) {
    const topLevel = path.split('.')[0];
    components.add(topLevel);
  }
  
  return Array.from(components).sort();
}

/**
 * Get configs for a specific component/path prefix
 */
export function getConfigsByPrefix(
  configs: Map<string, ConfigEntry>,
  prefix: string
): ConfigEntry[] {
  const results: ConfigEntry[] = [];
  
  for (const [path, entry] of configs) {
    if (path === prefix || path.startsWith(`${prefix}.`)) {
      results.push(entry);
    }
  }
  
  return results;
}
