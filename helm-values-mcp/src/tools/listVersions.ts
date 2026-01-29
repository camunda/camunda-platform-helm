/**
 * Tool: list_versions
 * List all available chart versions
 */

import { getVersions, getChartData } from '../data/store.js';

export interface VersionInfo {
  version: string;
  chartVersion: string;
  appVersion: string;
  chartDescription: string;
}

export interface ListVersionsOutput {
  versions: VersionInfo[];
  latest: string;
}

export function listVersions(): ListVersionsOutput {
  const { versions, latest } = getVersions();
  
  const versionInfos: VersionInfo[] = versions.map(v => {
    const chartData = getChartData(v);
    return {
      version: v,
      chartVersion: chartData.chartVersion,
      appVersion: chartData.appVersion,
      chartDescription: chartData.chartDescription,
    };
  });
  
  return {
    versions: versionInfos,
    latest,
  };
}
