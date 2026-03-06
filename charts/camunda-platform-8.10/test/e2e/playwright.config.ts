import { defineConfig } from "@playwright/test";
import * as dotenv from "dotenv";
import * as fs from "fs";
import * as path from "path";

dotenv.config();

// TODO: Remove the fallback once QA publishes the SM-8.10 e2e test suite
//  in the @camunda/e2e-test-suite npm package.
const smTestDir = "./node_modules/@camunda/e2e-test-suite/dist/tests/SM-8.10";
const testDir = fs.existsSync(path.resolve(__dirname, smTestDir))
  ? smTestDir
  : "./empty-test-dir";

// When SM-8.10 is missing, create a fallback directory with a single skipped
// test so Playwright exits with code 0 instead of failing on "No tests found".
// Named smoke-tests.spec.js so it matches both the "smoke-tests" and
// "full-suite" project globs.
const absEmpty = path.resolve(__dirname, "./empty-test-dir");
if (!fs.existsSync(absEmpty)) {
  fs.mkdirSync(absEmpty, { recursive: true });
}
const skipFile = path.resolve(absEmpty, "smoke-tests.spec.js");
if (!fs.existsSync(skipFile)) {
  fs.writeFileSync(
    skipFile,
    `const { test } = require("@playwright/test");\n` +
      `test.skip("SM-8.10 e2e suite not yet published", () => {});\n`
  );
}

export default defineConfig({
  testDir,
  projects: [
    {
      name: "smoke-tests",
      testMatch: ["**/smoke-tests.spec.{ts,js}"],
    },
    {
      name: "full-suite",
      testMatch: ["**/*.spec.{ts,js}"],
    },
  ],
  fullyParallel: true,
  retries: 2,
  timeout: 10 * 60 * 1000, // no test should take more than 3 minutes (failing fast is important so that we can run our tests on each PR)
  workers: "100%",
  //workers: process.env.CI == "true" ? 1 : "50%",
  use: {
    baseURL: getBaseURL(),
    actionTimeout: 10000,
    // Also applies to flaky tests
    screenshot: "only-on-failure",
    video: "retain-on-failure",
    trace: "on-first-retry",
  },
});

function getBaseURL(): string {
  if (process.env.IS_PROD === "true") {
    return "https://console.camunda.io";
  }

  if (typeof process.env.PLAYWRIGHT_BASE_URL === "string") {
    return process.env.PLAYWRIGHT_BASE_URL;
  }

  if (process.env.MINOR_VERSION?.includes("SM")) {
    return "https://gke-" + process.env.BASE_URL + ".ci.distro.ultrawombat.com";
  }

  if (process.env.MINOR_VERSION?.includes("Run")) {
    return "http://localhost:8080";
  }

  return "https://console.cloud.ultrawombat.com";
}
