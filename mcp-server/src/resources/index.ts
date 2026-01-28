/**
 * MCP Resources Registration
 * 
 * Exposes Camunda Helm chart data as MCP resources
 */

import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { GitHubDataService } from "../services/github-data.js";
import type { ChartVersion } from "../types.js";

export function registerResources(server: McpServer, githubService: GitHubDataService): void {

  // Resource: Default values.yaml for each version
  server.resource(
    "camunda://chart/values/{version}",
    "Default values.yaml for a specific Camunda chart version",
    {
      mimeType: "text/yaml",
    },
    async (uri) => {
      const version = uri.pathname.split("/").pop() as ChartVersion;
      
      try {
        const values = await githubService.getValuesYaml(version);
        return {
          contents: [{
            uri: uri.href,
            mimeType: "text/yaml",
            text: values,
          }],
        };
      } catch (error) {
        const err = error as Error;
        return {
          contents: [{
            uri: uri.href,
            mimeType: "text/plain",
            text: `Error fetching values.yaml for version ${version}: ${err.message}`,
          }],
        };
      }
    }
  );

  // Resource: Enterprise values for each version
  server.resource(
    "camunda://chart/values-enterprise/{version}",
    "Enterprise values.yaml overrides for a specific Camunda chart version",
    {
      mimeType: "text/yaml",
    },
    async (uri) => {
      const version = uri.pathname.split("/").pop() as ChartVersion;
      
      try {
        const values = await githubService.getEnterpriseValues(version);
        if (!values) {
          return {
            contents: [{
              uri: uri.href,
              mimeType: "text/plain",
              text: `No enterprise values found for version ${version}`,
            }],
          };
        }
        return {
          contents: [{
            uri: uri.href,
            mimeType: "text/yaml",
            text: values,
          }],
        };
      } catch (error) {
        const err = error as Error;
        return {
          contents: [{
            uri: uri.href,
            mimeType: "text/plain",
            text: `Error fetching enterprise values for version ${version}: ${err.message}`,
          }],
        };
      }
    }
  );

  // Resource: Version matrix
  server.resource(
    "camunda://version-matrix",
    "Camunda Platform version compatibility matrix showing Helm chart and component versions",
    {
      mimeType: "application/json",
    },
    async (uri) => {
      const matrix = await githubService.getVersionMatrix();
      return {
        contents: [{
          uri: uri.href,
          mimeType: "application/json",
          text: JSON.stringify(matrix, null, 2),
        }],
      };
    }
  );

  // Resource: Upgrade guide for a version
  server.resource(
    "camunda://docs/upgrade/{version}",
    "Upgrade documentation for a specific Camunda version",
    {
      mimeType: "text/markdown",
    },
    async (uri) => {
      const version = uri.pathname.split("/").pop() || "";
      
      const guide = await githubService.getUpgradeGuide(version);
      if (!guide) {
        return {
          contents: [{
            uri: uri.href,
            mimeType: "text/plain",
            text: `No upgrade guide found for version ${version}`,
          }],
        };
      }
      
      return {
        contents: [{
          uri: uri.href,
          mimeType: "text/markdown",
          text: guide,
        }],
      };
    }
  );

  // Resource: Installation guide
  server.resource(
    "camunda://docs/installation/{type}",
    "Camunda Helm chart installation documentation (quick or production)",
    {
      mimeType: "text/markdown",
    },
    async (uri) => {
      const type = uri.pathname.split("/").pop() as "quick" | "production";
      
      const guide = await githubService.getInstallationGuide(type);
      if (!guide) {
        return {
          contents: [{
            uri: uri.href,
            mimeType: "text/plain",
            text: `No installation guide found for type ${type}`,
          }],
        };
      }
      
      return {
        contents: [{
          uri: uri.href,
          mimeType: "text/markdown",
          text: guide,
        }],
      };
    }
  );

  // Resource: Chart README
  server.resource(
    "camunda://chart/readme/{version}",
    "README documentation for a specific Camunda chart version",
    {
      mimeType: "text/markdown",
    },
    async (uri) => {
      const version = uri.pathname.split("/").pop() as ChartVersion;
      
      const readme = await githubService.getChartReadme(version);
      if (!readme) {
        return {
          contents: [{
            uri: uri.href,
            mimeType: "text/plain",
            text: `No README found for chart version ${version}`,
          }],
        };
      }
      
      return {
        contents: [{
          uri: uri.href,
          mimeType: "text/markdown",
          text: readme,
        }],
      };
    }
  );

  // Resource: Available versions
  server.resource(
    "camunda://chart/versions",
    "List of available Camunda Platform Helm chart versions",
    {
      mimeType: "application/json",
    },
    async (uri) => {
      const versions = await githubService.getAvailableVersions();
      return {
        contents: [{
          uri: uri.href,
          mimeType: "application/json",
          text: JSON.stringify({
            versions,
            latestStable: versions[versions.length - 2] || versions[versions.length - 1], // Second to last is usually latest stable
            latest: versions[versions.length - 1],
            supportsOrchestration: versions.filter(v => {
              const parts = v.split(".").map(Number);
              return parts[0] > 8 || (parts[0] === 8 && parts[1] >= 8);
            }),
          }, null, 2),
        }],
      };
    }
  );

  // Resource: Releases
  server.resource(
    "camunda://chart/releases",
    "Recent GitHub releases of the Camunda Platform Helm chart",
    {
      mimeType: "application/json",
    },
    async (uri) => {
      const releases = await githubService.getReleases(10);
      return {
        contents: [{
          uri: uri.href,
          mimeType: "application/json",
          text: JSON.stringify(releases.map(r => ({
            version: r.tag_name,
            name: r.name,
            publishedAt: r.published_at,
            prerelease: r.prerelease,
            notes: r.body?.substring(0, 500) + (r.body && r.body.length > 500 ? "..." : ""),
          })), null, 2),
        }],
      };
    }
  );

  // Resource: Component reference for a version
  server.resource(
    "camunda://chart/components/{version}",
    "Component reference and structure for a specific Camunda chart version",
    {
      mimeType: "application/json",
    },
    async (uri) => {
      const version = uri.pathname.split("/").pop() as ChartVersion;
      const parts = version.split(".").map(Number);
      const usesOrchestration = parts[0] > 8 || (parts[0] === 8 && parts[1] >= 8);
      
      const componentReference = usesOrchestration ? {
        version,
        structure: "orchestration",
        description: "Camunda 8.8+ uses unified orchestration structure",
        topLevelKeys: [
          "global",
          "orchestration",
          "optimize",
          "identity",
          "webModeler",
          "connectors",
          "console",
          "elasticsearch",
          "postgresql",
        ],
        orchestrationSubKeys: [
          "zeebe",
          "zeebeGateway",
          "operate",
          "tasklist",
        ],
        coreComponents: {
          "orchestration.zeebe": "Zeebe broker cluster - workflow engine core",
          "orchestration.zeebeGateway": "Zeebe Gateway - gRPC API gateway",
          "orchestration.operate": "Operate - workflow monitoring and troubleshooting",
          "orchestration.tasklist": "Tasklist - human task management",
        },
        optionalComponents: {
          "optimize": "Process analytics and reporting",
          "identity": "Authentication and authorization (Keycloak-based)",
          "webModeler": "Browser-based BPMN modeler",
          "connectors": "Pre-built integration connectors",
          "console": "Cluster management console",
        },
      } : {
        version,
        structure: "component-based",
        description: "Pre-8.8 versions use separate component configurations",
        topLevelKeys: [
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
        ],
        coreComponents: {
          "zeebe": "Zeebe broker cluster - workflow engine core",
          "zeebeGateway": "Zeebe Gateway - gRPC API gateway",
          "operate": "Operate - workflow monitoring and troubleshooting",
          "tasklist": "Tasklist - human task management",
        },
        optionalComponents: {
          "optimize": "Process analytics and reporting",
          "identity": "Authentication and authorization (Keycloak-based)",
          "webModeler": "Browser-based BPMN modeler",
          "connectors": "Pre-built integration connectors",
          "console": "Cluster management console",
        },
      };
      
      return {
        contents: [{
          uri: uri.href,
          mimeType: "application/json",
          text: JSON.stringify(componentReference, null, 2),
        }],
      };
    }
  );
}
