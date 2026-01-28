/**
 * MCP Tools Registration
 * 
 * Registers all tools available in the Camunda Helm MCP server
 */

import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { z } from "zod";
import { GitHubDataService } from "../services/github-data.js";
import { validateValues, isValidYaml } from "../services/validation.js";
import { migrateValuesWithSchema, detectValuesVersion, getRequiredMigrations } from "../services/migration.js";
import type { JsonSchema } from "../services/schema-validation.js";
import type { ChartVersion } from "../types.js";

const SUPPORTED_VERSIONS: ChartVersion[] = ["8.2", "8.3", "8.4", "8.5", "8.6", "8.7", "8.8", "8.9"];

export function registerTools(server: McpServer, githubService: GitHubDataService): void {
  
  // Create a schema fetcher that uses the GitHub service
  const schemaFetcher = async (version: ChartVersion): Promise<JsonSchema | null> => {
    return await githubService.getValuesSchema(version) as JsonSchema | null;
  };
  
  // Tool: validate-values
  server.tool(
    "validate-values",
    "Validate a Helm values.yaml file against a specific Camunda chart version schema. Returns validation errors and warnings.",
    {
      valuesYaml: z.string().describe("The contents of the values.yaml file to validate"),
      chartVersion: z.enum(["8.2", "8.3", "8.4", "8.5", "8.6", "8.7", "8.8", "8.9"])
        .describe("Target Camunda chart version (e.g., '8.7' or '8.8')"),
    },
    async ({ valuesYaml, chartVersion }) => {
      const result = validateValues(valuesYaml, chartVersion);
      
      let response = `## Validation Results for Camunda ${chartVersion}\n\n`;
      response += `**Status:** ${result.valid ? "✅ Valid" : "❌ Invalid"}\n\n`;
      
      if (result.errors.length > 0) {
        response += `### Errors (${result.errors.length})\n\n`;
        for (const error of result.errors) {
          response += `- **${error.path || "root"}**: ${error.message}\n`;
          if (error.expectedType) {
            response += `  - Expected: ${error.expectedType}\n`;
          }
          if (error.actualValue !== undefined) {
            response += `  - Got: \`${JSON.stringify(error.actualValue)}\`\n`;
          }
        }
        response += "\n";
      }
      
      if (result.warnings.length > 0) {
        response += `### Warnings (${result.warnings.length})\n\n`;
        for (const warning of result.warnings) {
          response += `- **${warning.path}**: ${warning.message}\n`;
          if (warning.suggestion) {
            response += `  - Suggestion: ${warning.suggestion}\n`;
          }
        }
      }
      
      return {
        content: [{ type: "text", text: response }],
      };
    }
  );

  // Tool: migrate-values
  server.tool(
    "migrate-values",
    "Migrate a values.yaml file from one Camunda chart version to another. Handles the 8.8+ orchestration structure changes automatically.",
    {
      valuesYaml: z.string().describe("The contents of the values.yaml file to migrate"),
      fromVersion: z.enum(["8.2", "8.3", "8.4", "8.5", "8.6", "8.7", "8.8", "8.9"])
        .describe("Source Camunda chart version"),
      toVersion: z.enum(["8.2", "8.3", "8.4", "8.5", "8.6", "8.7", "8.8", "8.9"])
        .describe("Target Camunda chart version"),
    },
    async ({ valuesYaml, fromVersion, toVersion }) => {
      if (!isValidYaml(valuesYaml)) {
        return {
          content: [{ type: "text", text: "❌ Error: Invalid YAML syntax in the provided values file." }],
        };
      }
      
      // Use schema-based migration with GitHub schema fetcher
      const result = await migrateValuesWithSchema(valuesYaml, fromVersion, toVersion, schemaFetcher);
      
      let response = `## Values Migration: ${fromVersion} → ${toVersion}\n\n`;
      
      // Show schema usage status
      if (result.schemaUsed) {
        response += `✅ **Schema validation enabled** - values filtered against ${toVersion} schema\n\n`;
      } else {
        response += `⚠️ **Schema not available** - could not fetch schema for validation\n\n`;
      }
      
      if (result.changes.length === 0) {
        response += "No structural changes required for this version migration.\n\n";
        response += "**Note:** While the structure may be compatible, always review the values against the target version's documentation for any semantic changes.\n";
      } else {
        response += `### Changes Applied (${result.changes.length})\n\n`;
        for (const change of result.changes) {
          response += `- ${change}\n`;
        }
        response += "\n";
      }
      
      // Show warnings if any
      if (result.warnings.length > 0) {
        response += `### ⚠️ Warnings (${result.warnings.length})\n\n`;
        for (const warning of result.warnings) {
          response += `- ${warning}\n`;
        }
        response += "\n";
      }
      
      // Include validation results
      response += `### Validation Status: ${result.validation.valid ? "✅ Valid" : "⚠️ Has Issues"}\n\n`;
      
      if (result.validation.errors.length > 0) {
        response += `#### Errors (${result.validation.errors.length})\n\n`;
        for (const error of result.validation.errors) {
          response += `- **${error.path || "root"}**: ${error.message}\n`;
          if (error.expectedType) {
            response += `  - Expected: ${error.expectedType}\n`;
          }
          if (error.actualValue !== undefined) {
            response += `  - Got: \`${JSON.stringify(error.actualValue)}\`\n`;
          }
        }
        response += "\n";
      }
      
      if (result.validation.warnings.length > 0) {
        response += `#### Validation Warnings (${result.validation.warnings.length})\n\n`;
        for (const warning of result.validation.warnings) {
          response += `- **${warning.path}**: ${warning.message}\n`;
          if (warning.suggestion) {
            response += `  - Suggestion: ${warning.suggestion}\n`;
          }
        }
        response += "\n";
      }
      
      response += "### Migrated values.yaml\n\n```yaml\n" + result.yaml + "```\n";
      
      return {
        content: [{ type: "text", text: response }],
      };
    }
  );

  // Tool: get-upgrade-path
  server.tool(
    "get-upgrade-path",
    "Get the recommended upgrade path between two Camunda versions, including intermediate steps, breaking changes, and estimated downtime.",
    {
      fromVersion: z.string().describe("Current Camunda version (e.g., '8.5')"),
      toVersion: z.string().describe("Target Camunda version (e.g., '8.8')"),
    },
    async ({ fromVersion, toVersion }) => {
      const upgradePath = await githubService.getUpgradePath(fromVersion, toVersion);
      
      let response = `## Upgrade Path: Camunda ${fromVersion} → ${toVersion}\n\n`;
      response += `**Estimated Downtime:** ${upgradePath.estimatedDowntime}\n\n`;
      
      if (upgradePath.breakingChanges.length > 0) {
        response += `### ⚠️ Breaking Changes\n\n`;
        for (const change of upgradePath.breakingChanges) {
          response += `#### ${change.component}\n`;
          response += `- **Description:** ${change.description}\n`;
          response += `- **Migration:** ${change.migrationPath}\n`;
          if (change.valuePath) {
            response += `- **Value change:** \`${change.oldValue}\` → \`${change.newValue}\`\n`;
          }
          response += "\n";
        }
      }
      
      response += `### Upgrade Steps\n\n`;
      for (const step of upgradePath.steps) {
        response += `#### Step ${step.order}: ${step.description}\n\n`;
        
        if (step.preChecks && step.preChecks.length > 0) {
          response += "**Pre-checks:**\n";
          for (const check of step.preChecks) {
            response += `- [ ] ${check}\n`;
          }
          response += "\n";
        }
        
        if (step.commands && step.commands.length > 0) {
          response += "**Commands:**\n```bash\n";
          for (const cmd of step.commands) {
            response += `${cmd}\n`;
          }
          response += "```\n\n";
        }
        
        if (step.postChecks && step.postChecks.length > 0) {
          response += "**Post-checks:**\n";
          for (const check of step.postChecks) {
            response += `- [ ] ${check}\n`;
          }
          response += "\n";
        }
      }
      
      return {
        content: [{ type: "text", text: response }],
      };
    }
  );

  // Tool: generate-values
  server.tool(
    "generate-values",
    "Generate a values.yaml template for a specific Camunda installation scenario and version.",
    {
      chartVersion: z.enum(["8.6", "8.7", "8.8", "8.9"])
        .describe("Target Camunda chart version"),
      scenario: z.enum(["minimal", "development", "production", "production-ha"])
        .describe("Installation scenario: minimal (single-node), development (all features, low resources), production (recommended settings), production-ha (high availability)"),
      components: z.array(z.string()).optional()
        .describe("Optional list of components to include: zeebe, operate, tasklist, optimize, identity, webModeler, connectors, console"),
      enableElasticsearch: z.boolean().optional()
        .describe("Whether to include built-in Elasticsearch (default: true)"),
      enableIdentity: z.boolean().optional()
        .describe("Whether to enable Identity/Keycloak (default: true)"),
    },
    async ({ chartVersion, scenario, components, enableElasticsearch = true, enableIdentity = true }) => {
      const isOrch = chartVersion === "8.8" || chartVersion === "8.9";
      
      let yaml = `# Camunda Platform ${chartVersion} Helm Values\n`;
      yaml += `# Scenario: ${scenario}\n`;
      yaml += `# Generated by camunda-helm-mcp\n\n`;
      
      // Global section
      yaml += `global:\n`;
      yaml += `  image:\n`;
      yaml += `    tag: "${chartVersion}.0"\n`;
      yaml += `    pullPolicy: IfNotPresent\n`;
      
      if (enableIdentity) {
        yaml += `  identity:\n`;
        yaml += `    auth:\n`;
        yaml += `      enabled: true\n`;
      }
      
      yaml += `\n`;
      
      // Scenario-based resource configuration
      const resources = getResourcesForScenario(scenario);
      
      if (isOrch) {
        // 8.8+ structure
        yaml += `orchestration:\n`;
        yaml += `  zeebe:\n`;
        yaml += `    clusterSize: ${resources.zeebeClusterSize}\n`;
        yaml += `    partitionCount: ${resources.zeebePartitions}\n`;
        yaml += `    replicationFactor: ${resources.zeebeReplication}\n`;
        yaml += `    resources:\n`;
        yaml += `      requests:\n`;
        yaml += `        cpu: "${resources.zeebeCpuRequest}"\n`;
        yaml += `        memory: "${resources.zeebeMemoryRequest}"\n`;
        yaml += `      limits:\n`;
        yaml += `        cpu: "${resources.zeebeCpuLimit}"\n`;
        yaml += `        memory: "${resources.zeebeMemoryLimit}"\n`;
        yaml += `    pvcSize: "${resources.zeebePvcSize}"\n`;
        yaml += `\n`;
        yaml += `  operate:\n`;
        yaml += `    enabled: true\n`;
        yaml += `    resources:\n`;
        yaml += `      requests:\n`;
        yaml += `        cpu: "${resources.componentCpuRequest}"\n`;
        yaml += `        memory: "${resources.componentMemoryRequest}"\n`;
        yaml += `\n`;
        yaml += `  tasklist:\n`;
        yaml += `    enabled: true\n`;
        yaml += `    resources:\n`;
        yaml += `      requests:\n`;
        yaml += `        cpu: "${resources.componentCpuRequest}"\n`;
        yaml += `        memory: "${resources.componentMemoryRequest}"\n`;
      } else {
        // Pre-8.8 structure
        yaml += `zeebe:\n`;
        yaml += `  clusterSize: ${resources.zeebeClusterSize}\n`;
        yaml += `  partitionCount: ${resources.zeebePartitions}\n`;
        yaml += `  replicationFactor: ${resources.zeebeReplication}\n`;
        yaml += `  resources:\n`;
        yaml += `    requests:\n`;
        yaml += `      cpu: "${resources.zeebeCpuRequest}"\n`;
        yaml += `      memory: "${resources.zeebeMemoryRequest}"\n`;
        yaml += `    limits:\n`;
        yaml += `      cpu: "${resources.zeebeCpuLimit}"\n`;
        yaml += `      memory: "${resources.zeebeMemoryLimit}"\n`;
        yaml += `  pvcSize: "${resources.zeebePvcSize}"\n`;
        yaml += `\n`;
        yaml += `zeebeGateway:\n`;
        yaml += `  replicas: ${resources.gatewayReplicas}\n`;
        yaml += `  resources:\n`;
        yaml += `    requests:\n`;
        yaml += `      cpu: "${resources.componentCpuRequest}"\n`;
        yaml += `      memory: "${resources.componentMemoryRequest}"\n`;
        yaml += `\n`;
        yaml += `operate:\n`;
        yaml += `  enabled: true\n`;
        yaml += `  resources:\n`;
        yaml += `    requests:\n`;
        yaml += `      cpu: "${resources.componentCpuRequest}"\n`;
        yaml += `      memory: "${resources.componentMemoryRequest}"\n`;
        yaml += `\n`;
        yaml += `tasklist:\n`;
        yaml += `  enabled: true\n`;
        yaml += `  resources:\n`;
        yaml += `    requests:\n`;
        yaml += `      cpu: "${resources.componentCpuRequest}"\n`;
        yaml += `      memory: "${resources.componentMemoryRequest}"\n`;
      }
      
      yaml += `\n`;
      
      // Optional components
      const includeOptimize = !components || components.includes("optimize");
      const includeConnectors = !components || components.includes("connectors");
      
      yaml += `optimize:\n`;
      yaml += `  enabled: ${includeOptimize}\n`;
      if (includeOptimize) {
        yaml += `  resources:\n`;
        yaml += `    requests:\n`;
        yaml += `      cpu: "${resources.componentCpuRequest}"\n`;
        yaml += `      memory: "${resources.componentMemoryRequest}"\n`;
      }
      yaml += `\n`;
      
      yaml += `connectors:\n`;
      yaml += `  enabled: ${includeConnectors}\n`;
      yaml += `\n`;
      
      yaml += `identity:\n`;
      yaml += `  enabled: ${enableIdentity}\n`;
      yaml += `\n`;
      
      yaml += `elasticsearch:\n`;
      yaml += `  enabled: ${enableElasticsearch}\n`;
      if (enableElasticsearch) {
        yaml += `  master:\n`;
        yaml += `    replicaCount: ${resources.esReplicas}\n`;
        yaml += `    resources:\n`;
        yaml += `      requests:\n`;
        yaml += `        cpu: "${resources.esCpuRequest}"\n`;
        yaml += `        memory: "${resources.esMemoryRequest}"\n`;
      }
      
      let response = `## Generated Values for Camunda ${chartVersion} (${scenario})\n\n`;
      response += "```yaml\n" + yaml + "```\n\n";
      response += "### Next Steps\n\n";
      response += "1. Review and customize the values for your environment\n";
      response += "2. Configure ingress/networking as needed\n";
      response += "3. Set up secrets for any credentials\n";
      response += "4. Install with: `helm install camunda camunda/camunda-platform -f values.yaml`\n";
      
      return {
        content: [{ type: "text", text: response }],
      };
    }
  );

  // Tool: get-version-matrix
  server.tool(
    "get-version-matrix",
    "Get the Camunda version matrix showing Helm chart versions and component version compatibility.",
    {},
    async () => {
      const matrix = await githubService.getVersionMatrix();
      
      let response = `## Camunda Version Matrix\n\n`;
      response += `*Last updated: ${matrix.lastUpdated}*\n\n`;
      response += "| Camunda | Helm Chart | Zeebe | Operate | Tasklist | Optimize | Identity |\n";
      response += "|---------|------------|-------|---------|----------|----------|----------|\n";
      
      for (const ver of matrix.versions) {
        response += `| ${ver.camundaVersion} | ${ver.helmChartVersion} | ${ver.zeebe} | ${ver.operate} | ${ver.tasklist} | ${ver.optimize} | ${ver.identity} |\n`;
      }
      
      response += "\n### Key Notes\n\n";
      response += "- Chart version 8.8+ uses unified 'orchestration' component\n";
      response += "- Always upgrade sequentially (patch → patch, then minor → minor)\n";
      response += "- Check compatibility matrix before upgrading\n";
      
      return {
        content: [{ type: "text", text: response }],
      };
    }
  );

  // Tool: detect-values-version
  server.tool(
    "detect-values-version",
    "Detect which Camunda chart version structure a values.yaml file follows (pre-8.8 or 8.8+).",
    {
      valuesYaml: z.string().describe("The contents of the values.yaml file to analyze"),
    },
    async ({ valuesYaml }) => {
      const detectedVersion = detectValuesVersion(valuesYaml);
      const migrations = detectedVersion !== "unknown" 
        ? getRequiredMigrations(valuesYaml, "8.7" as ChartVersion, "8.8" as ChartVersion)
        : [];
      
      let response = `## Values Structure Analysis\n\n`;
      response += `**Detected Structure:** ${detectedVersion}\n\n`;
      
      if (detectedVersion === "pre-8.8") {
        response += "This values file uses the **component-based structure** with separate `zeebe`, `operate`, `tasklist` sections.\n\n";
        response += "Compatible with chart versions: 8.2 - 8.7\n\n";
        
        if (migrations.length > 0) {
          response += "### To migrate to 8.8+\n\n";
          response += "The following paths need migration:\n";
          for (const m of migrations) {
            response += `- ${m}\n`;
          }
        }
      } else if (detectedVersion === "8.8+") {
        response += "This values file uses the **orchestration structure** with unified `orchestration` section.\n\n";
        response += "Compatible with chart versions: 8.8+\n";
      } else {
        response += "Could not determine the values structure. The file may be:\n";
        response += "- A minimal configuration without component-specific settings\n";
        response += "- Using only global configuration\n";
        response += "- An invalid or empty file\n";
      }
      
      return {
        content: [{ type: "text", text: response }],
      };
    }
  );

  // Tool: explain-value
  server.tool(
    "explain-value",
    "Explain what a specific Helm values path does and provide examples.",
    {
      valuePath: z.string().describe("The dot-notation path to explain (e.g., 'zeebe.clusterSize' or 'global.identity.auth.enabled')"),
      chartVersion: z.enum(["8.6", "8.7", "8.8", "8.9"]).optional()
        .describe("Chart version for context (affects path structure)"),
    },
    async ({ valuePath, chartVersion = "8.8" }) => {
      const explanation = getValueExplanation(valuePath, chartVersion);
      
      let response = `## Value Explanation: \`${valuePath}\`\n\n`;
      response += explanation;
      
      return {
        content: [{ type: "text", text: response }],
      };
    }
  );
}

/**
 * Get resource configuration based on scenario
 */
function getResourcesForScenario(scenario: string): Record<string, string | number> {
  const configs: Record<string, Record<string, string | number>> = {
    minimal: {
      zeebeClusterSize: 1,
      zeebePartitions: 1,
      zeebeReplication: 1,
      gatewayReplicas: 1,
      zeebeCpuRequest: "200m",
      zeebeCpuLimit: "500m",
      zeebeMemoryRequest: "512Mi",
      zeebeMemoryLimit: "1Gi",
      zeebePvcSize: "10Gi",
      componentCpuRequest: "100m",
      componentMemoryRequest: "256Mi",
      esReplicas: 1,
      esCpuRequest: "200m",
      esMemoryRequest: "512Mi",
    },
    development: {
      zeebeClusterSize: 1,
      zeebePartitions: 2,
      zeebeReplication: 1,
      gatewayReplicas: 1,
      zeebeCpuRequest: "500m",
      zeebeCpuLimit: "1000m",
      zeebeMemoryRequest: "1Gi",
      zeebeMemoryLimit: "2Gi",
      zeebePvcSize: "32Gi",
      componentCpuRequest: "250m",
      componentMemoryRequest: "512Mi",
      esReplicas: 1,
      esCpuRequest: "500m",
      esMemoryRequest: "1Gi",
    },
    production: {
      zeebeClusterSize: 3,
      zeebePartitions: 3,
      zeebeReplication: 3,
      gatewayReplicas: 2,
      zeebeCpuRequest: "1000m",
      zeebeCpuLimit: "2000m",
      zeebeMemoryRequest: "2Gi",
      zeebeMemoryLimit: "4Gi",
      zeebePvcSize: "64Gi",
      componentCpuRequest: "500m",
      componentMemoryRequest: "1Gi",
      esReplicas: 3,
      esCpuRequest: "1000m",
      esMemoryRequest: "2Gi",
    },
    "production-ha": {
      zeebeClusterSize: 6,
      zeebePartitions: 6,
      zeebeReplication: 3,
      gatewayReplicas: 3,
      zeebeCpuRequest: "2000m",
      zeebeCpuLimit: "4000m",
      zeebeMemoryRequest: "4Gi",
      zeebeMemoryLimit: "8Gi",
      zeebePvcSize: "128Gi",
      componentCpuRequest: "1000m",
      componentMemoryRequest: "2Gi",
      esReplicas: 3,
      esCpuRequest: "2000m",
      esMemoryRequest: "4Gi",
    },
  };
  
  return configs[scenario] || configs.development;
}

/**
 * Get explanation for a value path
 */
function getValueExplanation(path: string, chartVersion: string): string {
  const explanations: Record<string, string> = {
    "zeebe.clusterSize": `
**Description:** Number of Zeebe broker nodes in the cluster.

**Type:** Integer (minimum: 1)

**Default:** 3

**Impact:**
- Higher values increase throughput and fault tolerance
- Must be >= replicationFactor
- Each broker consumes significant resources

**Example:**
\`\`\`yaml
zeebe:
  clusterSize: 3
\`\`\`

**Note:** For version 8.8+, use \`orchestration.zeebe.clusterSize\`
`,
    "zeebe.replicationFactor": `
**Description:** Number of copies of each partition across brokers.

**Type:** Integer (minimum: 1, maximum: clusterSize)

**Default:** 3

**Impact:**
- Higher values increase fault tolerance
- Affects write latency (more replicas = more synchronization)
- Recommended: 3 for production

**Example:**
\`\`\`yaml
zeebe:
  replicationFactor: 3
\`\`\`
`,
    "global.identity.auth.enabled": `
**Description:** Enable or disable Identity authentication for Camunda components.

**Type:** Boolean

**Default:** true (when Identity is enabled)

**Impact:**
- When true, all web applications require authentication
- Requires Identity/Keycloak to be properly configured
- Affects inter-component communication

**Example:**
\`\`\`yaml
global:
  identity:
    auth:
      enabled: true
      publicIssuerUrl: "https://keycloak.example.com/realms/camunda"
\`\`\`
`,
    "orchestration.zeebe.clusterSize": `
**Description:** Number of Zeebe broker nodes in the unified orchestration cluster (8.8+).

**Type:** Integer (minimum: 1)

**Default:** 3

**Impact:**
- Same as zeebe.clusterSize but in the new 8.8+ structure
- Part of the unified orchestration component

**Example:**
\`\`\`yaml
orchestration:
  zeebe:
    clusterSize: 3
\`\`\`
`,
  };
  
  const explanation = explanations[path];
  if (explanation) {
    return explanation;
  }
  
  // Generic explanation for unknown paths
  const parts = path.split(".");
  const component = parts[0];
  const setting = parts.slice(1).join(".");
  
  return `
**Path:** \`${path}\`

**Component:** ${component}

**Setting:** ${setting}

This value configures the ${setting} property of the ${component} component.

For detailed documentation, refer to:
- [Camunda Helm Chart Documentation](https://docs.camunda.io/docs/self-managed/deployment/helm/)
- Chart README: \`helm show readme camunda/camunda-platform\`

Use the \`get-default-values\` resource to see the default value for this path.
`;
}
