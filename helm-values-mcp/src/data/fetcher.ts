/**
 * Chart Fetcher - Downloads Helm charts from official repository
 * Fetches latest patch releases for supported minor versions
 */

import * as fs from 'fs';
import * as path from 'path';
import * as os from 'os';
import { execSync } from 'child_process';
import YAML from 'yaml';

// Hardcoded supported versions - update here when new versions are released
const SUPPORTED_VERSIONS = ['8.8', '8.9'];
const HELM_REPO_URL = 'https://helm.camunda.io';
const HELM_REPO_NAME = 'camunda';
const CHART_NAME = 'camunda-platform';

interface HelmIndexEntry {
  version: string;
  appVersion: string;
}

interface HelmIndex {
  entries: {
    [chartName: string]: HelmIndexEntry[];
  };
}

/**
 * Get the default chart data directory
 */
export function getDefaultChartPath(): string {
  return path.join(os.homedir(), '.helm-values-mcp');
}

/**
 * Fetch and parse the Helm repository index.yaml
 */
export async function fetchHelmIndex(): Promise<HelmIndex> {
  const indexUrl = `${HELM_REPO_URL}/index.yaml`;
  
  console.error(`Fetching Helm index from ${indexUrl}...`);
  
  const response = await fetch(indexUrl);
  if (!response.ok) {
    throw new Error(`Failed to fetch Helm index: ${response.status} ${response.statusText}`);
  }
  
  const yamlContent = await response.text();
  return YAML.parse(yamlContent) as HelmIndex;
}

/**
 * Find the latest patch version for each supported minor version
 * Returns a map of minor version -> chart version
 */
export function findLatestPatches(index: HelmIndex): Map<string, string> {
  const entries = index.entries[CHART_NAME];
  if (!entries || entries.length === 0) {
    throw new Error(`No entries found for chart: ${CHART_NAME}`);
  }

  const latestPatches = new Map<string, string>();

  for (const minorVersion of SUPPORTED_VERSIONS) {
    // Find all chart versions for this minor (appVersion starts with "8.X")
    const matchingEntries = entries.filter(entry => {
      const appVersion = entry.appVersion || '';
      return appVersion.startsWith(minorVersion);
    });

    if (matchingEntries.length === 0) {
      console.error(`Warning: No chart found for version ${minorVersion}`);
      continue;
    }

    // Sort by semver to find latest (excluding alpha/beta for stable)
    const stableEntries = matchingEntries.filter(e => 
      !e.version.includes('alpha') && !e.version.includes('beta')
    );
    
    // If no stable, use all entries
    const candidates = stableEntries.length > 0 ? stableEntries : matchingEntries;
    
    // Sort by version descending
    candidates.sort((a, b) => {
      const aParts = a.version.split('.').map(p => parseInt(p.replace(/\D/g, '')) || 0);
      const bParts = b.version.split('.').map(p => parseInt(p.replace(/\D/g, '')) || 0);
      
      for (let i = 0; i < Math.max(aParts.length, bParts.length); i++) {
        const diff = (bParts[i] || 0) - (aParts[i] || 0);
        if (diff !== 0) return diff;
      }
      return 0;
    });

    latestPatches.set(minorVersion, candidates[0].version);
  }

  return latestPatches;
}

/**
 * Ensure Helm repo is added
 */
function ensureHelmRepo(): void {
  try {
    execSync(`helm repo add ${HELM_REPO_NAME} ${HELM_REPO_URL} 2>/dev/null || true`, {
      stdio: 'pipe'
    });
    execSync('helm repo update', { stdio: 'pipe' });
  } catch (error) {
    throw new Error(`Failed to setup Helm repo: ${error}`);
  }
}

/**
 * Pull a specific chart version and extract to target directory
 */
export function pullChart(chartVersion: string, minorVersion: string, outputDir: string): void {
  const chartsDir = path.join(outputDir, 'charts');
  const targetDir = path.join(chartsDir, `camunda-platform-${minorVersion}`);
  const tempDir = path.join(chartsDir, '.tmp');

  // Create directories
  fs.mkdirSync(chartsDir, { recursive: true });
  fs.mkdirSync(tempDir, { recursive: true });

  // Remove existing target directory if exists
  if (fs.existsSync(targetDir)) {
    fs.rmSync(targetDir, { recursive: true });
  }

  console.error(`Pulling ${CHART_NAME} v${chartVersion} for ${minorVersion}...`);

  try {
    // Pull and untar to temp directory
    execSync(
      `helm pull ${HELM_REPO_NAME}/${CHART_NAME} --version ${chartVersion} --untar -d ${tempDir}`,
      { stdio: 'pipe' }
    );

    // Rename to minor version directory
    const pulledDir = path.join(tempDir, CHART_NAME);
    fs.renameSync(pulledDir, targetDir);

    console.error(`  → Extracted to ${targetDir}`);
  } catch (error) {
    throw new Error(`Failed to pull chart ${chartVersion}: ${error}`);
  } finally {
    // Cleanup temp directory
    if (fs.existsSync(tempDir)) {
      fs.rmSync(tempDir, { recursive: true });
    }
  }
}

/**
 * Fetch all supported chart versions
 */
export async function fetchAllCharts(outputDir: string): Promise<void> {
  console.error(`\nFetching Camunda Helm charts to ${outputDir}...`);
  console.error(`Supported versions: ${SUPPORTED_VERSIONS.join(', ')}\n`);

  // Fetch index
  const index = await fetchHelmIndex();

  // Find latest patches
  const latestPatches = findLatestPatches(index);
  
  if (latestPatches.size === 0) {
    throw new Error('No chart versions found to fetch');
  }

  // Setup Helm repo
  ensureHelmRepo();

  // Pull each version
  for (const [minorVersion, chartVersion] of latestPatches) {
    pullChart(chartVersion, minorVersion, outputDir);
  }

  console.error(`\nSuccessfully fetched ${latestPatches.size} chart versions.\n`);
}

/**
 * Get supported versions (for testing)
 */
export function getSupportedVersions(): string[] {
  return [...SUPPORTED_VERSIONS];
}
