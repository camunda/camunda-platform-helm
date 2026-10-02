/// <reference types="node" />

import * as fs from "fs";
import * as path from "path";

// Returns the "api" and "clock" projects of the Orchestration Cluster REST v2
// API suite from @camunda/e2e-test-suite's dist/tests/api/projects.js. Their
// testDir is absolute, so the chart config's testDir does not apply to them.
//
// Returns no projects when the installed package predates that module, unless
// REQUIRE_API_TEST_SUITE=true, which throws instead.
export function apiProjects(chartE2EDir: string): any[] {
  const modulePath = path.resolve(
    chartE2EDir,
    "node_modules/@camunda/e2e-test-suite/dist/tests/api/projects.js",
  );
  if (!fs.existsSync(modulePath)) {
    if (process.env.REQUIRE_API_TEST_SUITE === "true") {
      throw new Error(
        `The REST v2 API test suite is not installed: ${modulePath} is missing`,
      );
    }
    return [];
  }
  return require(modulePath).apiProjects;
}
