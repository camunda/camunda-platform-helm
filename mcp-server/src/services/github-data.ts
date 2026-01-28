/**
 * GitHub Data Service
 * 
 * Fetches Camunda Helm chart data from GitHub repositories:
 * - camunda/camunda-platform-helm - Helm charts and values
 * - camunda/camunda-docs - Documentation
 */

import { CacheService } from "./cache.js";
import type { 
  ChartVersion, 
  VersionMatrix, 
  GitHubContent, 
  GitHubRelease,
  UpgradePath,
  UpgradeStep,
  BreakingChange
} from "../types.js";

const HELM_REPO = "camunda/camunda-platform-helm";
const DOCS_REPO = "camunda/camunda-docs";
const GITHUB_API_BASE = "https://api.github.com";
const GITHUB_RAW_BASE = "https://raw.githubusercontent.com";

// Cache TTLs
const TTL_VERSION_MATRIX = 24 * 60 * 60 * 1000; // 24 hours
const TTL_VALUES_YAML = 6 * 60 * 60 * 1000; // 6 hours
const TTL_VALUES_SCHEMA = 24 * 60 * 60 * 1000; // 24 hours - schemas change less frequently
const TTL_DOCS = 12 * 60 * 60 * 1000; // 12 hours

// Known chart version to directory mapping
const CHART_DIRECTORIES: Record<ChartVersion, string> = {
  "8.2": "camunda-platform-8.2",
  "8.3": "camunda-platform-8.3",
  "8.4": "camunda-platform-8.4",
  "8.5": "camunda-platform-8.5",
  "8.6": "camunda-platform-8.6",
  "8.7": "camunda-platform-8.7",
  "8.8": "camunda-platform-8.8",
  "8.9": "camunda-platform-8.9",
};

export class GitHubDataService {
  private cache: CacheService;
  private headers: Record<string, string>;

  constructor(cache: CacheService) {
    this.cache = cache;
    this.headers = {
      "Accept": "application/vnd.github.v3+json",
      "User-Agent": "camunda-helm-mcp/1.0.0",
    };

    // Add auth token if available
    const token = process.env.GITHUB_TOKEN;
    if (token) {
      this.headers["Authorization"] = `Bearer ${token}`;
    }
  }

  /**
   * Fetch values.yaml for a specific chart version
   */
  async getValuesYaml(version: ChartVersion): Promise<string> {
    const cacheKey = `values_yaml_${version}`;
    const cached = await this.cache.get<string>(cacheKey);
    if (cached) return cached;

    const chartDir = CHART_DIRECTORIES[version];
    if (!chartDir) {
      throw new Error(`Unknown chart version: ${version}`);
    }

    const url = `${GITHUB_RAW_BASE}/${HELM_REPO}/main/charts/${chartDir}/values.yaml`;
    const response = await fetch(url, { headers: this.headers });

    if (!response.ok) {
      throw new Error(`Failed to fetch values.yaml for version ${version}: ${response.status}`);
    }

    const content = await response.text();
    await this.cache.set(cacheKey, content, TTL_VALUES_YAML);
    return content;
  }

  /**
   * Fetch enterprise values for a specific chart version
   */
  async getEnterpriseValues(version: ChartVersion): Promise<string | null> {
    const cacheKey = `values_enterprise_${version}`;
    const cached = await this.cache.get<string>(cacheKey);
    if (cached) return cached;

    const chartDir = CHART_DIRECTORIES[version];
    if (!chartDir) return null;

    const url = `${GITHUB_RAW_BASE}/${HELM_REPO}/main/charts/${chartDir}/values-enterprise.yaml`;
    
    try {
      const response = await fetch(url, { headers: this.headers });
      if (!response.ok) return null;

      const content = await response.text();
      await this.cache.set(cacheKey, content, TTL_VALUES_YAML);
      return content;
    } catch {
      return null;
    }
  }

  /**
   * Fetch values.schema.json for a specific chart version
   * This schema defines all valid configuration keys and their types
   */
  async getValuesSchema(version: ChartVersion): Promise<Record<string, unknown> | null> {
    const cacheKey = `values_schema_${version}`;
    const cached = await this.cache.get<Record<string, unknown>>(cacheKey);
    if (cached) return cached;

    const chartDir = CHART_DIRECTORIES[version];
    if (!chartDir) return null;

    const url = `${GITHUB_RAW_BASE}/${HELM_REPO}/main/charts/${chartDir}/values.schema.json`;
    
    try {
      const response = await fetch(url, { headers: this.headers });
      if (!response.ok) {
        console.error(`Failed to fetch values.schema.json for version ${version}: ${response.status}`);
        return null;
      }

      const content = await response.text();
      const schema = JSON.parse(content) as Record<string, unknown>;
      await this.cache.set(cacheKey, schema, TTL_VALUES_SCHEMA);
      return schema;
    } catch (error) {
      console.error(`Error fetching values.schema.json for version ${version}:`, error);
      return null;
    }
  }

  /**
   * Fetch the version matrix from the Helm repository
   */
  async getVersionMatrix(): Promise<VersionMatrix> {
    const cacheKey = "version_matrix";
    const cached = await this.cache.get<VersionMatrix>(cacheKey);
    if (cached) return cached;

    // Try to fetch the version matrix README
    const url = `${GITHUB_RAW_BASE}/${HELM_REPO}/main/version-matrix/README.md`;
    
    try {
      const response = await fetch(url, { headers: this.headers });
      if (!response.ok) {
        throw new Error(`Failed to fetch version matrix: ${response.status}`);
      }

      const content = await response.text();
      const matrix = this.parseVersionMatrix(content);
      await this.cache.set(cacheKey, matrix, TTL_VERSION_MATRIX);
      return matrix;
    } catch (error) {
      // Return a basic matrix if fetch fails
      return this.getDefaultVersionMatrix();
    }
  }

  /**
   * Parse version matrix from README markdown
   */
  private parseVersionMatrix(markdown: string): VersionMatrix {
    const versions: VersionMatrix["versions"] = [];
    
    // Parse markdown table format
    const lines = markdown.split("\n");
    let inTable = false;
    let headers: string[] = [];

    for (const line of lines) {
      if (line.includes("|") && line.includes("Helm Chart")) {
        inTable = true;
        headers = line.split("|").map(h => h.trim()).filter(Boolean);
        continue;
      }

      if (inTable && line.includes("---")) continue;

      if (inTable && line.includes("|")) {
        const cells = line.split("|").map(c => c.trim()).filter(Boolean);
        if (cells.length >= 2) {
          const mapping: Record<string, string> = {};
          headers.forEach((header, idx) => {
            if (cells[idx]) {
              mapping[header.toLowerCase().replace(/\s+/g, "")] = cells[idx];
            }
          });

          if (mapping["helmchart"] || mapping["camunda"]) {
            versions.push({
              camundaVersion: mapping["camunda"] || mapping["camundaplatform"] || "",
              helmChartVersion: mapping["helmchart"] || "",
              zeebe: mapping["zeebe"] || "",
              operate: mapping["operate"] || "",
              tasklist: mapping["tasklist"] || "",
              optimize: mapping["optimize"] || "",
              identity: mapping["identity"] || "",
              webModeler: mapping["webmodeler"],
              connectors: mapping["connectors"],
              console: mapping["console"],
              elasticsearch: mapping["elasticsearch"],
              keycloak: mapping["keycloak"],
              postgresql: mapping["postgresql"],
            });
          }
        }
      }
    }

    return {
      versions,
      lastUpdated: new Date().toISOString(),
    };
  }

  /**
   * Get default version matrix when fetch fails
   */
  private getDefaultVersionMatrix(): VersionMatrix {
    return {
      versions: [
        {
          camundaVersion: "8.6",
          helmChartVersion: "11.x",
          zeebe: "8.6.x",
          operate: "8.6.x",
          tasklist: "8.6.x",
          optimize: "8.6.x",
          identity: "8.6.x",
        },
        {
          camundaVersion: "8.7",
          helmChartVersion: "12.x",
          zeebe: "8.7.x",
          operate: "8.7.x",
          tasklist: "8.7.x",
          optimize: "8.7.x",
          identity: "8.7.x",
        },
        {
          camundaVersion: "8.8",
          helmChartVersion: "13.x",
          zeebe: "8.8.x",
          operate: "8.8.x",
          tasklist: "8.8.x",
          optimize: "8.8.x",
          identity: "8.8.x",
        },
      ],
      lastUpdated: new Date().toISOString(),
    };
  }

  /**
   * Fetch upgrade documentation for a version
   */
  async getUpgradeGuide(version: string): Promise<string | null> {
    const cacheKey = `upgrade_guide_${version}`;
    const cached = await this.cache.get<string>(cacheKey);
    if (cached) return cached;

    // Try different documentation paths
    const paths = [
      `docs/self-managed/deployment/helm/upgrade/helm-to-${version}.md`,
      `versioned_docs/version-${version}/self-managed/deployment/helm/upgrade.md`,
    ];

    for (const docPath of paths) {
      const url = `${GITHUB_RAW_BASE}/${DOCS_REPO}/main/${docPath}`;
      try {
        const response = await fetch(url, { headers: this.headers });
        if (response.ok) {
          const content = await response.text();
          await this.cache.set(cacheKey, content, TTL_DOCS);
          return content;
        }
      } catch {
        continue;
      }
    }

    return null;
  }

  /**
   * Fetch installation documentation
   */
  async getInstallationGuide(type: "quick" | "production" = "production"): Promise<string | null> {
    const cacheKey = `install_guide_${type}`;
    const cached = await this.cache.get<string>(cacheKey);
    if (cached) return cached;

    const filename = type === "quick" ? "quick-install.md" : "install.md";
    const url = `${GITHUB_RAW_BASE}/${DOCS_REPO}/main/docs/self-managed/deployment/helm/install/${filename}`;

    try {
      const response = await fetch(url, { headers: this.headers });
      if (response.ok) {
        const content = await response.text();
        await this.cache.set(cacheKey, content, TTL_DOCS);
        return content;
      }
    } catch {
      // Fall through
    }

    return null;
  }

  /**
   * Get available chart versions
   */
  async getAvailableVersions(): Promise<ChartVersion[]> {
    const cacheKey = "available_versions";
    const cached = await this.cache.get<ChartVersion[]>(cacheKey);
    if (cached) return cached;

    try {
      const url = `${GITHUB_API_BASE}/repos/${HELM_REPO}/contents/charts`;
      const response = await fetch(url, { headers: this.headers });
      
      if (!response.ok) {
        return Object.keys(CHART_DIRECTORIES) as ChartVersion[];
      }

      const contents = (await response.json()) as GitHubContent[];
      const versions = contents
        .filter(c => c.type === "dir" && c.name.startsWith("camunda-platform-8"))
        .map(c => c.name.replace("camunda-platform-", "") as ChartVersion)
        .sort();

      await this.cache.set(cacheKey, versions, TTL_VERSION_MATRIX);
      return versions;
    } catch {
      return Object.keys(CHART_DIRECTORIES) as ChartVersion[];
    }
  }

  /**
   * Get GitHub releases for the Helm chart
   */
  async getReleases(limit: number = 20): Promise<GitHubRelease[]> {
    const cacheKey = `releases_${limit}`;
    const cached = await this.cache.get<GitHubRelease[]>(cacheKey);
    if (cached) return cached;

    try {
      const url = `${GITHUB_API_BASE}/repos/${HELM_REPO}/releases?per_page=${limit}`;
      const response = await fetch(url, { headers: this.headers });
      
      if (!response.ok) return [];

      const releases = (await response.json()) as GitHubRelease[];
      await this.cache.set(cacheKey, releases, TTL_VERSION_MATRIX);
      return releases;
    } catch {
      return [];
    }
  }

  /**
   * Get the upgrade path between two versions
   */
  async getUpgradePath(fromVersion: string, toVersion: string): Promise<UpgradePath> {
    const steps: UpgradeStep[] = [];
    const breakingChanges: BreakingChange[] = [];

    // Parse versions
    const fromParts = fromVersion.split(".").map(Number);
    const toParts = toVersion.split(".").map(Number);

    // Determine intermediate versions
    const versionsToUpgrade: string[] = [];
    for (let major = fromParts[0]; major <= toParts[0]; major++) {
      const minorStart = major === fromParts[0] ? fromParts[1] : 0;
      const minorEnd = major === toParts[0] ? toParts[1] : 9;
      
      for (let minor = minorStart; minor <= minorEnd; minor++) {
        const ver = `${major}.${minor}`;
        if (ver !== fromVersion) {
          versionsToUpgrade.push(ver);
        }
      }
    }

    // Generate steps for each version
    let stepOrder = 1;
    for (const targetVer of versionsToUpgrade) {
      const guide = await this.getUpgradeGuide(targetVer);
      
      steps.push({
        order: stepOrder++,
        description: `Upgrade to Camunda ${targetVer}`,
        commands: [
          `helm repo update`,
          `helm upgrade camunda camunda/camunda-platform --version ${this.getHelmChartVersion(targetVer)} -f values.yaml`,
        ],
        preChecks: [
          "Backup existing installation",
          "Review release notes",
          "Test in staging environment",
        ],
        postChecks: [
          "Verify all pods are running",
          "Check application health endpoints",
          "Validate data integrity",
        ],
      });

      // Check for breaking changes
      const changes = this.getBreakingChangesForVersion(targetVer);
      breakingChanges.push(...changes);
    }

    return {
      fromVersion,
      toVersion,
      steps,
      breakingChanges,
      estimatedDowntime: this.estimateDowntime(fromVersion, toVersion),
    };
  }

  /**
   * Get Helm chart version for a Camunda version
   */
  private getHelmChartVersion(camundaVersion: string): string {
    const mapping: Record<string, string> = {
      "8.2": "8.2.x",
      "8.3": "8.3.x",
      "8.4": "9.x",
      "8.5": "10.x",
      "8.6": "11.x",
      "8.7": "12.x",
      "8.8": "13.x",
      "8.9": "14.x",
    };
    return mapping[camundaVersion] || `${camundaVersion}.x`;
  }

  /**
   * Get known breaking changes for a version
   */
  private getBreakingChangesForVersion(version: string): BreakingChange[] {
    const changes: Record<string, BreakingChange[]> = {
      "8.3": [
        {
          component: "Labels",
          description: "Label selectors changed, requiring migration",
          migrationPath: "Follow label migration guide to update selectors",
        },
      ],
      "8.8": [
        {
          component: "Orchestration",
          description: "Zeebe, Operate, and Tasklist unified into 'orchestration' component",
          migrationPath: "Migrate zeebe.*, operate.*, tasklist.* values to orchestration.*",
          valuePath: "zeebe",
          oldValue: "zeebe.replicas: 3",
          newValue: "orchestration.zeebe.replicas: 3",
        },
        {
          component: "Configuration",
          description: "Component configurations moved under orchestration key",
          migrationPath: "Update values.yaml structure to use orchestration.* prefix",
        },
      ],
    };

    return changes[version] || [];
  }

  /**
   * Estimate downtime for upgrade
   */
  private estimateDowntime(fromVersion: string, toVersion: string): string {
    const fromParts = fromVersion.split(".").map(Number);
    const toParts = toVersion.split(".").map(Number);
    
    const majorDiff = Math.abs(toParts[0] - fromParts[0]);
    const minorDiff = Math.abs(toParts[1] - fromParts[1]);

    if (majorDiff > 0) {
      return "1-4 hours (major version upgrade)";
    } else if (minorDiff > 2) {
      return "30-60 minutes (multiple minor versions)";
    } else if (minorDiff > 0) {
      return "15-30 minutes (minor version upgrade)";
    }
    return "5-15 minutes (patch upgrade)";
  }

  /**
   * Get README/documentation for the chart
   */
  async getChartReadme(version: ChartVersion): Promise<string | null> {
    const cacheKey = `chart_readme_${version}`;
    const cached = await this.cache.get<string>(cacheKey);
    if (cached) return cached;

    const chartDir = CHART_DIRECTORIES[version];
    if (!chartDir) return null;

    const url = `${GITHUB_RAW_BASE}/${HELM_REPO}/main/charts/${chartDir}/README.md`;
    
    try {
      const response = await fetch(url, { headers: this.headers });
      if (response.ok) {
        const content = await response.text();
        await this.cache.set(cacheKey, content, TTL_DOCS);
        return content;
      }
    } catch {
      // Fall through
    }

    return null;
  }
}
