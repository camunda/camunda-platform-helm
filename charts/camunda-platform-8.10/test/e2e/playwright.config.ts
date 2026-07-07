/// <reference types="node" />
/// <reference lib="esnext" />

import { defineConfig } from "@playwright/test";
import * as dotenv from "dotenv";
import * as fs from "fs";
import * as path from "path";

import { makeShadowConfig } from "../../../../test/e2e/playwright.base.config";

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

// Auth0 smoke directory in @camunda/e2e-test-suite. Distinct from the SM-8.10
// suite because the QA-owned smoke-tests.spec.js calls setupKeycloakUser which
// needs a real Keycloak admin — incompatible with the Auth0 deployment.
const auth0TestDir = path.resolve(
  __dirname,
  "./node_modules/@camunda/e2e-test-suite/dist/tests/auth0",
);

export default defineConfig(makeShadowConfig({
  version: "SM-8.10",
  testDir,
  includeSetupProject: true,
  extraProjects: [
    {
      // Auth0 scenario: HTTP-level smoke that asserts each Camunda component
      // route redirects to the Auth0 issuer with a well-formed authorize URL.
      // No browser fixtures, no Keycloak admin — just request/response checks.
      name: "auth0-smoke",
      testDir: auth0TestDir,
      testMatch: ["**/*.spec.{ts,js}"],
    },
  ],
}));
