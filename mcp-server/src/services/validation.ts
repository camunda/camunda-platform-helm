/**
 * Values Validation Service
 * 
 * Validates values.yaml against chart version schemas
 */

import * as yaml from "js-yaml";
import type { 
  ValidationResult, 
  ValidationError, 
  ValidationWarning,
  ChartVersion 
} from "../types.js";
import { usesOrchestration } from "./migration.js";

/**
 * Known configuration keys by version type
 */
const PRE_88_KEYS = [
  "global",
  "zeebe",
  "zeebeGateway",
  "operate",
  "tasklist",
  "optimize",
  "identity",
  "webModeler",
  "connectors",
  "console",
  "elasticsearch",
  "postgresql",
  "retentionPolicy",
];

const POST_88_KEYS = [
  "global",
  "orchestration",
  "optimize",
  "identity",
  "webModeler",
  "connectors",
  "console",
  "elasticsearch",
  "postgresql",
  "retentionPolicy",
];

/**
 * Common sub-keys expected in component configurations
 */
const COMPONENT_KEYS = [
  "enabled",
  "image",
  "replicas",
  "resources",
  "env",
  "envFrom",
  "podAnnotations",
  "podLabels",
  "podSecurityContext",
  "securityContext",
  "nodeSelector",
  "tolerations",
  "affinity",
  "service",
  "ingress",
  "configuration",
];

/**
 * Validate values.yaml structure
 */
export function validateValues(
  valuesYaml: string,
  chartVersion: ChartVersion
): ValidationResult {
  const errors: ValidationError[] = [];
  const warnings: ValidationWarning[] = [];
  
  let values: Record<string, unknown>;
  
  // Parse YAML
  try {
    values = yaml.load(valuesYaml) as Record<string, unknown>;
    if (typeof values !== "object" || values === null) {
      return {
        valid: false,
        errors: [{ path: "", message: "Invalid YAML: expected an object at root level" }],
        warnings: [],
      };
    }
  } catch (e) {
    const error = e as Error;
    return {
      valid: false,
      errors: [{ path: "", message: `YAML parse error: ${error.message}` }],
      warnings: [],
    };
  }
  
  const isOrchVersion = usesOrchestration(chartVersion);
  const expectedKeys = isOrchVersion ? POST_88_KEYS : PRE_88_KEYS;
  
  // Check for unknown top-level keys
  for (const key of Object.keys(values)) {
    if (!expectedKeys.includes(key)) {
      // Check if it's a version mismatch issue
      if (isOrchVersion && PRE_88_KEYS.includes(key) && !POST_88_KEYS.includes(key)) {
        errors.push({
          path: key,
          message: `Key '${key}' is not valid for chart version ${chartVersion}. This appears to be a pre-8.8 configuration key.`,
          expectedType: "Chart version 8.8+ uses 'orchestration' instead of individual component keys",
        });
      } else if (!isOrchVersion && POST_88_KEYS.includes(key) && !PRE_88_KEYS.includes(key)) {
        errors.push({
          path: key,
          message: `Key '${key}' is not valid for chart version ${chartVersion}. This appears to be a 8.8+ configuration key.`,
          expectedType: "Chart versions before 8.8 use individual component keys (zeebe, operate, etc.)",
        });
      } else {
        warnings.push({
          path: key,
          message: `Unknown top-level key '${key}'`,
          suggestion: "Verify this key is valid for your chart version",
        });
      }
    }
  }
  
  // Validate global section
  if (values.global) {
    validateGlobalSection(values.global as Record<string, unknown>, errors, warnings);
  }
  
  // Validate component sections based on version
  if (isOrchVersion) {
    validateOrchestrationSection(values, errors, warnings);
  } else {
    validatePreOrchestrationSections(values, errors, warnings);
  }
  
  // Check for common misconfigurations
  checkCommonMisconfigurations(values, chartVersion, errors, warnings);
  
  return {
    valid: errors.length === 0,
    errors,
    warnings,
  };
}

/**
 * Validate global section
 */
function validateGlobalSection(
  global: Record<string, unknown>,
  errors: ValidationError[],
  warnings: ValidationWarning[]
): void {
  // Check image configuration
  if (global.image && typeof global.image === "object") {
    const image = global.image as Record<string, unknown>;
    if (image.tag && typeof image.tag !== "string") {
      errors.push({
        path: "global.image.tag",
        message: "Image tag must be a string",
        expectedType: "string",
        actualValue: image.tag,
      });
    }
  }
  
  // Check identity auth
  if (global.identity && typeof global.identity === "object") {
    const identity = global.identity as Record<string, unknown>;
    if (identity.auth && typeof identity.auth === "object") {
      const auth = identity.auth as Record<string, unknown>;
      if (auth.enabled === true && !auth.issuer && !auth.publicIssuerUrl) {
        warnings.push({
          path: "global.identity.auth",
          message: "Identity auth is enabled but no issuer URL is configured",
          suggestion: "Set global.identity.auth.publicIssuerUrl for production deployments",
        });
      }
    }
  }
}

/**
 * Validate orchestration section (8.8+)
 */
function validateOrchestrationSection(
  values: Record<string, unknown>,
  errors: ValidationError[],
  warnings: ValidationWarning[]
): void {
  if (!values.orchestration) {
    warnings.push({
      path: "orchestration",
      message: "No orchestration configuration found",
      suggestion: "Add orchestration section for Zeebe, Operate, and Tasklist configuration",
    });
    return;
  }
  
  const orch = values.orchestration as Record<string, unknown>;
  
  // Validate zeebe subsection
  if (orch.zeebe) {
    validateComponentSection("orchestration.zeebe", orch.zeebe as Record<string, unknown>, errors, warnings);
  }
  
  // Validate operate subsection
  if (orch.operate) {
    validateComponentSection("orchestration.operate", orch.operate as Record<string, unknown>, errors, warnings);
  }
  
  // Validate tasklist subsection  
  if (orch.tasklist) {
    validateComponentSection("orchestration.tasklist", orch.tasklist as Record<string, unknown>, errors, warnings);
  }
}

/**
 * Validate pre-8.8 component sections
 */
function validatePreOrchestrationSections(
  values: Record<string, unknown>,
  errors: ValidationError[],
  warnings: ValidationWarning[]
): void {
  const components = ["zeebe", "zeebeGateway", "operate", "tasklist", "optimize", "identity", "webModeler", "connectors", "console"];
  
  for (const component of components) {
    if (values[component]) {
      validateComponentSection(component, values[component] as Record<string, unknown>, errors, warnings);
    }
  }
}

/**
 * Validate a component section
 */
function validateComponentSection(
  path: string,
  component: Record<string, unknown>,
  errors: ValidationError[],
  warnings: ValidationWarning[]
): void {
  // Check replicas
  if (component.replicas !== undefined) {
    if (typeof component.replicas !== "number" || component.replicas < 0) {
      errors.push({
        path: `${path}.replicas`,
        message: "Replicas must be a non-negative number",
        expectedType: "number >= 0",
        actualValue: component.replicas,
      });
    }
  }
  
  // Check resources
  if (component.resources && typeof component.resources === "object") {
    const resources = component.resources as Record<string, unknown>;
    validateResourceSpec(`${path}.resources.requests`, resources.requests, errors);
    validateResourceSpec(`${path}.resources.limits`, resources.limits, errors);
  }
  
  // Check image configuration
  if (component.image && typeof component.image === "object") {
    const image = component.image as Record<string, unknown>;
    if (image.pullPolicy && !["Always", "IfNotPresent", "Never"].includes(image.pullPolicy as string)) {
      errors.push({
        path: `${path}.image.pullPolicy`,
        message: "Invalid pullPolicy",
        expectedType: "Always | IfNotPresent | Never",
        actualValue: image.pullPolicy,
      });
    }
  }
}

/**
 * Validate resource specification
 */
function validateResourceSpec(
  path: string,
  spec: unknown,
  errors: ValidationError[]
): void {
  if (!spec) return;
  
  if (typeof spec !== "object") {
    errors.push({
      path,
      message: "Resource specification must be an object",
      expectedType: "object",
      actualValue: spec,
    });
    return;
  }
  
  const resources = spec as Record<string, unknown>;
  const validKeys = ["cpu", "memory", "ephemeral-storage"];
  
  for (const key of Object.keys(resources)) {
    if (!validKeys.includes(key)) {
      errors.push({
        path: `${path}.${key}`,
        message: `Unknown resource type '${key}'`,
        expectedType: validKeys.join(" | "),
      });
    }
  }
}

/**
 * Check for common misconfigurations
 */
function checkCommonMisconfigurations(
  values: Record<string, unknown>,
  chartVersion: ChartVersion,
  errors: ValidationError[],
  warnings: ValidationWarning[]
): void {
  const global = values.global as Record<string, unknown> | undefined;
  
  // Check for Elasticsearch disabled without alternative
  if (global?.elasticsearch) {
    const es = global.elasticsearch as Record<string, unknown>;
    if (es.enabled === false) {
      // Check if OpenSearch or external ES is configured
      const hasExternal = es.host || es.url;
      if (!hasExternal && !values.opensearch) {
        warnings.push({
          path: "global.elasticsearch",
          message: "Elasticsearch is disabled but no external search backend configured",
          suggestion: "Configure global.elasticsearch.host for external ES, or enable built-in Elasticsearch",
        });
      }
    }
  }
  
  // Check for production readiness
  const zeebe = usesOrchestration(chartVersion) 
    ? (values.orchestration as Record<string, unknown>)?.zeebe as Record<string, unknown> | undefined
    : values.zeebe as Record<string, unknown> | undefined;
    
  if (zeebe) {
    // Check Zeebe cluster size
    const clusterSize = zeebe.clusterSize as number | undefined;
    const replicas = zeebe.replicas as number | undefined;
    const replicationFactor = zeebe.replicationFactor as number | undefined;
    
    if (clusterSize === 1 || replicas === 1) {
      warnings.push({
        path: usesOrchestration(chartVersion) ? "orchestration.zeebe" : "zeebe",
        message: "Single-node Zeebe cluster configured - not recommended for production",
        suggestion: "Use at least 3 replicas with replicationFactor 3 for high availability",
      });
    }
    
    if (replicationFactor && clusterSize && replicationFactor > clusterSize) {
      errors.push({
        path: usesOrchestration(chartVersion) ? "orchestration.zeebe" : "zeebe",
        message: "Replication factor cannot be greater than cluster size",
        expectedType: `replicationFactor <= clusterSize (${clusterSize})`,
        actualValue: replicationFactor,
      });
    }
  }
}

/**
 * Quick validation check - just returns true/false
 */
export function isValidYaml(valuesYaml: string): boolean {
  try {
    const values = yaml.load(valuesYaml);
    return typeof values === "object" && values !== null;
  } catch {
    return false;
  }
}
