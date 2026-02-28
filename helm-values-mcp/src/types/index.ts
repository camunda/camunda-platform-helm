/**
 * Type definitions for Helm Values MCP Server
 */

/**
 * Represents dependency info from Chart.yaml
 */
export interface DependencyInfo {
  chart: string;           // Original chart name (e.g., "keycloak")
  version: string;         // Chart version
  source: 'local' | 'external';  // Local file or external repo
  repository: string;      // Repository URL or file path
  enableCondition?: string; // Condition to enable (e.g., "identityKeycloak.enabled")
}

/**
 * Represents a single configuration entry from the schema
 */
export interface ConfigEntry {
  path: string;           // "global.ingress.enabled"
  type: string;           // "boolean" | "string" | "number" | "object" | "array"
  description: string;    // From schema
  default: unknown;       // Default value
  enum?: string[];        // Allowed values if constrained
  deprecated?: boolean;   // If description contains "DEPRECATED"
  required?: boolean;     // If in required array
  parent?: string;        // Parent path
  children?: string[];    // Child paths for objects
}

/**
 * Represents a scenario file (values-*.yaml)
 */
export interface ScenarioFile {
  name: string;           // "local", "enterprise", "latest"
  file: string;           // "values-local.yaml"
  path: string;           // Full path to file
  description: string;    // Derived from filename
}

/**
 * Represents data for a single chart version
 */
export interface ChartData {
  version: string;        // "8.9"
  chartVersion: string;   // "14.0.0" from Chart.yaml
  appVersion: string;     // "8.9.0" from Chart.yaml
  chartDescription: string; // Description from Chart.yaml
  dependencies: Map<string, DependencyInfo>; // Component alias -> dependency info
  configs: Map<string, ConfigEntry>;
  scenarios: ScenarioFile[];
}

/**
 * The main data store interface
 */
export interface DataStore {
  chartPath: string;
  charts: Map<string, ChartData>;
  latestVersion: string;
}

/**
 * Component info for list_components
 */
export interface ComponentInfo {
  name: string;
  description: string;
  configCount: number;
  dependency?: DependencyInfo; // If component is a chart dependency
}

/**
 * Search result entry
 */
export interface SearchResult {
  path: string;
  type: string;
  description: string;
  default: unknown;
  relevance: number;
  required?: boolean;
}

/**
 * Tool input types
 */
export interface ListVersionsInput {}

export interface ListComponentsInput {
  version?: string;
}

export interface ListScenariosInput {
  version?: string;
}

export interface GetConfigInfoInput {
  path: string;
  version?: string;
}

export interface SearchConfigsInput {
  query: string;
  version?: string;
  limit?: number;
  requiredOnly?: boolean;  // Filter to only required fields
}

export interface GenerateValuesExampleInput {
  component?: string;
  scenario?: string;
  version?: string;
}
