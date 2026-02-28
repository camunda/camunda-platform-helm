/**
 * Tool: search_configs
 * Search configs by keyword or description
 */

import type { SearchConfigsInput, SearchResult } from '../types/index.js';
import { getChartData, searchConfigs as searchStore } from '../data/store.js';

export interface SearchConfigsOutput {
  version: string;
  query: string;
  results: SearchResult[];
  totalFound: number;
}

export function searchConfigs(input: SearchConfigsInput): SearchConfigsOutput {
  const chartData = getChartData(input.version);
  const limit = input.limit || 10;
  
  let entries = searchStore(input.query, input.version, limit * 2); // Get extra to filter
  
  // Filter to required only if requested
  if (input.requiredOnly) {
    entries = entries.filter(e => e.required === true);
  }
  
  // Calculate relevance scores for output
  const queryLower = input.query.toLowerCase();
  const queryTerms = queryLower.split(/\s+/).filter(t => t.length > 0);
  
  const results: SearchResult[] = entries.slice(0, limit).map(entry => {
    let relevance = 0;
    const pathLower = entry.path.toLowerCase();
    const descLower = entry.description.toLowerCase();
    
    for (const term of queryTerms) {
      if (pathLower === term) relevance += 1.0;
      else if (pathLower.includes(term)) relevance += 0.5;
      if (descLower.includes(term)) relevance += 0.2;
    }
    
    // Normalize relevance to 0-1 range
    relevance = Math.min(1, relevance / queryTerms.length);
    
    return {
      path: entry.path,
      type: entry.type,
      description: entry.description,
      default: entry.default,
      relevance: Math.round(relevance * 100) / 100,
      required: entry.required,
    };
  });
  
  return {
    version: chartData.version,
    query: input.query,
    results,
    totalFound: results.length,
  };
}
