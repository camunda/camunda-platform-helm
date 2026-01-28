/**
 * Values Migration Service
 * 
 * Handles migration of values.yaml between Camunda chart versions,
 * especially the 8.7 -> 8.8+ orchestration unification.
 * 
 * Uses dynamic schema validation from values.schema.json to filter
 * invalid keys at all levels.
 */

import * as yaml from "js-yaml";
import type { ChartVersion, ValidationResult } from "../types.js";
import { validateValues } from "./validation.js";
import { 
  filterValuesWithSchema, 
  getValidTopLevelKeys,
  type JsonSchema 
} from "./schema-validation.js";

/**
 * Schema fetcher function type - allows dependency injection
 */
export type SchemaFetcher = (version: ChartVersion) => Promise<JsonSchema | null>;

/**
 * Check if a version uses the new orchestration structure (8.8+)
 */
export function usesOrchestration(version: string): boolean {
  const parts = version.split(".").map(Number);
  return parts[0] > 8 || (parts[0] === 8 && parts[1] >= 8);
}

/**
 * Components that are moved under 'orchestration' in 8.8+
 * These entire sections are migrated, not just specific keys
 */
const ORCHESTRATION_COMPONENTS = ["zeebe", "zeebeGateway", "operate", "tasklist"];

/**
 * Deep clone an object
 */
function deepClone<T>(obj: T): T {
  return JSON.parse(JSON.stringify(obj));
}

/**
 * Migrate a component section to orchestration structure (8.7 -> 8.8+)
 */
function migrateComponentToOrchestration(
  values: Record<string, unknown>,
  component: string,
  changes: string[]
): void {
  if (!(component in values) || values[component] === undefined) {
    return;
  }

  const componentValue = values[component];
  if (!componentValue || typeof componentValue !== "object") {
    return;
  }

  // Initialize orchestration if not present
  if (!values.orchestration) {
    values.orchestration = {};
  }

  const orchestration = values.orchestration as Record<string, unknown>;
  
  // Deep clone the component value to avoid reference issues
  orchestration[component] = deepClone(componentValue);
  changes.push(`Moved '${component}' -> 'orchestration.${component}'`);
  
  // Remove the old top-level key
  delete values[component];
}

/**
 * Migrate orchestration section back to top-level components (8.8+ -> 8.7)
 */
function migrateOrchestrationToComponents(
  values: Record<string, unknown>,
  changes: string[]
): void {
  if (!values.orchestration || typeof values.orchestration !== "object") {
    return;
  }

  const orchestration = values.orchestration as Record<string, unknown>;
  
  for (const component of ORCHESTRATION_COMPONENTS) {
    if (component in orchestration && orchestration[component] !== undefined) {
      const componentValue = orchestration[component];
      if (componentValue && typeof componentValue === "object") {
        values[component] = deepClone(componentValue);
        changes.push(`Moved 'orchestration.${component}' -> '${component}'`);
        delete orchestration[component];
      }
    }
  }
  
  // Remove orchestration if empty
  if (Object.keys(orchestration).length === 0) {
    delete values.orchestration;
    changes.push("Removed empty 'orchestration' section");
  }
}

/**
 * Recursively clean empty objects from a values object
 */
function cleanEmptyObjects(obj: Record<string, unknown>): void {
  for (const key of Object.keys(obj)) {
    const value = obj[key];
    if (value && typeof value === "object" && !Array.isArray(value)) {
      cleanEmptyObjects(value as Record<string, unknown>);
      if (Object.keys(value as object).length === 0) {
        delete obj[key];
      }
    }
  }
}

/**
 * Migration result with detailed information
 */
export interface MigrationResult {
  yaml: string;
  changes: string[];
  removedPaths: string[];
  warnings: string[];
  validation: ValidationResult;
  schemaUsed: boolean;
}

/**
 * Migrate values.yaml from one version to another with schema-based validation
 * 
 * This async version fetches the target version's schema and uses it to
 * filter out invalid keys at all levels, not just top-level.
 */
export async function migrateValuesWithSchema(
  valuesYaml: string,
  fromVersion: ChartVersion,
  toVersion: ChartVersion,
  schemaFetcher: SchemaFetcher
): Promise<MigrationResult> {
  const values = yaml.load(valuesYaml) as Record<string, unknown>;
  const changes: string[] = [];
  const warnings: string[] = [];
  let removedPaths: string[] = [];
  let schemaUsed = false;
  
  const fromUsesOrch = usesOrchestration(fromVersion);
  const toUsesOrch = usesOrchestration(toVersion);
  
  // Step 1: Apply structural migrations (orchestration changes)
  if (!fromUsesOrch && toUsesOrch) {
    // Migrating TO orchestration structure (8.7 -> 8.8+)
    for (const component of ORCHESTRATION_COMPONENTS) {
      migrateComponentToOrchestration(values, component, changes);
    }
  } else if (fromUsesOrch && !toUsesOrch) {
    // Migrating FROM orchestration structure (8.8+ -> 8.7)
    migrateOrchestrationToComponents(values, changes);
  }
  
  // Step 2: Fetch target version schema and filter invalid keys
  const targetSchema = await schemaFetcher(toVersion);
  
  if (targetSchema) {
    schemaUsed = true;
    
    // Filter values against the schema at all levels (strict mode: remove unknown keys)
    const filterResult = filterValuesWithSchema(values, targetSchema as JsonSchema, false);
    
    // Replace values with filtered values
    const filteredKeys = Object.keys(filterResult.filteredValues);
    const originalKeys = Object.keys(values);
    
    // Clear and repopulate values object
    for (const key of originalKeys) {
      delete values[key];
    }
    for (const [key, value] of Object.entries(filterResult.filteredValues)) {
      values[key] = value;
    }
    
    // Record removed paths
    removedPaths = filterResult.removedPaths;
    for (const path of removedPaths) {
      changes.push(`Removed '${path}' (not valid in chart version ${toVersion} schema)`);
    }
    
    // Collect warnings
    warnings.push(...filterResult.warnings);
  } else {
    warnings.push(`Could not fetch schema for version ${toVersion}, skipping schema validation`);
  }
  
  // Step 3: Clean up empty objects
  cleanEmptyObjects(values);
  
  // Generate the migrated YAML
  const migratedYaml = yaml.dump(values, { indent: 2, lineWidth: 120, noRefs: true });
  
  // Validate the result
  const validation = validateValues(migratedYaml, toVersion);
  
  return {
    yaml: migratedYaml,
    changes,
    removedPaths,
    warnings,
    validation,
    schemaUsed,
  };
}

/**
 * Synchronous migration (legacy, without schema validation)
 * Kept for backwards compatibility
 */
export function migrateValues(
  valuesYaml: string,
  fromVersion: ChartVersion,
  toVersion: ChartVersion
): { yaml: string; changes: string[]; validation: ValidationResult } {
  const values = yaml.load(valuesYaml) as Record<string, unknown>;
  const changes: string[] = [];
  
  const fromUsesOrch = usesOrchestration(fromVersion);
  const toUsesOrch = usesOrchestration(toVersion);
  
  if (!fromUsesOrch && toUsesOrch) {
    for (const component of ORCHESTRATION_COMPONENTS) {
      migrateComponentToOrchestration(values, component, changes);
    }
  } else if (fromUsesOrch && !toUsesOrch) {
    migrateOrchestrationToComponents(values, changes);
  }
  
  cleanEmptyObjects(values);
  
  const migratedYaml = yaml.dump(values, { indent: 2, lineWidth: 120, noRefs: true });
  const validation = validateValues(migratedYaml, toVersion);
  
  return {
    yaml: migratedYaml,
    changes,
    validation,
  };
}

/**
 * Detect which version structure a values.yaml follows
 */
export function detectValuesVersion(valuesYaml: string): "pre-8.8" | "8.8+" | "unknown" {
  try {
    const values = yaml.load(valuesYaml) as Record<string, unknown>;
    
    if (values.orchestration) {
      return "8.8+";
    }
    
    if (values.zeebe || values.zeebeGateway || values.operate || values.tasklist) {
      return "pre-8.8";
    }
    
    return "unknown";
  } catch {
    return "unknown";
  }
}

/**
 * Get a list of paths that need migration between versions
 */
export function getRequiredMigrations(
  valuesYaml: string,
  fromVersion: ChartVersion,
  toVersion: ChartVersion
): string[] {
  const values = yaml.load(valuesYaml) as Record<string, unknown>;
  const migrations: string[] = [];
  
  const fromUsesOrch = usesOrchestration(fromVersion);
  const toUsesOrch = usesOrchestration(toVersion);
  
  if (!fromUsesOrch && toUsesOrch) {
    for (const component of ORCHESTRATION_COMPONENTS) {
      if (component in values && values[component] !== undefined) {
        migrations.push(`${component} -> orchestration.${component}`);
      }
    }
  } else if (fromUsesOrch && !toUsesOrch) {
    if (values.orchestration && typeof values.orchestration === "object") {
      const orchestration = values.orchestration as Record<string, unknown>;
      for (const component of ORCHESTRATION_COMPONENTS) {
        if (component in orchestration && orchestration[component] !== undefined) {
          migrations.push(`orchestration.${component} -> ${component}`);
        }
      }
    }
  }
  
  return migrations;
}
