/**
 * MCP Prompts Registration
 * 
 * Provides interactive prompts for common Camunda Helm chart workflows
 */

import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { z } from "zod";

export function registerPrompts(server: McpServer): void {

  // Prompt: Plan fresh installation
  server.prompt(
    "plan-installation",
    "Interactive guide for planning a new Camunda Platform installation",
    {
      environment: z.string().describe("Target environment: development, staging, or production"),
      version: z.string().describe("Target Camunda version (e.g., 8.7, 8.8)"),
      cloud_provider: z.string().optional().describe("Cloud provider or platform: aws, gcp, azure, openshift, or generic"),
    },
    async ({ environment, version, cloud_provider }) => {
      const provider = cloud_provider || "generic";
      
      return {
        messages: [{
          role: "user",
          content: {
            type: "text",
            text: `I need to plan a fresh Camunda Platform installation with the following requirements:

**Environment:** ${environment}
**Target Version:** Camunda ${version}
**Platform:** ${provider}

Please help me:

1. **Prerequisites Check**
   - What Kubernetes version is required?
   - What are the minimum resource requirements?
   - What tools do I need installed (helm, kubectl)?

2. **Component Selection**
   - Which components should I enable for a ${environment} environment?
   - Do I need Identity/Keycloak for authentication?
   - Should I use built-in Elasticsearch or external?

3. **Configuration Planning**
   - Generate a base values.yaml for my scenario
   - What are the critical settings I should customize?
   - Any ${provider}-specific configurations needed?

4. **Installation Steps**
   - Provide the complete installation commands
   - Include any pre-requisites (namespaces, secrets, etc.)
   - Post-installation verification steps

5. **Security Considerations**
   - What secrets need to be configured?
   - Network policies or ingress recommendations?
   - TLS configuration guidance?

Use the available tools to fetch the latest chart documentation and generate appropriate configurations.`,
          },
        }],
      };
    }
  );

  // Prompt: Plan upgrade
  server.prompt(
    "plan-upgrade",
    "Interactive guide for planning a Camunda Platform upgrade",
    {
      current_version: z.string().describe("Current Camunda version (e.g., 8.5, 8.6)"),
      target_version: z.string().describe("Target Camunda version (e.g., 8.8)"),
      current_values: z.string().optional().describe("Current values.yaml content (optional, for migration analysis)"),
    },
    async ({ current_version, target_version, current_values }) => {
      let prompt = `I need to plan an upgrade of my Camunda Platform installation:

**Current Version:** Camunda ${current_version}
**Target Version:** Camunda ${target_version}

Please help me with a comprehensive upgrade plan:

1. **Upgrade Path Analysis**
   - What intermediate versions do I need to go through?
   - Are there any versions that cannot be skipped?
   - Use the get-upgrade-path tool to determine the steps.

2. **Breaking Changes Review**
   - What breaking changes exist between these versions?
   - Specifically check if this crosses the 8.8 orchestration boundary
   - Document any API or configuration changes

3. **Values Migration**
   - Identify configuration changes needed in values.yaml`;

      if (current_values) {
        prompt += `

Here is my current values.yaml for analysis:
\`\`\`yaml
${current_values}
\`\`\`

   - Use the migrate-values tool to transform my configuration
   - Highlight any manual changes required`;
      } else {
        prompt += `
   - What configuration structure changes are expected?
   - Provide examples of common migrations needed`;
      }

      prompt += `

4. **Pre-Upgrade Checklist**
   - Database backup procedures
   - Data export recommendations
   - Health checks to perform before starting

5. **Upgrade Execution Steps**
   - Step-by-step upgrade commands
   - Rollback procedures if issues occur
   - Verification commands after each step

6. **Post-Upgrade Validation**
   - How to verify all components are healthy
   - Data integrity checks
   - Performance baseline comparison

7. **Estimated Timeline**
   - Expected downtime per step
   - Total estimated upgrade duration
   - Recommendations for minimizing impact

Use the available tools to analyze the upgrade path and generate migration commands.`;

      return {
        messages: [{
          role: "user",
          content: {
            type: "text",
            text: prompt,
          },
        }],
      };
    }
  );

  // Prompt: Troubleshoot deployment
  server.prompt(
    "troubleshoot-deployment",
    "Interactive troubleshooting guide for Camunda deployment issues",
    {
      issue_type: z.string().describe("Type of issue: pods-not-starting, connection-errors, performance, authentication, or other"),
      component: z.string().optional().describe("Affected component: zeebe, operate, tasklist, identity, elasticsearch, or all"),
      error_message: z.string().optional().describe("Specific error message or symptom observed"),
    },
    async ({ issue_type, component, error_message }) => {
      const comp = component || "all components";
      const error = error_message || "No specific error message provided";
      
      return {
        messages: [{
          role: "user",
          content: {
            type: "text",
            text: `I'm experiencing issues with my Camunda Platform deployment and need troubleshooting help:

**Issue Type:** ${issue_type}
**Affected Component:** ${comp}
**Error/Symptom:** ${error}

Please help me troubleshoot:

1. **Initial Diagnosis**
   - What are common causes for this type of issue?
   - What kubectl commands should I run to gather more information?
   - What logs should I examine?

2. **Configuration Review**
   - What values.yaml settings commonly cause this issue?
   - Are there version-specific gotchas I should be aware of?
   - Resource configuration problems?

3. **Common Solutions**
   Based on the issue type "${issue_type}", provide:
   - Step-by-step resolution procedures
   - Configuration fixes with examples
   - Commands to verify the fix

4. **Prevention**
   - How can I prevent this issue in the future?
   - Recommended monitoring setup
   - Health check configurations

5. **Escalation Path**
   - When should I contact Camunda support?
   - What information should I gather before escalating?
   - Links to relevant documentation

Please use the chart documentation resources to provide accurate configuration guidance.`,
          },
        }],
      };
    }
  );

  // Prompt: Configure specific component
  server.prompt(
    "configure-component",
    "Detailed configuration guide for a specific Camunda component",
    {
      component: z.string().describe("Component to configure: zeebe, operate, tasklist, optimize, identity, connectors, webModeler, or elasticsearch"),
      version: z.string().describe("Target Camunda version"),
      use_case: z.string().optional().describe("Specific use case or requirement (e.g., high-availability, external-database, custom-auth)"),
    },
    async ({ component, version, use_case }) => {
      const useCase = use_case || "general production deployment";
      
      return {
        messages: [{
          role: "user",
          content: {
            type: "text",
            text: `I need detailed configuration guidance for the ${component} component:

**Component:** ${component}
**Camunda Version:** ${version}
**Use Case:** ${useCase}

Please provide:

1. **Component Overview**
   - What does ${component} do in the Camunda platform?
   - What are its dependencies on other components?
   - Resource requirements for ${useCase}

2. **Configuration Reference**
   - Show me the relevant values.yaml structure for version ${version}
   - Note: Check if this version uses orchestration.${component} or direct ${component} key
   - List all important configuration options

3. **Use Case Configuration**
   For "${useCase}", provide:
   - Recommended configuration values
   - Environment variables that should be set
   - Any secrets or credentials needed

4. **Best Practices**
   - Production-ready configuration recommendations
   - Scaling considerations
   - Monitoring and alerting setup

5. **Integration Points**
   - How ${component} connects to other components
   - Network/service configuration
   - External system integration (if applicable)

6. **Complete Example**
   - Provide a complete values.yaml snippet for ${component}
   - Include comments explaining each setting

Use the chart values resources and documentation to provide accurate, version-specific guidance.`,
          },
        }],
      };
    }
  );

  // Prompt: Compare versions
  server.prompt(
    "compare-versions",
    "Compare features and configuration between two Camunda versions",
    {
      version_a: z.string().describe("First version to compare (e.g., 8.6)"),
      version_b: z.string().describe("Second version to compare (e.g., 8.8)"),
    },
    async ({ version_a, version_b }) => {
      return {
        messages: [{
          role: "user",
          content: {
            type: "text",
            text: `Please compare Camunda versions ${version_a} and ${version_b}:

1. **Structural Changes**
   - Use the component reference resources for both versions
   - Highlight differences in values.yaml structure
   - Note the orchestration vs component-based structure if applicable

2. **Feature Differences**
   - New features in ${version_b} vs ${version_a}
   - Deprecated features
   - Changed behaviors

3. **Configuration Migration**
   - What configuration keys changed?
   - Show mapping of old keys to new keys
   - Use the migration tools if crossing 8.8 boundary

4. **Component Version Changes**
   - Use the version matrix to show component version differences
   - Dependency changes (Elasticsearch, Keycloak, etc.)

5. **Upgrade Considerations**
   - Breaking changes between versions
   - Data migration requirements
   - Recommended upgrade approach

Please use the available tools and resources to gather accurate version information.`,
          },
        }],
      };
    }
  );
}
