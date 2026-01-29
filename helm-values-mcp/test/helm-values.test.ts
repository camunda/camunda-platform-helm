/**
 * Comprehensive tests for Helm Values MCP Server
 */

import { describe, it, expect, beforeAll } from 'vitest';
import * as path from 'path';
import { fileURLToPath } from 'url';

// Data layer imports
import { discoverChartVersions, loadChartData, discoverScenarios, loadScenarioContent, getChartDir } from '../src/data/loader.js';
import { getTopLevelComponents, getConfigsByPrefix } from '../src/data/parser.js';
import { initializeStore, getStore, getConfigByPath, searchConfigs, getScenarios } from '../src/data/store.js';
import { getSupportedVersions, findLatestPatches, getDefaultChartPath } from '../src/data/fetcher.js';

// Tool imports
import { listVersions } from '../src/tools/listVersions.js';
import { listComponents } from '../src/tools/listComponents.js';
import { listScenarios } from '../src/tools/listScenarios.js';
import { getConfigInfo } from '../src/tools/getConfigInfo.js';
import { searchConfigs as searchConfigsTool } from '../src/tools/searchConfigs.js';
import { generateValuesExample } from '../src/tools/generateExample.js';

// Get the chart path (parent directory of helm-values-mcp)
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const CHART_PATH = path.resolve(__dirname, '../..');

describe('Data Layer', () => {
  describe('Fetcher', () => {
    it('should have supported versions configured', () => {
      const versions = getSupportedVersions();
      
      expect(versions).toContain('8.8');
      expect(versions).toContain('8.9');
      expect(versions.length).toBeGreaterThanOrEqual(2);
    });

    it('should return default chart path in home directory', () => {
      const defaultPath = getDefaultChartPath();
      
      expect(defaultPath).toContain('.helm-values-mcp');
      expect(defaultPath).toContain(process.env.HOME || '/');
    });

    it('should find latest patches from index data', () => {
      // Mock index data structure
      const mockIndex = {
        entries: {
          'camunda-platform': [
            { version: '13.4.1', appVersion: '8.8.x' },
            { version: '13.4.0', appVersion: '8.8.x' },
            { version: '13.3.0', appVersion: '8.8.x' },
            { version: '14.0.0-alpha3', appVersion: '8.9.x' },
            { version: '14.0.0-alpha2', appVersion: '8.9.x' },
            { version: '12.0.0', appVersion: '8.7.x' }, // Older, not in supported
          ]
        }
      };

      const patches = findLatestPatches(mockIndex);
      
      expect(patches.get('8.8')).toBe('13.4.1');
      // 8.9 only has alpha versions, so it should pick the latest alpha
      expect(patches.get('8.9')).toBe('14.0.0-alpha3');
      expect(patches.has('8.7')).toBe(false); // Not in SUPPORTED_VERSIONS
    });

    it('should prefer stable over alpha/beta versions', () => {
      const mockIndex = {
        entries: {
          'camunda-platform': [
            { version: '13.5.0-alpha1', appVersion: '8.8.x' },
            { version: '13.4.1', appVersion: '8.8.x' },
            { version: '13.4.0', appVersion: '8.8.x' },
          ]
        }
      };

      const patches = findLatestPatches(mockIndex);
      
      // Should pick stable 13.4.1 over alpha 13.5.0-alpha1
      expect(patches.get('8.8')).toBe('13.4.1');
    });
  });

  describe('Loader', () => {
    it('should discover chart versions sorted descending', () => {
      const versions = discoverChartVersions(CHART_PATH);
      
      expect(versions.length).toBeGreaterThan(0);
      expect(versions).toContain('8.9');
      expect(versions).toContain('8.8');
      // Versions are sorted descending
      expect(versions[0]).toBe('8.9');
    });

    it('should load chart data with metadata and configs', () => {
      const chartData = loadChartData(CHART_PATH, '8.9');
      
      expect(chartData.version).toBe('8.9');
      expect(chartData.chartVersion).not.toBe('unknown');
      expect(chartData.appVersion).toContain('8.9');
      expect(chartData.chartDescription).toContain('Camunda');
      expect(chartData.configs.size).toBeGreaterThan(100);
      expect(chartData.scenarios.length).toBeGreaterThan(0);
      expect(chartData.dependencies.size).toBeGreaterThan(0);
    });

    it('should discover scenarios with descriptions from file headers', () => {
      const chartDir = getChartDir(CHART_PATH, '8.9');
      const scenarios = discoverScenarios(chartDir);
      
      const defaultScenario = scenarios.find(s => s.name === 'default');
      expect(defaultScenario?.file).toBe('values.yaml');
      expect(defaultScenario?.description.length).toBeGreaterThan(10);
      
      const bitnamiLegacy = scenarios.find(s => s.name === 'bitnami-legacy');
      expect(bitnamiLegacy?.description).toContain('Bitnami');
    });

    it('should load scenario content', () => {
      const chartDir = getChartDir(CHART_PATH, '8.9');
      const scenarios = discoverScenarios(chartDir);
      const content = loadScenarioContent(scenarios.find(s => s.name === 'default')!.path);
      
      expect(content).toHaveProperty('global');
    });
  });

  describe('Parser', () => {
    it('should parse schema with required field tracking', () => {
      const chartData = loadChartData(CHART_PATH, '8.9');
      
      const ingressEnabled = chartData.configs.get('global.ingress.enabled');
      expect(ingressEnabled?.type).toBe('boolean');
      expect(ingressEnabled?.required).toBe(false);
      
      // env[].name is required
      const envName = chartData.configs.get('identity.env[].name');
      expect(envName?.required).toBe(true);
    });

    it('should extract top-level components', () => {
      const chartData = loadChartData(CHART_PATH, '8.9');
      const components = getTopLevelComponents(chartData.configs);
      
      expect(components).toContain('global');
      expect(components).toContain('orchestration');
      expect(components).toContain('identity');
    });

    it('should get configs by prefix', () => {
      const chartData = loadChartData(CHART_PATH, '8.9');
      const globalConfigs = getConfigsByPrefix(chartData.configs, 'global');
      
      expect(globalConfigs.length).toBeGreaterThan(0);
      expect(globalConfigs.every(c => c.path.startsWith('global'))).toBe(true);
    });
  });

  describe('Store', () => {
    beforeAll(() => {
      initializeStore(CHART_PATH);
    });

    it('should initialize store with latest version', () => {
      const store = getStore();
      
      expect(store.chartPath).toBe(CHART_PATH);
      expect(store.charts.size).toBeGreaterThan(0);
      expect(store.latestVersion).toBe('8.9');
    });

    it('should get config by path', () => {
      const config = getConfigByPath('global.ingress.enabled');
      
      expect(config?.path).toBe('global.ingress.enabled');
      expect(config?.type).toBe('boolean');
    });

    it('should search configs', () => {
      const results = searchConfigs('ingress');
      
      expect(results.length).toBeGreaterThan(0);
      expect(results.some(r => r.path.includes('ingress'))).toBe(true);
    });

    it('should get scenarios', () => {
      const scenarios = getScenarios('8.9');
      
      expect(scenarios.some(s => s.name === 'default')).toBe(true);
    });
  });
});

describe('MCP Tools', () => {
  beforeAll(() => {
    try { getStore(); } catch { initializeStore(CHART_PATH); }
  });

  describe('list_versions', () => {
    it('should return versions with chart metadata', () => {
      const result = listVersions();
      
      expect(result.latest).toBe('8.9');
      
      const v89 = result.versions.find(v => v.version === '8.9');
      expect(v89?.chartVersion).not.toBe('unknown');
      expect(v89?.appVersion).toContain('8.9');
      expect(v89?.chartDescription).toContain('Camunda');
    });
  });

  describe('list_components', () => {
    it('should return components with descriptions and dependency info', () => {
      const result = listComponents({});
      
      expect(result.version).toBe('8.9');
      
      // Check descriptions are derived
      const global = result.components.find(c => c.name === 'global');
      expect(global?.description).toContain('Global');
      expect(global?.dependency).toBeUndefined();
      
      // Check dependency info for chart dependencies
      const elasticsearch = result.components.find(c => c.name === 'elasticsearch');
      expect(elasticsearch?.dependency?.chart).toBe('elasticsearch');
      expect(elasticsearch?.dependency?.source).toBe('external');
      expect(elasticsearch?.dependency?.enableCondition).toBe('elasticsearch.enabled');
      
      // Check local vs external
      const identityKeycloak = result.components.find(c => c.name === 'identityKeycloak');
      expect(identityKeycloak?.dependency?.source).toBe('local');
    });
  });

  describe('list_scenarios', () => {
    it('should return scenarios with derived descriptions', () => {
      const result = listScenarios({});
      
      const defaultScenario = result.scenarios.find(s => s.name === 'default');
      expect(defaultScenario?.file).toBe('values.yaml');
      expect(defaultScenario?.description.length).toBeGreaterThan(10);
    });
  });

  describe('get_config_info', () => {
    it('should return full config info including required status', () => {
      const result = getConfigInfo({ path: 'global.ingress.enabled' });
      
      expect(result.path).toBe('global.ingress.enabled');
      expect(result.type).toBe('boolean');
      expect(result.required).toBe(false);
      expect(result.example).toContain('global:');
    });

    it('should include children for object types', () => {
      const result = getConfigInfo({ path: 'global.ingress' });
      
      expect(result.type).toBe('object');
      expect(result.children!.length).toBeGreaterThan(0);
    });

    it('should throw error for invalid path', () => {
      expect(() => getConfigInfo({ path: 'invalid.path.here' })).toThrow();
    });

    it('should detect deprecated configs', () => {
      const result = getConfigInfo({ path: 'global.license.key' });
      expect(result.deprecated).toBe(true);
    });

    it('should mark required fields correctly', () => {
      const result = getConfigInfo({ path: 'identity.env[].name' });
      expect(result.required).toBe(true);
    });
  });

  describe('search_configs', () => {
    it('should find configs with relevance scores', () => {
      const result = searchConfigsTool({ query: 'ingress enabled' });
      
      expect(result.results.length).toBeGreaterThan(0);
      expect(result.results[0].relevance).toBeGreaterThan(0);
      expect(result.results[0].required).toBeDefined();
    });

    it('should respect limit parameter', () => {
      const result = searchConfigsTool({ query: 'enabled', limit: 5 });
      expect(result.results.length).toBeLessThanOrEqual(5);
    });

    it('should filter by requiredOnly', () => {
      const all = searchConfigsTool({ query: 'env name', limit: 50 });
      const required = searchConfigsTool({ query: 'env name', limit: 50, requiredOnly: true });
      
      expect(required.totalFound).toBeLessThanOrEqual(all.totalFound);
      for (const r of required.results) {
        expect(r.required).toBe(true);
      }
    });
  });

  describe('generate_values_example', () => {
    it('should generate example from scenario', () => {
      const result = generateValuesExample({ scenario: 'default' });
      
      expect(result.yaml).toBeTruthy();
      expect(result.sourceFile).toBe('values.yaml');
    });

    it('should filter by component', () => {
      const result = generateValuesExample({ scenario: 'default', component: 'global.ingress' });
      
      expect(result.yaml).toContain('global:');
      expect(result.yaml).toContain('ingress:');
    });

    it('should throw error for invalid scenario', () => {
      expect(() => generateValuesExample({ scenario: 'nonexistent' })).toThrow();
    });
  });
});

describe('Integration', () => {
  beforeAll(() => {
    try { getStore(); } catch { initializeStore(CHART_PATH); }
  });

  it('should handle full workflow: versions -> components -> search -> config info', () => {
    const versions = listVersions();
    const components = listComponents({ version: versions.latest });
    const search = searchConfigsTool({ query: 'ingress enabled' });
    const info = getConfigInfo({ path: search.results[0].path });
    
    expect(info.path).toBe(search.results[0].path);
  });

  it('should handle scenario workflow: list -> generate', () => {
    const scenarios = listScenarios({});
    const example = generateValuesExample({ scenario: scenarios.scenarios[0].name });
    
    expect(example.yaml).toBeTruthy();
  });
});
