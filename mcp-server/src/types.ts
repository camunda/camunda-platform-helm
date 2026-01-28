/**
 * Type definitions for Camunda Helm Chart data structures
 */

/**
 * Supported Camunda chart versions
 */
export type ChartVersion = "8.2" | "8.3" | "8.4" | "8.5" | "8.6" | "8.7" | "8.8" | "8.9";

/**
 * Mapping of Camunda app versions to Helm chart versions
 */
export interface VersionMapping {
  camundaVersion: string;
  helmChartVersion: string;
  zeebe: string;
  operate: string;
  tasklist: string;
  optimize: string;
  identity: string;
  webModeler?: string;
  connectors?: string;
  console?: string;
  elasticsearch?: string;
  keycloak?: string;
  postgresql?: string;
}

/**
 * Version matrix containing all version mappings
 */
export interface VersionMatrix {
  versions: VersionMapping[];
  lastUpdated: string;
}

/**
 * Component configuration for Helm values
 */
export interface ComponentConfig {
  enabled?: boolean;
  image?: {
    repository?: string;
    tag?: string;
    pullPolicy?: string;
  };
  replicas?: number;
  resources?: {
    requests?: { cpu?: string; memory?: string };
    limits?: { cpu?: string; memory?: string };
  };
  [key: string]: unknown;
}

/**
 * Global configuration section of values.yaml
 */
export interface GlobalConfig {
  image?: {
    tag?: string;
    pullPolicy?: string;
    pullSecrets?: Array<{ name: string }>;
  };
  annotations?: Record<string, string>;
  labels?: Record<string, string>;
  zeebeClusterName?: string;
  elasticsearch?: {
    enabled?: boolean;
    host?: string;
    port?: number;
  };
  identity?: {
    auth?: {
      enabled?: boolean;
      issuer?: string;
      publicIssuerUrl?: string;
      [key: string]: unknown;
    };
  };
  [key: string]: unknown;
}

/**
 * Pre-8.8 values.yaml structure (component-based)
 */
export interface ValuesYamlPre88 {
  global?: GlobalConfig;
  zeebe?: ComponentConfig;
  zeebeGateway?: ComponentConfig;
  operate?: ComponentConfig;
  tasklist?: ComponentConfig;
  optimize?: ComponentConfig;
  identity?: ComponentConfig;
  webModeler?: ComponentConfig;
  connectors?: ComponentConfig;
  console?: ComponentConfig;
  elasticsearch?: ComponentConfig;
  [key: string]: unknown;
}

/**
 * 8.8+ values.yaml structure (orchestration-based)
 */
export interface ValuesYaml88Plus {
  global?: GlobalConfig;
  orchestration?: ComponentConfig;
  optimize?: ComponentConfig;
  identity?: ComponentConfig;
  webModeler?: ComponentConfig;
  connectors?: ComponentConfig;
  console?: ComponentConfig;
  elasticsearch?: ComponentConfig;
  [key: string]: unknown;
}

/**
 * Union type for any values.yaml
 */
export type ValuesYaml = ValuesYamlPre88 | ValuesYaml88Plus;

/**
 * Validation result for values.yaml
 */
export interface ValidationResult {
  valid: boolean;
  errors: ValidationError[];
  warnings: ValidationWarning[];
}

export interface ValidationError {
  path: string;
  message: string;
  expectedType?: string;
  actualValue?: unknown;
}

export interface ValidationWarning {
  path: string;
  message: string;
  suggestion?: string;
}

/**
 * Upgrade path between versions
 */
export interface UpgradePath {
  fromVersion: string;
  toVersion: string;
  steps: UpgradeStep[];
  breakingChanges: BreakingChange[];
  estimatedDowntime: string;
}

export interface UpgradeStep {
  order: number;
  description: string;
  commands?: string[];
  preChecks?: string[];
  postChecks?: string[];
}

export interface BreakingChange {
  component: string;
  description: string;
  migrationPath: string;
  valuePath?: string;
  oldValue?: string;
  newValue?: string;
}

/**
 * Installation scenario configuration
 */
export interface InstallationScenario {
  name: string;
  description: string;
  components: string[];
  prerequisites: string[];
  recommendedResources: ResourceRequirements;
}

export interface ResourceRequirements {
  minCpu: string;
  minMemory: string;
  minStorage: string;
  recommendedCpu: string;
  recommendedMemory: string;
  recommendedStorage: string;
}

/**
 * Cache entry with TTL
 */
export interface CacheEntry<T> {
  data: T;
  timestamp: number;
  ttl: number;
}

/**
 * GitHub API response types
 */
export interface GitHubContent {
  name: string;
  path: string;
  sha: string;
  size: number;
  url: string;
  html_url: string;
  git_url: string;
  download_url: string | null;
  type: "file" | "dir";
  content?: string;
  encoding?: string;
}

export interface GitHubRelease {
  tag_name: string;
  name: string;
  body: string;
  published_at: string;
  prerelease: boolean;
}
