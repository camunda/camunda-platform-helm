/**
 * Tool: list_scenarios
 * List available scenario files (values-*.yaml)
 */

import type { ListScenariosInput, ScenarioFile } from '../types/index.js';
import { getChartData, getScenarios } from '../data/store.js';

export interface ListScenariosOutput {
  version: string;
  scenarios: ScenarioFile[];
}

export function listScenarios(input: ListScenariosInput): ListScenariosOutput {
  const chartData = getChartData(input.version);
  const scenarios = getScenarios(input.version);
  
  return {
    version: chartData.version,
    scenarios,
  };
}
