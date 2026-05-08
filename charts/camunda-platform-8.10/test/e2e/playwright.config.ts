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

// Local auth0 smoke directory. Lives in this repo (not in @camunda/e2e-test-suite)
// because the QA-owned smoke-tests.spec.js calls setupKeycloakUser which needs a
// real Keycloak admin — incompatible with the Auth0 deployment.
const auth0TestDir = path.resolve(__dirname, "./auth0-tests");

export default defineConfig({
  testDir,
  projects: [
    {
      name: "smoke-tests",
      testMatch: ["**/smoke-tests.spec.{ts,js}"],
    },
    {
      name: "full-suite-setup",
      testMatch: ["**/test-setup.spec.{ts,js}"],
      use: {
        extraHTTPHeaders: {
          "X-Test-Tasklist-Version": "v2",
        },
      },
    },
    {
      name: "full-suite",
      dependencies: ["full-suite-setup"],
      testMatch: ["**/*.spec.{ts,js}"],
      // cluster-variables requires Vault-managed secrets not available in PR CI.
      // test-setup is run via the full-suite-setup dependency, not directly.
      testIgnore: ["**/cluster-variables.spec.{ts,js}", "**/test-setup.spec.{ts,js}"],
      // Match the E2E repo's chromium-v2 project behavior:
      // - @tasklistV1: These tests require Tasklist v1 mode with RBA enabled
      //   (CAMUNDA_TASKLIST_IDENTITY_RESOURCE_PERMISSIONS_ENABLED=true), which
      //   is not deployed in standard PR CI scenarios. Also excludes all Optimize
      //   tests (under 'Optimize User Flow Tests @tasklistV1' describe) which
      //   require long Optimize warm-up not available on fresh clusters.
      // - Connector Secrets/Custom Tags/Properties: Require QA-specific config.
      grep: /^(?!.*(@tasklistV1|Connector Secrets User Flow|Custom Tags|Custom Properties)).*$/,
      use: {
        extraHTTPHeaders: {
          "X-Test-Tasklist-Version": "v2",
        },
      },
    },
    {
      // Auth0 scenario: HTTP-level smoke that asserts each Camunda component
      // route redirects to the Auth0 issuer with a well-formed authorize URL.
      // No browser fixtures, no Keycloak admin — just request/response checks.
      name: "auth0-smoke",
      testDir: auth0TestDir,
      testMatch: ["**/*.spec.{ts,js}"],
    },
  ],
  fullyParallel: true,
  retries: 2,
  timeout: 12 * 60 * 1000, // match E2E repo timeout (12 minutes); test.slow() triples this
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
