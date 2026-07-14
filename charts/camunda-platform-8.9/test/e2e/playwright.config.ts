/// <reference types="node" />
/// <reference lib="esnext" />

import { defineConfig } from "@playwright/test";
import * as dotenv from "dotenv";

import { makeShadowConfig } from "../../../../test/e2e/playwright.base.config";

dotenv.config();

export default defineConfig(makeShadowConfig({ version: "SM-8.9", includeSetupProject: false, fullyParallel: true, retries: 2, timeout: 10 * 60 * 1000, workers: "100%" }));
