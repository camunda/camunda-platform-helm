/// <reference types="node" />
/// <reference lib="esnext" />

import { defineConfig } from "@playwright/test";
import * as dotenv from "dotenv";

import { apiProjects } from "../../../../test/e2e/playwright.api-projects";
import { makeShadowConfig } from "../../../../test/e2e/playwright.base.config";

dotenv.config();

export default defineConfig(
  makeShadowConfig({
    version: "SM-8.9",
    includeSetupProject: true,
    extraTestIgnore: ["**/topology-orchestration-smoke.spec.{ts,js}"],
    fullyParallel: true,
    retries: 2,
    timeout: 10 * 60 * 1000,
    workers: "100%",
    extraProjects: apiProjects(__dirname),
  }),
);
