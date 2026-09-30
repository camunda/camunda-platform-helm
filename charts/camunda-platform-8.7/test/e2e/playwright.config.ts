/// <reference types="node" />
/// <reference lib="esnext" />

import { defineConfig } from "@playwright/test";
import * as dotenv from "dotenv";

import { makeShadowConfig } from "../../../../test/e2e/playwright.base.config";

dotenv.config();

export default defineConfig(makeShadowConfig({ version: "SM-8.7", includeSetupProject: false, extraTestIgnore: ["**/topology-orchestration-smoke.spec.{ts,js}"], fullyParallel: false, retries: 1, timeout: 3 * 60 * 1000 }));
