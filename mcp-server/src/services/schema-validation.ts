/**
 * Schema-based Validation Service
 * 
 * Uses JSON Schema from values.schema.json to validate and filter
 * Helm values at all levels, not just top-level keys.
 */

import type { ChartVersion } from "../types.js";

/**
 * JSON Schema structure for Helm values
 */
export interface JsonSchema {
  type?: string;
  properties?: Record<string, JsonSchema>;
  additionalProperties?: boolean | JsonSchema;
  items?: JsonSchema;
  required?: string[];
  $ref?: string;
  $defs?: Record<string, JsonSchema>;
  allOf?: JsonSchema[];
  anyOf?: JsonSchema[];
  oneOf?: JsonSchema[];
}

/**
 * Result of filtering values against a schema
 */
export interface SchemaFilterResult {
  filteredValues: Record<string, unknown>;
  removedPaths: string[];
  warnings: string[];
}

/**
 * Extract all valid property keys from a JSON schema at a given level
 */
export function getSchemaProperties(schema: JsonSchema): Set<string> {
  const properties = new Set<string>();
  
  if (schema.properties) {
    for (const key of Object.keys(schema.properties)) {
      properties.add(key);
    }
  }
  
  // Handle allOf, anyOf, oneOf by merging properties
  for (const combiner of [schema.allOf, schema.anyOf, schema.oneOf]) {
    if (combiner) {
      for (const subSchema of combiner) {
        const subProps = getSchemaProperties(subSchema);
        for (const prop of subProps) {
          properties.add(prop);
        }
      }
    }
  }
  
  return properties;
}

/**
 * Get the schema for a specific property
 */
export function getPropertySchema(schema: JsonSchema, propertyName: string): JsonSchema | null {
  if (schema.properties && schema.properties[propertyName]) {
    return schema.properties[propertyName];
  }
  
  // Check combiners
  for (const combiner of [schema.allOf, schema.anyOf, schema.oneOf]) {
    if (combiner) {
      for (const subSchema of combiner) {
        const propSchema = getPropertySchema(subSchema, propertyName);
        if (propSchema) return propSchema;
      }
    }
  }
  
  return null;
}

/**
 * Check if a schema allows additional properties
 */
export function allowsAdditionalProperties(schema: JsonSchema): boolean {
  // Default is true if not specified
  if (schema.additionalProperties === undefined) {
    return true;
  }
  return schema.additionalProperties !== false;
}

/**
 * Recursively filter values to remove keys not present in the schema
 * 
 * @param values - The values object to filter
 * @param schema - The JSON schema to validate against
 * @param currentPath - Current path for error reporting
 * @param removedPaths - Array to collect removed paths
 * @param warnings - Array to collect warnings
 * @param strictMode - If true, remove keys not in schema even if additionalProperties is allowed
 * @returns Filtered values object
 */
export function filterValuesBySchema(
  values: Record<string, unknown>,
  schema: JsonSchema,
  currentPath: string = "",
  removedPaths: string[] = [],
  warnings: string[] = [],
  strictMode: boolean = false
): Record<string, unknown> {
  // If no schema or schema allows additional properties freely, return as-is
  if (!schema || !schema.properties) {
    return values;
  }
  
  const validProperties = getSchemaProperties(schema);
  const allowsExtra = allowsAdditionalProperties(schema);
  const filtered: Record<string, unknown> = {};
  
  for (const [key, value] of Object.entries(values)) {
    const keyPath = currentPath ? `${currentPath}.${key}` : key;
    
    if (validProperties.has(key)) {
      // Key is valid according to schema
      const propertySchema = getPropertySchema(schema, key);
      
      if (value !== null && typeof value === "object" && !Array.isArray(value) && propertySchema) {
        // Recursively filter nested objects
        filtered[key] = filterValuesBySchema(
          value as Record<string, unknown>,
          propertySchema,
          keyPath,
          removedPaths,
          warnings,
          strictMode
        );
      } else {
        // Keep the value as-is (primitive, array, or null)
        filtered[key] = value;
      }
    } else if (allowsExtra && !strictMode) {
      // Additional properties are allowed and not in strict mode, keep the value
      filtered[key] = value;
      warnings.push(`Key '${keyPath}' is not in schema but additionalProperties is allowed`);
    } else {
      // Key is not valid (strict mode or additionalProperties not allowed)
      removedPaths.push(keyPath);
    }
  }
  
  return filtered;
}

/**
 * Filter values against a JSON schema and return detailed results
 * 
 * @param values - The values object to filter
 * @param schema - The JSON schema to validate against
 * @param strictMode - If true, remove keys not in schema even if additionalProperties is allowed
 */
export function filterValuesWithSchema(
  values: Record<string, unknown>,
  schema: JsonSchema,
  strictMode: boolean = false
): SchemaFilterResult {
  const removedPaths: string[] = [];
  const warnings: string[] = [];
  
  const filteredValues = filterValuesBySchema(values, schema, "", removedPaths, warnings, strictMode);
  
  return {
    filteredValues,
    removedPaths,
    warnings,
  };
}

/**
 * Check if a key path exists in the schema
 */
export function isPathValidInSchema(schema: JsonSchema, path: string): boolean {
  const parts = path.split(".");
  let currentSchema: JsonSchema | null = schema;
  
  for (const part of parts) {
    if (!currentSchema) return false;
    
    const validProps = getSchemaProperties(currentSchema);
    if (!validProps.has(part)) {
      // Check if additional properties are allowed
      if (allowsAdditionalProperties(currentSchema)) {
        // Can't validate further, assume valid
        return true;
      }
      return false;
    }
    
    currentSchema = getPropertySchema(currentSchema, part);
  }
  
  return true;
}

/**
 * Get all valid top-level keys from a schema
 */
export function getValidTopLevelKeys(schema: JsonSchema): string[] {
  return Array.from(getSchemaProperties(schema));
}
