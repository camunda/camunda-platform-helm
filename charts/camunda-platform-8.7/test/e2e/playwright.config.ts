import { defineConfig } from "@playwright/test";
import * as dotenv from "dotenv";

dotenv.config();

export default defineConfig({
  testDir: "./node_modules/@camunda/e2e-test-suite/dist/tests/SM-8.7",
  projects: [
    {
      name: "smoke-tests",
      testMatch: ["**/smoke-tests.spec.{ts,js}"],
    },
    {
      name: "full-suite",
      testMatch: ["**/*.spec.{ts,js}"],
      testIgnore: [/test-setup\.spec\.[jt]s$/],
      // Exclude tests that require QA-specific config:
      // - Connector Secrets: Requires Vault-managed secrets
      // - Custom Tags/Custom Properties: Require Console QA cluster config
      grep: /^(?!.*(Connector Secrets User Flow|Custom Tags|Custom Properties)).*$/,
    },
  ],
  // Match the E2E repo's SM-8.7 settings: short timeout so tests that hit
  // unresponsive services (e.g. web-modeler-restapi) fail fast instead of
  // hanging until the job timeout.
  fullyParallel: false,
  retries: 1,
  timeout: 3 * 60 * 1000,
  workers: process.env.CI === "true" ? 37 : "100%",
  use: {
    baseURL: getBaseURL(),
    actionTimeout: 10000,
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
