// NOTE: If you get the message connection error: desc = "error reading server preface: http2: frame too large"V
// this is likely due to --insecure on the zbctl call while the endpoint is TLS enabled.

/// <reference types="node" />
/*
import { config as dotenv } from "dotenv";
dotenv(); // ← loads .env before anything else

import { test, expect, APIRequestContext } from "@playwright/test";
import { execFileSync } from "child_process";
import { test as c8test } from "playwright-automation/dist/fixtures/SM-8.7";

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
  base: {
    console: requireEnv("CONSOLE_BASE_URL"),
    keycloak: requireEnv("KEYCLOAK_BASE_URL"),
    identity: requireEnv("IDENTITY_BASE_URL"),
    operate: requireEnv("OPERATE_BASE_URL"),
    optimize: requireEnv("OPTIMIZE_BASE_URL"),
    tasklist: requireEnv("TASKLIST_BASE_URL"),
    webModeler: requireEnv("WEBMODELER_BASE_URL"),
    connectors: requireEnv("CONNECTORS_BASE_URL"),
    zeebeGRPC: requireEnv("ZEEBE_GATEWAY_GRPC"),
    zeebeREST: requireEnv("ZEEBE_GATEWAY_REST"),
  },
  loginPath: {
    Console: requireEnv("CONSOLE_LOGIN_PATH"),
    Keycloak: requireEnv("KEYCLOAK_LOGIN_PATH"),
    Identity: process.env["IDENTITY_LOGIN_PATH"],
    Operate: requireEnv("OPERATE_LOGIN_PATH"),
    Optimize: requireEnv("OPTIMIZE_LOGIN_PATH"),
    Tasklist: requireEnv("TASKLIST_LOGIN_PATH"),
    WebModeler: requireEnv("WEBMODELER_LOGIN_PATH"),
    connectors: requireEnv("CONNECTORS_LOGIN_PATH"),
    zeebeGRPC: requireEnv("ZEEBE_GATEWAY_GRPC"),
    zeebeREST: requireEnv("ZEEBE_GATEWAY_REST"),
  },
  secrets: {
    connectors: requireEnv("PLAYWRIGHT_VAR_CONNECTORS_CLIENT_SECRET"),
    tasklist: requireEnv("PLAYWRIGHT_VAR_TASKLIST_CLIENT_SECRET"),
    operate: requireEnv("PLAYWRIGHT_VAR_OPERATE_CLIENT_SECRET"),
    optimize: requireEnv("PLAYWRIGHT_VAR_OPTIMIZE_CLIENT_SECRET"),
    zeebe: requireEnv("PLAYWRIGHT_VAR_TEST_CLIENT_SECRET"),
  },
  venomID: process.env.TEST_CLIENT_ID ?? "venom",
  venomSec: requireEnv("PLAYWRIGHT_VAR_TEST_CLIENT_SECRET"),
  fixturesDir: process.env.FIXTURES_DIR || "/mnt/fixtures",
};

// ---------- tests ----------
test.describe("Camunda core", () => {
  for (const name of [
    //    "Console",
    "Tasklist",
    //    "Modeler",
    //    "Optimize",
    //    "Operate",
    //    "Identity",
  ]) {
    c8test(
      `Go to the ${name} homeage`,
      async ({
        taskDetailsPage,
        taskPanelPage,
        modelerHomePage,
        navigationPage,
        modelerCreatePage,
        operateHomePage,
        operateProcessesPage,
        operateProcessInstancePage,
        page,
      }) => {
        navigationPage.goToTasklist()
        //await navigationPage[`goTo${name}`]();
      },
    );
  }
});
*/