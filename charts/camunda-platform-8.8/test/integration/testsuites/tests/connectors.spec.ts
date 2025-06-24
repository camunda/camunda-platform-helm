/// <reference types="node" />
import { config as dotenv } from "dotenv";
dotenv(); // ← loads .env before anything else

import { test, expect, APIRequestContext } from "@playwright/test";
import { execFileSync } from "child_process";

// ---------- config & helpers ----------

// Helper to require environment variables
function requireEnv(name: string): string {
  const value = process.env[name];
  if (!value) throw new Error(`Missing required env var: ${name}`);
  return value;
}

// Grouped config for base URLs
const config = {
  authURL: requireEnv("AUTH_URL"),
  testBasePath: requireEnv("TEST_BASE_PATH"),
  base: {
    connectors: requireEnv("CONNECTORS_BASE_URL"),
  },
  loginPath: {
    connectors: requireEnv("CONNECTORS_LOGIN_PATH"),
  },
  secrets: {
    connectors: requireEnv("PLAYWRIGHT_VAR_CONNECTORS_CLIENT_SECRET"),
  },
  venomID: process.env.TEST_CLIENT_ID ?? "venom",
  venomSec: requireEnv("PLAYWRIGHT_VAR_TEST_CLIENT_SECRET"),
};

// Helper to fetch a token
async function fetchToken(id: string, sec: string, api: APIRequestContext) {
  const r = await api.post(config.authURL, {
    form: {
      client_id: id,
      client_secret: sec,
      grant_type: "client_credentials",
    },
  });
  expect(
    r.ok(),
    `Failed to get token for client_id=${id}: ${r.status()}`,
  ).toBeTruthy();
  return (await r.json()).access_token as string;
}

// ---------- tests ----------
test.describe("Camunda core", () => {
  let api: APIRequestContext;
  let venomJWT: string;

  test.beforeAll(async ({ playwright }) => {
    api = await playwright.request.newContext();
    venomJWT = await fetchToken(config.venomID, config.venomSec, api);
  });

  test("Connectors inbound page", async () => {
    expect(
      (await api.get(config.base.connectors, { timeout: 45_000 })).ok(),
      "Connectors inbound page failed",
    ).toBeTruthy();
  });

  test(`TEST - Check Connectors webhook`, async () => {
    const r = await api.post(config.base.connectors + '/test-mywebhook', {
      data: { "webhookDataKey": "webhookDataValue" },
      headers: {
        Authorization: `Bearer ${venomJWT}`,
        //Authorization: `Basic ZGVtbzpkZW1v`,
        "Content-Type": "application/json",
      },
    });
    expect(
      r.ok(),
      `API call failed with ${r.status()}`,
    ).toBeTruthy();
  });

  test.afterAll(async ({ }, testInfo) => {
    // If the test outcome is different from what was expected (i.e. the test failed),
    // dump the resolved configuration so that it is visible in the Playwright output.
    if (testInfo.status !== testInfo.expectedStatus) {
      // Secrets are dumped as-is because the surrounding CI already treats logs as sensitive.
      // If this becomes a concern, mask the values here before logging.
      console.error(
        "\n===== CONFIG DUMP (test failed) =====\n" +
        JSON.stringify(config, null, 2),
      );
    }
  });
});
